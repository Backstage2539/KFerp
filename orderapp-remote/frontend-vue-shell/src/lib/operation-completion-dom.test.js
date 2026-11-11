import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
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
test('execution batches require an explicitly entered output and preserve the batch identity',async()=>{
 const calls=[]
 const p={...structuredClone(preview),execution:{reporting_type:'batch'},material_consumption_policy:'bom_start_v1',input_unit:'kg',output_unit:'kg',input_qty:59.962,started_input_qty:59.962,planned_qty:60.112}
 const batches={version:'v1',can_initialize:false,batches:[{id:11,batch_no:1,planned_input_qty:15,status:'pending'},{id:12,batch_no:2,planned_input_qty:14.962,status:'pending'}]}
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:1},{apiGet:async path=>path.endsWith('/execution-batches')?batches:p,apiSend:async(path,{body})=>{calls.push({path,body});return {completion:{message:'完成'}}}})
 try{
  assert.match(view.el.textContent,/第 1 批/);assert.match(view.el.textContent,/第 2 批/)
  const inputs=view.el.querySelectorAll('fieldset input[type=number]')
  assert.equal(inputs[0].value,'');assert.equal(inputs[1].value,'15')
  button(view.el,'完成本批').click();await settle();assert.match(view.el.textContent,/填写实际/);assert.equal(calls.length,0)
  await input(inputs[0],0);button(view.el,'完成本批').click();await settle();assert.match(view.el.textContent,/原因/);assert.equal(calls.length,0)
  await input(inputs[0],12);await input(inputs[1],14.9);button(view.el,'完成本批').click();await settle()
  assert.equal(calls[0].body.execution_batch_id,11);assert.equal(calls[0].body.actual_input_qty,14.9);assert.equal(calls[0].body.actual_output_qty,12)
 }finally{view.close()}
})
test('legacy batch initialization uses its version and failed batch submission keeps entered values',async()=>{
 const calls=[];let initialized=false,previewVersion='v1'
 const p={...structuredClone(preview),execution:{reporting_type:'batch'},material_consumption_policy:'bom_start_v1',input_unit:'kg',output_unit:'kg',input_qty:59.962,started_input_qty:59.962,planned_qty:60.112}
 const saved={version:'b2',batches:[{id:11,batch_no:1,planned_input_qty:15,status:'pending'}]}
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:1},{apiGet:async path=>path.endsWith('/execution-batches')?(initialized?saved:{version:'b1',can_initialize:true,batches:[]}):{...p,version:previewVersion},apiSend:async(path,{body})=>{calls.push({path,body});if(path.endsWith('/execution-batches')){initialized=true;return saved}throw Object.assign(new Error('库存变化'),{status:409})}})
 try{
  button(view.el,'启用分批报工').click();await settle();assert.equal(calls[0].body.expected_version,'b1');assert.ok(calls[0].body.request_id)
  const inputs=view.el.querySelectorAll('fieldset input[type=number]');await input(inputs[0],12);await input(inputs[1],14.5)
  button(view.el,'完成本批').click();await settle();assert.match(view.el.textContent,/库存变化/);assert.equal(inputs[0].value,'12');assert.equal(inputs[1].value,'14.5')
  previewVersion='v2';button(view.el,'重新核对').click();await settle();assert.equal(inputs[0].value,'12');assert.equal(inputs[1].value,'14.5')
  button(view.el,'完成本批').click();await settle();assert.equal(calls.at(-1).body.expected_version,'v2')
 }finally{view.close()}
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
