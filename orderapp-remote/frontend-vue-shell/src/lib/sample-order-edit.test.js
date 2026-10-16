import {test} from 'node:test'
import assert from 'node:assert/strict'
import {buildOrderPayload} from './order-entry.js'
import {isSampleOrderEdit,validManualOrderPrice,storedSampleSpecKey,preservesSampleSpec} from './sample-order-edit.js'

test('free sample edit is allowed only for server-marked existing internal orders',()=>{
 assert.equal(isSampleOrderEdit({edit_id:12,is_sample_order:true}),true)
 assert.equal(isSampleOrderEdit({edit_id:12,is_sample_order:false}),false)
 assert.equal(isSampleOrderEdit({edit_id:0,is_sample_order:true}),false)
 assert.equal(isSampleOrderEdit({edit_id:12,is_sample_order:true},{copyMode:true}),false)
 assert.equal(isSampleOrderEdit({edit_id:12,is_sample_order:true},{customerPortal:true}),false)
})
test('manual zero stays valid for sample edits while ordinary order zero and blank are rejected',()=>{
 assert.equal(validManualOrderPrice({manual_price:true,unit_price:0},true),true)
 assert.equal(validManualOrderPrice({manual_price:true,unit_price:'0'},false),false)
 for(const unit_price of ['',null,' ',NaN,-1,Infinity]) assert.equal(validManualOrderPrice({manual_price:true,unit_price},true),false)
 assert.equal(validManualOrderPrice({manual_price:true,unit_price:2.3},false),true)
})
test('sample historical specification allowance does not follow a product or spec change',()=>{
 const row={item_id:91,parent_product_id:11,product_id:11,bom_spec_id:3,bom_variant_id:7}
 row.sample_stored_spec_key=storedSampleSpecKey(row)
 assert.equal(preservesSampleSpec(row,true),true)
 assert.equal(preservesSampleSpec({...row,bom_spec_id:4},true),false)
 assert.equal(preservesSampleSpec({...row,parent_product_id:12},true),false)
 assert.equal(preservesSampleSpec(row,false),false)
})
test('order payload preserves explicit numeric zero and does not transmit sample privilege',()=>{
 const payload=buildOrderPayload({form:{edit_id:12,is_sample_order:true},rows:[{product_id:11,parent_product_id:11,bom_spec_id:3,migration_state:'cutover',qty:1,unit:'袋',unit_price:0,manual_price:true,price_selection_mode:'manual'}]})
 assert.deepEqual(payload.unit_price,['0']); assert.deepEqual(payload.tier_id,['manual'])
 assert.equal(Object.hasOwn(payload,'is_sample_order'),false)
})

test('sample edits preserve a zero publication header and BOM identity in their write payload',()=>{
 const payload=buildOrderPayload({form:{edit_id:12,bean_list_publication_id:0},rows:[{product_id:11,parent_product_id:11,bom_spec_id:3,bom_variant_id:7,migration_state:'cutover',qty:2,unit:'袋',unit_price:'0',manual_price:true,bean_list_publication_id:0,sample_stored_spec_key:'11:3'}]})
 assert.equal(payload.bean_list_publication_id,0);assert.deepEqual(payload.item_bean_list_publication_id,['0']);assert.deepEqual(payload.bom_spec_id,['3']);assert.deepEqual(payload.bom_variant_id,['7']);assert.equal(Object.hasOwn(payload,'sample_stored_spec_key'),false)
})

test('existing sample line identifier survives form payload while copied/new orders clear it',()=>{
 const row={item_id:91,product_id:11,parent_product_id:11,bom_spec_id:3,bom_variant_id:7,migration_state:'cutover',qty:2,unit:'袋',unit_price:'0',manual_price:true}
 assert.deepEqual(buildOrderPayload({form:{edit_id:12},rows:[row]}).item_id,['91'])
 assert.deepEqual(buildOrderPayload({form:{edit_id:0},rows:[row]}).item_id,['0'])
})
test('sample stored identity restores every backend identity after catalogue hydration',async()=>{
 const {sampleStoredItemIdentity}=await import('./sample-order-edit.js')
 const original={item_id:91,product_id:11,parent_product_id:11,bom_spec_id:3,bom_variant_id:7,customer_product_alias_id:0,customer_product_reference_id:0}
 const hydrated={item_id:0,product_id:99,parent_product_id:99,bom_spec_id:9,bom_variant_id:90,customer_product_alias_id:77,customer_product_reference_id:88}
 Object.assign(hydrated,sampleStoredItemIdentity(original))
 assert.deepEqual(hydrated,original)
 const key=storedSampleSpecKey(hydrated)
 for(const field of ['item_id','product_id','parent_product_id','bom_spec_id','bom_variant_id','customer_product_alias_id','customer_product_reference_id']) assert.notEqual(storedSampleSpecKey({...hydrated,[field]:Number(hydrated[field])+1}),key)
})
