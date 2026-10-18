import test from 'node:test'
import assert from 'node:assert/strict'
import { convertedOrderRow, conversionTargets, conversionSnapshot } from './spec-conversion.js'
import { buildOrderPayload } from './order-entry.js'
test('converted order stores target identity and independent source selection',()=>{
 const q={group_id:1,revision:2,source_bom_spec_id:11,source_bom_variant_id:21,target_bom_spec_id:12,target_bom_variant_id:22,product_id:7,publication_id:8,price_row_key:'row',target_unit:'袋',target_spec_name:'500g袋装',unit_price:'52.86',version_no:'V1',price_source_json:'{}'}
 const row=convertedOrderRow({qty:2,product_name:'咖啡',item_id:33},q)
 const p=buildOrderPayload({form:{edit_id:77},rows:[row]})
 assert.deepEqual(p.bom_spec_id,['12']);assert.deepEqual(p.price_selection_mode,['converted_tier']);assert.deepEqual(p.selected_price_row_key,['row']);assert.deepEqual(p.item_id,['33']);assert.equal(JSON.parse(p.spec_conversion[0]).source_bom_spec_id,11)
 assert.equal(row.unit_price,'52.86');assert.equal(row.spec_invalid_message,'')
 assert.deepEqual(buildOrderPayload({form:{},rows:[row]}).item_id,['0'])
})
test('explicit template and spec matching never uses names',()=>{
 const group={members:[{template_id:4,template_key:'a',inventory_unit:'袋'},{bom_spec_id:2,inventory_unit:'盒'}]}
 const specs=[{bom_spec_id:1,template_id:4,template_key:'a',inventory_unit:'袋'},{bom_spec_id:2,inventory_unit:'盒'},{bom_spec_id:3,spec_name:'500g',inventory_unit:'袋'}]
 assert.deepEqual(conversionTargets(group,specs).map(s=>s.bom_spec_id),[1,2]);assert.equal(conversionSnapshot({price_source_json:'invalid'}),null)
})
test('superseded and closed quotes cannot overwrite the newest preview', async()=>{
 const { latestConversionRequest }=await import('./spec-conversion.js')
 const request=latestConversionRequest();const old=request.next();const current=request.next()
 assert.equal(request.current(old),false);assert.equal(request.current(current),true)
 request.cancel();assert.equal(request.current(current),false)
})
