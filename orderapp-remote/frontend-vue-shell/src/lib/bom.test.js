import test from 'node:test'
import assert from 'node:assert/strict'
import {
  bomContextCustomerIDs,
  productionBomLabel,
  productionBomVersionWarning,
  filterBomRowsByProductFocus,
  filterBomContextProducts,
  isBomProductCandidate,
  productionBomDetailAsRecipeDetail,
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

test('production BOM detail is projected as recipe detail with output product label', () => {
  const detail = productionBomDetailAsRecipeDetail({
    id: 186,
    code: 'BOM-000186',
    name: 'Nenka嫩咖 生产 BOM',
    output_product_id: 88,
    output_product_name: '10条盒装挂耳',
    output_product_code: 'SKU-000088',
    group_id: 8,
    group_name: '客户配方',
    status: 'active',
    latest_version_id: 901,
    latest_version_no: 'V001',
    expected_loss_rate: 0.12,
    expected_yield_rate: 0.88,
    items: [
      { id: 1, component_type: 'material', consume_unit: 'ratio_pct', ratio_pct: 60, material_name: 'A 豆' },
      { id: 2, component_type: 'material', consume_unit: 'g_per_bag', qty_per_unit: 12, material_name: '袋材' },
      { id: 3, component_type: 'finished_product', consume_unit: 'ratio_pct', ratio_pct: 40, component_product_name: '成品' },
    ],
  })

  assert.equal(detail.product_id, 88)
  assert.equal(detail.product_name, '10条盒装挂耳')
  assert.equal(detail.output_product_code, 'SKU-000088')
  assert.equal(detail.production_bom_id, 186)
  assert.equal(detail.production_bom_code, 'BOM-000186')
  assert.equal(detail.production_bom_name, 'Nenka嫩咖 生产 BOM')
  assert.equal(detail.production_bom_version_id, 901)
  assert.equal(detail.production_bom_version_no, 'V001')
  assert.equal(detail.production_bom_group_name, '客户配方')
  assert.equal(detail.expected_loss_rate, 0.12)
  assert.equal(detail.expected_yield_rate, 0.88)
  assert.equal(detail.total_ratio, 60)
  assert.equal(detail.items.length, 3)
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
    code: 'BOM-009',
    name: '独立配方',
    latest_version_no: 'V004',
  }), 'BOM-009 独立配方 / V004')

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

test('BOM view exposes grouped manufacturing BOM library and no longer edits product-bound production config fields', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const appSource = fs.readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  assert.match(source, /生产 BOM（制造主档）/)
  assert.match(source, /产出商品/)
  assert.match(source, /产出数量/)
  assert.match(source, /组件来源/)
  assert.match(source, /商品组件/)
  assert.match(source, /多层展开/)
  assert.match(source, /usedByBoms/)
  assert.match(source, /outputProductOptions/)
  assert.match(source, /output_product_id/)
  assert.match(source, /bom-return-banner/)
  assert.match(source, /return_navigation/)
  assert.doesNotMatch(source, /searchParams\.get\('return_product_id'\)/)
  assert.match(appSource, /transientReturnNavigation/)
  assert.match(appSource, /returnNavigation/)
  assert.match(source, /增加分组/)
  assert.match(source, /全部分组/)
  assert.match(source, /未分类/)
  assert.match(source, /移动到分组/)
  assert.match(source, /bom-list-toolbar/)
  assert.match(source, /bom-list-tabs-row/)
  assert.match(source, /bom-list-filters/)
  assert.match(source, /bom-list-panel-scroll/)
  assert.match(source, /批量失效/)
  assert.match(source, /deactivateSelectedProductionBoms/)
  assert.match(source, /selectedActiveBomRecordsForDeactivate/)
  assert.match(source, /isMovableBomRow/)
  assert.match(source, /配方明细/)
  assert.match(source, /openBomRowPrimary/)
  assert.match(source, /@click\.stop="openBomRowPrimary\(row\)"/)
  assert.match(source, /productionBomDetailAsRecipeDetail/)
  assert.match(source, /await selectUnboundProductionBom\(row\)/)
  assert.match(source, /openReferencedProductConfig/)
  assert.match(source, /产出商品/)
  assert.match(source, /返回BOM编辑/)
  assert.match(source, /returnNavigation/)
  assert.match(source, /targetKey:\s*'productMaster'/)
  assert.match(source, /open_product_config_id/)
  assert.match(source, /当前引用/)
  assert.match(source, /DELETE/)
  assert.doesNotMatch(source, /openEditProductionBomRecord\(bomRecordFromRow\(row\)\)\s*await selectUnboundProductionBom\(row\)/)
  assert.doesNotMatch(source, /失效当前 BOM/)
  assert.doesNotMatch(source, /async function deleteBom/)
  assert.doesNotMatch(source, /生产 BOM 档案/)
  assert.doesNotMatch(source, /商品档案在生产配置中引用某个 BOM 版本/)
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
  assert.doesNotMatch(source, /context-eyebrow">SKU归属/)
  assert.doesNotMatch(source, /bom-move-card/)
  assert.doesNotMatch(source, /bom-batch-deactivate-card/)
})

test('BOM detail keeps versions and bag-spec mapping inside the edit detail panel', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const detailPanel = template.match(/<section class="panel detail-panel"[\s\S]*?<\/section>/)?.[0] || ''
  const versionPanel = template.match(/<div v-if="detail" class="detail-subpanel bom-version-panel"[\s\S]*?<div v-if="detail && !isWorkspaceCustomerLocked" class="detail-subpanel bag-spec-mapping-panel">/)?.[0] || ''

  assert.match(detailPanel, /BOM版本/)
  assert.match(detailPanel, /复制为新版草稿/)
  assert.match(detailPanel, /已发布版本只读，复制为新版草稿后编辑/)
  assert.match(versionPanel, /配方明细/)
  assert.match(versionPanel, /合计比例/)
  assert.match(versionPanel, /保存组件/)
  assert.match(detailPanel, /规格袋材映射/)
  assert.match(detailPanel, /全局规格袋材映射/)
  assert.match(detailPanel, /openReferencedProductConfig\(product\)/)
  assert.match(detailPanel, /referenced-product-button/)
  assert.match(detailPanel, /product\.product_name/)
  assert.match(detailPanel, /product\.product_code/)
  assert.doesNotMatch(source, /versionDrawerOpen/)
  assert.doesNotMatch(source, /bagSpecMappingDrawerOpen/)
  assert.doesNotMatch(source, /class="drawer bom-version-drawer"/)
  assert.doesNotMatch(source, /class="drawer bag-spec-mapping-drawer"/)
  assert.doesNotMatch(template, /<button class="text-button"[^>]*openBomVersionDrawer\(row\)[\s\S]*BOM版本/)
  assert.doesNotMatch(template, /<button[^>]*openBagSpecMappingDrawer\(row\)[\s\S]*规格袋材映射/)
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
  const template = source.split('<script setup>')[0] || source
  const listFilters = template.match(/<div class="bom-list-filters"[\s\S]*?<\/div>\s*<div class="table-wrap bom-list-panel-scroll">/)?.[0] || ''
  const tabRow = template.match(/<div class="bom-list-tabs-row"[\s\S]*?<\/div>\s*<div class="bom-list-toolbar">/)?.[0] || ''
  const toolbar = template.match(/<div class="bom-list-toolbar"[\s\S]*?<div class="bom-list-filters">/)?.[0] || ''
  for (const marker of [
    'bom-list-toolbar',
    'bom-list-panel-scroll',
    'productionBomStatusFilter',
    'productionBomSearchQuery',
    '新建生产 BOM',
    'BOM版本',
    '全局规格袋材映射',
    '规格袋材映射',
    '复制',
    '失效',
    'selectedBomRowKeys',
    'moveSelectedProductBomsToGroup',
    'isMovableBomRow',
    'copyProductionBomRecord',
    'deactivateProductionBomRecord',
    'referencedProductsLabel',
    '产出商品',
    '/api/production-boms/${bomForm.id}',
    '/api/production-boms/${bomForm.source_id}/copy',
    '/api/production-boms?status=all',
  ]) {
    assert.match(source, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.match(listFilters, /状态/)
  assert.match(listFilters, /搜索 BOM/)
  assert.match(listFilters, /批量失效/)
  assert.match(tabRow, /bom-list-tabs/)
  assert.match(tabRow, /新建生产 BOM/)
  assert.match(toolbar, /移动到分组/)
  assert.match(toolbar, /增加分组/)
  assert.doesNotMatch(source, /商品 BOM列表/)
  assert.doesNotMatch(source, /商品过滤/)
  assert.doesNotMatch(source, /createProductionBomForProductRow/)
  assert.doesNotMatch(source, /isMissingProductBomRow/)
  assert.doesNotMatch(source, /mergeProductionBomRows/)
  assert.doesNotMatch(source, /v-for="row in bomContextRows"/)
  assert.doesNotMatch(source, /<th>商品<\/th>/)
  assert.doesNotMatch(source, /<td>\{\{ row\.product \}\}<\/td>/)
  assert.doesNotMatch(source, /无生产 BOM/)
  assert.doesNotMatch(source, /class="bom-workspace-header"/)
  assert.doesNotMatch(source, /class="bom-workspace-actions"/)
  assert.doesNotMatch(source, /bom-batch-deactivate-card/)
  assert.doesNotMatch(source, /bom-move-card/)
  assert.doesNotMatch(template, /<div class="filters">[\s\S]*选择商品/)
  const headStart = source.indexOf('class="panel-head bom-list-head"')
  const headEnd = source.indexOf('bom-list-panel-scroll')
  assert.notEqual(headStart, -1)
  assert.notEqual(headEnd, -1)
  const listHead = source.slice(headStart, headEnd)
  assert.match(listHead, /productionBomStatusFilter/)
  assert.match(listHead, /productionBomSearchQuery/)
  assert.match(listHead, /deactivateSelectedProductionBoms/)
  assert.match(source, /createVersion/)
  assert.match(source, /saveMapping/)
  const deactivateStart = source.indexOf('async function deactivateProductionBomRecord')
  const deactivateEnd = source.indexOf('async function createVersion', deactivateStart)
  assert.notEqual(deactivateStart, -1)
  assert.notEqual(deactivateEnd, -1)
  const deactivateSource = source.slice(deactivateStart, deactivateEnd)
  assert.doesNotMatch(deactivateSource, /window\.confirm/)
  assert.doesNotMatch(deactivateSource, /确认失效/)
  assert.doesNotMatch(source, /<section class="panel">\s*<div class="panel-title">BOM版本<\/div>/)
  assert.doesNotMatch(source, /<section v-if="!isWorkspaceCustomerLocked" class="panel">\s*<div class="panel-title">规格袋材映射<\/div>/)
})

test('production BOM custom groups expose inner category grouping and version draft editing affordances', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const tabRow = template.match(/<div class="bom-list-tabs-row"[\s\S]*?<\/div>\s*<div class="bom-list-toolbar">/)?.[0] || ''

  for (const marker of [
    '组内分类',
    '新增小分类',
    '移动到小分类',
    'productionBomGroupCategories',
    'groupProductionBomRowsByInnerCategory',
    'moveSelectedProductBomsToGroupCategory',
    'openGroupCategoryDrawer',
    'deleteProductionBomGroupCategory',
    'selectedProductionBomGroupCategoryID',
    'group_category_id',
    '/api/production-bom-groups/${groupID}/categories',
    '/api/production-bom-group-categories/${categoryForm.id}',
  ]) {
    assert.match(source, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.match(
    template,
    /<div class="group-category-move-controls"[\s\S]*?<button class="secondary compact-action" type="button" @click="openGroupCategoryDrawer">新增小分类<\/button>\s*<label>[\s\S]*?<span>目标小分类<\/span>[\s\S]*?<select v-model\.number="selectedProductionBomGroupCategoryID">/
  )
  assert.match(tabRow, /全部分组/)
  assert.match(tabRow, /未分类/)
  assert.match(source, /version\.status === 'published'/)
  assert.match(source, /copyVersionAsDraft/)
  assert.match(source, /loadProductionBomDetailForVersion\(currentProductionBomID\.value,\s*versionID\)/)
})
