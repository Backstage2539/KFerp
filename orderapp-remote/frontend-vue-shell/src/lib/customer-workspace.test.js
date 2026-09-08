import test from 'node:test'
import assert from 'node:assert/strict'
import { customerWorkspaceMenu, customerWorkspaceCapability, resetCustomerContinuation } from './customer-workspace.js'

test('capabilities each have a page and inactive capabilities are absent', () => {
 const capabilities=['bean_list','product_order','direct_ship','inventory_custody','settlement']
 const keys=customerWorkspaceMenu(capabilities).flatMap(g=>g.items.map(i=>i.key))
 assert(keys.includes('customerProcessingPortal'))
 assert(keys.includes('customerOrders'))
 for(const code of capabilities) assert(keys.some(key=>customerWorkspaceCapability(key)===code),code)
 assert(!keys.includes('customerProcessing'))
 assert(!keys.includes('customerMall'))
 assert.equal(customerWorkspaceCapability('customerDirectShip'),'direct_ship')
})
test('continuation keeps dates and customer but drops previous recipient and money', () => {
 const form={customer_id:14,order_date:'2026-07-19',document_date:'2026-09-09',receiver_name:'previous',receiver_phone:'123',receiver_address:'address',notes:'old',ship_tracking_no:'old',discount_amount:'20'}
 resetCustomerContinuation(form)
 assert.equal(form.customer_id,14)
 assert.equal(form.order_date,'2026-07-19')
 assert.equal(form.document_date,'2026-09-09')
 assert.equal(form.receiver_name,'')
 assert.equal(form.receiver_phone,'')
 assert.equal(form.receiver_address,'')
 assert.equal(form.notes,'')
})
