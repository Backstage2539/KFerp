import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
const page=readFileSync(new URL('../views/OrderEntryView.vue',import.meta.url),'utf8')
test('ERP order entry uses price-table specs without all-BOM or temporary conversion entry',()=>{
 assert.doesNotMatch(page,/loadOrderBOMSpecs|selectOrderBOMTier|OrderSpecConversionDrawer|临时换规格/)
 assert.match(page,/orderFamilySpecOptions/)
 assert.match(page,/规格列和规格搜索只使用当前已选已发布价格表中有价的规格/)
})
