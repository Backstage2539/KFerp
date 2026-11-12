import { test } from 'node:test'
import assert from 'node:assert/strict'
import { frozenWorkOrderDetails, workstationTodoTarget, executionBatchProgress, workOrderCostDetails, snapshotMaterialQuantity } from './work-order-detail-card.js'

test('detail uses frozen snapshot, never current BOM defaults, and preserves notes', () => {
  const detail = frozenWorkOrderDetails({ frozen_bom_version_id: 461, material_snapshot: '[{"material_name":"半成品","qty_per_unit":1,"consume_unit":"kg"}]', process_snapshot_json: '{"yield_rate":0.8}', related_orders:[{order_no:'SO-1',original_order_note:'2.5kg袋装'}] })
  assert.equal(detail.bomVersion, 461)
  assert.equal(detail.lossRate, 20)
  assert.equal(detail.materials[0].material_name, '半成品')
  assert.equal(detail.orders[0].original_order_note, '2.5kg袋装')
  assert.equal(frozenWorkOrderDetails({bom_version_id:999,expected_loss_rate:.2}).bomVersion, null)
  assert.equal(frozenWorkOrderDetails({bom_version_id:999,expected_loss_rate:.2}).lossRate, null)
})

test('workstation link resolves exact unfinished target with focus', () => {
  const target = workstationTodoTarget({related_links:[{view:'workstationView',params:{work_order_id:7,job_card_id:12}}]}, [{job_card_id:12,workstation:'包装台',workstation_id:3,status:'running'}],7)
  assert.deepEqual(target,{work_order_id:7,job_card_id:12,workstation:'包装台',workstation_id:3,station_id:3,focus:'workstation_task'})
  assert.equal(workstationTodoTarget({}, [], 7), null)
})

test('batch counts use actual batches, historical records are not guessed', () => {
  assert.deepEqual(executionBatchProgress({batches:[{status:'completed'},{status:'pending'}]}),{completed:1,total:2})
  assert.equal(executionBatchProgress({preview_batches:[{status:'pending'}]}), null)
})

test('unavailable costs never become actual zero', () => {
  assert.equal(workOrderCostDetails({id:0,total_cost:0},[]).final, null)
  assert.equal(workOrderCostDetails({id:4,total_cost:9,material_cost:4,operation_cost:5},[],null,{status:"completed",receipt_complete:true}).final, 9)
})

test('snapshot recipe presents the real output basis and never displays internal unit codes', () => {
  assert.equal(frozenWorkOrderDetails({process_snapshot_json:'{"bom_version_no":"V004"}'}).bomVersionLabel, 'V004')
  assert.equal(snapshotMaterialQuantity({qty_per_unit: 0.28375, consume_unit:'kg', output_qty:100, output_unit:'袋'}), '0.28375 kg / 100 袋')
  assert.equal(snapshotMaterialQuantity({qty_per_unit: 3, consume_unit:'unit_per_box'}), '3 个 / 盒')
  assert.equal(snapshotMaterialQuantity({qty_per_unit: 1.5, consume_unit:'mystery'}), '用量单位未记录')
  assert.equal(snapshotMaterialQuantity({qty_per_unit: 1, consume_unit:'kg'}), '1 kg（产出基准未记录）')
})

test('partial batch accounting is accumulated cost, never final cost', () => {
  const partial = workOrderCostDetails({id:1,total_cost:100,material_cost:80,operation_cost:20},[],80,{status:'running', receipt_complete:false})
  assert.equal(partial.final, null)
  assert.equal(partial.accumulated,100)
  const finished = workOrderCostDetails({id:1,total_cost:100,material_cost:80,operation_cost:20},[],80,{status:'completed', receipt_complete:true})
  assert.equal(finished.final,100)
})
