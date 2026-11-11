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
const preview={reporting_type:'piece',material_consumption_policy:'bom_start_v1',version:'v1',remaining_qty:20,max_output_qty:20,output_unit:'袋',warehouse_name:'成品仓',items:[{job_card_id:1,work_order_no:'WO1',product_name:'咖啡',remaining_qty:10,planned_qty:10,completed_qty:0,unit:'袋',packaging_available_g:5540,packaging_standard_g_per_piece:454},{job_card_id:2,work_order_no:'WO2',product_name:'咖啡',remaining_qty:10,planned_qty:10,completed_qty:0,unit:'袋',packaging_available_g:5540,packaging_standard_g_per_piece:454}],materials:[]}
test('group requires separate confirmation and retries identical request after lost response',async()=>{
 let calls=[],lost=true
 const api={apiGet:async()=>({rows:[]}),apiSend:async(path,{body})=>{if(path.endsWith('/preview'))return structuredClone(preview);calls.push(structuredClone(body));if(lost){lost=false;throw new Error('network lost')}return {completed_qty:20,remaining_qty:0,output_unit:'袋',items:[]}}}
 const view=await mount('../components/WorkstationPackagingCompletionForm.vue',{taskIds:[1,2]},api)
 try {
  assert.match(view.el.textContent,/逐张工单称重余料/);const submit=[...view.el.querySelectorAll('button')].find(b=>b.textContent.includes('确认完成并入库'));assert.ok(submit.disabled)
  let checks=view.el.querySelectorAll('input[type=checkbox]');assert.equal(checks.length,2);checks[0].click();await nextTick();assert.ok(submit.disabled);checks[1].click();await nextTick();assert.equal(submit.disabled,false)
  submit.click();await settle();assert.equal(calls.length,1);assert.match(view.el.textContent,/尚未收到完成结果/);assert.equal(view.el.querySelector('fieldset').disabled,true)
  submit.click();await settle();assert.equal(calls.length,2);assert.deepEqual(calls[0],calls[1]);assert.deepEqual(calls[0].actual_remainders_g,{'1':1000,'2':1000});assert.equal(calls[0].finish_group,true)
 }finally{view.close()}
})
test('shared stock is opt-in and selection requires an exact available quantity',async()=>{
 const p=structuredClone(preview);p.items[0].packaging_unissued_candidates=[{batch_code:'B1',warehouse:'wip',owner_customer_id:0,available_g:1280}]
 const calls=[];const api={apiGet:async()=>({rows:[]}),apiSend:async(path,{body})=>{if(path.endsWith('/preview'))return p;calls.push(structuredClone(body));return {completed_qty:20,remaining_qty:0,output_unit:'袋',items:[]}}}
 const view=await mount('../components/WorkstationPackagingCompletionForm.vue',{taskIds:[1,2]},api)
 try{
  assert.match(view.el.textContent,/未领可用库存（共享）/)
  const boxes=view.el.querySelectorAll('input[type=checkbox]');assert.equal(boxes.length,3);boxes[0].click();boxes[1].click();boxes[2].click();await nextTick()
  const submit=[...view.el.querySelectorAll('button')].find(b=>b.textContent.includes('确认完成并入库'));assert.equal(submit.disabled,true)
  const selected=view.el.querySelector('input[max="1280"]');await input(selected,1280);assert.equal(submit.disabled,false);submit.click();await settle()
  assert.deepEqual(calls[0].selected_unissued_remainders['1'],[{batch_code:'B1',warehouse:'wip',owner_customer_id:0,quantity_g:1280}])
 }finally{view.close()}
})
