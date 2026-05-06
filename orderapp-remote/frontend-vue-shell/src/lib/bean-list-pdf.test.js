import test from 'node:test'
import assert from 'node:assert/strict'

import {
  DEFAULT_BEAN_LIST_PDF_VERSION,
  buildBeanListPdfGroups,
  buildBeanListPdfTitle,
  copyBeanListPublicationContentGroups,
  copyBeanListPublicationConfig,
  filterBeanListItemsForScope,
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

test('PDF bean-list helper builds separate commercial and retail groups from Excel metadata', () => {
  const commercial = buildBeanListPdfGroups(rows, 'commercial')
  const retail = buildBeanListPdfGroups(rows, 'retail')

  assert.deepEqual(commercial.map((group) => group.category), ['1、工厂量单', '5、原产地精选豆：', '6、差异性爆款：'])
  assert.equal(commercial[0].items[0].code, '1.1')
  assert.equal(commercial[0].items[0].prices[0].unit, 'kg')
  assert.equal(retail.length, 2)
  assert.equal(retail[0].items[0].code, '3.2')
  assert.equal(retail[0].items[0].recommendedUse, '手冲/SOE/冷萃')
  assert.equal(buildBeanListPdfTitle('commercial'), '棵凡咖啡批发豆单')
  assert.equal(buildBeanListPdfTitle('retail'), '棵凡咖啡零售豆单')
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
  assert.equal(buildBeanListPdfTitle('commercial', theme.brandName), '烘豆实验室批发豆单')
  assert.equal(buildBeanListPdfTitle('retail', theme.brandName), '烘豆实验室零售豆单')

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
