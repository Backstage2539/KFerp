import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

function viewSource(name) {
  return fs.readFileSync(new URL(`../views/${name}`, import.meta.url), 'utf8')
}

test('shared category workspace uses a left scrollable tree and a right move-mode toolbar', () => {
  const workspace = fs.readFileSync(new URL('../components/BusinessGroupWorkspace.vue', import.meta.url), 'utf8')
  const controls = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')

  for (const marker of [
    'business-group-category-panel',
    'business-group-tree-scroll',
    'business-group-category-footer',
    '请选择要移动到的分类',
    '前往分组模板',
    '设置分组模板',
    'beginBusinessGroupMoveState',
    'restoreBusinessGroupMoveState',
    'scrollTop',
    "emit('target'",
    'business-group-list-disabled',
  ]) {
    assert.ok(workspace.includes(marker), `shared category workspace missing ${marker}`)
  }
  assert.match(workspace, /\.business-group-tree-scroll\s*\{[^}]*overflow-y:\s*auto;/s)
  assert.match(workspace, /watch\([\s\S]*props\.moveActive[\s\S]*await nextTick\(\)/s)
  assert.match(controls, /business-group-breadcrumb/)
  assert.match(controls, /移动到分类/)
  assert.doesNotMatch(controls, /目标分类/)
  assert.doesNotMatch(controls, /<select/)
})

const viewContracts = [
  {
    name: 'MaterialsView.vue',
    state: 'materialCategoryMoveActive',
    selected: 'selectedMaterialCategoryKey',
    target: 'handleMaterialCategoryMoveTarget',
    preserved: [
      'v-model.trim="q"',
      'placeholder="名称/编码/批次号"',
      'v-model="activeFilter"',
      "url.searchParams.set('active', activeFilter.value)",
      "url.searchParams.set('q', q.value)",
    ],
  },
  {
    name: 'BomView.vue',
    state: 'productionBomCategoryMoveActive',
    selected: 'selectedProductionBomCategoryKey',
    target: 'handleProductionBomCategoryMoveTarget',
    preserved: [
      'v-model="productionBomStatusFilter"',
      'v-model.trim="productionBomSearchQuery"',
      '按 BOM 名称或编号搜索',
      'filterProductionBomRows',
    ],
  },
  {
    name: 'ProductSettingsView.vue',
    state: 'productCategoryMoveActive',
    selected: 'selectedProductBusinessGroupCategoryKey',
    target: 'handleProductCategoryMoveTarget',
    preserved: [
      'v-model.trim="skuFilters.query"',
      '搜索商品名称/类型/备注',
      'v-model="skuFilters.active"',
      'filterSkuRows',
      'skuGroupTableState',
    ],
  },
  {
    name: 'WarehouseInventoryView.vue',
    state: 'inventoryCategoryMoveActive',
    selected: 'selectedInventoryCategoryKey',
    target: 'handleInventoryCategoryMoveTarget',
    preserved: [
      'v-model.trim="q"',
      'placeholder="物品/批次"',
      'v-model="itemType"',
      'warehouse-flat-list',
      'selectWarehouse(',
      'loadInventoryPage(1)',
    ],
  },
]

for (const contract of viewContracts) {
  test(`${contract.name} wires the unified immediate category move without replacing its filters`, () => {
    const source = viewSource(contract.name)
    for (const marker of [
      'BusinessGroupWorkspace',
      contract.state,
      contract.selected,
      contract.target,
      '@target=',
      ':move-active=',
      'businessGroupGroupsForCategorySelection',
      ...contract.preserved,
    ]) {
      assert.ok(source.includes(marker), `${contract.name} missing ${marker}`)
    }
    assert.doesNotMatch(source, /v-model:move-model-value=/)
  })
}

test('materials and warehouse category browsing cannot leave invisible rows selected for a later move', () => {
  const materials = viewSource('MaterialsView.vue')
  const warehouse = viewSource('WarehouseInventoryView.vue')

  for (const marker of [
    'pruneMaterialSelectionToVisibleCategory',
    'watch(selectedMaterialCategoryKey, pruneMaterialSelectionToVisibleCategory)',
    'visibleMaterialDisplayGroups.value',
  ]) {
    assert.ok(materials.includes(marker), `MaterialsView.vue missing ${marker}`)
  }
  assert.match(materials, /if \(materialCategoryMoveActive\.value\) return/)

  for (const marker of [
    'pruneInventorySelectionToVisibleCategory',
    'watch(selectedInventoryCategoryKey, pruneInventorySelectionToVisibleCategory)',
    'renderedInventoryRows.value',
  ]) {
    assert.ok(warehouse.includes(marker), `WarehouseInventoryView.vue missing ${marker}`)
  }
  assert.match(warehouse, /if \(inventoryCategoryMoveActive\.value\) return/)
})

test('all four pages disable refresh while a category move is active', () => {
  for (const [name, moveState] of [
    ['MaterialsView.vue', 'materialCategoryMoveActive'],
    ['BomView.vue', 'productionBomCategoryMoveActive'],
    ['ProductSettingsView.vue', 'productCategoryMoveActive'],
    ['WarehouseInventoryView.vue', 'inventoryCategoryMoveActive'],
  ]) {
    const source = viewSource(name)
    const marker = `:disabled="loading || ${moveState}">刷新</button>`
    assert.ok(source.includes(marker), `${name} must disable refresh with ${moveState}`)
  }
})
