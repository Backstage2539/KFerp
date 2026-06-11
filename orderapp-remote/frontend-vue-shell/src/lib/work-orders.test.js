import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

test('work orders display frozen route operations from process snapshot when no job-card summary exists', () => {
  const source = fs.readFileSync(new URL('../views/WorkOrdersView.vue', import.meta.url), 'utf8')

  assert.match(source, /processSnapshot\(row\)/)
  assert.match(source, /Array\.isArray\(snapshot\.operations\)/)
  assert.match(source, /operation:\s*item\.operation\s*\|\|\s*item\.operation_name/)
  assert.match(source, /workstation:\s*item\.workstation\s*\|\|\s*item\.workstation_name/)
  assert.match(source, /status:\s*item\.status\s*\|\|\s*'frozen'/)
  assert.match(source, /item\.workstation\s*\|\|\s*item\.workstation_name/)
})
