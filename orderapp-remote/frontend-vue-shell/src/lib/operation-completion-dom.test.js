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
const button=(el,label)=>[...el.querySelectorAll('button')].find(b=>b.textContent.trim()===label)
test('completion DOM validates integer, saves draft without movement, retries original lost response',async()=>{
 const calls=[];let lost=true;let p=structuredClone(preview)
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:1},{apiGet:async()=>structuredClone(p),apiSend:async(path,{body})=>{calls.push({path,body:structuredClone(body)});if(path.endsWith('/progress')){p={...p,version:'v2',draft:body};return structuredClone(p)};if(lost){lost=false;throw new Error('response lost')};return {completion:{message:'已入成品仓',output_qty:28,output_unit:'袋',stock_entry_ids:[8]}}}})
 try{
  assert.match(view.el.textContent,/227g袋装/);assert.match(view.el.textContent,/成品仓/)
  const fields=view.el.querySelectorAll('.fields input')
  await input(fields[0],1.5);button(view.el,'完成').click();await settle();assert.match(view.el.textContent,/整数/);assert.equal(calls.length,0)
  await input(fields[0],18);button(view.el,'保存进度').click();await settle();assert.equal(calls.length,1);assert.ok(calls[0].path.endsWith('/progress'));assert.match(view.el.textContent,/库存未改变/)
  await input(fields[0],28);await input(fields[3],'现场实际多装8袋');button(view.el,'完成').click();await settle();assert.match(view.el.textContent,/尚未收到完成结果/)
  assert.equal(view.el.querySelector('fieldset').disabled,true)
  button(view.el,'重试原次完成').click();await settle();assert.deepEqual(calls[1].body,calls[2].body);assert.equal(calls[2].body.expected_version,'v2');assert.match(view.el.textContent,/已入成品仓/)
 }finally{view.close()}
})
test('refresh preserves entered actual quantities and reason',async()=>{
 let revision='v1'
 const view=await mount('../components/OperationCompletionForm.vue',{jobCardId:1},{apiGet:async()=>({...structuredClone(preview),version:revision}),apiSend:async()=>{throw new Error('unexpected write')}})
 try{const fields=view.el.querySelectorAll('.fields input');await input(fields[0],18);await input(fields[3],'现场已做完');revision='v2';button(view.el,'刷新建议').click();await settle();assert.equal(fields[0].value,'18');assert.equal(fields[3].value,'现场已做完')}finally{view.close()}
})
