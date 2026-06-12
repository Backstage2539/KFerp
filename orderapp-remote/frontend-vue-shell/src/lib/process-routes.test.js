import assert from 'node:assert/strict'
import fs from 'node:fs'
import { test } from 'node:test'

test('process route page is route-only and does not select SKU or BOM versions', () => {
  const source = fs.readFileSync(new URL('../views/ProcessTemplatesView.vue', import.meta.url), 'utf8')

  assert.match(source, /<h2>工艺路线<\/h2>/)
  assert.match(source, /\/api\/process-routes/)
  assert.match(source, /路线工序/)
  assert.match(source, /工位\/设备/)
  assert.match(source, /quality_checklist_json/)
  assert.doesNotMatch(source, /绑定 SKU/)
  assert.doesNotMatch(source, /BOM版本/)
  assert.doesNotMatch(source, /form\.product_id/)
  assert.doesNotMatch(source, /bom_version_id/)
  assert.doesNotMatch(source, /\/api\/process-templates/)
  assert.doesNotMatch(source, /key_params_json/)
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
})

test('operation and workstation master data are maintained on separate pages', () => {
  const operationSource = fs.readFileSync(new URL('../views/ManufacturingOperationsView.vue', import.meta.url), 'utf8')
  const workstationSource = fs.readFileSync(new URL('../views/ManufacturingWorkstationsView.vue', import.meta.url), 'utf8')
  const appSource = fs.readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  assert.match(operationSource, /<h2>工序<\/h2>/)
  assert.match(operationSource, /\/api\/manufacturing-operations/)
  assert.doesNotMatch(operationSource, /\/api\/manufacturing-workstations/)
  assert.match(workstationSource, /<h2>工位\/设备<\/h2>/)
  assert.match(workstationSource, /\/api\/manufacturing-workstations/)
  assert.doesNotMatch(workstationSource, /\/api\/manufacturing-operations/)
  assert.match(appSource, /manufacturingOperations:\s*ManufacturingOperationsView/)
  assert.match(appSource, /manufacturingWorkstations:\s*ManufacturingWorkstationsView/)
})
