import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { buildOrderPayload } from './order-entry.js'

test('date-only retained drip order keeps its frozen package quantity and weight', async () => {
  const filename = new URL('../views/OrderEntryView.vue', import.meta.url)
  const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
  let code = compileScript(descriptor, { id: 'retained-drip-date' }).content
  code = code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, (_, name) => `const ${name}={render:()=>null}`)
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, filename).href : import.meta.resolve(path))}`)
  const { default: View } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
  let editor
  await renderToString(createSSRApp({ setup() {
    editor = View.setup({ embedded: true, editId: 12, copyId: 0, customerPortal: false, fulfillmentMode: false, viewParams: {} }, { expose() {}, emit() {} })
    return () => h('div')
  } }))
  editor.productFamilies.value = [{ __order_product_family: true, __order_concrete_price_family: true,
    parent_product_id: 8758, customer_id: 8, product_kind: 'drip_bag', name: '测试挂耳',
    specs: [{ sku_id: 8769, bom_spec_id: 8769, bom_variant_id: 9903, product_kind: 'drip_bag',
      migration_state: 'cutover', inventory_unit: '盒', sales_unit: 'box', unit_bag_count: 10, unit_bean_g: 10,
      spec_name: '10袋装', tiers: [{ publication_id: 51, unit_price: 30, sales_unit: 'box', unit_bag_count: 10, unit_bean_g: 10 }] }] }]
  editor.beanListVersionOptions.value = [{ id: 51, customer_id: 8, list_type: 'drip', is_customer_owned: true, is_default: true }]
  editor.applyEditData({ edit_id: 12, customer_id: 8, delivery_mode: 'customer_stock', delivery_warehouse_code: 'CUSTOMER-8',
    items: [{ item_id: 21, product_id: 8758, bom_spec_id: 8769, bom_variant_id: 9902, product_kind: 'drip_bag',
      product_name: '测试挂耳', spec: '12袋装', sales_unit: 'box', unit_bag_count: 12, unit_bean_g: 8,
      qty: 4, unit: '盒', unit_price: '30', bean_list_publication_id: 51, price_source_json: '{}' }] })
  editor.form.order_date = '2026-11-05'
  editor.repriceHydratedRows()
  const payload = buildOrderPayload({ form: editor.form, rows: editor.rows.value })
  assert.deepEqual(payload.sales_unit, ['box'])
  assert.deepEqual(payload.unit_bag_count, ['12'], 'historical box count must not follow the current catalog')
  assert.deepEqual(payload.unit_bean_g, ['8'], 'historical bag weight must not follow the current catalog')
})
