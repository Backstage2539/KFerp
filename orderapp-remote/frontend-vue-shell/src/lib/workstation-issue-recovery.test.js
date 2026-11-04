import test from 'node:test'
import assert from 'node:assert/strict'
import {readPendingIssue,savePendingIssue,clearPendingIssue,validIssueResult} from './workstation-issue-recovery.js'
const request={request_id:'id-1',job_card_ids:[42],preview_version:'v1',groups:[{key:'g',version:'gv',quantity:5.302}]}
function storage(){const map=new Map();return {getItem:k=>map.get(k)||null,setItem:(k,v)=>map.set(k,v),removeItem:k=>map.delete(k)}}
test('persists an exact request and restores the same ID and quantities after refresh',()=>{const s=storage();savePendingIssue(s,request);assert.deepEqual(readPendingIssue(s),request);savePendingIssue(s,request);clearPendingIssue(s,request);assert.equal(readPendingIssue(s),null)})
test('prevents another work order from overwriting or deleting an unresolved request',()=>{const s=storage();savePendingIssue(s,request);const other={...request,request_id:'id-2',job_card_ids:[43]};assert.throws(()=>savePendingIssue(s,other));assert.throws(()=>clearPendingIssue(s,other));assert.deepEqual(readPendingIssue(s),request)})
test('storage errors or corrupt records never silently discard an unresolved request',()=>{const s=storage();s.setItem('kferp:workstation-batch-issue:pending:v1','bad');assert.throws(()=>readPendingIssue(s));assert.throws(()=>savePendingIssue(s,request));assert.throws(()=>savePendingIssue({getItem(){throw Error('disabled')}},request))})
test('validates a complete matching receipt before clearing',()=>{assert.equal(validIssueResult({request_id:'id-1',success_count:1,failed_count:0,groups:[{key:'g',status:'submitted',quantity:5302,extra:0,remaining:0,unit:'kg',quantity_basis:'weight',documents:[{id:9,entry_no:'SE-9'}]}]},request),true);assert.equal(validIssueResult({request_id:'id-1',success_count:1,failed_count:0,groups:[]},request),false);assert.equal(validIssueResult({request_id:'other',success_count:1,failed_count:0,groups:[{key:'g',status:'submitted',quantity:5302,extra:0,remaining:0,unit:'kg',quantity_basis:'weight',documents:[{id:9,entry_no:'SE-9'}]}]},request),false)})
test('incomplete or inconsistent receipt details never clear a pending issue',()=>{
 const valid={request_id:'id-1',success_count:1,failed_count:0,groups:[{key:'g',status:'submitted',quantity:5302,extra:473,remaining:0,unit:'kg',quantity_basis:'weight',documents:[{id:9,entry_no:'SE-9'}]}]}
 assert.equal(validIssueResult(valid,request),true)
 for(const row of [
  {key:'g',status:'submitted'},
  {...valid.groups[0],documents:[]},
  {...valid.groups[0],quantity:0},
  {...valid.groups[0],quantity:5302,extra:5303},
  {...valid.groups[0],documents:[{id:0,entry_no:'SE-9'}]},
 ]) assert.equal(validIssueResult({...valid,groups:[row]},request),false)
 const failed={request_id:'id-1',success_count:0,failed_count:1,groups:[{key:'g',status:'failed',quantity:0,extra:0,remaining:10,documents:[],error:'库存变化'}]}
 assert.equal(validIssueResult(failed,request),true)
 assert.equal(validIssueResult({...failed,groups:[{...failed.groups[0],error:''}]},request),false)
})
