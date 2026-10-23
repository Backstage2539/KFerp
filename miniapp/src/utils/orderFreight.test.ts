import {describe,it,expect} from 'vitest'
import {emptyFreightState,manualFreightCents,freightAmount,freightOverride,freightStateFromOrder,missingWeightItemNames,regionDestination,searchOrderRegions} from './orderFreight'
describe('employee freight state',()=>{
 it('preserves blank, explicit zero and free through a serialized draft',()=>{
  const state=emptyFreightState();expect(manualFreightCents(state.manual)).toBeNull();state.manual='20';expect(freightAmount(state)).toBe(20);state.free=true
  const draft=JSON.parse(JSON.stringify(state));expect(freightAmount(draft)).toBe(0);draft.free=false;expect(freightAmount(draft)).toBe(20)
  state.manual='0';state.free=false;expect(freightOverride(state)).toEqual({charge_mode:'manual_current',manual_amount_cents:0,free_shipping:false})
  for(const invalid of ['-1','NaN','1.001','999999999999999999999'])expect(()=>manualFreightCents(invalid)).toThrow()
 })
 it('copies the fee choice independently from actual bills and old tokens',()=>{
  const state=freightStateFromOrder({shipping_amount:'20',freight:{manual_amount_cents:2000,policy:{free_shipping:false}}});expect(state.manual).toBe('20.00');expect(state).not.toHaveProperty('token');expect(state).not.toHaveProperty('actual_cost_cents')
 })
 it('preserves an existing order freight until explicit repricing',()=>{
  const state=freightStateFromOrder({id:42,shipping_amount:'10.00',freight:{charge_mode:'deferred',policy:{free_shipping:false}}});expect(state.preserveExisting).toBe(true);expect(freightAmount(state)).toBe(10);expect(freightOverride(state)).toEqual({})
 })
 it('uses accepted automatic quotes, selected templates and explicit gross parcel weight',()=>{
  const state=emptyFreightState();state.autoAmountCents=1250;expect(freightAmount(state)).toBe(12.5);expect(freightOverride(state).charge_mode).toBe('auto_current')
  state.templateId=8;state.templateOverride=true;state.parcelWeightG='2300';expect(freightOverride(state)).toMatchObject({charge_mode:'auto_current',template_id:8,parcel_weight_g:2300})
  state.parcelWeightG='2.3';expect(()=>freightOverride(state)).toThrow(/包裹总重量/)
 })
 it('names each item missing a logistics weight for the parcel-weight prompt',()=>{
  expect(missingWeightItemNames([{product_id:1,bom_spec_id:2,name:'测试生豆'}],{estimate_known:false,item_weights:[{product_id:1,bom_spec_id:2,logistics_unit_weight_g:null}]})).toEqual(['测试生豆'])
  expect(missingWeightItemNames([{product_id:1,bom_spec_id:2,name:'测试生豆'}],{estimate_known:true,item_weights:[]})).toEqual([])
 })
 it('selects a county by code with its parent hierarchy',()=>{
  const rows=[{code:'530000',parent_code:'',name:'云南省',level:1},{code:'530800',parent_code:'530000',name:'普洱市',level:2},{code:'530827',parent_code:'530800',name:'孟连傣族拉祜族佤族自治县',alias:'孟连县',level:3,path:'云南省 / 普洱市 / 孟连傣族拉祜族佤族自治县'}]
  const hit=searchOrderRegions(rows,'孟连县')[0];expect(regionDestination(rows,hit.code)).toMatchObject({province:'云南省',city:'普洱市',district_code:'530827'})
 })
})
