import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { clearWorkstationFormCache } from './workstation-form-cache.js'
test.afterEach(() => clearWorkstationFormCache('completion', 1))
import { parse, compileScript } from '@vue/compiler-sfc'
import { JSDOM } from 'jsdom'
const dom=new JSDOM('<html><body></body></html>',{url:'http://127.0.0.1/'})
for(const key of ['window','document','HTMLElement','Element','SVGElement','Node','Event','CustomEvent'])globalThis[key]=dom.window[key]
const {createApp,nextTick}=await import('vue')
let seq=0
async function mount(file,props,api){
 const key=`operationDOM${++seq}`;globalThis[key]=api
 const url=new URL(file,import.meta.url);const {descriptor}=parse(readFileSync(url,'utf8'))
 let code=compileScript(descriptor,{id:key,inlineTemplate:true}).content
 code=code.replace(/import \{ apiGet, apiSend \} from ['"][^'"]+['"]/,`const {apiGet,apiSend}=globalThis.${key}`)
 code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,url).href:import.meta.resolve(path))}`)
 const Component=(await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
 const el=document.createElement('div');document.body.appendChild(el);const app=createApp(Component,props);app.mount(el)
 await settle();return {el,close(){app.unmount();el.remove();delete globalThis[key]}}
}
async function settle(){await new Promise(resolve=>setImmediate(resolve));await nextTick()}
async function input(el,value){el.value=String(value);el.dispatchEvent(new Event('input',{bubbles:true}));await nextTick()}
const preview={enabled:true,job_card_id:1,version:'v1',execution:{receipt_enabled:true},product_name:'验收咖啡',package_spec_label:'227g袋装',output_unit:'袋',input_unit:'g',input_qty:4540,planned_qty:20,warehouse_name:'成品仓',materials:[{reservation_id:7,key:'bean',name:'熟豆',unit:'g',qty:4540,warehouse:'wip',warehouse_name:'在制仓',owner_customer_id:0}],draft:{}}
const button=(el,label)=>[...el.querySelectorAll('button')].find(b=>b.textContent.trim()===label||b.textContent.trim().includes(label))
test('all batch rows show independent planned defaults and submit together',async()=>{
 const calls=[]
 const p={...structuredClone(preview),execution:{reporting_type:'batch'},material_consumption_policy:'bom_start_v1',input_unit:'kg',output_unit:'kg',input_qty:2.5,started_input_qty:2.5,planned_qty:2}
 const state={version:'v1',batches:[{id:11,batch_no:1,planned_input_qty:1.25,planned_output_qty:1,planned_minutes:18,input_unit:'kg',output_unit:'kg',status:'pending'},{id:12,batch_no:2,planned_input_qty:1.25,planned_output_qty:1,planned_minutes:18,input_unit:'kg',output_unit:'kg',status:'pending'}]}
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:1},{apiGet:async path=>path.endsWith('/execution-batches')?state:p,apiSend:async(path,{body})=>{calls.push({path,body});return {items:[]}}})
 try{
  const rows=view.el.querySelectorAll('.batch-row');assert.equal(rows.length,2)
  assert.deepEqual([...rows[0].querySelectorAll('input[type=number]')].map(x=>x.value),['1.25','18','1'])
  await input(rows[1].querySelectorAll('input[type=number]')[2],0.9)
  button(view.el,'完成所有批次').click();await settle()
  assert.ok(calls[0].path.endsWith('/execution-batches/complete-all'));assert.equal(calls[0].body.batches[1].actual_output_qty,0.9)
 }finally{view.close()}
})
test('a single batch can be completed without losing other drafts and zero output requires a remark',async()=>{
 const calls=[];let completed=false
 const p={...structuredClone(preview),execution:{reporting_type:'batch'},material_consumption_policy:'bom_start_v1',input_unit:'kg',output_unit:'kg',input_qty:2.5,started_input_qty:2.5,planned_qty:2}
 const state=()=>({version:'v1',batches:[{id:11,batch_no:1,planned_input_qty:1.25,planned_output_qty:1,planned_minutes:18,status:completed?'completed':'pending',actual_input_qty:completed?1.25:null,actual_output_qty:completed?1:null},{id:12,batch_no:2,planned_input_qty:1.25,planned_output_qty:1,planned_minutes:18,status:'pending'}]})
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:51},{apiGet:async path=>path.endsWith('/execution-batches')?state():p,apiSend:async(path,{body})=>{calls.push({path,body});completed=true;return {completion:{message:'完成'}}}})
 try{let rows=view.el.querySelectorAll('.batch-row');await input(rows[1].querySelectorAll('input[type=number]')[2],0.9);await input(rows[0].querySelectorAll('input[type=number]')[2],0);button(rows[0],'批次完成').click();await settle();assert.match(view.el.textContent,/备注/);assert.equal(calls.length,0);await input(rows[0].querySelectorAll('input[type=number]')[2],1);button(rows[0],'批次完成').click();await settle();rows=view.el.querySelectorAll('.batch-row');assert.equal(calls[0].body.execution_batch_id,11);assert.equal(calls[0].body.actual_minutes_explicit,true);assert.equal(rows[1].querySelectorAll('input[type=number]')[2].value,'0.9');assert.match(rows[0].textContent,/已完成/)}finally{view.close()}
})
test('legacy initialization and conflict refresh retain batch inputs',async()=>{
 const calls=[];let initialized=false,previewVersion='v1'
 const p={...structuredClone(preview),execution:{reporting_type:'batch'},material_consumption_policy:'bom_start_v1',input_unit:'kg',output_unit:'kg',input_qty:15,started_input_qty:15,planned_qty:12}
 const saved={version:'b2',batches:[{id:11,batch_no:1,planned_input_qty:15,planned_output_qty:12,planned_minutes:18,status:'pending'}]}
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:52},{apiGet:async path=>path.endsWith('/execution-batches')?(initialized?saved:{version:'b1',can_initialize:true,batches:[]}):{...p,version:previewVersion},apiSend:async(path,{body})=>{calls.push({path,body});if(path.endsWith('/execution-batches')){initialized=true;return saved}throw Object.assign(new Error('库存变化'),{status:409})}})
 try{button(view.el,'启用分批报工').click();await settle();assert.equal(calls[0].body.expected_version,'b1');let inputs=view.el.querySelectorAll('.batch-row input[type=number]');await input(inputs[0],14.5);await input(inputs[2],11);button(view.el,'批次完成').click();await settle();assert.match(view.el.textContent,/库存变化/);previewVersion='v2';button(view.el,'重新核对').click();await settle();inputs=view.el.querySelectorAll('.batch-row input[type=number]');assert.equal(inputs[0].value,'14.5');assert.equal(inputs[2].value,'11')}finally{view.close()}
})
test('completion form only asks for output, validates integer quantities, and retries the identical request after a lost response',async()=>{
 const calls=[];let lost=true
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:1},{apiGet:async()=>structuredClone(preview),apiSend:async(path,{body})=>{calls.push({path,body:structuredClone(body)});if(lost){lost=false;throw new Error('response lost')};return {completion:{message:'已入成品仓',output_qty:18,output_unit:'袋',stock_entry_ids:[8]}}}})
 try{
  assert.match(view.el.textContent,/227g袋装/);assert.match(view.el.textContent,/成品仓/)
  const fields=view.el.querySelectorAll('.completion-form input')
  assert.equal(fields.length,1)
  assert.doesNotMatch(view.el.textContent,/本单用料|实际投入|实际工时|保存进度|差异原因/)
  await input(fields[0],1.5);button(view.el,'确认工序完成').click();await settle();assert.match(view.el.textContent,/整数/);assert.equal(calls.length,0)
  await input(fields[0],18);button(view.el,'确认工序完成').click();await settle();assert.equal(calls.length,1);assert.ok(calls[0].path.endsWith('/complete'));assert.match(view.el.textContent,/尚未收到完成结果/)
  assert.equal(view.el.querySelector('fieldset').disabled,true)
  button(view.el,'重试本次完成').click();await settle();assert.deepEqual(calls[0].body,calls[1].body);assert.equal(calls[1].body.expected_version,'v1');assert.equal(calls[1].body.actual_output_qty,18);assert.equal('materials' in calls[1].body,false);assert.equal('actual_input_qty' in calls[1].body,false);assert.match(view.el.textContent,/已入成品仓/)
 }finally{view.close()}
})
test('completion quantity defaults to the server-provided remaining batch output',async()=>{
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:1},{apiGet:async()=>({...structuredClone(preview),planned_qty:2.497,output_unit:'kg'}),apiSend:async()=>{throw new Error('unexpected write')}})
 try{const fields=view.el.querySelectorAll('.completion-form input');assert.equal(fields.length,1);assert.equal(fields[0].value,'2.497');assert.match(view.el.textContent,/本次完成数量（kg）/)}finally{view.close()}
})
test('only a genuinely incomplete BOM links to BOM repair; execution blockers explain themselves in place',async()=>{
 const legacy=await mount('../components/OperationCompletionForm.vue',{jobCardId:1},{apiGet:async()=>({...structuredClone(preview),enabled:false,bom_id:88,blocked_code:'legacy_execution_unresolved',blocked_reason:'旧工单缺少可自动接入的执行配置'}),apiSend:async()=>{throw new Error('blocked task must not submit')}})
 try{assert.match(legacy.el.textContent,/旧工单缺少可自动接入的执行配置/);assert.equal(button(legacy.el,'去 BOM 补齐配料'),undefined);assert.equal(button(legacy.el,'确认工序完成').disabled,true)}finally{legacy.close()}
 const incomplete=await mount('../components/OperationCompletionForm.vue',{jobCardId:1},{apiGet:async()=>({...structuredClone(preview),enabled:false,bom_id:88,blocked_code:'bom_incomplete',blocked_reason:'BOM 配料比例合计 70%，请在 BOM 中补齐后继续'}),apiSend:async()=>{throw new Error('blocked task must not submit')}})
 try{assert.match(incomplete.el.textContent,/BOM 配料比例合计 70%/);assert.ok(button(incomplete.el,'去 BOM 补齐配料'));let target;window.addEventListener('kferp:navigate-view',event=>{target=event.detail},{once:true});button(incomplete.el,'去 BOM 补齐配料').click();assert.equal(target.params.production_bom_id,88)}finally{incomplete.close()}
})
test('incoming completion shows waste and calculated output instead of editable output',async()=>{
 const calls=[]
 const p={...structuredClone(preview),execution:{reporting_type:'incoming'},material_consumption_policy:'bom_start_v1',input_qty:84,input_unit:'kg',output_unit:'kg',planned_qty:80}
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:1},{apiGet:async()=>p,apiSend:async(path,{body})=>{calls.push(body);return {completion:{message:'完成'}}}})
 try{
  assert.match(view.el.textContent,/废料重量/);assert.match(view.el.textContent,/合格产出：84 kg/)
  const inputs=view.el.querySelectorAll('fieldset input')
  await input(inputs[1],1);assert.match(view.el.textContent,/合格产出：83 kg/)
  await input(view.el.querySelector('textarea'),'色选废料');button(view.el,'确认工序完成').click();await settle()
  assert.equal(calls[0].waste_qty,1);assert.equal(calls[0].actual_output_qty,83)
 }finally{view.close()}
})
test('piece completion requests weighed remainder, warehouse and preserves a partial target',async()=>{
 const calls=[]
 const p={...structuredClone(preview),execution:{reporting_type:'piece'},material_consumption_policy:'bom_start_v1',input_qty:100,input_unit:'袋',output_unit:'袋',planned_qty:100,total_planned_qty:100,packaging_available_g:46400,packaging_standard_g_per_piece:454}
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:1},{apiGet:async path=>path.includes('warehouses')?{rows:[{code:'wip',name:'WIP'},{code:'finished_goods',name:'成品仓'}]}:p,apiSend:async(path,{body})=>{calls.push(body);return {completion:{message:'完成'}}}})
 try{
  assert.match(view.el.textContent,/预计余料 1000 g/)
  const fields=view.el.querySelectorAll('fieldset input');await input(fields[0],50)
  assert.match(view.el.textContent,/预计余料 23700 g/)
  button(view.el,'确认工序完成').click();await settle();assert.match(view.el.textContent,/称重/);assert.equal(calls.length,0)
  const checkbox=view.el.querySelector('input[type=checkbox]');checkbox.click();await nextTick();button(view.el,'确认工序完成').click();await settle()
  assert.equal(calls[0].actual_remainder_g,23700);assert.equal(calls[0].finish_operation,false);assert.equal(calls[0].remainder_warehouse,'wip')
 }finally{view.close()}
})

test('unmounting and reopening retains the input for the same task only',async()=>{
 const p={...structuredClone(preview),execution:{reporting_type:'batch'},material_consumption_policy:'bom_start_v1',input_unit:'kg',output_unit:'kg',input_qty:1.25,started_input_qty:1.25,planned_qty:1}
 const state={version:'v1',batches:[{id:11,batch_no:1,planned_input_qty:1.25,planned_output_qty:1,planned_minutes:18,status:'pending'}]}
 const api={apiGet:async path=>path.endsWith('/execution-batches')?state:p,apiSend:async()=>{throw new Error('unexpected write')}}
 const first=await mount('../components/OperationCompletionForm.vue',{jobCardId:71},api);await input(first.el.querySelectorAll('.batch-row input[type=number]')[2],0.8);first.close()
 const reopened=await mount('../components/OperationCompletionForm.vue',{jobCardId:71},api);const other=await mount('../components/OperationCompletionForm.vue',{jobCardId:72},api)
 try{assert.equal(reopened.el.querySelectorAll('.batch-row input[type=number]')[2].value,'0.8');assert.equal(other.el.querySelectorAll('.batch-row input[type=number]')[2].value,'1')}finally{reopened.close();other.close()}
})

test('a task not yet started allows viewing but does not allow batch completion',async()=>{
 const p={...structuredClone(preview),execution:{reporting_type:'batch'},material_consumption_policy:'bom_start_v1',input_unit:'kg',output_unit:'kg'}
 const state={version:'v1',batches:[{id:11,batch_no:1,planned_input_qty:1.25,planned_output_qty:1,planned_minutes:18,status:'pending'}]}
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:73,canComplete:false},{apiGet:async path=>path.endsWith('/execution-batches')?state:p,apiSend:async()=>{throw new Error('unexpected write')}})
 try{assert.equal(button(view.el,'批次完成').disabled,true);assert.equal(button(view.el,'完成所有批次').disabled,true)}finally{view.close()}
})

test('an uncertain batch completion retries the identical request even after reopening',async()=>{
 const calls=[];let lost=true
 const p={...structuredClone(preview),execution:{reporting_type:'batch'},material_consumption_policy:'bom_start_v1',input_unit:'kg',output_unit:'kg',input_qty:1.25,started_input_qty:1.25,planned_qty:1}
 const state={version:'v1',batches:[{id:11,batch_no:1,planned_input_qty:1.25,planned_output_qty:1,planned_minutes:18,status:'pending'}]}
 const api={apiGet:async path=>path.endsWith('/execution-batches')?state:p,apiSend:async(path,{body})=>{calls.push(structuredClone(body));if(lost){lost=false;throw new Error('response lost')}return {completion:{}}}}
 const first=await mount('../components/OperationCompletionForm.vue',{jobCardId:74},api);button(first.el,'批次完成').click();await settle();assert.match(first.el.textContent,/重试原次操作/);first.close()
 const reopened=await mount('../components/OperationCompletionForm.vue',{jobCardId:74},api)
 try{button(reopened.el,'重试本次完成').click();await settle();assert.deepEqual(calls[0],calls[1])}finally{reopened.close()}
})

test('switching tasks restores the unsubmitted completion quantity',async()=>{
 const id=98766,api={apiGet:async()=>structuredClone(preview),apiSend:async()=>{throw new Error('unexpected write')}}
 let view=await mount('../components/OperationCompletionForm.vue',{jobCardId:id},api)
 try {await input(view.el.querySelector('.completion-form input'),7)} finally {view.close()}
 view=await mount('../components/OperationCompletionForm.vue',{jobCardId:id},api)
 try {assert.equal(view.el.querySelector('.completion-form input').value,'7')} finally {view.close()}
})
