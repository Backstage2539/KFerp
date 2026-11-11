import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import * as Vue from 'vue'

const url = new URL('../components/ProductionPlanSemiFinishedStock.vue', import.meta.url)
const fixture = { draft_token:'v1',items:[{key:'material:1',name:'初晓',unit:'kg',required_qty:60.112,stock_covered_qty:0,batches:[{batch_id:3,warehouse:'raw_materials',available_qty:15,selected_qty:0}]}] }
async function setup(send, detail = { id:26, draft_token:'v1', status:'draft' }) {
 const calls=[], events=[]
 const client={apiGet:async()=>structuredClone(fixture),apiSend:async (...args)=>{calls.push(args);return send(...args)}}
 const {descriptor}=parse(fs.readFileSync(url,'utf8'))
 const compiled=compileScript(descriptor,{id:'confirm-flow'}),bindings={}
 for(const [name,b] of Object.entries(compiled.imports)) bindings[name]=(b.source==='vue'?Vue:b.source==='../api/client'?client:await import(new URL(b.source.endsWith('.js')?b.source:`${b.source}.js`,url)))[b.imported]
 const code=compiled.content.replace(/^import\s+[\s\S]*?\s+from\s+(['"])[^'"]+\1;?\n/gm,'').replace('export default','return')
 const component=new Function(...Object.keys(bindings),code)(...Object.values(bindings)), scope=Vue.effectScope()
 const state=scope.run(()=>component.setup({detail,disabled:false,allowTransfer:true},{expose(){},emit:(...args)=>events.push(args)}))
 await state.load()
 return {state,calls,events,close:()=>scope.stop()}
}

test('one confirm checks and saves exactly the same selection',async()=>{
 const app=await setup(async(_,o)=>o.body.check_only?{preview_token:'p1',changes:[]}:{plan:{id:26,draft_token:'v2'}})
 try{await app.state.open();app.state.values.value['material:1|raw_materials|3']=15;await app.state.commit();assert.equal(app.calls.length,2);assert.equal(app.calls[0][1].body.check_only,true);assert.equal(app.calls[1][1].body.check_only,false);assert.equal(app.calls[1][1].body.preview_token,'p1');assert.deepEqual(app.calls[0][1].body.selections,app.calls[1][1].body.selections);assert.deepEqual(app.events.find(e=>e[0]==='updated'),['updated',{id:26,draft_token:'v2'}])}finally{app.close()}
})

test('check failure and commit conflict retain selected input',async()=>{
 for(const phase of ['check','save']){const app=await setup(async(_,o)=>{if(o.body.check_only){if(phase==='check')throw Error('库存已变化');return{preview_token:'p',changes:[]}}throw Error('库存已变化')});try{await app.state.open();app.state.values.value['material:1|raw_materials|3']=10;await app.state.commit();assert.equal(app.state.values.value['material:1|raw_materials|3'],10);assert.equal(app.state.opened.value,true);assert.match(app.state.actionError.value,/库存已变化/);assert.equal(app.calls.length,phase==='check'?1:2)}finally{app.close()}}
})

test('double click does not make a second request',async()=>{
 let release;const app=await setup(async(_,o)=>{if(o.body.check_only){await new Promise(r=>release=r);return{preview_token:'p',changes:[]}}return{plan:{id:26}}});try{await app.state.open();const a=app.state.commit();await app.state.commit();assert.equal(app.calls.length,1);release();await a;assert.equal(app.calls.length,2)}finally{app.close()}
})

test('transfer retries with identical idempotency key and updates plan on success',async()=>{
 let fail=true;const detail={id:26,draft_token:'v1',status:'draft',component_sources:[{id:2,allocation_mode:'semi_finished_selected',transfer_g:15000}]};const app=await setup(async()=>{if(fail)throw Error('网络中断');return{plan:{id:26,draft_token:'v2'}}},detail)
 try{await app.state.transferToWIP();assert.match(app.state.transferError.value,/网络中断/);fail=false;await app.state.transferToWIP();assert.match(app.calls[0][0],/semi-finished-stock\/transfer-to-wip$/);assert.equal(app.calls[0][1].body.draft_token,'v1');assert.ok(app.calls[0][1].body.idempotency_key);assert.equal(app.calls[0][1].body.idempotency_key,app.calls[1][1].body.idempotency_key);assert.ok(app.events.some(e=>e[0]==='updated'))}finally{app.close()}
})

test('capacity page has no per-task reporting recheck and selection has no visible preview',()=>{
 const source=fs.readFileSync(url,'utf8');const view=fs.readFileSync(new URL('../views/ProducePlanView.vue',import.meta.url),'utf8')
 assert.doesNotMatch(source,/调整预览|预览变化|确认并更新安排|生产仓/);assert.doesNotMatch(view,/<OperationReportingRecheck/);assert.match(source,/确认使用/);assert.match(source,/转入 WIP 仓/);assert.match(source,/@media\(max-width:700px\)/)
})

test('selected WIP-only inventory hides transfer action and inflight appears in the equation',async()=>{
 const detail={id:26,draft_token:'v1',status:'draft',component_sources:[{id:1,allocation_mode:'semi_finished_selected',wip_covered_g:15000,transfer_g:0}]}
 const app=await setup(async()=>({}),detail)
 try{assert.equal(app.state.selectedTransferSources.value.length,0);assert.equal(app.state.selectedWipReady.value,true);await app.state.transferToWIP();assert.equal(app.calls.length,0)}finally{app.close()}
 const source=fs.readFileSync(url,'utf8');assert.match(source,/v-if="Number\(item.inflight_covered_qty \|\| 0\) > 0"/);assert.match(source,/已在 WIP 仓，可直接使用/)
})
