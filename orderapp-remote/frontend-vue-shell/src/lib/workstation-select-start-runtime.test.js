import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
let harnessSequence = 0

async function harness(mocks = {}) {
  const file = new URL('../views/WorkstationView.vue', import.meta.url)
  const { descriptor } = parse(readFileSync(file, 'utf8'))
  const script = compileScript(descriptor, { id: 'packaging-issue' })
  const template = compileTemplate({ id: 'packaging-issue', source: descriptor.template.content, filename: file.pathname, compilerOptions: { bindingMetadata: script.bindings } })
  assert.deepEqual(template.errors, [])
  let code = script.content.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, 'const $1 = { name: "$1", render: () => null }')
  code = code.replace(/import \{ fetchProductionWorkstationOverview, runProductionTaskAction \} from ['"][^'"]+['"]/, 'const { fetchProductionWorkstationOverview, runProductionTaskAction } = globalThis.__workstationStartMocks')
  globalThis.__workstationStartMocks = { apiGet: mocks.apiGet || (async () => ({})), apiSend: mocks.apiSend || (() => { throw new Error('unexpected write') }), fetchProductionWorkstationOverview: mocks.fetchOverview || (async () => ({ tasks: [] })), runProductionTaskAction: () => { throw new Error('unexpected write') } }
  code = code.replace(/import \{ apiGet, apiSend \} from ['"][^'"]+['"]/, 'const { apiGet, apiSend } = globalThis.__workstationStartMocks')
  code = code.replace('export default', 'const component =') + '\n' + template.code.replace('export function render', 'function render') + '\ncomponent.render = render; export default component;'
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
  code += `\n// harness ${++harnessSequence}`
  const View = (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
  let state
  const props = { viewParams: { scope: 'all' }, actor: mocks.actor || null }
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

function textContent(node){if(typeof node==='string')return node;if(!node||typeof node!=='object')return '';return Array.isArray(node.children)?node.children.map(textContent).join(' '):String(node.children||'')}

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

test('expanding a pending batch shows its planned batches without a start material form', async () => {
 const {state,tree}=await harness()
 const task={...packagingTasks()[0],job_card_id:505,operation:'咖啡烘焙+除石',reporting_type:'batch',planned_batch_count:2}
 state.overview.value={tasks:[task]}
 state.toggleFinishPanel(task)
 const node=await tree()
 assert.ok(findNode(node,n=>n.type?.name==='ExecutionBatchPreview'),'planned batch preview should remain visible')
 assert.ok(!findNode(node,n=>n.type?.name==='OperationStartForm'),'obsolete start material section must not render')
})

test('start button posts the frozen planned materials and opens replenishment when short', async () => {
 const entries=new Map(), oldWindow=globalThis.window
 globalThis.window={sessionStorage:{getItem:k=>entries.get(k)??null,setItem:(k,v)=>entries.set(k,v),removeItem:k=>entries.delete(k)}}
 const preview={version:'v1',input_qty:30,input_unit:'kg',recipe_ratio_pct:100,materials:[{key:'bean',name:'生豆',qty:30,unit:'kg',ratio_pct:100,warehouse:'wip',available_qty:30}]}
 const posts=[]
 try {
  const {state,tree}=await harness({actor:{employee_id:7},apiGet:async()=>structuredClone(preview),apiSend:async(path,{body})=>{posts.push({path,body});return {ok:true}}})
  const task={...packagingTasks()[0],job_card_id:505,operation:'咖啡烘焙+除石',reporting_type:'batch',planned_batch_count:2}
  state.overview.value={tasks:[task]};state.toggleFinishPanel(task)
  const start=findNode(await tree(),n=>n.type==='button'&&n.children==='开工')
  assert.ok(start);await start.props.onClick();assert.equal(posts.length,1,state.error.value);assert.equal(posts[0].body.actual_input_qty,30)
  state.overview.value={tasks:[task]};preview.materials[0].available_qty=29.759
  await state.startTask(task);assert.equal(posts.length,1);assert.equal(state.batchIssue.open,true);assert.equal(state.batchIssue.startShortages[0].shortage_qty,0.241)
 } finally {globalThis.window=oldWindow}
})

test('successful start reports a failed state refresh without resubmitting', async () => {
 const entries=new Map(),oldWindow=globalThis.window
 globalThis.window={sessionStorage:{getItem:k=>entries.get(k)??null,setItem:(k,v)=>entries.set(k,v),removeItem:k=>entries.delete(k)}}
 let writes=0
 try {
  const {state}=await harness({actor:{employee_id:7},apiGet:async()=>({version:'v',input_qty:1,input_unit:'kg',recipe_ratio_pct:100,materials:[{key:'b',name:'豆',unit:'kg',qty:1,ratio_pct:100,warehouse:'wip',available_qty:1}]}),apiSend:async()=>{writes++;return {ok:true}},fetchOverview:async()=>{throw Error('网络中断')}})
  await state.startTask({...packagingTasks()[0],job_card_id:506})
  assert.equal(writes,1);assert.match(state.error.value,/开工已成功.*刷新失败/);assert.equal(entries.size,0)
 } finally {globalThis.window=oldWindow}
})

test('all pending packaging orders remain rows with cross-spec selection and frozen labels', async () => {
 const {state,tree}=await harness()
 const tasks=packagingTasks().map((t,i)=>({...t,packaging_group_key:`different-${i}`,spec_label:['1KG袋装','2.5kg袋装','227g袋装'][i],related_orders:[{order_id:i+1,order_item_id:i+11,order_no:`SO-${i+1}`,customer_name:`客户${i+1}`,operation_notes:[{operation_name:'包装',note:`要求${i+1}`}]}]}))
 state.overview.value={tasks}
 let node=await tree()
 for(const value of ['1KG袋装','2.5kg袋装','227g袋装','客户2','要求3']) assert.ok(textContent(node).includes(value),value)
 assert.ok(!textContent(node).includes('0g袋装'))
 const selectAll=findNode(node,n=>n.type==='input'&&n.props?.['aria-label']==='全选当前工位可开工工单')
 assert.ok(selectAll,'pending group rows must support select-all')
 selectAll.props.onChange()
 assert.deepEqual(state.selectedStartTasks.value,[101,102,103])
 node=await tree()
 const start=findNode(node,n=>n.type==='button'&&n.children==='合并开工')
 assert.ok(start);assert.equal(start.props.disabled,false);start.props.onClick()
 assert.deepEqual(state.startGroup.ids,[101,102,103])
 assert.equal(state.startGroup.open,true)
 assert.deepEqual(state.ungroupedTasks({tasks}),tasks)
})

test('batch start selection excludes running or unattended tasks and preserves other stations', async()=>{
 const {state}=await harness()
 const tasks=packagingTasks();tasks[1].status='running';tasks[2].roster_employee_id=0;tasks[2].roster_status='unattended'
 assert.deepEqual(state.startableTasks({tasks}).map(t=>t.job_card_id),[101])
 state.selectedStartTasks.value=[999]
 state.toggleStartSelection({tasks})
 assert.deepEqual(state.selectedStartTasks.value,[999,101])
 state.startGroup.locked=true
 state.toggleStartSelection({tasks})
 assert.deepEqual(state.selectedStartTasks.value,[999,101])
 state.openSelectedStart({workstation:'包装台',tasks:packagingTasks()})
 assert.equal(state.startGroup.open,false)
})

test('packaging summary uses only visible unfinished rows and separates frozen specs and units',async()=>{
 const {state,tree}=await harness()
 const base={...packagingTasks()[0],product_id:8,bom_spec_id:11,bom_variant_id:111,spec_label:'1kg袋装',planned_input_qty:0,operation_total_input_qty:0,actual_output_qty:0}
 const tasks=[
  {...base,job_card_id:1,work_order_id:1,planned_output_inventory_qty:50,planned_start_at:'2026-11-12 08:00'},
  {...base,job_card_id:2,work_order_id:2,planned_output_inventory_qty:30,status:'running',actual_output_qty:5},
  {...base,job_card_id:3,work_order_id:3,spec_label:'2.5kg袋装',bom_spec_id:12,bom_variant_id:112,planned_output_inventory_qty:10},
  {...base,job_card_id:4,work_order_id:4,inventory_unit:'箱',planned_output_inventory_qty:3},
  {...base,job_card_id:5,work_order_id:5,status:'completed',planned_output_inventory_qty:999},
  {...base,job_card_id:6,work_order_id:6,work_order_status:'cancelled',planned_output_inventory_qty:999},
 ]
 const groups=state.packagingSummary({tasks})
 const bags=groups.find(x=>x.spec_label==='1kg袋装'&&x.unit==='袋')
 assert.equal(bags.remaining_qty,75)
 assert.equal(bags.order_count,2)
 assert.equal(bags.unscheduled_count,1)
 assert.equal(groups.length,3)
 assert.equal(state.packagingSummary({tasks:tasks.slice(0,1)})[0].remaining_qty,50)
 state.overview.value={tasks}
 const content=textContent(await tree())
 assert.doesNotMatch(content,/待包装汇总/)
})

test('packaging summary does not merge missing specs or double-count a split work order',async()=>{
 const {state}=await harness()
 const base={...packagingTasks()[0],product_id:8,bom_spec_id:11,bom_variant_id:111,spec_label:'1kg袋装',planned_output_inventory_qty:10,operation_total_input_qty:10}
 const split=state.packagingSummary({tasks:[{...base,job_card_id:1,work_order_id:1,planned_input_qty:6,actual_output_qty:3},{...base,job_card_id:2,work_order_id:1,planned_input_qty:4,actual_output_qty:0}]})
 assert.equal(split[0].remaining_qty,7)
 assert.equal(split[0].order_count,1)
 const unknown=state.packagingSummary({tasks:[{...base,job_card_id:3,work_order_id:3,spec_label:''},{...base,job_card_id:4,work_order_id:4,spec_label:''}]})
 assert.equal(unknown.length,2)
 assert.ok(unknown.every(row=>row.spec_label==='规格未记录'&&row.order_count===1))
 const unavailable=state.packagingSummary({tasks:[{...base,job_card_id:5,work_order_id:5,planned_output_inventory_qty:undefined}]})
 assert.equal(unavailable[0].unknown_quantity_count,1)
})

test('started packaging cards show recorded inputs and preserve resume without obsolete controls', async () => {
 const {state,tree}=await harness()
 const rows=packagingTasks().slice(0,2).map((row,index)=>({...row,status:index?'running':'paused',status_label:index?'执行中':'已暂停',actual_output_qty:0,spec_label:'1KG袋装',material_readiness_state:'ready',material_readiness:[{material_id:99,material_name:'曲奇-半成品',unit:'kg',required_g:50000,issued_g:50000,remaining_required_g:0,wip_available_g:0,shortage_g:0}]}))
 state.overview.value={tasks:rows}
 const node=await tree(), content=textContent(node)
 for(const removed of ['待包装汇总','当前待办','待协同岗位','开工扣料说明','刷新','补料 / 退料 / 撤销','同规格工单完成并入库']) assert.ok(!content.includes(removed), `unexpected ${removed}`)
 assert.ok(content.includes('物料已投入'))
 assert.ok(content.includes('曲奇-半成品'))
 assert.ok(content.includes('50 kg'))
 assert.ok(content.includes('已完成 0 袋'))
 assert.ok(findNode(node,n=>n.type==='button'&&n.children==='恢复'))
 assert.ok(!findNode(node,n=>n.type==='button'&&n.children==='暂停'))
 assert.ok(findNode(node,n=>n.props?.class?.includes?.('task-card')))
})

test('paused and running cards show their execution state independently from blockers', async () => {
 const {state,tree}=await harness()
 const rows=packagingTasks().slice(0,2).map((row,index)=>({...row,status:index?'running':'paused',status_label:index?'待处理':'异常',blocking_reason:index?'前序未完成':'已暂停',is_blocked:true,material_readiness_state:'ready'}))
 state.overview.value={tasks:rows}
 const node=await tree(), content=textContent(node)
 assert.ok(content.includes('已暂停'),'paused state must not become 异常')
 assert.ok(content.includes('执行中'),'running state must not become 待处理')
 assert.ok(content.includes('前序未完成'),'independent blocker must remain visible')
})

test('split packaging task target reflects its own share rather than the full order',async()=>{
 const {state,tree}=await harness()
 state.overview.value={tasks:[{...packagingTasks()[0],job_card_id:901,status:'running',planned_output_inventory_qty:100,operation_total_input_qty:100,planned_input_qty:25,actual_output_qty:7}]}
 const content=textContent(await tree())
 assert.match(content,/目标 25 袋/);assert.match(content,/已完成 7 袋/)
})

test('material details retain readable field names on narrow screens', async () => {
 const {state,tree}=await harness()
 state.overview.value={tasks:[{...packagingTasks()[0],status:'running',material_readiness_state:'ready',material_readiness:[{material_id:99,material_name:'曲奇-半成品',unit:'kg',required_g:50000,issued_g:50000,remaining_required_g:0,wip_available_g:0,shortage_g:0}]}]}
 const node=await tree()
 for(const label of ['需求','已投入','剩余需求','WIP 可用','缺口']) assert.ok(findNode(node,n=>n.props?.['data-label']===label),`missing mobile field label: ${label}`)
})
