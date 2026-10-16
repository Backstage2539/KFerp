import {describe,it,expect} from 'vitest'
import {sampleTotal, sampleStatus, confirmRows, canConfirmSample, validSampleMoney} from './samples'
import {canUseEmployeePriceCenter} from './priceCenterAccess'
describe('sample work order interaction',()=>{
 it('accepts explicit zero but rejects blank, excessive precision and invalid money',()=>{for(const value of [0,'0','0.00',1.25])expect(validSampleMoney(value)).toBe(true);for(const value of ['',null,undefined,-1,NaN,Infinity,'1.005','word'])expect(validSampleMoney(value)).toBe(false)})
 it('keeps free samples and sums money in cents',()=>{expect(sampleTotal([{qty:2,unit_price:0}],0)).toBe(0);expect(sampleTotal([{qty:3,unit_price:0.1}],0.2)).toBe(0.5)})
 it('does not invent a missing BOM specification',()=>{expect(confirmRows([{product_id:1,name:'豆'}],[])[0].bom_spec_id).toBe(0)})
 it('binds each specification to its own product',()=>{const rows=confirmRows([{product_id:1,name:'豆一'},{product_id:2,name:'豆二'}],[{id:8,product_id:2,name:'100g',unit:'袋'}]);expect(rows.map(r=>r.bom_spec_id)).toEqual([0,8]);expect(rows.every(r=>r.qty===1&&r.unit_price===0)).toBe(true)})
 it('requires assignment and no previous order or terminal followup',()=>{expect(canConfirmSample({work_order_id:1,order_id:0,followup_status:'pending'})).toBe(true);expect(canConfirmSample({work_order_id:1,order_id:2,followup_status:'following'})).toBe(false);expect(canConfirmSample({work_order_id:1,order_id:0,followup_status:'abandoned'})).toBe(false)})
 it('uses role scope for staff access and readable states',()=>{expect(canUseEmployeePriceCenter({accountType:'employee',roles:['sales']})).toBe(true);expect(canUseEmployeePriceCenter({accountType:'employee',roles:['warehouse']})).toBe(false);expect(sampleStatus('pending_contact')).toBe('收到工单 · 待联系')})
})
