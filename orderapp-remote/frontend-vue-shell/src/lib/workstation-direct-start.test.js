import test from 'node:test'
import assert from 'node:assert/strict'
import {directStart, readPendingStart} from './workstation-direct-start.js'

function storage() { const m=new Map(); return {getItem:k=>m.get(k) ?? null,setItem:(k,v)=>m.set(k,v),removeItem:k=>m.delete(k)} }
function preview(available=30) { return {version:'v1',input_qty:30,input_unit:'kg',recipe_ratio_pct:100,materials:[{key:'bean',name:'生豆',qty:30,unit:'kg',ratio_pct:100,warehouse:'wip',available_qty:available}]} }
function args(extra={}) { return {jobCardId:12,employeeId:7,storage:storage(),get:async()=>preview(),post:async()=>({ok:true}),makeID:()=> 'request-1',...extra} }

test('one click uses the frozen planned input and does not require a separate form',async()=>{
 const calls=[];const a=args({post:async(path,{body})=>{calls.push({path,body});return {ok:true}}})
 const result=await directStart(a)
 assert.equal(result.result.ok,true);assert.equal(calls.length,1);assert.equal(calls[0].body.actual_input_qty,30);assert.equal(calls[0].body.materials[0].qty,30);assert.equal(calls[0].body.expected_version,'v1');assert.equal(readPendingStart(a.storage),null)
})
test('shortage opens replenishment with exact planned start demand and makes no write',async()=>{
 let writes=0;const a=args({get:async()=>preview(29.759),post:async()=>{writes++}})
 const result=await directStart(a)
 assert.equal(writes,0);assert.equal(result.shortages[0].shortage_qty,0.241);assert.equal(result.input.actual_input_qty,30);assert.equal(readPendingStart(a.storage),null)
})
test('zero availability omitted by the API is a real shortage',async()=>{
 const p=preview();delete p.materials[0].available_qty
 const result=await directStart(args({get:async()=>p,post:async()=>{throw Error('must not write')}}))
 assert.equal(result.shortages[0].shortage_qty,30)
})
test('a blocked browser store never submits an unrepeatable start',async()=>{
 let writes=0;const bad={getItem:()=>null,setItem:()=>{throw Error('quota')},removeItem:()=>{}}
 await assert.rejects(directStart(args({storage:bad,post:async()=>{writes++}})),/无法保存开工重试记录/)
 assert.equal(writes,0)
})
test('unknown outcome retries the identical payload, including after reload',async()=>{
 const calls=[];let fail=true;const a=args({post:async(_path,{body})=>{calls.push(structuredClone(body));if(fail){fail=false;throw Object.assign(Error('lost'),{status:400})}return {ok:true}}})
 await assert.rejects(directStart(a),/尚未收到开工结果/)
 assert.equal(readPendingStart(a.storage).payload.request_id,'request-1')
 await directStart({...a,get:async()=>{throw Error('retry must not read a new preview')}})
 assert.deepEqual(calls[0],calls[1]);assert.equal(readPendingStart(a.storage),null)
})
test('a stale retry button cannot accidentally start a new operation',async()=>{
 let writes=0;const a=args({post:async()=>{writes++}})
 await assert.rejects(directStart({...a,requirePending:true}),/上次开工记录已变化/);assert.equal(writes,0)
})
test('a confirmed rollback clears the retry and changed employee cannot replay',async()=>{
 const a=args({post:async()=>{throw Object.assign(Error('库存变化'),{not_committed:true,status:400})}})
 await assert.rejects(directStart(a),/库存变化/);assert.equal(readPendingStart(a.storage),null)
 await assert.rejects(directStart({...a,post:async()=>{throw Error('network')}}),/尚未收到开工结果/)
 await assert.rejects(directStart({...a,employeeId:8}),/原操作人员/)
})
test('predecessor operations do not demand upstream BOM materials and blocked starts never post',async()=>{
 let writes=0;const a=args({get:async()=>({...preview(0),predecessor_required:true}),post:async()=>{writes++;return {ok:true}}})
 await directStart(a);assert.equal(writes,1)
 await assert.rejects(directStart({...a,get:async()=>({...preview(),blocked_reason:'前序未完成'})}),/前序未完成/);assert.equal(writes,1)
})
test('a pending BOM refresh requires explicit confirmation before any write',async()=>{
 let writes=0;const p={...preview(),bom_refresh_pending:true,effective_bom_version_no:'V002'}
 const a=args({get:async()=>p,post:async()=>{writes++;return {ok:true}}})
 const cancelled=await directStart({...a,confirmRefresh:()=>false});assert.equal(cancelled.cancelled,true);assert.equal(writes,0)
 await directStart({...a,confirmRefresh:v=>v.effective_bom_version_no==='V002'});assert.equal(writes,1)
})
