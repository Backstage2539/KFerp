import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

test('a single running legacy packaging card uses completion instead of legacy material and exception actions', async () => {
  const file = new URL('../views/WorkstationView.vue', import.meta.url)
  const { descriptor } = parse(readFileSync(file, 'utf8'))
  const script = compileScript(descriptor, { id: 'legacy-packaging' })
  const template = compileTemplate({ id: 'legacy-packaging', source: descriptor.template.content, filename: file.pathname, compilerOptions: { bindingMetadata: script.bindings } })
  assert.deepEqual(template.errors, [])
  let code = script.content.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, "const $1 = { render: () => null }")
  code = code.replace(/import \{ fetchProductionWorkstationOverview, runProductionTaskAction \} from ['"][^'"]+['"]/, "const fetchProductionWorkstationOverview = async () => ({ tasks: [] }); const runProductionTaskAction = () => { throw new Error('unexpected write') }")
  code = code.replace(/import \{ apiGet, apiSend \} from ['"][^'"]+['"]/, "const apiGet = async () => ({}); const apiSend = () => { throw new Error('unexpected write') }")
  code = code.replace('export default', 'const component =') + '\n' + template.code.replace('export function render', 'function render') + '\ncomponent.render = render; export default component;'
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
  const View = (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
  let state
  const props = { viewParams: { scope: 'all' }, actor: null }
  await renderToString(createSSRApp({ setup() { state = View.setup(props, { expose() {} }); return () => h('div') } }))
  state.selectedWorkstationValue.value = '包装台'
  state.overview.value = { tasks: [{ job_card_id: 16, work_order_id: 14, work_order_no: 'WO-LEGACY-PACK', product_name: '曜石2.0', operation: '包装', workstation: '包装台', status: 'running', status_label: '执行中', automatic_completion: false, packaging_group_key: 'single-pack', planned_input_qty: 14, planned_output_inventory_qty: 4, inventory_unit: '袋', roster_employee_id: 1, assigned_to: '测试员', available_actions: ['pause', 'complete', 'report_exception', 'material_call'] }] }
  const html = await renderToString(createSSRApp({ ...View, setup: () => state }, props))
  assert.match(html, /工序完成/)
  assert.match(html, /查看工单/)
  assert.doesNotMatch(html, /物料操作|报异常|呼叫补料|完成本工序/)
})
