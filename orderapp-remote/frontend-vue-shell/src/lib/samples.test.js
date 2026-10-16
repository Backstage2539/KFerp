import {test} from 'node:test'
import assert from 'node:assert/strict'
import {sampleTotal,confirmRows,canConfirmSample} from './samples.js'
test('sample totals preserve zero prices and decimal cents',()=>{assert.equal(sampleTotal([{qty:2,unit_price:0}],0),0);assert.equal(sampleTotal([{qty:3,unit_price:.1}],.2),.5)})
test('sample spec choices are restricted to the requested product',()=>{const rows=confirmRows([{product_id:1,name:'甲'},{product_id:2,name:'乙'}],[{id:10,product_id:1,name:'50g'}]);assert.equal(rows[0].bom_spec_id,10);assert.equal(rows[1].bom_spec_id,0)})
test('completed or unassigned work cannot create another order',()=>{assert.equal(canConfirmSample({work_order_id:0,order_id:0}),false);assert.equal(canConfirmSample({work_order_id:2,order_id:3}),false)})
test('terminal followup blocks confirmation and pending contact permits it',()=>{assert.equal(canConfirmSample({work_order_id:2,order_id:0,followup_status:'won'}),false);assert.equal(canConfirmSample({work_order_id:2,order_id:0,status:'pending_contact',followup_status:'pending'}),true)})

test('sample messages open their work order and leave ordinary order messages unchanged',async()=>{const {sampleNotificationNavigation}=await import('./samples.js');assert.deepEqual(sampleNotificationNavigation({source_type:'sample_request',source_id:12,payload:{view:'sampleRequests',request_id:12}}),{key:'sampleRequests',params:{request_id:12}});assert.equal(sampleNotificationNavigation({source_type:'sales_order',source_id:12}),null)})
test('sample entry is limited to internal sales and administrators',async()=>{const {actorCanUseSamples}=await import('./samples.js');assert.equal(actorCanUseSamples({roles:[{code:'sales'}],account_type:'internal_employee'}),true);assert.equal(actorCanUseSamples({roles:[{code:'warehouse'}]}),false);assert.equal(actorCanUseSamples({roles:[{code:'sales'}],account_type:'channel_customer'}),false)})
