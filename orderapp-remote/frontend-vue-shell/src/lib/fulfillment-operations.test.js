import test from 'node:test'
import assert from 'node:assert/strict'
import { fulfillmentStockQuantities, fulfillmentTodoCards, fulfillmentModuleTabs, fulfillmentBusinessLabel } from './fulfillment-operations.js'
import { fulfillmentSettingsForm, fulfillmentSettingsPayload, setFulfillmentFieldDefault } from './fulfillment-settings.js'

test('stable modules preserve read access regardless of capabilities', () => {
 assert.deepEqual(fulfillmentModuleTabs().map(x => x.key), ['overview', 'inventory', 'orders', 'dispatch', 'fees'])
 assert.equal(fulfillmentBusinessLabel('on_demand'), '现做现发')
})
test('physical available never includes purchased pending inbound', () => {
 assert.deepEqual(fulfillmentStockQuantities({total_qty:20,available_qty:15,occupied_qty:5,physical_available_qty:7,pending_inbound_qty:10,pending_reserved_qty:2}), {physical:10,reserved:3,shippable:7,pending:10,pendingReserved:2})
})
test('todo counts use authoritative totals and route to the exact server queue', () => {
 const cards=fulfillmentTodoCards({pending_shipment:131,stock_waiting:4,pending_payment:7,unsettled_fee_count:5})
 assert.equal(cards.find(x=>x.key==='pending_shipment').value,131)
 assert.equal(cards.find(x=>x.key==='stock_waiting').queue,'stock_waiting')
 assert.equal(cards.find(x=>x.key==='settlement').tab,'fees')
})
test('field defaults restore zero/false explicitly and retain revision and exceptions on save', () => {
 const f=fulfillmentSettingsForm({customer_id:1,revision:3,defaults_revision:2,defaults:{revision:2,modes:[{mode:'on_demand',fee_configured:true,fixed_fee_cents:300,per_package_fee_cents:0,shipping_free:false}]},modes:[{mode:'on_demand',enabled:true,fee_configured:true,fixed_fee_cents:0,per_package_fee_cents:0,shipping_free:true,inherited_fields:[]}]})
 const m=f.modes[1]
 assert.equal(m.fixed_fee,'0.00')
 setFulfillmentFieldDefault(m,'fixed_fee_cents',true,f.defaults)
 setFulfillmentFieldDefault(m,'shipping_free',true,f.defaults)
 assert.equal(m.fixed_fee,'3.00');assert.equal(m.shipping_free,false)
 setFulfillmentFieldDefault(m,'fixed_fee_cents',false,f.defaults);m.fixed_fee='0'
 const p=fulfillmentSettingsPayload(f)
 assert.equal(p.defaults_revision,2);assert.equal(p.modes[1].fixed_fee_cents,0)
 assert.deepEqual(p.modes[1].inherited_fields,['shipping_free'])
})
