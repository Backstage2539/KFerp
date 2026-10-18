import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import {parse,compileScript} from '@vue/compiler-sfc'
import {createRenderer,h,reactive} from 'vue'
const filename=new URL('../components/OrderDetailsCopyButton.vue',import.meta.url)
const {descriptor}=parse(fs.readFileSync(filename,'utf8'),{filename:filename.pathname})
let code=compileScript(descriptor,{id:'order-copy-runtime'}).content
code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(path,filename).href:import.meta.resolve(path))}`)
const {default:Button}=await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
test('real copy button calls scoped API, presents manual fallback, clears it on customer switch',async()=>{
 const props=reactive({orderId:42,customerPortal:true,customerId:8});let button
 const renderer=createRenderer({createElement:()=>({children:[]}),insert:(child,parent)=>parent.children.push(child),remove:()=>{},patchProp:()=>{},createText:text=>({text}),createComment:()=>({}),setText:()=>{},setElementText:()=>{},parentNode:()=>null,nextSibling:()=>null})
 const app=renderer.createApp({setup(){button=Button.setup(props,{expose(){}});return()=>h('div')}});app.mount({children:[]})
 const original=globalThis.fetch;const paths=[]
 globalThis.fetch=async(url,options)=>{paths.push([url,options]);return {ok:true,json:async()=>({order_id:42,text:'订单号：SO-42'})}}
 try{await button.copy();assert.equal(button.manualText.value,'订单号：SO-42');assert.equal(button.busy.value,false);assert.match(paths[0][0],/customer-processing\/portal\/orders\/42\/copy-details\?customer_id=8/);assert.equal(paths[0][1].cache,'no-store');props.customerId=9;assert.equal(button.manualText.value,'')}finally{app.unmount();globalThis.fetch=original}
})
