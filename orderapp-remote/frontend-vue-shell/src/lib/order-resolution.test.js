import test from 'node:test'
import assert from 'node:assert/strict'
import { blankResolution, resolutionPayload, resolutionStatus } from './order-resolution.js'
test('new request never selects consequential dispositions',()=>{const c=blankResolution();for(const key of ['action','reason','scope','timing','external_status','posting_mode','posting_date'])assert.equal(c[key],'');assert.deepEqual(c.dispositions,[])})
test('selected quantities and dispositions stay explicit',()=>{const c=blankResolution();c.scope='selected';c.lines=[{item_id:1,quantity:2},{item_id:2,quantity:0}];c.dispositions=[{kind:'production',id:19,choice:''}];const body=resolutionPayload(c,'v2','key');assert.deepEqual(body.lines,[{item_id:1,quantity:2}]);assert.equal(body.dispositions[0].choice,'');assert.equal(body.version,'v2');assert.equal(body.idempotency_key,'key')})
test('processing status is independent of fulfilment status',()=>{assert.equal(resolutionStatus('closed',false),'已关闭');assert.equal(resolutionStatus('partially_closed',true),'已暂停 · 部分关闭')})

test('pending tasks start without money, account or disposition defaults', async()=>{
 const {blankResolutionStep,resolutionStepPayload}=await import('./order-resolution.js')
 const c=blankResolutionStep();assert.equal(c.choice,'');assert.equal(c.amount,'');assert.equal(c.account,'');assert.equal(c.posting_date,'')
 assert.throws(()=>resolutionStepPayload({...c,choice:'refund',amount:'1.001'},'retry-key'),/两位/)
 assert.deepEqual(resolutionStepPayload({...c,choice:'refund',amount:'12.34',account:'chosen',reference:'voucher',posting_date:'2026-11-07',note:'实际退款'},'retry-key'),{choice:'refund',amount_cents:1234,account:'chosen',reference:'voucher',posting_date:'2026-11-07',note:'实际退款',target_id:0,idempotency_key:'retry-key'})
})

test('fee negotiation distinguishes omitted amount from explicitly entered zero',()=>{
 const form=blankResolution();form.dispositions=[{kind:'fee',id:-1,choice:'negotiated',amount:''}]
 assert.throws(()=>resolutionPayload(form,'v','k'),/费用金额/)
 form.dispositions[0].amount='0'
 const body=resolutionPayload(form,'v','k');assert.equal(body.dispositions[0].amount_cents,0);assert.equal(body.dispositions[0].amount_specified,true)
 form.dispositions[0].amount='12.345';assert.throws(()=>resolutionPayload(form,'v','k'),/两位/)
})
