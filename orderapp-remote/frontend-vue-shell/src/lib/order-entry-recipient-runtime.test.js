import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import {parse,compileScript} from '@vue/compiler-sfc'
import {createSSRApp,h,nextTick} from 'vue'
import {renderToString} from '@vue/server-renderer'
import {buildOrderPayload} from './order-entry.js'
const filename=new URL('../views/OrderEntryView.vue',import.meta.url)
const {descriptor}=parse(fs.readFileSync(filename,'utf8'),{filename:filename.pathname})
let code=compileScript(descriptor,{id:'order-entry-runtime'}).content
code=code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g,(_,name)=>`const ${name}={render:()=>null}`)
code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,filename).href:import.meta.resolve(path))}`)
const {default:View}=await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
test('real Vue editor loads recipient snapshot and does not leak previous order fields',async()=>{
 let editor
 const wrapper={setup(){editor=View.setup({embedded:true,editId:0,copyId:0,customerPortal:false,fulfillmentMode:false,viewParams:{}},{expose(){},emit(){}});return()=>h('div')}}
 await renderToString(createSSRApp(wrapper))
 editor.customers.value=[{id:8,name:'样品客户'}]
 const first={edit_id:12,customer_id:8,is_sample_order:true,receiver_name:'收件人',receiver_phone:'13800000000',receiver_address:'确认的寄样地址',receiver_company:'寄样公司',items:[]}
 editor.applyEditData(first)
 editor.form.notes='仅修改备注'
 let payload=buildOrderPayload({form:editor.form,rows:editor.rows.value})
 for(const key of ['receiver_name','receiver_phone','receiver_address','receiver_company']) assert.equal(payload[key],first[key])
 assert.equal(payload.portal_service_code,undefined)
 editor.applyEditData({...first,edit_id:13,receiver_name:'另一个人',receiver_address:'另一地址',receiver_company:'',receiver_phone:''})
 payload=buildOrderPayload({form:editor.form,rows:editor.rows.value})
 assert.equal(payload.receiver_name,'另一个人')
 assert.equal(payload.receiver_company,'')
 assert.equal(payload.receiver_phone,'')
 await nextTick()
})
