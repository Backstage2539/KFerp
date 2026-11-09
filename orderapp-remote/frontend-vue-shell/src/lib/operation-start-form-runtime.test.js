import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {parse,compileScript,compileTemplate} from '@vue/compiler-sfc'
import {createSSRApp,h,reactive} from 'vue'
import {renderToString} from '@vue/server-renderer'
import {startDraft} from './operation-start.js'
let sequence=0
async function loadForm(mocks){
 const file=new URL('../components/OperationStartForm.vue',import.meta.url),id=`pr747-start-${++sequence}`
 const {descriptor}=parse(readFileSync(file,'utf8'),{filename:file.pathname}),script=compileScript(descriptor,{id}),template=compileTemplate({id,source:descriptor.template.content,filename:file.pathname,compilerOptions:{bindingMetadata:script.bindings}})
 assert.deepEqual(template.errors,[]);globalThis[id]=mocks
 let code=script.content.replace(/import \{apiGet,apiSend\} from ['"][^'"]+['"]/,`const {apiGet,apiSend}=globalThis[${JSON.stringify(id)}]`)
 code=code.replace('export default','const component =')+'\n'+template.code.replace('export function render','function render')+'\ncomponent.render=render;export default component;'
 code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,file).href:import.meta.resolve(path))}`)
 const component=(await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default;delete globalThis[id];return component
}
test('start form shows batch and warehouse, then retries the same confirmation after a lost response',async()=>{
 const writes=[],events=[];let fail=true
 const Component=await loadForm({apiGet:async()=>{throw Error('unexpected read')},apiSend:async(url,options)=>{writes.push({url,body:options.body});if(fail){fail=false;throw new Error('connection interrupted')}return {ok:true}}})
 const props=reactive({jobCardId:42});let state
 await renderToString(createSSRApp({setup(){state=Component.setup(props,{expose(){},emit:(event,result)=>events.push({event,result})});return()=>h('div')}}))
 const preview={product_name:'测试红岩',effective_bom_version_no:'V002',operation_name:'烘焙',planned_qty:4,output_unit:'kg',input_qty:5,input_unit:'kg',recipe_ratio_pct:100,version:'v1',materials:[{key:'a',name:'测试生豆',unit:'kg',qty:5,ratio_pct:100,warehouse:'wip',warehouse_name:'WIP',owner_customer_id:0,eligible_batches:[{batch_code:'PR747-BATCH',available_qty:10}]}]}
 state.preview.value=preview;Object.assign(state.draft,startDraft(preview))
 const html=await renderToString(createSSRApp({...Component,setup:()=>state},props));for(const label of ['V002','WIP','PR747-BATCH','确认开工并扣料'])assert.ok(html.includes(label),label)
 await state.submit();assert.equal(state.retry.uncertain,true);assert.equal(writes.length,1)
 await state.submit();assert.equal(writes.length,2);assert.deepEqual(writes[1].body,writes[0].body);assert.match(writes[0].body.request_id,/^[0-9a-f-]{36}$/);assert.equal(events[0].event,'posted')
})
test('short WIP opens replenishment with the actual start deficit and prevents uncertain retries from changing stock',async()=>{
 const events=[]
 const Component=await loadForm({apiGet:async()=>{throw Error('unexpected read')},apiSend:async()=>({})})
 const props=reactive({jobCardId:42});let state
 await renderToString(createSSRApp({setup(){state=Component.setup(props,{expose(){},emit:(...args)=>events.push(args)});return()=>h('div')}}))
 const preview={input_qty:45,input_unit:'kg',recipe_ratio_pct:100,materials:[{key:'bean',name:'水洗 A',unit:'kg',qty:45,ratio_pct:100,warehouse:'wip',available_qty:39.698}]}
 state.preview.value=preview;Object.assign(state.draft,startDraft(preview))
 const html=()=>renderToString(createSSRApp({...Component,setup:()=>state},props))
 assert.match(await html(),/补齐物料/)
 assert.match(await html(),/5\.302/)
 state.openIssue();assert.equal(events[0][0],'issue');assert.equal(events[0][1][0].shortage_qty,5.302)
 state.retry.begin({},()=> 'test');state.retry.fail(new Error('network'));assert.match(await html(),/disabled[^>]*>补齐物料/)
 state.openIssue();assert.equal(events.length,1)
 state.retry.clear();state.preview.value.predecessor_required=true
 assert.doesNotMatch(await html(),/补齐物料/)
})
test('start form locks while replenishment drawer is open and warns that successful replenishment reloads the draft',async()=>{
 const events=[];const Component=await loadForm({apiGet:async()=>({}),apiSend:async()=>({})})
 const props=reactive({jobCardId:42,locked:true});let state
 await renderToString(createSSRApp({setup(){state=Component.setup(props,{expose(){},emit:(...args)=>events.push(args)});return()=>h('div')}}))
 const preview={input_qty:45,input_unit:'kg',recipe_ratio_pct:100,materials:[{key:'bean',name:'水洗 A',unit:'kg',qty:45,ratio_pct:100,warehouse:'wip',available_qty:39.698}]}
 state.preview.value=preview;Object.assign(state.draft,startDraft(preview))
 const html=await renderToString(createSSRApp({...Component,setup:()=>state},props))
 assert.match(html,/disabled[^>]*>确认开工并扣料/)
 assert.match(html,/补料成功后.*重新读取.*重置/)
 state.openIssue();assert.equal(events.length,0)
})
test('zero WIP omitted by the API still shows replenishment and the full reserved-material deficit',async()=>{
 const events=[];const Component=await loadForm({apiGet:async()=>({}),apiSend:async()=>({})})
 const props=reactive({jobCardId:67});let state
 await renderToString(createSSRApp({setup(){state=Component.setup(props,{expose(){},emit:(...args)=>events.push(args)});return()=>h('div')}}))
 // Production serializes zero available_qty with omitempty.
 const preview={input_qty:98,input_unit:'袋',materials:[{key:'material:108:0:0',name:'曜石2.0-半成品',unit:'kg',qty:44.493,shortage_qty:44.493,warehouse:'wip',warehouse_name:'WIP在制仓'}]}
 state.preview.value=preview;Object.assign(state.draft,startDraft(preview))
 const html=()=>renderToString(createSSRApp({...Component,setup:()=>state},props))
 assert.match(await html(),/补齐物料/)
 assert.match(await html(),/可用 0/)
 state.openIssue();assert.equal(events[0][0],'issue');assert.equal(events[0][1][0].shortage_qty,44.493)
 state.draft.actual_input_qty=49;state.recalculate()
 assert.equal(state.startShortages.value[0].shortage_qty,22.247)
 state.draft.materials[0].available_qty=0
 assert.equal(state.startShortages.value[0].shortage_qty,22.247)
 state.draft.materials[0].available_qty=22.247
 assert.doesNotMatch(await html(),/补齐物料/)
 state.draft.materials[0].available_qty='invalid'
 assert.doesNotMatch(await html(),/补齐物料/)
})
