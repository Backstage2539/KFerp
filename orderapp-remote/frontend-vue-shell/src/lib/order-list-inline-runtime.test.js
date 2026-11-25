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

test('current return filter reads all matches from the server and reloads the batch after selection', async () => {
  let view
  await renderToString(createSSRApp({setup() {
    view = View.setup({viewParams:{}, customerContextId:0}, {expose(){}})
    return () => h('div')
  }}))
  const previousWindow = globalThis.window, previousFetch = globalThis.fetch
  const url = new URL('http://localhost/vue-shell/?view=orders&shipping_return_batch_id=8')
  globalThis.window = {location:url, history:{replaceState() {}}}
  const paths = []
  let applied = false
  globalThis.fetch = async (u, opts) => {
    assert.notEqual(opts.method, 'POST')
    const requested = new URL(u, url)
    paths.push(requested.pathname)
    let result = {}
    if (requested.pathname === '/api/orders') {
      assert.equal(requested.searchParams.get('shipping_return_batch_id'), '8')
      result = {rows:[{id:135,order_no:'SO-TEST'}],summary:{orders:1},total:1}
    }
    if (requested.pathname === '/api/orders/shipping-return-batches/8') {
      const candidates = applied ? [] : [{order_id:135, order_no:'SO-TEST', version:'v1'}]
      result = {batch_id:8, rows:[6,7].map(id => ({id,status:applied?'applied':'pending',order_id:applied?135:0,candidates}))}
    }
    return {ok:true,json:async()=>result}
  }
  try {
    view.applyUrlFilters()
    await view.load()
    assert.equal(view.error.value, '')
    assert.equal(view.rows.value.length, 1)
    assert.equal(view.batchMatches.value.orderCount, 1)
    assert.equal(view.batchMatches.value.pendingCount, 2)
    assert.deepEqual(view.selectedOrderIDs.value, [])
    const candidate = view.returnReport.value[0].candidates[0]
    view.openReturnPicker(candidate, true)
    assert.deepEqual(view.pickerRows.value.map(r=>r.id), [6,7])
    applied = true
    await view.returnSelectionSaved()
    assert.equal(view.returnPicker.value, null)
    assert.equal(view.batchMatches.value.pendingCount, 0)
    assert.equal(view.batchMatches.value.orderCount, 1)
    assert.deepEqual(view.returnReport.value.map(r=>r.status), ['applied','applied'])
    assert.equal(paths.includes('/api/orders/shipping-return-candidates'), false)
    assert.equal(paths.filter(path=>path==='/api/orders/shipping-return-batches/8').length, 2)
  } finally { globalThis.window = previousWindow; globalThis.fetch = previousFetch }
})

test('role administrators can confirm shipping returns without an explicit orders.write grant', async () => {
  for (const [actor, expected] of [
    [{ account_type: 'employee', roles: [{code: 'admin'}] }, true],
    [{ account_type: 'employee', permissions: ['orders.read'] }, false],
    [{ account_type: 'channel_customer', roles: [{code:'admin'}], permissions: ['orders.write'] }, false],
  ]) {
    let view
    await renderToString(createSSRApp({setup() {
      view = View.setup({viewParams:{}, actor}, {expose(){}})
      return () => h('div')
    }}))
    assert.equal(view.canManageShippingReturns.value, expected)
  }
})

test('a missing return batch clears stale context and reloads normal orders with the other filters', async () => {
  const oldWindow = globalThis.window, oldFetch = globalThis.fetch
  try {
    for (const cancelFilter of [false, true]) {
      let view
      await renderToString(createSSRApp({setup() {
        view = View.setup({viewParams:{}, customerContextId:0}, {expose(){}})
        return () => h('div')
      }}))
      const location = new URL('http://localhost/vue-shell/?view=orders&shipping_return_batch_id=19&q=demo&page=4')
      let historyURL = ''
      globalThis.window = {location, history:{replaceState(_, __, url) {historyURL = url}}}
      const requests = []
      globalThis.fetch = async url => {
        const u = new URL(url, location)
        requests.push(u)
        if (u.pathname.endsWith('/shipping-return-batches/19')) {
          return {ok:false, status:404, json:async()=>({error:'导入批次不存在'})}
        }
        if (u.pathname === '/api/orders' && u.searchParams.has('shipping_return_batch_id')) {
          return {ok:false, status:400, json:async()=>({error:'无效筛选'})}
        }
        return {ok:true,json:async()=>u.pathname === '/api/orders'
          ? {rows:[{id:91,order_no:'SO-恢复'}],summary:{orders:1},total:1}
          : {}}
      }
      view.applyUrlFilters()
      view.returnReport.value = [{id:5}]
      if (cancelFilter) view.onlyReturnMatches.value = false
      await view.load()
      assert.equal(view.error.value, '')
      assert.equal(view.onlyReturnMatches.value, false)
      assert.equal(view.returnBatchID.value, 0)
      assert.deepEqual(view.returnReport.value, [])
      assert.equal(view.rows.value[0].id,91)
      assert.match(view.shippingError.value,/批次.*不存在/)
      const last = requests.filter(u=>u.pathname==='/api/orders').at(-1)
      assert.equal(last.searchParams.get('q'), 'demo')
      assert.equal(last.searchParams.get('page'), '1')
      assert.equal(last.searchParams.has('shipping_return_batch_id'), false)
      assert.doesNotMatch(historyURL,/shipping_return_batch_id/)
      await view.load()
      assert.equal(requests.filter(u=>u.pathname.endsWith('/shipping-return-batches/19')).length,1)
    }
  } finally { globalThis.window = oldWindow; globalThis.fetch = oldFetch }
})
