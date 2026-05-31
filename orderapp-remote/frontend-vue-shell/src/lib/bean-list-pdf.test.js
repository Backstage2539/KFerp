import test from 'node:test'
import assert from 'node:assert/strict'

import {
  DEFAULT_BEAN_LIST_PDF_VERSION,
  applyCustomerProductAliasesToBeanListItems,
  buildBeanListPdfGroups,
  buildBeanListPdfTitle,
  beanListPublicationPdfOptions,
  copyBeanListPublicationContentGroups,
  copyBeanListPublicationConfig,
  defaultBeanListDraftVersion,
  filterBeanListItemsForScope,
  nextBeanListVersion,
  priceUnit,
  sanitizeBeanListPdfTheme,
  splitHighlightedText,
} from './bean-list-pdf.js'

const rows = [
  {
    product_id: 10,
    name: 'Uraga乌拉嘎',
    commercial_bean_list: {
      code: '5.2',
      category: '5、原产地精选豆：',
      display_name: 'Uraga乌拉嘎',
      recommended_use: '手冲/SOE/冷萃',
      flavor: '明显的花香、柑橘、荔枝，红糖甜，绿茶',
      description: '埃塞·古吉·Uraga、74112水洗处理、浅度烘焙（新产季埃塞水洗）',
    },
    retail_bean_list: {
      code: '3.2',
      category: '3、原产地精选豆：',
      display_name: 'Uraga乌拉嘎',
      recommended_use: '手冲/SOE/冷萃',
      flavor: '明显的花香、柑橘、荔枝，红糖甜，绿茶',
      description: '埃塞·古吉·Uraga、74112水洗处理、浅度烘焙（新产季埃塞水洗）',
    },
    commercial_wholesale_tiers: [{ label: '2包-13包', spec_g: 454, price_per_unit: 119 }],
    retail_bean_tiers: [{ label: '227g', price_per_unit: 82 }],
  },
  {
    product_id: 20,
    name: '曲奇拼配',
    commercial_bean_list: {
      code: '1.1',
      category: '1、工厂量单',
      display_name: '曲奇拼配',
      flavor: '坚果、焦糖、巧克力曲奇',
    },
    retail_bean_list: {},
    commercial_wholesale_tiers: [{ label: '24-49kg', spec_g: 1000, price_per_unit: 82 }],
    retail_bean_tiers: [],
  },
  {
    product_id: 30,
    name: 'Nenka',
    commercial_bean_list: {
      code: '6.1',
      category: '6、差异性爆款：',
      display_name: 'Nenka',
      recommended_use: '手冲/SOE',
      flavor: '热带水果、花香',
      description: '卡蒂姆日晒、中度烘焙（庄园差异性产品）',
    },
    retail_bean_list: {
      code: '4.1',
      category: '4、差异性爆款：',
      display_name: 'Nenka',
      recommended_use: '手冲/SOE',
      flavor: '热带水果、花香',
      description: '卡蒂姆日晒、中度烘焙（庄园差异性产品）',
    },
    commercial_wholesale_tiers: [{ label: '2包-13包', spec_g: 454, price_per_unit: 127 }],
    retail_bean_tiers: [{ label: '227g', price_per_unit: 88 }],
  },
]

test('PDF bean-list helper defaults to V3.0.5 and keeps mobile print theme settings', () => {
  assert.equal(DEFAULT_BEAN_LIST_PDF_VERSION, 'V3.0.5')

  const theme = sanitizeBeanListPdfTheme({
    listType: 'retail',
    version: ' V3.0.6 ',
    backgroundColor: '#112233',
    fontColor: '#fafafa',
    backgroundImage: 'data:image/png;base64,abc',
  })

  assert.equal(theme.listType, 'retail')
  assert.equal(theme.version, 'V3.0.6')
  assert.equal(theme.backgroundColor, '#112233')
  assert.equal(theme.fontColor, '#fafafa')
  assert.equal(theme.backgroundImage, 'data:image/png;base64,abc')
})

test('PDF bean-list helper carries selected product special KV attributes into snapshot items', () => {
  const groups = buildBeanListPdfGroups([{
    product_id: 8801,
    name: '速溶盒装',
    commercial_bean_list: { code: '8.1', category: '8、速溶咖啡', display_name: '速溶盒装' },
    product_attributes: [
      { key: 'roast_level', label: '烘焙度', value: '中深烘' },
      { key: 'caffeine', label: '咖啡因', value: '低因' },
    ],
    commercial_wholesale_tiers: [{ label: '10盒起', display_unit: '盒', price_per_unit: 15 }],
  }], 'commercial')

  assert.deepEqual(groups[0].items[0].productAttributes, [
    { key: 'roast_level', label: '烘焙度', value: '中深烘' },
    { key: 'caffeine', label: '咖啡因', value: '低因' },
  ])
  assert.deepEqual(groups[0].items[0].attributeLines, ['烘焙度：中深烘', '咖啡因：低因'])
})

test('PDF bean-list helper increments customer draft versions by the next 0.01-style suffix', () => {
  assert.equal(nextBeanListVersion('V1'), 'V1.01')
  assert.equal(nextBeanListVersion('V1.01'), 'V1.02')
  assert.equal(nextBeanListVersion('V1.09'), 'V1.10')
  assert.equal(nextBeanListVersion('V3.0.5'), 'V3.0.6')
  assert.equal(nextBeanListVersion(''), DEFAULT_BEAN_LIST_PDF_VERSION)
})

test('PDF bean-list helper prefers current customer version before copied price source version', () => {
  assert.equal(defaultBeanListDraftVersion([{ version: 'V1', status: 'published' }], { version: 'V9' }), 'V1.01')
  assert.equal(defaultBeanListDraftVersion([{ version: 'V1.01', status: 'published' }], { version: 'V1' }), 'V1.02')
  assert.equal(defaultBeanListDraftVersion([], { version: 'V1' }), 'V1.01')
})

test('PDF bean-list helper builds separate commercial and retail groups from Excel metadata', () => {
  const commercial = buildBeanListPdfGroups(rows, 'commercial')
  const retail = buildBeanListPdfGroups(rows, 'retail')

  assert.deepEqual(commercial.map((group) => group.category), ['1、工厂量单', '5、原产地精选豆：', '6、差异性爆款：'])
  assert.equal(commercial[0].items[0].code, '1.1')
  assert.equal(commercial[0].items[0].prices[0].unit, 'kg')
  assert.equal(retail.length, 2)
  assert.equal(retail[0].items[0].code, '3.2')
  assert.equal(retail[0].items[0].recommendedUse, '手冲/SOE/冷萃')
  assert.equal(buildBeanListPdfTitle('commercial'), '棵凡咖啡批发产品价格表')
  assert.equal(buildBeanListPdfTitle('retail'), '棵凡咖啡零售产品价格表')
})

test('PDF bean-list helper builds a green bean list from template tiers and quality data', () => {
  const groups = buildBeanListPdfGroups([{
    product_id: 90,
    product_kind: 'green_bean',
    name: '埃塞瑰夏生豆',
    green_bean_list: {
      code: 'G.1',
      category: '生豆销售',
      display_name: '埃塞瑰夏生豆',
      flavor: '茉莉、柑橘',
      description: '水洗处理，阶梯模板报价',
    },
    bean_list_quality: {
      factory_flavor_description: '茉莉花、柑橘',
      moisture: '10.8%',
      density: '780g/L',
      inspection_created_at: '2026-05-18 09:30',
    },
    green_bean_sale_tiers: [{ label: '1kg+', spec_g: 1000, price_per_unit: 128, display_unit: 'kg' }],
  }], 'green')

  assert.equal(buildBeanListPdfTitle('green'), '棵凡咖啡生豆产品价格表')
  assert.equal(groups.length, 1)
  assert.equal(groups[0].category, 'G、生豆销售')
  assert.equal(groups[0].categoryCode, 'G')
  assert.equal(groups[0].items[0].name, '埃塞瑰夏生豆')
  assert.equal(groups[0].items[0].prices[0].price, 128)
  assert.equal(groups[0].items[0].prices[0].unit, 'kg')
  assert.deepEqual(groups[0].items[0].green_bean_sale_tiers, [
    { label: '1kg+', spec_g: 1000, price_per_unit: 128, display_unit: 'kg' },
  ])
  assert.deepEqual(groups[0].items[0].beanListQuality, {
    factoryFlavorDescription: '茉莉花、柑橘',
    moisture: '10.8%',
    density: '780g/L',
    inspectionCreatedAt: '2026-05-18 09:30',
  })
  assert.deepEqual(groups[0].items[0].qualityLines, [
    { label: '工厂风味', value: '茉莉花、柑橘' },
    { label: '水分', value: '10.8%' },
    { label: '密度', value: '780g/L' },
    { label: '质检时间', value: '2026-05-18 09:30' },
  ])
})

test('PDF bean-list helper applies manual green bean kg price overrides to kg template tiers', () => {
  const groups = buildBeanListPdfGroups([{
    product_id: 90,
    product_kind: 'green_bean',
    name: '兰卡拼配生豆',
    green_bean_list: {
      code: 'G.1',
      category: '生豆销售',
      display_name: '兰卡拼配生豆',
    },
    green_bean_sale_tiers: [
      { label: '24-49kg', template_tier_id: 2401, spec_g: 1000, price_per_unit: 60, price_per_lb: 27.24, display_unit: 'kg' },
      { label: '60kg+', template_tier_id: 2402, spec_g: 1000, price_per_unit: 51.75, price_per_lb: 23.49, display_unit: 'kg' },
    ],
  }], 'green', {
    customizers: {
      90: {
        greenPriceOverrides: {
          2402: 62,
        },
      },
    },
  })

  assert.equal(groups[0].items[0].prices[0].price, 60)
  assert.equal(groups[0].items[0].prices[0].unit, 'kg')
  assert.equal(groups[0].items[0].prices[1].price, 62)
  assert.equal(groups[0].items[0].prices[1].unit, 'kg')
  assert.equal(groups[0].items[0].green_bean_sale_tiers[0].price_per_unit, 60)
  assert.equal(groups[0].items[0].green_bean_sale_tiers[0].price_per_lb, 27.24)
  assert.equal(groups[0].items[0].green_bean_sale_tiers[1].display_unit, 'kg')
  assert.equal(groups[0].items[0].green_bean_sale_tiers[1].price_unit, 'kg')
  assert.equal(groups[0].items[0].green_bean_sale_tiers[1].price_per_unit, 62)
  assert.equal(groups[0].items[0].green_bean_sale_tiers[1].price_per_lb, 28.15)
  assert.equal(groups[0].items[0].green_bean_sale_tiers[1].price_per_kg, 62)
})

test('bean-list scope filter keeps customer SKUs isolated by customer', () => {
  const scopedRows = [
    { product_id: 1, name: '公共豆', customer_id: 0 },
    { product_id: 2, name: '客户 A 专属', customer_id: 42 },
    { product_id: 3, name: '客户 B 专属', customer_id: 88 },
  ]

  assert.deepEqual(filterBeanListItemsForScope(scopedRows, 'official', 42).map((item) => item.product_id), [1])
  assert.deepEqual(filterBeanListItemsForScope(scopedRows, 'customer', 42).map((item) => item.product_id), [1, 2])
  assert.deepEqual(filterBeanListItemsForScope(scopedRows, 'customer', 0).map((item) => item.product_id), [1])
})

test('customer bean-list scope hides public catalog when public categories are disabled', () => {
  const scopedRows = [
    { product_id: 1, name: '公共豆', customer_id: 0 },
    { product_id: 2, name: '客户 A 专属', customer_id: 42 },
    { product_id: 3, name: '客户 B 专属', customer_id: 88 },
  ]

  assert.deepEqual(filterBeanListItemsForScope(scopedRows, 'customer', 42, { usePublicCategories: false }).map((item) => item.product_id), [2])
  assert.deepEqual(filterBeanListItemsForScope(scopedRows, 'customer', 42, { usePublicCategories: true }).map((item) => item.product_id), [1, 2])
})

test('PDF commercial price units follow gradient template display units', () => {
  const groups = buildBeanListPdfGroups([{
    product_id: 40,
    name: '小包装模板豆',
    commercial_bean_list: { code: '2.1', category: '2、小包装', display_name: '小包装模板豆' },
    commercial_wholesale_tiers: [{ label: '10份+', spec_g: 100, display_unit: 'g100', price_per_unit: 9 }],
  }], 'commercial')

  assert.equal(groups[0].items[0].prices[0].unit, '100g')
})

test('PDF commercial price units keep custom quote units from product price list snapshots', () => {
  assert.equal(priceUnit({ display_unit: '盒', spec_g: 100, price_per_unit: 15 }), '盒')

  const groups = buildBeanListPdfGroups([{
    product_id: 41,
    name: '速溶盒装',
    commercial_bean_list: { code: '8.1', category: '8、速溶咖啡', display_name: '速溶盒装' },
    commercial_wholesale_tiers: [{ label: '10盒起', spec_g: 100, display_unit: '盒', price_per_unit: 15 }],
  }], 'commercial')

  assert.equal(groups[0].items[0].prices[0].unit, '盒')
})

test('PDF drip bean-list helper expands live bag tiers to bag and box prices', () => {
  const groups = buildBeanListPdfGroups([{
    product_id: 50,
    name: '耶加挂耳',
    product_kind: 'drip_bag',
    drip_box_bag_count: 10,
    drip_bean_list: { code: '1.1', category: '1、挂耳', display_name: '耶加挂耳' },
    drip_wholesale_tiers: [{ label: '100袋', packed_price_per_bag: 3 }],
  }], 'drip')

  assert.deepEqual(groups[0].items[0].prices, [
    { label: '100袋', price: 3, unit: '袋', red: false },
    { label: '100袋', price: 30, unit: '盒(10袋)', red: false },
  ])
})

test('PDF drip bean-list helper preserves published box tier snapshots', () => {
  const groups = buildBeanListPdfGroups([{
    product_id: 51,
    name: '花魁挂耳',
    product_kind: 'drip_bag',
    drip_bean_list: { code: '1.2', category: '1、挂耳', display_name: '花魁挂耳' },
    drip_wholesale_tiers: [
      { label: '100袋', sales_unit: 'bag', unit_bag_count: 1, price_per_unit: 3 },
      { label: '100袋', sales_unit: 'box', unit_bag_count: 10, price_per_unit: 30 },
    ],
  }], 'drip')

  assert.deepEqual(groups[0].items[0].prices, [
    { label: '100袋', price: 3, unit: '袋', red: false },
    { label: '100袋', price: 30, unit: '盒(10袋)', red: false },
  ])
})

test('PDF bean-list helper supports product selection, category filtering, and Excel-style renumbering', () => {
  const groups = buildBeanListPdfGroups(rows, 'commercial', {
    selectedProductIDs: [10, 30],
    showCategoryNumbers: true,
    visibleCategoryCodes: ['5', '6'],
  })

  assert.deepEqual(groups.map((group) => group.categoryCode), ['1', '2'])
  assert.deepEqual(groups.map((group) => group.originalCategoryCode), ['5', '6'])
  assert.equal(groups[0].items[0].code, '1.2')
  assert.equal(groups[1].items[0].code, '2.1')

  const flat = buildBeanListPdfGroups(rows, 'commercial', {
    selectedProductIDs: [10, 30],
    showCategoryNumbers: false,
  })
  assert.equal(flat.length, 1)
  assert.deepEqual(flat[0].items.map((item) => item.code), ['1', '2'])

  const none = buildBeanListPdfGroups(rows, 'commercial', {
    selectedProductIDs: [],
    visibleCategoryCodes: [],
  })
  assert.deepEqual(none, [])
})

test('PDF bean-list helper preserves layout, brand, changelog, badge, and red-highlight settings', () => {
  const theme = sanitizeBeanListPdfTheme({
    listType: 'commercial',
    brandName: '烘豆实验室',
    layoutStyle: 'table',
    cardsPerRow: '3',
    logoImage: 'data:image/png;base64,logo',
    brandIntro: '专注精品咖啡烘焙',
    showVersion: false,
    showChangelog: true,
    changelog: 'V3.0.6 调整庄园精品豆',
  })

  assert.equal(theme.layoutStyle, 'table')
  assert.equal(theme.brandName, '烘豆实验室')
  assert.equal(theme.cardsPerRow, 3)
  assert.equal(theme.logoImage, 'data:image/png;base64,logo')
  assert.equal(theme.brandIntro, '专注精品咖啡烘焙')
  assert.equal(theme.showVersion, false)
  assert.equal(theme.showChangelog, true)
  assert.equal(theme.changelog, 'V3.0.6 调整庄园精品豆')
  assert.equal(buildBeanListPdfTitle('commercial', theme.brandName), '烘豆实验室批发产品价格表')
  assert.equal(buildBeanListPdfTitle('retail', theme.brandName), '烘豆实验室零售产品价格表')

  const groups = buildBeanListPdfGroups(rows, 'commercial', {
    selectedProductIDs: [30],
    customizers: {
      30: {
        badge: 'new',
        highlightTerms: ['庄园差异性产品', '127/包'],
      },
    },
  })
  const item = groups[0].items[0]
  assert.equal(item.badge, 'new')
  assert.deepEqual(item.highlightTerms, ['庄园差异性产品', '127/包'])
  assert.equal(item.prices[0].red, false)

  assert.deepEqual(splitHighlightedText(item.description, item.highlightTerms), [
    { text: '卡蒂姆日晒、中度烘焙（', red: false },
    { text: '庄园差异性产品', red: true },
    { text: '）', red: false },
  ])
  assert.deepEqual(splitHighlightedText('127/包', item.highlightTerms), [{ text: '127/包', red: true }])
  assert.deepEqual(splitHighlightedText('55/包', ['55/包']), [{ text: '55/包', red: true }])
})

test('PDF bean-list helper copies a published configuration for editing against current items', () => {
  const copied = copyBeanListPublicationConfig({
    id: 9,
    list_type: 'commercial',
    version: 'V3.0.6',
    changelog: '旧版本说明',
    config: {
      listType: 'commercial',
      version: 'V3.0.6',
      brandName: '测试品牌',
      layoutStyle: 'table',
      cardsPerRow: 3,
      backgroundColor: '#112233',
      fontColor: '#445566',
      showVersion: false,
      showChangelog: true,
      showCategoryNumbers: false,
      selectedProductIDs: [10, 999],
      visibleCategoryCodes: ['1', '9'],
      customizers: {
        10: { badge: 'new', highlightTerms: '55/包,庄园差异性产品' },
        999: { badge: 'medal' },
      },
    },
  }, {
    listType: 'retail',
    brandName: '当前品牌',
    version: DEFAULT_BEAN_LIST_PDF_VERSION,
  }, {
    productIDs: ['10', '20'],
    categoryCodes: ['1', '2'],
  })

  assert.equal(copied.options.listType, 'commercial')
  assert.equal(copied.options.version, 'V3.0.6')
  assert.equal(copied.options.brandName, '测试品牌')
  assert.equal(copied.options.layoutStyle, 'table')
  assert.equal(copied.options.cardsPerRow, 3)
  assert.equal(copied.options.backgroundColor, '#112233')
  assert.equal(copied.options.fontColor, '#445566')
  assert.equal(copied.options.showVersion, false)
  assert.equal(copied.options.showChangelog, true)
  assert.equal(copied.options.showCategoryNumbers, false)
  assert.equal(copied.options.changelog, '旧版本说明')
  assert.deepEqual(copied.selectedProductIDs, ['10'])
  assert.deepEqual(copied.visibleCategoryCodes, ['1'])
  assert.deepEqual(copied.customizers, {
    10: { badge: 'new', highlightTerms: '55/包,庄园差异性产品' },
  })
})

test('PDF bean-list helper copies published content groups as an immutable price snapshot', () => {
  const publication = {
    content: {
      groups: [{
        category: '6、差异性爆款：',
        items: [{
          productId: 30,
          code: '6.1',
          name: 'Nenka',
          prices: [{ label: '2包-13包', price: 127, unit: '包' }],
        }],
      }],
    },
  }

  const groups = copyBeanListPublicationContentGroups(publication)
  groups[0].items[0].prices[0].price = 1

  assert.equal(publication.content.groups[0].items[0].prices[0].price, 127)
  assert.equal(copyBeanListPublicationContentGroups({ content: {} }).length, 0)
})

test('PDF bean-list helper builds download options from published green bean snapshots', () => {
  const got = beanListPublicationPdfOptions({
    list_type: 'green',
    version: 'V1.02',
    changelog: '60KG+ 改为 62',
    config: {
      brandName: '岩师傅',
      layoutStyle: 'table',
      showCategoryNumbers: false,
      cardsPerRow: 3,
      backgroundColor: '#ffffff',
      fontColor: '#111111',
    },
  }, {
    brandName: '棵凡咖啡',
    layoutStyle: 'card',
    showCategoryNumbers: true,
  })

  assert.equal(got.listType, 'green')
  assert.equal(got.version, 'V1.02')
  assert.equal(got.brandName, '岩师傅')
  assert.equal(got.layoutStyle, 'table')
  assert.equal(got.showCategoryNumbers, false)
  assert.equal(got.changelog, '60KG+ 改为 62')
})

test('PDF bean-list helper reapplies green kg overrides when copying a kg price source snapshot', () => {
  const publication = {
    list_type: 'green',
    content: {
      groups: [{
        category: 'G、生豆销售',
        items: [{
          productId: 414,
          code: '1.414',
          name: '兰卡拼配生豆',
          prices: [{ label: '60kg+', price: 51.75, unit: 'kg', red: false }],
          green_bean_sale_tiers: [{
            label: '60kg+',
            template_tier_id: 51,
            spec_g: 1000,
            min_qty: 60,
            price_per_unit: 51.75,
            price_per_lb: 23.49,
            display_unit: 'kg',
          }],
        }],
      }],
    },
  }

  const groups = copyBeanListPublicationContentGroups(publication, {
    listType: 'green',
    customizers: {
      414: {
        greenPriceOverrides: {
          51: 62,
        },
      },
    },
  })

  assert.equal(groups[0].items[0].prices[0].price, 62)
  assert.equal(groups[0].items[0].prices[0].unit, 'kg')
  assert.equal(groups[0].items[0].green_bean_sale_tiers[0].price_per_lb, 28.15)
  assert.equal(groups[0].items[0].green_bean_sale_tiers[0].price_unit, 'kg')
  assert.equal(groups[0].items[0].green_bean_sale_tiers[0].price_per_kg, 62)
  assert.equal(publication.content.groups[0].items[0].prices[0].price, 51.75)
})

test('applyCustomerProductAliasesToBeanListItems scopes customer price lists by aliases and decorates display names', () => {
  const source = [
    {
      product_id: 10,
      name: '工厂拼配',
      product_code: 'K001',
      commercial_bean_list: { code: '1.1', category: '1、商用', display_name: '工厂拼配' },
    },
    {
      product_id: 11,
      name: '未授权拼配',
      product_code: 'K002',
      commercial_bean_list: { code: '1.2', category: '1、商用', display_name: '未授权拼配' },
    },
  ]
  const aliases = [
    { id: 101, customer_id: 42, product_id: 10, display_name: 'Karen 贴牌拼配', customer_item_code: 'KA-001', brand_name: '', display_category_name: 'Karen 批发', include_in_price_list: true, active: true },
    { id: 102, customer_id: 42, product_id: 11, display_name: '不进价格表', include_in_price_list: false, active: true },
    { id: 103, customer_id: 7, product_id: 11, display_name: '其他客户商品', include_in_price_list: true, active: true },
  ]

  const scoped = applyCustomerProductAliasesToBeanListItems(source, aliases, 42)

  assert.equal(scoped.length, 1)
  assert.equal(scoped[0].customer_product_alias_id, 101)
  assert.equal(scoped[0].customer_id, 42)
  assert.equal(scoped[0].name, 'Karen 贴牌拼配')
  assert.equal(scoped[0].product_name, '工厂拼配')
  assert.equal(scoped[0].customer_item_code, 'KA-001')
  assert.equal(scoped[0].commercial_bean_list.display_name, 'Karen 贴牌拼配')
})

test('buildBeanListPdfGroups freezes customer alias and product snapshots in publication content', () => {
  const groups = buildBeanListPdfGroups([{
    product_id: 10,
    product_code: 'K001',
    product_name: '工厂拼配',
    name: 'Karen 贴牌拼配',
    customer_id: 42,
    customer_product_alias_id: 101,
    customer_product_display_name: 'Karen 贴牌拼配',
    customer_item_code: 'KA-001',
    brand_name: '',
    display_category_name: 'Karen 批发',
    bom_version_id: 5,
    bom_version_no: 'v3',
    bom_usage_mode: 'inherit_current',
    yield_rate: 0.82,
    product_attributes: [{ key: 'pack', label: '包装', value: '客户专属袋' }],
    gradient_template: { id: 9, name: '批发模板' },
    commercial_wholesale_tiers: [{ label: '24-49kg', price_per_unit: 88, price_per_kg: 88, price_per_lb: 40, display_unit: 'kg' }],
    commercial_bean_list: { code: '1.1', category: '1、商用', display_name: 'Karen 贴牌拼配' },
  }], 'commercial')

  const item = groups[0].items[0]
  assert.equal(item.customer_product_alias_id, 101)
  assert.equal(item.customer_id, 42)
  assert.equal(item.product_id, 10)
  assert.equal(item.display_name_snapshot, 'Karen 贴牌拼配')
  assert.equal(item.customer_item_code_snapshot, 'KA-001')
  assert.equal(item.brand_name_snapshot, '')
  assert.equal(item.display_category_snapshot, 'Karen 批发')
  assert.equal(item.product_code_snapshot, 'K001')
  assert.equal(item.product_name_snapshot, '工厂拼配')
  assert.equal(item.bom_version_id_snapshot, 5)
  assert.equal(item.bom_usage_mode_snapshot, 'inherit_current')
  assert.equal(item.price_unit_snapshot, 'kg')
  assert.deepEqual(item.special_attrs_snapshot, [{ key: 'pack', label: '包装', value: '客户专属袋' }])
  assert.equal(item.price_source_json.gradient_template_id, 9)
})
