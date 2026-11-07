import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
const filename = new URL('../components/OrderResolutionBalance.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename,'utf8'),{filename:filename.pathname})
let code = compileScript(descriptor,{id:'resolution-balance-runtime'}).content
code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,filename).href:import.meta.resolve(path))}`)
const {default:View}=await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
async function setup(permissions){let editor;await renderToString(createSSRApp({...View,setup(p,c){editor=View.setup(p,c);return editor},render(){return h('div')}},{orderId:7,balances:[{id:3,available_cents:2000}],targets:[{id:9,available_cents:3000}],permissions,options:{funds:[{value:'offset'},{value:'refund'},{value:'customer_credit'}]}}));return editor}
test('balance form requires explicit source and finance role, and submits observed balances',async t=>{
 const previous=globalThis.fetch;t.after(()=>globalThis.fetch=previous)
 const writes=[];globalThis.fetch=async(url,options)=>{writes.push(JSON.parse(options.body));return {ok:false,status:409,json:async()=>({error:'余额已变化'})}}
 const denied=await setup({'orders.resolve':true});await denied.submit();assert.equal(writes.length,0)
 const editor=await setup({'finance.resolve':true});assert.equal(editor.sourceID.value,0);assert.equal(editor.form.choice,'')
 Object.assign(editor.form,{choice:'offset',amount:'10',target_id:9,note:'指定应收',posting_date:'2026-11-07'});await editor.submit();assert.equal(writes.length,0)
 editor.sourceID.value=3;await editor.submit();await editor.submit()
 assert.equal(writes[0].source_entry_id,3);assert.equal(writes[0].expected_available_cents,2000);assert.equal(writes[0].expected_target_due_cents,3000);assert.deepEqual(writes[0],writes[1]);assert.match(editor.error.value,/余额已变化/)
 assert.equal(editor.choices.value.some(v=>v.value==='customer_credit'),false)
})
