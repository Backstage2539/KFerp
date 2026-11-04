import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const view = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')
const form = readFileSync(new URL('../components/OperationCompletionForm.vue', import.meta.url), 'utf8')

test('roasting task card exposes policy-aware start, completion and work order actions', () => {
  const actions = view.match(/<template v-if="isSimplifiedTask\(task\)">([\s\S]*?)<\/template>/)?.[1] || ''
  assert.match(actions, /工序完成/)
  assert.match(actions, /查看工单/)
  assert.equal((actions.match(/<button\b/g) || []).length, 3)
  assert.match(actions, /needsConfirmedStart\(task\)/)
  assert.match(actions, /开工/)
  assert.doesNotMatch(view, /继续处理下一工序|核对旧单并继续生产/)
})

test('completion form shows actual input for new policy and blocks an incomplete roasting BOM', () => {
  assert.match(form, /v-model="draft\.actual_output_qty"/)
  assert.match(form, /preview\.blocked_reason/)
  assert.match(form, /preview\.material_consumption_policy === 'bom_start_v1'/)
  assert.match(form, /v-model="draft\.actual_input_qty"/)
  assert.match(form, /已扣用料/)
  assert.match(form, /去 BOM 补齐配料/)
})
