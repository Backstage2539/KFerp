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
