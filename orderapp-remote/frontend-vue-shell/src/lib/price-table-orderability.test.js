import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { priceTableOrderabilityBlockedReason } from './price-table-orderability.js'

test('legacy green bean SKU cannot generate an orderable price table', () => {
  const families = [{ parent_product_id: 97, name: '测试生豆' }]
  const reason = priceTableOrderabilityBlockedReason([{ parent_product_id: 97, sku_id: 806 }], families)
  assert.match(reason, /测试生豆/)
  assert.match(reason, /录单/)
  assert.match(reason, /BOM/)
  assert.equal(priceTableOrderabilityBlockedReason([{ parent_product_id: 97, bom_spec_id: 9001, bom_variant_id: 9101 }], families), '')
  assert.equal(priceTableOrderabilityBlockedReason([], families), '')
})

test('PDF generation validates orderability before saving the generated draft', () => {
  const source = readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
  const generate = source.slice(source.indexOf('async function generateBeanListPdf()'), source.indexOf('async function publishBeanList()'))
  assert.ok(generate.includes('priceListOrderabilityBlockedReason.value'))
  assert.ok(generate.indexOf('/validate-orderability') >= 0)
  assert.ok(generate.indexOf('/validate-orderability') < generate.indexOf("'/api/costing/bean-list/drafts'"))
  assert.match(source, /if \(priceListOrderabilityBlockedReason.value\) return priceListOrderabilityBlockedReason.value/)
})
