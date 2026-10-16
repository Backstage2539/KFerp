import test from 'node:test'
import assert from 'node:assert/strict'
import { orderedBOMSpecs, bomTierSelection, bomQuoteIsCurrent } from './bom-order-pricing.js'
test('all published BOM specs survive without table prices, default first without mutation',()=>{
 const specs=[{bom_spec_id:1,sort_order:1},{bom_spec_id:2,is_default:true,sort_order:9},{bom_spec_id:3,sort_order:2}]
 assert.deepEqual(orderedBOMSpecs(specs).map(s=>s.bom_spec_id),[2,1,3]);assert.equal(specs[0].bom_spec_id,1)
})
test('manual source tier is retained across specs and quantity; never picks a tier automatically',()=>{
 const tier={bom_spec_id:1,bom_variant_id:11,price_row_key:'second'}
 assert.equal(bomTierSelection(null).selected_price_row_key,'')
 const chosen=bomTierSelection(tier);assert.equal(chosen.spec_conversion.source_bom_spec_id,1);assert.equal(chosen.selected_price_row_key,'second')
 assert.equal(chosen.price_selection_mode,'bom_tier')
})
test('late quotes cannot cross customer, target, source or publication',()=>{
 const context={customer:3,product:7,spec:2,variant:12,publication:88,key:'second'}
 assert.equal(bomQuoteIsCurrent(context,{...context}),true)
 for(const key of Object.keys(context))assert.equal(bomQuoteIsCurrent(context,{...context,[key]:'changed'}),false)
})
