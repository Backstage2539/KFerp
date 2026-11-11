import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { parse, compileScript } from '@vue/compiler-sfc'

const dom = new JSDOM('<div id="app"></div>', {url:'http://localhost/app/'})
for (const key of ['window','document','Node','Element','HTMLElement','SVGElement','Event','MouseEvent','CustomEvent']) globalThis[key] = dom.window[key]
const {createApp,nextTick} = await import('vue')
const file = new URL('../components/SpecPricingGroupManager.vue', import.meta.url)
const {descriptor} = parse(fs.readFileSync(file,'utf8'), {filename:file.pathname})
let code = compileScript(descriptor,{id:file.pathname,inlineTemplate:true}).content
code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(path,file).href : import.meta.resolve(path))}`)
const Component = (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
const response = data => ({ok:true,status:200,json:async()=>data})
const options = {templates:[{id:1,name:'袋装',versions:[{id:11,version_no:'V1',variants:[{spec_key:'s1',name:'227g',inventory_unit:'袋'},{spec_key:'s2',name:'100g',inventory_unit:'袋'}]}]}],pricing_rules:[{id:21,name:'模板甲',active:true},{id:22,name:'模板乙',active:true}]}
const flush = async () => {for(let i=0;i<5;i++){await nextTick();await new Promise(resolve=>setImmediate(resolve))}}
const button = name => [...document.querySelectorAll('button')].find(row => row.textContent.trim() === name)
const change = (el, value) => {el.value = String(value);el.dispatchEvent(new Event('change',{bubbles:true}))}

test('创建规格计价组并按规格选择不同价格模板', async () => {
  const original = globalThis.fetch; let submitted
  globalThis.fetch = async (url, init={}) => {const path=String(url);if(path.includes('/auth/me'))return response({permissions:['products.write']});if(path.endsWith('/options'))return response(options);if(init.method==='POST'){submitted=JSON.parse(init.body);return response({group:{...submitted,id:9}})}return response({groups:[]})}
  const app=createApp(Component);try{app.mount(document.querySelector('#app'));await flush();button('新建计价组').click();await flush();const name=document.querySelector('.spec-group-body input:not([type=checkbox])');name.value='孟豆豆组';name.dispatchEvent(new Event('input',{bubbles:true}));change(document.querySelector('.spec-group-body label:nth-of-type(3) select'),1);await flush();const boxes=document.querySelectorAll('input[aria-label^="选择2"],input[aria-label="选择100g"]');assert.equal(boxes.length,2);for(const box of boxes){box.checked=true;box.dispatchEvent(new Event('change',{bubbles:true}))};await flush();change(document.querySelector('[aria-label="227g价格计算模板"]'),21);change(document.querySelector('[aria-label="100g价格计算模板"]'),22);await flush();assert.equal(document.querySelector('[aria-label="选择全部规格"]').checked,true);button('保存计价组').click();await flush();assert.equal(submitted.name,'孟豆豆组');assert.deepEqual(submitted.items.map(x=>[x.spec_key,x.pricing_rule_id]),[['s1',21],['s2',22]])}finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})

test('只读用户可看组但不能新增或保存', async () => {
 const original=globalThis.fetch;globalThis.fetch=async url=>{const path=String(url);if(path.includes('/auth/me'))return response({permissions:['costing.read']});if(path.endsWith('/options'))return response(options);return response({groups:[{id:1,name:'只读组',active:true,spec_template_id:1,spec_template_version_id:11,items:[{spec_key:'s1',inventory_unit:'袋',pricing_rule_id:21}]}]})};const app=createApp(Component);try{app.mount(document.querySelector('#app'));await flush();assert.equal(button('新建计价组'),undefined);button('查看').click();await flush();assert.equal(button('保存计价组'),undefined);assert.equal(document.querySelector('[aria-label="选择227g"]').disabled,true)}finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})

test('查看规格模板往返保留未保存的计价组', async () => {
 const original=globalThis.fetch;globalThis.fetch=async url=>{const path=String(url);if(path.includes('/auth/me'))return response({permissions:['products.write']});if(path.endsWith('/options'))return response(options);return response({groups:[]})}
 let navigation;const listener=event=>{navigation=event.detail};window.addEventListener('kferp:navigate-view',listener)
 let app=createApp(Component);try{app.mount(document.querySelector('#app'));await flush();button('新建计价组').click();await flush();const name=document.querySelector('.spec-group-body input:not([type=checkbox])');name.value='未保存的组';name.dispatchEvent(new Event('input',{bubbles:true}));change(document.querySelector('.spec-group-body label:nth-of-type(3) select'),1);await flush();button('查看规格模板').click();assert.equal(navigation.key,'productSpecTemplates');assert.equal(navigation.params.spec_template_id,1);assert.equal(navigation.returnNavigation.key,'productPriceManagement');app.unmount();document.body.innerHTML='<div id="app"></div>';window.history.replaceState({}, '', `?spec_pricing_group_return_key=${navigation.returnNavigation.params.spec_pricing_group_return_key}&price_management_tab=spec-pricing-groups`);app=createApp(Component,{viewParams:navigation.returnNavigation.params});app.mount(document.querySelector('#app'));await flush();assert.equal(document.querySelector('.spec-group-body input:not([type=checkbox])').value,'未保存的组');assert.doesNotMatch(window.location.search,/spec_pricing_group_return_key|price_management_tab/)}finally{app.unmount();window.removeEventListener('kferp:navigate-view',listener);globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})

test('管理员角色即使没有显式权限也能维护规格计价组', async () => {
 const original=globalThis.fetch;globalThis.fetch=async url=>{if(String(url).includes('/auth/me'))return response({roles:[{code:'admin'}],permissions:[]});if(String(url).endsWith('/options'))return response(options);return response({groups:[]})};const app=createApp(Component);try{app.mount(document.querySelector('#app'));await flush();assert.ok(button('新建计价组'))}finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})

test('组引用的价格模板失效后仍可停用原组',async()=>{
 const original=globalThis.fetch;let payload
 const stale={id:8,name:'失效规则组',revision:2,active:true,spec_template_id:1,spec_template_version_id:11,items:[{spec_key:'s1',inventory_unit:'袋',pricing_rule_id:999,sort_order:1}]}
 globalThis.fetch=async(url,init={})=>{const path=String(url);if(path.includes('/auth/me'))return response({permissions:['products.write']});if(path.endsWith('/options'))return response(options);if(init.method==='PUT'){payload=JSON.parse(init.body);return response({group:payload})}return response({groups:[stale]})}
 const app=createApp(Component);try{app.mount(document.querySelector('#app'));await flush();button('编辑').click();await flush();const enabled=document.querySelector('.spec-group-check input');enabled.checked=false;enabled.dispatchEvent(new Event('change',{bubbles:true}));await flush();button('保存计价组').click();await flush();assert.equal(payload.active,false);assert.equal(payload.items[0].pricing_rule_id,999)}finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})
