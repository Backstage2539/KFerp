import test from 'node:test'
import assert from 'node:assert/strict'
import { orderReceiptRequest } from './order-receipt.js'

test('构造逐笔收款，保留实际日期、订单版本及幂等标识', () => {
 const request=orderReceiptRequest({orderId:17,revision:'rev1',dueCents:5000,amount:'20.50',date:'2026-11-10',method:'现金',key:'safe-1'})
 assert.deepEqual(request,{edit_revision:'rev1',expected_due_cents:5000,actual_cents:2050,receipt_date:'2026-11-10',payment_method:'现金',payment_voucher_asset_id:0,idempotency_key:'safe-1'})
})
test('拒绝缺日期、超额和无效金额', () => {
 for(const v of [{amount:'1',date:''},{amount:'50.01',date:'2026-11-10'},{amount:'0',date:'2026-11-10'}]) assert.throws(()=>orderReceiptRequest({orderId:17,revision:'rev1',dueCents:5000,method:'现金',key:'safe-1',...v}))
})

test('不同员工的待确认请求隔离，缺身份时不允许登记', async () => {
 const {receiptPendingKey,receiptActorIdentity}=await import('./order-receipt.js')
 assert.notEqual(receiptPendingKey(receiptActorIdentity({employee_id:11}),17),receiptPendingKey(receiptActorIdentity({employee_id:12}),17))
 assert.notEqual(receiptActorIdentity({basic_auth_admin:true,name:'admin'}),receiptActorIdentity({employee_id:11}))
 assert.equal(receiptActorIdentity({basic_auth_admin:true,name:'admin'}),'basic-admin')
 assert.throws(()=>receiptPendingKey(receiptActorIdentity({}),17))
})

test('任何不确定响应都保留待确认请求', async () => {
 const {receiptKeepPending}=await import('./order-receipt.js')
 for (const status of [0,400,401,403,404,408,409,429,500]) assert.equal(receiptKeepPending(status),true)
 assert.equal(receiptKeepPending(400,true,true),false)
 assert.equal(receiptKeepPending(400,true,false),true)
 assert.equal(receiptKeepPending(403,true,true),true)
 assert.equal(receiptKeepPending(409,true,true),false)
 assert.equal(receiptKeepPending(409,true,false),true)
})

test('只有员工新订单明确填写到账金额时要求实际到账日期', async () => {
 const {initialReceiptDateRequired}=await import('./order-receipt.js')
 const order={prepayment_amount:'20',payment_goods_amount:'',payment_shipping_amount:'',edit_id:0}
 assert.equal(initialReceiptDateRequired(order,{}),true)
 assert.equal(initialReceiptDateRequired({...order,edit_id:1},{}),false)
 assert.equal(initialReceiptDateRequired(order,{customerPortal:true}),false)
 assert.equal(initialReceiptDateRequired(order,{freeSample:true}),false)
 assert.equal(initialReceiptDateRequired(order,{backfill:true}),false)
 assert.equal(initialReceiptDateRequired(order,{pendingConfirmation:true}),false)
 assert.equal(initialReceiptDateRequired({...order,prepayment_amount:'0'},{}),false)
})

test('首次收款应由用户明确货款和运费，合计需等于实际收款', async () => {
 const {validateInitialReceiptBreakdown,initialReceiptPrepayment}=await import('./order-receipt.js')
 const base={total:'80',freight:'20',status:'预付款（付款未完成）'}
 assert.equal(validateInitialReceiptBreakdown({...base,prepayment:'20',goods:'20',shipping:'0'}),'')
 assert.match(validateInitialReceiptBreakdown({...base,prepayment:'20',goods:'',shipping:''}),/货款和运费分项/)
 assert.match(validateInitialReceiptBreakdown({...base,prepayment:'20',goods:'10',shipping:'5'}),/合计/)
 assert.match(validateInitialReceiptBreakdown({...base,prepayment:'20',goods:'20.001',shipping:'0'}),/最多两位小数/)
 assert.match(validateInitialReceiptBreakdown({...base,status:'已付款',prepayment:'20',goods:'20',shipping:'0'}),/必须等于应收/)
 assert.match(validateInitialReceiptBreakdown({...base,prepayment:'30',goods:'0',shipping:'30'}),/不能超过/)
 assert.equal(validateInitialReceiptBreakdown({...base,status:'已付款',prepayment:'80',goods:'60',shipping:'20'}),'')
 assert.equal(initialReceiptPrepayment({prepayment:'0',goods:'60',shipping:'20',status:'已付款'}),'80.00')
 assert.equal(initialReceiptPrepayment({prepayment:'20',goods:'60',shipping:'20',status:'已付款'}),'20')
})

test('新订单不明结果按员工隔离并只允许原请求路径', async () => {
 const {orderCreatePendingKey,validPendingOrderCreate}=await import('./order-receipt.js')
 assert.notEqual(orderCreatePendingKey('employee-11'),orderCreatePendingKey('employee-12'))
 assert.throws(()=>orderCreatePendingKey(''))
 const p={route:'/api/order',payload:{request_id:'initial-order-request-123',customer_id:3}}
 assert.equal(validPendingOrderCreate(p),true)
 assert.equal(validPendingOrderCreate({...p,route:'/api/other'}),false)
 assert.equal(validPendingOrderCreate({...p,payload:{request_id:'',customer_id:3}}),false)
})

test('关闭并重开后只能重试相同员工保存的原始新单', async () => {
 const {savePendingOrderCreate,readPendingOrderCreate}=await import('./order-receipt.js')
 const data=new Map(),storage={setItem:(k,v)=>data.set(k,v),getItem:k=>data.get(k)||null}
 const original={route:'/api/order',payload:{request_id:'initial-order-request-123',customer_id:3,prepayment_amount:'20',receipt_date:'2026-11-10'}}
 savePendingOrderCreate(storage,'employee-11',original)
 assert.deepEqual(readPendingOrderCreate(storage,'employee-11'),original)
 assert.equal(readPendingOrderCreate(storage,'employee-12'),null)
})
