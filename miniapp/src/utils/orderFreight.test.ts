import {describe,it,expect} from 'vitest'
import {emptyFreightState,manualFreightCents,freightAmount,freightOverride,freightStateFromOrder,regionDestination,searchOrderRegions} from './orderFreight'
describe('employee freight state',()=>{
 it('preserves blank, explicit zero and free through a serialized draft',()=>{
  const state=emptyFreightState();expect(manualFreightCents(state.manual)).toBeNull();state.manual='20';expect(freightAmount(state)).toBe(20);state.free=true
  const draft=JSON.parse(JSON.stringify(state));expect(freightAmount(draft)).toBe(0);draft.free=false;expect(freightAmount(draft)).toBe(20)
  state.manual='0';state.free=false;expect(freightOverride(state)).toEqual({manual_amount_cents:0,free_shipping:false})
  for(const invalid of ['-1','NaN','1.001','999999999999999999999'])expect(()=>manualFreightCents(invalid)).toThrow()
 })
 it('copies the fee choice independently from actual bills and old tokens',()=>{
  const state=freightStateFromOrder({shipping_amount:'20',freight:{manual_amount_cents:2000,policy:{free_shipping:false}}});expect(state.manual).toBe('20.00');expect(state).not.toHaveProperty('token');expect(state).not.toHaveProperty('actual_cost_cents')
 })
 it('selects a county by code with its parent hierarchy',()=>{
  const rows=[{code:'530000',parent_code:'',name:'云南省',level:1},{code:'530800',parent_code:'530000',name:'普洱市',level:2},{code:'530827',parent_code:'530800',name:'孟连傣族拉祜族佤族自治县',alias:'孟连县',level:3,path:'云南省 / 普洱市 / 孟连傣族拉祜族佤族自治县'}]
  const hit=searchOrderRegions(rows,'孟连县')[0];expect(regionDestination(rows,hit.code)).toMatchObject({province:'云南省',city:'普洱市',district_code:'530827'})
 })
})
