import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

function source(relativePath) {
  const url = new URL(relativePath, import.meta.url)
  return fs.existsSync(url) ? fs.readFileSync(url, 'utf8') : ''
}

const inlineWorkspace = source('../components/BusinessGroupInlineWorkspace.vue')
const materialsView = source('../views/MaterialsView.vue')
const bomView = source('../views/BomView.vue')
const productView = source('../views/ProductSettingsView.vue')
const warehouseView = source('../views/WarehouseInventoryView.vue')

test('shared inline business group workspace replaces the permanent left category tree', () => {
  assert.match(inlineWorkspace, /data-business-group-inline-workspace/)
  assert.match(inlineWorkspace, /<BusinessGroupControls/)
  assert.match(inlineWorkspace, /v-for="group in visibleGroups"/)
  assert.match(inlineWorkspace, /<slot name="group" :group="group"/)
  assert.match(inlineWorkspace, /update:collapsedKeys/)
  assert.match(inlineWorkspace, /请选择要移动到的分类/)
  assert.match(inlineWorkspace, /点击分类标题立即移动，不再二次确认/)
  assert.match(inlineWorkspace, /group_id: Number\(group\.group_id/)
  assert.match(inlineWorkspace, /group_item_id: Number\(group\.group_item_id/)
  assert.match(inlineWorkspace, /unclassified: Boolean\(group\.unclassified\)/)
  assert.match(inlineWorkspace, /IconChevronDown/)
  assert.match(inlineWorkspace, /IconChevronRight/)
  assert.match(inlineWorkspace, /IconFolderOff/)
  assert.match(inlineWorkspace, /from '@tabler\/icons-vue'/)
  assert.doesNotMatch(inlineWorkspace, /<aside/)
  assert.doesNotMatch(inlineWorkspace, /business-group-category-tree/)
  assert.doesNotMatch(inlineWorkspace, /isCollapsed\(group\.key\) \? '\+' : '−'/)
})

for (const [label, viewSource] of [
  ['物料档案', materialsView],
  ['生产 BOM', bomView],
  ['商品档案', productView],
  ['仓库内部', warehouseView],
]) {
  test(`${label} uses the shared inline category list with a repeated header and group-keyed pager`, () => {
    assert.match(viewSource, /import BusinessGroupInlineWorkspace from/)
    assert.match(viewSource, /<BusinessGroupInlineWorkspace/)
    assert.match(viewSource, /v-model:collapsed-keys=/)
    assert.match(viewSource, /#group="\{ group \}"/)
    assert.match(viewSource, /<thead>/)
    assert.match(viewSource, /<PaginationControls[\s\S]*@change="handle\w+GroupPaginationChange\(group\.key, \$event\)"/)
    assert.match(viewSource, /data-business-group-item-row/)
    assert.doesNotMatch(viewSource, /<BusinessGroupWorkspace/)
  })
}

test('production BOM name opens one complete settings drawer instead of a permanent detail column', () => {
  assert.match(bomView, /data-bom-settings-drawer/)
  assert.match(bomView, /@click\.stop="openBomRowPrimary\(row\)"/)
  assert.match(bomView, /data-bom-settings-drawer[\s\S]*BOM 明细[\s\S]*BOM版本[\s\S]*配方明细/)
  assert.doesNotMatch(bomView, /class="panel detail-panel"/)
  assert.doesNotMatch(bomView, /@click="selectBomRow\(row\)"/)
})

test('material and production BOM category pagers only render above the shared threshold', () => {
  assert.match(materialsView, /<PaginationControls\s+v-if="group\.needsPagination"/)
  assert.match(bomView, /<PaginationControls\s+v-if="group\.needsPagination"/)
})

test('material name opens the material detail drawer instead of a permanent detail column', () => {
  assert.match(materialsView, /data-material-detail-drawer/)
  assert.match(materialsView, /material-name-button/)
  assert.match(materialsView, /物料详情/)
  assert.doesNotMatch(materialsView, /material-detail-panel/)
})

test('product inline categories preserve the existing product configuration drawer', () => {
  assert.match(productView, /productProductionConfigDrawerOpen/)
  assert.match(productView, /商品档案配置/)
  assert.match(productView, /@click="openProductProductionConfig\(row\)"/)
})

test('warehouse inline categories preserve the outer warehouse selector and exact inventory identity', () => {
  assert.match(warehouseView, /class="panel warehouse-panel"/)
  assert.match(warehouseView, /warehouse-flat-list/)
  assert.match(warehouseView, /inventoryCategoryWorkspaceEnabled/)
  assert.match(warehouseView, /warehouse_inventory_item/)
  assert.match(warehouseView, /inventoryItemObjectRef/)
  assert.match(warehouseView, /loadInventoryPage\(1\)/)
})

test('each page keeps its own search and filter contract', () => {
  assert.match(materialsView, /placeholder="名称\/编码\/批次号"/)
  assert.match(materialsView, /v-model="filters\.active"/)
  assert.match(bomView, /placeholder="按 BOM 名称或编号搜索"/)
  assert.match(bomView, /v-model="filters\.status"/)
  assert.match(productView, /placeholder="搜索商品名称\/类型\/备注"/)
  assert.match(productView, /v-model="skuFilters\.active"/)
  assert.match(warehouseView, /placeholder="物品\/批次"/)
  assert.match(warehouseView, /v-model="itemType"/)
})
