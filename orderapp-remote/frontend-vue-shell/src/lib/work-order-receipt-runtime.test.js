import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h, reactive } from 'vue'
import { renderToString } from '@vue/server-renderer'

let sequence = 0
const preview = () => ({ work_order_id: 64, reported_qty: 14, received_qty: 0, planned_qty: 1, output_unit: '袋', version: 'v1', package_spec_label: '227g袋装', target_warehouse_name: '成品仓', correction_cards: [{ id: 9 }], materials: [{ reservation_id: 2, name: '熟豆', key: 'material:2:0:0', qty: .227, consumed_qty: 0, unit: 'kg', warehouse: 'wip' }] })
async function setup(options = {}) {
  const calls = [], emitted = [], key = `receiptTest${++sequence}`
  globalThis[key] = {
    apiGet: async () => { if (options.warehouseError) throw new Error('仓库不可用'); return [{ code: 'wip', name: 'WIP在制仓' }] },
    apiSend: async (url, request) => {
      calls.push({ url, body: structuredClone(request.body) })
      if (url.endsWith('/preview')) return { rows: [options.row || preview()] }
      return options.submit ? options.submit(request.body) : { rows: [{ work_order_id: 64, succeeded: true, completion: { output_qty: 1, output_unit: '袋', stock_entry_ids: [3] } }] }
    },
  }
  const file = new URL('../components/WorkOrderReceiptForm.vue', import.meta.url)
  const { descriptor } = parse(readFileSync(file, 'utf8'))
  let code = compileScript(descriptor, { id: key }).content
  code = code.replace(/import \{ apiGet, apiSend \} from ['"][^'"]+['"]/, `const { apiGet, apiSend } = globalThis.${key}`)
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(path, file).href : import.meta.resolve(path))}`)
  const Component = (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
  delete globalThis[key]
  let state
  await renderToString(createSSRApp({ setup() { state = Component.setup(reactive({ workOrderId: 64, workOrderNo: 'WO-116' }), { expose() {}, emit: (...args) => emitted.push(args) }); return () => h('div') } }))
  await state.load()
  return { state, calls, emitted }
}
function fill(state, quantity = 1) { state.draft.value.actual_output_qty = quantity; state.draft.value.reason = '现场确认'; state.suggestMaterials() }

test('opening inline receipt only previews; old 14 and plan 1 never become actual output', async () => {
  const { state, calls } = await setup()
  assert.equal(state.draft.value.actual_output_qty, '')
  assert.equal(calls.length, 1)
  await state.submit()
  assert.match(state.error.value, /现场核实/)
  assert.equal(calls.length, 1)
  fill(state, 2)
  assert.equal(state.draft.value.materials[0].qty, .454)
  state.draft.value.materials[0].qty = .5; state.draft.value.materials[0].edited = true
  fill(state, 3); assert.equal(state.draft.value.materials[0].qty, .5)
})
test('one inline submit posts selected work order, version, actual quantity and documents', async () => {
  const { state, calls, emitted } = await setup()
  fill(state); await state.submit()
  assert.equal(calls.length, 2)
  const item = calls[1].body.items[0]
  assert.equal(item.work_order_id, 64); assert.equal(item.actual_output_qty, 1); assert.equal(item.expected_version, 'v1')
  assert.equal(item.materials[0].qty, .227)
  assert.ok(item.request_id)
  assert.equal(emitted.find(item => item[0] === 'posted')[1].completion.stock_entry_ids[0], 3)
})
test('lost response retains exact request and locks changes until safe replay succeeds', async () => {
  let tries = 0
  const { state, calls, emitted } = await setup({ submit: async () => { if (++tries === 1) throw new Error('network lost'); return { rows: [{ work_order_id: 64, succeeded: true, completion: { stock_entry_ids: [3] } }] } } })
  fill(state); await state.submit()
  assert.equal(state.uncertain.value, true)
  assert.match(state.error.value, /重试本次入库/)
  fill(state, 9); await state.submit()
  assert.deepEqual(calls[1].body, calls[2].body)
  assert.equal(state.uncertain.value, false)
  assert.equal(emitted.filter(item => item[0] === 'posted').length, 1)
})
test('warehouses, freeze, integer and multiple-source totals block unsafe submission', async () => {
  for (const options of [{ warehouseError: true }, { row: { ...preview(), review_blocked_reason: '质量冻结' } }]) {
    const { state, calls } = await setup(options); fill(state); await state.submit(); assert.equal(calls.length, 1)
  }
  const { state, calls } = await setup({ row: { ...preview(), correction_cards: [{ id: 1 }, { id: 2 }] } })
  fill(state, 1.5); await state.submit(); assert.match(state.error.value, /整数/)
  fill(state, 3); state.draft.value.card_quantities = { 1: 1, 2: 1 }; await state.submit(); assert.match(state.error.value, /合计/)
  state.draft.value.card_quantities[2] = 2; await state.submit(); assert.equal(calls.length, 2)
})
