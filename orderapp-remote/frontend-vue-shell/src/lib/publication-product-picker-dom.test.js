import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { parse, compileScript } from '@vue/compiler-sfc'
const dom = new JSDOM('<div id="app"></div>', {url:'http://localhost/app/'})
for (const key of ['window','document','Node','Element','HTMLElement','SVGElement','Event','MouseEvent','CustomEvent']) globalThis[key] = dom.window[key]
const {createApp,nextTick} = await import('vue')
const file=new URL('../components/PublicationProductPicker.vue',import.meta.url)
const {descriptor}=parse(fs.readFileSync(file,'utf8'),{filename:file.pathname})
let code=compileScript(descriptor,{id:file.pathname,inlineTemplate:true}).content
code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(path,file).href:import.meta.resolve(path))}`)
const Component=(await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
const response=data=>({ok:true,status:200,json:async()=>data})
const flush=async()=>{for(let i=0;i<5;i++){await nextTick();await new Promise(resolve=>setImmediate(resolve))}}
const button=name=>[...document.querySelectorAll('button')].find(row=>row.textContent.trim()===name)
const source={id:7,table_name:'公共表',version:'V1',status:'published',owner_type:'official',publication_purpose:'factory_supply',list_type:'commercial',content:{price_rows:[{parent_product_id:3,product_name:'咖啡甲'},{parent_product_id:3,product_name:'咖啡甲'},{parent_product_id:4,product_name:'咖啡乙'}]}}

test('预览不写业务数据，确认后补客户引用并将商品交给新草稿', async()=>{
 const original=globalThis.fetch;const calls=[];let prepared
 globalThis.fetch=async(url,init={})=>{calls.push({url:String(url),init});if(String(url).includes('/auth/me'))return response({permissions:['products.write']});if(init.method==='POST')return response({created:2});return response(source)}
 const app=createApp(Component,{customerId:517,sources:[source],existingNames:['原表'],createDraft:async payload=>{prepared=payload}})
 try{app.mount(document.querySelector('#app'));await flush();const select=document.querySelector('select');select.value='7';select.dispatchEvent(new Event('change',{bubbles:true}));const name=document.querySelector('input:not([type=checkbox])');name.value='新表';name.dispatchEvent(new Event('input',{bubbles:true}));await flush();button('查看来源商品').click();await flush();assert.match(document.body.textContent,/已选 2 \/ 2 款/);assert.equal(calls.filter(c=>c.init.method==='POST').length,0);button('补齐客户商品并创建草稿').click();await flush();const post=calls.find(c=>c.init.method==='POST');assert.deepEqual(JSON.parse(post.init.body),{customer_id:517,mode:'selected',product_ids:[3,4]});assert.equal(prepared.tableName,'新表');assert.deepEqual(prepared.productIds,[3,4])}finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})

test('同名草稿不会写入客户商品',async()=>{
 const original=globalThis.fetch;const calls=[];globalThis.fetch=async(url,init={})=>{calls.push(init.method);return String(url).includes('/auth/me')?response({permissions:['products.write']}):response(source)}
 const app=createApp(Component,{customerId:517,sources:[source],existingNames:['原表'],createDraft:async()=>{throw Error('unexpected')}})
 try{app.mount(document.querySelector('#app'));await flush();const select=document.querySelector('select');select.value='7';select.dispatchEvent(new Event('change',{bubbles:true}));const name=document.querySelector('input:not([type=checkbox])');name.value='原表';name.dispatchEvent(new Event('input',{bubbles:true}));await flush();button('查看来源商品').click();await flush();button('补齐客户商品并创建草稿').click();await flush();assert.match(document.body.textContent,/已有同名价格表/);assert.ok(!calls.includes('POST'))}finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})

test('客户引用已补齐而草稿保存失败时显示明确提示，不报告完成',async()=>{
 const original=globalThis.fetch;let completed=false;globalThis.fetch=async(url,init={})=>{if(String(url).includes('/auth/me'))return response({permissions:['products.write']});if(init.method==='POST')return response({created:2});return response(source)}
 const app=createApp(Component,{customerId:517,sources:[source],createDraft:async()=>{throw new Error('网络异常')},onCreated:()=>{completed=true}})
 try{app.mount(document.querySelector('#app'));await flush();const select=document.querySelector('select');select.value='7';select.dispatchEvent(new Event('change',{bubbles:true}));const name=document.querySelector('input:not([type=checkbox])');name.value='新表';name.dispatchEvent(new Event('input',{bubbles:true}));await flush();button('查看来源商品').click();await flush();button('补齐客户商品并创建草稿').click();await flush();assert.match(document.body.textContent,/客户商品已补齐，但草稿未能保存：网络异常/);assert.equal(completed,false)}finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})

test('管理员角色可以补齐客户商品',async()=>{
 const original=globalThis.fetch;globalThis.fetch=async url=>String(url).includes('/auth/me')?response({roles:[{code:'admin'}],permissions:[]}):response(source)
 const app=createApp(Component,{customerId:517,sources:[source],createDraft:async()=>{}})
 try{app.mount(document.querySelector('#app'));await flush();assert.doesNotMatch(document.body.textContent,/没有客户商品编辑权限/)}finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})
