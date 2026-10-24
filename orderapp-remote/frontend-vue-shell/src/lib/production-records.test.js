import test from 'node:test'
import assert from 'node:assert/strict'
import { legacyReviewDraft, legacyReviewPayload, correctionPayload } from './production-records.js'

const row={work_order_id:7,reported_qty:14,planned_qty:1,received_qty:0,output_unit:'袋',version:'v1',correction_cards:[{id:9}],materials:[{reservation_id:2,key:'material:2:0:0',qty:10,consumed_qty:0,unit:'kg',warehouse:'wip'}]}
test('old mistaken output and planned quantities never become confirmed quantities',()=>{
 const draft=legacyReviewDraft(row)
 assert.equal(draft.actual_output_qty,'')
 assert.equal(draft.materials[0].qty,'')
 assert.throws(()=>legacyReviewPayload(row,draft,'request'),/现场核实/)
 draft.actual_output_qty=1;draft.materials[0].qty=.227;draft.reason='现场复核'
 const payload=legacyReviewPayload(row,draft,'request')
 assert.equal(payload.actual_output_qty,1);assert.equal(payload.materials[0].qty,.227)
 assert.equal(payload.expected_version,'v1');assert.equal(payload.request_id,'request')
})
test('multiple source cards require explicit quantities summing to confirmed total',()=>{
 const multi={...row,correction_cards:[{id:9},{id:10}]};const draft=legacyReviewDraft(multi)
 Object.assign(draft,{actual_output_qty:2,reason:'实点'});draft.materials[0].qty=.454
 assert.throws(()=>legacyReviewPayload(multi,draft,'r'),/数量/)
 draft.card_quantities={9:1,10:1};assert.equal(legacyReviewPayload(multi,draft,'r').actual_output_qty,2)
 draft.card_quantities[10]=2;assert.throws(()=>legacyReviewPayload(multi,draft,'r'),/合计/)
})
test('correction requires actual integer and reason but does not impose planned cap',()=>{
 const record={unit:'袋',version:'v'}
 assert.throws(()=>correctionPayload(record,{quantity:1.5,reason:'复点'},'r'),/整数/)
 assert.throws(()=>correctionPayload(record,{quantity:28,reason:''},'r'),/原因/)
 assert.equal(correctionPayload(record,{quantity:28,reason:'实点'},'r').actual_output_qty,28)
})
