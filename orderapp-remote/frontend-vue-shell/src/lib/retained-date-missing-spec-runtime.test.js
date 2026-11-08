import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { buildOrderPayload } from './order-entry.js'

async function setupEditor() {
  const filename = new URL('../views/OrderEntryView.vue', import.meta.url)
  const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
  let code = compileScript(descriptor, { id: 'retained-missing-spec' }).content
  code = code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, (_, name) => `const ${name}={render:()=>null}`)
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, filename).href : import.meta.resolve(path))}`)
  const { default: View } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
  let editor
  await renderToString(createSSRApp({ setup() {
    editor = View.setup({ embedded: true, editId: 12, copyId: 0, customerPortal: false, fulfillmentMode: false, viewParams: {} }, { expose() {}, emit() {} })
    return () => h('div')
  } }))
  editor.beanListVersionOptions.value = [{ id: 51, customer_id: 8, list_type: 'commercial', is_customer_owned: true, is_default: true }]
  return editor
}

const order = () => ({ edit_id: 12, customer_id: 8, delivery_mode: 'customer_stock', delivery_warehouse_code: 'CUSTOMER-8',
  items: [{ item_id: 21, product_id: 8758, bom_spec_id: 8769, bom_variant_id: 9902,
    product_kind: 'roasted_bean', product_name: '测试商品', spec: '227g', qty: 4, unit: '袋', unit_price: '30',
    bean_list_publication_id: 51, price_source_json: '{}' }] })

test('date-only edit can preserve an original specification absent from the current catalog', async () => {
  const editor = await setupEditor()
  editor.productFamilies.value = []
  editor.applyEditData(order())
  editor.form.order_date = '2026-11-05'
  editor.repriceHydratedRows()
  assert.equal(editor.rowHasBlockingPrice(editor.rows.value[0]), false, 'the unchanged historical line must remain eligible for date-only save')
  const payload = buildOrderPayload({ form: editor.form, rows: editor.rows.value })
  assert.deepEqual(payload.bom_spec_id, ['8769'])
  assert.deepEqual(payload.bom_variant_id, ['9902'])
})

test('date-only edit preserves the historical kind when the catalog kind changes', async () => {
  const editor = await setupEditor()
  editor.productFamilies.value = [{ __order_product_family: true, __order_concrete_price_family: true,
    parent_product_id: 8758, customer_id: 8, product_kind: 'drip_bag', name: '测试商品',
    specs: [{ sku_id: 8769, bom_spec_id: 8769, bom_variant_id: 9903, product_kind: 'drip_bag', migration_state: 'cutover',
      inventory_unit: '袋', spec_name: '227g', tiers: [{ publication_id: 51, unit_price: 30 }] }] }]
  editor.applyEditData(order())
  editor.form.order_date = '2026-11-05'
  editor.repriceHydratedRows()
  const payload = buildOrderPayload({ form: editor.form, rows: editor.rows.value })
  assert.deepEqual(payload.product_kind, ['roasted_bean'], 'current product category must not rewrite the historical line')
})
