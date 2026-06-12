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
