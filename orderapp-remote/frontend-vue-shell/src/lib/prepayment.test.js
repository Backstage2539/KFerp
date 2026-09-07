import {test} from 'node:test'
import assert from 'node:assert/strict'
import {buildOrderPayload,requiresOrderPaymentMethod} from './order-entry.js'
test('prepayment is explicitly sent and requires receipt method',()=>{
 const form={prepayment_amount:'30.50',pay_status_id:9}
 assert.equal(buildOrderPayload({form,rows:[]}).prepayment_amount,'30.50')
 assert.equal(requiresOrderPaymentMethod(form,[{id:9,name:'预付款（付款未完成）'}]),true)
})
