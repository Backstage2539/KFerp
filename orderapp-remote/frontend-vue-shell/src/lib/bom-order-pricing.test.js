import test from 'node:test'
import assert from 'node:assert/strict'
import { orderedBOMSpecs, bomTierSelection, bomQuoteIsCurrent, resolveBOMTier } from './bom-order-pricing.js'
test('all published BOM specs survive without table prices, default first without mutation',()=>{
 const specs=[{bom_spec_id:1,sort_order:1},{bom_spec_id:2,is_default:true,sort_order:9},{bom_spec_id:3,sort_order:2}]
 assert.deepEqual(orderedBOMSpecs(specs).map(s=>s.bom_spec_id),[2,1,3]);assert.equal(specs[0].bom_spec_id,1)
})
test('explicit manual tier selection records source identity without guessing',()=>{
 const tier={bom_spec_id:1,bom_variant_id:11,price_row_key:'second'}
 assert.equal(bomTierSelection(null).selected_price_row_key,'')
 const chosen=bomTierSelection(tier);assert.equal(chosen.spec_conversion.source_bom_spec_id,1);assert.equal(chosen.selected_price_row_key,'second')
 assert.equal(chosen.price_selection_mode,'bom_tier')
})
test('late quotes cannot cross customer, target, source or publication',()=>{
 const context={customer:3,product:7,spec:2,variant:12,publication:88,key:'second',quantity:2}
 assert.equal(bomQuoteIsCurrent(context,{...context}),true)
 for(const key of Object.keys(context))assert.equal(bomQuoteIsCurrent(context,{...context,[key]:'changed'}),false)
})

const spec = { bom_spec_id: 1, bom_variant_id: 11, is_default: true }
const tier = (key, min, max, extra = {}) => ({ product_id:7, bom_spec_id:1, bom_variant_id:11, price_row_key:key, min_qty:min, max_qty:max, quantity_basis:'sales_spec_count', ...extra })
const catalog = { specs:[spec,{bom_spec_id:2,bom_variant_id:12}], source_tiers:[tier('low',2,13),tier('high',14,null)] }
test('default specification matches inclusive quantity boundaries and open upper limit',()=>{
 for(const [qty,key] of [[2,'low'],[13,'low'],[14,'high'],[100,'high']]){
  const result=resolveBOMTier(catalog,1,qty)
  assert.equal(result.mode,'auto');assert.equal(result.tier.price_row_key,key)
 }
 for(const qty of [0,1,-1,NaN]) assert.equal(resolveBOMTier(catalog,1,qty).tier,undefined)
})
test('default fallback never guesses by name, label, invalid ranges or ambiguous tiers',()=>{
 const other={...catalog,source_tiers:[tier('other',1,null,{bom_spec_id:2,spec_name:'default'})]}
 assert.equal(resolveBOMTier(other,1,4).mode,'fallback')
 for(const tiers of [[tier('a',1,8),tier('b',4,9)],[tier('a',null,9)],[tier('a',1,9,{quantity_basis:''})],[tier('a',9,2)]]){
  const result=resolveBOMTier({...catalog,source_tiers:tiers},1,4)
  assert.equal(result.mode,'fallback');assert.equal(result.tier,undefined);assert.ok(result.reason)
 }
})
test('manual fallback survives no-match quantity changes but a match restores automatic selection',()=>{
 assert.equal(resolveBOMTier(catalog,1,1,'high','manual').tier.price_row_key,'high')
 assert.equal(resolveBOMTier(catalog,1,1,'high','auto').tier,undefined)
 assert.equal(resolveBOMTier(catalog,1,5,'high','manual').tier.price_row_key,'low')
 assert.equal(resolveBOMTier(catalog,1,1,'deleted','manual').tier,undefined)
})
test('non-default tier stays manual across quantities; an explicit spec change starts empty',()=>{
 for(const qty of [1,100]){
  assert.equal(resolveBOMTier(catalog,2,qty,'low','manual').tier.price_row_key,'low')
  assert.equal(resolveBOMTier(catalog,2,qty).tier,undefined)
 }
})
