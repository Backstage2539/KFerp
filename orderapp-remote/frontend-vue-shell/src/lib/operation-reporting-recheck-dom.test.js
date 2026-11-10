import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {parse,compileScript} from '@vue/compiler-sfc'
import {JSDOM} from 'jsdom'
const dom=new JSDOM('<html><body></body></html>',{url:'http://localhost/'})
for(const key of ['window','document','HTMLElement','Element','SVGElement','Node','Event'])globalThis[key]=dom.window[key]
const {createApp,nextTick}=await import('vue')
const source=new URL('../components/OperationReportingRecheck.vue',import.meta.url)
const {descriptor}=parse(readFileSync(source,'utf8'))
let code=compileScript(descriptor,{id:'recheck-test',inlineTemplate:true}).content
code=code.replace(/import \{apiGet, apiSend\} from ['"][^'"]+['"]/,`const {apiGet,apiSend}=globalThis.testRecheckAPI`)
code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,source).href:import.meta.resolve(path))}`)
globalThis.testRecheckAPI={apiGet:(...args)=>globalThis.recheckGet(...args),apiSend:(...args)=>globalThis.recheckSend(...args)}
const component=(await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
const settle=async()=>{await new Promise(resolve=>setImmediate(resolve));await nextTick()}
test('recheck requires preview before confirmation and sends preview version',async()=>{
 const requests=[];globalThis.recheckGet=async path=>{requests.push(['GET',path]);return {version:'v1',changes:[{seq:1,operation:'色选',from:'legacy',to:'incoming'}]}};globalThis.recheckSend=async(path,{body})=>{requests.push(['POST',path,body]);return {}}
 const el=document.createElement('div');document.body.appendChild(el);const app=createApp(component,{kind:'work_order',id:8});app.mount(el)
 try{el.querySelector('button').click();await settle();assert.match(el.textContent,/色选.*原规则.*来料加工/s);assert.equal(requests.length,1);[...el.querySelectorAll('button')].find(b=>b.textContent.includes('确认采用')).click();await settle();assert.deepEqual(requests[1],['POST','/api/work-orders/8/reporting-recheck',{version:'v1'}])}finally{app.unmount();el.remove();delete globalThis.testRecheckAPI}
})
