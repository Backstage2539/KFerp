import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

const read = (path) => fs.readFileSync(new URL(path, import.meta.url), 'utf8')

test('work order list keeps six status and action columns', () => {
  const source = read('../views/WorkOrdersView.vue')
  const start = source.indexOf('data-work-order-status-list')
  const end = source.indexOf('</table>', start)
  const table = source.slice(start, end)
  for (const label of ['产品 / 工单', '目标数量', '当前状态', '工序进度', '当前待办', '快捷操作']) {
    assert.match(table, new RegExp(label.replace('/', '\\/')))
  }
  assert.equal((table.match(/<th>/g) || []).length, 6)
  assert.match(table, /查看工单/)
  for (const label of ['编辑拆分', '取消工单']) assert.match(table, new RegExp(label))
  for (const obsolete of ['更多', '打印', 'printWorkOrder']) assert.doesNotMatch(table, new RegExp(obsolete))
})

test('work order list becomes a task card before tablet actions overflow', () => {
  const source = read('../views/WorkOrdersView.vue')
  assert.match(source, /@media\s*\(max-width:\s*900px\)/)
  assert.match(source, /\.work-order-list-card thead\{display:none\}/)
  assert.match(source, /\.work-order-list-card tr\{display:grid/)
  assert.match(source, /data-label="快捷操作"/)
  assert.match(source, /content:attr\(data-label\)/)
})

test('work order detail is a status bus with embedded operation records', () => {
  const source = read('../components/ProductionExecutionHubDrawer.vue')
  assert.match(source, /工单详情/)
  assert.match(source, /工序记录/)
  assert.match(source, /进入工位/)
  assert.match(source, /分配任务/)
  assert.doesNotMatch(source, />开始生产</)
  assert.doesNotMatch(source, />生产领料</)
})

test('workstation owns task execution and report excludes receipt warehouse', () => {
  const source = read('../views/WorkstationView.vue')
  assert.match(source, /我的今日工位/)
  assert.match(source, /今日负责人/)
  assert.match(source, /接手工位/)
  assert.match(source, /查看生产排班/)
  assert.doesNotMatch(source, /领取任务/)
  assert.doesNotMatch(source, /ProductionTaskStaffEditor/)
  assert.doesNotMatch(source, /ProductionTopNav/)
  assert.match(source, /查看工单/)
  assert.match(source, /第.*批.*共.*批/)
  assert.match(source, /物料名称/)
  assert.match(source, /进入本工位/)
  assert.match(source, /loadStatusLabel/)
  assert.match(source, /ungroupedTasks\(section\)\.length/)
  assert.match(source, /class="task-table"/)
  assert.doesNotMatch(source, /<span>入库仓<\/span>/)
})

test('quality is task first, explicit, and uses ordinary metric fields', () => {
  const source = read('../views/QualityInspectionsView.vue')
  assert.match(source, /待质检任务/)
  assert.match(source, /请选择检查结果/)
  assert.match(source, /提交后将影响/)
  assert.doesNotMatch(source, /指标 JSON/)
  assert.doesNotMatch(source, /form\.result[^\n]*['"]pass['"]/)
})

test('production flow embeds work order records without a standalone reconciliation entry', () => {
  const source = read('../views/ProductionFlowView.vue')
  assert.match(source, /工序记录/)
  assert.doesNotMatch(source, /生产记录与核对/)
  assert.doesNotMatch(source, /生产验收/)

  const receipt = read('../components/WorkOrderReceiptForm.vue')
  assert.match(receipt, /确认数量并入库/)
  assert.match(receipt, /已入库/)
  assert.match(receipt, /legacy-review\/submit/)
  assert.doesNotMatch(read('../components/ProductionExecutionHubDrawer.vue'), /ProductionRecordsView|去完工入库|showReceiptRecords/)
  assert.doesNotMatch(receipt.slice(0, receipt.indexOf('<script setup>')), /完成记录|旧单核对|acceptance-smoke/)

  const operationRecords = read('../views/JobCardsView.vue')
  const menu = read('./menu-ia.js')
  const operationTable = operationRecords.slice(operationRecords.indexOf('<table'), operationRecords.indexOf('</table>'))
  assert.doesNotMatch(operationRecords, /原执行枢纽/)
  assert.match(menu, /jobCards:\s*'工序记录'/)
  assert.equal((operationTable.match(/<th>/g) || []).length, 5)
})
