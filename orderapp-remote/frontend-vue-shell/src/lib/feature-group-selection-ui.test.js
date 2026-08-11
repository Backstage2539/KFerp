import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { test } from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))

function viewSource(name) {
  return readFileSync(resolve(here, `../views/${name}`), 'utf8')
}

test('shared inline business group workspace owns collapsible category headings and emits immediate move targets', () => {
  const source = readFileSync(resolve(here, '../components/BusinessGroupInlineWorkspace.vue'), 'utf8')

  assert.match(source, /data-business-group-inline-workspace/)
  assert.match(source, /v-for="group in visibleGroups"/)
  assert.match(source, /data-inline-group-header/)
  assert.match(source, /@click="activateGroup\(group\)"/)
  assert.match(source, /emit\('update:collapsedKeys'/)
  assert.match(source, /点击分类标题立即移动，不再二次确认/)
  assert.match(source, /emit\('target',\s*\{/)
  assert.match(source, /group_id:\s*Number\(group\.group_id/)
  assert.match(source, /group_item_id:\s*Number\(group\.group_item_id/)
  assert.match(source, /unclassified:\s*Boolean\(group\.unclassified\)/)
  assert.doesNotMatch(source, /business-group-category-tree|business-group-tree-node/)
})

const featureViews = [
  {
    name: 'MaterialsView.vue',
    featureKey: 'material_catalog',
    title: '物料档案使用的分组模板',
    draft: 'materialGroupFeatureSelectionDraft',
    selectedRows: 'materialCatalogBusinessGroups',
    displayGroups: 'paginatedMaterialGroups',
    collapsed: 'collapsedMaterialCategoryKeys',
    legacySelectedCategory: 'selectedMaterialCategoryKey',
    moveActive: 'materialCategoryMoveActive',
    targetHandler: 'handleMaterialCategoryMoveTarget',
    paginationHandler: 'handleMaterialGroupPaginationChange',
    selectionMarker: 'data-feature-key="material_catalog"',
    draftBinding: 'v-model="materialGroupFeatureSelectionDraft"',
  },
  {
    name: 'BomView.vue',
    featureKey: 'production_bom',
    title: '生产 BOM 使用的分组模板',
    draft: 'productionBomGroupFeatureSelectionDraft',
    selectedRows: 'productionBomSelectedBusinessGroups',
    displayGroups: 'productionBomDisplayGroups',
    collapsed: 'collapsedProductionBomGroups',
    legacySelectedCategory: 'selectedProductionBomCategoryKey',
    moveActive: 'productionBomCategoryMoveActive',
    targetHandler: 'handleProductionBomCategoryMoveTarget',
    paginationHandler: 'handleProductionBomGroupPaginationChange',
    selectionMarker: 'data-feature-key="production_bom"',
    draftBinding: 'v-model="productionBomGroupFeatureSelectionDraft"',
  },
  {
    name: 'ProductSettingsView.vue',
    featureKey: 'product_catalog',
    title: '商品档案分组模板设置',
    draft: 'productGroupFeatureSelectionDraft',
    selectedRows: 'productCatalogBusinessGroups',
    displayGroups: 'displaySkuGroups',
    collapsed: 'collapsedProductClassificationGroups',
    legacySelectedCategory: 'selectedProductBusinessGroupCategoryKey',
    moveActive: 'productCategoryMoveActive',
    targetHandler: 'handleProductCategoryMoveTarget',
    paginationHandler: 'handleProductGroupPaginationChange',
    selectionMarker: 'product-group-template-list',
    draftBinding: 'productGroupFeatureSelectionDraft.includes(',
  },
  {
    name: 'WarehouseInventoryView.vue',
    featureKey: 'warehouse_inventory',
    title: '当前仓库内物品使用的分组模板',
    draft: 'warehouseGroupFeatureSelectionDraft',
    selectedRows: 'warehouseSelectedBusinessGroups',
    displayGroups: 'pagedInventoryDisplayGroups',
    collapsed: 'collapsedInventoryGroupKeys',
    legacySelectedCategory: 'selectedInventoryCategoryKey',
    moveActive: 'inventoryCategoryMoveActive',
    targetHandler: 'handleInventoryCategoryMoveTarget',
    paginationHandler: 'handleInventoryGroupPaginationChange',
    selectionMarker: 'data-feature-key="warehouse_inventory"',
    draftBinding: 'v-model="warehouseGroupFeatureSelectionDraft"',
  },
]

for (const view of featureViews) {
  test(`${view.featureKey} owns an ordered multi-template selection and only exposes selected templates`, () => {
    const source = viewSource(view.name)
    const endpoint = `/api/business-group-feature-selections/${view.featureKey}`

    assert.match(source, new RegExp(endpoint.replaceAll('/', '\\/')))
    assert.match(source, /businessGroupFeatureSelectionIDs/)
    assert.match(source, /businessGroupFeatureSelectionPayload/)
    assert.match(source, /businessGroupRowsForFeatureSelection/)
    assert.ok(source.includes(view.selectionMarker))
    assert.match(source, new RegExp(view.title))
    assert.ok(source.includes(view.draftBinding))
    assert.ok(source.includes(`${view.draft}.value`))
    assert.match(source, /type="checkbox"/)
    assert.match(source, new RegExp(`${view.selectedRows}[^]*businessGroupRowsForFeatureSelection`))
    assert.ok(source.includes(`templates: ${view.selectedRows}.value`))
    assert.doesNotMatch(source, /preferredBusinessGroupTemplateID/)

    assert.match(source, /BusinessGroupInlineWorkspace/)
    assert.ok(source.includes(`v-model:collapsed-keys="${view.collapsed}"`))
    assert.ok(source.includes(`:groups="${view.displayGroups}"`))
    assert.ok(source.includes(`:move-active="${view.moveActive}"`))
    assert.ok(source.includes(`@target="${view.targetHandler}"`))
    assert.match(source, /<template #group="\{ group \}">/)
    assert.match(source, /<table[^>]*data-auto-pagination="off"[^>]*>[\s\S]*?<thead>/)
    assert.match(source, new RegExp(`<PaginationControls[\\s\\S]*?@change="${view.paginationHandler}\\(group\\.key, \\$event\\)"`))
    assert.match(source, /@manage=/)
    assert.match(source, /@configure=/)
    assert.doesNotMatch(source, /<BusinessGroupWorkspace(?:\s|>)/)
    assert.doesNotMatch(source, new RegExp(view.legacySelectedCategory))
    assert.doesNotMatch(source, /businessGroupGroupsForCategorySelection/)
    assert.match(source, new RegExp(`async function ${view.targetHandler}\\(target(?: = \\{\\})?\\)`))
    assert.match(source, /unclassified/)
    assert.match(source, /group_id/)
    assert.match(source, /group_item_id/)
    assert.doesNotMatch(source, /v-model:move-model-value=/)

    const saveCallStart = source.indexOf(`apiSend('${endpoint}'`)
    assert.notEqual(saveCallStart, -1)
    const saveCall = source.slice(saveCallStart, saveCallStart + 320)
    assert.match(saveCall, /method:\s*'PUT'/)
    assert.match(saveCall, /body:\s*payload/)
    assert.doesNotMatch(saveCall, /business-group-assignments/)
  })
}
