import test from 'node:test'
import assert from 'node:assert/strict'
import * as bomLib from './bom.js'
import { businessGroupInlineListState, businessGroupVisibleRows } from './business-grouping.js'
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
  productionBomDraftItemKey,
  removeProductionBomDraftItem,
  sortBomContextProducts,
  filterProductionBomCatalog,
  bomProductOptionLabel,
  productionBomSpecTemplateReapplyStrategy,
} from './bom.js'

test('legacy single-output product BOM template reapply creates a replacement draft', () => {
  const versions = [
    { id: 341, status: 'draft' },
    { id: 338, status: 'published' },
    { id: 203, status: 'archived' },
  ]

  assert.deepEqual(productionBomSpecTemplateReapplyStrategy('single', versions), {
    mode: 'replacement',
    sourceVersionID: 338,
  })
  assert.deepEqual(productionBomSpecTemplateReapplyStrategy('spec_group', versions), {
    mode: 'reapply',
    sourceVersionID: 0,
  })
  assert.deepEqual(productionBomSpecTemplateReapplyStrategy('single', [{ id: 12, status: 'draft' }]), {
    mode: 'convert',
    sourceVersionID: 0,
  })
})

test('production BOM draft item deletion keeps persisted and unsaved rows independent', () => {
  const rows = [
    { id: 101, material_id: 1 },
    { id: 0, local_key: 'draft-item-new-1', material_id: 2 },
    { id: 0, local_key: 'draft-item-new-2', material_id: 3 },
  ]
  assert.equal(productionBomDraftItemKey(rows[0], 0), 'bom-item:101')
  assert.equal(productionBomDraftItemKey(rows[1], 1), 'draft-item-new-1')
  assert.deepEqual(removeProductionBomDraftItem(rows, 'bom-item:101').map((row) => row.material_id), [2, 3])
  assert.deepEqual(removeProductionBomDraftItem(rows, 'draft-item-new-1').map((row) => row.material_id), [1, 3])
  assert.deepEqual(removeProductionBomDraftItem(rows, 'draft-item-new-2').map((row) => row.material_id), [1, 2])
})

test('production BOM inline categories paginate every category independently and keep parent direct rows', () => {
  assert.equal(typeof bomLib.productionBomAccordionPageState, 'undefined')
  const groups = [
    { key: 'business-template-9', label: '咖啡豆', group_id: 9, group_item_id: 0, is_template_group: true, template_total: 14, rows: [] },
    { key: 'business-group-9-90', label: '熟豆', group_id: 9, group_item_id: 90, parent_group_item_id: 0, rows: [{ id: 31 }, { id: 32 }] },
    { key: 'business-group-9-91', label: '拼配', group_id: 9, group_item_id: 91, parent_group_item_id: 90, rows: Array.from({ length: 12 }, (_, index) => ({ id: index + 1 })) },
    { key: 'business-template-10', label: '挂耳', group_id: 10, group_item_id: 0, is_template_group: true, template_total: 2, rows: [] },
    { key: 'business-group-10-101', label: '盒装', group_id: 10, group_item_id: 101, parent_group_item_id: 0, rows: [{ id: 21 }, { id: 22 }] },
    { key: 'business-group-unclassified', label: '未分类', group_id: 0, group_item_id: 0, unclassified: true, rows: Array.from({ length: 25 }, (_, index) => ({ id: index + 101 })) },
  ]

  const state = businessGroupInlineListState(groups, {
    'business-group-9-91': { page: 2, pageSize: 10 },
    'business-group-10-101': { page: 1, pageSize: 10 },
    'business-group-unclassified': { page: 3, pageSize: 10 },
  })
  const byKey = new Map(state.groups.map((group) => [group.key, group]))
  assert.deepEqual(byKey.get('business-group-9-90').rows.map((row) => row.id), [31, 32], 'parent-assigned BOMs must not disappear')
  assert.deepEqual(byKey.get('business-group-9-91').rows.map((row) => row.id), [11, 12])
  assert.deepEqual(byKey.get('business-group-10-101').rows.map((row) => row.id), [21, 22])
  assert.deepEqual(byKey.get('business-group-unclassified').rows.map((row) => row.id), [121, 122, 123, 124, 125])
  assert.equal(byKey.get('business-group-9-91').total, 12)
  assert.equal(byKey.get('business-group-unclassified').page, 3)
  assert.deepEqual(state.pagination['business-group-9-90'], { page: 1, pageSize: 10 })
})

test('production BOM inline categories hide collapsed descendants from selection without changing their pages', () => {
  const grouped = [
    { key: 'business-template-9', label: '咖啡豆', group_id: 9, group_item_id: 0, is_template_group: true, template_total: 3, rows: [] },
    { key: 'business-group-9-90', label: '熟豆', group_id: 9, group_item_id: 90, parent_group_item_id: 0, rows: [{ id: 30 }] },
    { key: 'business-group-9-91', label: '拼配', group_id: 9, group_item_id: 91, parent_group_item_id: 90, rows: [{ id: 31 }, { id: 32 }] },
    { key: 'business-group-unclassified', label: '未分类', group_id: 0, group_item_id: 0, unclassified: true, rows: [{ id: 40 }] },
  ]
  const state = businessGroupInlineListState(grouped, { 'business-group-9-91': { page: 1, pageSize: 10 } })
  assert.deepEqual(businessGroupVisibleRows(state.groups, []).map((row) => row.id), [30, 31, 32, 40])
  assert.deepEqual(businessGroupVisibleRows(state.groups, ['business-group-9-90']).map((row) => row.id), [40])
  assert.deepEqual(state.pagination['business-group-9-91'], { page: 1, pageSize: 10 })
})

test('production BOM inline categories keep no-template flat lists independently paginated', () => {
  const rows = Array.from({ length: 25 }, (_, index) => ({ id: index + 1 }))
  const state = businessGroupInlineListState([
    { key: 'all-products', label: '全部 BOM', group_id: 0, group_item_id: 0, all: true, rows },
  ], { 'all-products': { page: 2, pageSize: 10 } })
  assert.equal(state.groups[0].total, 25)
  assert.equal(state.groups[0].page, 2)
  assert.deepEqual(state.groups[0].rows.map((row) => row.id), [11, 12, 13, 14, 15, 16, 17, 18, 19, 20])
})

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

test('BOM output selector includes active green beans while product components require BOM specification authority', async () => {
  assert.equal(isBomProductCandidate({ id: 8, product_kind: 'roasted_bean', active: false }), false)
  assert.equal(isBomProductCandidate({ id: 9, product_kind: 'roasted_bean', status: 'inactive' }), false)
  assert.equal(isBomProductCandidate({ id: 10, product_kind: 'drip_bag', active: true }), true)
  assert.equal(isProductionBomOutputProductCandidate({ id: 11, product_kind: 'green_bean', active: true }), true)
  assert.equal(isProductionBomOutputProductCandidate({ id: 12, product_kind: 'green_bean', status: 'inactive' }), false)

  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const workspaceSource = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')
  const controlsSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const workspace = source.match(/<BusinessGroupInlineWorkspace[\s\S]*?<\/BusinessGroupInlineWorkspace>/)?.[0] || ''

  assert.match(workspace, /@move="beginProductionBomCategoryMove"/)
  assert.match(workspace, /@target="handleProductionBomCategoryMoveTarget"/)
  assert.match(workspace, /:move-active="productionBomCategoryMoveActive"/)
  assert.match(controlsSource, /移动到分类/)
  assert.doesNotMatch(controlsSource, /目标分类|<select/)
  assert.match(workspaceSource, /请选择要移动到的分类/)
  assert.match(workspaceSource, /emit\('target'/)
	assert.match(source, /outputProductOptions = computed\(\(\) => products\.value\.filter\(isProductionBomOutputProductCandidate\)/)
	assert.match(source, /productComponentOptions = computed\(\(\) => products\.value\.filter\(\(product\) => isBomProductCandidate\(product\) && product\.bom_spec_authoritative === true/)
  assert.match(workspaceSource, /前往分组模板/)
  assert.match(workspaceSource, /设置分组模板/)
  assert.match(source, /\/api\/business-group-assignments/)
  assert.match(source, /businessGroupMoveAssignmentPayload/)
  assert.match(source, /businessGroupControlOptions/)
  assert.doesNotMatch(workspace, /组内分类|新增小分类|移动到小分类|目标小分类/)
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

test('BOM view can set the typed output default BOM while preserving the product endpoint', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')

  assert.match(source, /设为产出对象默认 BOM/)
  assert.match(source, /currentProductionBomDefaultVersion/)
  assert.match(source, /setCurrentProductionBomAsDefault/)
  assert.match(source, /\/api\/products\/\$\{outputID\}\/default-production-bom/)
  assert.match(source, /default_production_bom_id:\s*bomID/)
  assert.doesNotMatch(source, /production_bom_version_id:\s*versionID/)
  assert.doesNotMatch(source, /production_bom_version_id:\s*selectedProductionBomVersionID/)
})

test('BOM version settings use one recipe list and one consume mode per variant', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const itemFormBlock = template.match(/<form class="inline-form" @submit\.prevent="saveItem"[\s\S]*?<\/form>/)?.[0] || ''

  assert.match(source, /原料损耗比/)
  assert.match(source, /损耗比例 %/)
  assert.match(source, /versionMaterialLossRateEnabled/)
  assert.match(source, /handleVersionMaterialLossToggle/)
  assert.match(source, /v-if="isMaterialOutputBom"[\s\S]{0,400}?原料损耗比/)
  assert.doesNotMatch(source, /有损耗的配方/)
  assert.doesNotMatch(source, /无损耗的配方/)
  assert.doesNotMatch(source, /selectedMaterialLossZone/)
  assert.doesNotMatch(source, /selectMaterialLossZone/)
  assert.doesNotMatch(source, /detailItemSections/)
  assert.match(source, /recipeConsumeMode/)
  assert.match(source, /componentInventoryConsumeUnitOptions/)
  assert.match(source, /itemForm\.component_type === 'product'/)
  assert.match(source, /materialLossRateDisplay/)
  assert.match(source, /配方比例 ÷ \(1 - 原料损耗率\)/)
  assert.doesNotMatch(source, /配方比例 × \(1 \+ 原料损耗比\)/)
  assert.doesNotMatch(itemFormBlock, /损耗比例/)
  assert.doesNotMatch(source, /itemForm\.material_loss_rate_enabled/)
})

test('BOM drawer exposes copied spec groups and uses a responsive aligned header grid', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const searchable = fs.readFileSync(new URL('../components/SearchableSelect.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const form = template.match(/<form class="inline-form bom-record-form"[\s\S]*?<\/form>/)?.[0] || ''

  assert.match(form, /规格模板（可选）/)
  assert.match(form, /规格主体组件/)
  assert.doesNotMatch(form, /主投入物料/)
  assert.match(source, /规格组/)
  assert.match(source, /variants/)
  assert.doesNotMatch(source, /稳定规格键/)
  assert.match(source, /规格编码（自动）/)
  assert.match(source, /新条码（可选）/)
  assert.match(source, /规格一经发布，库存单位不可修改/)
  assert.match(source, /barcode:\s*String\(variant\.barcode/)
  assert.match(source, /grid-template-columns:\s*repeat\(4,\s*minmax\(0,\s*1fr\)\)/)
  assert.match(source, /\.bom-record-form\s*\{[^}]*align-items:\s*start/)
  assert.match(form, /class="bom-record-form-action"[\s\S]*class="bom-record-form-action-spacer"[\s\S]*保存 BOM/)
  assert.match(source, /\.bom-record-form-action-spacer\s*\{[^}]*visibility:\s*hidden/)
  assert.match(source, /@media \(max-width: 900px\)[\s\S]*repeat\(2,\s*minmax\(0,\s*1fr\)\)/)
  assert.match(source, /@media \(max-width: 600px\)[\s\S]*grid-template-columns:\s*1fr[\s\S]*\.bom-record-form-action-spacer\s*\{\s*display:\s*none/)
  assert.match(searchable, /box-sizing:\s*border-box/)
})

test('spec template and BOM spec group forms align selectors with hidden spacers', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')

  assert.match(source, /\.bom-spec-template-reapply-form\s*\{[^}]*align-items:\s*start/)
  assert.match(source, /\.bom-spec-identity-form\s*\{[^}]*align-items:\s*start/)
  assert.match(source, /\.reapply-action-spacer\s*\{[^}]*visibility:\s*hidden/)
  assert.match(source, /\.identity-action-spacer\s*\{[^}]*visibility:\s*hidden/)
})

test('product BOM draft can add, remove, and reapply a published specification template', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const specGroup = template.match(/<div[^>]*class="bom-spec-group-panel">[\s\S]*?<div class="version-recipe-panel">/)?.[0] || ''

  assert.match(specGroup, /添加规格/)
  assert.match(specGroup, /删除该规格/)
  assert.match(specGroup, /重新套用已发布模板/)
  assert.doesNotMatch(specGroup, /主投入物料/)
  assert.match(specGroup, /规格主体组件/)
  assert.match(source, /function addProductionBomDraftVariant/)
  assert.match(source, /function removeProductionBomDraftVariant/)
  assert.match(source, /async function reapplyProductionBomSpecTemplate/)
  assert.match(source, /\/api\/production-bom-versions\/\$\{draftVersionID\}\/spec-template/)
  assert.match(source, /spec_template_version_id:/)
  assert.match(source, /main_input_material_id:/)
  assert.match(source, /main_input_component:/)
  assert.match(source, /reapplyMainInputComponentType/)
  assert.match(source, /loadMainInputProductSpecs/)
})

test('product BOM creation always uses BOM specification groups', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const form = template.match(/<form class="inline-form bom-record-form"[\s\S]*?<\/form>/)?.[0] || ''

  assert.match(form, /产出结构/)
  assert.match(form, /value="BOM 规格组" disabled/)
  assert.doesNotMatch(form, /v-model="bomForm\.specification_mode"/)
  assert.doesNotMatch(form, /单一产出|多规格产出/)
  assert.match(form, /规格模板（可选）[\s\S]*bomForm\.spec_template_version_id/)
  assert.match(form, /Number\(bomForm\.spec_template_version_id \|\| 0\) > 0[\s\S]*bomForm\.main_input_component_type/)
  assert.match(form, /Number\(bomForm\.spec_template_version_id \|\| 0\) === 0[\s\S]*商品 BOM 规格/)
  assert.match(source, /specification_mode:\s*normalizeSpecificationMode\(bomForm\.specification_mode/)
  assert.match(source, /spec_template_version_id:\s*Number\(bomForm\.spec_template_version_id/)
  assert.match(source, /main_input_material_id:\s*Number\(bomForm\.main_input_material_id/)
  assert.match(source, /main_input_component:/)
  assert.match(source, /bomForm\.specification_mode === 'spec_group'/)
})

test('BOM version editor exposes process route selector and route labels', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')

  assert.match(source, /工艺路线/)
  assert.match(source, /processRoutes/)
  assert.match(source, /currentRecipeTarget\.process_route_id/)
  assert.match(source, /process_route_id:\s*Number\(selectedProductionBomVersion\.value\?\.process_route_id/)
  assert.match(source, /process_route_id:\s*Number\(variant\.process_route_id/)
  assert.match(source, /process_route_name/)
  assert.match(source, /is_latest_usable/)
  assert.match(source, /\/api\/process-routes\?status=active/)
  assert.doesNotMatch(source, /\/api\/process-templates/)
  assert.doesNotMatch(source, /linkedProcessTemplates/)
})

test('BOM view exposes grouped manufacturing BOM library and no longer edits product-bound production config fields', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const workspaceSource = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')
  const controlsSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
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
  assert.match(source, /BusinessGroupInlineWorkspace/)
  assert.doesNotMatch(source, /<BusinessGroupWorkspace/)
  assert.match(source, /productionBomDisplayGroups/)
  assert.match(workspaceSource, /前往分组模板/)
  assert.match(workspaceSource, /设置分组模板/)
  assert.match(controlsSource, /移动到分类/)
  assert.match(source, /key:\s*'groupTemplates'/)
  assert.match(source, /returnNavigation/)
  assert.match(source, /bom-business-group-inline-workspace/)
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
  assert.match(source, /await selectUnboundProductionBom\(record\)/)
  assert.match(source, /openReferencedProductConfig/)
  assert.match(source, /产出商品/)
  assert.match(source, /返回BOM编辑/)
  assert.match(source, /returnNavigation/)
  assert.match(source, /targetKey:\s*'productMaster'/)
  assert.match(source, /open_product_config_id/)
  assert.match(source, /当前引用/)
  assert.match(source, /删除/)
  assert.match(workspaceSource, /business-group-inline-footer/)
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
  const detailPanel = template.match(/<section class="bom-settings-detail">[\s\S]*?<\/section>\s*<\/Teleport>/)?.[0] || ''
  const versionPanel = detailPanel.match(/<div v-if="detail" class="detail-subpanel bom-version-panel"[\s\S]*/)?.[0] || ''

  assert.match(template, /data-bom-settings-drawer/)
  assert.doesNotMatch(template, /class="panel detail-panel"/)
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
  assert.match(template, /data-bom-settings-drawer/)
  assert.match(template, /<Teleport v-if="bomDrawerOpen && bomForm\.mode === 'edit'"[\s\S]*BOM 明细[\s\S]*BOM版本[\s\S]*配方明细/)
  assert.doesNotMatch(template, /class="panel detail-panel"|@click="selectBomRow\(row\)"/)
  assert.match(source, /if \(pendingProductionBomID\.value > 0\)[\s\S]*await openEditProductionBomRecord\(pendingRecord\)/)
  assert.match(source, /pendingProductionBomID\.value = Number\(copied\?\.id \|\| 0\)/)
  assert.match(source, /pendingProductionBomID\.value = Number\(created\?\.id \|\| 0\)/)
})

test('production BOM list preserves status and name search before inline category grouping and inactive copy actions', async () => {
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
  const workspaceStart = template.indexOf('<BusinessGroupInlineWorkspace')
  const filtersStart = template.indexOf('<div class="bom-list-filters"')
  const tableStart = template.indexOf('<div class="table-wrap bom-list-panel-scroll"')
  assert.notEqual(workspaceStart, -1)
  assert.notEqual(filtersStart, -1)
  assert.notEqual(tableStart, -1)
  const workspaceHead = template.slice(workspaceStart, filtersStart)
  const listFilters = template.slice(filtersStart, tableStart)
  const tableBlock = template.slice(tableStart, template.indexOf('</table>', tableStart))
  const bomRecordForm = template.match(/<form class="inline-form bom-record-form"[\s\S]*?<\/form>/)?.[0] || ''
  for (const marker of [
    'BusinessGroupInlineWorkspace',
    'bom-business-group-inline-workspace',
    'bom-list-panel-scroll',
    'filters.status',
    'productionBomSearchQuery',
    'filterProductionBomRows',
    'productionBomVisibleRows',
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
  assert.match(bomRecordForm, /规格模板（可选）/)
  assert.match(bomRecordForm, /规格主体组件/)
  assert.match(source, /inventory_unit_explicit/)
  assert.doesNotMatch(source, /请先到销售规格模板设置库存单位/)
  assert.match(source, /outputUnitMismatchWarning/)
  assert.match(source, /历史版本不会自动回改/)
  assert.match(bomRecordForm, /bomForm\.output_type === 'product' && bomForm\.mode !== 'edit'/)
  assert.ok(workspaceStart < filtersStart, 'classification workspace should render above and around list filters')
  assert.match(source, /新建生产 BOM/)
  assert.match(workspaceHead, /:groups="productionBomDisplayGroups"/)
  assert.match(workspaceHead, /:move-active="productionBomCategoryMoveActive"/)
  assert.match(workspaceHead, /@move="beginProductionBomCategoryMove"/)
  assert.match(workspaceHead, /@target="handleProductionBomCategoryMoveTarget"/)
  assert.match(workspaceHead, /@manage="openBusinessGroupManagement"/)
  assert.match(workspaceHead, /@configure="openProductionBomGroupFeatureSelectionDrawer"/)
  assert.match(source, /productionBomDisplayGroups/)
  assert.match(source, /function filterProductionBomRows\(rows = \[\]\)[\s\S]*filterProductionBomCatalog\(rows,\s*\{[\s\S]*status:\s*filters\.status,[\s\S]*query:\s*productionBomSearchQuery\.value/)
  assert.match(source, /businessGroupInlineListState\(\s*fullProductionBomDisplayGroups\.value,\s*productionBomPaginationByGroup\.value/)
  assert.match(source, /businessGroupVisibleRows\(\s*productionBomDisplayGroups\.value,\s*collapsedProductionBomGroups\.value/)
  assert.match(source, /visibleMovableBomRows = computed\(\(\) => productionBomVisibleRows\.value\.filter\(isMovableBomRow\)\)/)
  assert.match(template, /#group="\{ group \}"/)
  assert.match(tableBlock, /v-for="row in group\.rows"/)
  assert.doesNotMatch(source, /businessGroupGroupsForCategorySelection|selectedProductionBomCategoryKey|productionBomAccordionPageState/)
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
  assert.match(listHead, /filters\.status/)
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
  const workspaceSource = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')
  const controlsSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const handlerStart = source.indexOf('async function handleProductionBomCategoryMoveTarget')
  const moveStart = source.indexOf('async function moveSelectedProductBomsToGroup')
  const moveEnd = source.indexOf('async function deactivateProductionBomRecords', moveStart)
  const saveStart = source.indexOf('async function saveProductionBomRecord')
  const saveEnd = source.indexOf('async function deactivateProductionBomRecord', saveStart)
  const moveSource = source.slice(moveStart, moveEnd)
  const handlerSource = source.slice(handlerStart, moveStart)
  const saveSource = source.slice(saveStart, saveEnd)
  const workspaceStart = template.indexOf('<BusinessGroupInlineWorkspace')
  const filtersStart = template.indexOf('<div class="bom-list-filters"')
  assert.notEqual(workspaceStart, -1)
  assert.notEqual(filtersStart, -1)
  const workspace = template.slice(workspaceStart, filtersStart)

  for (const marker of [
    '/api/business-group-assignments',
    'businessGroupMoveAssignmentPayload',
    'businessGroupControlOptions',
    'groupRowsByBusinessGroupTemplate',
    'businessGroupInlineListState',
    'businessGroupVisibleRows',
    'productionBomDisplayGroups',
    'productionBomPaginationByGroup',
    'productionBomCategoryMoveActive',
    'handleProductionBomCategoryMoveTarget',
    "usage_key: 'production_bom'",
    "object_key: 'production_bom'",
    'openBusinessGroupManagement',
  ]) {
    assert.match(source, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  assert.match(workspaceSource, /请选择要移动到的分类/)
  assert.match(workspaceSource, /emit\('target'/)
  assert.match(controlsSource, /移动到分类/)
  assert.doesNotMatch(controlsSource, /目标分类|<select/)
  assert.match(workspace, /:groups="productionBomDisplayGroups"/)
  assert.match(workspace, /@target="handleProductionBomCategoryMoveTarget"/)
  assert.doesNotMatch(source, /productionBomUsedGroupOptions/)
  assert.doesNotMatch(source, /businessGroupGroupsForCategorySelection|selectedProductionBomCategoryKey|productionBomAccordionPageState/)
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
  assert.notEqual(handlerStart, -1)
  assert.notEqual(saveStart, -1)
  assert.notEqual(saveEnd, -1)
  assert.match(moveSource, /apiSend\('\/api\/business-group-assignments'/)
  assert.match(moveSource, /clearProductionBomBusinessGroupAssignment\(bom\.id\)/)
  assert.match(moveSource, /selectedBomRowKeys\.value = \[\]/)
  assert.match(moveSource, /completed = true/)
  assert.match(handlerSource, /const completed = await moveSelectedProductBomsToGroup\(target\)/)
  assert.match(handlerSource, /if \(completed\) productionBomCategoryMoveActive\.value = false/)
  assert.doesNotMatch(handlerSource, /finally[\s\S]*productionBomCategoryMoveActive\.value = false/)
  assert.doesNotMatch(moveSource, /window\.confirm/)
  assert.doesNotMatch(moveSource, /\/api\/production-boms\/\$\{bom\.id\}/)
  assert.doesNotMatch(saveSource, /group_id:/)
  assert.doesNotMatch(saveSource, /group_category_id:/)
  assert.doesNotMatch(source, /\/api\/production-bom-groups\/\$\{groupID\}\/categories/)
  assert.doesNotMatch(source, /\/api\/production-bom-group-categories\/\$\{categoryForm\.id\}/)
  assert.match(workspaceSource, /allLabel:\s*\{\s*type:\s*String,\s*default:\s*'全部分类'/)
  assert.doesNotMatch(source, /bom-list-tabs-row/)
  assert.match(source, /version\.status === 'published'/)
  assert.match(source, /copyVersionAsDraft/)
  assert.match(source, /loadProductionBomDetailForVersion\(currentProductionBomID\.value,\s*versionID\)/)
})

test('production BOM move mode starts from selected rows and exits only after a successful immediate target', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const workspaceSource = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')
  const controlsSource = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const workspaceStart = template.indexOf('<BusinessGroupInlineWorkspace')
  const filtersStart = template.indexOf('<div class="bom-list-filters"')
  const workspace = template.slice(workspaceStart, filtersStart)

  assert.match(workspace, /:can-move="canBeginProductionBomCategoryMove"/)
  assert.match(workspace, /:move-active="productionBomCategoryMoveActive"/)
  assert.match(workspace, /@move="beginProductionBomCategoryMove"/)
  assert.match(workspace, /@cancel="cancelProductionBomCategoryMove"/)
  assert.match(workspace, /@target="handleProductionBomCategoryMoveTarget"/)
  assert.match(source, /const canBeginProductionBomCategoryMove = computed\(\(\) => Boolean\([\s\S]*productionBomSelectedBusinessGroups\.value\.length && selectedBomRecordsForMove\.value\.length/)
  assert.match(source, /function beginProductionBomCategoryMove\(\)[\s\S]*productionBomCategoryMoveActive\.value = true/)
  assert.match(source, /async function handleProductionBomCategoryMoveTarget\(target\)[\s\S]*const completed = await moveSelectedProductBomsToGroup\(target\)[\s\S]*if \(completed\) productionBomCategoryMoveActive\.value = false/)
  assert.match(workspaceSource, /请选择要移动到的分类/)
  assert.match(workspaceSource, /emit\('target'/)
  assert.match(controlsSource, /:disabled="loading \|\| \(!moveActive && !canMove\)"/)
  assert.doesNotMatch(controlsSource, /目标分类|<select/)
})

test('production BOM list renders all filtered categories inline with group-keyed pagination', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const listBlock = template.match(/<section class="panel list-panel">[\s\S]*?<\/BusinessGroupInlineWorkspace>\s*<\/section>/)?.[0] || ''

  assert.match(source, /BusinessGroupInlineWorkspace/)
  assert.doesNotMatch(source, /<BusinessGroupWorkspace/)
  assert.match(source, /groupRowsByBusinessGroupTemplate/)
  assert.match(listBlock, /v-model:collapsed-keys="collapsedProductionBomGroups"/)
  assert.match(listBlock, /:groups="productionBomDisplayGroups"/)
  assert.match(listBlock, /#group="\{ group \}"/)
  assert.match(listBlock, /v-for="row in group\.rows"/)
  assert.match(source, /groupRowsByBusinessGroupTemplates\(/)
  assert.match(source, /businessGroupInlineListState\(\s*fullProductionBomDisplayGroups\.value,\s*productionBomPaginationByGroup\.value/)
  assert.match(source, /businessGroupVisibleRows\(\s*productionBomDisplayGroups\.value,\s*collapsedProductionBomGroups\.value/)
  assert.match(listBlock, /<thead>[\s\S]*BOM名称/)
  assert.match(listBlock, /<PaginationControls[\s\S]*@change="handleProductionBomGroupPaginationChange\(group\.key, \$event\)"/)
  assert.match(source, /function productionBomGroupShowsTable\(group = \{\}\)[\s\S]*parent_group_item_id/)
  assert.match(source, /function handleProductionBomGroupPaginationChange\(groupKey, \{ page, pageSize \} = \{\}\)/)
  assert.doesNotMatch(source, /selectedProductionBomCategoryKey|productionBomAccordionPageState|expandedProductionBomGroupKey|productionBomListPage/)
  assert.doesNotMatch(listBlock, /bom-list-tabs-row/)
  assert.doesNotMatch(source, /productionBomUsedGroupOptions/)
  assert.doesNotMatch(source, /filterProductionBomCatalog\(productionBoms\.value,[\s\S]*groupItemID:/)
})

test('production BOM group template config opens from a drawer like warehouse inventory, not inline', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const workspaceSource = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const listPanel = template.match(/<section class="panel list-panel">[\s\S]*?<\/BusinessGroupInlineWorkspace>\s*<\/section>/)?.[0] || ''

  // Template selection (checkboxes + save) lives in a drawer, not inline in the list panel.
  assert.match(source, /productionBomGroupFeatureDrawerOpen/)
  assert.match(source, /openProductionBomGroupFeatureSelectionDrawer/)
  assert.doesNotMatch(listPanel, /data-feature-key="production_bom"/)
  assert.match(source, /v-if="productionBomGroupFeatureDrawerOpen"[\s\S]*data-feature-key="production_bom"/)

  // Both template actions stay in the shared inline workspace footer.
  assert.match(source, /<BusinessGroupInlineWorkspace[\s\S]*@manage="openBusinessGroupManagement"[\s\S]*@configure="openProductionBomGroupFeatureSelectionDrawer"/)
  assert.match(workspaceSource, /business-group-inline-footer/)
  assert.match(workspaceSource, /前往分组模板/)
  assert.match(workspaceSource, /设置分组模板/)
  assert.doesNotMatch(listPanel, /尚未选择分组模板，当前平铺展示/)

  // Saving the selection closes the drawer.
  assert.match(source, /productionBomGroupFeatureDrawerOpen\.value = false/)
})

test('production BOM inline workspace restores browse state around immediate movement and keeps the list full width', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const workspaceSource = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')

  assert.match(workspaceSource, /moveSnapshot\.value = \{[\s\S]*collapsedKeys:[\s\S]*captureScrollState\(\)/)
  assert.match(workspaceSource, /emit\('update:collapsedKeys', businessGroupMoveCollapsedKeys\(props\.groups\)\)/)
  assert.match(workspaceSource, /restoreScrollState\(snapshot\)/)
  assert.match(workspaceSource, /business-group-inline-disabled[^}]*pointer-events:\s*none/)
  assert.match(source, /\.grid\s*\{[^}]*align-items:\s*stretch/)
  assert.match(source, /\.grid\s*\{[^}]*grid-template-columns:\s*minmax\(0, 1fr\)/)
  assert.match(source, /\.list-panel\s*\{[^}]*display:\s*flex/)
  assert.match(source, /\.bom-business-group-inline-workspace\s*\{[^}]*flex:/)
  assert.doesNotMatch(source, /class="panel detail-panel"/)
  assert.equal(source.slice(source.lastIndexOf('</style>') + '</style>'.length).trim(), '', 'BOM styles must stay inside the scoped style block')
})

test('production BOM repeats the table header and owns one pager per rendered category', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const listPanel = template.match(/<section class="panel list-panel">[\s\S]*?<\/BusinessGroupInlineWorkspace>\s*<\/section>/)?.[0] || ''

  assert.doesNotMatch(listPanel, /生产 BOM 是生产端主档案/)
  assert.equal((listPanel.match(/<PaginationControls/g) || []).length, 1)
  assert.match(listPanel, /#group="\{ group \}"[\s\S]*<table[\s\S]*<thead>[\s\S]*v-for="row in group\.rows"[\s\S]*<PaginationControls/)
  assert.match(listPanel, /:page="group\.page"/)
  assert.match(listPanel, /:page-size="group\.pageSize"/)
  assert.match(listPanel, /:total="group\.total"/)
  assert.match(source, /productionBomPaginationByGroup/)
  assert.match(source, /resetProductionBomGroupPages/)
  assert.doesNotMatch(source, /productionBomAccordionPageState|expandedProductionBomGroupKey|productionBomListState/)
})

test('production BOM selection and all-select only use expanded category current-page rows', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const listPanel = template.match(/<section class="panel list-panel">[\s\S]*?<\/BusinessGroupInlineWorkspace>\s*<\/section>/)?.[0] || ''

  assert.match(listPanel, /<table[^>]*data-auto-pagination="off"/)
  assert.match(listPanel, /:checked="isAllVisibleBomsSelected\(group\)"/)
  assert.match(listPanel, /@change="toggleAllVisibleBoms\(\$event, group\.rows\)"/)
  assert.match(source, /selectedBomRows = computed\(\(\) => \{[\s\S]*productionBomVisibleRows\.value\.filter/)
  assert.match(source, /watch\(\[productionBomVisibleRows, productionBomCategoryMoveActive\][\s\S]*if \(productionBomCategoryMoveActive\.value\) return[\s\S]*visibleKeys/)
  assert.match(source, /businessGroupVisibleRows/)
  assert.doesNotMatch(listPanel, /@click="selectBomRow\(row\)"/)
})

test('production BOM output identity supports material and product while preserving legacy product rows', () => {
  assert.deepEqual(bomLib.productionBomOutputIdentity({
    output_type: 'material', output_id: 27, output_material_id: 27,
    output_material_name: '烘焙熟豆', output_material_code: 'MAT-000027', output_unit: 'kg',
  }), { type: 'material', id: 27, name: '烘焙熟豆', code: 'MAT-000027', unit: 'kg' })
  assert.deepEqual(bomLib.productionBomOutputIdentity({
    output_product_id: 88, output_product_name: '227g 咖啡豆', output_product_code: 'SKU-000088',
  }), { type: 'product', id: 88, name: '227g 咖啡豆', code: 'SKU-000088', unit: '' })
  assert.deepEqual(bomLib.productionBomOutputPayload({ output_type: 'material', output_id: 27 }), {
    output_type: 'material', output_id: 27, output_product_id: 0, output_material_id: 27,
  })
  assert.deepEqual(bomLib.productionBomOutputPayload({ output_product_id: 88 }), {
    output_type: 'product', output_id: 88, output_product_id: 88, output_material_id: 0,
  })
  assert.equal(bomLib.productionBomOutputLabel({ output_type: 'material', output_material_id: 27, output_material_name: '烘焙熟豆' }), '物料 · 烘焙熟豆')
  assert.equal(bomLib.productionBomOutputLabel({ output_product_id: 88, output_product_name: '227g 咖啡豆' }), '商品 · 227g 咖啡豆')
})

test('BOM view edits one ordinary typed output contract and keeps PR596 inline grouping', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const form = template.match(/<form class="inline-form bom-record-form"[\s\S]*?<\/form>/)?.[0] || ''

  assert.match(form, /v-model="bomForm\.output_type"/)
  assert.match(form, /<option value="product">商品<\/option>/)
  assert.match(form, /<option value="material">物料<\/option>/)
  assert.match(form, /outputTargetOptions/)
  assert.match(source, /output_material_id/)
  assert.match(source, /productionBomOutputPayload/)
  assert.match(source, /productionBomOutputLabel/)
  assert.match(template, /<th>产出对象<\/th>/)
  assert.match(template, /设为产出对象默认 BOM/)
  assert.match(source, /setCurrentProductionBomAsDefault/)
  assert.match(source, /\/api\/materials\/\$\{outputID\}\/default-production-bom/)
  assert.match(source, /\/api\/products\/\$\{outputID\}\/default-production-bom/)
  assert.match(source, /BusinessGroupInlineWorkspace[\s\S]*productionBomDisplayGroups/)
  for (const forbidden of ['bom_kind', 'spec_packaging_bom_id', 'semi_finished_packaging_required']) {
    assert.doesNotMatch(source, new RegExp(forbidden))
  }
})

test('BOM product components always require a published BOM specification', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.match(template, /商品 BOM 规格/)
  assert.match(template, /v-model\.number="itemForm\.component_bom_spec_id"/)
  assert.match(source, /\/api\/products\/\$\{id\}\/bom-spec-options/)
	assert.doesNotMatch(source, /selectedComponentProductIdentityMode/)
	assert.doesNotMatch(template, /直接商品/)
	assert.match(source, /component_bom_spec_id: componentType === 'product' \? Number\(itemForm\.component_bom_spec_id/)
  assert.match(source, /商品组件必须选择明确的已发布 BOM 规格/)
  assert.match(source, /selectedComponentBomSpec\.value\?\.inventory_unit/)
})

test('BOM specification template variants edit material and published product-spec components independently', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const templateEditor = template.match(/<div v-if="selectedSpecTemplateVersion" class="spec-template-version-editor">[\s\S]*?<p v-if="!templateDraftVariants\.length"/)?.[0] || ''

  assert.match(templateEditor, /v-model="item\.component_type"/)
  assert.match(templateEditor, /<option value="material">物料<\/option>/)
  assert.match(templateEditor, /<option value="product"[^>]*>商品组件<\/option>/)
  assert.match(templateEditor, /v-model="item\.component_product_id"/)
  assert.match(templateEditor, /v-model\.number="item\.component_bom_spec_id"/)
  assert.match(templateEditor, /item\.component_bom_spec_options/)
  assert.match(source, /loadTemplateComponentProductSpecs\(variantIndex,\s*itemIndex/)
  assert.match(source, /\/api\/products\/\$\{id\}\/bom-spec-options/)
  assert.match(source, /templateVariantRecipeMode\(variant\)/)
  assert.match(source, /templateVariantItemConsumeUnitOptions\(variant,\s*item\)/)
  assert.match(source, /templateVariantItemInventoryUnit\(item\)/)
  assert.match(source, /component_type:\s*componentType/)
  assert.match(source, /component_product_id:\s*componentType === 'product'/)
  assert.match(source, /component_bom_spec_id:\s*componentType === 'product'/)
  assert.match(source, /商品规格组件只能使用固定用量/)
  assert.match(source, /物料固定用量必须使用该物料的库存单位/)
})

test('material option labels never fabricate SKU- prefixes for materials', async () => {
  assert.equal(bomLib.materialOptionLabel({ id: 123, name: '鲜豆', product_code: 'WL-0001' }), 'WL-0001 鲜豆')
  assert.equal(bomLib.materialOptionLabel({ id: 123, name: '鲜豆', code: 'WL-0001' }), 'WL-0001 鲜豆')
  assert.equal(bomLib.materialOptionLabel({ id: 123, name: '鲜豆' }), '鲜豆')
  assert.equal(bomLib.materialOptionLabel({ id: 123 }), '物料 #123')
  assert.doesNotMatch(bomLib.materialOptionLabel({ id: 123, name: '鲜豆' }), /SKU-/)

  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')

  assert.match(source, /materialOptionLabel,/)
  const materialSelectors = source.match(/:options="materialComponentOptions"[^>]*:option-label="optionLabel"/g) || []
  assert.equal(materialSelectors.length, 0, 'material selectors must not use product optionLabel')
  assert.match(source, /:options="materialComponentOptions"[^>]*:option-label="materialOptionLabel"|:option-label="materialOptionLabel"[^>]*:options="materialComponentOptions"/)
})

test('spec keys are generated internally as spec-N and hidden from editing UI', async () => {
  assert.equal(bomLib.nextSpecKey([]), 'spec-1')
  assert.equal(bomLib.nextSpecKey(['spec-1']), 'spec-2')
  assert.equal(bomLib.nextSpecKey(['spec-1', 'spec-3']), 'spec-4')
  assert.equal(bomLib.nextSpecKey(['spec-2', 'bag-227g']), 'spec-3')
  assert.equal(bomLib.nextSpecKey(['spec-10']), 'spec-11')

  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  assert.doesNotMatch(template, /稳定规格键/)
  assert.doesNotMatch(template, /placeholder="bag-227g"/)
  assert.doesNotMatch(template, /placeholder="例如 bag-227g"/)
  assert.match(source, /function nextSpecKey|nextSpecKey,/)
  assert.match(source, /spec_key:\s*nextSpecKey/)
  assert.doesNotMatch(source, /请填写每个规格的规格键、名称和库存单位/)
})

test('spec variant card uses compact default checkbox and 规格用量 naming', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const card = template.match(/<article v-for="\(variant, variantIndex\) in templateDraftVariants"[\s\S]*?<div class="spec-variant-components">/)?.[0] || ''

  assert.match(card, /规格用量/)
  assert.doesNotMatch(card, /主投入用量/)
  assert.doesNotMatch(card, /损耗比例/)
  assert.match(card, /<label class="checkbox-row compact-checkbox"[^>]*>\s*<input v-model="variant\.is_default" type="checkbox"/)
  assert.match(source, /\.spec-variant-grid \.checkbox-row input\s*\{[^}]*width:\s*16px/)
  assert.match(source, /规格用量和包材均在本规格内维护|规格用量/)
})

test('BomView loss-rate helpers stay wired to material output only', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const script = source.split('<script setup>')[1] || source

  for (const required of ['syncVersionMaterialLossRateFromSelectedVersion', 'handleVersionMaterialLossToggle', 'versionMaterialLossRateEnabled', 'isMaterialOutputBom']) {
    assert.ok(script.includes(required), `script must define ${required}`)
  }
  const syncCalls = (script.match(/syncVersionMaterialLossRateFromSelectedVersion\(\)/g) || []).length
  assert.ok(syncCalls >= 3, `version/variant switches must sync loss state, found ${syncCalls}`)
  assert.match(script, /const isMaterialOutputBom = computed/)
})

test('material loss changes recalculate every existing ratio component immediately', () => {
  const rows = [50, 15, 20, 15].map((ratio_pct, index) => ({
    id: index + 1,
    component_type: 'material',
    consume_unit: 'ratio_pct',
    ratio_pct,
    material_loss_rate: index === 0 ? 0.01 : 0,
  }))
  const updated = bomLib.applyVersionMaterialLossRate(rows, 0.195)
  assert.deepEqual(updated.map((row) => row.material_loss_rate), [0.195, 0.195, 0.195, 0.195])
  assert.deepEqual(updated.map((row) => Number(bomLib.materialLossAdjustedRatioPct(row).toFixed(2))), [62.11, 18.63, 24.84, 18.63])
  assert.deepEqual(rows.map((row) => row.material_loss_rate), [0.01, 0, 0, 0], 'helper must not mutate the saved snapshot')
})

test('BOM drawer keeps save feedback visible and routes immutable published output to replacement draft', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  const drawer = template.match(/<aside class="drawer bom-settings-drawer"[\s\S]*?<\/aside>/)?.[0] || ''
  assert.match(drawer, /data-bom-workspace-feedback/)
  assert.match(drawer, /v-if="error"/)
  assert.match(drawer, /v-if="ok"/)
  assert.match(source, /@input="handleVersionMaterialLossRateInput"/)
  assert.match(source, /published_output_identity_immutable/)
  assert.match(source, /\/replacement-draft/)
  assert.match(source, /bomWorkspaceSaveFailed/)
  assert.match(source, /当前 BOM 草稿保存失败/)
})

test('assignVariantSpecKeys fills missing keys and only rejects real duplicates', () => {
  const variants = [
    { spec_key: 'bag-227', name: 'a' },
    { spec_key: '', name: 'b' },
    { spec_key: 'spec-1', name: 'c' },
  ]
  bomLib.assignVariantSpecKeys(variants)
  assert.equal(variants[0].spec_key, 'bag-227')
  assert.equal(variants[1].spec_key, 'spec-2')
  assert.equal(variants[2].spec_key, 'spec-1')

  const fresh = [{ spec_key: '' }, { spec_key: '' }]
  bomLib.assignVariantSpecKeys(fresh)
  assert.equal(fresh[0].spec_key, 'spec-1')
  assert.equal(fresh[1].spec_key, 'spec-2')

  assert.throws(() => bomLib.assignVariantSpecKeys([{ spec_key: 'bag-227' }, { spec_key: 'BAG-227' }]), /规格键重复/)
})


test('material-output BOMs restore loss configuration while product spec groups keep fixed-only editing', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source

  const lossControl = template.match(/<div v-if="isMaterialOutputBom" class="material-loss-control[\s\S]*?<\/div>\s*<\/div>/)?.[0] || ''
  assert.match(lossControl, /原料损耗比/)
  assert.match(lossControl, /损耗比例 %/)
  assert.match(lossControl, /v-model="versionMaterialLossRateEnabled"/)
  assert.match(lossControl, /@change="handleVersionMaterialLossToggle"/)
  const templateCard = template.match(/<article v-for="\(variant, variantIndex\) in templateDraftVariants"[\s\S]*?<div class="spec-variant-components">/)?.[0] || ''
  assert.doesNotMatch(templateCard, /损耗比例/, 'spec template cards stay fixed-qty only')
  assert.doesNotMatch(template, /materialLossRateDisplay\}\}/, 'no dangling function-source rendering')
})

test('switching BOM output to material clears the spec group and saves a flat recipe', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const script = source.split('<script setup>')[1] || source

  assert.match(script, /function syncBomOutputType\(\)/)
  assert.match(script, /productionBomDetail\.value\.variants\s*=\s*\[\]/)
  assert.match(script, /isSpecGroupOutput\s*\?\s*\{\s*variants:/)
  assert.match(script, /:\s*\{\s*items:\s*detailItems\.value\.map\(productionBomDraftItemPayloadFromItem\)/)
  assert.doesNotMatch(script, /body:\s*\{[\s\S]{0,500}items:[\s\S]{0,500}variants:/)
  const hint = source.match(/改为物料产出[\s\S]{0,60}/)?.[0] || ''
  assert.match(hint, /规格组|规格模板/)
})
