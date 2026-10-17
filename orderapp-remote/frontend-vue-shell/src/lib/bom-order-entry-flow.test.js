import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { orderedBOMSpecs, bomTierSelection, bomQuoteIsCurrent, resolveBOMTier } from './bom-order-pricing.js'
import { convertedOrderRow } from './spec-conversion.js'
const specs=[{bom_spec_id:1,bom_variant_id:11,is_default:true,spec_name:'默认',inventory_unit:'袋'},{bom_spec_id:2,bom_variant_id:12,spec_name:'500g',inventory_unit:'袋'},{bom_spec_id:3,bom_variant_id:13,spec_name:'250g',inventory_unit:'袋'}]
const source_tiers=[{bom_spec_id:1,bom_variant_id:11,price_row_key:'low',min_qty:2,max_qty:13,quantity_basis:'sales_spec_count'},{bom_spec_id:1,bom_variant_id:11,price_row_key:'high',min_qty:14,max_qty:null,quantity_basis:'sales_spec_count'}]
function fixture(){
 const pending=[]
 const ctx={orderedBOMSpecs,bomTierSelection,bomQuoteIsCurrent,resolveBOMTier,convertedOrderRow,props:{customerPortal:false},form:{customer_id:9},ensureTrailingBlankRow(){},apiGet:async()=>({specs,source_tiers,groups:[]}),apiSend:(path,{body})=>new Promise(resolve=>pending.push({body,resolve}))}
 const source=readFileSync(new URL('../views/OrderEntryView.vue',import.meta.url),'utf8')
 vm.runInNewContext(source.slice(source.indexOf('function bomContext('),source.indexOf('import OperationManualView')),ctx)
 const row={product_id:7,bom_spec_id:1,bom_variant_id:11,bean_list_publication_id:8,qty:2,bom_catalog:{specs,source_tiers}}
 async function finish(index=pending.length-1){const {body:b,resolve}=pending[index];resolve({...b,unit_price:b.price_row_key==='low'?'10.00':'8.00',quote_revision:'verified',target_spec_name:'目标',target_unit:'袋',pricing_method:'bom_rule',price_source_json:'{}'});await new Promise(r=>setImmediate(r))}
 return {ctx,row,pending,finish}
}
test('ERP default quantity changes tiers, every nondefault spec change resets, switching back auto matches',async()=>{
 const {ctx,row,pending,finish}=fixture()
 ctx.selectOrderBOMSpec(row,specs[0]);assert.equal(row.selected_price_row_key,'low');await finish()
 row.qty=14;ctx.syncOrderBOMTier(row);assert.equal(row.selected_price_row_key,'high');await finish()
 ctx.selectOrderBOMSpec(row,specs[1]);assert.equal(row.selected_price_row_key,'');assert.equal(row.unit_price,'');assert.equal(row.price_missing,true)
 ctx.selectOrderBOMTier(row,{target:{value:'low'}});await finish();const count=pending.length
 row.qty=100;ctx.syncOrderBOMTier(row);assert.equal(row.unit_price,'10.00');assert.equal(pending.length,count)
 ctx.selectOrderBOMSpec(row,specs[2]);assert.equal(row.selected_price_row_key,'')
 ctx.selectOrderBOMSpec(row,specs[0]);assert.equal(row.selected_price_row_key,'high');await finish()
})
test('ERP pending quantity/spec changes isolate late responses; frozen rows only reprice explicitly',async()=>{
 const {ctx,row,pending,finish}=fixture()
 ctx.selectOrderBOMSpec(row,specs[0]);row.qty=14;ctx.syncOrderBOMTier(row)
 await finish(0);assert.equal(row.price_missing,true);assert.equal(row.unit_price,'')
 await finish(1);assert.equal(row.unit_price,'8.00')
 row.spec_conversion.reprice=false;row.qty=2;ctx.syncOrderBOMTier(row);assert.equal(row.unit_price,'8.00');assert.equal(pending.length,2)
 await ctx.repriceOrderBOMTier(row);assert.equal(row.selected_price_row_key,'low');await finish()
 ctx.selectOrderBOMSpec(row,specs[1]);ctx.selectOrderBOMTier(row,{target:{value:'low'}})
 ctx.selectOrderBOMSpec(row,specs[2]);await finish();assert.equal(row.selected_price_row_key,'');assert.equal(row.price_missing,true)
})
test('ERP no-match preserves only manual fallback and changing quantity within a pending tier completes',async()=>{
 const {ctx,row,finish}=fixture();row.qty=1;ctx.selectOrderBOMSpec(row,specs[0]);assert.equal(row.selected_price_row_key,'')
 ctx.selectOrderBOMTier(row,{target:{value:'high'}});await finish();ctx.syncOrderBOMTier(row);assert.equal(row.selected_price_row_key,'high')
 row.qty=2;ctx.syncOrderBOMTier(row);row.qty=3;ctx.syncOrderBOMTier(row);await finish();assert.equal(row.selected_price_row_key,'low');assert.equal(row.price_missing,false)
 row.qty=1;ctx.syncOrderBOMTier(row);assert.equal(row.selected_price_row_key,'');assert.equal(row.price_missing,true)
})
