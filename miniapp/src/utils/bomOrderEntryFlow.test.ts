import { it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import ts from 'typescript'
import { orderedBOMSpecs, selectBOMSpec, resolveBOMTier } from './bomOrderPricing'
import { applyConversionQuote } from './specConversion'
const specs=[{bom_spec_id:1,bom_variant_id:11,is_default:true,spec_name:'默认',inventory_unit:'袋'},{bom_spec_id:2,bom_variant_id:12,spec_name:'500g',inventory_unit:'袋'},{bom_spec_id:3,bom_variant_id:13,spec_name:'250g',inventory_unit:'袋'}]
const source_tiers=[{bom_spec_id:1,bom_variant_id:11,price_row_key:'low',min_qty:2,max_qty:13,quantity_basis:'sales_spec_count'},{bom_spec_id:1,bom_variant_id:11,price_row_key:'high',min_qty:14,max_qty:null,quantity_basis:'sales_spec_count'}]
function fixture(){
 const pending:Array<{body:any;resolve:(value:any)=>void}>=[]
 const ctx:any={orderedBOMSpecs,selectBOMSpec,resolveBOMTier,applyConversionQuote,form:{value:{customer_id:9}},session:{token:'employee'},fetchEmployeeSpecConversions:async()=>({specs,source_tiers,groups:[]}),quoteEmployeeSpecConversion:(_token:string,body:any)=>new Promise(resolve=>pending.push({body,resolve}))}
 const source=readFileSync(new URL('../pages/employee-order-entry/employee-order-entry.vue',import.meta.url),'utf8')
 const block=source.slice(source.indexOf('function bomContext('),source.indexOf('import EmployeeSpecConversionSheet'))
 vm.runInNewContext(ts.transpileModule(block,{compilerOptions:{target:ts.ScriptTarget.ES2022}}).outputText,ctx)
 const item:any={product_id:7,bom_spec_id:1,bom_variant_id:11,bean_list_publication_id:8,qty:2,bom_catalog:{specs,source_tiers}}
 async function finish(index=pending.length-1){const {body:b,resolve}=pending[index];resolve({...b,unit_price:b.price_row_key==='low'?'10.00':'8.00',quote_revision:'verified',target_spec_name:'目标',target_unit:'袋',pricing_method:'bom_rule',price_source_json:'{}'});await new Promise(r=>setImmediate(r))}
 return {ctx,item,pending,finish}
}
it('employee initial default and quantity changes automatically quote; nondefault manual price survives quantity',async()=>{
 const {ctx,item,pending,finish}=fixture();await ctx.loadBOMSpecs(item);expect(item.selected_price_row_key).toBe('low');await finish()
 item.qty=14;ctx.syncBOMTier(item);expect(item.selected_price_row_key).toBe('high');await finish()
 Object.assign(item,selectBOMSpec(item,specs[1] as any));expect(item.selected_price_row_key).toBe('')
 ctx.chooseBOMTier(item,{detail:{value:1}});await finish();const count=pending.length
 item.qty=100;ctx.syncBOMTier(item);expect(item.unit_price).toBe(10);expect(pending.length).toBe(count)
 Object.assign(item,selectBOMSpec(item,specs[2] as any));expect(item.selected_price_row_key).toBe('')
 Object.assign(item,selectBOMSpec(item,specs[0] as any));expect(item.selected_price_row_key).toBe('high')
})
it('employee late quote cannot overwrite new quantity; frozen default only reprices on request',async()=>{
 const {ctx,item,pending,finish}=fixture();await ctx.loadBOMSpecs(item);item.qty=14;ctx.syncBOMTier(item)
 await finish(0);expect(item.unit_price).toBe(0);expect(item.validation_error).toBeTruthy()
 await finish(1);expect(item.unit_price).toBe(8)
 item.spec_conversion.reprice=false;item.qty=2;ctx.syncBOMTier(item);expect(pending).toHaveLength(2);expect(item.unit_price).toBe(8)
 await ctx.repriceBOMTier(item);expect(item.selected_price_row_key).toBe('low');await finish();expect(item.unit_price).toBe(10)
})
it('employee no-match retains manual selection until automatic match exists',async()=>{
 const {ctx,item,finish}=fixture();item.qty=1;await ctx.loadBOMSpecs(item);expect(item.selected_price_row_key).toBe('')
 ctx.chooseBOMTier(item,{detail:{value:2}});await finish();ctx.syncBOMTier(item);expect(item.selected_price_row_key).toBe('high')
 item.qty=2;ctx.syncBOMTier(item);await finish();expect(item.selected_price_row_key).toBe('low')
 item.qty=1;ctx.syncBOMTier(item);expect(item.selected_price_row_key).toBe('');expect(item.validation_error).toBeTruthy()
})
