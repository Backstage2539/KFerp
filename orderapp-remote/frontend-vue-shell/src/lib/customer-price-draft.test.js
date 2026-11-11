import {test} from 'node:test'
import assert from 'node:assert/strict'
import {seedCustomerPriceRows,applyCustomerPriceRows} from './customer-price-draft.js'
import { migrateLegacyFixedPriceFlatRowOverrides } from './product-price-list-draft.js'
import { priceListFlatRowsReady } from './costing-price-list-workflow.js'
const row=(spec,price,min=0,max=null)=>({parent_product_id:1,product_id:1,sku_id:1,bom_spec_id:spec,price_unit:'袋',final_unit_price:price,min_qty:min,max_qty:max,row_key:`${spec}:${min}`,product_name:'工厂名',group_snapshot:{group_item_name:'工厂分类'}})
const generated=[{...row(10,999),product_name:'客户名',group_snapshot:{group_item_name:'客户分类'},customer_reference_snapshot:{customer_id:42}}]
test('BOM version changes retain exact product/spec quotes with current write identity and original cost provenance',()=>{
 const old={...row(10,65,24),bom_id:90,bom_version_id:901,bom_variant_id:101, bom_spec_snapshot:{bom_spec_id:10,bom_variant_id:101},cost_source_snapshot:{bom_version_id:901}}
 const current={...generated[0],bom_id:91,bom_version_id:902,bom_variant_id:102,bom_spec_snapshot:{bom_spec_id:10,bom_variant_id:102}}
 const pub={id:20,status:'published',owner_type:'customer',owner_key:'42',content:{price_rows:[old,{...old,parent_product_id:2,product_id:2,final_unit_price:1}]}}
 const saved=structuredClone(pub)
 const seeds=seedCustomerPriceRows([],[current],[pub],42)
 assert.equal(seeds.length,1)
 const [result]=applyCustomerPriceRows([current],seeds,{'10:24':66},42)
 assert.equal(result.final_unit_price,66)
 assert.equal(result.original_final_unit_price,65)
 assert.equal(result.min_qty,24)
 assert.equal(result.bom_version_id,902)
 assert.equal(result.bom_variant_id,102)
 assert.equal(result.bom_spec_snapshot.bom_variant_id,102)
 assert.equal(result.cost_source_snapshot.bom_version_id,901)
 assert.equal(result.cost_source_snapshot.customer_price_source.publication_id,20)
 assert.deepEqual(pub,saved)
 const withoutSnapshot={...current};delete withoutSnapshot.bom_spec_snapshot
 assert.equal(applyCustomerPriceRows([withoutSnapshot],seeds,{},42)[0].bom_spec_snapshot,undefined)
})
test('seed exact product/spec published tiers, retain customer prices and never seed from another customer',()=>{
 const pub={id:1,status:'published',owner_type:'official',content:{price_rows:[row(10,80,0,24),row(10,70,24),row(11,40)]}}
 const own={id:2,status:'published',owner_type:'customer',owner_key:'42',content:{price_rows:[row(10,65,0,24),row(10,60,24)]}}
 const foreign={...own,id:3,owner_key:'43',content:{price_rows:[row(10,1)]}}
 const seeded=seedCustomerPriceRows([],generated,[foreign,own,pub],42)
 assert.equal(seeded.length,2);assert.deepEqual(seeded.map(r=>r.final_unit_price),[65,60])
 assert.deepEqual(seedCustomerPriceRows(seeded,generated,[{...pub,id:4,content:{price_rows:[row(10,999)]}}],42),seeded)
 const output=applyCustomerPriceRows(generated,seeded,{'10:0':66},42)
 assert.equal(output[0].final_unit_price,66);assert.equal(output[0].product_name,'客户名');assert.equal(output[0].group_snapshot.group_item_name,'客户分类')
 assert.equal(output[1].min_qty,24)
})
test('missing publication price remains unquoted; retained overrides and current generated prices cannot silently become quotes',()=>{
 const rows=applyCustomerPriceRows(generated,[],{},42);assert.equal(rows[0].final_unit_price,0);assert.equal(rows[0].customer_quote_missing,true)
 const seeds=seedCustomerPriceRows([],generated,[{id:1,status:'published',owner_type:'official',content:{price_rows:[row(11,3)]}}],42);assert.equal(seeds.length,0)
})

test('a missing spec in the latest public table does not borrow an older spec quote',()=>{
 const old={id:110,status:'published',owner_type:'official',content:{price_rows:[row(7,65)]}}
 const current={id:122,status:'published',owner_type:'official',content:{price_rows:[row(3,33)]}}
 const requested=[{...row(7,999),customer_reference_snapshot:{customer_id:302}}]
 const seeds=seedCustomerPriceRows([],requested,[old,current],302)
 assert.deepEqual(seeds,[])
 assert.equal(applyCustomerPriceRows(requested,seeds,{},302)[0].customer_quote_missing,true)
})

import {savePriceListGenerationDraft,readPriceListGenerationDraft} from './product-price-list-draft.js'
test('customer quote draft survives reload without resetting imported tiers or local edits',()=>{
 const data=new Map();const storage={getItem:k=>data.get(k),setItem:(k,v)=>data.set(k,v)}
 const draft={customerPriceSeedRows:[row(10,65,24)],flatRowOverrides:{'10:24':66}}
 savePriceListGenerationDraft('customer42',draft,storage)
 assert.deepEqual(readPriceListGenerationDraft('customer42',storage).customerPriceSeedRows,draft.customerPriceSeedRows)
 assert.equal(readPriceListGenerationDraft('customer42',storage).flatRowOverrides['10:24'],66)
 assert.equal(readPriceListGenerationDraft('customer43',storage),null)
})
test('customer price rows use the reference display name even when canonical product_name is present',()=>{
  const current={...generated[0],product_name:'工厂名',customer_reference_snapshot:{customer_id:42,customer_display_name:'客户专用名'}}
  assert.equal(applyCustomerPriceRows([current],[row(10,65)],{},42)[0].product_name,'客户专用名')
})

test('copied final prices remain frozen even when copied pricing templates are configured',()=>{
 const frozen={...row(10,65,24),frozen_final_price:true}
 const output=applyCustomerPriceRows([{...generated[0],pricing_mode:'tier_template',tier_template_id:9}],[frozen],{},42,{configuredSources:{default:true}})
 assert.equal(output.length,1)
 assert.equal(output[0].final_unit_price,65)
 assert.equal(output[0].pricing_mode,'fixed_price')
})

test('a new customer fixed quote survives draft hydration and remains ready to publish', () => {
 const current={...row(527,0),sku_id:527,product_id:974,parent_product_id:974,row_key:'527:fixed_price',pricing_mode:'fixed_price',inventory_unit:'袋'}
 const restored=migrateLegacyFixedPriceFlatRowOverrides({'527:fixed_price':88},{},[{...current}],{customerID:471})
 const [quote]=applyCustomerPriceRows([current],[],restored.flatRowOverrides,471)
 assert.equal(quote.final_unit_price,88)
 assert.equal(quote.customer_quote_missing,false)
 assert.equal(quote.bom_spec_id,527)
 assert.equal(priceListFlatRowsReady([quote]),true)
 assert.equal(priceListFlatRowsReady(applyCustomerPriceRows([current],[],{},471)),false)
})

test('a previously migrated customer fixed quote is recovered only for its exact specification', () => {
 const current={...row(527,88),sku_id:527,product_id:974,parent_product_id:974,row_key:'527:fixed_price',pricing_mode:'fixed_price',fixed_unit_price:88}
 const options={productOverrides:{'sku:527':{scope:'sku',sku_id:527,parent_product_id:974,fixed_unit_price:88}}}
 const [quote]=applyCustomerPriceRows([current],[],{},471,options)
 assert.equal(quote.final_unit_price,88)
 assert.equal(quote.customer_quote_missing,false)
 const other={...current,sku_id:528,bom_spec_id:528,row_key:'528:fixed_price'}
 assert.equal(applyCustomerPriceRows([other],[],{},471,options)[0].customer_quote_missing,true)
 const otherProduct={...current,parent_product_id:975,product_id:975}
 assert.equal(applyCustomerPriceRows([otherProduct],[],{},471,options)[0].customer_quote_missing,true)
})

test('customer manual edits of an inherited fixed quote survive draft hydration', () => {
 const current={...row(527,0),sku_id:527,product_id:974,parent_product_id:974,row_key:'527:fixed_price',pricing_mode:'fixed_price'}
 const seed={...current,final_unit_price:65,customer_price_source_publication_id:155}
 const restored=migrateLegacyFixedPriceFlatRowOverrides({'527:fixed_price':88},{},[current],{customerID:471})
 const [quote]=applyCustomerPriceRows([current],[seed],restored.flatRowOverrides,471)
 assert.equal(quote.final_unit_price,88)
 assert.equal(quote.original_final_unit_price,65)
 assert.equal(quote.customer_quote_missing,false)
 assert.equal(quote.customer_price_source_publication_id,155)
})

test('a migrated BOM-spec quote without parent metadata recovers only its exact BOM identity', () => {
 const current={...row(527,0),sku_id:527,product_id:974,parent_product_id:974,row_key:'527:fixed_price',pricing_mode:'fixed_price'}
 const options={productOverrides:{'sku:527':{scope:'sku',sku_id:527,parent_product_id:0,fixed_unit_price:88}}}
 assert.equal(applyCustomerPriceRows([current],[],{},471,options)[0].final_unit_price,88)
 const ambiguous={...current,bom_spec_id:0}
 assert.equal(applyCustomerPriceRows([ambiguous],[],{},471,options)[0].customer_quote_missing,true)
 const normalized={...current}; delete normalized.sku_id
 assert.equal(applyCustomerPriceRows([normalized],[],{},471,options)[0].final_unit_price,88)
})

test('missing customer quote asks for a fixed price without requiring a public publication', async () => {
 const { readFileSync } = await import('node:fs')
 const view = readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
 const start = view.indexOf('function priceListFlatRowVisibleErrors(')
 const end = view.indexOf('\nfunction priceListFlatRowHasLegacyUnitMismatch', start)
 const errors = new Function('priceListFlatRowErrors', 'priceListFlatRowPricingTrialStatus', 'priceListFlatRowPricingTrialError', `${view.slice(start,end)}; return priceListFlatRowVisibleErrors`)(() => [], () => '', () => '')
 assert.deepEqual(errors({customer_quote_missing:true}),['待报价：请填写客户固定价格。'])
 assert.deepEqual(errors({customer_quote_missing:false}),[])
})

test('the real price-row input handler retains a customer amount through draft reload', async () => {
 const { readFileSync } = await import('node:fs')
 const { ref } = await import('vue')
 const view = readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
 const start = view.indexOf('function setPriceListFlatRowPrice(')
 const end = view.indexOf('\nfunction revertPriceListFlatRowPrice', start)
 const overrides = ref({})
 let saves = 0
 const setPrice = new Function('activeBeanListCustomerID', 'priceListFlatRowOverrides', 'setPriceListSkuFixedPrice', 'savePriceListGenerationDraftForActiveType', `${view.slice(start,end)}; return setPriceListFlatRowPrice`)(
   ref(471), overrides, () => { throw Error('customer quote must stay in row overrides') }, () => { saves++ },
 )
 const current={...row(527,0),sku_id:527,product_id:974,parent_product_id:974,row_key:'527:fixed_price',pricing_mode:'fixed_price',inventory_unit:'袋'}
 setPrice(current,'88')
 assert.equal(saves,1)
 const draft=migrateLegacyFixedPriceFlatRowOverrides(overrides.value,{},[current],{customerID:471})
 const [quote]=applyCustomerPriceRows([current],[],draft.flatRowOverrides,471)
 assert.equal(quote.final_unit_price,88)
 assert.equal(priceListFlatRowsReady([quote]),true)
})

test('explicitly applying a spec group releases frozen copied prices only in that scope', () => {
 const current = {...row(10,30), parent_product_id:10, spec_pricing_group_id:4, pricing_mode:'pricing_rule', pricing_mode_source:'parent_product'}
 const frozen = {...row(10,65), parent_product_id:10, frozen_final_price:true}
 const original = applyCustomerPriceRows([current], [frozen], {}, 42, {configuredSources:{'product:10':true}})
 assert.equal(original[0].final_unit_price,65)
 const applied = applyCustomerPriceRows([current], [frozen], {}, 42, {configuredSources:{'product:10':true,'spec-group:product:10':true}})
 assert.equal(applied[0].final_unit_price,30)
 assert.equal(applied[0].spec_pricing_group_id,4)
})

test('a group draft follows current calculation rather than reusing older published prices', () => {
 const current = {...row(10,30), parent_product_id:10, spec_pricing_group_id:4, pricing_mode:'pricing_rule', pricing_mode_source:'default'}
 const historic = {...row(10,65), parent_product_id:10, frozen_final_price:false}
 assert.equal(applyCustomerPriceRows([current], [historic], {}, 42)[0].final_unit_price,30)
})
