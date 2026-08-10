import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { test } from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))

function viewSource(name) {
  return readFileSync(resolve(here, `../views/${name}`), 'utf8')
}

test('shared business group workspace owns the category tree and emits immediate move targets', () => {
  const source = readFileSync(resolve(here, '../components/BusinessGroupWorkspace.vue'), 'utf8')

  assert.match(source, /business-group-category-tree/)
  assert.match(source, /business-group-tree-node/)
  assert.match(source, /node\.targetable/)
  assert.match(source, /emit\('target',\s*\{/)
  assert.match(source, /group_id:\s*Number\(node\.group_id/)
  assert.match(source, /group_item_id:\s*Number\(node\.group_item_id/)
  assert.match(source, /unclassified:\s*node\.kind === 'unclassified'/)
})

const featureViews = [
  {
    name: 'MaterialsView.vue',
    featureKey: 'material_catalog',
    title: '物料档案使用的分组模板',
    draft: 'materialGroupFeatureSelectionDraft',
    selectedRows: 'materialCatalogBusinessGroups',
    displayGroups: 'materialDisplayGroups',
    selectedCategory: 'selectedMaterialCategoryKey',
    moveActive: 'materialCategoryMoveActive',
    targetHandler: 'handleMaterialCategoryMoveTarget',
    selectionMarker: 'data-feature-key="material_catalog"',
    draftBinding: 'v-model="materialGroupFeatureSelectionDraft"',
  },
  {
    name: 'BomView.vue',
    featureKey: 'production_bom',
    title: '生产 BOM 使用的分组模板',
    draft: 'productionBomGroupFeatureSelectionDraft',
    selectedRows: 'productionBomSelectedBusinessGroups',
    displayGroups: 'fullProductionBomDisplayGroups',
    selectedCategory: 'selectedProductionBomCategoryKey',
    moveActive: 'productionBomCategoryMoveActive',
    targetHandler: 'handleProductionBomCategoryMoveTarget',
    selectionMarker: 'data-feature-key="production_bom"',
    draftBinding: 'v-model="productionBomGroupFeatureSelectionDraft"',
  },
  {
    name: 'ProductSettingsView.vue',
    featureKey: 'product_catalog',
    title: '商品档案分组模板设置',
    draft: 'productGroupFeatureSelectionDraft',
    selectedRows: 'productCatalogBusinessGroups',
    displayGroups: 'fullDisplaySkuGroups',
    selectedCategory: 'selectedProductBusinessGroupCategoryKey',
    moveActive: 'productCategoryMoveActive',
    targetHandler: 'handleProductCategoryMoveTarget',
    selectionMarker: 'product-group-template-list',
    draftBinding: 'productGroupFeatureSelectionDraft.includes(',
  },
  {
    name: 'WarehouseInventoryView.vue',
    featureKey: 'warehouse_inventory',
    title: '当前仓库内物品使用的分组模板',
    draft: 'warehouseGroupFeatureSelectionDraft',
    selectedRows: 'warehouseSelectedBusinessGroups',
    displayGroups: 'inventoryDisplayGroups',
    selectedCategory: 'selectedInventoryCategoryKey',
    moveActive: 'inventoryCategoryMoveActive',
    targetHandler: 'handleInventoryCategoryMoveTarget',
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

    assert.match(source, /BusinessGroupWorkspace/)
    assert.ok(source.includes(`v-model="${view.selectedCategory}"`))
    assert.ok(source.includes(`:groups="${view.displayGroups}"`))
    assert.ok(source.includes(`:move-active="${view.moveActive}"`))
    assert.ok(source.includes(`@target="${view.targetHandler}"`))
    assert.match(source, /@manage=/)
    assert.match(source, /@configure=/)
    assert.match(source, /businessGroupGroupsForCategorySelection/)
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
