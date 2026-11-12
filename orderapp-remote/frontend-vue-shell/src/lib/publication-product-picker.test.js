import test from 'node:test'
import assert from 'node:assert/strict'
import { publicationProducts, publicationProductPickerError } from './publication-product-picker.js'

test('按商品身份对已发布价格表多规格行去重，不按名称匹配', () => {
 const source={id:8,status:'published',owner_type:'official',publication_purpose:'factory_supply',content:{price_rows:[{parent_product_id:7,product_id:71,product_name:'同名豆'},{parent_product_id:7,product_id:72,product_name:'同名豆'},{parent_product_id:8,product_name:'同名豆'}]}}
 assert.deepEqual(publicationProducts(source).map(row=>row.id),[7,8])
 assert.equal(publicationProductPickerError(source), '')
})

test('不接受不完整、未发布或非公共供货价格表', () => {
 const source={id:8,status:'published',owner_type:'official',publication_purpose:'factory_supply',content:{price_rows:[{parent_product_id:7},{product_name:'缺身份'}]}}
 assert.match(publicationProductPickerError(source), /商品身份/)
 assert.match(publicationProductPickerError({...source,status:'withdrawn'}), /已发布/)
 assert.match(publicationProductPickerError({...source,owner_type:'customer'}), /公共/)
 assert.match(publicationProductPickerError({...source,content:{price_rows:[]}}), /商品/)
})
