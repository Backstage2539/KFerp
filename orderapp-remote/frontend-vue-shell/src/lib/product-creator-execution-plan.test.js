import test from 'node:test'
import assert from 'node:assert/strict'
import { buildExecutionPlan } from './product-creator-execution-plan.js'
import { upgradeWorkflowToV8 } from './product-creator-variables.js'
const workflow = {version:8,nodes:[
 {id:'raw',kind:'material'}, {id:'route',kind:'process'}, {id:'roast',kind:'bom'},
 {id:'semi',kind:'material',config:{data_role:'output'}}, {id:'grind',kind:'bom'},
 {id:'product',kind:'product',config:{data_role:'output'}}
],edges:[['raw','roast'],['route','roast'],['roast','semi'],['semi','grind'],['grind','product']].map(([source,target])=>({source,target}))}
test('reuse boundaries preserve shared branches and restore on new, irrespective of array order',()=>{
 const w=structuredClone(workflow),inputs={semi:{action:'reuse'}}
 let p=buildExecutionPlan(w,inputs)
 assert.equal(p.raw.status,'skipped');assert.deepEqual(p.raw.reused_by_node_ids,['semi']);assert.equal(p.grind.status,'ready')
 w.edges.push({source:'route',target:'grind'})
 p=buildExecutionPlan(w,inputs);assert.equal(p.route.status,'ready');assert.equal(p.roast.status,'skipped')
 w.nodes.reverse();assert.deepEqual(buildExecutionPlan(w,inputs),p)
 inputs.semi.action='create';assert.equal(buildExecutionPlan(w,inputs).raw.status,'ready')
 inputs.product={action:'reuse'};p=buildExecutionPlan(w,inputs);assert.equal(p.grind.status,'skipped');assert.equal(p.semi.status,'skipped')
 const saved=JSON.parse(JSON.stringify(inputs));assert.deepEqual(buildExecutionPlan(w,saved),p)
 w.version=7;assert.ok(Object.values(buildExecutionPlan(w,inputs)).every(s=>s.status==='ready'))
})
test('multiple terminal outputs keep a shared input active',()=>{
 const w=structuredClone(workflow)
 w.nodes.push({id:'other',kind:'product',config:{data_role:'output'}});w.edges.push({source:'roast',target:'other'})
 const p=buildExecutionPlan(w,{semi:{action:'reuse'}})
 assert.equal(p.raw.status,'ready');assert.equal(p.roast.status,'ready')
})
test('upgrade preserves V7 config and naming without mutating historical workflow',()=>{
 const w=structuredClone(workflow);w.version=7;w.nodes[3].config.name_parts=[{type:'text',value:'半成品-'}]
 const v8=upgradeWorkflowToV8(w);assert.equal(v8.version,8);assert.equal(w.version,7);assert.deepEqual(v8.nodes,w.nodes)
})
