import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { buildOrderPayload } from './order-entry.js'

test('date-only retained order edit preserves historical variant and price source through hydration and repricing', async () => {
  const filename = new URL('../views/OrderEntryView.vue', import.meta.url)
  const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
  let code = compileScript(descriptor, { id: 'retained-date-hydration' }).content
  code = code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, (_, name) => `const ${name}={render:()=>null}`)
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, filename).href : import.meta.resolve(path))}`)
  const { default: View } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
  let editor
  await renderToString(createSSRApp({ setup() {
    editor = View.setup({ embedded: true, editId: 12, copyId: 0, customerPortal: false, fulfillmentMode: false, viewParams: {} }, { expose() {}, emit() {} })
    return () => h('div')
  } }))
  editor.productFamilies.value = [{ __order_product_family: true, __order_concrete_price_family: true,
    parent_product_id: 8758, customer_id: 8, customer_product_reference_id: 7766, customer_product_alias_id: 0,
    name: '客户商品', specs: [{ sku_id: 8769, bom_spec_id: 8769, bom_variant_id: 9903,
      migration_state: 'cutover', inventory_unit: '袋', spec_name: '227g', tiers: [{ publication_id: 51, unit_price: 30 }] }] }]
  editor.beanListVersionOptions.value = [{ id: 51, customer_id: 8, list_type: 'commercial', is_customer_owned: true, is_default: true }]
  editor.applyEditData({ edit_id: 12, customer_id: 8, delivery_mode: 'customer_stock', delivery_warehouse_code: 'CUSTOMER-8',
    items: [{ item_id: 21, product_id: 8758, bom_spec_id: 8769, bom_variant_id: 9902,
      customer_product_reference_id: 7766, customer_product_alias_id: 0, product_name: '客户商品',
      spec: '227g', qty: 4, unit: '袋', unit_price: '30', bean_list_publication_id: 51,
      price_source_json: '{"customer_reference_snapshot":{"customer_product_alias_id":7766},"effective_sales_spec":{"bom_variant_id":9901}}' }] })
  const row = editor.rows.value[0]
  assert.equal(row.customer_product_reference_id, 7766)
  editor.form.order_date = '2026-11-05'
  editor.repriceHydratedRows()
  editor.syncRowsForType({ priceListChanged: true, preserveRetainedSnapshots: true })
  const payload = buildOrderPayload({ form: editor.form, rows: editor.rows.value })
  assert.deepEqual(payload.customer_product_reference_id, ['7766'])
  assert.deepEqual(payload.customer_product_alias_id, ['0'], 'a date-only edit must submit the original stored alias identity')
  assert.deepEqual(payload.bom_variant_id, ['9902'], 'the later catalog variant must not replace the historical order variant')
  assert.deepEqual(payload.price_source_json, ['{"customer_reference_snapshot":{"customer_product_alias_id":7766},"effective_sales_spec":{"bom_variant_id":9901}}'])
  editor.syncRowsForType({ priceListChanged: true })
  const changed = buildOrderPayload({ form: editor.form, rows: editor.rows.value })
  assert.notDeepEqual(changed.bom_variant_id, ['9902'], 'an explicit price table change must still use the newly selected specification')
})
