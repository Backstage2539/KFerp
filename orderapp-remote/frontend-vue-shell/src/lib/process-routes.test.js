import assert from 'node:assert/strict'
import fs from 'node:fs'
import { test } from 'node:test'
import { workstationCapacityCostMeta } from './workstation-capacity-costing.js'

test('process route page is route-only and does not select SKU or BOM versions', () => {
  const source = fs.readFileSync(new URL('../views/ProcessTemplatesView.vue', import.meta.url), 'utf8')

  assert.match(source, /<h2>工艺路线<\/h2>/)
  assert.match(source, /\/api\/process-routes/)
  assert.match(source, /路线工序/)
  assert.match(source, /quality_checklist_json/)
  assert.doesNotMatch(source, /绑定 SKU/)
  assert.doesNotMatch(source, /BOM版本/)
  assert.doesNotMatch(source, /form\.product_id/)
  assert.doesNotMatch(source, /bom_version_id/)
  assert.doesNotMatch(source, /\/api\/process-templates/)
  assert.doesNotMatch(source, /key_params_json/)
})

test('publishing a process route saves current operation edits before changing status', () => {
  const source = fs.readFileSync(new URL('../views/ProcessTemplatesView.vue', import.meta.url), 'utf8')
  const publishRouteSource = source.match(/async function publishRoute\(\) \{([\s\S]*?)\n\}/)?.[1] || ''
  const saveCall = "apiSend('/api/process-routes', { body: routePayload() })"
  const publishCall = 'apiSend(`/api/process-routes/${routeId}/publish`, { body: {} })'

  assert.match(publishRouteSource, /const saved = await apiSend\('\/api\/process-routes', \{ body: routePayload\(\) \}\)/)
  assert.match(publishRouteSource, /const routeId = Number\(saved\.id \|\| form\.id \|\| 0\)/)
  assert.match(publishRouteSource, /apiSend\(`\/api\/process-routes\/\$\{routeId\}\/publish`, \{ body: \{\} \}\)/)
  assert.match(publishRouteSource, /\|\| \{ \.\.\.saved, status: 'active' \}/)
  assert.ok(publishRouteSource.indexOf(saveCall) < publishRouteSource.indexOf(publishCall))
})

test('process route page keeps route list and editor in a desktop two pane layout', () => {
  const source = fs.readFileSync(new URL('../views/ProcessTemplatesView.vue', import.meta.url), 'utf8')

  assert.match(source, /class="grid process-route-layout"/)
  assert.match(source, /class="panel route-list-panel"/)
  assert.match(source, /class="panel editor route-editor-panel"/)
  assert.match(source, /\.process-route-layout\s*\{[^}]*grid-template-columns:\s*minmax\(300px,\s*360px\)\s*minmax\(0,\s*1fr\)/s)
  assert.match(source, /\.route-editor-panel\s*\{[^}]*min-width:\s*0;[^}]*overflow:\s*hidden;/s)
  assert.match(source, /\.form-grid\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/s)
  assert.doesNotMatch(source, /@media \(max-width:\s*1180px\)[\s\S]*\.grid/)
})

test('process route operation editor aligns fields and hides raw quality checklist JSON', () => {
  const source = fs.readFileSync(new URL('../views/ProcessTemplatesView.vue', import.meta.url), 'utf8')

  assert.match(source, /class="operation-row-fields"/)
  assert.match(source, /class="operation-quality"/)
  assert.match(source, /qualityChecklistTextFromJSON/)
  assert.match(source, /qualityChecklistJSONFromText/)
  assert.match(source, /每行一个质检项/)
  assert.doesNotMatch(source, /质检项 JSON/)
  assert.doesNotMatch(source, /v-model\.trim="op\.quality_checklist_json"/)
  assert.doesNotMatch(source, /工位\/设备/)
})

test('process route operation row selects standard cost capacity without actual production capacity fields', () => {
  const source = fs.readFileSync(new URL('../views/ProcessTemplatesView.vue', import.meta.url), 'utf8')

  assert.match(source, /\/api\/manufacturing-workstation-capacities\?status=active/)
  assert.match(source, /标准成本产能档/)
  assert.match(source, /standard_cost_capacity_id/)
  assert.match(source, /standardCostCapacityOptions/)
  assert.match(source, /standardCostCapacitySummary/)
  assert.equal(workstationCapacityCostMeta({
    cost_method: 'time',
    hourly_rate: 24,
    batch_size_qty: 10,
    batch_size_unit: 'kg',
    standard_minutes: 30,
  }), '小时费率 24 × 标准分钟 30 / 60 / 标准批量 10kg = 1.2/kg')
  assert.match(source, /只用于 BOM\/价格标准成本/)
  assert.doesNotMatch(source, /生产计划实际产能/)
  assert.doesNotMatch(source, /实际工位产能/)
  assert.doesNotMatch(source, /workstation_capacity_id/)
  assert.doesNotMatch(source, /workstation_capacity_name/)
  assert.doesNotMatch(source, /applyWorkstationCapacity/)
  assert.doesNotMatch(source, /plannedOperationCostFormula/)
  assert.doesNotMatch(source, /自动折算计划工序成本/)
  assert.doesNotMatch(source, /planned_operation_cost:\s*Number/)
  assert.doesNotMatch(source, /planned_batch_count/)
  assert.doesNotMatch(source, /planned_minutes/)
  assert.doesNotMatch(source, /标准分钟\/批/)
  assert.doesNotMatch(source, /工位能力模式/)
  assert.doesNotMatch(source, /默认工时\(分钟\)/)
})

test('process route standard cost summary supports time and piece capacity methods', () => {
  const source = fs.readFileSync(new URL('../views/ProcessTemplatesView.vue', import.meta.url), 'utf8')

  assert.match(source, /workstationCapacityCostMeta/)
  assert.match(source, /cost_method/)
  assert.match(source, /piece_rate/)
  assert.equal(workstationCapacityCostMeta({
    cost_method: 'piece',
    piece_rate: 0.5,
    batch_size_unit: '件',
  }), '计件成本 0.5元/销售规格件')
})

test('operation and workstation master data are maintained on separate pages', () => {
  const operationSource = fs.readFileSync(new URL('../views/ManufacturingOperationsView.vue', import.meta.url), 'utf8')
  const workstationSource = fs.readFileSync(new URL('../views/ManufacturingWorkstationsView.vue', import.meta.url), 'utf8')
  const appSource = fs.readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  assert.match(operationSource, /<h2>工序<\/h2>/)
  assert.match(operationSource, /\/api\/manufacturing-operations/)
  assert.doesNotMatch(operationSource, /标准工序成本/)
  assert.doesNotMatch(operationSource, /v-model\.number="form\.standard_operation_cost"/)
  assert.doesNotMatch(operationSource, /元\/库存单位/)
  assert.doesNotMatch(operationSource, /\/api\/manufacturing-workstations/)
  assert.match(workstationSource, /<h2>工位\/设备<\/h2>/)
  assert.match(workstationSource, /\/api\/manufacturing-workstations/)
  assert.match(workstationSource, /\/api\/manufacturing-workstation-capacities/)
  assert.match(workstationSource, /\/api\/manufacturing-operations/)
  assert.match(workstationSource, /工位产能/)
  assert.match(workstationSource, /适用工序/)
  assert.match(workstationSource, /form\.applicable_operation_ids/)
  assert.doesNotMatch(workstationSource, /capacityForm\.applicable_operation_ids/)
  assert.doesNotMatch(workstationSource, /未配置适用工序的产能/)
  assert.match(appSource, /manufacturingOperations:\s*ManufacturingOperationsView/)
  assert.match(appSource, /manufacturingWorkstations:\s*ManufacturingWorkstationsView/)
})

test('operation and workstation master data no longer expose default minutes as authoritative fields', () => {
  const operationSource = fs.readFileSync(new URL('../views/ManufacturingOperationsView.vue', import.meta.url), 'utf8')
  const workstationSource = fs.readFileSync(new URL('../views/ManufacturingWorkstationsView.vue', import.meta.url), 'utf8')

  assert.doesNotMatch(operationSource, /默认工时\(分钟\)|默认分钟/)
  assert.doesNotMatch(workstationSource, /默认工时\(分钟\)|默认分钟/)
  assert.match(workstationSource, /机器成本\/小时/)
  assert.match(workstationSource, /人工成本\/小时/)
  assert.match(workstationSource, /其他成本\/小时/)
  assert.match(workstationSource, /小时成本合计/)
  assert.doesNotMatch(workstationSource, /默认小时费率/)
  assert.doesNotMatch(workstationSource, /v-model\.number="capacityForm\.hourly_rate"/)
  assert.doesNotMatch(operationSource, /工位能力模式/)
  assert.doesNotMatch(workstationSource, /工位能力模式/)
})

test('operation and workstation pages use left list and right detail layout', () => {
  const operationSource = fs.readFileSync(new URL('../views/ManufacturingOperationsView.vue', import.meta.url), 'utf8')
  const workstationSource = fs.readFileSync(new URL('../views/ManufacturingWorkstationsView.vue', import.meta.url), 'utf8')

  for (const source of [operationSource, workstationSource]) {
    assert.match(source, /class="grid master-data-layout"/)
    assert.match(source, /class="panel master-list-panel/)
    assert.match(source, /class="panel editor master-editor-panel/)
    assert.match(source, /\.master-data-layout\s*\{[^}]*grid-template-columns:\s*minmax\(300px,\s*360px\)\s*minmax\(0,\s*1fr\)/s)
    assert.match(source, /\.master-editor-panel\s*\{[^}]*min-width:\s*0;[^}]*overflow:\s*hidden;/s)
    assert.match(source, /\.form-grid\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/s)
    assert.doesNotMatch(source, /class="panel table-wrap"/)
    assert.doesNotMatch(source, /table\s*\{[^}]*min-width:\s*(680|760)px/s)
    assert.doesNotMatch(source, /@media \(max-width:\s*980px\)[\s\S]*\.grid/)
  }

  assert.match(operationSource, /@click="editOperation\(row\)"/)
  assert.match(workstationSource, /@click="editWorkstation\(row\)"/)
})
