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
