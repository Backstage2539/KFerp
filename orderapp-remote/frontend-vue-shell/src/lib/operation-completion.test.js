import test from 'node:test'
import assert from 'node:assert/strict'
import { completionDraft, completionPayload, routeExecution, completionQuantityLabel, completionRetry } from './operation-completion.js'

const preview = {version:'v1',planned_qty:20,input_qty:4540,input_unit:'g',output_unit:'袋',materials:[{key:'bean',unit:'g',qty:4540}]}
test('completion uses real quantity without plan cap and validates count units',()=>{
 const draft=completionDraft(preview);draft.actual_output_qty=28;draft.difference_reason='实际多装8袋'
 assert.equal(completionPayload(preview,draft,'r1').actual_output_qty,28)
 draft.actual_output_qty=1.5;assert.throws(()=>completionPayload(preview,draft,'r1'),/整数/)
 assert.equal(completionQuantityLabel('袋'),'本次完成袋数')
})
test('last route step receives by default and stable identity survives edits',()=>{
 assert.equal(routeExecution({},true).receipt_enabled,true)
 assert.equal(routeExecution({execution_json:'{"node_key":"s1","receipt_enabled":false,"source_warehouse":"raw"}'},false).node_key,'s1')
})
test('lost response keeps original request and payload; known rejection permits editing',()=>{
 const retry=completionRetry();const first=retry.begin({qty:1},()=> 'a');retry.fail({});assert.deepEqual(retry.begin({qty:2},()=> 'b'),first)
 assert.equal(retry.uncertain,true);retry.fail({status:400});assert.equal(retry.uncertain,false);assert.equal(retry.begin({qty:2},()=> 'b').request_id,'b')
})
test('progress keeps an unfinished draft without demanding completion difference reason',async()=>{
 const {progressPayload}=await import('./operation-completion.js')
 const draft=completionDraft(preview);draft.actual_output_qty=18
 assert.equal(progressPayload(preview,draft).actual_output_qty,18)
 assert.equal(progressPayload(preview,draft).difference_reason,'')
})
