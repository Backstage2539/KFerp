import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

import { canStartWorkOrder, workOrderStartEndpoint, workOrderStatusOptions } from './work-orders.js'

test('work orders display frozen route operations from process snapshot when no job-card summary exists', () => {
  const source = fs.readFileSync(new URL('../views/WorkOrdersView.vue', import.meta.url), 'utf8')

  assert.match(source, /processSnapshot\(row\)/)
  assert.match(source, /Array\.isArray\(snapshot\.operations\)/)
  assert.match(source, /operation:\s*item\.operation\s*\|\|\s*item\.operation_name/)
  assert.match(source, /workstation:\s*item\.workstation\s*\|\|\s*item\.workstation_name/)
  assert.match(source, /status:\s*item\.status\s*\|\|\s*'frozen'/)
  assert.match(source, /item\.workstation\s*\|\|\s*item\.workstation_name/)
  assert.match(source, /开始生产/)
  assert.match(source, /startWorkOrder\(row\)/)
  assert.match(source, /workOrderStartEndpoint\(row\)/)
})

test('workOrderStatusOptions includes draft and released lifecycle states before running', () => {
  assert.deepEqual(workOrderStatusOptions().map((item) => item.value), [
    '',
    'draft',
    'released',
    'running',
    'completed',
    'cancelled',
  ])
})

test('canStartWorkOrder allows only released work orders', () => {
  assert.equal(canStartWorkOrder({ id: 41, status: 'released', running_item_id: 0 }), true)
  assert.equal(canStartWorkOrder({ id: 42, status: 'running', running_item_id: 99 }), false)
  assert.equal(canStartWorkOrder({ id: 43, status: 'draft', running_item_id: 0 }), false)
  assert.equal(canStartWorkOrder({ status: 'released', running_item_id: 0 }), false)
})

test('workOrderStartEndpoint uses formal work order start API', () => {
  assert.equal(workOrderStartEndpoint({ id: 41 }), '/api/work-orders/41/start')
  assert.equal(workOrderStartEndpoint({ id: 0 }), '')
})
