import test from 'node:test'
import assert from 'node:assert/strict'
import { buildBeanListPdfGroupsFromCategoryRows, applyPriceListFlatRowsToBeanListPdfGroups, buildPriceListGenerationSnapshot } from './bean-list-pdf.js'

// PDF and long-image generation receive the same published content groups.
test('三规格独立价格进入预览和导出快照且不串价', () => {
  const specs = [
    { id: 11, name:'227g', price:33.80, rule:101 },
    { id: 12, name:'100g', price:16.80, rule:102 },
    { id: 13, name:'50g', price:9.80, rule:103 },
  ]
  const sourceItems = specs.map((s) => ({ product_id:42, parent_product_id:42, sku_id:0,
    bom_id:3, bom_version_id:4, bom_spec_id:s.id, bom_variant_id:s.id+100,
    sku_name:s.name, spec_label:s.name, __price_list_sales_spec_label:s.name,
    name:'模拟咖啡', product_name:'模拟咖啡', inventory_unit:'袋', quote_unit:'袋',
    commercial_bean_list:{code:'1.1',category:'咖啡豆',display_name:'模拟咖啡'},
  }))
  const rows = specs.map((s) => ({ product_id:s.id, parent_product_id:42, sku_id:s.id,
    bom_spec_id:s.id, bom_variant_id:s.id+100, product_name:'模拟咖啡',
    row_key:`${s.id}:group:1:3:${s.rule}`, spec_pricing_group_id:1, spec_pricing_group_revision:3,
    pricing_mode:'pricing_rule', pricing_rule_id:s.rule, pricing_rule_version:'v1',
    quantity_basis:'sales_spec_count', min_qty:0, max_qty:null, tier_label:'基础价',
    price_unit:'袋', inventory_unit:'袋', inventory_conversion_json:{袋:{袋:1}},
    final_unit_price:s.price, original_final_unit_price:s.price,
  }))
  const groups = buildBeanListPdfGroupsFromCategoryRows([{code:'coffee',label:'咖啡豆',items:sourceItems}], 'commercial', { selectedProductIDs: specs.map(s => String(s.id)) })
  const displayed = applyPriceListFlatRowsToBeanListPdfGroups(groups, rows, 'commercial')
  assert.equal(displayed.length, 1)
  assert.equal(displayed[0].items.length, 3)
  for (let i=0;i<specs.length;i++) {
    const item = displayed[0].items[i]
    assert.equal(item.bom_spec_id, specs[i].id)
    assert.deepEqual(item.prices, [{label:'基础价',price:specs[i].price,unit:'袋',red:false}])
    assert.ok(item.productAttributes.some(attr => attr.label === '规格' && attr.value === specs[i].name))
  }
  const snapshot = buildPriceListGenerationSnapshot({ rows, defaults:{pricing_mode:'spec_pricing_group',spec_pricing_group_id:1} })
  assert.deepEqual(snapshot.content.price_rows.map(r => [r.bom_spec_id,r.final_unit_price,r.pricing_rule_id,r.spec_pricing_group_revision]), specs.map(s => [s.id,s.price,s.rule,3]))
})

test('BOM 规格匹配同时限定父商品，保留旧 SKU 精确匹配', () => {
  const groups = [{category:'咖啡',items:[
    {product_id:42,parent_product_id:42,sku_id:0,bom_spec_id:11,name:'同名'},
    {product_id:52,parent_product_id:52,sku_id:0,bom_spec_id:11,name:'同名'},
    {product_id:61,parent_product_id:60,sku_id:61,name:'旧规格'},
  ]}]
  const rows = [
    {product_id:11,parent_product_id:42,sku_id:11,bom_spec_id:11,final_unit_price:10,price_unit:'袋',inventory_unit:'袋'},
    {product_id:11,parent_product_id:52,sku_id:11,bom_spec_id:11,final_unit_price:20,price_unit:'袋',inventory_unit:'袋'},
    {product_id:61,parent_product_id:60,sku_id:61,final_unit_price:30,price_unit:'袋',inventory_unit:'袋'},
  ]
  const result = applyPriceListFlatRowsToBeanListPdfGroups(groups, rows, 'commercial')
  assert.deepEqual(result[0].items.map(item => item.prices.map(price => price.price)), [[10],[20],[30]])
})
