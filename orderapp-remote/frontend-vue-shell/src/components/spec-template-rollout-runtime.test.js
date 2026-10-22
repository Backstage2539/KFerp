import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import {parse,compileScript} from '@vue/compiler-sfc'
import {createSSRApp,h} from 'vue'
import {renderToString} from '@vue/server-renderer'
const file=new URL('./SpecTemplateRollout.vue',import.meta.url)
const {descriptor}=parse(fs.readFileSync(file,'utf8'),{filename:file.pathname})
let code=compileScript(descriptor,{id:'template-rollout'}).content
code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,p)=>`from ${JSON.stringify(p.startsWith('.')?new URL(p,file).href:import.meta.resolve(p))}`)
const {default:View}=await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
test('rollout preview defaults only ready rows; deselection and partial retry preserve expected versions',async()=>{
 let editor;const requests=[];const events=[]
 const original=globalThis.fetch
 globalThis.fetch=async(url,options={})=>{requests.push({url,options});const result=options.method==='POST'?{batch_id:'b',rows:[{bom_id:1,status:'failed',reason:'publish failed'}]}:{rows:[{bom_id:1,current_version_id:11,status:'ready'},{bom_id:2,current_version_id:22,status:'ready'},{bom_id:3,status:'blocked',reason:'草稿'},{bom_id:4,status:'already_updated'}]};return new Response(JSON.stringify(result),{status:200,headers:{'Content-Type':'application/json'}})}
 try {
 await renderToString(createSSRApp({setup(){editor=View.setup({versionId:9},{expose(){},emit:(...a)=>events.push(a)});return()=>h('div')}}))
 await editor.load();assert.deepEqual(editor.selected.value,[1,2]);editor.selected.value=[1];await editor.apply()
 const body=JSON.parse(requests.find(r=>r.options.method==='POST').options.body)
 assert.deepEqual(body,{targets:[{bom_id:1,expected_version_id:11}]})
 assert.equal(editor.rows.value.find(r=>r.bom_id===1).status,'failed');assert.equal(editor.rows.value.find(r=>r.bom_id===2).status,'ready');assert.deepEqual(editor.selected.value,[1])
 assert.ok(events.some(e=>e[0]==='updated'))
 }finally{globalThis.fetch=original}
})
