import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const source = readFileSync(new URL('../views/WarehouseInventoryView.vue', import.meta.url), 'utf8')
const template = source.split('<script setup>')[0] || source
const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

test('warehouse inventory keeps its outer warehouse selector and renders inline grouped tables only for a concrete factory warehouse', () => {
  const warehousePanel = template.match(/<aside class="panel warehouse-panel">[\s\S]*?<\/aside>/)?.[0] || ''

  assert.match(warehousePanel, /warehouse-flat-list/)
  assert.match(warehousePanel, /selectWarehouse\(row\.code\)/)
  assert.doesNotMatch(warehousePanel, /BusinessGroupInlineWorkspace/)

  assert.match(template, /<BusinessGroupInlineWorkspace[\s\S]*v-if="inventoryCategoryWorkspaceEnabled"/)
  assert.match(template, /v-model:collapsed-keys="collapsedInventoryGroupKeys"/)
  assert.match(template, /:groups="pagedInventoryDisplayGroups"/)
  assert.match(template, /#filters/)
  assert.match(template, /#group="\{ group \}"/)
  assert.match(template, /v-for="row in group\.rows"/)
  assert.match(template, /<thead>[\s\S]*<th>批次<\/th>[\s\S]*<th>操作<\/th>/)
  assert.match(template, /<PaginationControls[\s\S]*:page="group\.page"[\s\S]*:page-size="group\.pageSize"[\s\S]*:total="group\.total"[\s\S]*@change="handleInventoryGroupPaginationChange\(group\.key, \$event\)"/)
  assert.match(template, /<template v-else>[\s\S]*v-for="row in rows"/)
  assert.match(template, /<PaginationControls[\s\S]*v-if="!inventoryCategoryWorkspaceEnabled"[\s\S]*@change="handleInventoryPaginationChange"/)
  assert.doesNotMatch(template, /<BusinessGroupWorkspace/)
})

test('warehouse inline groups load the complete filtered warehouse result before independent category pagination', () => {
  assert.match(script, /import BusinessGroupInlineWorkspace from/)
  assert.match(script, /businessGroupInlineListState/)
  assert.match(script, /businessGroupVisibleRows/)
  assert.match(script, /const inventoryGroupPagination\s*=\s*ref\(\{\}\)/)
  assert.match(script, /const collapsedInventoryGroupKeys\s*=\s*ref\(\[\]\)/)
  assert.match(script, /businessGroupInlineListState\([\s\S]*?inventoryDisplayGroups\.value,[\s\S]*?inventoryGroupPagination\.value/)
  assert.match(script, /businessGroupVisibleRows\([\s\S]*?pagedInventoryDisplayGroups\.value,[\s\S]*?collapsedInventoryGroupKeys\.value/)
  assert.match(script, /function handleInventoryGroupPaginationChange\(groupKey, \{ page:[^}]+pageSize \}\)/)
  assert.match(script, /inventoryGroupPagination\.value\s*=\s*\{[\s\S]*\.\.\.inventoryGroupPagination\.value,[\s\S]*\[key\]:/)

  const requestURL = script.match(/function inventoryRequestURL\([\s\S]*?\n\}/)?.[0] || ''
  for (const key of ['q', 'warehouse', 'item_type', 'customer_id', 'page', 'limit']) {
    assert.match(requestURL, new RegExp(`url\\.searchParams\\.set\\('${key}'`))
  }
  assert.doesNotMatch(requestURL, /group_id|group_item_id/)
  const groupedLoad = script.match(/async function loadGroupedInventoryRows\(\)[\s\S]*?\n\}/)?.[0] || ''
  assert.match(groupedLoad, /GROUPED_INVENTORY_FETCH_LIMIT/)
  assert.match(groupedLoad, /Promise\.all/)
  assert.match(groupedLoad, /total_pages/)
  assert.match(groupedLoad, /rows\.value = \[firstRows,/)
  const loadInventory = script.match(/async function loadInventory\(\)[\s\S]*?\n\}/)?.[0] || ''
  assert.match(loadInventory, /inventoryCategoryWorkspaceEnabled\.value[\s\S]*loadGroupedInventoryRows\(\)/)
  assert.match(loadInventory, /else \{[\s\S]*paginationFromApi/)

  assert.doesNotMatch(script, /selectedInventoryCategoryKey/)
  assert.doesNotMatch(script, /businessGroupGroupsForCategorySelection/)
  assert.doesNotMatch(script, /pruneInventorySelectionToVisibleCategory/)
})

test('warehouse inline refactor preserves exact item-spec assignment identity and operational drawers', () => {
  assert.match(script, /return `\$\{selectedWarehouse\.value \|\| ''\}:\$\{inventoryItemKey\(row\)\}`/)
  assert.match(script, /return `\$\{row\.item_type \|\| ''\}:\$\{Number\(row\.item_id \|\| 0\)\}:\$\{Number\(row\.spec_g \|\| 0\)\}`/)
  assert.match(script, /objectKey:\s*'warehouse_inventory_item'/)
  assert.match(script, /const prefix = `\$\{selectedWarehouse\.value\}:`/)

  for (const marker of [
    'warehouseGroupFeatureDrawerOpen',
    'warehouseSettingsDrawerOpen',
    'traceDrawerOpen',
    'reservationDrawerOpen',
    '/api/stock/trace',
    '/api/produce/wip-reservations',
  ]) {
    assert.ok(source.includes(marker), `WarehouseInventoryView.vue missing ${marker}`)
  }
})
