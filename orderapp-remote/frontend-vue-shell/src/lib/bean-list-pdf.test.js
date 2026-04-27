import test from 'node:test'
import assert from 'node:assert/strict'

import {
  DEFAULT_BEAN_LIST_PDF_VERSION,
  buildBeanListPdfGroups,
  buildBeanListPdfTitle,
  sanitizeBeanListPdfTheme,
} from './bean-list-pdf.js'

const rows = [
  {
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

  assert.deepEqual(commercial.map((group) => group.category), ['1、工厂量单', '5、原产地精选豆：'])
  assert.equal(commercial[0].items[0].code, '1.1')
  assert.equal(commercial[0].items[0].prices[0].unit, 'kg')
  assert.equal(retail.length, 1)
  assert.equal(retail[0].items[0].code, '3.2')
  assert.equal(retail[0].items[0].recommendedUse, '手冲/SOE/冷萃')
  assert.equal(buildBeanListPdfTitle('commercial'), '棵凡咖啡批发豆单')
  assert.equal(buildBeanListPdfTitle('retail'), '棵凡咖啡零售豆单')
})
