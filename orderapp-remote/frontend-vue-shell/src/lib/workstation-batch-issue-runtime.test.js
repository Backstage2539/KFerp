import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import { createSSRApp, h, reactive } from 'vue'
import { renderToString } from '@vue/server-renderer'
let sequence = 0
async function harness() {
  const file = new URL('../components/WorkstationBatchIssueDrawer.vue', import.meta.url)
  const { descriptor } = parse(readFileSync(file, 'utf8'), { filename: file.pathname })
  const id = `batch-issue-test-${++sequence}`
  const script = compileScript(descriptor, { id })
  const template = compileTemplate({ id, source: descriptor.template.content, filename: file.pathname, compilerOptions: { bindingMetadata: script.bindings } })
  assert.deepEqual(template.errors, [])
  const writes = [], events = [], navigation = []
  globalThis.window = { dispatchEvent: event => navigation.push(event.detail) }
  globalThis.CustomEvent = class { constructor(type, options) { this.type = type; this.detail = options.detail } }
  const group = { key: 'g', version: 'gv', name: '测试包装袋', owner_name: '工厂自有', unit: '个', quantity_basis: 'count', from_warehouse: 'raw_materials', required: 20, wip: 5, remaining: 15, available: 15, max_quantity: 100, allocations: [{ job_card_id: 1, work_order_no: 'WO-1', batch_code: 'B-1', remaining: 15, available: 15, extra_available: 85 }] }
  const mocks = { fail: false, apiSend: async (url, options) => {
    writes.push({ url, body: JSON.parse(JSON.stringify(options.body)) })
    if (url.endsWith('/preview')) return { job_card_ids: [1], version: 'v', groups: [group], blocked: [] }
    if (mocks.fail) throw new Error('网络暂时中断')
    return { success_count: 1, failed_count: 0, groups: [{ key: 'g', name: group.name, unit: '个', quantity_basis: 'count', status: 'submitted', quantity: 30, extra: 15, remaining: 0, documents: [{ id: 9, entry_no: 'SE-9' }] }] }
  } }
  globalThis[id] = mocks
  let code = script.content.replace(/import \{ apiSend \} from ['"][^'"]+['"]/, `const { apiSend } = globalThis[${JSON.stringify(id)}]`)
  code = code.replace('export default', 'const component =') + '\n' + template.code.replace('export function render', 'function render') + '\ncomponent.render = render; export default component;'
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(path, file).href : import.meta.resolve(path))}`)
  const View = (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
  delete globalThis[id]
  const props = reactive({ taskIds: [1], workstation: '包装台', returnParams: { scope: 'all' } })
  let state
  await renderToString(createSSRApp({ setup() { state = View.setup(props, { expose() {}, emit: (...args) => events.push(args) }); return () => h('div') } }))
  const html = () => renderToString(createSSRApp({ ...View, setup: () => state }, props))
  await state.refresh()
  return { state, writes, events, mocks, html, navigation }
}
test('drawer defaults to remaining demand, supports edits, results and correct document navigation', async () => {
  const { state, writes, events, html, navigation } = await harness()
  assert.equal(state.quantities.value.g, 15)
  assert.deepEqual(state.selected.value, ['g'])
  state.quantities.value.g = 30
  assert.match(await html(), /额外 WIP/)
  await state.submit()
  assert.equal(writes.at(-1).body.groups[0].quantity, 30)
  assert.equal(events.at(-1)[0], 'posted')
  assert.match(await html(), /已领 1 组/)
  state.openDocument({ id: 9 })
  assert.equal(navigation[0].params.stock_entry_id, 9)
  assert.equal(navigation[0].returnNavigation.params.workstation, '包装台')
})
test('uncertain network result locks edits and retries the identical request without new quantity', async () => {
  const { state, writes, mocks } = await harness()
  mocks.fail = true
  await state.submit()
  const first = writes.at(-1).body
  assert.equal(state.uncertain.value, true)
  assert.equal(state.locked.value, true)
  mocks.fail = false
  await state.submit()
  assert.deepEqual(writes.at(-1).body, first)
  assert.equal(state.uncertain.value, false)
  assert.equal(state.result.value.success_count, 1)
})
