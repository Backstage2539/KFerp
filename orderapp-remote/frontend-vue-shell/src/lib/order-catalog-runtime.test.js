import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { storedSampleSpecKey } from './sample-order-edit.js'

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

for (const mode of ['frozen conversion', 'stored sample']) {
  test(`catalog refresh preserves ${mode} lines while validating ordinary lines`, async () => {
    let editor
    await renderToString(createSSRApp({ setup() {
      editor = View.setup({ embedded: true, editId: 1, copyId: 0, customerPortal: false, fulfillmentMode: false, viewParams: {} }, { expose() {}, emit() {} })
      return () => h('div')
    } }))
    editor.loading.value = true
    editor.form.customer_id = 487
    editor.form.edit_id = 1
    editor.sampleOrderEditing.value = mode === 'stored sample'
    const row = editor.rows.value[0]
    Object.assign(row, {
      item_id: 1, product_id: 100, parent_product_id: 100, bom_spec_id: 901, bom_variant_id: 1901,
      bean_list_publication_id: 10, unit_price: mode === 'stored sample' ? '0' : '38', qty: 2,
      price_source_json: '{"publication_id":10,"frozen":true}', spec_source: 'price_list_sku',
      historical_spec_readonly: false, spec_invalid_message: '', price_missing: false,
      price_selection_mode: mode === 'stored sample' ? 'manual' : 'bom_tier',
      spec_conversion: mode === 'stored sample' ? null : { reprice: false },
      manual_price: mode === 'stored sample', sample_stored_manual: true,
    })
    if (mode === 'stored sample') row.sample_stored_spec_key = storedSampleSpecKey(row)
    const snapshot = JSON.stringify(row)
    editor.selectedBeanListPublicationIDs['classification:88'] = 10
    const requests = []
    let updated = true
    globalThis.catalogRuntimeAPI = async url => {
      requests.push(url)
      const id = url.includes('selected_price_table_ids=13') ? 13 : 12
      return { customer_id: 487, catalog_updated: updated, selected_price_table_ids: [id], product_families: [], price_table_options: [
        { id: 12, list_type: 'commercial', classification_template_id: 88, is_default: true },
        { id: 13, list_type: 'commercial', classification_template_id: 88 },
      ] }
    }
    try {
      assert.equal(await editor.refreshStaffOrderCatalog(), true)
      assert.doesNotMatch(requests[0], /selected_price_table_ids=10(?:&|$)/, 'historical-only publication must not block reading the current catalog')
      assert.equal(JSON.stringify(row), snapshot)
      assert.equal(editor.rowHasBlockingPrice(row), false)
      updated = false
      editor.setBeanListVersion('classification:88', 13)
      await editor.refreshStaffOrderCatalog()
      assert.equal(JSON.stringify(row), snapshot, 'manual table switches must preserve frozen lines')
      const ordinary = { ...row, key: 'ordinary', item_id: 2, price_selection_mode: 'auto', spec_conversion: null, manual_price: false, historical_spec_readonly: false, sample_stored_spec_key: '' }
      editor.rows.value.push(ordinary)
      editor.selectedBeanListPublicationIDs['classification:88'] = 10
      updated = true
      await editor.refreshStaffOrderCatalog()
      assert.match(requests.at(-1), /selected_price_table_ids=10(?:&|$)/, 'ordinary lines must still check their old publication')
      assert.equal(editor.rowHasBlockingPrice(row), false)
      assert.equal(editor.rowHasBlockingPrice(ordinary), true)
    } finally { delete globalThis.catalogRuntimeAPI }
  })
}
