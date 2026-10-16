import { describe, it, expect } from 'vitest'
import { applyConversionQuote, conversionSnapshot, conversionTargets } from './specConversion'
import { createEmployeeOrderItem, applyEmployeeOrderQuantityChange, buildEmployeeOrderItemsPayload, hydrateEmployeeOrderEditItems, copyEmployeeOrderItems, revalidateEmployeeOrderItems, restoreEmployeeOrderAutomaticPrice } from './employeeOrder'
const quote: any = { product_id:7,publication_id:8,group_id:1,revision:2,source_bom_spec_id:11,source_bom_variant_id:21,target_bom_spec_id:12,target_bom_variant_id:22,price_row_key:'row',target_spec_name:'500g',target_unit:'袋',unit_price:'52.86',price_source_json:JSON.stringify({quantity_basis:'sales_spec_count',price_selection_mode:'converted_tier',selected_price_row_key:'row',spec_conversion:{group_id:1,revision:2,source_bom_spec_id:11,source_bom_variant_id:21,target_bom_spec_id:12,target_bom_variant_id:22,target_spec_name:'500g',target_unit:'袋',unit_price:'52.86',product_id:7,publication_id:8,price_row_key:'row'}}) }
describe('temporary specification pricing', () => {
 it('keeps target identity and exact tier when quantity changes without current catalog', () => {
  const row = applyConversionQuote({...createEmployeeOrderItem(),product_name:'Coffee',qty:2}, quote)
  const changed = applyEmployeeOrderQuantityChange(row, undefined, 3)
  expect(changed.accepted).toBe(true); expect(changed.item.unit_price).toBe(52.86)
  const payload = buildEmployeeOrderItemsPayload([changed.item])[0]
  expect(payload.bom_spec_id).toBe(12);expect(payload.price_selection_mode).toBe('converted_tier');expect(payload.selected_price_row_key).toBe('row');expect(payload.spec_conversion?.source_bom_spec_id).toBe(11)
 })
 it('hydrates frozen conversion outside current table and marks copies for repricing', () => {
  const detail:any={item_id:3,product_id:7,bom_spec_id:12,bom_variant_id:22,qty:2,unit_price:52.86,price_source_json:quote.price_source_json,price_selection_mode:'converted_tier'}
  const row=hydrateEmployeeOrderEditItems([detail],[],3)[0]
  expect(row.validation_error).toBe('');expect(row.spec_conversion?.reprice).toBe(false)
  expect(revalidateEmployeeOrderItems([row],[],3)[0].unit_price).toBe(52.86)
  const copied=copyEmployeeOrderItems([detail],[],3)[0];expect(copied.item_id).toBe(0);expect(copied.spec_conversion?.reprice).toBe(true);expect(copied.validation_error).toContain('报价')
  expect(restoreEmployeeOrderAutomaticPrice(row).validation_error).toBeTruthy()
 })
 it('matches configured identity instead of name',()=>{
  expect(conversionTargets({members:[{bom_spec_id:12,inventory_unit:'袋'}]} as any,[{bom_spec_id:13,inventory_unit:'袋',spec_name:'500g'}] as any)).toEqual([])
  expect(conversionSnapshot({price_source_json:'bad'})).toBeNull()
 })
})
