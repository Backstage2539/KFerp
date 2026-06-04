import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { test } from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))
const materialsSource = readFileSync(resolve(here, '../views/MaterialsView.vue'), 'utf8')
const warehouseSource = readFileSync(resolve(here, '../views/WarehouseInventoryView.vue'), 'utf8')
const stockAdjustmentsSource = readFileSync(resolve(here, '../views/StockAdjustmentsView.vue'), 'utf8')

test('warehouse settings opens for ordinary warehouses and shows empty state', () => {
  assert.doesNotMatch(warehouseSource, /:disabled="!selectedWarehouse \|\| !isExternalWarehouse"/)
  assert.match(warehouseSource, /openWarehouseSettingsDrawer/)
  assert.match(warehouseSource, /当前仓库暂无可配置项/)
})

test('materials view uses classification tabs and editable material records', () => {
  for (const expected of [
    '全部分类',
    '未分类',
    '增加分类',
    '移动到分类',
    '移动到小分类',
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

test('materials list owns filters, selection and batch deprecate layout', () => {
  assert.match(materialsSource, /class="material-list-toolbar"/)
  assert.match(materialsSource, /deprecateSelectedMaterials/)
  assert.match(materialsSource, /全选物料/)
  assert.match(materialsSource, /allSelected/)
  assert.match(materialsSource, /toggle-all/)
  assert.match(materialsSource, /min-width:\s*920px/)
  assert.match(materialsSource, /overflow-x:\s*auto/)

  const compactHeadStart = materialsSource.indexOf('<section class="panel compact-head">')
  const compactHeadEnd = materialsSource.indexOf('<div class="materials-layout">')
  const compactHeadSource = materialsSource.slice(compactHeadStart, compactHeadEnd)
  assert.doesNotMatch(compactHeadSource, /v-model\.trim="q"/)
  assert.doesNotMatch(compactHeadSource, /v-model="activeFilter"/)

  const listPanelStart = materialsSource.indexOf('<section class="panel material-list-panel">')
  const listPanelSource = materialsSource.slice(listPanelStart)
  assert.match(listPanelSource, /v-model\.trim="q"/)
  assert.match(listPanelSource, /v-model="activeFilter"/)
  assert.match(listPanelSource, /@click="deprecateSelectedMaterials"/)
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
