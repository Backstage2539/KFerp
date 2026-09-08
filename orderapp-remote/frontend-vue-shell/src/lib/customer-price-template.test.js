import test from 'node:test'
import assert from 'node:assert/strict'
import { applyCustomerPriceRows } from './customer-price-draft.js'
import { readPriceListGenerationDraft, savePriceListGenerationDraft } from './product-price-list-draft.js'

const base = { parent_product_id: 1063, product_id: 1063, bom_spec_id: 3, bom_variant_id: 483, price_unit: '袋', product_name: '初晓-商品', customer_reference_snapshot: { customer_id: 102, customer_display_name: 'NB的初晓' } }
const imported = [28,33,26,24].map((price,i)=>({ ...base, row_key:`old:${i}`, final_unit_price:price, pricing_mode:'fixed_price', pricing_mode_source:'customer_quote', tier_template_id:0, template_tier_id:59+i, min_qty:[14,2,24,48][i], max_qty:[23,13,47,null][i], customer_price_source_publication_id:126 }))
const generated = [27,24].map((price,i)=>({ ...base, row_key:`template16:${i}`, final_unit_price:price, pricing_mode:'tier_template', pricing_mode_source:'default', tier_template_id:16, template_tier_id:63+i, pricing_rule_id:[16,11][i], min_qty:[0,24][i], max_qty:[24,null][i] }))

test('an explicitly chosen customer template controls its tiers and calculated prices',()=>{
 const result=applyCustomerPriceRows(generated,imported,{},102,{configuredSources:{default:true}})
 assert.deepEqual(result.map(r=>[r.min_qty,r.max_qty,r.final_unit_price]),[[0,24,27],[24,null,24]])
 assert.equal(result[0].tier_template_id,16)
 assert.equal(result[0].pricing_mode,'tier_template')
 assert.equal(result[0].product_name,'NB的初晓')
 assert.equal(imported.length,4)
})

test('a saved customer draft with a different template is recovered using source publication configuration',()=>{
 const publications=[{id:126,config:{price_list_template_selection:{defaults:{tier_template_id:11}}}}]
 const result=applyCustomerPriceRows(generated,imported,{},102,{publications})
 assert.deepEqual(result.map(r=>r.template_tier_id),[63,64])
 const unchanged=generated.map(r=>({...r,tier_template_id:11}))
 assert.deepEqual(applyCustomerPriceRows(unchanged,imported,{},102,{publications}).map(r=>r.final_unit_price),[28,33,26,24])
})

test('changing the table default preserves independently configured product quotes',()=>{
 const other={...base,parent_product_id:2000,product_id:2000,row_key:'product:2000',pricing_mode:'fixed_price',pricing_mode_source:'parent_product',final_unit_price:999}
 const result=applyCustomerPriceRows([...generated,other],[...imported,{...other,final_unit_price:80}],{},102,{configuredSources:{default:true}})
 assert.deepEqual(result.map(r=>r.final_unit_price),[27,24,80])
})

test('category and product choices override only their resolved pricing scope, including missing or invalid templates',()=>{
 const category=generated.map(r=>({...r,pricing_mode_source:'subgroup',pricing_mode_source_group_item_id:42}))
 assert.equal(applyCustomerPriceRows(category,imported,{},102,{configuredSources:{'group:42':true}}).length,2)
 assert.equal(applyCustomerPriceRows(category,imported,{},102,{configuredSources:{'group:43':true}}).length,4)
 const invalid=[{...generated[0],pricing_mode_source:'parent_product',tier_template_id:0,final_unit_price:0,tier_unit_compatible:false}]
 const result=applyCustomerPriceRows(invalid,imported,{},102,{configuredSources:{'product:1063':true}})
 assert.equal(result.length,1)
 assert.equal(result[0].final_unit_price,0)
 assert.equal(result[0].tier_unit_compatible,false)
})

test('customer pricing choices survive refresh and remain isolated between named drafts',()=>{
 const values=new Map(),storage={getItem:k=>values.get(k),setItem:(k,v)=>values.set(k,v)}
 savePriceListGenerationDraft('customer102:table:a',{customerPriceConfiguredSources:{default:true,'group:42':true}},storage)
 savePriceListGenerationDraft('customer102:table:b',{},storage)
 assert.deepEqual(readPriceListGenerationDraft('customer102:table:a',storage).customerPriceConfiguredSources,{default:true,'group:42':true})
 assert.equal(readPriceListGenerationDraft('customer102:table:b',storage).customerPriceConfiguredSources,undefined)
})
