import test from 'node:test'
import assert from 'node:assert/strict'
import { customerWorkspaceMenu, customerWorkspaceCapability, resetCustomerContinuation } from './customer-workspace.js'

test('shared workbench replaces duplicate menus while specialized capabilities remain', () => {
 const keys=customerWorkspaceMenu(['bean_list','product_order','direct_ship','inventory_custody','settlement','processing','mall']).flatMap(g=>g.items.map(i=>i.key))
 assert.deepEqual(keys,['customerProcessingPortal','customerPriceTables','customerProductOrder','customerProcessing','customerMall'])
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
