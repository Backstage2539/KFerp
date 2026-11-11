import test from 'node:test'
import assert from 'node:assert/strict'
import { completionDraft, completionPayload, routeExecution, completionQuantityLabel, completionRetry, suggestedMaterialQty, executionBatchInputQty } from './operation-completion.js'

test('batch planned input refuses to relabel a different frozen input unit',()=>{
 assert.equal(executionBatchInputQty({planned_input_qty:15000,input_unit:'g'},'kg'),null)
 assert.equal(executionBatchInputQty({planned_input_qty:14.962},'kg'),14.962)
 assert.equal(executionBatchInputQty({planned_input_qty:5,input_unit:'袋'},'kg'),null)
})

const preview = {version:'v1',planned_qty:20,input_qty:4540,input_unit:'g',output_unit:'袋',materials:[{key:'bean',unit:'g',qty:4540}]}
test('completion defaults to the planned batch and submits only the output quantity',()=>{
 const p={...preview,planned_qty:2.497,max_output_qty:0,input_qty:3,input_unit:'kg',output_unit:'kg',materials:[{reservation_id:1,unit:'kg',qty:1.5}]}
 const draft=completionDraft(p)
 assert.equal(draft.actual_output_qty,2.497)
 draft.actual_output_qty=2.8
 const payload=completionPayload(p,draft,'r1')
 assert.equal(payload.actual_output_qty,2.8)
 assert.equal(payload.finish_operation,true)
 assert.equal('actual_input_qty' in payload,false)
 assert.equal('materials' in payload,false)
 assert.equal('difference_reason' in payload,false)
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
 assert.deepEqual(progressPayload(preview,draft),{expected_version:preview.version,output_unit:preview.output_unit,actual_output_qty:18})
})
test('packaging material preview still suggests incremental usage per completed output',()=>{
 const p={version:'v2',planned_qty:11,total_planned_qty:12,completed_output_qty:1,input_qty:4540,input_unit:'g',output_unit:'袋',materials:[{reservation_id:4,key:'labels',unit:'个',qty:9,total_qty:10,consumed_qty:1}]}
 assert.equal(suggestedMaterialQty(p,p.materials[0],1),1)
})
test('completion default is not reduced by the available stock suggestion',()=>{
 const p={version:'v3',planned_qty:12,max_output_qty:1,input_qty:2724,input_unit:'g',output_unit:'袋',output_spec_g:227,materials:[{reservation_id:7,key:'beans',unit:'g',qty:2724,total_qty:2724,consumed_qty:0,available_qty:227,shortage_qty:2497}]}
 const draft=completionDraft(p)
 assert.equal(draft.actual_output_qty,12)
 assert.equal(completionPayload(p,draft,'full').finish_operation,true)
 assert.equal('materials' in completionPayload(p,draft,'full'),false)
 assert.equal(completionPayload(p,draft,'full').actual_output_qty,12)
})
test('weight suggestions follow integer gram ledger across partial completions',()=>{
 const p={planned_qty:4,total_planned_qty:4,completed_output_qty:0}
 assert.equal(suggestedMaterialQty(p,{unit:'kg',total_qty:3.385},3),2.539)
 assert.equal(suggestedMaterialQty({...p,planned_qty:1,completed_output_qty:3},{unit:'kg',total_qty:3.385,consumed_qty:2.539},1),.846)
 assert.equal(suggestedMaterialQty(p,{unit:'g',total_qty:3385},3),2539)
})
test('started batches report actual processed input and partial finish explicitly',()=>{
 const p={...preview,material_consumption_policy:'bom_start_v1',input_qty:5,input_unit:'kg',planned_qty:4,output_unit:'kg'}
 const d=completionDraft(p);assert.equal(d.actual_input_qty,5)
 d.actual_input_qty=2;d.actual_output_qty=1.6;d.finish_operation=false;d.actual_minutes=12;d.difference_reason='分批处理'
 const payload=completionPayload(p,d,'part')
 assert.equal(payload.actual_input_qty,2);assert.equal(payload.finish_operation,false);assert.equal(payload.actual_minutes,12)
 assert.equal('materials' in payload,false)
})
test('incoming reporting defaults to real input and submits waste for server calculation',()=>{
 const p={...preview,execution:{reporting_type:'incoming'},material_consumption_policy:'bom_start_v1',input_qty:84,input_unit:'kg',output_unit:'kg',planned_qty:80}
 const d=completionDraft(p);assert.equal(d.actual_input_qty,84);assert.equal(d.waste_qty,0)
 d.waste_qty=1;d.difference_reason='色选废料';const payload=completionPayload(p,d,'sort')
 assert.equal(payload.waste_qty,1);assert.equal(payload.actual_output_qty,83)
 d.waste_qty=85;assert.throws(()=>completionPayload(p,d,'sort'),/废料/)
})
test('route configuration keeps explicit reporting type and legacy default',()=>{
 assert.equal(routeExecution({},true).reporting_type,'legacy')
 assert.equal(routeExecution({execution_json:'{"reporting_type":"incoming"}'},false).reporting_type,'incoming')
})
test('incoming weight is converted between kg and g and requires a reason for waste',()=>{
 const p={...preview,execution:{reporting_type:'incoming'},material_consumption_policy:'bom_start_v1',input_qty:50.4,input_unit:'kg',output_unit:'g'}
 const d=completionDraft(p);d.waste_qty=1
 assert.throws(()=>completionPayload(p,d,'sort'),/原因/)
 d.difference_reason='色选废料'
 assert.equal(completionPayload(p,d,'sort').actual_output_qty,49400)
})
test('piece completion requires actual weighed remainder and never closes a partial target',()=>{
 const p={...preview,execution:{reporting_type:'piece'},material_consumption_policy:'bom_start_v1',input_qty:100,input_unit:'袋',output_unit:'袋',planned_qty:100,packaging_available_g:46400,packaging_standard_g_per_piece:454}
 const d=completionDraft(p);assert.equal(d.actual_remainder_g,1000);assert.equal(d.remainder_warehouse,'wip')
 assert.throws(()=>completionPayload(p,d,'piece'),/称重/)
 d.remainder_confirmed=true;d.actual_output_qty=50;d.actual_remainder_g=23700
 const payload=completionPayload(p,d,'piece');assert.equal(payload.actual_input_qty,50);assert.equal(payload.finish_operation,false);assert.equal(payload.actual_remainder_g,23700);assert.equal(payload.remainder_warehouse,'wip')
 d.actual_output_qty=101;assert.throws(()=>completionPayload(p,d,'piece'),/超过/)
})
test('packaging estimate rounds consumed grams before subtracting',async()=>{
 const {packagingExpectedRemainder}=await import('./operation-completion.js')
 assert.equal(packagingExpectedRemainder({packaging_available_g:1000,packaging_standard_g_per_piece:454.5},1),545)
})
test('unissued shared stock is never selected automatically and is bounded by available batch quantity',()=>{
 const p={...preview,execution:{reporting_type:'piece'},material_consumption_policy:'bom_start_v1',input_qty:100,input_unit:'袋',output_unit:'袋',planned_qty:100,packaging_available_g:45400,packaging_standard_g_per_piece:454,packaging_unissued_candidates:[{batch_code:'MP-1',warehouse:'wip',owner_customer_id:4,available_g:1280}]}
 const d=completionDraft(p);d.remainder_confirmed=true
 assert.deepEqual(completionPayload(p,d,'x').selected_unissued_remainders,[])
 d.selected_unissued_remainders=[{batch_code:'MP-1',warehouse:'wip',owner_customer_id:4,quantity_g:1281}]
 assert.throws(()=>completionPayload(p,d,'x'),/可用/)
 d.selected_unissued_remainders[0].quantity_g=1280
 assert.equal(completionPayload(p,d,'x').selected_unissued_remainders[0].quantity_g,1280)
})
