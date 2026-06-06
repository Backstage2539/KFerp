import test from 'node:test'
import assert from 'node:assert/strict'

import {
  UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID,
  buildClassificationPriceListTypeOptions,
  classificationTemplateIDOfItem,
  classificationTemplateIDOfPublication,
  classificationTemplateNameOfPublication,
  matchesPublicationProductType,
  matchesProductTypeCategory,
  publicationVersionListState,
  priceListRenderTypeForItem,
} from './product-price-list-types.js'

test('price list type options mirror product archive classification tabs', () => {
  const options = buildClassificationPriceListTypeOptions([
    { product_id: 1, name: '未归类生豆', product_kind: 'green_bean', product_type_category_id: 19, product_type_name: '咖啡生豆' },
    { product_id: 2, name: '未归类挂耳', product_kind: 'drip_bag', product_type_category_id: 2, product_type_name: '挂耳' },
    { product_id: 3, name: '挂耳 A', classification_template_id: 2, classification_template_name: '咖啡挂耳', classification_category_id: 0, classification_category_name: '未分类', product_type_category_id: 182 },
    { product_id: 4, name: '挂耳 B', classification_template_id: 2, classification_template_name: '咖啡挂耳', classification_category_id: 0, classification_category_name: '未分类', product_type_category_id: 182 },
    { product_id: 5, name: '速溶 A', classification_template_id: 3, classification_template_name: '速溶咖啡', classification_category_id: 0, classification_category_name: '未分类', product_type_category_id: 182 },
  ])

  assert.deepEqual(options.map((option) => option.label), ['未分类商品', '咖啡挂耳', '速溶咖啡'])
  assert.equal(options[0].id, UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID)
  assert.equal(options[0].itemCount, 2)
  assert.equal(options.find((option) => option.label === '咖啡挂耳')?.itemCount, 2)
  assert.equal(options.find((option) => option.label === '速溶咖啡')?.itemCount, 1)
})

test('legacy product type id does not count as current product archive classification', () => {
  const legacyOnly = { product_id: 1, product_type_category_id: 19, product_type_name: '咖啡生豆', product_kind: 'green_bean' }
  const classified = { product_id: 2, classification_template_id: 3, classification_template_name: '速溶咖啡', product_type_category_id: 182 }

  assert.equal(classificationTemplateIDOfItem(legacyOnly), 0)
  assert.equal(matchesProductTypeCategory(legacyOnly, UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID), true)
  assert.equal(matchesProductTypeCategory(legacyOnly, 3), false)
  assert.equal(matchesProductTypeCategory(classified, UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID), false)
  assert.equal(matchesProductTypeCategory(classified, 3), true)
})

test('direct product categories drive price list type options after template removal', () => {
  const legacyOnly = { product_id: 1, product_type_category_id: 19, product_type_name: '咖啡生豆', product_kind: 'green_bean' }
  const directLeaf = {
    product_id: 2,
    product_category_id: 7,
    product_type_category_id: 3,
    product_type_name: '咖啡烘焙豆',
    product_subtype_category_id: 7,
    product_subtype_name: '工厂量单',
    category_primary_name: '咖啡烘焙豆',
    category_secondary_name: '工厂量单',
    category_primary_position: 20,
  }
  const directParent = {
    product_id: 3,
    product_category_id: 8,
    product_type_category_id: 8,
    product_type_name: '周边商品',
    category_primary_name: '周边商品',
    category_primary_position: 30,
  }

  const options = buildClassificationPriceListTypeOptions([legacyOnly, directParent, directLeaf])

  assert.equal(classificationTemplateIDOfItem(legacyOnly), 0)
  assert.equal(classificationTemplateIDOfItem(directLeaf), 3)
  assert.equal(matchesProductTypeCategory(directLeaf, 3), true)
  assert.equal(matchesProductTypeCategory(directLeaf, UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID), false)
  assert.deepEqual(options.map((option) => option.label), ['未分类商品', '咖啡烘焙豆', '周边商品'])
  assert.equal(options.find((option) => option.label === '咖啡烘焙豆')?.itemCount, 1)
})

test('unclassified legacy green bean still renders with green bean price rows', () => {
  const legacyGreen = { product_id: 1, product_type_category_id: 19, product_type_name: '咖啡生豆', product_kind: 'green_bean' }
  const classifiedDrip = { product_id: 2, classification_template_id: 2, classification_template_name: '咖啡挂耳', product_kind: 'drip_bag' }

  assert.equal(classificationTemplateIDOfItem(legacyGreen), 0)
  assert.equal(priceListRenderTypeForItem(legacyGreen), 'green')
  assert.equal(priceListRenderTypeForItem(classifiedDrip), 'commercial')
})

test('published price list rows use current classification instead of legacy product type labels', () => {
  const legacyOther = { product_type_category_id: 154, product_type_name: '其他', list_type: 'commercial', content: { groups: [] } }
  const classified = { classification_template_id: 3, classification_template_name: '速溶咖啡', product_type_category_id: 154, product_type_name: '其他' }
  const inferredFromContent = {
    product_type_category_id: 0,
    product_type_name: '其他',
    content: {
      groups: [
        { items: [{ classification_template_id: 2, classification_template_name: '咖啡挂耳' }] },
      ],
    },
  }

  assert.equal(classificationTemplateIDOfPublication(legacyOther), 0)
  assert.equal(matchesPublicationProductType(legacyOther, UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID), true)
  assert.equal(matchesPublicationProductType(legacyOther, 3), false)
  assert.equal(classificationTemplateNameOfPublication(classified), '速溶咖啡')
  assert.equal(matchesPublicationProductType(classified, 3), true)
  assert.equal(classificationTemplateIDOfPublication(inferredFromContent), 2)
  assert.equal(classificationTemplateNameOfPublication(inferredFromContent), '咖啡挂耳')
})

test('published price list version list supports search pagination and collapsed state', () => {
  const rows = Array.from({ length: 13 }, (_, index) => ({
    id: index + 1,
    version: `V${index + 1}`,
    changelog: index === 11 ? '曹杰专属新版' : '常规更新',
    owner_type: index === 11 ? 'customer' : 'official',
    owner_key: index === 11 ? 169 : 0,
    status: index === 12 ? 'withdrawn' : 'published',
    publication_purpose: index === 11 ? 'customer_resale' : 'factory_supply',
  }))

  const firstPage = publicationVersionListState(rows, { page: 1, pageSize: 5, collapsed: false })
  assert.equal(firstPage.total, 13)
  assert.equal(firstPage.totalPages, 3)
  assert.deepEqual(firstPage.rows.map((row) => row.id), [1, 2, 3, 4, 5])
  assert.equal(firstPage.collapsed, false)

  const searched = publicationVersionListState(rows, { query: '曹杰 169', page: 2, pageSize: 5 })
  assert.equal(searched.page, 1)
  assert.equal(searched.total, 1)
  assert.deepEqual(searched.rows.map((row) => row.id), [12])

  const collapsed = publicationVersionListState(rows, { collapsed: true, page: 2, pageSize: 5 })
  assert.equal(collapsed.total, 13)
  assert.equal(collapsed.totalPages, 3)
  assert.deepEqual(collapsed.rows, [])
})
