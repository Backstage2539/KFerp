import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
const filename = new URL('../components/OrderResolutionPolicy.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename,'utf8'),{filename:filename.pathname})
let code = compileScript(descriptor,{id:'resolution-policy-runtime'}).content
code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,filename).href:import.meta.resolve(path))}`)
const {default:View}=await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
async function setup(){
 let editor
 await renderToString(createSSRApp({...View,setup(p,c){editor=View.setup(p,c);return editor},render(){return h('div')}}))
 return editor
}
test('policy editor loads saved choices and submits the observed revision',async t=>{
 const previous=globalThis.fetch;t.after(()=>globalThis.fetch=previous)
 const writes=[]
 globalThis.fetch=async(url,options)=>{
  if(!options.method||options.method==='GET')return {ok:true,json:async()=>({revision:3,policy:{disabled:{action:['cancel'],scrap_cost:['reclassify_loss']},approvals:{close:'finance.resolve'}},options:{action:[{value:'close'},{value:'cancel'}]}})}
  writes.push(JSON.parse(options.body));return {ok:false,status:409,json:async()=>({error:'配置已变化'})}
 }
 const editor=await setup();await editor.load()
 assert.deepEqual(editor.form.disabled.action,['cancel']);assert.equal(editor.form.approvals.close,'finance.resolve')
 await editor.save();assert.equal(writes[0].revision,3);assert.deepEqual(writes[0].policy.disabled.action,['cancel']);assert.deepEqual(writes[0].policy.disabled.scrap_cost,['reclassify_loss']);assert.match(editor.error.value,/配置已变化/)
})
