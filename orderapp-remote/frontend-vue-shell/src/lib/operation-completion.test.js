import test from 'node:test'
import assert from 'node:assert/strict'
import { completionDraft, completionPayload, routeExecution, completionQuantityLabel, completionRetry, suggestedMaterialQty } from './operation-completion.js'

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
test('partial batch can be posted while leaving the operation open and suggesting only incremental materials',()=>{
 const p={version:'v2',planned_qty:11,total_planned_qty:12,completed_output_qty:1,input_qty:4540,input_unit:'g',output_unit:'袋',materials:[{reservation_id:4,key:'labels',unit:'个',qty:9,total_qty:10,consumed_qty:1}]}
 const draft=completionDraft(p)
 assert.equal(draft.finish_operation,true)
 assert.equal(draft.materials[0].qty,9)
 draft.actual_output_qty=1;draft.finish_operation=false
 assert.equal(suggestedMaterialQty(p,p.materials[0],1),1)
 assert.equal(completionPayload(p,draft,'r2').finish_operation,false)
 draft.finish_operation=true
 assert.throws(()=>completionPayload(p,draft,'r3'),/原因/)
 draft.difference_reason='现场确认最后只完成1袋'
 assert.equal(completionPayload(p,draft,'r4').finish_operation,true)
})
test('completion defaults to the quantity supported by reserved good stock and leaves the rest open',()=>{
 const p={version:'v3',planned_qty:12,max_output_qty:1,input_qty:2724,input_unit:'g',output_unit:'袋',output_spec_g:227,materials:[{reservation_id:7,key:'beans',unit:'g',qty:2724,total_qty:2724,consumed_qty:0,available_qty:227,shortage_qty:2497}]}
 const draft=completionDraft(p)
 assert.equal(draft.actual_output_qty,1)
 assert.equal(draft.actual_input_qty,227)
 assert.equal(draft.finish_operation,false)
 assert.equal(draft.materials[0].qty,227)
 assert.equal(completionPayload(p,draft,'partial').actual_output_qty,1)
})

test('weight suggestions follow integer gram ledger across partial completions',()=>{
 const p={planned_qty:4,total_planned_qty:4,completed_output_qty:0}
 assert.equal(suggestedMaterialQty(p,{unit:'kg',total_qty:3.385},3),2.539)
 assert.equal(suggestedMaterialQty({...p,planned_qty:1,completed_output_qty:3},{unit:'kg',total_qty:3.385,consumed_qty:2.539},1),.846)
 assert.equal(suggestedMaterialQty(p,{unit:'g',total_qty:3385},3),2539)
})

test('partial input follows frozen input output ratio when specification grams are not a legacy field',()=>{
 const p={planned_qty:24,max_output_qty:18,input_qty:5448,input_unit:'g',output_unit:'袋',output_spec_g:0,materials:[]}
 assert.equal(completionDraft(p).actual_input_qty,4086)
})
