import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

function viewSource(name) {
  return fs.readFileSync(new URL(`../views/${name}`, import.meta.url), 'utf8')
}

test('shared inline category workspace uses collapsible headings as immediate move targets', () => {
  const workspace = fs.readFileSync(new URL('../components/BusinessGroupInlineWorkspace.vue', import.meta.url), 'utf8')
  const controls = fs.readFileSync(new URL('../components/BusinessGroupControls.vue', import.meta.url), 'utf8')

  for (const marker of [
    'data-business-group-inline-workspace',
    'business-group-inline-sections',
    'data-inline-group-header',
    'business-group-inline-footer',
    '请选择要移动到的分类',
    '点击分类标题立即移动，不再二次确认',
    '前往分组模板',
    '设置分组模板',
    'activateGroup(group)',
    'update:collapsedKeys',
    'moveSnapshot',
    'scrollTop',
    "emit('target'",
    'business-group-inline-disabled',
  ]) {
    assert.ok(workspace.includes(marker), `shared inline category workspace missing ${marker}`)
  }
  assert.match(workspace, /v-for="group in visibleGroups"/)
  assert.match(workspace, /watch\([\s\S]*props\.moveActive[\s\S]*emit\('update:collapsedKeys', \[\]\)[\s\S]*await nextTick\(\)/s)
  assert.match(workspace, /emit\('target',\s*\{[\s\S]*group_id:[\s\S]*group_item_id:[\s\S]*unclassified:/s)
  assert.doesNotMatch(workspace, /business-group-category-tree|business-group-tree-node|business-group-category-panel/)
  assert.match(controls, /business-group-breadcrumb/)
  assert.match(controls, /移动到分类/)
  assert.doesNotMatch(controls, /目标分类/)
  assert.doesNotMatch(controls, /<select/)
})

const viewContracts = [
  {
    name: 'MaterialsView.vue',
    state: 'materialCategoryMoveActive',
    collapsed: 'collapsedMaterialCategoryKeys',
    legacySelected: 'selectedMaterialCategoryKey',
    groups: 'paginatedMaterialGroups',
    target: 'handleMaterialCategoryMoveTarget',
    pagination: 'handleMaterialGroupPaginationChange',
    identity: ["const MATERIAL_CATALOG_USAGE = 'material_catalog'", "const MATERIAL_OBJECT_KEY = 'material'"],
    preserved: [
      'v-model.trim="q"',
      'placeholder="名称/编码/批次号"',
      'v-model="filters.active"',
      "url.searchParams.set('active', filters.active)",
      "url.searchParams.set('q', q.value)",
    ],
  },
  {
    name: 'BomView.vue',
    state: 'productionBomCategoryMoveActive',
    collapsed: 'collapsedProductionBomGroups',
    legacySelected: 'selectedProductionBomCategoryKey',
    groups: 'productionBomDisplayGroups',
    target: 'handleProductionBomCategoryMoveTarget',
    pagination: 'handleProductionBomGroupPaginationChange',
    identity: ["usageKey: 'production_bom'", "objectKey: 'production_bom'"],
    preserved: [
      'v-model="filters.status"',
      'v-model.trim="productionBomSearchQuery"',
      '按 BOM 名称或编号搜索',
      'filterProductionBomRows',
    ],
  },
  {
    name: 'ProductSettingsView.vue',
    state: 'productCategoryMoveActive',
    collapsed: 'collapsedProductClassificationGroups',
    legacySelected: 'selectedProductBusinessGroupCategoryKey',
    groups: 'displaySkuGroups',
    target: 'handleProductCategoryMoveTarget',
    pagination: 'handleProductGroupPaginationChange',
    identity: ["usageKey: 'product_catalog'", "objectKey: 'product'"],
    preserved: [
      'v-model.trim="skuFilters.query"',
      '搜索商品名称/类型/备注',
      'v-model="skuFilters.active"',
      'filterSkuRows',
      'businessGroupInlineListState',
    ],
  },
  {
    name: 'WarehouseInventoryView.vue',
    state: 'inventoryCategoryMoveActive',
    collapsed: 'collapsedInventoryGroupKeys',
    legacySelected: 'selectedInventoryCategoryKey',
    groups: 'pagedInventoryDisplayGroups',
    target: 'handleInventoryCategoryMoveTarget',
    pagination: 'handleInventoryGroupPaginationChange',
    identity: ["usageKey: 'warehouse_inventory'", "objectKey: 'warehouse_inventory_item'", 'objectRef: `${selectedWarehouse.value}:${key}`'],
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
      'BusinessGroupInlineWorkspace',
      contract.state,
      contract.collapsed,
      contract.target,
      `v-model:collapsed-keys="${contract.collapsed}"`,
      `:groups="${contract.groups}"`,
      '@target=',
      ':move-active=',
      '<template #group="{ group }">',
      'data-auto-pagination="off"',
      '<thead>',
      '<PaginationControls',
      `@change="${contract.pagination}(group.key, $event)"`,
      '/api/business-group-assignments',
      ...contract.identity,
      ...contract.preserved,
    ]) {
      assert.ok(source.includes(marker), `${contract.name} missing ${marker}`)
    }
    assert.doesNotMatch(source, /<BusinessGroupWorkspace(?:\s|>)/)
    assert.doesNotMatch(source, new RegExp(contract.legacySelected))
    assert.doesNotMatch(source, /businessGroupGroupsForCategorySelection/)
    assert.doesNotMatch(source, /v-model:move-model-value=/)
  })
}

test('materials page-wide selection only toggles the current inline category page', () => {
  const materials = viewSource('MaterialsView.vue')

  assert.match(materials, /:checked="areRowsSelected\(group\.rows\)"/)
  assert.match(materials, /@change="toggleMaterialRows\(group\.rows\)"/)
  assert.match(materials, /v-for="row in group\.rows"/)
})

test('move mode exits only after a successful target operation and remains active on failure', () => {
  const materials = viewSource('MaterialsView.vue')
  const bom = viewSource('BomView.vue')
  const product = viewSource('ProductSettingsView.vue')
  const warehouse = viewSource('WarehouseInventoryView.vue')

  assert.match(materials, /async function handleMaterialCategoryMoveTarget[\s\S]*?materialCategoryMoveActive\.value = false[\s\S]*?return true[\s\S]*?catch \(err\) \{[\s\S]*?return false/)
  assert.match(bom, /const completed = await moveSelectedProductBomsToGroup\(target\)[\s\S]*?if \(completed\) productionBomCategoryMoveActive\.value = false/)
  assert.match(bom, /async function moveSelectedProductBomsToGroup[\s\S]*?let completed = false[\s\S]*?completed = true[\s\S]*?return completed/)
  assert.match(product, /const moved = await saveSelectedProductBusinessGroupAssignment\(target\)[\s\S]*?if \(moved\) productCategoryMoveActive\.value = false/)
  assert.match(product, /async function saveSelectedProductBusinessGroupAssignment[\s\S]*?catch \(err\) \{[\s\S]*?return false/)
  assert.match(warehouse, /async function handleInventoryCategoryMoveTarget[\s\S]*?try \{[\s\S]*?inventoryCategoryMoveActive\.value = false[\s\S]*?\} catch \(err\) \{[\s\S]*?移动物品分类失败/)
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
