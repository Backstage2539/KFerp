import test from 'node:test'
import assert from 'node:assert/strict'
import { orderTypeLabels, inlineMoney, inlineOrderItems, inlineOrderTotals, shouldToggleOrderRow, createOrderInlineDetails } from './order-list-inline.js'

test('all actual types and true sample association, deduplicated with safe legacy fallback', () => {
  assert.deepEqual(orderTypeLabels({order_type:'批发',is_sample_order:true,product_type_labels:['冻干粉','挂耳','冻干粉']}),['批发','样品','冻干粉','挂耳'])
  assert.deepEqual(orderTypeLabels({order_type:'样品',is_sample_order:true,product_type_labels:['挂耳']}),['样品','挂耳'])
  assert.deepEqual(orderTypeLabels({order_type:'批发',product_kind_summary:'roasted_bean,green_bean,mystery'}),['批发','熟豆','生豆','未分类'])
  assert.deepEqual(orderTypeLabels({}),['未分类'])
})
test('snapshot specifications, names, zero prices and authoritative totals', () => {
  assert.equal(inlineMoney(0),'¥0.00'); assert.equal(inlineMoney(null),'—')
  const [item]=inlineOrderItems({items:[{item_id:8,product_name:'当前名称',product_name_snapshot:'下单名称',bom_spec_name:'寄样 50g',spec:'999',qty:'2',unit:'袋',unit_price:0,line_total:0,note:'磨粉'}]})
  assert.equal(item.name,'下单名称'); assert.equal(item.spec,'寄样 50g'); assert.equal(item.quantity,'2 袋'); assert.equal(item.price,'¥0.00'); assert.equal(item.subtotal,'¥0.00'); assert.equal(item.note,'磨粉')
  assert.deepEqual(inlineOrderTotals({shipping_amount:'12',discount_amount:'3',outsource_total_fee:'8',grand_total:'117'}).map(x=>[x.label,x.value]),[['运费','¥12.00'],['优惠','−¥3.00'],['委外费用','¥8.00'],['总价','¥117.00']])
})
test('plain row clicks toggle; controls, selected text and prevented events do not',()=>{
  const ordinary={target:{closest:()=>null},button:0}
  assert.equal(shouldToggleOrderRow(ordinary,''),true)
  assert.equal(shouldToggleOrderRow(ordinary,'复制订单名称'),false)
  assert.equal(shouldToggleOrderRow({...ordinary,defaultPrevented:true},''),false)
  for(const tag of ['a','button','input','label','select','textarea','summary','[role="button"]','[contenteditable]']) {
    assert.equal(shouldToggleOrderRow({target:{closest:selector=>selector.includes(tag)?{}:null}},''),false,tag)
  }
})
const deferred=()=>{let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b});return{promise,resolve,reject}}
test('one expanded order, lazy caching and late results cannot cross orders',async()=>{
 const a=deferred(),b=deferred(),calls=[];const state={};const c=createOrderInlineDetails(id=>{calls.push(id);return id===1?a.promise:b.promise},state)
 assert.deepEqual(calls,[])
 const p=c.toggle(1);const q=c.toggle(2);a.resolve({edit_data:{notes:'A'}});await p
 assert.equal(state.id,2);assert.equal(state.detail,null)
 b.resolve({edit_data:{notes:'B'}});await q;assert.equal(state.detail.notes,'B')
 await c.toggle(2);assert.equal(state.id,0);await c.toggle(1);assert.equal(state.detail.notes,'A');assert.deepEqual(calls,[1,2])
})
test('refresh invalidates cache and in-flight responses; retry reports then recovers',async()=>{
 let call=0;const pending=deferred();const state={};const c=createOrderInlineDetails(()=>{call++;return call===1?pending.promise:Promise.resolve({edit_data:{notes:'fresh'}})},state)
 const p=c.toggle(1);c.reset();pending.resolve({edit_data:{notes:'old'}});await p;assert.equal(state.id,0);assert.equal(state.detail,null)
 await c.toggle(1);assert.equal(state.detail.notes,'fresh');assert.equal(call,2)
 const s={};let fail=true;const d=createOrderInlineDetails(async()=>{if(fail)throw Error('无权访问');return{edit_data:{items:[]}}},s)
 await d.toggle(3);assert.equal(s.error,'无权访问');assert.equal(s.loading,false);fail=false;await d.retry();assert.equal(s.error,'');assert.deepEqual(s.detail.items,[])
})
test('collapse while loading and reopen reuses request without flashing another order',async()=>{
 const pending=deferred();let calls=0;const s={};const c=createOrderInlineDetails(()=>{calls++;return pending.promise},s)
 const p=c.toggle(1);await c.toggle(1);const q=c.toggle(1);pending.resolve({edit_data:{notes:'same'}});await Promise.all([p,q]);assert.equal(s.detail.notes,'same');assert.equal(calls,1)
})

test('customer snapshot wins over catalogue names and stored numeric specs keep grams',()=>{
 const [item]=inlineOrderItems({items:[{customer_product_display_name_snapshot:'客户约定名称',product_name_snapshot:'工厂旧名',product_name:'现名',bom_spec_name:'454',qty:1,unit:'袋',unit_price:'12.50',line_total:'12.50'}]})
 assert.equal(item.name,'客户约定名称');assert.equal(item.spec,'454g')
})
