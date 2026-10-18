import test from 'node:test'
import assert from 'node:assert/strict'
import { createOrderDetailsCopyAction } from './order-details-copy.js'
const deferred=()=>{let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve}}
const payload={order_id:42,text:'订单号：SO-42\n付款状态：已付款'}
test('copy waits for fresh details and clipboard success; concurrent clicks are suppressed',async()=>{
 const request=deferred(),clipboard=deferred(),writes=[];let reads=0
 const action=createOrderDetailsCopyAction({context:()=> 'A',fetchDetails:()=>{reads++;return request.promise},writeText:text=>{writes.push(text);return clipboard.promise}})
 const first=action.copy(42);assert.equal(action.busy,true);assert.equal((await action.copy(42)).status,'busy');assert.deepEqual(writes,[])
 request.resolve(payload);await Promise.resolve();assert.deepEqual(writes,[payload.text]);assert.equal(action.busy,true)
 clipboard.resolve();assert.equal((await first).status,'copied');assert.equal(action.busy,false);assert.equal(reads,1)
})
test('fetch failure never writes; clipboard failure returns selectable text',async()=>{
 let writes=0;const bad=createOrderDetailsCopyAction({context:()=>'',fetchDetails:async()=>{throw Error('读取失败')},writeText:async()=>{writes++}})
 assert.equal((await bad.copy(42)).status,'error');assert.equal(writes,0)
 const denied=createOrderDetailsCopyAction({context:()=>'',fetchDetails:async()=>payload,writeText:async()=>{throw Error('denied')}})
 assert.deepEqual(await denied.copy(42),{status:'manual',text:payload.text});assert.equal(denied.busy,false)
})
test('customer switch, logout and disposal discard pending data; wrong order is rejected',async()=>{
 for(const mode of ['switch','logout','dispose']){let context='token/customerA';const d=deferred(),writes=[];const action=createOrderDetailsCopyAction({context:()=>context,fetchDetails:()=>d.promise,writeText:async text=>writes.push(text)});const p=action.copy(42);if(mode==='dispose')action.invalidate();else context=mode==='switch'?'token/customerB':'';d.resolve(payload);assert.equal((await p).status,'cancelled');assert.deepEqual(writes,[])}
 const action=createOrderDetailsCopyAction({context:()=>'',fetchDetails:async()=>({...payload,order_id:99}),writeText:async()=>{throw Error('should not write')}})
 assert.equal((await action.copy(42)).status,'error')
})
