import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import { createSSRApp, h, reactive } from 'vue'
import { renderToString } from '@vue/server-renderer'
let sequence = 0
async function harness(startShortages = [], overrideGroup = null, savedStorage = null) {
  const file = new URL('../components/WorkstationBatchIssueDrawer.vue', import.meta.url)
  const { descriptor } = parse(readFileSync(file, 'utf8'), { filename: file.pathname })
  const id = `batch-issue-test-${++sequence}`
  const script = compileScript(descriptor, { id })
  const template = compileTemplate({ id, source: descriptor.template.content, filename: file.pathname, compilerOptions: { bindingMetadata: script.bindings } })
  assert.deepEqual(template.errors, [])
  const writes = [], events = [], navigation = []
  const values = new Map()
  const sessionStorage = savedStorage || {getItem:key=>values.get(key)||null,setItem:(key,value)=>values.set(key,value),removeItem:key=>values.delete(key)}
  globalThis.window = { sessionStorage, dispatchEvent: event => navigation.push(event.detail), addEventListener(){}, removeEventListener(){} }
  globalThis.CustomEvent = class { constructor(type, options) { this.type = type; this.detail = options.detail } }
  const group = overrideGroup || { key: 'g', version: 'gv', name: '测试包装袋', owner_name: '工厂自有', unit: '个', quantity_basis: 'count', from_warehouse: 'raw_materials', required: 20, wip: 5, remaining: 15, available: 15, max_quantity: 100, allocations: [{ job_card_id: 1, work_order_no: 'WO-1', batch_code: 'B-1', remaining: 15, available: 15, extra_available: 85 }] }
  const mocks = { fail: false, apiGet: async () => ({ rows: [{ code: 'packaging', name: '包材仓' }] }), apiSend: async (url, options) => {
    writes.push({ url, body: JSON.parse(JSON.stringify(options.body)) })
    if (url.endsWith('/preview')) return { job_card_ids: [1], version: 'v', groups: [group], blocked: [] }
    if (mocks.fail) { if (mocks.fail instanceof Error) throw mocks.fail; throw new Error('网络暂时中断') }
    return { request_id:options.body.request_id, success_count: 1, failed_count: 0, groups: [{ key: 'g', name: group.name, unit: '个', quantity_basis: 'count', status: 'submitted', quantity: 30, extra: 15, remaining: 0, documents: [{ id: 9, entry_no: 'SE-9' }] }] }
  } }
  globalThis[id] = mocks
  let code = script.content.replace(/import \{ apiGet, apiSend \} from ['"][^'"]+['"]/, `const { apiGet, apiSend } = globalThis[${JSON.stringify(id)}]`)
  code = code.replace('export default', 'const component =') + '\n' + template.code.replace('export function render', 'function render') + '\ncomponent.render = render; export default component;'
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(path, file).href : import.meta.resolve(path))}`)
  const View = (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
  delete globalThis[id]
  const props = reactive({ taskIds: [1], workstation: '包装台', returnParams: { scope: 'all' }, startShortages })
  let state
  await renderToString(createSSRApp({ setup() { state = View.setup(props, { expose() {}, emit: (...args) => events.push(args) }); return () => h('div') } }))
  const html = () => renderToString(createSSRApp({ ...View, setup: () => state }, props))
  await state.refresh()
  return { state, writes, events, mocks, html, navigation, sessionStorage }
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
test('replenishment shows start deficit separately from reservation and permits the extra needed amount', async () => {
  const group = { key:'g',version:'gv',name:'水洗 A',unit:'kg',quantity_basis:'weight',from_warehouse:'raw_materials',required:45000,wip:40171,remaining:4829,available:4829,max_quantity:10000,allocations:[{job_card_id:1,work_order_no:'WO-1',remaining:4829,available:4829,extra_available:5171}] }
  const { state, html, writes } = await harness([{key:'g',name:'水洗 A', unit:'kg', shortage_qty:5.302}], group)
  assert.match(await html(), /本批开工缺口/)
  assert.match(await html(), /5\.302/)
  assert.equal(state.quantities.value.g,4.829)
  state.quantities.value.g=5.302
  assert.equal(state.invalid.value,false)
  await state.submit()
  assert.equal(writes.at(-1).body.groups[0].quantity,5.302)
})
test('uncertain result keeps the drawer open and blocks navigation until the same request is resolved', async () => {
  const { state, mocks, html, navigation } = await harness()
  mocks.fail = true
  await state.submit()
  const markup = await html()
  assert.match(markup, /disabled[^>]*>关闭/)
  state.openDocument({id: 9})
  assert.equal(navigation.length, 0)
  assert.match(markup, /不可关闭/)
})
test('reload restores the exact pending request and a subsequent 4xx cannot discard it', async () => {
  const first=await harness();first.mocks.fail=true;await first.state.submit()
  const sent=first.writes.at(-1).body
  const second=await harness([],null,first.sessionStorage)
  second.state.restorePending()
  assert.equal(second.state.uncertain.value,true)
  second.mocks.fail=Object.assign(new Error('账号或请求不匹配'),{status:400})
  await second.state.submit()
  assert.deepEqual(second.writes.at(-1).body,sent)
  assert.ok(first.sessionStorage.getItem('kferp:workstation-batch-issue:pending:v1'))
  second.mocks.fail=false;await second.state.submit()
  assert.deepEqual(second.writes.at(-1).body,sent)
  assert.equal(first.sessionStorage.getItem('kferp:workstation-batch-issue:pending:v1'),null)
})
test('an initial 400 remains uncertain because the endpoint also returns 400 for commit errors', async () => {
  const { state, mocks, sessionStorage } = await harness()
  mocks.fail=Object.assign(new Error('请求被拒绝'),{status:400})
  await state.submit()
  assert.equal(state.uncertain.value,true)
  assert.ok(sessionStorage.getItem('kferp:workstation-batch-issue:pending:v1'))
})
test('an initial timeout keeps the original request ID and retries it exactly',async()=>{
  const {state,mocks,writes,sessionStorage}=await harness()
  mocks.fail=Object.assign(new Error('超时'),{status:408})
  await state.submit()
  const first=writes.at(-1).body
  assert.equal(state.uncertain.value,true)
  assert.ok(sessionStorage.getItem('kferp:workstation-batch-issue:pending:v1'))
  mocks.fail=false;await state.submit()
  assert.deepEqual(writes.at(-1).body,first)
  assert.equal(sessionStorage.getItem('kferp:workstation-batch-issue:pending:v1'),null)
})
test('storage failure prevents sending and an existing request cannot be overwritten',async()=>{
  const denied={getItem(){throw Error('denied')},setItem(){throw Error('denied')}}
  const blocked=await harness([],null,denied);await blocked.state.submit()
  assert.equal(blocked.writes.filter(row=>row.url.endsWith('/submit')).length,0)
  const first=await harness();first.mocks.fail=true;await first.state.submit()
  const second=await harness([],null,first.sessionStorage);await second.state.submit()
  assert.equal(second.writes.filter(row=>row.url.endsWith('/submit')).length,0)
})
