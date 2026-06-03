import test from 'node:test'
import assert from 'node:assert/strict'
import {
  bomContextCustomerIDs,
  productionBomLabel,
  productionBomVersionWarning,
  filterBomRowsByProductFocus,
  filterBomContextProducts,
  isBomProductCandidate,
  sortBomContextProducts,
  filterProductionBomCatalog,
} from './bom.js'

test('BOM context shows public and current-customer SKUs while hiding other customers and green beans', () => {
  const rows = [
    { id: 1, name: '岩师傅熟豆', customer_id: 152, product_kind: 'roasted_bean' },
    { id: 2, name: '兰卡拼配生豆', customer_id: 152, product_kind: 'green_bean', green_bean_bom_product_id: 1 },
    { id: 3, name: '岩师傅挂耳', customer_id: 152, product_kind: 'drip_bag' },
    { id: 4, name: '公共熟豆', customer_id: 0, product_kind: 'roasted_bean' },
    { id: 5, name: '其他客户熟豆', customer_id: 153, product_kind: 'roasted_bean' },
  ]

  assert.equal(isBomProductCandidate(rows[1]), false)
  assert.deepEqual(filterBomContextProducts(rows, 152).map((row) => row.id), [3, 1, 4])
  assert.deepEqual(filterBomContextProducts(rows, 0).map((row) => row.id), [4])
})

test('BOM context sorts customer SKUs first and frequent order products before lower usage rows', () => {
  const rows = [
    { id: 1, name: '公共低频', customer_id: 0, product_kind: 'roasted_bean', order_usage_count: 1 },
    { id: 2, name: '客户低频', customer_id: 152, product_kind: 'roasted_bean', order_usage_count: 1 },
    { id: 3, name: '公共高频', customer_id: 0, product_kind: 'roasted_bean', order_usage_count: 9 },
    { id: 4, name: '客户高频', customer_id: 152, product_kind: 'drip_bag', order_usage_count: 6 },
  ]

  assert.deepEqual(sortBomContextProducts(rows, 152).map((row) => row.id), [4, 2, 3, 1])
  assert.deepEqual(filterBomContextProducts(rows, 152).map((row) => row.id), [4, 2, 3, 1])
})

test('BOM rows can be focused to the SKU product from settings navigation', () => {
  const rows = [
    { product_id: 10, product: '目标 SKU' },
    { product_id: 11, product: '同客户其他 SKU' },
    { product_id: 12, product: '公共 SKU' },
  ]

  assert.deepEqual(filterBomRowsByProductFocus(rows, 10).map((row) => row.product_id), [10])
  assert.deepEqual(filterBomRowsByProductFocus(rows, 0).map((row) => row.product_id), [10, 11, 12])
})

test('BOM customer selector ignores customers that only have green bean SKUs', () => {
  const products = [
    { id: 1, customer_id: 9, product_kind: 'green_bean' },
    { id: 2, customer_id: 10, product_kind: 'roasted_bean' },
  ]
  const bomRows = [
    { product_id: 3, customer_id: 11, product_kind: 'green_bean' },
    { product_id: 4, customer_id: 12, product_kind: 'drip_bag' },
  ]

  assert.deepEqual([...bomContextCustomerIDs(products, bomRows)].sort((a, b) => a - b), [10, 12])
})

test('production BOM label shows BOM code name and bound version without source terminology', () => {
  assert.equal(productionBomLabel({
    production_bom_code: 'BOM-001',
    production_bom_name: '精品拼配',
    production_bom_version_no: 'V003',
  }), 'BOM-001 精品拼配 / V003')

  assert.equal(productionBomLabel({ bom_status: 'missing' }), '无生产 BOM')
  assert.equal(productionBomVersionWarning({
    production_bom_version_no: 'V002',
    latest_bom_version_no: 'V003',
    is_latest_bom_version: false,
  }), '当前引用 V002，最新 V003')
  assert.equal(productionBomVersionWarning({
    production_bom_version_no: 'V003',
    latest_bom_version_no: 'V003',
    is_latest_bom_version: true,
  }), '')
})

test('BOM view exposes grouped recipe library and no longer edits production config fields', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const appSource = fs.readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  assert.match(source, /生产 BOM（配方库）/)
  assert.match(source, /bom-return-banner/)
  assert.match(source, /return_navigation/)
  assert.doesNotMatch(source, /searchParams\.get\('return_product_id'\)/)
  assert.match(appSource, /transientReturnNavigation/)
  assert.match(appSource, /returnNavigation/)
  assert.match(source, /增加分组/)
  assert.match(source, /全部分组/)
  assert.match(source, /未分类/)
  assert.match(source, /移动到分组/)
  assert.ok(source.indexOf('bom-move-card') < source.indexOf('bom-list-tabs'))
  assert.match(source, /bom-workspace-actions/)
  assert.match(source, /bom-workspace-header/)
  assert.match(source, /bom-list-panel-scroll/)
  assert.match(source, /isMovableBomRow/)
  assert.match(source, /配方明细/)
  assert.match(source, /当前引用/)
  assert.match(source, /DELETE/)
  assert.doesNotMatch(source, /生产 BOM 档案/)
  assert.doesNotMatch(source, /默认分组/)
  assert.doesNotMatch(source, /group-tree/)
  assert.doesNotMatch(source, /特殊属性/)
  assert.doesNotMatch(source, /special_attrs_schema_json/)
  assert.doesNotMatch(source, /special_attrs_json/)
  assert.doesNotMatch(source, /预期产出率/)
  assert.doesNotMatch(source, /保存预期损耗率/)
  assert.doesNotMatch(source, /include_inactive=1/)
  assert.doesNotMatch(source, /production-bom-groups\/\$\{group\.id\}\/disable/)
  assert.doesNotMatch(source, /跟随默认 BOM/)
  assert.doesNotMatch(source, /固定 BOM 版本/)
  assert.doesNotMatch(source, /复制为单独维护 BOM/)
  assert.doesNotMatch(source, /派生自有 BOM/)
  assert.doesNotMatch(source, /lockBomVersion/)
})

test('production BOM list supports status filters name search group tabs and inactive copy actions', async () => {
  const rows = [
    { id: 1, code: 'BOM-001', name: '精品拼配', status: 'active', group_id: 2 },
    { id: 2, code: 'BOM-002', name: '旧版深烘', status: 'inactive', group_id: 2 },
    { id: 3, code: 'BOM-003', name: '挂耳配方', status: 'active', group_id: 3 },
    { id: 4, code: 'BOM-004', name: '未分类配方', status: 'active', group_id: 0 },
  ]

  assert.deepEqual(filterProductionBomCatalog(rows, { status: 'active', query: 'BOM-00' }).map((row) => row.id), [1, 3, 4])
  assert.deepEqual(filterProductionBomCatalog(rows, { status: 'inactive', query: '深烘' }).map((row) => row.id), [2])
  assert.deepEqual(filterProductionBomCatalog(rows, { status: 'all', query: '拼配' }).map((row) => row.id), [1])
  assert.deepEqual(filterProductionBomCatalog(rows, { status: 'active', groupID: -1 }).map((row) => row.id), [4])

  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  for (const marker of [
    'bom-workspace-actions',
    'bom-list-panel-scroll',
    'productionBomStatusFilter',
    'productionBomSearchQuery',
    '新建生产 BOM',
    'openBomVersionDrawer',
    'BOM版本',
    'openBagSpecMappingDrawer',
    '全局规格袋材映射',
    '规格袋材映射',
    '复制',
    '失效',
    'selectedBomRowKeys',
    'moveSelectedProductBomsToGroup',
    'isMovableBomRow',
    'copyProductionBomRecord',
    'deactivateProductionBomRecord',
    '/api/production-boms/${bomForm.id}',
    '/api/production-boms/${bomForm.source_id}/copy',
  ]) {
    assert.match(source, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  const headStart = source.indexOf('class="panel-head bom-list-head"')
  const headEnd = source.indexOf('bom-list-panel-scroll')
  assert.notEqual(headStart, -1)
  assert.notEqual(headEnd, -1)
  const listHead = source.slice(headStart, headEnd)
  assert.match(listHead, /productionBomStatusFilter/)
  assert.match(listHead, /productionBomSearchQuery/)
  assert.doesNotMatch(listHead, /移动到分组/)
  assert.match(source, /versionDrawerOpen/)
  assert.match(source, /bagSpecMappingDrawerOpen/)
  assert.doesNotMatch(source, /<section class="panel">\s*<div class="panel-title">BOM版本<\/div>/)
  assert.doesNotMatch(source, /<section v-if="!isWorkspaceCustomerLocked" class="panel">\s*<div class="panel-title">规格袋材映射<\/div>/)
})
