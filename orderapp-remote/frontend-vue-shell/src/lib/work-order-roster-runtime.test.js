import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h, reactive } from 'vue'
import { renderToString } from '@vue/server-renderer'
const ops = [
  { id: 21, workstation_id: 1, workstation: '烘焙工位', operation: '烘焙', status: 'pending' },
  { id: 22, workstation_id: 4, workstation: '色选工位', operation: '色选', status: 'pending' },
]
const order = (id, items = ops, status = 'released') => ({ id, status, work_order_no: `WO-QA-${id}`, operation_summary_json: JSON.stringify(items) })
const roster = () => ({ date: '2026-10-23', roster_version: 1, assignments: [
  { workstation_id: 1, workstation: '烘焙工位', employee_id: 2, employee_name: '测试负责人', source: 'primary', unattended: false, handover_task_ids: [] },
  { workstation_id: 4, workstation: '色选工位', employee_id: 3, employee_name: '测试替补', source: 'backup', unattended: false, handover_task_ids: [] },
] })
let sequence = 0
async function setup(getRoster = async () => roster()) {
  const calls = [], events = [], rows = [order(19), order(18, [ops[0]])]
  globalThis.window = { location: { origin: 'https://example.test' }, dispatchEvent: event => events.push(event.detail) }
  globalThis.CustomEvent = class { constructor(type, options) { this.type = type; this.detail = options.detail } }
  const key = `workOrderRosterTest${++sequence}`
  globalThis[key] = {
    apiGet: async value => {
      const path = new URL(value, window.location.origin).pathname
      calls.push(String(value))
      if (path === '/api/production-roster/today') return getRoster()
      if (path === '/api/produce/work-orders') return { rows }
      if (path === '/api/manufacturing-workstation-capacities') return { rows: [] }
      throw new Error(`Unexpected read: ${value}`)
    },
    apiSend: async () => { throw new Error('Roster status must not write business data') },
  }
  const file = new URL('../views/WorkOrdersView.vue', import.meta.url)
  const { descriptor } = parse(readFileSync(file, 'utf8'))
  let code = compileScript(descriptor, { id: key }).content
  code = code.replace(/import \{ apiGet, apiSend \} from ['"][^'"]+['"]/, `const { apiGet, apiSend } = globalThis.${key}`)
  code = code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, 'const $1 = {}')
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
  const Component = (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
  delete globalThis[key]
  let state
  await renderToString(createSSRApp({ setup() { state = Component.setup(reactive({ embedded: true, viewParams: {} }), { expose() {} }); return () => h('div') } }))
  await state.load()
  return { state, calls, events, rows }
}

test('saved workstation roster resolves both reported orders without task assignment fields', async () => {
  const { state, calls, rows, events } = await setup()
  assert.ok(calls.includes('/api/production-roster/today?scope=all'))
  for (const row of rows) {
    assert.equal(state.workOrderTodo(row).title, '去烘焙')
    assert.match(state.workOrderTodo(row).detail, /测试负责人/)
    assert.equal(state.primaryActionLabel(row), '去烘焙')
  }
  state.openPrimaryAction(rows[0])
  assert.equal(events.at(-1).key, 'workstationView')
  assert.equal(events.at(-1).params.work_order_id, 19)
  assert.equal(events.at(-1).returnNavigation.key, 'workOrders')
})

test('unattended or invalid overrides still open the required operation and explain roster setup', async () => {
  const data = roster(); data.assignments[0] = { ...data.assignments[0], employee_id: 0, unattended: true, reason: '主负责人休息，无替补' }
  data.assignments[1].override_invalid = true
  const { state, rows, events } = await setup(async () => data)
  assert.equal(state.workOrderTodo(rows[0]).title, '去烘焙')
  assert.match(state.workOrderTodo(rows[0]).detail, /主负责人休息/)
  assert.equal(state.primaryActionLabel(rows[0]), '去烘焙')
  state.openPrimaryAction(rows[0])
  assert.equal(events.at(-1).key, 'workstationView')
  assert.equal(events.at(-1).returnNavigation.params.work_order_id, 19)
})

test('unsaved week cannot use old assignment names as proof of current duty', async () => {
  const { state } = await setup(async () => ({ ...roster(), roster_version: 0 }))
  const row = order(1, [{ ...ops[0], assigned_to: '旧负责人' }])
  assert.equal(state.workOrderTodo(row).title, '去烘焙')
  assert.match(state.workOrderTodo(row).detail, /本周排班尚未保存/)
  assert.equal(state.primaryActionLabel(row), '去烘焙')
})

test('roster failure keeps orders visible and retry refreshes the actual roster', async () => {
  let fail = true
  const { state, rows } = await setup(async () => { if (fail) throw new Error('没有读取排班权限'); return roster() })
  assert.equal(state.rows.value.length, 2)
  assert.equal(state.workOrderTodo(rows[0]).title, '去烘焙')
  assert.match(state.workOrderTodo(rows[0]).detail, /没有读取排班权限/)
  assert.equal(state.primaryActionLabel(rows[0]), '去烘焙')
  state.openPrimaryAction(rows[0])
  assert.equal(state.workOrderTodo(rows[0]).title, '去烘焙')
  fail = false; await state.load()
})

test('closed tasks need no new staff and an empty order is not ready for receipt', async () => {
  const { state } = await setup()
  assert.equal(state.workOrderTodo(order(1, ops, 'completed')).title, '查看记录')
  assert.equal(state.workOrderTodo(order(1, ops, 'cancelled')).title, '已取消')
  assert.equal(state.workOrderTodo(order(1, ops.map(row => ({ ...row, status: 'completed' })))).title, '确认数量并入库')
  assert.equal(state.workOrderTodo(order(1, [])).title, '等待生成工序任务')
})

test('running tasks retain execution state and handover matches only this task', async () => {
  const data = roster(); data.assignments[0].handover_task_ids = [99]
  const { state } = await setup(async () => data)
  const running = order(1, [{ ...ops[0], status: 'running' }], 'running')
  assert.equal(state.workOrderTodo(running).title, '继续烘焙')
  assert.equal(state.workOrderTodo(order(1, [{ ...ops[0], status: 'paused' }])).title, '继续烘焙')
  data.assignments[0].handover_task_ids = [21]; await state.load()
  assert.equal(state.workOrderTodo(running).title, '待工位交接')
  assert.equal(state.primaryActionLabel(running), '待工位交接')
})

test('legacy station names resolve only without ID; duplicate tasks count one station', async () => {
  const { state } = await setup()
  assert.equal(state.workOrderTodo(order(1, [{ ...ops[0], workstation_id: 0 }])).title, '去烘焙')
  assert.equal(state.workOrderTodo(order(1, [{ ...ops[0], workstation_id: 999 }])).title, '去烘焙')
  const data = roster(); data.assignments[0].employee_id = 0; data.assignments[0].unattended = true
  const other = await setup(async () => data)
  assert.equal(other.state.workOrderTodo(order(1, [ops[0], { ...ops[0], id: 23 }])).title, '去烘焙')
})

test('an older load cannot overwrite a newly saved roster', async () => {
  let resolveOld, count = 0
  const { state, rows } = await setup(async () => {
    count++
    if (count === 2) return new Promise(resolve => { resolveOld = resolve })
    return roster()
  })
  const oldLoad = state.load(); await state.load()
  resolveOld({ ...roster(), roster_version: 0 }); await oldLoad
  assert.equal(state.workOrderTodo(rows[0]).title, '去烘焙')
})
