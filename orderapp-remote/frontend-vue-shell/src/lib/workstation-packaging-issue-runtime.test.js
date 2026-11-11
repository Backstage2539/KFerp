import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

async function harness() {
  const file = new URL('../views/WorkstationView.vue', import.meta.url)
  const { descriptor } = parse(readFileSync(file, 'utf8'))
  const script = compileScript(descriptor, { id: 'packaging-issue' })
  const template = compileTemplate({ id: 'packaging-issue', source: descriptor.template.content, filename: file.pathname, compilerOptions: { bindingMetadata: script.bindings } })
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
  const tree = async () => {
    let node
    await renderToString(createSSRApp({ ...View, setup: () => state, render(...args) { node = View.render.apply(this, args); return node } }, props))
    return node
  }
  return { state, tree }
}

function findNode(node, predicate) {
  if (!node || typeof node !== 'object') return null
  if (predicate(node)) return node
  for (const child of Array.isArray(node.children) ? node.children : []) {
    const found = findNode(child, predicate)
    if (found) return found
  }
  return null
}

function packagingTasks() {
  return [20, 16, 8].map((qty, index) => ({
    job_card_id: index + 101, work_order_id: index + 201, work_order_no: `WO-PACK-${index + 1}`,
    product_name: '曜石2.0', operation: '包装', workstation: '包装台', status: 'pending',
    automatic_completion: true, material_consumption_policy: 'bom_start_v1', stock_action: '',
    packaging_group_key: 'packaging-44', planned_input_qty: qty, operation_total_input_qty: qty,
    planned_output_inventory_qty: qty, inventory_unit: '袋', roster_employee_id: 1, assigned_to: '测试员',
    material_data_complete: true, material_readiness_state: 'shortage',
    material_readiness: [{ material_id: 99, material_name: '咖啡豆袋-1kg', unit: 'pic', required_units: qty, wip_available_units: 0, shortage_units: qty }],
    available_actions: ['start', 'report_exception'],
  }))
}

test('packaging refill opens the drawer for all simplified group tasks despite empty stock actions', async () => {
  const { state, tree } = await harness()
  const tasks = packagingTasks()
  state.overview.value = { tasks }
  const button = findNode(await tree(), node => node.type === 'button' && node.children === '补齐物料')
  assert.ok(button, 'the grouped card renders its refill button')
  assert.equal(button.props.disabled, false)
  button.props.onClick()
  assert.equal(state.batchIssue.open, true, 'clicking refill must open the existing drawer')
  assert.deepEqual(state.batchIssue.ids, [101, 102, 103])
  const drawer = findNode(await tree(), node => Array.isArray(node.props?.['task-ids']) && node.props?.workstation === '包装台')
  assert.deepEqual(drawer?.props['task-ids'], [101, 102, 103])
})

test('packaging refill reports an empty selection and preserves an already open issue', async () => {
  const { state } = await harness()
  state.openPackagingGroupIssue({ tasks: [{ job_card_id: 0 }, { job_card_id: -2 }, { job_card_id: 1.5 }] })
  assert.equal(state.batchIssue.open, false)
  assert.match(state.error.value, /任务.*刷新/)
  state.openPackagingGroupIssue({ tasks: [...packagingTasks(), packagingTasks()[0]] })
  assert.deepEqual(state.batchIssue.ids, [101, 102, 103])
  assert.equal(state.error.value, '')
  state.openPackagingGroupIssue({ tasks: [{ job_card_id: 999 }] })
  assert.deepEqual(state.batchIssue.ids, [101, 102, 103])
})

test('ordinary merged issue keeps its existing stock-action eligibility filter', async () => {
  const { state } = await harness()
  state.openBatchIssueForTasks([{ job_card_id: 1, stock_action: 'issue' }, { job_card_id: 2, stock_action: '' }, { job_card_id: 3, stock_action: 'unavailable' }])
  assert.equal(state.batchIssue.open, true)
  assert.deepEqual(state.batchIssue.ids, [1])
})

test('non-Chinese piece tasks render the packaging group and open all sources', async () => {
  const { state, tree } = await harness()
  state.overview.value = { tasks: packagingTasks().map(task => ({...task,operation:'assembly',reporting_type:'piece'})) }
  const button = findNode(await tree(), node => node.type === 'button' && node.children === '补齐物料')
  assert.ok(button);button.props.onClick();assert.deepEqual(state.batchIssue.ids,[101,102,103])
})

test('all configured operation types use the simplified workstation flow regardless of name', async () => {
  const { state } = await harness()
  for (const mode of ['batch','incoming','piece']) assert.equal(state.isSimplifiedTask({operation:'process-A',reporting_type:mode,automatic_completion:false}),true)
  assert.equal(state.isSimplifiedTask({operation:'包装',reporting_type:'unknown',automatic_completion:true}),false)
  assert.equal(state.isSimplifiedTask({operation:'包装',reporting_type:'',automatic_completion:false}),true)
  const wrongType=packagingTasks().map(task=>({...task,reporting_type:'incoming'}))
  assert.deepEqual(state.packagingGroups({tasks:wrongType}),[])
})

test('multiple cards from one work order render individually instead of a duplicated packaging group',async()=>{
 const {state,tree}=await harness()
 const tasks=packagingTasks().map((task,index)=>({...task,operation:'assembly',reporting_type:'piece',work_order_id:index<2?201:203}))
 state.overview.value={tasks}
 assert.deepEqual(state.packagingGroups({tasks}),[])
 assert.equal(state.ungroupedTasks({tasks}).length,3)
 assert.equal(findNode(await tree(),node=>node.type==='button'&&node.children==='补齐物料'),null)
})

test('single or multiple weight batches expand through the task title without starting it', async () => {
  const { state } = await harness()
  const base = {job_card_id:42,work_order_id:3,reporting_type:'batch',material_consumption_policy:'bom_start_v1',status:'pending'}
  assert.equal(state.isInlineBatchTask({...base,planned_batch_count:1}),true)
  assert.equal(state.isInlineBatchTask({...base,planned_batch_count:4}),true)
  assert.equal(state.isInlineBatchTask({...base,reporting_type:'piece'}),false)
  assert.equal(state.isInlineBatchTask({...base,reporting_type:'incoming'}),false)
  state.toggleFinishPanel(base);assert.equal(state.finishPanel.open,true)
  state.toggleFinishPanel(base);assert.equal(state.finishPanel.open,false)
})

test('task header completes the expanded batch form',async()=>{
 const {state,tree}=await harness()
 const task={job_card_id:222,work_order_id:333,work_order_no:'WO-TEST',product_name:'豆',operation:'烘焙',workstation:'包装台',status:'running',roster_employee_id:1,reporting_type:'batch',material_consumption_policy:'bom_start_v1',planned_batch_count:1}
 state.overview.value={tasks:[task]};state.openFinishPanel(task);let count=0;state.finishForm.value=[{completeAll(){count++}}]
 const button=findNode(await tree(),node=>node.type==='button'&&node.children==='完成所有批次')
 assert.ok(button);assert.equal(button.props.disabled,false);button.props.onClick();assert.equal(count,1)
})
