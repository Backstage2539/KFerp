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
test('piece completion omits remainder fields and keeps entered minutes',async()=>{
 const calls=[]
 const p={...structuredClone(preview),execution:{reporting_type:'piece'},material_consumption_policy:'bom_start_v1',input_qty:100,input_unit:'袋',output_unit:'袋',planned_qty:100,total_planned_qty:100,planned_minutes:18,packaging_available_g:46400,packaging_standard_g_per_piece:454}
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:1},{apiGet:async()=>p,apiSend:async(path,{body})=>{calls.push(body);return {completion:{message:'完成'}}}})
 try{
  assert.doesNotMatch(view.el.textContent,/实际余料|预计余料|余料去向|已称重|同批次未领/)
  const fields=view.el.querySelectorAll('fieldset input');assert.equal(fields[1].value,'18');await input(fields[1],19);await input(fields[0],50);assert.equal(fields[1].value,'19')
  button(view.el,'确认工序完成').click();await settle()
  assert.equal(calls[0].remainder_mode,'plan_transfer');assert.equal(calls[0].finish_operation,false);assert.equal('actual_remainder_g' in calls[0],false)
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

test('plan packaging remainder remains accessible and submits source quantities only after all tasks finish',async()=>{
 const calls=[]
 const api={apiGet:async path=>path==='/api/packaging-remainders'?{groups:[{plan_id:26,product_id:1,product_name:'曲奇',total_tasks:2,completed_tasks:2,can_confirm:true}]}:path==='/api/stock/warehouses'?{rows:[{code:'wip',name:'WIP在制仓'},{code:'raw',name:'原料仓'}]}:{plan_id:26,product_id:1,product_name:'曲奇',version:'v1',can_confirm:true,tasks:[{job_card_id:1,work_order_no:'WO1',status:'completed'}],sources:[{source_id:'issued:1',batch_code:'MP1',warehouse:'wip',owner_customer_id:0,available_g:270}]},apiSend:async(path,{body})=>{calls.push({path,body});return {}}}
 const view=await mount('../components/PackagingRemainderPanel.vue',{},api)
 try {button(view.el,'选择').click();await settle();await input(view.el.querySelector('input[type=number]'),270);button(view.el,'确认处理').click();await settle();assert.equal(calls[0].body.expected_version,'v1');assert.deepEqual(calls[0].body.sources,[{source_id:'issued:1',quantity_g:270}]);assert.equal(calls[0].body.target_warehouse,'wip')}finally{view.close()}
})

test('plan remainder preserves independent inputs across groups and submits explicit zero',async()=>{
 const calls=[]
 const api={apiGet:async path=>path==='/api/packaging-remainders'?{groups:[{plan_id:1,product_id:1,product_name:'一',total_tasks:1,completed_tasks:1},{plan_id:2,product_id:1,product_name:'二',total_tasks:1,completed_tasks:1}]}:path==='/api/stock/warehouses'?{rows:[{code:'wip',name:'WIP'}]}:{plan_id:path.includes('/1/')?1:2,product_id:1,product_name:'咖啡',version:'v',can_confirm:true,tasks:[],sources:[{source_id:'shared',batch_code:'MP',warehouse:'wip',available_g:10}]},apiSend:async(path,{body})=>{calls.push(body);return {}}}
 const view=await mount('../components/PackagingRemainderPanel.vue',{},api)
 try{let choices=[...view.el.querySelectorAll('button')].filter(x=>x.textContent.trim()==='选择');choices[0].click();await settle();await input(view.el.querySelector('input[type=number]'),5);button(view.el,'返回列表').click();await nextTick();choices=[...view.el.querySelectorAll('button')].filter(x=>x.textContent.trim()==='选择');choices[1].click();await settle();assert.equal(view.el.querySelector('input[type=number]').value,'');await input(view.el.querySelector('input[type=number]'),0);button(view.el,'确认处理').click();await settle();assert.deepEqual(calls[0].sources,[{source_id:'shared',quantity_g:0}]);choices=[...view.el.querySelectorAll('button')].filter(x=>x.textContent.trim()==='选择');choices[0].click();await settle();assert.equal(view.el.querySelector('input[type=number]').value,'5')}finally{view.close()}
})

test('piece planned minutes track changed quantity until operator overrides',async()=>{
 const p={...structuredClone(preview),execution:{reporting_type:'piece'},material_consumption_policy:'bom_start_v1',planned_qty:50,total_planned_qty:100,completed_output_qty:50,planned_minutes:21,input_unit:'袋',output_unit:'袋'}
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:108},{apiGet:async()=>p,apiSend:async()=>({})})
 try{const fields=view.el.querySelectorAll('fieldset input');assert.equal(fields[1].value,'10');await input(fields[0],25);assert.equal(fields[1].value,'5');await input(fields[1],7);await input(fields[0],20);assert.equal(fields[1].value,'7')}finally{view.close();clearWorkstationFormCache('completion',108)}
})

test('remainder panel empty list and unfinished tasks cannot submit',async()=>{
 const empty=await mount('../components/PackagingRemainderPanel.vue',{}, {apiGet:async path=>path==='/api/stock/warehouses'?{rows:[]}:{groups:[]},apiSend:async()=>{throw new Error('must not submit')}})
 try{assert.match(empty.el.textContent,/暂无待处理/)}finally{empty.close()}
 const api={apiGet:async path=>path==='/api/packaging-remainders'?{groups:[{plan_id:1,product_id:2,product_name:'曲奇',total_tasks:2,completed_tasks:1}]}:path==='/api/stock/warehouses'?{rows:[]}:{plan_id:1,product_id:2,can_confirm:false,version:'v1',tasks:[{job_card_id:1,status:'running',work_order_no:'WO1'}],sources:[{source_id:'s1',available_g:2}]},apiSend:async()=>{throw new Error('must not submit')}}
 const view=await mount('../components/PackagingRemainderPanel.vue',{},api)
 try{button(view.el,'选择').click();await settle();assert.match(view.el.textContent,/还有包装任务未完成/);assert.equal(button(view.el,'确认处理').disabled,true);assert.equal(view.el.querySelector('input[type=number]').disabled,true)}finally{view.close()}
})
test('remainder conflict preserves input across recheck',async()=>{
 let rev=1
 const api={apiGet:async path=>path==='/api/packaging-remainders'?{groups:[{plan_id:1,product_id:2,product_name:'曲奇'}]}:path==='/api/stock/warehouses'?{rows:[]}:{plan_id:1,product_id:2,can_confirm:true,version:`v${rev++}`,tasks:[],sources:[{source_id:'s1',batch_code:'MP',available_g:10}]},apiSend:async()=>{throw Object.assign(new Error('库存变化'),{status:409})}}
 const view=await mount('../components/PackagingRemainderPanel.vue',{},api)
 try{button(view.el,'选择').click();await settle();await input(view.el.querySelector('input[type=number]'),5);button(view.el,'确认处理').click();await settle();assert.match(view.el.textContent,/库存变化/);assert.equal(button(view.el,'确认处理').disabled,true);button(view.el,'重新核对').click();await settle();assert.equal(view.el.querySelector('input[type=number]').value,'5');assert.equal(button(view.el,'确认处理').disabled,false)}finally{view.close()}
})

test('remainder displays the specific shared-batch blocker and disallows confirmation',async()=>{
 const api={apiGet:async path=>path==='/api/packaging-remainders'?{groups:[{plan_id:1,product_id:2,product_name:'曲奇'}]}:path==='/api/stock/warehouses'?{rows:[]}:{plan_id:1,product_id:2,can_confirm:false,blocked_reason:'该批次还关联其他计划，余料归属待核对',version:'v1',tasks:[],sources:[{source_id:'s1',available_g:2}]},apiSend:async()=>{throw new Error('must not submit')}}
 const view=await mount('../components/PackagingRemainderPanel.vue',{},api)
 try{button(view.el,'选择').click();await settle();assert.match(view.el.textContent,/该批次还关联其他计划，余料归属待核对/);assert.doesNotMatch(view.el.textContent,/还有包装任务未完成/);assert.equal(button(view.el,'确认处理').disabled,true)}finally{view.close()}
})

test('explicit remainder recheck preserves inputs and rereads stock after external movement',async()=>{
 let stale=false;let checks=0
 const api={apiGet:async path=>path==='/api/packaging-remainders'?{groups:[{plan_id:1,product_id:2,product_name:'曲奇'}]}:path==='/api/stock/warehouses'?{rows:[]}:{plan_id:1,product_id:2,product_name:'曲奇',can_confirm:!stale,recheck_required:stale,blocked_reason:stale?'来源批次有外部库存变化，请重新核对余料':'',version:stale?'v2':'v1',tasks:[],sources:[{source_id:'s1',batch_code:'MP',available_g:10}]},apiSend:async(path,{body})=>{if(path.endsWith('/recheck')){checks++;assert.equal(body.expected_version,'v2');stale=false;return {plan_id:1,product_id:2,product_name:'曲奇',can_confirm:true,recheck_required:false,reweigh_required:true,version:'v3',tasks:[],sources:[{source_id:'s1',batch_code:'MP',available_g:10}]}};stale=true;throw Object.assign(new Error('库存变化'),{status:409})}}
 const view=await mount('../components/PackagingRemainderPanel.vue',{},api)
 try{button(view.el,'选择').click();await settle();await input(view.el.querySelector('input[type=number]'),5);button(view.el,'确认处理').click();await settle();button(view.el,'重新核对').click();await settle();assert.match(view.el.textContent,/外部库存变化/);assert.equal(view.el.querySelector('input[type=number]').value,'5');button(view.el,'重新核对余料').click();await settle();assert.equal(checks,1);assert.match(view.el.textContent,/重新称重/);assert.equal(view.el.querySelector('input[type=number]').value,'5');assert.equal(button(view.el,'确认处理').disabled,true);await input(view.el.querySelector('input[type=number]'),5);assert.equal(button(view.el,'确认处理').disabled,false)}finally{view.close()}
})

test('failed explicit remainder recheck keeps entered numbers',async()=>{
 const api={apiGet:async path=>path==='/api/packaging-remainders'?{groups:[{plan_id:1,product_id:2,product_name:'曲奇'}]}:path==='/api/stock/warehouses'?{rows:[]}:{plan_id:1,product_id:2,can_confirm:false,recheck_required:true,blocked_reason:'需要重新核对',version:'v1',tasks:[],sources:[{source_id:'s1',batch_code:'MP',available_g:10}]},apiSend:async()=>{throw Object.assign(new Error('库存再次变化'),{status:409})}}
 const view=await mount('../components/PackagingRemainderPanel.vue',{},api)
 try{button(view.el,'选择').click();await settle();const field=view.el.querySelector('input[type=number]');await input(field,6);button(view.el,'重新核对余料').click();await settle();assert.match(view.el.textContent,/库存再次变化/);assert.equal(field.value,'6');assert.ok(button(view.el,'重新核对'))}finally{view.close()}
})

test('persisted reweigh requirement survives reopening and is explicitly confirmed',async()=>{
 const calls=[];const api={apiGet:async path=>path==='/api/packaging-remainders'?{groups:[{plan_id:1,product_id:2,product_name:'曲奇'}]}:path==='/api/stock/warehouses'?{rows:[]}:{plan_id:1,product_id:2,can_confirm:true,reweigh_required:true,version:'v1',tasks:[],sources:[{source_id:'s1',batch_code:'MP',available_g:10}]},apiSend:async(path,{body})=>{calls.push(body);return {}}}
 const view=await mount('../components/PackagingRemainderPanel.vue',{},api)
 try{button(view.el,'选择').click();await settle();assert.match(view.el.textContent,/重新称重/);assert.equal(button(view.el,'确认处理').disabled,true);await input(view.el.querySelector('input[type=number]'),3);assert.equal(button(view.el,'确认处理').disabled,false);button(view.el,'确认处理').click();await settle();assert.equal(calls[0].reweigh_confirmed,true)}finally{view.close()}
})

test('rechecked remainder shows batch discrepancy as pending both before and after confirmation',async()=>{
 const api={apiGet:async path=>path==='/api/packaging-remainders'?{groups:[{plan_id:1,product_id:2,product_name:'曲奇'}]}:path==='/api/stock/warehouses'?{rows:[]}:{plan_id:1,product_id:2,can_confirm:true,version:'v1',tasks:[],sources:[{source_id:'s1',batch_code:'MP1',warehouse:'wip',owner_customer_id:0,available_g:5}],discrepancy_g:5,discrepancies:[{source_id:'s1',batch_code:'MP1',warehouse:'wip',owner_customer_id:0,quantity_g:5}]},apiSend:async()=>({outstanding_g:5,discrepancy_g:5})}
 const view=await mount('../components/PackagingRemainderPanel.vue',{},api)
 try{button(view.el,'选择').click();await settle();assert.match(view.el.textContent,/差额待核对/);assert.match(view.el.textContent,/MP1/);assert.match(view.el.textContent,/5 g/);await input(view.el.querySelector('input[type=number]'),5);button(view.el,'确认处理').click();await settle();assert.match(view.el.textContent,/差额.*5 g.*待核对/);assert.doesNotMatch(view.el.textContent,/全部完成/)}finally{view.close()}
})

test('discrepancy review requires selected source and reason and keeps input on conflict',async()=>{
 const calls=[];let fail=true
 const detail={plan_id:1,product_id:2,can_confirm:true,version:'v1',tasks:[],sources:[{source_id:'s1',batch_code:'MP1',warehouse:'wip',available_g:5}],discrepancy_g:3,discrepancies:[{id:42,source_id:'s1',batch_code:'MP1',warehouse:'wip',quantity_g:3}]}
 const api={apiGet:async path=>path==='/api/packaging-remainders'?{groups:[{plan_id:1,product_id:2,product_name:'曲奇'}]}:path==='/api/stock/warehouses'?{rows:[]}:detail,apiSend:async(path,{body})=>{calls.push({path,body});if(fail)throw Object.assign(new Error('版本变化'),{status:409});return {...detail,version:'v2',discrepancy_g:0,discrepancies:[],reweigh_required:true}}}
 const view=await mount('../components/PackagingRemainderPanel.vue',{},api)
 try{button(view.el,'选择').click();await settle();assert.equal(button(view.el,'确认核对差额').disabled,true);view.el.querySelector('input[type=checkbox]').click();await input(view.el.querySelector('textarea'),'仓库已盘点');button(view.el,'确认核对差额').click();await settle();assert.equal(view.el.querySelector('textarea').value,'仓库已盘点');assert.equal(view.el.querySelector('input[type=checkbox]').checked,true);assert.match(view.el.textContent,/版本变化/);button(view.el,'重新核对').click();await settle();fail=false;button(view.el,'确认核对差额').click();await settle();assert.deepEqual(calls.at(-1).body.discrepancy_ids,[42]);assert.equal(calls.at(-1).body.reason,'仓库已盘点');assert.match(view.el.textContent,/重新.*确认|重新称重/);assert.equal(button(view.el,'确认处理').disabled,true)}finally{view.close()}
})

test('explicitly ineligible historical packaging retains per-order remainder compatibility',async()=>{
 const calls=[];const p={...structuredClone(preview),execution:{reporting_type:'piece'},material_consumption_policy:'bom_start_v1',plan_remainder_eligible:false,input_qty:10,input_unit:'袋',output_unit:'袋',planned_qty:10,total_planned_qty:10,planned_minutes:18,packaging_available_g:4600,packaging_standard_g_per_piece:454}
 const api={apiGet:async path=>path==='/api/stock/warehouses'?{rows:[{code:'wip',name:'WIP在制仓'},{code:'raw',name:'原料仓'}]}:p,apiSend:async(path,{body})=>{calls.push(body);return {completion:{message:'完成'}}}}
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:110},api)
 try{assert.match(view.el.textContent,/历史工单未关联可识别生产计划/);assert.match(view.el.textContent,/实际余料/);assert.match(view.el.textContent,/已称重并确认余料/);view.el.querySelector('input[type=checkbox]').click();await nextTick();button(view.el,'确认工序完成').click();await settle();assert.equal(calls[0].actual_remainder_g,60);assert.equal(calls[0].remainder_warehouse,'wip');assert.equal('remainder_mode' in calls[0],false)}finally{view.close();clearWorkstationFormCache('completion',110)}
})
