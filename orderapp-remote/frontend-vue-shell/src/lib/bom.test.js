import test from 'node:test'
import assert from 'node:assert/strict'
import {
  bomContextCustomerIDs,
  normalizeProductionBomName,
  productionBomListName,
  productionBomLabel,
  productionBomVersionWarning,
  filterBomRowsByProductFocus,
  filterBomContextProducts,
  isBomProductCandidate,
  isProductionBomOutputProductCandidate,
  productionBomDetailAsRecipeDetail,
  sortBomContextProducts,
  filterProductionBomCatalog,
  bomProductOptionLabel,
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
  assert.equal(isProductionBomOutputProductCandidate(rows[1]), true)
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

test('BOM product selector labels include stable SKU codes before duplicate names', () => {
  assert.equal(bomProductOptionLabel({ id: 518, name: '初晓' }), 'SKU-000518 初晓')
  assert.equal(bomProductOptionLabel({ product_id: 884, product_name: '初晓拼配' }), 'SKU-000884 初晓拼配')
  assert.equal(bomProductOptionLabel({ id: 518, product_code: 'SKU-000518', name: '初晓' }), 'SKU-000518 初晓')
  assert.equal(bomProductOptionLabel({ id: 518, product_code: 'SKU-000518', name: 'SKU-000518 初晓' }), 'SKU-000518 初晓')
})

test('BOM output selector includes active green beans while finished-product components keep the old scope', async () => {
  assert.equal(isBomProductCandidate({ id: 8, product_kind: 'roasted_bean', active: false }), false)
  assert.equal(isBomProductCandidate({ id: 9, product_kind: 'roasted_bean', status: 'inactive' }), false)
  assert.equal(isBomProductCandidate({ id: 10, product_kind: 'drip_bag', active: true }), true)
  assert.equal(isProductionBomOutputProductCandidate({ id: 11, product_kind: 'green_bean', active: true }), true)
  assert.equal(isProductionBomOutputProductCandidate({ id: 12, product_kind: 'green_bean', status: 'inactive' }), false)

  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const componentSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const componentTemplate = componentSource.split('<script setup>')[0] || componentSource
  const toolbar = source.match(/<BusinessGroupControls[\s\S]*?\/>/)?.[0] || ''

  assert.match(toolbar, /@move="moveSelectedProductBomsToGroup"/)
  assert.match(componentSource, /移动到分类/)
  assert.match(componentSource, /目标分类/)
  assert.ok(componentTemplate.indexOf('{{ moveLabel }}') < componentTemplate.indexOf('目标分类'), '移动到分类 button should be left of 目标分类')
	assert.match(source, /outputProductOptions = computed\(\(\) => products\.value\.filter\(isProductionBomOutputProductCandidate\)/)
	assert.match(source, /productComponentOptions = computed\(\(\) => products\.value\.filter\(isBomProductCandidate\)/)
  assert.match(componentSource, /前往分组模板/)
  assert.match(source, /\/api\/business-group-assignments/)
  assert.match(source, /businessGroupMoveAssignmentPayload/)
  assert.match(source, /businessGroupControlOptions/)
  assert.doesNotMatch(toolbar, /组内分类|新增小分类|移动到小分类|目标小分类/)
  assert.doesNotMatch(source, /groupDrawerOpen/)
  assert.doesNotMatch(source, /groupCategoryDrawerOpen/)
  assert.match(source, /isSystemDefaultBusinessGroup/)
  assert.doesNotMatch(source, /name:\s*group\.name \|\| '生产 BOM 分组'/)
})

test('production BOM detail is projected as recipe detail with output product label', () => {
  const detail = productionBomDetailAsRecipeDetail({
    id: 186,
    code: 'BOM-000186',
    name: 'Nenka嫩咖 生产 BOM',
    output_product_id: 88,
    output_product_name: '10条盒装挂耳',
    output_product_code: 'SKU-000088',
    business_group_id: 8,
    business_group_name: '客户配方',
    group_item_id: 31,
    group_item_name: '浅烘',
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
  assert.equal(detail.production_bom_group_id, 8)
  assert.equal(detail.production_bom_group_name, '客户配方')
  assert.equal(detail.production_bom_group_category_id, 31)
  assert.equal(detail.production_bom_group_category_name, '浅烘')
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

test('production BOM label shows the normalized business name without code or version', () => {
  assert.equal(productionBomLabel({
    code: 'BOM-009',
    name: '独立配方',
    latest_version_no: 'V004',
  }), '独立配方')

  assert.equal(productionBomLabel({
    production_bom_code: 'BOM-001',
    production_bom_name: 'BOM000643 精品拼配 生产 BOM / V003',
    production_bom_version_no: 'V003',
  }), '精品拼配')

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

test('production BOM list name hides the code, generated suffix, and version', () => {
  assert.equal(productionBomListName({
    code: 'BOM-000659',
    name: 'ALO TOH#1 生产 BOM',
    latest_version_no: 'V001',
  }), 'ALO TOH#1')

  assert.equal(productionBomListName({
    name: 'BOM-000659 ALO TOH#1 生产 BOM / V001',
  }), 'ALO TOH#1')

  assert.equal(productionBomListName({
    name: 'BOM000643 曲奇拼配 生产 BOM / V001',
  }), '曲奇拼配')

  assert.equal(productionBomListName({
    name: 'BOM000643曲奇拼配 生产 BOM',
  }), '曲奇拼配')

  assert.equal(productionBomListName({
    name: 'BOM-003262 PR442-SCENARIO Production BOM',
  }), 'PR442-SCENARIO')

  assert.equal(productionBomListName({
    name: 'GoalE2E 咖啡熟豆 BOM',
  }), 'GoalE2E 咖啡熟豆')

  assert.equal(productionBomListName({
    name: '生产 BOM 校准配方',
  }), '生产 BOM 校准配方')

  assert.equal(productionBomListName({
    name: 'ALO TOH#1 生产 BOM 副本 副本',
  }), 'ALO TOH#1 副本 副本')

  assert.equal(productionBomListName({
    name: 'ALO TOH#1 生产 BOM 特殊属性副本',
  }), 'ALO TOH#1 特殊属性副本')

  for (const value of [
    'BOM-000659 ALO TOH#1 生产 BOM / V001',
    'BOM000643 曲奇拼配 生产 BOM',
    'PR442-SCENARIO Production BOM',
    'GoalE2E 咖啡熟豆 BOM',
    'ALO TOH#1 副本 副本',
    '生产 BOM 校准配方',
  ]) {
    const normalized = normalizeProductionBomName(value)
    assert.equal(normalizeProductionBomName(normalized), normalized)
  }
})

test('BOM view can set the output product default BOM with the current published version', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')

  assert.match(source, /设为产出商品默认 BOM/)
  assert.match(source, /currentProductionBomDefaultVersion/)
  assert.match(source, /setCurrentProductionBomAsDefault/)
  assert.match(source, /\/api\/products\/\$\{productID\}\/default-production-bom/)
  assert.match(source, /default_production_bom_id:\s*bomID/)
  assert.doesNotMatch(source, /production_bom_version_id:\s*versionID/)
  assert.doesNotMatch(source, /production_bom_version_id:\s*selectedProductionBomVersionID/)
})

test('BOM version settings expose material loss switch and ratio-only explanation', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const itemFormBlock = template.match(/<form class="inline-form" @submit\.prevent="saveItem"[\s\S]*?<\/form>/)?.[0] || ''

  assert.match(source, /原料损耗比/)
  assert.match(source, /损耗比例 %/)
  assert.match(source, /material_loss_rate/)
  assert.match(source, /versionMaterialLossRateEnabled/)
  assert.match(source, /开启后组件消耗单位只能使用比例 %/)
  assert.match(source, /selectedVersionMaterialLossRate/)
  assert.match(source, /materialLossRatioOnlyConsumeUnitOptions/)
  assert.match(source, /不含原料损耗/)
  assert.match(source, /materialLossRateDisplay/)
  assert.match(source, /配方比例 × \(1 \+ 原料损耗比\)/)
  assert.doesNotMatch(source, /1 \/ \(1 - 原料损耗比\)/)
  assert.match(source, /material_loss_rate:\s*selectedVersionMaterialLossRate/)
  assert.doesNotMatch(itemFormBlock, /原料损耗比/)
  assert.doesNotMatch(itemFormBlock, /损耗比例 %/)
  assert.doesNotMatch(source, /itemForm\.material_loss_rate_enabled/)
})

test('BOM version editor exposes process route selector and route labels', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')

  assert.match(source, /工艺路线/)
  assert.match(source, /processRoutes/)
  assert.match(source, /selectedProductionBomVersion\.process_route_id/)
  assert.match(source, /process_route_id:\s*Number\(selectedProductionBomVersion\.value\?\.process_route_id/)
  assert.match(source, /process_route_name/)
  assert.match(source, /is_latest_usable/)
  assert.match(source, /\/api\/process-routes\?status=active/)
  assert.doesNotMatch(source, /\/api\/process-templates/)
  assert.doesNotMatch(source, /linkedProcessTemplates/)
})

test('BOM view exposes grouped manufacturing BOM library and no longer edits product-bound production config fields', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const componentSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
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
  assert.match(source, /BusinessGroupControls/)
  assert.match(source, /productionBomDisplayGroups/)
  assert.match(componentSource, /前往分组模板/)
  assert.match(componentSource, /移动到分类/)
  assert.match(source, /key:\s*'groupTemplates'/)
  assert.match(source, /returnNavigation/)
  assert.match(source, /bom-list-toolbar/)
  assert.doesNotMatch(source, /bom-list-tabs-row/)
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
  assert.match(source, /删除/)
  assert.match(componentSource, /前往分组模板/)
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
  assert.doesNotMatch(source, /groupDrawerOpen/)
  assert.doesNotMatch(source, /groupCategoryDrawerOpen/)
  assert.doesNotMatch(source, /跟随默认 BOM/)
  assert.doesNotMatch(source, /固定 BOM 版本/)
  assert.doesNotMatch(source, /复制为单独维护 BOM/)
  assert.doesNotMatch(source, /派生自有 BOM/)
  assert.doesNotMatch(source, /lockBomVersion/)
  assert.doesNotMatch(source, /context-eyebrow">SKU归属/)
  assert.doesNotMatch(source, /bom-move-card/)
  assert.doesNotMatch(source, /bom-batch-deactivate-card/)
})

test('BOM detail keeps version recipe editing without global bag-spec mapping panel', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const componentSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const detailPanel = template.match(/<section class="panel detail-panel"[\s\S]*?<\/section>/)?.[0] || ''
  const versionPanel = template.match(/<div v-if="detail" class="detail-subpanel bom-version-panel"[\s\S]*?<\/div>\s*<\/div>\s*<\/section>/)?.[0] || ''

  assert.match(detailPanel, /BOM版本/)
  assert.match(detailPanel, /复制为新版草稿/)
  assert.match(detailPanel, /已发布版本只读，复制为新版草稿后编辑/)
  assert.match(versionPanel, /配方明细/)
  assert.match(versionPanel, /合计比例/)
  assert.match(versionPanel, /保存组件/)
  assert.match(source, /loadProductUnitDefinitions/)
  assert.match(source, /\/api\/product-settings\/units/)
  assert.match(source, /unitDictionaryConsumeUnitOptions/)
  assert.doesNotMatch(detailPanel, /规格袋材映射/)
  assert.doesNotMatch(detailPanel, /全局规格袋材映射/)
  assert.doesNotMatch(source, /bag-spec-mapping-panel/)
  assert.doesNotMatch(source, /全局规格袋材映射/)
  assert.doesNotMatch(source, /const materialConsumeUnitOptions = \[/)
  assert.doesNotMatch(source, /const finishedProductConsumeUnitOptions = \[/)
  assert.match(detailPanel, /openReferencedProductConfig\(product\)/)
  assert.match(detailPanel, /referenced-product-button/)
  assert.match(detailPanel, /product\.product_name/)
  assert.match(detailPanel, /product\.product_code/)
  assert.match(source, /referencedProductKey\(product\)/)
  assert.match(source, /isActiveReferencedProduct/)
  assert.doesNotMatch(detailPanel, /product\.bom_version_no/)
  assert.doesNotMatch(source, /versionDrawerOpen/)
  assert.doesNotMatch(source, /bagSpecMappingDrawerOpen/)
  assert.doesNotMatch(source, /class="drawer bom-version-drawer"/)
  assert.doesNotMatch(source, /class="drawer bag-spec-mapping-drawer"/)
  assert.doesNotMatch(template, /<button class="text-button"[^>]*openBomVersionDrawer\(row\)[\s\S]*BOM版本/)
  assert.doesNotMatch(template, /<button[^>]*openBagSpecMappingDrawer\(row\)[\s\S]*规格袋材映射/)
})

test('production BOM name opens the settings drawer and list no longer shows edit button', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const tableBlock = template.match(/<div class="table-wrap bom-list-panel-scroll">[\s\S]*?<\/table>\s*<\/div>/)?.[0] || ''

  assert.match(tableBlock, /@click\.stop="openBomRowPrimary\(row\)"/)
  assert.match(tableBlock, /\{\{ productionBomListName\(row\) \}\}/)
  assert.doesNotMatch(tableBlock, /\{\{ productionBomLabel\(row\) \}\}/)
  assert.doesNotMatch(tableBlock, />编辑<\/button>/)
  assert.match(source, /async function openBomRowPrimary\(row\)[\s\S]*openEditProductionBomRecord\(bomRecordFromRow\(row\)\)/)
})

test('production BOM list supports status filters name search group tabs and inactive copy actions', async () => {
  const rows = [
    { id: 1, code: 'BOM-001', name: '精品拼配', latest_version_no: 'V001', status: 'active', group_id: 2, group_item_id: 21 },
    { id: 2, code: 'BOM-002', name: '旧版深烘', status: 'inactive', group_id: 2, group_item_id: 21 },
    { id: 3, code: 'BOM-003', name: '挂耳配方', status: 'active', group_id: 3, group_item_id: 31 },
    { id: 4, code: 'BOM-004', name: '未分类配方', status: 'active', group_id: 0, group_item_id: 0 },
  ]

  assert.deepEqual(filterProductionBomCatalog(rows, { status: 'active', query: 'BOM-00' }).map((row) => row.id), [1, 3, 4])
  assert.deepEqual(filterProductionBomCatalog(rows, { status: 'active', query: 'V001' }).map((row) => row.id), [1])
  assert.deepEqual(filterProductionBomCatalog(rows, { status: 'inactive', query: '深烘' }).map((row) => row.id), [2])
  assert.deepEqual(filterProductionBomCatalog(rows, { status: 'all', query: '拼配' }).map((row) => row.id), [1])
  assert.deepEqual(filterProductionBomCatalog(rows, { status: 'active', groupItemID: -1 }).map((row) => row.id), [4])
  assert.deepEqual(filterProductionBomCatalog(rows, { status: 'active', groupItemID: 21 }).map((row) => row.id), [1])
  assert.deepEqual(filterProductionBomCatalog(rows, { status: 'active', groupID: 2, groupItemID: -1 }).map((row) => row.id), [3, 4])
  assert.deepEqual(filterProductionBomCatalog(rows, { status: 'active', groupID: 2, groupItemID: 21 }).map((row) => row.id), [1])

  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const toolbarStart = template.indexOf('<BusinessGroupControls')
  const filtersStart = template.indexOf('<div class="bom-list-filters"')
  const tableStart = template.indexOf('<div class="table-wrap bom-list-panel-scroll"')
  assert.notEqual(toolbarStart, -1)
  assert.notEqual(filtersStart, -1)
  assert.notEqual(tableStart, -1)
  const toolbar = template.slice(toolbarStart, filtersStart)
  const listFilters = template.slice(filtersStart, tableStart)
  const tableBlock = template.slice(tableStart, template.indexOf('</table>', tableStart))
  const bomRecordForm = template.match(/<form class="inline-form bom-record-form"[\s\S]*?<\/form>/)?.[0] || ''
  for (const marker of [
    'bom-list-toolbar',
    'bom-list-panel-scroll',
    'productionBomStatusFilter',
    'productionBomSearchQuery',
    '新建生产 BOM',
    'BOM版本',
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
  assert.match(bomRecordForm, /产出数量/)
  assert.match(bomRecordForm, /产出单位/)
  assert.match(bomRecordForm, /来源：销售规格模板库存单位/)
  assert.match(source, /inventory_unit_explicit/)
  assert.match(source, /请先到销售规格模板设置库存单位/)
  assert.match(source, /outputUnitMismatchWarning/)
  assert.match(source, /历史版本不会自动回改/)
  assert.doesNotMatch(bomRecordForm, /v-if="bomForm\.mode !== 'edit'"/)
  assert.ok(toolbarStart < filtersStart, 'group controls should render above list filters')
  assert.match(source, /新建生产 BOM/)
  assert.match(toolbar, /:move-options="productionBomMoveGroupOptions"/)
  assert.match(toolbar, /@manage="openBusinessGroupManagement"/)
  assert.match(toolbar, /@move="moveSelectedProductBomsToGroup"/)
  assert.match(source, /productionBomDisplayGroups/)
  assert.doesNotMatch(source, /productionBomUsedGroupOptions/)
  assert.doesNotMatch(source, /selectedProductionBomGroupItemID/)
  assert.doesNotMatch(tableBlock, /<th>分组<\/th>/)
  assert.doesNotMatch(tableBlock, /bomGroupLabel\(row\)/)
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
  assert.doesNotMatch(source, /saveMapping/)
  assert.doesNotMatch(source, /deleteMapping/)
  assert.doesNotMatch(source, /api\/bom\/bag-spec-mappings/)
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

test('production BOM uses generic business group assignment instead of its own group logic', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const componentSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const moveStart = source.indexOf('async function moveSelectedProductBomsToGroup')
  const moveEnd = source.indexOf('async function deactivateProductionBomRecords', moveStart)
  const saveStart = source.indexOf('async function saveProductionBomRecord')
  const saveEnd = source.indexOf('async function deactivateProductionBomRecord', saveStart)
  const moveSource = source.slice(moveStart, moveEnd)
  const saveSource = source.slice(saveStart, saveEnd)
  const toolbarStart = template.indexOf('<BusinessGroupControls')
  const filtersStart = template.indexOf('<div class="bom-list-filters"')
  assert.notEqual(toolbarStart, -1)
  assert.notEqual(filtersStart, -1)
  const toolbar = template.slice(toolbarStart, filtersStart)

  for (const marker of [
    '/api/business-group-assignments',
    'businessGroupMoveAssignmentPayload',
    'businessGroupControlOptions',
    'groupRowsByBusinessGroupTemplate',
    'productionBomMoveGroupOptions',
    'productionBomDisplayGroups',
    'selectedProductionBomTemplateID',
    "usage_key: 'production_bom'",
    "object_key: 'production_bom'",
    'openBusinessGroupManagement',
  ]) {
    assert.match(source, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.match(componentSource, /选择分组模板/)
  assert.match(componentSource, /移动到分类/)
  assert.match(toolbar, /:move-options="productionBomMoveGroupOptions"/)
  assert.doesNotMatch(source, /productionBomUsedGroupOptions/)
  assert.doesNotMatch(source, /productionBomUsedGroupItemIDs/)
  assert.doesNotMatch(source, /businessGroupItemMoveOptions/)
  assert.doesNotMatch(source, /selectedProductionBomUseGroupID/)
  assert.doesNotMatch(source, /useSelectedProductionBomGroup/)
  assert.doesNotMatch(source, /\/usages/)
  for (const marker of [
    '组内分类',
    '新增小分类',
    '移动到小分类',
    '目标小分类',
    'productionBomGroupCategories',
    'groupProductionBomRowsByInnerCategory',
    'moveSelectedProductBomsToGroupCategory',
    'openGroupCategoryDrawer',
    'deleteProductionBomGroupCategory',
    'selectedProductionBomGroupCategoryID',
    'groupCategoryDrawerOpen',
    'groupDrawerOpen',
    'saveProductionBomGroup',
    'saveProductionBomGroupCategory',
  ]) {
    assert.doesNotMatch(source, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.notEqual(moveStart, -1)
  assert.notEqual(moveEnd, -1)
  assert.notEqual(saveStart, -1)
  assert.notEqual(saveEnd, -1)
  assert.match(moveSource, /apiSend\('\/api\/business-group-assignments'/)
  assert.doesNotMatch(moveSource, /\/api\/production-boms\/\$\{bom\.id\}/)
  assert.doesNotMatch(saveSource, /group_id:/)
  assert.doesNotMatch(saveSource, /group_category_id:/)
  assert.doesNotMatch(source, /\/api\/production-bom-groups\/\$\{groupID\}\/categories/)
  assert.doesNotMatch(source, /\/api\/production-bom-group-categories\/\$\{categoryForm\.id\}/)
  assert.doesNotMatch(source, /全部分类/)
  assert.doesNotMatch(source, /bom-list-tabs-row/)
  assert.match(source, /version\.status === 'published'/)
  assert.match(source, /copyVersionAsDraft/)
  assert.match(source, /loadProductionBomDetailForVersion\(currentProductionBomID\.value,\s*versionID\)/)
})

test('production BOM move target stays selectable after selecting a BOM row', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const componentSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const toolbarStart = template.indexOf('<BusinessGroupControls')
  const filtersStart = template.indexOf('<div class="bom-list-filters"')
  const toolbar = template.slice(toolbarStart, filtersStart)

  assert.match(toolbar, /:can-move="canMoveSelectedBoms"/)
  assert.match(toolbar, /:can-select-target="canSelectBomMoveTarget"/)
  assert.match(source, /const canSelectBomMoveTarget = computed\(\(\) => selectedBomRecordsForMove\.value\.length > 0\)/)
  assert.match(componentSource, /canSelectTargetEffective/)
  assert.match(componentSource, /:disabled="!canSelectTargetEffective \|\| loading"/)
  assert.match(componentSource, /:disabled="!canMove \|\| loading"/)
})

test('production BOM list is grouped by template tree without category filter tabs', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const componentSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const listBlock = template.slice(
    template.indexOf('<section class="panel list-panel">'),
    template.indexOf('<aside class="panel detail-panel">'),
  )

  assert.match(source, /BusinessGroupControls/)
  assert.match(source, /groupRowsByBusinessGroupTemplate/)
  assert.match(componentSource, /选择分组模板/)
  assert.match(componentSource, /移动到分类/)
  assert.match(listBlock, /v-for="group in productionBomDisplayGroups"/)
  assert.match(source, /groupRowsByBusinessGroupTemplates\(/)
  assert.match(source, /skuGroupHiddenByCollapsedAncestor/)
  assert.match(listBlock, /group\.is_template_group/)
  assert.match(listBlock, /classification-template-row/)
  assert.match(listBlock, /group\.template_total/)
  assert.doesNotMatch(listBlock, /bom-list-tabs-row/)
  assert.doesNotMatch(listBlock, /全部分类/)
  assert.doesNotMatch(listBlock, /@click="selectProductionBomGroupItem/)
  assert.doesNotMatch(source, /productionBomUsedGroupOptions/)
  assert.doesNotMatch(source, /filterProductionBomCatalog\(productionBoms\.value,[\s\S]*groupItemID:/)
})

test('production BOM group template config opens from a drawer like warehouse inventory, not inline', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const listPanel = template.slice(
    template.indexOf('<section class="panel list-panel">'),
    template.indexOf('<section class="panel detail-panel">'),
  )

  // Template selection (checkboxes + save) lives in a drawer, not inline in the list panel.
  assert.match(source, /productionBomGroupFeatureDrawerOpen/)
  assert.match(source, /openProductionBomGroupFeatureSelectionDrawer/)
  assert.doesNotMatch(listPanel, /data-feature-key="production_bom"/)
  assert.match(source, /v-if="productionBomGroupFeatureDrawerOpen"[\s\S]*data-feature-key="production_bom"/)

  // A compact "设置分组模板" entry is exposed in the controls extra-actions and the empty state.
  assert.match(source, /<template #extra-actions>[\s\S]*设置分组模板/)
  assert.match(source, /尚未选择分组模板，当前平铺展示[\s\S]*设置分组模板/)

  // Saving the selection closes the drawer.
  assert.match(source, /productionBomGroupFeatureDrawerOpen\.value = false/)
})

test('production BOM grouping toolbar is a compact single row and the list window matches the detail panel height', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')

  // All grouping actions sit on one button-height row.
  assert.match(source, /\.bom-business-group-controls\s*\{[^}]*display:\s*flex/)
  assert.match(source, /\.bom-business-group-controls\s+label\s+span\s*\{[^}]*display:\s*inline/)
  // List panel stretches to be at least as tall as the detail panel so group headers do not dominate the window.
  assert.match(source, /\.grid\s*\{[^}]*align-items:\s*stretch/)
  assert.match(source, /\.list-panel\s*\{[^}]*display:\s*flex/)
  assert.match(source, /\.bom-list-panel-scroll\s*\{[^}]*flex:/)
  assert.doesNotMatch(source, /bom-list-panel-scroll\s*\{[^}]*max-height:\s*min\(62vh/)
})
