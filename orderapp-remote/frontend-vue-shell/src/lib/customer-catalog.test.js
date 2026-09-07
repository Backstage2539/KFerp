import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {customerCatalogProjection,customerCatalogCopyPayload,customerCatalogCustomerID,customerCatalogProductRows} from './customer-catalog.js'
test('customer catalog projects source identities with independent names and full tree',()=>{
 const data={nodes:[{id:90,source_group_id:10,source_item_id:0,name:'客户豆单',sort_order:1},{id:91,source_group_id:10,source_item_id:11,parent_source_item_id:0,name:'客户精品',sort_order:1},{id:92,source_group_id:10,source_item_id:12,parent_source_item_id:11,name:'客户产地',sort_order:1}],assignments:[{object_id:1,group_id:10,group_item_id:12}]}
 const p=customerCatalogProjection(data);assert.equal(p.groups[0].id,10);assert.equal(p.groups[0].name,'客户豆单');assert.equal(p.groups[0].items[0].children[0].name,'客户产地');assert.deepEqual(p.assignments,data.assignments)
})
test('all copy never uses loaded or selected products; selected copy deduplicates',()=>{
 assert.deepEqual(customerCatalogCopyPayload(42,'all',[1]),{customer_id:42,mode:'all',product_ids:[]})
 assert.deepEqual(customerCatalogCopyPayload(42,'selected',[1,1,2]),{customer_id:42,mode:'selected',product_ids:[1,2]})
 assert.throws(()=>customerCatalogCopyPayload(0,'all',[]));assert.throws(()=>customerCatalogCopyPayload(42,'selected',[]))
})
test('ownership filter resolves customer alias and never another customer name',()=>{
 assert.equal(customerCatalogCustomerID('customer:42',0),42)
 const refs=[{product_id:1,customer_id:42,customer_display_name:'客户名',active:true},{product_id:1,customer_id:43,customer_display_name:'其他名',active:true}]
 const rows=customerCatalogProductRows([{id:1,name:'原名'}],refs,42);assert.equal(rows[0].name,'客户名');assert.equal(rows[0].canonical_name,'原名')
})
test('three copy actions share the API and customer editing cannot write the product master',()=>{
 const s=readFileSync(new URL('../views/ProductSettingsView.vue',import.meta.url),'utf8')
 assert.ok(s.includes('复制全部商品到客户'));assert.ok(s.includes('复制所选商品到客户'))
 assert.ok(s.includes('/api/product-settings/customer-catalog/copy'));assert.ok(s.includes('openCustomerAwareProductName'))
 const costing=readFileSync(new URL('../views/CostingView.vue',import.meta.url),'utf8');assert.ok(costing.includes('customerCatalogProjection'))
})
test('copy feedback remains visible in the product page and price ownership searches the complete customer directory',()=>{
 const product=readFileSync(new URL('../views/ProductSettingsView.vue',import.meta.url),'utf8')
 assert.match(product.slice(0,product.indexOf('<section class="settings-workbench">')),/role="status"/)
 const price=readFileSync(new URL('../views/CostingView.vue',import.meta.url),'utf8')
 assert.match(price,/<SearchableSelect[\s\S]*?v-model="versionListScope"/)
 assert.match(price,/await fetchAllCustomerOptions\(\)/)
})
import {buildProductCatalogTemplatePriceListTypeOptions,matchesProductCatalogPriceListType} from './product-price-list-types.js'
test('customer price picker keeps uncategorized copied products selectable',()=>{
 const items=[{product_id:1,name:'已分类'},{product_id:2,name:'未分类'}]
 const assignments=[{group_id:10,group_item_id:11,object_id:1,object_key:'product',usage_key:'product_catalog'}]
 const types=buildProductCatalogTemplatePriceListTypeOptions(items,{templates:[{id:10,name:'客户类',items:[{id:11,name:'子类'}]}],assignments,includeUnclassified:true})
 const unclassified=types.find(t=>t.productCatalogUnclassified)
 assert.ok(unclassified);assert.equal(unclassified.itemCount,1)
 assert.equal(matchesProductCatalogPriceListType(items[1],unclassified,{assignments}),true)
 assert.equal(matchesProductCatalogPriceListType(items[0],unclassified,{assignments}),false)
})
