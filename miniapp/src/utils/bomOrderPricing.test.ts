import assert from 'node:assert/strict'
import { describe,it,expect } from 'vitest'
import { orderedBOMSpecs, selectBOMSpec, resolveBOMTier } from './bomOrderPricing'
import { applyConversionQuote } from './specConversion'
import { createEmployeeOrderItem, applyEmployeeOrderQuantityChange, buildEmployeeOrderItemsPayload, hydrateEmployeeOrderEditItems, copyEmployeeOrderItems } from './employeeOrder'
describe('BOM order pricing',()=>{
 it('defaults to the actual BOM default even when unpriced',()=>{expect(orderedBOMSpecs([{bom_spec_id:1,sort_order:1},{bom_spec_id:2,is_default:true,sort_order:9}] as any).map(s=>s.bom_spec_id)).toEqual([2,1])})
 it('clears source tier when switching target specs',()=>{
 const row:any={...createEmployeeOrderItem(),product_id:7,qty:3,price_selection_mode:'bom_tier',selected_price_row_key:'second',spec_conversion:{pricing_method:'bom_rule',source_bom_spec_id:1,source_bom_variant_id:11,quote_revision:'old'},bom_catalog:{source_tiers:[{price_row_key:'second',bom_spec_id:1,bom_variant_id:11}],specs:[]}}
 const changed=selectBOMSpec(row,{bom_spec_id:2,bom_variant_id:12,spec_name:'500g',inventory_unit:'袋'} as any)
 expect(changed.qty).toBe(3);expect(changed.selected_price_row_key).toBe('');expect(changed.spec_conversion).toBeUndefined();expect(changed.spec_conversion?.quote_revision).toBeUndefined();expect(changed.unit_price).toBe(0)
 })
 it('quantity preserves frozen BOM price and snapshot; copies must reprice',()=>{
 const q:any={product_id:7,publication_id:8,pricing_method:'bom_rule',quote_revision:'hash',group_id:0,revision:0,source_bom_spec_id:1,source_bom_variant_id:11,target_bom_spec_id:2,target_bom_variant_id:12,price_row_key:'second',target_spec_name:'500g',target_unit:'袋',unit_price:'52.86'}
 q.price_source_json=JSON.stringify({quantity_basis:'sales_spec_count',price_selection_mode:'bom_tier',selected_price_row_key:'second',spec_conversion:q})
 const row=applyConversionQuote({...createEmployeeOrderItem(),product_name:'Coffee',qty:2},q)
 const changed=applyEmployeeOrderQuantityChange(row,undefined,100)
 expect(changed.accepted).toBe(true);expect(changed.item.unit_price).toBe(52.86)
 expect(buildEmployeeOrderItemsPayload([changed.item])[0].price_selection_mode).toBe('bom_tier')
 const detail:any={...row,item_id:3,price_source_json:q.price_source_json}
 expect(hydrateEmployeeOrderEditItems([detail],[],3)[0].spec_conversion?.reprice).toBe(false)
 expect(copyEmployeeOrderItems([detail],[],3)[0].spec_conversion?.reprice).toBe(true)
 })
})

const test = it

const spec = { bom_spec_id: 1, bom_variant_id: 11, is_default: true }
const tier = (key:string, min:number|null, max:number|null, extra = {}) => ({ product_id:7, bom_spec_id:1, bom_variant_id:11, price_row_key:key, min_qty:min, max_qty:max, quantity_basis:'sales_spec_count', ...extra })
const catalog:any = { specs:[spec,{bom_spec_id:2,bom_variant_id:12}], source_tiers:[tier('low',2,13),tier('high',14,null)] }
test('default specification matches inclusive quantity boundaries and open upper limit',()=>{
 for(const [qty,key] of [[2,'low'],[13,'low'],[14,'high'],[100,'high']]){
  const result=resolveBOMTier(catalog,1,Number(qty))
  assert.equal(result.mode,'auto');assert.equal(result.tier!.price_row_key,key)
 }
 for(const qty of [0,1,-1,NaN]) assert.equal(resolveBOMTier(catalog,1,Number(qty)).tier,undefined)
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
 assert.equal(resolveBOMTier(catalog,1,1,'high','manual').tier!.price_row_key,'high')
 assert.equal(resolveBOMTier(catalog,1,1,'high','auto').tier,undefined)
 assert.equal(resolveBOMTier(catalog,1,5,'high','manual').tier!.price_row_key,'low')
 assert.equal(resolveBOMTier(catalog,1,1,'deleted','manual').tier,undefined)
})
test('non-default tier stays manual across quantities; an explicit spec change starts empty',()=>{
 for(const qty of [1,100]){
  assert.equal(resolveBOMTier(catalog,2,qty,'low','manual').tier!.price_row_key,'low')
  assert.equal(resolveBOMTier(catalog,2,qty).tier,undefined)
 }
})

it('copy and reprice recompute the default and retain only valid nondefault manual selections',()=>{
 const item:any={product_id:7,qty:100,selected_price_row_key:'low',bom_catalog:catalog}
 expect(selectBOMSpec(item,spec as any,true).selected_price_row_key).toBe('high')
 item.qty=1
 expect(selectBOMSpec(item,spec as any,true).selected_price_row_key).toBe('')
 expect(selectBOMSpec(item,catalog.specs[1],true).selected_price_row_key).toBe('low')
 item.selected_price_row_key='expired'
 expect(selectBOMSpec(item,catalog.specs[1],true).selected_price_row_key).toBe('')
})
