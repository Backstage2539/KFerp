import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { test } from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))

function viewSource(name) {
  return readFileSync(resolve(here, `../views/${name}`), 'utf8')
}

const featureViews = [
  {
    name: 'MaterialsView.vue',
    featureKey: 'material_catalog',
    title: '物料档案使用的分组模板',
    draft: 'materialGroupFeatureSelectionDraft',
    selectedRows: 'materialCatalogBusinessGroups',
  },
  {
    name: 'BomView.vue',
    featureKey: 'production_bom',
    title: '生产 BOM 使用的分组模板',
    draft: 'productionBomGroupFeatureSelectionDraft',
    selectedRows: 'productionBomSelectedBusinessGroups',
  },
  {
    name: 'WarehouseInventoryView.vue',
    featureKey: 'warehouse_inventory',
    title: '当前仓库内物品使用的分组模板',
    draft: 'warehouseGroupFeatureSelectionDraft',
    selectedRows: 'warehouseSelectedBusinessGroups',
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
    assert.match(source, new RegExp(`data-feature-key="${view.featureKey}"`))
    assert.match(source, new RegExp(view.title))
    assert.match(source, new RegExp(`v-model="${view.draft}"`))
    assert.match(source, /type="checkbox"/)
    assert.match(source, /保存模板选择/)
    assert.match(source, new RegExp(`${view.selectedRows}[^]*businessGroupRowsForFeatureSelection`))
    assert.ok(source.includes(`businessGroupControlOptions(${view.selectedRows}`))
    assert.match(source, new RegExp(`v-if=["'][^"']*${view.selectedRows}\.length`))
    assert.match(source, /尚未选择分组模板，当前平铺展示/)
    assert.doesNotMatch(source, /preferredBusinessGroupTemplateID/)

    const saveCallStart = source.indexOf(`apiSend('${endpoint}'`)
    assert.notEqual(saveCallStart, -1)
    const saveCall = source.slice(saveCallStart, saveCallStart + 320)
    assert.match(saveCall, /method:\s*'PUT'/)
    assert.match(saveCall, /body:\s*payload/)
    assert.doesNotMatch(saveCall, /business-group-assignments/)
  })
}
