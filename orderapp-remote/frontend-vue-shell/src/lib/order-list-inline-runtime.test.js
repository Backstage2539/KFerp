import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import {parse,compileScript} from '@vue/compiler-sfc'
import {createSSRApp,h} from 'vue'
import {renderToString} from '@vue/server-renderer'
const filename=new URL('../views/OrdersView.vue',import.meta.url)
const {descriptor}=parse(fs.readFileSync(filename,'utf8'),{filename:filename.pathname})
let code=compileScript(descriptor,{id:'order-list-inline-runtime'}).content
code=code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g,(_,name)=>`const ${name}={render:()=>null}`)
code=code.replace(/from (['"])([^'"]+)\1/g,(_,q,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,filename).href:import.meta.resolve(path))}`)
const {default:View}=await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
test('real OrdersView integrates row toggle, retry, and invalidation without opening editor',async()=>{
 let view;await renderToString(createSSRApp({setup(){view=View.setup({viewParams:{},workspaceMode:'',customerContextId:0,customerContextLabel:''},{expose(){}});return()=>h('div')}}))
 const original=globalThis.fetch;const paths=[]
 globalThis.fetch=async url=>{paths.push(String(url));return {ok:true,json:async()=>String(url).includes('/detail')?{edit_data:{items:[{item_id:5,product_name_snapshot:'样品',bom_spec_name:'50g',qty:'1',unit:'袋',unit_price:0,line_total:0}],shipping_amount:0,grand_total:0,notes:'冻结备注'}}:{rows:[],summary:{},page:1,limit:20}}}
 try {
   const plain={target:{closest:()=>null}}
   await view.handleInlineRowClick({id:4},plain)
   assert.equal(view.inlineDetail.id,4);assert.equal(view.inlineDetail.detail.notes,'冻结备注');assert.equal(view.orderDetailDrawerOpen.value,false)
   await view.handleInlineRowClick({id:5},{target:{closest:()=>({})}});assert.equal(view.inlineDetail.id,4)
   await view.load();assert.equal(view.inlineDetail.id,0);assert.equal(view.inlineDetail.detail,null)
   await view.inlineOrders.toggle(4);assert.equal(paths.filter(p=>p.includes('/detail')).length,2)
   await view.handlePaginationChange({page:2,pageSize:20});assert.equal(view.inlineDetail.id,0)
 }finally{globalThis.fetch=original}
})

test('real OrdersView rematches only the persisted batch and keeps shipment confirmation separate',async()=>{
 let view;await renderToString(createSSRApp({setup(){view=View.setup({viewParams:{},workspaceMode:'',customerContextId:0,customerContextLabel:''},{expose(){}});return()=>h('div')}}))
 const original=globalThis.fetch;const writes=[]
 globalThis.fetch=async(url,opts={})=>{if(opts.method==='POST')writes.push({url:String(url),body:JSON.parse(opts.body)});return {ok:true,json:async()=>String(url).includes('shipping-return-rematch')?{batch_id:1,rematched:true,pending:2,rows:[{row_no:3,status:'pending'},{row_no:5,status:'pending'}]}:{rows:[],summary:{},page:1,limit:20}}}
 try {view.returnBatchID.value=1;await view.rematchShippingReturn();assert.deepEqual(writes,[{url:'/api/orders/shipping-return-rematch',body:{batch_id:1}}]);assert.equal(view.returnReport.value.length,2);assert.match(view.shippingMessage.value,/未执行发货/);assert.equal(view.trackingExcelLoading.value,false)
 globalThis.fetch=async()=>({ok:false,status:400,json:async()=>({error:'重新匹配失败'})});await view.rematchShippingReturn();assert.equal(view.returnReport.value.length,2);assert.equal(view.shippingError.value,'重新匹配失败')
 }finally{globalThis.fetch=original}
})
