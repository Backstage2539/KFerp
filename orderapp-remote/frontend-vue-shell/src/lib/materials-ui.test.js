import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { test } from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))
const materialsSource = readFileSync(resolve(here, '../views/MaterialsView.vue'), 'utf8')
const materialReceiptsSource = readFileSync(resolve(here, '../views/MaterialReceiptsView.vue'), 'utf8')
const warehouseSource = readFileSync(resolve(here, '../views/WarehouseInventoryView.vue'), 'utf8')
const stockAdjustmentsSource = readFileSync(resolve(here, '../views/StockAdjustmentsView.vue'), 'utf8')
const purchaseSource = readFileSync(resolve(here, '../views/PurchaseView.vue'), 'utf8')
const stockEntriesSource = readFileSync(resolve(here, '../views/StockEntriesView.vue'), 'utf8')

test('warehouse settings opens from selected warehouse while grouping is handled by shared controls', () => {
  const componentSource = readFileSync(resolve(here, '../components/BusinessGroupControls.vue'), 'utf8')

  assert.doesNotMatch(warehouseSource, /:disabled="!selectedWarehouse \|\| !isExternalWarehouse"/)
  assert.match(warehouseSource, /openWarehouseSettingsDrawer/)
  assert.match(warehouseSource, /BusinessGroupInlineWorkspace/)
  assert.doesNotMatch(componentSource, /选择分组模板|目标分类|<select/)
  assert.match(componentSource, /移动到分类/)
  assert.match(warehouseSource, /\/api\/business-group-assignments/)
  assert.doesNotMatch(warehouseSource, /warehouseGroupForm/)
})

test('warehouse inventory uses shared business grouping helpers', () => {
  assert.match(warehouseSource, /businessGroupControlOptions/)
  assert.match(warehouseSource, /businessGroupRowsForFeatureSelection/)
  assert.match(warehouseSource, /businessGroupFeatureSelectionPayload/)
  assert.match(warehouseSource, /business-group-feature-selections\/warehouse_inventory/)
  assert.match(warehouseSource, /groupRowsByBusinessGroupTemplates\(/)
  assert.match(warehouseSource, /businessGroupMoveAssignmentPayload/)
  assert.match(warehouseSource, /objectKey:\s*'warehouse_inventory_item'/)
  assert.match(warehouseSource, /inventoryItemObjectRef/)
  assert.doesNotMatch(warehouseSource, /preferredBusinessGroupTemplateID/)
  assert.doesNotMatch(warehouseSource, /\[group\.name \|\| '库存分组', parentName, item\.name/)
  assert.doesNotMatch(warehouseSource, /仓库库存默认分组/)
  assert.doesNotMatch(warehouseSource, /businessGroupItemMoveOptions/)
})

test('warehouse inventory groups warehouses by template without ordinary customer warehouse sections', () => {
  const componentSource = readFileSync(resolve(here, '../components/BusinessGroupControls.vue'), 'utf8')
  const template = warehouseSource.split('<script setup>')[0] || warehouseSource
  const warehousePanel = template.match(/<aside class="panel warehouse-panel">[\s\S]*?<\/aside>/)?.[0] || template

  assert.match(warehouseSource, /BusinessGroupInlineWorkspace/)
  assert.match(warehouseSource, /groupRowsByBusinessGroupTemplates\(/)
  assert.doesNotMatch(componentSource, /选择分组模板|目标分类|<select/)
  assert.match(componentSource, /移动到分类/)
  assert.doesNotMatch(warehousePanel, /BusinessGroupWorkspace/)
  assert.doesNotMatch(warehousePanel, /v-for="group in warehouseDisplayGroups"/)
  assert.doesNotMatch(warehousePanel, /toggleWarehouseSelection/)
  assert.match(warehousePanel, /warehouse-flat-list/)
  assert.doesNotMatch(warehousePanel, /普通仓库/)
  assert.doesNotMatch(warehousePanel, /客户仓库/)
  assert.doesNotMatch(warehouseSource, /generalWarehouses/)
  assert.doesNotMatch(warehouseSource, /customerWarehouses/)
  assert.doesNotMatch(warehouseSource, /warehouseSections/)
  assert.match(warehouseSource, /inventoryDisplayGroups/)
  assert.match(warehouseSource, /toggleInventoryItemSelection/)
  assert.match(warehouseSource, /canMoveSelectedInventoryItems/)
})

test('warehouse inventory script setup has no stale pre-rename grouping identifiers', () => {
  const scriptSetup = warehouseSource.split('<script setup>')[1]?.split('</script>')[0] || ''
  assert.ok(scriptSetup, 'expected a <script setup> block in WarehouseInventoryView.vue')

  const staleIdentifiers = [
    'selectedWarehouseGroupTemplateID',
    'warehouseGroupItemOptions',
    'selectedWarehouseMoveGroupItemID',
    'selectedWarehouseKeys',
    'collapsedWarehouseGroups',
    'syncSelectedWarehouseGroupTemplate',
  ]
  for (const name of staleIdentifiers) {
    assert.doesNotMatch(scriptSetup, new RegExp(name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }

  assert.match(scriptSetup, /const collapsedInventoryGroupKeys\s*=\s*ref\(\[\]\)/)
  assert.match(scriptSetup, /const inventoryCategoryMoveActive\s*=\s*ref\(false\)/)
  assert.match(scriptSetup, /handleInventoryCategoryMoveTarget/)
})

test('group template page manages categories without business objects', () => {
  const settingsSource = readFileSync(resolve(here, '../views/GroupTemplatesView.vue'), 'utf8')
  const templatePanel = settingsSource.match(/data-section-mode="groupTemplates"[\s\S]*/)?.[0] || settingsSource

  assert.match(settingsSource, /分组模板/)
  assert.match(settingsSource, /新增分组模板/)
  assert.match(settingsSource, /删除模板/)
  assert.match(settingsSource, /deleteGroupTemplate/)
  assert.match(settingsSource, /\/api\/business-groups\/\$\{id\}/)
  assert.match(settingsSource, /method:\s*'DELETE'/)
  assert.match(settingsSource, /新增大类/)
  assert.match(settingsSource, /新增小类/)
  assert.match(settingsSource, /\/api\/business-groups/)
  assert.match(settingsSource, /\/api\/business-group-items/)
  assert.doesNotMatch(templatePanel, /groupTemplateForm\.active/)
  assert.doesNotMatch(templatePanel, /template\.active === false \? '停用' : '启用'/)
  assert.doesNotMatch(templatePanel, /已选|勾选|移动到分类|\/api\/business-group-assignments/)
})

test('materials view uses group template classification and editable material records', () => {
  for (const expected of [
    'BusinessGroupInlineWorkspace',
    '新建物料',
    '批量失效',
    'saveMaterial',
    'industry_field_template_id',
    'materialIndustryFields',
  ]) {
    assert.match(materialsSource, new RegExp(expected.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  for (const forbidden of [
    '销售价',
    '咖啡生豆属性',
    'copySelectedMaterial',
    '基础档案字段锁定',
    '库存(g)',
    '库存(个)',
    '目标库存(g)',
    '目标库存(个)',
    '物料类型',
  ]) {
    assert.doesNotMatch(materialsSource, new RegExp(forbidden.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
})

test('materials archive classification uses the shared inline category list without a target dropdown', () => {
  const componentSource = readFileSync(resolve(here, '../components/BusinessGroupControls.vue'), 'utf8')
  const listPanelStart = materialsSource.indexOf('<section class="panel material-list-panel">')
  const listPanelSource = materialsSource.slice(listPanelStart)

  assert.match(materialsSource, /BusinessGroupInlineWorkspace/)
  assert.doesNotMatch(materialsSource, /<BusinessGroupWorkspace/)
  assert.doesNotMatch(materialsSource, /<BusinessGroupControls/)
  assert.match(materialsSource, /businessGroupControlOptions/)
  assert.match(materialsSource, /businessGroupRowsForFeatureSelection/)
  assert.match(materialsSource, /businessGroupFeatureSelectionPayload/)
  assert.match(materialsSource, /business-group-feature-selections\/material_catalog/)
  assert.doesNotMatch(materialsSource, /preferredBusinessGroupTemplateID/)
  assert.match(materialsSource, /groupRowsByBusinessGroupTemplates\(/)
  assert.doesNotMatch(materialsSource, /groupRowsByBusinessGroupTemplate\(/)
  assert.match(materialsSource, /businessGroupInlineListState/)
  assert.match(materialsSource, /businessGroupMoveAssignmentPayload/)
  assert.match(materialsSource, /MATERIAL_CATALOG_USAGE\s*=\s*'material_catalog'/)
  assert.match(materialsSource, /MATERIAL_OBJECT_KEY\s*=\s*'material'/)
  assert.match(materialsSource, /objectKey:\s*MATERIAL_OBJECT_KEY/)
  assert.match(materialsSource, /\/api\/business-group-assignments/)
  assert.match(materialsSource, /v-model:collapsed-keys="collapsedMaterialCategoryKeys"/)
  assert.match(materialsSource, /:move-active="materialCategoryMoveActive"/)
  assert.match(materialsSource, /@target="handleMaterialCategoryMoveTarget"/)
  assert.match(materialsSource, /:disabled="loading \|\| materialCategoryMoveActive">刷新/)
  assert.doesNotMatch(materialsSource, /selectedMaterialCategoryKey|pruneMaterialSelectionToVisibleCategory|visibleMaterialDisplayGroups/)
  assert.doesNotMatch(componentSource, /选择分组模板|目标分类|<select/)
  assert.match(componentSource, /移动到分类/)
  assert.doesNotMatch(listPanelSource, /v-model:move-model-value|selectedMaterialMoveGroupItemID/)
  assert.doesNotMatch(listPanelSource, /增加分类|新增小分类|移动到小分类/)
  assert.doesNotMatch(materialsSource, /\/api\/material-classification-groups/)
  assert.doesNotMatch(materialsSource, /\/api\/material-classification-assignments/)
})

test('materials archive aligns every selected template into the category workspace', () => {
  assert.match(materialsSource, /const materialCatalogBusinessGroups = computed\(\(\) => businessGroupRowsForFeatureSelection\(/)
  assert.match(materialsSource, /templates:\s*materialCatalogBusinessGroups\.value/)
  assert.match(materialsSource, /:groups="paginatedMaterialGroups"/)
  assert.match(materialsSource, /#group="\{ group \}"/)
  assert.match(materialsSource, /openMaterialGroupFeatureSelectionDrawer/)
  assert.match(materialsSource, /materialGroupFeatureDrawerOpen/)
  assert.match(materialsSource, /设置分组模板/)
  assert.doesNotMatch(materialsSource, /businessGroupGroupsForCategorySelection|skuGroupHiddenByCollapsedAncestor/)
})

test('materials inline categories repeat the table header and paginate every category independently', () => {
  assert.match(materialsSource, /const materialGroupPagination\s*=\s*ref\(\{\}\)/)
  assert.match(materialsSource, /businessGroupInlineListState\(\s*materialDisplayGroups\.value,\s*materialGroupPagination\.value/)
  assert.match(materialsSource, /const paginatedMaterialGroups = computed/)
  assert.match(materialsSource, /<template #group="\{ group \}">[\s\S]*<thead>[\s\S]*<\/thead>/)
  assert.match(materialsSource, /<table class="materials-table" data-auto-pagination="off">/)
  assert.match(materialsSource, /<PaginationControls[\s\S]*:page="group\.page"[\s\S]*:page-size="group\.pageSize"[\s\S]*:total="group\.total"/)
  assert.match(materialsSource, /@change="handleMaterialGroupPaginationChange\(group\.key, \$event\)"/)
  assert.match(materialsSource, /function handleMaterialGroupPaginationChange\(groupKey, \{ page, pageSize \}\)/)
  assert.match(materialsSource, /toggleMaterialRows\(group\.rows\)/)
  assert.match(materialsSource, /areRowsSelected\(group\.rows\)/)
})

test('materials category target executes immediately and only clears move state after success', () => {
  const handlerStart = materialsSource.indexOf('async function handleMaterialCategoryMoveTarget(target = {})')
  const handlerEnd = materialsSource.indexOf('\nfunction openMaterialBusinessGroupManagement', handlerStart)
  assert.ok(handlerStart >= 0 && handlerEnd > handlerStart, 'expected material category target handler')

  const handlerSource = materialsSource.slice(handlerStart, handlerEnd)
  const catchStart = handlerSource.indexOf('} catch (err) {')
  const finallyStart = handlerSource.indexOf('} finally {', catchStart)
  assert.ok(catchStart > 0 && finallyStart > catchStart, 'expected retry-preserving error handling')
  const successSource = handlerSource.slice(0, catchStart)
  const failureSource = handlerSource.slice(catchStart, finallyStart)

  assert.match(handlerSource, /materialCategoryMoveActive\.value/)
  assert.match(handlerSource, /target\.group_id/)
  assert.match(handlerSource, /target\.group_item_id/)
  assert.match(handlerSource, /target\.unclassified/)
  assert.match(handlerSource, /clearMaterialBusinessGroupAssignment/)
  assert.match(handlerSource, /await apiSend\('\/api\/business-group-assignments'/)
  assert.match(handlerSource, /businessGroupMoveAssignmentPayload\(/)
  assert.match(successSource, /return true/)

  const clearSelectionIndex = successSource.indexOf('selectedMaterialIDs.value = []')
  const finishMoveIndex = successSource.indexOf('materialCategoryMoveActive.value = false')
  const refreshAssignmentsIndex = successSource.indexOf('await loadMaterialBusinessGroupAssignments()')
  const successMessageIndex = successSource.indexOf('ok.value = `已移动')
  assert.ok(refreshAssignmentsIndex > 0, 'successful move refreshes assignments')
  assert.ok(successMessageIndex > refreshAssignmentsIndex, 'success is announced only after assignment refresh succeeds')
  assert.ok(clearSelectionIndex > 0, 'successful move clears checked materials')
  assert.ok(finishMoveIndex > clearSelectionIndex, 'move mode ends only after checked materials are cleared')

  assert.match(failureSource, /移动物料分类失败，请重试/)
  assert.match(failureSource, /return false/)
  assert.doesNotMatch(failureSource, /selectedMaterialIDs\.value\s*=\s*\[\]/)
  assert.doesNotMatch(failureSource, /materialCategoryMoveActive\.value\s*=\s*false/)
  assert.doesNotMatch(failureSource, /loadMaterialBusinessGroupAssignments/, 'failed refresh keeps the pre-move assignment snapshot for a retry')
})

test('materials list owns filters, selection and batch deprecate layout', () => {
  assert.match(materialsSource, /class="material-list-toolbar"/)
  assert.match(materialsSource, /deprecateSelectedMaterials/)
  assert.match(materialsSource, /全选物料/)
  assert.match(materialsSource, /toggleMaterialRows\(group\.rows\)/)
  assert.match(materialsSource, /table-layout:\s*fixed/)
  assert.match(materialsSource, /min-width:\s*660px/)
  assert.match(materialsSource, /overflow-x:\s*auto/)
  assert.match(materialsSource, /col\.name-col\)\s*\{\s*width:\s*130px;\s*\}/)
  assert.match(materialsSource, /\.materials-table th:nth-child\(2\)\),\s*\.table-wrap :deep\(\.materials-table td:nth-child\(2\)\)\s*\{\s*width:\s*130px;\s*max-width:\s*130px;/)
  assert.match(materialsSource, /\.material-name-button\s*\{[^}]*overflow-wrap:\s*anywhere/)
  assert.match(materialsSource, /td small\)\s*\{[^}]*max-width:\s*120px/)
  assert.doesNotMatch(materialsSource, /col\.name-col\)\s*\{\s*width:\s*390px;\s*\}/)

  const compactHeadStart = materialsSource.indexOf('<section class="panel compact-head">')
  const compactHeadEnd = materialsSource.indexOf('<div class="materials-layout">')
  const compactHeadSource = materialsSource.slice(compactHeadStart, compactHeadEnd)
  assert.doesNotMatch(compactHeadSource, /v-model\.trim="q"/)
  assert.doesNotMatch(compactHeadSource, /v-model="filters\.active"/)

  const listPanelStart = materialsSource.indexOf('<section class="panel material-list-panel">')
  const listPanelSource = materialsSource.slice(listPanelStart)
  assert.match(listPanelSource, /v-model\.trim="q" placeholder="名称\/编码\/批次号" @keyup\.enter="applyMaterialFilters"/)
  assert.match(listPanelSource, /v-model="filters\.active" @change="applyMaterialFilters"/)
  assert.match(listPanelSource, /<option value="active">启用<\/option>/)
  assert.match(listPanelSource, /<option value="inactive">失效<\/option>/)
  assert.match(listPanelSource, /<option value="all">全部<\/option>/)
  assert.match(listPanelSource, /@click="deprecateSelectedMaterials"/)
  assert.match(materialsSource, /const filters\s*=\s*reactive\(\{\s*active:\s*'active'/)
  assert.match(materialsSource, /url\.searchParams\.set\('limit', '500'\)/)
  assert.match(materialsSource, /url\.searchParams\.set\('active', filters\.active\)/)
  assert.match(materialsSource, /if \(q\.value\) url\.searchParams\.set\('q', q\.value\)/)
})

test('materials list is full width and material names open the detail drawer', () => {
  assert.match(materialsSource, /\.materials-layout\s*\{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\)/)
  assert.match(materialsSource, /\.material-list-panel\s*\{[^}]*min-width:\s*0/)
  assert.match(materialsSource, /\.table-wrap\s*\{[^}]*max-width:\s*100%[^}]*overflow-x:\s*auto/)
  assert.match(materialsSource, /\.form-grid\s*\{[^}]*repeat\(auto-fit,\s*minmax\(180px,\s*1fr\)\)/)
  assert.match(materialsSource, /data-material-detail-drawer/)
  assert.match(materialsSource, /class="material-name-button"[\s\S]*@click\.stop="selectMaterial\(row\)"/)
  assert.match(materialsSource, /materialDetailDrawerOpen\.value\s*=\s*true/)
  assert.doesNotMatch(materialsSource, /material-detail-panel|onClick:\s*\(\)\s*=>\s*emit\('select'/)
})

test('materials and stock adjustments use single material quantity from material unit', () => {
  assert.match(materialsSource, /stockBackfill\.target_qty/)
  assert.match(materialsSource, /target_qty/)
  assert.match(materialsSource, /unit_code/)
  assert.match(materialsSource, /全局单位字典/)
  assert.doesNotMatch(materialsSource, /stockBackfill\.target_g/)
  assert.doesNotMatch(materialsSource, /stockBackfill\.target_units/)

  assert.match(stockAdjustmentsSource, /selectedMaterialUnitLabel/)
  assert.match(stockAdjustmentsSource, /form\.target_qty/)
  assert.match(stockAdjustmentsSource, /unit_code/)
  assert.doesNotMatch(stockAdjustmentsSource, /<span>目标\(g\/散装g\)<\/span>[\s\S]*v-model\.number="form\.target_g"/)
  assert.doesNotMatch(stockAdjustmentsSource, /<span>目标件数<\/span>[\s\S]*v-model\.number="form\.target_units"/)
})

test('existing material inventory unit is locked after create', () => {
  assert.match(materialsSource, /materialInventoryUnitLocked/)
  assert.match(materialsSource, /:disabled="materialInventoryUnitLocked"/)
  assert.match(materialsSource, /库存单位保存后不可修改/)
  assert.match(materialsSource, /unit:\s*draftMode\.value\s*\?\s*draft\.value\.unit\s*:\s*\(selected\.value\?\.unit\s*\|\|\s*draft\.value\.unit\)/)
})

test('materials archive uses inventory unit as the only purchase and cost price unit', () => {
  assert.doesNotMatch(materialsSource, /采购价与成本单价单位/)
  assert.doesNotMatch(materialsSource, /materialCostUnitLocked/)
  assert.doesNotMatch(materialsSource, /data-field="cost_unit"/)
  assert.doesNotMatch(materialsSource, /defaultMaterialCostUnit/)
  assert.match(materialsSource, /重量物料库存统一使用 kg；BOM 配方仍可按 g 录入并自动换算/)
  assert.match(materialsSource, /采购价、批次单位成本和 BOM 成本试算均按库存单位计价/)
  assert.match(materialsSource, /采购价（元\/\{\{\s*draft\.unit\s*\}\}）/)
  assert.match(materialsSource, /cost_unit:\s*draftMode\.value\s*\?\s*draft\.value\.unit\s*:\s*\(selected\.value\?\.unit\s*\|\|\s*draft\.value\.unit\)/)
  assert.match(materialsSource, /unit_type:\s*row\.unit_type\s*\?\?\s*row\.UnitType\s*\?\?\s*'other'/)
  assert.match(materialsSource, /function isMaterialWeightUnitType\(unitType\)/)
  assert.match(materialsSource, /normalized === 'weight' \|\| normalized === '重量'/)
  assert.match(materialsSource, /function isCanonicalMaterialInventoryUnit\(unitCode,\s*unitType\)/)
  assert.match(materialsSource, /filter\(\(row\) => isCanonicalMaterialInventoryUnit\(row\.code,\s*row\.unit_type\)\)/)
  assert.match(materialsSource, /find\(\(row\) => normalizeMaterialUnitCode\(row\.code\) === 'kg'\)/)
  assert.match(materialsSource, /const unit = unitOptions\.value\.find\(\(row\) => normalizeMaterialUnitCode\(row\.code\) === 'kg'\)\?\.code \|\| 'kg'/)
})

test('material receipt, stock adjustment and purchase prices use the inventory unit directly', () => {
  assert.match(materialReceiptsSource, /成本（元\/\{\{\s*selectedMaterialUnitLabel\s*\}\}）/)
  assert.match(materialReceiptsSource, /const selectedMaterialUnitLabel = computed\(\(\) => unitDisplay\(selectedMaterial\.value\?\.unit \|\| selectedMaterial\.value\?\.Unit \|\| '-'\)\)/)
  assert.match(materialReceiptsSource, /入库数量（\{\{ selectedMaterialUnitLabel \}\}）/)
  assert.doesNotMatch(materialReceiptsSource, /CostUnit|cost_unit|materialCostUnit|selectedMaterialCostUnitLabel/)

  assert.match(stockAdjustmentsSource, /目标成本（元\/\{\{\s*selectedMaterialUnitLabel\s*\}\}）/)
  assert.match(stockAdjustmentsSource, /补录成本（元\/\{\{\s*selectedMaterialUnitLabel\s*\}\}）/)
  assert.doesNotMatch(stockAdjustmentsSource, /CostUnit|cost_unit|materialCostUnit|selectedMaterialCostUnitLabel/)
  assert.doesNotMatch(stockAdjustmentsSource, /目标成本\/千克|补录成本\/千克/)

  assert.match(purchaseSource, /selectedPurchaseMaterialUnit/)
  assert.match(purchaseSource, /单价（元\/\{\{\s*selectedPurchaseMaterialUnit\s*\}\}）/)
  assert.match(purchaseSource, /materialUnit\(row\.material_id\)/)
  assert.doesNotMatch(purchaseSource, /CostUnit|cost_unit|materialCostUnit|selectedPurchaseMaterialCostUnit/)
  assert.match(purchaseSource, /purchasableMaterials/)
  assert.match(purchaseSource, /当前采购单按克记录数量，仅支持重量物料/)
  assert.match(stockAdjustmentsSource, /isSelectedMaterialWeight/)
  assert.match(stockAdjustmentsSource, /批次成本调整当前只支持重量物料/)
})

test('material receipt freezes unit to the selected material master', () => {
  assert.match(materialReceiptsSource, /<input\s+:value="selectedMaterialUnitLabel"\s+readonly/)
  assert.doesNotMatch(materialReceiptsSource, /<select\s+v-model="form\.unit_code"/)
  assert.doesNotMatch(materialReceiptsSource, /apiGet\('\/api\/product-settings'\)/)
  assert.doesNotMatch(materialReceiptsSource, /productUnitDefinitions|unitOptions|form\.unit_code/)
  assert.match(materialReceiptsSource, /unit_code:\s*selectedMaterial\.value\?\.unit\s*\|\|\s*selectedMaterial\.value\?\.Unit\s*\|\|\s*''/)
})

test('materials keep derived manufacturing status beside output BOM links without duplicate technical hints', () => {
  const template = materialsSource.split('<script setup>')[0] || materialsSource

  assert.match(template, /v-model="draft\.is_semi_finished"/)
  assert.match(template, /是否半成品/)
  assert.match(template, /can_manufacture/)
  assert.match(template, /产出该物料的 BOM/)
  assert.match(template, /使用该物料的 BOM/)
  assert.match(template, /draft\.can_manufacture\s*\?\s*'可制造（已有默认发布 BOM）'\s*:\s*'不可制造（无默认发布 BOM）'/)
  assert.doesNotMatch(template, /可制造能力（只读）|data-field="can_manufacture"/)
  assert.doesNotMatch(template, /任意有效物料都可以作为普通 BOM 的产出对象；半成品勾选不参与校验。/)
  assert.match(materialsSource, /producedByBoms/)
  assert.match(materialsSource, /usedByBoms/)
  assert.match(materialsSource, /output_type=material/)
  assert.match(materialsSource, /component_type=material/)
  assert.match(materialsSource, /function openMaterialBom/)
  assert.match(materialsSource, /kferp:navigate-view/)
  assert.match(materialsSource, /returnNavigation/)
  assert.match(materialsSource, /open_material_id/)
  assert.match(materialsSource, /materialReturnNavigation/)
  assert.match(materialsSource, /returnToMaterialSource/)
  assert.match(materialsSource, /is_semi_finished:\s*Boolean\(draft\.value\.is_semi_finished\)/)
})

test('semi-finished material is manufacture-only in material, purchase, receipt, and stock-entry forms', () => {
  const materialsTemplate = materialsSource.split('<script setup>')[0] || materialsSource

  assert.match(materialsTemplate, /<label\s+v-if="!draft\.is_semi_finished"><span>采购价/)
  assert.match(materialsSource, /watch\(\(\)\s*=>\s*draft\.value\?\.is_semi_finished,[\s\S]*draft\.value\.purchase_price\s*=\s*0/)
  assert.match(materialsSource, /purchase_price:\s*draft\.value\.is_semi_finished\s*\?\s*0\s*:\s*Number\(draft\.value\.purchase_price\s*\|\|\s*0\)/)

  assert.match(purchaseSource, /isSemiFinishedMaterial/)
  assert.match(purchaseSource, /const purchasableMaterials = computed\(\(\) => materials\.value\.filter\(\(material\) => isMaterialWeight\(material\) && !isSemiFinishedMaterial\(material\)\)\)/)
  assert.match(purchaseSource, /isPurchasableMaterialByID/)

  assert.match(stockEntriesSource, /selectableStockEntryMaterials/)
  assert.match(stockEntriesSource, /const stockEntryMaterialOptions = computed\(\(\) => selectableStockEntryMaterials\(materials\.value, form\.purpose_key\)\)/)
  assert.match(stockEntriesSource, /:options="item\.item_type === 'material' \? stockEntryMaterialOptions : products"/)
})

test('materials locally filter the loaded page by semi-finished marker without changing manufacturing capability', () => {
  const template = materialsSource.split('<script setup>')[0] || materialsSource

  assert.match(template, /<span>半成品标识<\/span>[\s\S]*v-model="filters\.semiFinished"/)
  assert.match(template, /<option value="all">全部<\/option>/)
  assert.match(template, /<option value="semi_finished">半成品<\/option>/)
  assert.match(template, /<option value="non_semi_finished">非半成品<\/option>/)
  assert.match(template, /v-model="filters\.semiFinished" @change="applySemiFinishedFilter"/)
  assert.match(materialsSource, /const filters\s*=\s*reactive\(\{\s*active:\s*'active',\s*semiFinished:\s*'all'\s*\}\)/)
  assert.match(materialsSource, /const filteredMaterialRows = computed\(\(\) => \{[\s\S]*filters\.semiFinished === 'semi_finished'[\s\S]*row\.is_semi_finished[\s\S]*filters\.semiFinished === 'non_semi_finished'[\s\S]*!row\.is_semi_finished/)
  assert.match(materialsSource, /groupRowsByBusinessGroupTemplates\(filteredMaterialRows\.value,/)
  assert.match(materialsSource, /function applySemiFinishedFilter\(\)\s*\{[\s\S]*resetMaterialGroupPages\(\)/)

  const loadMaterialsStart = materialsSource.indexOf('async function loadMaterials(')
  const loadMaterialsEnd = materialsSource.indexOf('\nfunction applyMaterialFilters()', loadMaterialsStart)
  const loadMaterialsSource = materialsSource.slice(loadMaterialsStart, loadMaterialsEnd)
  assert.doesNotMatch(loadMaterialsSource, /semi_finished|semiFinished/)
  assert.match(materialsSource, /is_semi_finished:\s*Boolean\(row\.IsSemiFinished \?\? row\.is_semi_finished \?\? false\),\s*\n\s*can_manufacture:\s*Boolean\(row\.CanManufacture \?\? row\.can_manufacture \?\? false\)/)
})
