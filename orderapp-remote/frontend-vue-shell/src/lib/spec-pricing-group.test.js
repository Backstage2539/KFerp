import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveSpecPricingGroup } from './spec-pricing-group.js'

test('matches group entries by template, stable spec key and unit', () => {
  const group = { active: true, spec_template_id: 8, items: [
    { spec_key: 's1', inventory_unit: '袋', pricing_rule_id: 10 },
    { spec_key: 's2', inventory_unit: '袋', pricing_rule_id: 11 },
  ] }
  const specs = [
    { source_spec_template_id: 8, source_spec_template_key: 's1', inventory_unit: '袋', bom_spec_id: 31 },
    { source_spec_template_id: 8, source_spec_template_key: 's2', inventory_unit: '袋', bom_spec_id: 32 },
    { source_spec_template_id: 9, source_spec_template_key: 's1', inventory_unit: '袋', bom_spec_id: 99 },
  ]
  const result = resolveSpecPricingGroup(group, specs)
  assert.deepEqual(result.items.map(({ spec, item }) => [spec.bom_spec_id, item.pricing_rule_id]), [[31, 10], [32, 11]])
  assert.deepEqual(result.errors, [])
})

test('missing, ambiguous, inactive or different-template specs never silently match', () => {
  const group = { active: true, spec_template_id: 8, items: [{ spec_key: 's1', inventory_unit: '袋', pricing_rule_id: 10 }] }
  assert.ok(resolveSpecPricingGroup(group, [{ source_spec_template_id: 9, source_spec_template_key: 's1', inventory_unit: '袋' }]).errors.length)
  assert.ok(resolveSpecPricingGroup(group, [{ source_spec_template_id: 8, source_spec_template_key: 's1', inventory_unit: '盒' }]).errors.length)
  const same = { source_spec_template_id: 8, source_spec_template_key: 's1', inventory_unit: '袋' }
  assert.ok(resolveSpecPricingGroup(group, [same, same]).errors.length)
  assert.ok(resolveSpecPricingGroup({ ...group, active: false }, [same]).errors.length)
})

import { resolvePriceTableTemplateInheritance, priceTablePricingResolutionWarning } from './product-settings.js'
import { normalizeParentSharedPriceListProductOverrides } from './product-price-list-draft.js'

test('nearest classification or product selects entire independent pricing group', () => {
  const base = { defaults: { pricing_mode: 'spec_pricing_group', spec_pricing_group_id: 1 }, groupAssignments: [
    { group_item_id: 10, pricing_mode: 'spec_pricing_group', spec_pricing_group_id: 2 },
    { group_item_id: 11, parent_group_item_id: 10, pricing_mode: 'spec_pricing_group', spec_pricing_group_id: 3 },
  ], product: { parent_product_id: 42, group_item_id: 11 } }
  assert.equal(resolvePriceTableTemplateInheritance(base).spec_pricing_group_id, 3)
  assert.equal(resolvePriceTableTemplateInheritance({ ...base, productOverrides: [{ scope:'parent_product', product_id:42, parent_product_id:42, pricing_mode:'spec_pricing_group', spec_pricing_group_id:4 }] }).spec_pricing_group_id, 4)
  assert.match(priceTablePricingResolutionWarning({ pricing_mode: 'spec_pricing_group' }), /计价组/)
})

test('shared-parent draft normalization retains pricing group', () => {
  const out = normalizeParentSharedPriceListProductOverrides({ 'parent:42': { scope:'parent_product', product_id:42, parent_product_id:42, pricing_mode:'spec_pricing_group', spec_pricing_group_id:9 } })
  assert.equal(out.overrides['parent:42'].spec_pricing_group_id, 9)
})

import { buildPriceListGenerationSnapshot } from './bean-list-pdf.js'
test('publication snapshot retains group identity and revision', () => {
  const snapshot = buildPriceListGenerationSnapshot({ defaults: { pricing_mode: 'spec_pricing_group', spec_pricing_group_id: 9 }, rows:[{ product_id:1, pricing_mode:'pricing_rule', spec_pricing_group_id:9, spec_pricing_group_revision:2, pricing_rule_id:4 }] })
  assert.equal(snapshot.config.price_list_template_selection.defaults.spec_pricing_group_id,9)
  assert.equal(snapshot.content.price_rows[0].spec_pricing_group_revision,2)
})

import { applyPricingRuleTrialToPriceTableRow } from './product-settings.js'
test('group quote freezes the formula fingerprint returned by the pricing trial', () => {
 const row={product_id:1,pricing_mode:'pricing_rule',pricing_rule_id:4,spec_pricing_group_id:9,bom_spec_id:10,bom_variant_id:11,quantity_basis:'sales_spec_count'}
 const trial={product_id:1,pricing_rule_id:4,bom_spec_id:10,bom_variant_id:11,final_unit_price:12,pricing_rule_fingerprint:'sha256:test'}
 const result=applyPricingRuleTrialToPriceTableRow(row,trial)
 assert.equal(result.pricing_rule_fingerprint,'sha256:test')
 const snapshot=buildPriceListGenerationSnapshot({rows:[result]})
 assert.equal(snapshot.content.price_rows[0].pricing_rule_fingerprint,'sha256:test')
})

test('group with a disabled template is not available for calculation', () => {
 const group={active:true,template_available:false,spec_template_id:8,items:[{spec_key:'s1',inventory_unit:'袋',pricing_rule_id:10}]}
 const spec={source_spec_template_id:8,source_spec_template_key:'s1',inventory_unit:'袋'}
 assert.ok(resolveSpecPricingGroup(group,[spec]).errors.length)
})
