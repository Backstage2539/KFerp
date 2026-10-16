import { describe,it,expect } from 'vitest'
import { orderedBOMSpecs, selectBOMSpec } from './bomOrderPricing'
import { applyConversionQuote } from './specConversion'
import { createEmployeeOrderItem, applyEmployeeOrderQuantityChange, buildEmployeeOrderItemsPayload, hydrateEmployeeOrderEditItems, copyEmployeeOrderItems } from './employeeOrder'
describe('BOM order pricing',()=>{
 it('defaults to the actual BOM default even when unpriced',()=>{expect(orderedBOMSpecs([{bom_spec_id:1,sort_order:1},{bom_spec_id:2,is_default:true,sort_order:9}] as any).map(s=>s.bom_spec_id)).toEqual([2,1])})
 it('preserves source tier when switching target specs',()=>{
 const row:any={...createEmployeeOrderItem(),product_id:7,qty:3,price_selection_mode:'bom_tier',selected_price_row_key:'second',spec_conversion:{pricing_method:'bom_rule',source_bom_spec_id:1,source_bom_variant_id:11,quote_revision:'old'},bom_catalog:{source_tiers:[{price_row_key:'second',bom_spec_id:1,bom_variant_id:11}],specs:[]}}
 const changed=selectBOMSpec(row,{bom_spec_id:2,bom_variant_id:12,spec_name:'500g',inventory_unit:'袋'} as any,true)
 expect(changed.qty).toBe(3);expect(changed.selected_price_row_key).toBe('second');expect(changed.spec_conversion?.source_bom_spec_id).toBe(1);expect(changed.spec_conversion?.quote_revision).toBeUndefined();expect(changed.unit_price).toBe(0)
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
