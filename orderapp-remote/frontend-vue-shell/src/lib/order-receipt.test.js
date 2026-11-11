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
