import {describe,it,expect} from 'vitest'
import {hydrateEmployeeOrderEditItems,revalidateEmployeeOrderItems,applyEmployeeOrderQuantityChange,buildEmployeeOrderItemsPayload,isEmployeeOrderUnitPriceValid,copyEmployeeOrderItems} from './employeeOrder'
const item={item_id:31,product_id:10,bom_spec_id:9,bom_variant_id:4,migration_state:'cutover',product_name:'样品豆',spec:'100g',qty:'2',unit:'袋',unit_price:'0.00',line_total:'0.00',price_override:true,price_selection_mode:'manual' as const}
const families=[{parent_product_id:10,customer_id:0,name:'样品豆',default_publication_id:80,default_publication_version_no:'V8',migration_state:'cutover',specs:[{product_id:10,bom_spec_id:9,bom_variant_id:4,migration_state:'cutover',spec_name:'100g',spec_label:'100g',inventory_unit:'袋',tiers:[]}]}]
describe('editing existing sample orders',()=>{
 it('preserves confirmed free BOM items even when the sample spec has no retail price tier',()=>{const rows=hydrateEmployeeOrderEditItems([item],families,5,false,true);expect(rows[0]).toMatchObject({unit_price:0,price_override:true,price_selection_mode:'manual',validation_error:''});const refreshed=revalidateEmployeeOrderItems(rows,families,5,{preserveManualPrice:true,preserveUnavailable:true,preserveSampleManualPrice:true});expect(refreshed[0]).toMatchObject({unit_price:0,price_override:true,validation_error:''});expect(buildEmployeeOrderItemsPayload(refreshed)[0]).toMatchObject({unit_price:0,price_override:true,price_selection_mode:'manual',bom_spec_id:9,bean_list_publication_id:0,bean_list_version_no:''})})
 it('keeps confirmed items for address-only edits when catalog pricing does not contain the sample spec',()=>{const rows=hydrateEmployeeOrderEditItems([item],[],5,false,true);expect(rows[0].validation_error).toBe('');const result=applyEmployeeOrderQuantityChange(rows[0],undefined,3,true);expect(result.accepted).toBe(true);expect(result.item).toMatchObject({qty:3,unit_price:0,price_override:true})})
 it('does not broaden ordinary new, copied or non-sample order zero-price rules',()=>{expect(isEmployeeOrderUnitPriceValid(0,false)).toBe(false);expect(isEmployeeOrderUnitPriceValid(0,true)).toBe(true);expect(isEmployeeOrderUnitPriceValid(-1,true)).toBe(false);expect(hydrateEmployeeOrderEditItems([item],[],5)[0].validation_error).not.toBe('');const copied=copyEmployeeOrderItems([item],families,5);expect(copied[0].item_id).toBe(0);expect(applyEmployeeOrderQuantityChange(copied[0],families[0],3).accepted).toBe(false)})
})

describe('sample historical identity during order edit',()=>{
 it.each([0,5])('preserves the saved variant and references when catalog variant is %s',catalogVariant=>{
  const saved={...item,parent_product_id:10,customer_product_alias_id:202,bom_variant_id:4,bean_list_publication_id:0,bean_list_version_no:'',price_source_json:'{"manual_adjusted":true,"sample_reference":"SP-42"}'}
  const changedCatalog=[{...families[0],customer_id:5,customer_product_alias_id:202,default_publication_id:80,default_publication_version_no:'V8',specs:[{...families[0].specs[0],bom_variant_id:catalogVariant}]}]
  const [hydrated]=hydrateEmployeeOrderEditItems([saved],changedCatalog,5,false,true)
  const [payload]=buildEmployeeOrderItemsPayload([hydrated])
  expect(payload).toMatchObject({item_id:31,product_id:10,parent_product_id:10,bom_spec_id:9,bom_variant_id:4,customer_product_alias_id:202,bean_list_publication_id:0,bean_list_version_no:'',price_source_json:saved.price_source_json,unit_price:0,price_selection_mode:'manual'})
 })
})
