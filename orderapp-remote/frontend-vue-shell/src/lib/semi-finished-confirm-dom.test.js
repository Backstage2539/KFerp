import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { parse, compileScript } from '@vue/compiler-sfc'

const dom=new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>',{url:'http://localhost/app/vue-shell'})
for(const name of ['window','document','Node','Element','HTMLElement','SVGElement','MutationObserver','Event']) globalThis[name]=dom.window[name]
const Vue=await import('vue')
const file=new URL('../components/ProductionPlanSemiFinishedStock.vue',import.meta.url)
async function component(){
 const {descriptor}=parse(fs.readFileSync(file,'utf8'),{filename:file.pathname}),compiled=compileScript(descriptor,{id:'semi-dom',inlineTemplate:true}),bindings={}
 for(const [,imports] of compiled.content.matchAll(/import\s+\{([^}]+)\}\s+from\s+"vue"/g))for(const part of imports.split(',')){const [imported,local]=part.trim().split(/\s+as\s+/);bindings[local||imported]=Vue[imported]}
 for(const b of Object.values(compiled.imports)){const mod=b.source==='vue'?Vue:await import(new URL(b.source.endsWith('.js')?b.source:`${b.source}.js`,file));bindings[b.local]=mod[b.imported]}
 return new Function(...Object.keys(bindings),compiled.content.replace(/^import\s+[\s\S]*?\s+from\s+(['"])[^'"]+\1;?\s*$/gm,'').replace('export default','return'))(...Object.values(bindings))
}
async function settle(){for(let i=0;i<5;i++){await new Promise(r=>setImmediate(r));await Vue.nextTick()}}
const button=label=>[...document.querySelectorAll('button')].find(e=>e.textContent.trim()===label)
const stock={draft_token:'v1',items:[{key:'material:1',name:'初晓-半成品',unit:'kg',required_qty:60.112,stock_covered_qty:15,inflight_covered_qty:0,manufacture_qty:45.112,batches:[{batch_id:3,batch_code:'MP-3',warehouse:'raw_materials',warehouse_name:'原料仓',available_qty:15,selected_qty:15}]}]}
const detail={id:26,status:'draft',draft_token:'v1',component_sources:[{id:3,component_name:'初晓-半成品',allocation_mode:'semi_finished_selected',unit:'kg',required_g:15000,transfer_g:15000,allocations:[{warehouse:'raw_materials',warehouse_name:'原料仓',batches:[{batch_id:3,batch_code:'MP-3',qty_g:15000}]}]}]}

test('actual drawer confirms in one click and transfer action stays on capacity page',async()=>{
 const calls=[];globalThis.fetch=async(url,options={})=>{let path=String(url);if(!options.method||options.method==='GET')return new Response(JSON.stringify(stock),{status:200,headers:{'content-type':'application/json'}});const body=JSON.parse(options.body);calls.push({path,body});return new Response(JSON.stringify(path.endsWith('/transfer-to-wip')?{plan:{...detail,draft_token:'v2'}}:body.check_only?{preview_token:'p',changes:[]}:{plan:{...detail,draft_token:'v2'}}),{status:200,headers:{'content-type':'application/json'}})}
 const Component=await component(),app=Vue.createApp(Component,{detail,allowTransfer:true});app.mount('#app')
 try{await settle();assert.match(document.body.textContent,/60\.112 kg/);assert.match(document.body.textContent,/45\.112 kg/);assert.ok(button('转入 WIP 仓'));button('修改用量').click();await settle();assert.ok(button('确认使用'));assert.equal(button('预览变化'),undefined);button('确认使用').click();await settle();assert.equal(calls.length,2);assert.deepEqual(calls[0].body.selections,calls[1].body.selections);assert.equal(document.querySelector('[role="dialog"]'),null);button('转入 WIP 仓').click();await settle();assert.match(calls.at(-1).path,/transfer-to-wip$/)}finally{app.unmount();document.body.innerHTML='<div id="app"></div>'}
})
