import test from 'node:test'
import assert from 'node:assert/strict'
import * as trial from './price-template-editor-trial.js'

const rows = [
  { parent_product_id: 10, product_id: 10, product_name: '商品 A', bom_id: 5, bom_version_id: 50, bom_spec_id: 101, bom_variant_id: 201, spec_name: '袋装', pricing_mode: 'tier_template', tier_pricing_rule_id: 7 },
  { parent_product_id: 10, product_id: 10, product_name: '商品 A', bom_id: 5, bom_version_id: 50, bom_spec_id: 102, bom_variant_id: 202, spec_name: '袋装', pricing_mode: 'pricing_rule', pricing_rule_id: 8 },
  { parent_product_id: 20, product_id: 20, product_name: '商品 B', bom_spec_id: 103, bom_variant_id: 203, pricing_mode: 'fixed_price' },
]

test('candidate identity deduplicates tiers but never merges same-name specifications', () => {
  const options = trial.priceTemplateTrialCandidates([rows[0], { ...rows[0], tier_pricing_rule_id: 9 }, rows[1], rows[2]])
  assert.equal(options.length, 3)
  assert.deepEqual(options[0].ruleIDs, [7, 9])
  assert.notEqual(options[0].key, options[1].key)
  assert.deepEqual(trial.priceTemplateTrialCandidates([]), [])
  assert.equal(trial.selectPriceTemplateTrialCandidate(options, '', 8).key, options[1].key)
  assert.equal(trial.selectPriceTemplateTrialCandidate(options, options[0].key, 8).key, options[0].key)
  assert.equal(trial.selectPriceTemplateTrialCandidate(options, 'removed', 77).key, options[0].key)
})

test('draft payload uses selected spec and customer without changing source rows', () => {
  const source = structuredClone(rows)
  const candidate = trial.priceTemplateTrialCandidates(rows)[1]
  const form = { id: 7, name: '未保存', margin_rate: 0, tax_mode: 'none', rounding_mode: 'yuan', other_cost_rows: [] }
  const payload = trial.priceTemplateTrialPayload(candidate, form, 90)
  assert.equal(payload.product_id, 10)
  assert.equal(payload.bom_spec_id, 102)
  assert.equal(payload.bom_variant_id, 202)
  assert.equal(payload.customer_id, 90)
  assert.equal(payload.pricing_rule_id, 7)
  assert.equal(payload.pricing_rule_draft.margin_rate, 0)
  assert.equal(payload.pricing_rule_draft.calculation_json.tax_mode, 'none')
  assert.deepEqual(payload.pricing_rule_draft.calculation_json.other_costs, {})
  assert.equal(payload.pricing_rule_draft.rounding_mode, 'yuan')
  assert.equal('overrides' in payload, false)
  assert.deepEqual(rows, source)
  assert.equal(trial.priceTemplateTrialPayload(null, form, 90), null)
  assert.equal(trial.priceTemplateTrialPayload(trial.priceTemplateTrialCandidates(rows)[2], form, 90).pricing_rule_id, 7)
})

function deferred() {
  let resolve, reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

test('scheduler debounces, invalidates immediately and ignores stale success or failure', async () => {
  const timers = new Map()
  let counter = 0
  const calls = []
  const states = []
  const runner = trial.createPriceTemplateTrialRunner({
    request: (payload) => { const d = deferred(); calls.push({ ...d, payload }); return d.promise },
    onState: (state) => states.push(state),
    setTimer: (fn, ms) => { assert.equal(ms, 250); timers.set(++counter, fn); return counter },
    clearTimer: (id) => timers.delete(id),
  })
  runner.schedule({ v: 1 })
  runner.schedule({ v: 2 })
  assert.equal(timers.size, 1)
  const run = [...timers.values()][0](); timers.clear()
  assert.equal(calls.length, 1)
  assert.equal(calls[0].payload.v, 2)
  runner.schedule({ v: 3 })
  calls[0].resolve({ final_unit_price: 100 })
  await run
  assert.equal(states.at(-1).result, null)
  const newer = [...timers.values()][0](); timers.clear()
  calls[1].resolve({ final_unit_price: 200 })
  await newer
  assert.equal(states.at(-1).result.final_unit_price, 200)
  runner.schedule({ v: 4 })
  const oldError = [...timers.values()][0](); timers.clear()
  runner.cancel()
  calls[2].reject(new Error('stale'))
  await oldError
  assert.equal(states.at(-1).error, '')
  assert.equal(states.at(-1).result, null)
  runner.schedule({ v: 5 })
  runner.cancel()
  assert.equal(timers.size, 0)
})

test('scheduler exposes request errors and allows a fresh retry', async () => {
  let callback, count = 0, state
  const runner = trial.createPriceTemplateTrialRunner({
    request: async () => { if (++count === 1) throw new Error('BOM 成本缺失'); return { final_unit_price: 80 } },
    onState: (next) => { state = next },
    setTimer: (fn) => { callback = fn; return 1 }, clearTimer: () => {},
  })
  runner.schedule({})
  await callback()
  assert.equal(state.error, 'BOM 成本缺失')
  assert.equal(state.result, null)
  runner.schedule({})
  await callback()
  assert.equal(state.result.final_unit_price, 80)
})
