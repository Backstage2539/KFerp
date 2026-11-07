import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
const filename = new URL('../components/OrderResolutionStep.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename,'utf8'),{filename:filename.pathname})
let code = compileScript(descriptor,{id:'resolution-step-runtime'}).content
code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,filename).href:import.meta.resolve(path))}`)
const {default:View}=await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
async function setup(permissions={}){
 let editor
 await renderToString(createSSRApp({...View,setup(p,c){editor=View.setup(p,c);return editor},render(){return h('div')}},{orderId:7,requestId:8,step:{id:9,kind:'funds',permission:'finance.resolve',status:'pending'},permissions,options:{funds:[{value:'refund',label:'退款'}]}}))
 return editor
}
test('funds form denies writes without finance permission',async()=>{const editor=await setup({'orders.resolve':true});assert.equal(editor.allowed.value,false);await editor.submit();assert.equal(editor.busy.value,false)})
test('failed manual refund keeps its content and idempotency key for safe retry',async t=>{
 const original=globalThis.fetch; t.after(()=>globalThis.fetch=original)
 const writes=[];globalThis.fetch=async(url,options)=>{writes.push(JSON.parse(options.body));return {ok:false,status:503,json:async()=>({error:'暂时无法保存'})}}
 const editor=await setup({'finance.resolve':true})
 Object.assign(editor.form,{choice:'refund',amount:'12.34',account:'explicit',reference:'bank-ref',posting_date:'2026-11-07',note:'核实实际付款'})
 await editor.submit();await editor.submit()
 assert.equal(writes.length,2);assert.deepEqual(writes[0],writes[1]);assert.equal(editor.form.amount,'12.34');assert.match(editor.error.value,/暂时无法保存/)
})
