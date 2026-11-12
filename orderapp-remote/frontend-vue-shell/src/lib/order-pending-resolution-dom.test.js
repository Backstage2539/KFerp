import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'

const dom = new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>', { url: 'http://localhost/', pretendToBeVisual: true })
for (const key of ['window','document','Node','Element','HTMLElement','SVGElement','Event','MouseEvent','CustomEvent']) globalThis[key]=dom.window[key]
dom.window.HTMLElement.prototype.scrollIntoView=()=>{}
Object.defineProperty(window.navigator,'locks',{value:{request:async(_key,_opts,fn)=>fn({})},configurable:true})
const {createApp,nextTick}=await import('vue')
const filename=new URL('../views/OrderEntryView.vue',import.meta.url)
const {descriptor}=parse(fs.readFileSync(filename,'utf8'),{filename:filename.pathname})
let code=compileScript(descriptor,{id:'pending-resolution-dom',inlineTemplate:true}).content
code=code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g,(_,name)=>name==='OrderOperationNotes' ? `const ${name}={setup(_,ctx){ctx.expose({validate:async()=>true});return ()=>null}}` : `const ${name}={render:()=>null}`)
code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,filename).href:import.meta.resolve(path))}`)
const {default:View}=await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
const response=(value,status=200)=>({ok:status<400,status,json:async()=>value})
async function flush(){for(let i=0;i<6;i++){await nextTick();await new Promise(resolve=>setImmediate(resolve))}}
const button=text=>[...document.querySelectorAll('button')].find(node=>node.textContent.trim()===text)

test('预付款页面可编辑分项，合计不符不提交，修正后发送',async()=>{
 const script=compileScript(descriptor,{id:'receipt-editor-dom'})
 let js=script.content.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g,(_,name)=>name==='OrderFreightEstimate' ? `const ${name}={setup(_,ctx){ctx.expose({preview:async()=>({token:'freight-1'}),payload:()=>({expected_freight_token:'freight-1',shipping_amount:String(globalThis.receiptDomShippingAmount ?? '20')})});return ()=>null}}` : name==='OrderOperationNotes' ? `const ${name}={setup(_,ctx){ctx.expose({validate:async()=>true});return ()=>null}}` : `const ${name}={render:()=>null}`)
 js=js.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,filename).href:import.meta.resolve(path))}`)
 const {default:Editor}=await import(`data:text/javascript;base64,${Buffer.from(js).toString('base64')}`)
 let renderCode=compileTemplate({source:descriptor.template.content,filename:filename.pathname,id:'receipt-editor-dom',compilerOptions:{bindingMetadata:script.bindings}}).code
 renderCode=renderCode.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(import.meta.resolve(path))}`)
 Editor.render=(await import(`data:text/javascript;base64,${Buffer.from(renderCode).toString('base64')}`)).render
 const oldFetch=globalThis.fetch,writes=[]
 window.localStorage.setItem('auth_token','test-token')
 globalThis.fetch=async(url,options={})=>{
  const path=new URL(String(url),window.location.origin).pathname
  if(path==='/api/auth/me')return response({employee_id:11,permissions:['orders.write']})
  if(path==='/api/order/catalog')return response({customer_id:8,price_table_options:[],product_families:[]})
  if(path==='/api/order/stock-batch-preview')return response({sufficient:false})
  if(path==='/api/order' && options.method==='POST'){writes.push(JSON.parse(options.body));return response({order_id:12,order_no:'SO-TEST'})}
  return response({rows:[],groups:[],assignments:[]})
 }
 let app=createApp(Editor,{embedded:true,editId:0,copyId:0,customerPortal:false,fulfillmentMode:false,viewParams:{}})
 const input=(node,value)=>{assert.ok(node);node.value=value;node.dispatchEvent(new Event('input',{bubbles:true}))}
 try {
  app.mount('#app');await flush()
  const s=app._instance.setupState
  s.payStatuses=[{id:1,name:'未付款'},{id:2,name:'已付款'},{id:3,name:'预付款（付款未完成）'}]
  s.applyEditData({edit_id:12,edit_revision:'revision-1',customer_id:8,pay_status_id:1,shipping_amount:'20',freight:{token:'freight-1',current_receivable_cents:2000,charge_mode:'manual_current'},items:[{item_id:21,product_id:7,product_name:'测试商品',spec:'227g',qty:1,unit:'袋',unit_price:'120',manual_price:true,tier_id:'manual'}]})
  Object.assign(s.form,{edit_id:0,delivery_mode:'ship',payment_method:'现金',receipt_date:'2026-11-10'})
  s.freightEstimator={preview:async()=>({token:'freight-1'}),payload:()=>({expected_freight_token:'freight-1',shipping_amount:'20'})}
  await flush()
  input(document.querySelector('input[placeholder="填写实际已收金额"]'),'30');await flush()
  const goods=()=>document.querySelector('[data-error-field="payment_goods_amount"] input')
  const shipping=()=>document.querySelector('[data-error-field="payment_shipping_amount"] input')
  assert.ok(goods());assert.ok(shipping());input(goods(),'10');input(shipping(),'0');await flush()
  button('保存订单').click();await flush();assert.equal(writes.length,0);assert.match(document.body.textContent,/分项合计必须等于/)
  assert.equal(window.localStorage.getItem('kferp.order-create.pending.employee-11'),null)
  input(goods(),'30');button('保存订单').click();await flush()
  assert.equal(writes.length,1,document.body.textContent);assert.equal(writes[0].payment_goods_amount,'30');assert.equal(writes[0].payment_shipping_amount,'0')
  app.unmount();document.body.innerHTML='<div id="app"></div>';window.localStorage.clear();window.localStorage.setItem('auth_token','test-token')
  app=createApp(Editor,{embedded:true,editId:0,copyId:0,customerPortal:false,fulfillmentMode:false,viewParams:{}});app.mount('#app');await flush()
  const full=app._instance.setupState
  full.payStatuses=[{id:1,name:'未付款'},{id:2,name:'已付款'},{id:3,name:'预付款（付款未完成）'}]
  full.applyEditData({edit_id:12,edit_revision:'revision-1',customer_id:8,pay_status_id:2,shipping_amount:'20',freight:{token:'freight-1',current_receivable_cents:2000,charge_mode:'manual_current'},items:[{item_id:21,product_id:7,product_name:'测试商品',spec:'227g',qty:1,unit:'袋',unit_price:'120',manual_price:true,tier_id:'manual'}]})
  Object.assign(full.form,{edit_id:0,delivery_mode:'ship',pay_status_id:2,payment_method:'现金',payment_voucher_asset_id:71,prepayment_amount:'0',receipt_date:'2026-11-10'})
  await flush();input(goods(),'20');input(shipping(),'0');await flush();button('保存订单').click();await flush()
  assert.equal(writes.length,1);assert.match(document.body.textContent,/实收总额必须等于应收总额/);assert.equal(window.localStorage.getItem('kferp.order-create.pending.employee-11'),null)
  input(goods(),'60');input(shipping(),'20');await flush();button('保存订单').click();await flush()
  assert.equal(writes.length,2,document.body.textContent);assert.equal(writes[1].prepayment_amount,'80.00')
  for (const mode of ['free','deferred','unknown']) {
   const before=writes.length
   app.unmount();document.body.innerHTML='<div id="app"></div>';window.localStorage.clear();window.localStorage.setItem('auth_token','test-token');globalThis.receiptDomShippingAmount='0'
   app=createApp(Editor,{embedded:true,editId:0,copyId:0,customerPortal:false,fulfillmentMode:false,viewParams:{}});app.mount('#app');await flush()
   const next=app._instance.setupState;next.payStatuses=[{id:1,name:'未付款'},{id:2,name:'已付款'},{id:3,name:'预付款（付款未完成）'},{id:4,name:'已收款'}]
   next.applyEditData({edit_id:12,edit_revision:'revision-1',customer_id:8,pay_status_id:1,shipping_amount:'0',freight:{token:'freight-1',current_receivable_cents:0,charge_mode:'manual_current'},items:[{item_id:21,product_id:7,product_name:'测试商品',spec:'227g',qty:1,unit:'袋',unit_price:'120',manual_price:true,tier_id:'manual'}]})
   Object.assign(next.form,{edit_id:0,delivery_mode:'ship',payment_method:'现金',receipt_date:'2026-11-10',payment_shipping_amount:'',freight_state:{free:mode==='free',manual:false,chargeMode:mode==='deferred'?'deferred':'auto_current',autoAmountCents:null}})
   if(mode==='unknown'){next.form.pay_status_id=4;await flush();assert.equal(shipping().value,'');assert.equal(shipping().disabled,false);next.form.pay_status_id=1}
   await flush();input(document.querySelector('input[placeholder="填写实际已收金额"]'),'30');await flush();next.form.payment_method='现金';await flush()
   if(mode==='unknown'){
    assert.equal(shipping().value,'');assert.equal(shipping().disabled,false)
    input(shipping(),'2');next.form.freight_state={free:true,manual:false,chargeMode:'auto_current',autoAmountCents:null};await flush()
    assert.equal(shipping().value,'2');assert.equal(shipping().disabled,false)
    input(goods(),'28');button('保存订单').click();await flush();assert.equal(writes.length,before);assert.match(document.body.textContent,/不能超过对应应收/)
    input(shipping(),'0');input(goods(),'30');await flush();button('保存订单').click();await flush()
    assert.equal(writes.length,before+1);assert.equal(writes.at(-1).payment_shipping_amount,'0');continue
   }
   assert.equal(shipping().value,'0',mode);input(goods(),'30');await flush();button('保存订单').click();await flush()
   assert.equal(writes.length,before+1,mode);assert.equal(writes.at(-1).payment_shipping_amount,'0',mode)
  }
 } finally {app.unmount();document.body.innerHTML='<div id="app"></div>';globalThis.fetch=oldFetch;delete globalThis.receiptDomShippingAmount;window.localStorage.clear()}
})

test('管理员从页面输入员工请求号解除，员工页面确认后才清除记录',async()=>{
 const oldFetch=globalThis.fetch,requestID='employee-order-request-12345'
 let isAdmin=true,cancelled=false
 window.localStorage.setItem('auth_token','admin-token')
 window.localStorage.setItem('kferp.order-create.pending.employee-11',JSON.stringify({route:'/api/order',payload:{request_id:requestID,customer_id:3}}))
 window.prompt=()=> '管理员核对'
 globalThis.fetch=async(url,options={})=>{
  const path=new URL(String(url),window.location.origin).pathname
  if(path==='/api/auth/me')return response({employee_id:isAdmin?22:11,roles:isAdmin?[{code:'admin'}]:[]})
  if(path==='/api/order/pending-request/resolve'){assert.equal(options.headers.Authorization,'Bearer admin-token');assert.equal(JSON.parse(options.body).request_id,requestID);cancelled=true;return response({status:'cancelled'})}
  if(path.endsWith('/status')){assert.equal(options.headers.Authorization,'Bearer staff-token');return response({cancelled})}
  return response({rows:[],groups:[],assignments:[]})
 }
 let app=createApp(View,{embedded:true,editId:0,copyId:0,customerPortal:false,fulfillmentMode:false,viewParams:{}})
 try {
  app.mount('#app');await flush()
  assert.doesNotMatch(document.body.textContent,new RegExp(requestID))
  const input=document.querySelector('input[placeholder="输入完整请求号"]');assert.ok(input)
  input.value=requestID;input.dispatchEvent(new Event('input',{bubbles:true}));await flush()
  button('核对并解除未提交请求').click();await flush();assert.equal(cancelled,true)
  assert.ok(window.localStorage.getItem('kferp.order-create.pending.employee-11'))
  app.unmount();document.body.innerHTML='<div id="app"></div>';isAdmin=false;window.localStorage.setItem('auth_token','staff-token')
  app=createApp(View,{embedded:true,editId:0,copyId:0,customerPortal:false,fulfillmentMode:false,viewParams:{}});app.mount('#app');await flush()
  assert.match(document.body.textContent,new RegExp(requestID))
  button('核查管理员解除结果').click();await flush()
  assert.equal(window.localStorage.getItem('kferp.order-create.pending.employee-11'),null)
  assert.match(document.body.textContent,/管理员已核对并解除原请求/)
 } finally {app.unmount();globalThis.fetch=oldFetch;window.localStorage.clear()}
})
