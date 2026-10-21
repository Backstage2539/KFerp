import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

const filename = new URL('../views/OrderEntryView.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
let code = compileScript(descriptor, { id: 'order-catalog-runtime' }).content
code = code.replace("import { apiGet, apiSend } from '../api/client'", 'const apiGet = (...args) => globalThis.catalogRuntimeAPI(...args); const apiSend = () => { throw Error("unexpected write") }')
code = code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, (_, name) => `const ${name}={render:()=>null}`)
code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, filename).href : import.meta.resolve(path))}`)
const { default: View } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)

test('real Vue order editor uses selected published tables for a retail customer, including new categories', async () => {
  let editor
  await renderToString(createSSRApp({ setup() {
    editor = View.setup({ embedded: true, editId: 0, copyId: 0, customerPortal: false, fulfillmentMode: false, viewParams: {} }, { expose() {}, emit() {} })
    return () => h('div')
  } }))
  editor.loading.value = true // Suppress automatic requests while arranging the editor.
  editor.form.customer_id = 487
  editor.form.order_type_id = 2
  editor.orderTypes.value = [{ id: 2, name: '零售订单' }]
  const tables = [{ id: 10, list_type: 'commercial', classification_template_id: 88, classification_template_name: '新商品类型', is_default: true }, { id: 11, list_type: 'retail', classification_template_id: 88 }]
  const family = id => ({ parent_product_id: 100, name: '已发布商品', product_kind: 'generic', visibility: 'public', migration_state: 'cutover', specs: [{ product_id: 100, bom_spec_id: 901, bom_variant_id: 1901, migration_state: 'cutover', spec_name: '袋装', inventory_unit: '袋', tiers: [{ publication_id: id, list_type: id === 11 ? 'retail' : 'commercial', quantity_basis: 'sales_spec_count', min: 1, unit_price: id === 11 ? 48 : 38 }] }] })
  const requests = []
  globalThis.catalogRuntimeAPI = async url => {
    requests.push(url)
    const id = url.includes('selected_price_table_ids=11') ? 11 : 10
    return { customer_id: 487, price_table_options: tables, selected_price_table_ids: [id], product_families: [family(id)] }
  }
  try {
    await editor.refreshStaffOrderCatalog()
    assert.equal(editor.retailOrder.value, true)
    assert.equal(editor.productOptions(editor.rows.value[0]).length, 1)
    assert.deepEqual(editor.beanListVersionGroups.value[0].options.map(row => row.id), [10, 11])
    assert.equal(editor.productKindFilterOptions()[1].label, '新商品类型')
    editor.chooseProduct(editor.rows.value[0], editor.productOptions(editor.rows.value[0])[0])
    assert.equal(Number(editor.rows.value[0].unit_price), 38)
    editor.setBeanListVersion('classification:88', 11)
    await editor.refreshStaffOrderCatalog()
    assert.equal(editor.rows.value[0].spec_invalid_message, '')
    assert.equal(editor.rows.value[0].bom_spec_id, 901)
    assert.equal(editor.rows.value[0].bean_list_publication_id, 11)
    assert.equal(Number(editor.rows.value[0].unit_price), 48)
    assert.equal(requests.length, 2)
  } finally { delete globalThis.catalogRuntimeAPI }
})
