import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { summarizeReplanQuantities } from './production-replan.js'

test('replan summary preserves business units and groups different units separately', () => {
  assert.equal(summarizeReplanQuantities([{ quantity: 11, unit: '袋' }]), '11 袋')
  assert.equal(summarizeReplanQuantities([{ quantity_g: 1749 }]), '1.749 kg')
  assert.equal(summarizeReplanQuantities([{ quantity: 11, unit: '袋' }, { quantity: 4, unit: 'kg' }]), '11 袋 · 4 kg')
  assert.equal(summarizeReplanQuantities([]), '0')
})

test('unstarted demand replan is reviewed then opens one new draft', () => {
  const page = readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  const workspace = readFileSync(new URL('../components/ProductionReplanWorkspace.vue', import.meta.url), 'utf8')
  for (const marker of ['/replan/preview', '/replan`', 'request_id', 'ProductionReplanWorkspace', '撤回并创建新草稿']) {
    assert.match(page, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  }
  for (const marker of ['原计划撤回范围', '合并新增待计划需求', '已领 WIP 怎么处理', '新计划合计', '创建新草稿前核对']) {
    assert.ok(workspace.includes(marker), marker)
  }
  assert.match(workspace, /原安排在最终确认前保持有效/)
})

test('workstation exposes replan only for pending tasks with plan identity', () => {
  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')
  assert.match(source, /\['pending','ready'\]\.includes\(task\.status\)/)
  assert.match(source, /replan_plan_id/)
  assert.match(source, /replan_item_id/)
})
