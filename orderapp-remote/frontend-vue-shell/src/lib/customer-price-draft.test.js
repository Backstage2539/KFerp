import {test} from 'node:test'
import assert from 'node:assert/strict'
import {seedCustomerPriceRows,applyCustomerPriceRows} from './customer-price-draft.js'
const row=(spec,price,min=0,max=null)=>({parent_product_id:1,product_id:1,sku_id:1,bom_spec_id:spec,price_unit:'袋',final_unit_price:price,min_qty:min,max_qty:max,row_key:`${spec}:${min}`,product_name:'工厂名',group_snapshot:{group_item_name:'工厂分类'}})
const generated=[{...row(10,999),product_name:'客户名',group_snapshot:{group_item_name:'客户分类'},customer_reference_snapshot:{customer_id:42}}]
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
