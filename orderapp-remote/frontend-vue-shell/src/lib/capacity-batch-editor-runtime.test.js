import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h, reactive } from 'vue'
import { renderToString } from '@vue/server-renderer'

let sequence = 0
async function setupEditor(props, apiSend) {
  const file = new URL('../components/CapacityBatchEditor.vue', import.meta.url)
  const { descriptor } = parse(readFileSync(file, 'utf8'))
  const key = `capacityPresetTest${++sequence}`
  globalThis[key] = { apiSend }
  let code = compileScript(descriptor, { id: key }).content.replace(/import \{ apiSend \} from ['"][^'"]+['"]/, `const { apiSend } = globalThis.${key}`)
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
  const Component = (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
  let state; const events = []
  await renderToString(createSSRApp({ setup() { state = Component.setup(reactive(props), { expose() {}, emit(...args) { events.push(args) } }); return () => h('div') } }))
  return { state, events }
}
const capacity = { id: 7, workstation_id: 2, batch_size_qty: 15, batch_size_unit: 'kg', standard_minutes: 18, hourly_rate: 102 }
function editorProps() {
  const split = { ...capacity, workstation_capacity_id: 7, custom_batch_size_qty: 18.7, batch_size_qty: 18.7, batch_input: true, planned_batch_count: 2, planned_qty: 37.4, batch_count_source: 'manual' }
  return { split, rows: [split], item: { planned_g: 45113 }, capacities: [capacity], disabled: false, canSavePreset: true }
}

test('saving a custom preset sends only the size and keeps the current row unchanged', async () => {
  const props = editorProps(); const before = JSON.stringify(props.split); const calls = []
  const savedCapacity = { ...capacity, id: 8, batch_size_qty: 18.7 }
  const { state, events } = await setupEditor(props, async (url, options) => { calls.push({ url, options }); return { capacity: savedCapacity, created: true } })
  await state.savePreset()
  assert.deepEqual(calls, [{ url: '/api/manufacturing-workstation-capacities/7/batch-presets', options: { method: 'POST', body: { batch_size_qty: 18.7 } } }])
  assert.equal(JSON.stringify(props.split), before)
  assert.deepEqual(events, [['preset-saved', savedCapacity]])
  assert.match(state.presetMessage.value, /已保存/)
})

test('no permission, decline and server errors never prevent using the custom arrangement', async () => {
  const props = editorProps(); props.canSavePreset = false
  let requests = 0
  const { state, events } = await setupEditor(props, async () => { requests++; throw new Error('保存失败，请重试') })
  await state.savePreset(); assert.equal(requests, 0)
  props.canSavePreset = true
  state.declinePreset(); assert.equal(state.showPresetPrompt.value, false)
  await state.savePreset(); assert.equal(requests, 1)
  assert.match(state.presetError.value, /保存失败/)
  assert.equal(props.split.custom_batch_size_qty, 18.7)
  assert.equal(events.length, 0)
})

test('batch input is manual and changing custom size retains the selected batch count', async () => {
  const props = editorProps(); const { state, events } = await setupEditor(props, async () => {})
  state.changeBatch('2')
  const edited = events.at(-1)[1]
  assert.equal(edited.planned_qty, 37.4)
  assert.equal(edited.batch_count_source, 'manual')
  state.changeCustom('18.5')
  assert.equal(events.at(-1)[1].planned_batch_count, 2)
  assert.equal(events.at(-1)[1].planned_qty, 37)
  state.useRecommendation()
  assert.equal(events.at(-1)[1].planned_batch_count, 3)
})

test('preset prompt appears only after a changed valid quantity is committed', async () => {
  const props = editorProps(); props.split.custom_batch_size_qty = null; props.split.batch_size_qty = 15
  const { state, events } = await setupEditor(props, async () => {})
  state.changeMode('custom'); Object.assign(props.split, events.at(-1)[1])
  state.commitCustom()
  assert.equal(state.showPresetPrompt.value, false)
  state.changeCustom('18.7'); Object.assign(props.split, events.at(-1)[1])
  assert.equal(state.showPresetPrompt.value, false)
  state.commitCustom()
  assert.equal(state.showPresetPrompt.value, true)
  state.declinePreset()
  assert.equal(state.showPresetPrompt.value, false)
  state.changeCustom(''); Object.assign(props.split, events.at(-1)[1]); state.commitCustom()
  assert.equal(state.showPresetPrompt.value, false)
})

test('answered preset choices are remembered for every normalized quantity in the row session', async () => {
  const props = editorProps()
  const { state, events } = await setupEditor(props, async () => {})
  for (const value of ['18.1', '18.7']) {
    state.changeCustom(value); Object.assign(props.split, events.at(-1)[1]); state.commitCustom()
    assert.equal(state.showPresetPrompt.value, true)
    state.declinePreset()
  }
  state.changeCustom('18.100'); Object.assign(props.split, events.at(-1)[1]); state.commitCustom()
  assert.equal(state.showPresetPrompt.value, false)
})
