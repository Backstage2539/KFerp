import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

import {
  capacityCostMethodLabel,
  isCountCapacityUnit,
  normalizeCapacityCostMethod,
  workstationCapacityCostMeta,
  workstationCapacityOptionLabel,
  workstationCapacityUnitCost,
} from './workstation-capacity-costing.js'

test('legacy workstation capacities remain time costed', () => {
  const capacity = {
    name: '布勒 18kg',
    batch_size_qty: 18,
    batch_size_unit: 'kg',
    standard_minutes: 15,
    hourly_rate: 300,
  }

  assert.equal(normalizeCapacityCostMethod(capacity), 'time')
  assert.equal(capacityCostMethodLabel(capacity), '按时间')
  assert.equal(workstationCapacityUnitCost(capacity), 4.1667)
  assert.match(workstationCapacityCostMeta(capacity), /小时费率 300 × 标准分钟 15 \/ 60 \/ 标准批量 18kg = 4\.1667\/kg/)
  assert.equal(workstationCapacityOptionLabel(capacity), '布勒 18kg · 18kg · 15分钟/批 · 300/小时')
})

test('piece-costed workstation capacity presents an explicit per-piece rate', () => {
  const capacity = {
    name: '包装100件',
    cost_method: 'piece',
    piece_rate: 0.5,
    batch_size_qty: 100,
    batch_size_unit: '件',
    standard_minutes: 20,
    hourly_rate: 90,
  }

  assert.equal(normalizeCapacityCostMethod(capacity), 'piece')
  assert.equal(capacityCostMethodLabel(capacity), '按件')
  assert.equal(workstationCapacityUnitCost(capacity), 0.5)
  assert.equal(workstationCapacityCostMeta(capacity), '计件成本 0.5元/销售规格件')
  assert.equal(workstationCapacityOptionLabel(capacity), '包装100件 · 100件 · 20分钟/批 · 0.5元/销售规格件')
})

test('piece costing uses one generic sales-spec item instead of ambiguous package layers', () => {
  for (const unit of ['件', 'unit', 'pcs', 'piece']) {
    assert.equal(isCountCapacityUnit(unit), true, unit)
  }
  for (const unit of ['个', '袋', '盒', '包', '条', 'kg', 'g', 'lb', '磅']) {
    assert.equal(isCountCapacityUnit(unit), false, unit)
  }
})

test('workstation capacity editor exposes time and piece cost methods without editing inherited hourly rate', () => {
  const source = fs.readFileSync(new URL('../views/ManufacturingWorkstationsView.vue', import.meta.url), 'utf8')

  for (const marker of [
    '成本方式',
    '按时间',
    '按件',
    'capacityForm.cost_method',
    'capacityForm.piece_rate',
    '计件成本',
    '候选单位成本',
  ]) {
    assert.match(source, new RegExp(marker))
  }
  assert.doesNotMatch(source, /v-model\.number="capacityForm\.hourly_rate"/)
  assert.match(source, /@change="onCapacityCostMethodChange"/)
  assert.match(source, /v-if="capacityForm\.cost_method === 'piece'" value="件" type="text" readonly/)
  assert.match(source, /机器成本\/小时/)
  assert.match(source, /人工成本\/小时/)
  assert.match(source, /其他成本\/小时/)
})

test('production plan and work-order split editors share piece-aware capacity labels and snapshots', () => {
  for (const view of ['ProducePlanView.vue', 'WorkOrdersView.vue']) {
    const source = fs.readFileSync(new URL(`../views/${view}`, import.meta.url), 'utf8')
    assert.match(source, /workstationCapacityOptionLabel/)
    assert.match(source, /normalizeCapacityCostMethod/)
    assert.match(source, /cost_method/)
    assert.match(source, /piece_rate/)
  }
})
