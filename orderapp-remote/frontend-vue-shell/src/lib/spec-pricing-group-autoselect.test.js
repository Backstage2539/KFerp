import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { ref, watch, nextTick } from 'vue'
import { resolveSpecPricingGroup } from './spec-pricing-group.js'
import { priceListParentProductID, priceListSkuID, togglePriceListProductSpecSelection } from './product-price-list-selection.js'

const source = readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
const start = source.indexOf('watch(() => JSON.stringify([namedEditorBlocked.value, pdfProductSpecSelections.value')
const end = source.indexOf('\nfunction togglePdfProduct(', start)
assert.ok(start >= 0 && end > start, 'group selection watcher exists')
const watcher = source.slice(start, end)

function mountWatcher() {
  const specs = [11, 12, 13, 14].map((id, i) => ({ sku_id:id, bom_spec_id:id, bom_variant_id:100+id,
    source_spec_template_id: 7, source_spec_template_key: `s${i+1}`, inventory_unit:'袋' }))
  const family = { parent_product_id:42, default_sku_id:11, sku_options:specs }
  const refs = {
    namedEditorBlocked: ref(true), pdfProductSpecSelections: ref([{parent_product_id:42, sku_id:11}]),
    specPricingGroups: ref([{ id:1, active:true, spec_template_id:7, items:specs.slice(0,3).map(s => ({spec_key:s.source_spec_template_key, inventory_unit:'袋', pricing_rule_id:s.sku_id})) }]),
    pdfAvailableItems: ref([family]), priceListTemplateDefaults:ref({}), priceListParentTemplateSelections:ref({}),
    priceListGroupTemplateSelections:ref({}), priceListProductTemplateOverrides:ref({}),
  }
  const bindings = { ...refs, watch, resolveSpecPricingGroup, priceListParentProductID, priceListSkuID,
    togglePriceListProductSpecSelection, priceListResolvedTemplateForItem: () => ({ pricing_mode:'spec_pricing_group', spec_pricing_group_id:1 }),
    updatePdfProductSpecSelections: (value) => { refs.pdfProductSpecSelections.value = value },
  }
  new Function(...Object.keys(bindings), watcher)(...Object.values(bindings))
  return { ...refs, family }
}

test('组选择随编辑器加载完成自动扩展规格，取消商品不会被重新选中', async () => {
  const state = mountWatcher()
  state.namedEditorBlocked.value = false
  await nextTick(); await nextTick()
  assert.deepEqual(state.pdfProductSpecSelections.value.map(row => row.sku_id), [11,12,13])
  state.pdfProductSpecSelections.value = []
  await nextTick(); await nextTick()
  assert.equal(state.pdfProductSpecSelections.value.length, 0)
})

test('商品缺组规格时保留原商品选择而不静默丢弃', async () => {
  const state = mountWatcher()
  state.family.sku_options = state.family.sku_options.slice(0, 2)
  state.namedEditorBlocked.value = false
  await nextTick(); await nextTick()
  assert.deepEqual(state.pdfProductSpecSelections.value.map(row => row.sku_id), [11])
})

test('价格表展开三个规格分别使用组内对应模板', () => {
  const start = source.indexOf('function priceListFlatRowsFromGroups(')
  const end = source.indexOf('\nfunction priceListFlatRowFromSource(', start)
  assert.ok(start >= 0 && end > start)
  const group = { id: 1, revision: 3, active: true, spec_template_id: 7, items: [
    { spec_key: 's1', inventory_unit:'袋', pricing_rule_id:101 },
    { spec_key: 's2', inventory_unit:'袋', pricing_rule_id:102 },
    { spec_key: 's3', inventory_unit:'袋', pricing_rule_id:103 },
  ] }
  const specs = [11,12,13].map((id, i) => ({ product_id:42, parent_product_id:42, sku_id:0, bom_spec_id:id, bom_variant_id:100+id, source_spec_template_id:7, source_spec_template_key:`s${i+1}`, inventory_unit:'袋' }))
  const family = { parent_product_id:42, sku_options:specs }
  const bindings = {
    tierKeyForListType: () => 'tiers', pdfTheme:ref({listType:'commercial'}),
    itemSkuID:(item, fallback) => item.sku_id || fallback, itemProductID:item => item.product_id,
    itemParentProductID:item => item.parent_product_id, priceListParentProductID,
    priceListGroupForItem:() => ({}), resolvePriceTableTemplateInheritance:() => ({pricing_mode:'spec_pricing_group', spec_pricing_group_id:1, pricing_mode_source:'subgroup'}),
    priceListTemplateDefaults:ref({}), priceListTemplateAssignments:() => [], priceListProductOverridesForSnapshot:() => [],
    specPricingGroups:ref([group]), pdfAvailableItems:ref([family]), resolveSpecPricingGroup, priceListSkuID,
    priceListFlatRowFromSource: args => ({ pricing_rule_id: args.pricingRuleID, bom_spec_id: args.item.bom_spec_id, row_key:args.rowKey, max_qty:args.maxQty }),
    pricingRuleByID:id => ({id}),
  }
  const fn = new Function(...Object.keys(bindings), source.slice(start,end) + '\nreturn priceListFlatRowsFromGroups')(...Object.values(bindings))
  const rows = fn([{items:specs}])
  assert.deepEqual(rows.map(row => [row.bom_spec_id,row.pricing_rule_id,row.spec_pricing_group_id,row.spec_pricing_group_revision]), [[11,101,1,3],[12,102,1,3],[13,103,1,3]])
  assert.ok(rows.every(row => row.max_qty === null))
})
