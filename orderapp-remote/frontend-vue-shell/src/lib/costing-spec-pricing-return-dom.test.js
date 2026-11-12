import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { parse, compileScript } from '@vue/compiler-sfc'
import { saveSpecPricingGroupReturnDraft } from './spec-pricing-groups.js'
const dom=new JSDOM('<div id="app"></div>',{url:'http://localhost/app/?view=costing'})
for(const key of ['window','document','Node','Element','HTMLElement','SVGElement','Event','MouseEvent','CustomEvent'])globalThis[key]=dom.window[key]
const Vue=await import('vue')
const file=new URL('../components/SpecPricingGroupManager.vue',import.meta.url)
const {descriptor}=parse(fs.readFileSync(file,'utf8'),{filename:file.pathname})
let code=compileScript(descriptor,{id:file.pathname,inlineTemplate:true}).content
code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(path,file).href:import.meta.resolve(path))}`)
const Manager=(await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
const source=fs.readFileSync(new URL('../views/CostingView.vue',import.meta.url),'utf8')
const watcher=source.split('\n').find(line=>line.includes('watch(() => props.viewParams?.spec_pricing_group_return_key'))
assert.ok(watcher,'价格表必须监听规格计价组返回')
const startWatcher=new Function('watch','props','specPricingGroupManagerOpen',watcher)
const response=data=>({ok:true,status:200,json:async()=>data})
const flush=async()=>{for(let i=0;i<5;i++){await Vue.nextTick();await new Promise(resolve=>setImmediate(resolve))}}

test('从规格模板返回价格表时自动打开管理抽屉并恢复未保存组配置',async()=>{
 const oldFetch=globalThis.fetch
 globalThis.fetch=async url=>String(url).includes('/auth/me')?response({permissions:['products.write']}):String(url).includes('/options')?response({templates:[],pricing_rules:[]}):response({groups:[]})
 const key=saveSpecPricingGroupReturnDraft({id:0,name:'待保存的计价组',spec_template_id:3,spec_template_version_id:11,active:true,items:[]})
 window.history.replaceState({},'',`?view=costing&spec_pricing_group_return_key=${key}`)
 const props=Vue.reactive({viewParams:{spec_pricing_group_return_key:key}})
 const Host={setup(){const opened=Vue.ref(false);startWatcher(Vue.watch,props,opened);return()=>opened.value?Vue.h(Manager,{viewParams:props.viewParams,returnView:'costing'}):Vue.h('div','已关闭')}}
 const app=Vue.createApp(Host)
 try{app.mount(document.querySelector('#app'));await flush();const drawer=document.querySelector('[aria-label="规格计价组编辑"]');assert.ok(drawer);assert.equal(drawer.querySelector('input:not([type=checkbox])').value,'待保存的计价组');assert.doesNotMatch(window.location.search,/spec_pricing_group_return_key/)}finally{app.unmount();globalThis.fetch=oldFetch;document.body.innerHTML='<div id="app"></div>'}
})
