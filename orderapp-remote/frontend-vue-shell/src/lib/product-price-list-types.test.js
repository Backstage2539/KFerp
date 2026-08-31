import test from 'node:test'
import assert from 'node:assert/strict'

import {
  businessGroupFeatureSelectionIDs,
  businessGroupRowsForFeatureSelection,
} from './business-grouping.js'

import {
  PRODUCT_CATALOG_PUBLICATION_TYPE_ID_BASE,
  UNCLASSIFIED_PRODUCT_PRICE_LIST_TYPE_ID,
  buildClassificationPriceListTypeOptions,
  buildProductCatalogTemplatePriceListTypeOptions,
  buildProductCatalogPriceListTypeOptions,
  classificationTemplateIDOfItem,
  classificationTemplateIDOfPublication,
  classificationTemplateNameOfPublication,
  matchesProductCatalogPriceListType,
  matchesPublicationProductType,
  matchesProductTypeCategory,
  preferredPublicationForPriceListType,
  priceListTypeOptionForPublication,
  publicationTypeIdentityForPriceListType,
  publicationVersionListState,
  priceListRenderTypeForItem,
  priceListSelectionStateKey,
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

test('product catalog groups drive product price-list types before legacy product categories', () => {
  const groupTemplate = {
    id: 128,
    name: '商品分组',
    items: [
      {
        id: 3296,
        name: '咖啡熟豆',
        parent_id: 0,
        sort_order: 10,
        children: [
          { id: 3297, name: '意式拼配豆', parent_id: 3296, sort_order: 10 },
        ],
      },
    ],
  }
  const rows = [
    {
      product_id: 538,
      name: 'PR439-20260606182321 熟豆下单商品',
      product_kind: 'roasted',
      product_category_id: 3,
      product_type_category_id: 1,
      category_primary_name: '咖啡烘焙豆',
      category_secondary_name: '精品意式拼配',
    },
    {
      product_id: 550,
      name: '熟豆-红岩拼配',
      product_kind: 'roasted',
      product_category_id: 223,
      product_type_category_id: 221,
      category_primary_name: '熟豆',
      category_secondary_name: '默认熟豆',
    },
  ]
  const assignments = [
    { usage_key: 'product_catalog', object_key: 'product', object_id: 538, group_id: 128, group_item_id: 3297 },
    { usage_key: 'product_catalog', object_key: 'product', object_id: 550, group_id: 128, group_item_id: 3297 },
  ]

  const options = buildProductCatalogPriceListTypeOptions(rows, { template: groupTemplate, assignments })

  assert.deepEqual(options.map((option) => option.label), ['咖啡熟豆'])
  assert.equal(options[0].itemCount, 2)
  assert.equal(options[0].listType, 'commercial')
  assert.equal(matchesProductCatalogPriceListType(rows[1], options[0], { assignments }), true)
})

test('product catalog price-list types accept flat business group items', () => {
  const groupTemplate = {
    id: 128,
    name: '商品分组',
    items: [
      { id: 3296, name: '咖啡熟豆', parent_id: 0, sort_order: 10 },
      { id: 3297, name: '意式拼配豆', parent_id: 3296, sort_order: 10 },
    ],
  }
  const rows = [
    {
      product_id: 550,
      name: '熟豆-红岩拼配',
      product_kind: 'roasted',
      product_category_id: 223,
      product_type_category_id: 221,
      category_primary_name: '熟豆',
      category_secondary_name: '默认熟豆',
    },
  ]
  const assignments = [
    { usage_key: 'product_catalog', object_key: 'product', object_id: 550, group_id: 128, group_item_id: 3297 },
  ]

  const options = buildProductCatalogPriceListTypeOptions(rows, { template: groupTemplate, assignments })

  assert.deepEqual(options.map((option) => option.label), ['咖啡熟豆'])
  assert.equal(options[0].itemCount, 1)
  assert.equal(matchesProductCatalogPriceListType(rows[0], options[0], { assignments }), true)
})

test('product catalog price-list types keep their own selection state key', () => {
  const productCatalogType = {
    id: -1003296,
    key: 'product-catalog:128:3296',
    label: '咖啡熟豆',
    listType: 'commercial',
    productCatalogGroupID: 128,
    productCatalogGroupItemID: 3296,
  }

  assert.equal(
    priceListSelectionStateKey([productCatalogType], 'commercial', -1003296),
    'product-catalog:128:3296',
  )
  assert.notEqual(
    priceListSelectionStateKey([productCatalogType], 'commercial', -1003296),
    'legacy:commercial',
  )
})

test('price list product types partition products by the product catalog templates selected by product archive', () => {
  const groups = [
    {
      id: 128,
      name: '商品-咖啡豆',
      active: true,
      usages: [{ usage_key: 'product_catalog', active: true }],
      items: [
        { id: 3296, name: '咖啡熟豆', parent_id: 0, sort_order: 10 },
        { id: 3297, name: '意式拼配豆', parent_id: 3296, sort_order: 10 },
      ],
    },
    {
      id: 129,
      name: '商品-挂耳',
      active: true,
      usages: [{ usage_key: 'product_catalog', active: true }],
      items: [{ id: 3396, name: '挂耳咖啡', parent_id: 0, sort_order: 10 }],
    },
    {
      id: 130,
      name: '历史价格表专用模板',
      active: true,
      usages: [{ usage_key: 'price_list', active: true }],
      items: [{ id: 3496, name: '旧价格表分类', parent_id: 0, sort_order: 10 }],
    },
  ]
  const featureSelection = {
    feature_key: 'product_catalog',
    group_template_ids: [128, 129, 128],
    template_ids: [130],
  }
  const templates = businessGroupRowsForFeatureSelection(groups, businessGroupFeatureSelectionIDs(featureSelection))
  const rows = [
    { product_id: 11, name: '咖啡豆 A', product_kind: 'roasted' },
    { product_id: 111, effective_parent_product_id: 11, parent_product_id: 11, sku_id: 111, name: '咖啡豆 A / 227g', product_kind: 'roasted' },
    { product_id: 12, name: '挂耳 A', product_kind: 'drip_bag' },
    { product_id: 13, name: '未归类商品', product_kind: 'roasted' },
  ]
  const assignments = [
    { usage_key: 'product_catalog', object_key: 'product', object_id: 11, group_id: 128, group_item_id: 3297 },
    { usage_key: 'product_catalog', object_key: 'product', object_id: 12, group_id: 129, group_item_id: 3396 },
    { usage_key: 'price_list', object_key: 'product', object_id: 13, group_id: 130, group_item_id: 3496 },
  ]

  assert.deepEqual(templates.map((template) => template.id), [128, 129])
  const options = buildProductCatalogTemplatePriceListTypeOptions(rows, { templates, assignments })
  assert.deepEqual(options.map((option) => [option.label, option.itemCount]), [
    ['商品-咖啡豆', 1],
    ['商品-挂耳', 1],
  ])

  const membership = new Map(options.map((option) => [
    option.label,
    rows.filter((row) => matchesProductCatalogPriceListType(row, option, { assignments })).map((row) => row.product_id),
  ]))
  assert.deepEqual(membership.get('商品-咖啡豆'), [11, 111])
  assert.deepEqual(membership.get('商品-挂耳'), [12])
  assert.deepEqual(new Set(Array.from(membership.values()).flat().map((id) => id === 111 ? 11 : id)), new Set([11, 12]))
  assert.equal(Array.from(membership.values()).flat().includes(13), false)
})

test('green product catalog keeps the dominant green price-list type when one direct product has commercial metadata', () => {
  const template = {
    id: 618,
    name: '咖啡生豆',
    active: true,
    items: [{ id: 6181, name: '兴福茶咖厂', parent_id: 0, sort_order: 10 }],
  }
  const rows = [
    ...Array.from({ length: 7 }, (_, index) => ({
      product_id: index + 1,
      name: `生豆 ${index + 1}`,
      product_kind: 'green_bean',
      green_bean_list: { code: `G.${index + 1}` },
    })),
    {
      product_id: 8,
      name: '直接商品身份生豆',
      product_kind: 'base_product',
      spec_identity_mode: 'product',
      commercial_bean_list: { code: 'C.1' },
    },
  ]
  const assignments = rows.map((row) => ({
    usage_key: 'product_catalog',
    object_key: 'product',
    object_id: row.product_id,
    group_id: template.id,
    group_item_id: 6181,
  }))

  const [option] = buildProductCatalogTemplatePriceListTypeOptions(rows, {
    templates: [template],
    assignments,
  })

  assert.equal(option.itemCount, 8)
  assert.equal(option.listType, 'green')
})

test('two commercial product catalog templates keep stable isolated publication identities', () => {
  const options = buildProductCatalogTemplatePriceListTypeOptions([], {
    templates: [
      { id: 128, name: '商品-咖啡豆', active: true, items: [] },
      { id: 129, name: '商品-挂耳', active: true, items: [] },
    ],
  })

  assert.deepEqual(options.map((option) => option.listType), ['commercial', 'commercial'])
  assert.deepEqual(options.map((option) => publicationTypeIdentityForPriceListType(option)), [
    {
      productTypeCategoryID: PRODUCT_CATALOG_PUBLICATION_TYPE_ID_BASE + 128,
      classificationTemplateID: PRODUCT_CATALOG_PUBLICATION_TYPE_ID_BASE + 128,
    },
    {
      productTypeCategoryID: PRODUCT_CATALOG_PUBLICATION_TYPE_ID_BASE + 129,
      classificationTemplateID: PRODUCT_CATALOG_PUBLICATION_TYPE_ID_BASE + 129,
    },
  ])
  assert.equal(Number.isSafeInteger(options[0].publicationProductTypeCategoryID), true)
  assert.notEqual(options[0].publicationProductTypeCategoryID, options[1].publicationProductTypeCategoryID)

  const coffeePublication = {
    id: 2,
    status: 'published',
    list_type: 'commercial',
    product_type_category_id: PRODUCT_CATALOG_PUBLICATION_TYPE_ID_BASE + 128,
    classification_template_id: PRODUCT_CATALOG_PUBLICATION_TYPE_ID_BASE + 128,
  }
  const dripPublication = {
    id: 3,
    status: 'published',
    list_type: 'commercial',
    product_type_category_id: PRODUCT_CATALOG_PUBLICATION_TYPE_ID_BASE + 129,
    classification_template_id: PRODUCT_CATALOG_PUBLICATION_TYPE_ID_BASE + 129,
  }
  assert.equal(priceListTypeOptionForPublication(options, coffeePublication)?.id, options[0].id)
  assert.equal(priceListTypeOptionForPublication(options, dripPublication)?.id, options[1].id)
  assert.equal(matchesPublicationProductType(coffeePublication, options[0]), true)
  assert.equal(matchesPublicationProductType(coffeePublication, options[1]), false)

  const legacyGlobal = {
    id: 1,
    status: 'published',
    list_type: 'commercial',
    product_type_category_id: 0,
    classification_template_id: 0,
  }
  assert.equal(preferredPublicationForPriceListType([legacyGlobal, dripPublication, coffeePublication], options[0]), coffeePublication)
  assert.equal(preferredPublicationForPriceListType([legacyGlobal, coffeePublication, dripPublication], options[1]), dripPublication)
})

test('price list uses one safe flat product type when product archive selected no group templates', () => {
  const rows = [
    { product_id: 11, name: '咖啡豆 A', product_kind: 'roasted' },
    { product_id: 111, effective_parent_product_id: 11, parent_product_id: 11, sku_id: 111, name: '咖啡豆 A / 227g', product_kind: 'roasted' },
    { product_id: 12, name: '挂耳 A', product_kind: 'drip_bag' },
  ]
  const groups = [
    { id: 130, name: '历史价格表专用模板', active: true, usages: [{ usage_key: 'price_list', active: true }] },
  ]
  const featureSelection = {
    feature_key: 'product_catalog',
    group_template_ids: [],
    template_ids: [130],
  }
  const templates = businessGroupRowsForFeatureSelection(groups, businessGroupFeatureSelectionIDs(featureSelection))

  assert.deepEqual(templates, [])
  const options = buildProductCatalogTemplatePriceListTypeOptions(rows, {
    templates,
    assignments: [
      { usage_key: 'price_list', object_key: 'product', object_id: 11, group_id: 130, group_item_id: 3496 },
    ],
  })
  assert.deepEqual(options.map((option) => ({ key: option.key, label: option.label, itemCount: option.itemCount })), [
    { key: 'product-catalog:flat', label: '全部商品', itemCount: 2 },
  ])
  assert.equal(rows.every((row) => matchesProductCatalogPriceListType(row, options[0], { assignments: [] })), true)
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
  const legacyGlobalCommercial = { product_type_category_id: 0, product_type_name: '商品价格表', list_type: 'commercial', publication_purpose: 'factory_supply', content: { groups: [] } }
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
  assert.equal(matchesPublicationProductType(legacyGlobalCommercial, 3), true)
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
