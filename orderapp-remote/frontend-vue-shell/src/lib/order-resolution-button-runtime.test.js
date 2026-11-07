import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
const filename = new URL('../components/OrderResolutionButton.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename,'utf8'),{filename:filename.pathname})
let code = compileScript(descriptor,{id:'resolution-button-runtime'}).content
code=code.replace(/^import (OrderResolution(?:Step|Policy|Balance)) from .+$/gm,'const $1 = {}')
code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,filename).href:import.meta.resolve(path))}`)
const {default:View}=await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
async function setup(){let editor;await renderToString(createSSRApp({...View,setup(p,c){editor=View.setup(p,c);return editor},render(){return h('div')}},{orderId:7}));return editor}
test('pending resolution refresh keeps explicit selections and leaves new dependencies unselected',async t=>{
 const previous=globalThis.fetch;t.after(()=>globalThis.fetch=previous)
 const writes=[]
 const data={context:{order_no:'SO-7',version:'latest',lines:[{id:71}],dependencies:[{kind:'production',id:3,status:'completed'},{kind:'fee',id:9,status:'active'}]},options:{},permissions:{'orders.resolve':true},history:[{id:10,status:'pending',applied:false,command:{action:'close',reason:'customer_cancel',scope:'all',dispositions:[{kind:'production',id:3,choice:'keep_company_stock'}]}}]}
 globalThis.fetch=async(url,options)=>{
  if(!options.method)return {ok:true,json:async()=>data}
  writes.push({url,body:JSON.parse(options.body)});return {ok:true,json:async()=>({status:'pending'})}
 }
 const editor=await setup();await editor.load(true)
 assert.equal(editor.form.action,'close');assert.equal(editor.form.dispositions[0].choice,'keep_company_stock');assert.equal(editor.form.dispositions[1].choice,'')
 editor.preview.value={version:'latest',plan:{}}
 await editor.submit()
 assert.match(writes[0].url,/resolution\/10\/refresh$/);assert.equal(writes[0].body.version,'latest')
})

test('related order navigation carries an in-memory return to the original order',async t=>{
 const previous=globalThis.window;t.after(()=>globalThis.window=previous)
 let event;globalThis.window={dispatchEvent:value=>{event=value}}
 const editor=await setup();editor.relatedOrder({id:9})
 assert.equal(event.type,'kferp:navigate-view');assert.deepEqual(event.detail,{key:'orders',params:{order_id:9},returnNavigation:{key:'orders',label:'返回原订单',params:{order_id:7}}})
})

test('replacement supply starts with blank warehouse choices and explicit quantities',async t=>{
 const previous=globalThis.fetch;t.after(()=>globalThis.fetch=previous)
 const source={id:31,quantity:3,label:'source package',targets:[{id:41,label:'matching purchase',available_qty:5}]}
 const writes=[]
 globalThis.fetch=async(url,options={})=>{if(options.method)writes.push({url,body:JSON.parse(options.body)});return {ok:true,json:async()=>({context:{version:'v1',lines:[],dependencies:[{kind:'direct_ship',id:16,relation:'supply',status:'reserved',supply_reservations:[source]}]},options:{direct_ship:[{value:'replace_supply',label:'换来源'}]},permissions:{'orders.resolve':true},history:[]})}}
 const editor=await setup();await editor.load(true)
 const choice=editor.form.dispositions[0]
 assert.equal(choice.choice,'')
 assert.deepEqual(JSON.parse(JSON.stringify(choice.supplies)),[{reservation_id:31,target_id:0,quantity:''}])
 choice.choice='replace_supply'
 assert.throws(()=>editor.body(),/替代货源|数量/)
 await editor.saveDraft()
 assert.equal(editor.error.value,'')
 assert.match(writes[0].url,/resolution\/draft$/)
 assert.deepEqual(writes[0].body.dispositions[0].supplies,[{reservation_id:31,target_id:0,quantity:0}])
 choice.supplies[0].target_id=41;choice.supplies[0].quantity='3'
 assert.deepEqual(editor.body().dispositions[0].supplies,[{reservation_id:31,target_id:41,quantity:3}])
})

test('confirmed paid fees allow explicit negotiated credit without choosing a refund',async t=>{
 const previous=globalThis.fetch;t.after(()=>globalThis.fetch=previous)
 const fee={kind:'fee',id:91,status:'settled',amount_cents:1000,paid_cents:1000}
 globalThis.fetch=async()=>({ok:true,json:async()=>({context:{version:'paid-fee',lines:[],dependencies:[fee]},options:{fee:[{value:'keep'},{value:'partial_waiver'},{value:'waive'}]},permissions:{'orders.resolve':true},history:[]})})
 const editor=await setup();await editor.load(true)
 assert.equal(editor.form.dispositions[0].choice,'')
 assert.equal(editor.choices(fee).find(v=>v.value==='partial_waiver').disabled,false)
 assert.equal(editor.form.dispositions.some(v=>v.kind==='funds'),false)
})

test('mixed supply requires a blank physical retention choice and preserves its version in payload',async t=>{
 const previous=globalThis.fetch;t.after(()=>globalThis.fetch=previous)
 const source={id:31,quantity:2,converted_quantity:1,physical_version:'physical-v1',targets:[{id:41,available_qty:5}]}
 globalThis.fetch=async()=>({ok:true,json:async()=>({context:{version:'v1',lines:[],dependencies:[{kind:'direct_ship',id:16,relation:'supply',status:'reserved',supply_reservations:[source]}]},options:{direct_ship:[{value:'replace_supply'}]},permissions:{'orders.resolve':true},history:[]})})
 const editor=await setup();await editor.load(true)
 const d=editor.form.dispositions[0];d.choice='replace_supply'
 assert.equal(d.supplies[0].physical_choice,'')
 d.supplies[0].target_id=41;d.supplies[0].quantity='2'
 assert.throws(()=>editor.body(),/已入库|实物/)
 d.supplies[0].physical_choice='keep_reserved'
 assert.deepEqual(editor.body().dispositions[0].supplies,[{reservation_id:31,target_id:41,quantity:2,physical_choice:'keep_reserved',physical_version:'physical-v1'}])
})

test('fully converted reservation requires retention but no replacement target or quantity',async t=>{
 const previous=globalThis.fetch;t.after(()=>globalThis.fetch=previous)
 const dep={kind:'direct_ship',id:16,relation:'supply',status:'reserved',supply_reservations:[{id:31,quantity:0,converted_quantity:3,physical_version:'full-v1',targets:[]},{id:32,quantity:2,targets:[{id:41,available_qty:5}]}]}
 globalThis.fetch=async()=>({ok:true,json:async()=>({context:{version:'v1',lines:[],dependencies:[dep]},options:{direct_ship:[{value:'replace_supply'}]},permissions:{'orders.resolve':true},history:[]})})
 const editor=await setup();await editor.load(true)
 assert.equal(editor.choices(dep)[0].disabled,false)
 const d=editor.form.dispositions[0];d.choice='replace_supply';d.supplies[1].target_id=41;d.supplies[1].quantity='2'
 assert.throws(()=>editor.body(),/已入库|实物/)
 d.supplies[0].physical_choice='keep_reserved'
 assert.deepEqual(editor.body().dispositions[0].supplies[0],{reservation_id:31,target_id:0,quantity:0,physical_choice:'keep_reserved',physical_version:'full-v1'})
})

test('fee-only scope explicitly excludes fulfillment choices and keeps cash dispositions blank',async t=>{
 const previous=globalThis.fetch;t.after(()=>globalThis.fetch=previous)
 const production={kind:'production',id:19,status:'completed',shared:true}, fee={kind:'fee',id:91,status:'settled',amount_cents:500,paid_cents:1000,settled_out_cents:500}
 globalThis.fetch=async()=>({ok:true,json:async()=>({context:{version:'closed-fee',status:'closed',lines:[{id:3,closed:2}],dependencies:[production,fee]},options:{scope:[{value:'all'},{value:'fees_only'}],timing:[{value:'stop_first'},{value:'settle_first'}],fee:[{value:'keep'},{value:'waive'}]},permissions:{'orders.resolve':true},history:[]})})
 const editor=await setup();await editor.load(true)
 assert.equal(editor.form.scope,'');editor.form.action='credit';editor.form.scope='fees_only'
 assert.deepEqual(editor.visibleDependencies.value.map(v=>v.kind),['fee'])
 assert.deepEqual(editor.timingOptions.value.map(v=>v.value),['settle_first'])
 assert.equal(editor.form.timing,'');assert.equal(editor.form.dispositions[1].choice,'')
 editor.form.dispositions[1].choice='waive'
 assert.deepEqual(editor.body().dispositions.map(v=>v.kind),['fee'])
 assert.equal(editor.body().lines.length,0)
 const unknown={...fee,paid_cents:0,cash_issue:'历史结算尚无可核对收款'}
 assert.equal(editor.choices(unknown).find(v=>v.value==='waive').disabled,true)
 assert.equal(editor.choices({...fee,amount_cents:-100}).find(v=>v.value==='waive').disabled,true)
})
