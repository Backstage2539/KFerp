import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import {
  navItemsWithProductionBadges,
  productionCompletionMetrics,
  productionCompletionOutputQty,
  productionTaskActionEndpoint,
  productionTaskActionErrorMessage,
  productionTaskActionNeedsRefresh,
  materialReadinessState,
  productionTopNavItems,
  stockOperationContextParams,
  taskQuantityLines,
  workstationCanOpenIssue,
  workstationGroupMaterialSuggestions,
  workstationGroupOutputAllocations,
  workstationGroupCompletionValidation,
  workstationVisibleActions,
  workstationTaskSections,
  workstationPackagingGroups,
  workstationPackagingMaterialIdentity,
  workstationFocusDisposition,
  workstationTaskRequiresSchedule,
} from './production-workstation.js'

test('production top nav lists high-frequency views before legacy production pages', () => {
  assert.deepEqual(productionTopNavItems.map((item) => item.key), [
    'workstationView',
    'productionFlow',
    'produceRunning',
    'produceLogs',
  ])
  assert.equal(productionTopNavItems[0].label, '工位视图')
})

test('same frozen packaging work is grouped across customers and BOM variants with once-counted materials', () => {
  const tasks = [10, 6, 4, 1].map((qty, index) => ({
    packaging_group_key: 'pack-454-owner0-finished',
    job_card_id: index + 1,
    work_order_id: index + 1,
    work_order_no: `WO-${index + 1}`,
    product_name: '小菠萝', operation: '包装', workstation: '包装台',
    planned_output_inventory_qty: qty,
    customer_id: [454, 14, 73, 464][index],
    bom_variant_id: [491, 48, 48, 48][index],
    material_readiness: [
      { material_id: 127, material_name: '小菠萝-半成品', unit: 'g', required_g: qty * 454, wip_available_g: qty * 454, shortage_g: 0 },
      { material_id: 100, material_name: '454g袋', unit: 'pic', required_units: qty, wip_available_units: 0, shortage_units: qty },
    ],
  }))
  const groups = workstationPackagingGroups(tasks)
  assert.equal(groups.length, 1)
  assert.deepEqual(groups[0].tasks.map(row => row.job_card_id), [1, 2, 3, 4])
  assert.equal(groups[0].planned_output_qty, 21)
  assert.deepEqual(groups[0].materials.map(row => [row.material_id, row.required_g, row.wip_available_g, row.shortage_g, row.required_units, row.wip_available_units, row.shortage_units]), [
    [127, 9534, 9534, 0, 0, 0, 0],
    [100, 0, 0, 0, 21, 0, 21],
  ])
})

test('singleton packaging cards stay on their existing individual workflow', () => {
  const groups = workstationPackagingGroups([
    { packaging_group_key: 'one', job_card_id: 1, operation: '包装', status: 'pending', work_order_status: 'released' },
    { packaging_group_key: 'one', job_card_id: 2, operation: '包装', status: 'completed', work_order_status: 'partially_completed' },
  ])
  assert.equal(groups.length, 0)
})

test('completed cards are excluded from a still-open packaging group', () => {
  const groups = workstationPackagingGroups([
    { packaging_group_key: 'same', job_card_id: 1, operation: '包装', status: 'pending', work_order_status: 'released' },
    { packaging_group_key: 'same', job_card_id: 2, operation: '包装', status: 'completed', work_order_status: 'partially_completed' },
    { packaging_group_key: 'same', job_card_id: 3, operation: '包装', status: 'pending', work_order_status: 'released' },
  ])
  assert.equal(groups.length, 1)
  assert.deepEqual(groups[0].tasks.map(row => row.job_card_id), [1, 3])
})

test('packaging material summary preserves frozen component specification identity', () => {
  const groups = workstationPackagingGroups([
    { packaging_group_key: 'same-key', job_card_id: 1, operation: '包装', status: 'pending', work_order_status: 'released', planned_output_inventory_qty: 1, material_readiness: [{ material_id: 100, bom_spec_id: 227, spec_g: 227, unit: 'pic', required_units: 1 }] },
    { packaging_group_key: 'same-key', job_card_id: 2, operation: '包装', status: 'pending', work_order_status: 'released', planned_output_inventory_qty: 1, material_readiness: [{ material_id: 100, bom_spec_id: 454, spec_g: 454, unit: 'pic', required_units: 1 }] },
  ])
  assert.equal(groups.length, 1)
  assert.equal(groups[0].materials.length, 2)
  assert.notEqual(groups[0].materials[0].identity, groups[0].materials[1].identity)
  assert.equal(workstationPackagingMaterialIdentity({ material_id: 100, bom_spec_id: 227, unit: 'pic' }), 'material:100:227:0:pic:0')
})

test('group completion suggestions follow partial output quantity and work-order allocation', () => {
  const items = [10, 6, 4, 1].map((remaining_qty, index) => ({ job_card_id: index + 1, remaining_qty, work_order_no: `WO-${index + 1}`, unit: '袋' }))
  const preview = {
    items,
    materials: [
      { key: 'bean', unit: 'g', sources: items.map((item, index) => ({ job_card_id: item.job_card_id, unit: 'g', qty: [4540, 2724, 1816, 454][index] })) },
      { key: 'bag', unit: 'pic', sources: items.map((item, index) => ({ job_card_id: item.job_card_id, unit: 'pic', qty: [10, 6, 4, 1][index] })) },
    ],
  }
  assert.deepEqual(workstationGroupOutputAllocations(items, 6).map(row => row.qty), [6, 0, 0, 0])
  assert.deepEqual(workstationGroupMaterialSuggestions(preview, 6), [{ key: 'bean', qty: 2724 }, { key: 'bag', qty: 6 }])
  assert.deepEqual(workstationGroupMaterialSuggestions(preview, 21), [{ key: 'bean', qty: 9534 }, { key: 'bag', qty: 21 }])
})

test('group completion blocks output beyond available stock and material edits beyond availability', () => {
  const materials = [{ key: 'bags', unit: 'pic', qty: 21, available_qty: 15 }]
  assert.match(workstationGroupCompletionValidation({ outputQty: 16, maxOutputQty: 15, remainingQty: 21, materials }), /最多支持完成 15/)
  assert.match(workstationGroupCompletionValidation({ outputQty: 15, maxOutputQty: 15, remainingQty: 21, materials }), /超过可用库存/)
  assert.equal(workstationGroupCompletionValidation({ outputQty: 15, maxOutputQty: 15, remainingQty: 21, materials: [{ ...materials[0], qty: 15 }] }), '')
})

test('workstation exposes one material action and one completion form per compatible packaging group', () => {
  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')
  const form = readFileSync(new URL('../components/WorkstationPackagingCompletionForm.vue', import.meta.url), 'utf8')
  assert.match(source, /workstationPackagingGroups\(section\.tasks\)/)
  assert.match(source, /@click="openBatchIssueForTasks\(group\.tasks\)"\>补齐物料/)
  assert.match(source, /!groupHasShortage\(group\)/)
  assert.match(source, /@click="openGroupCompletion\(group\)"\>完成并入库/)
  assert.match(source, /来源仓可补/)
  assert.match(source, /apiSend\('\/api\/production\/workstation\/material-issue\/preview'/)
  assert.match(source, /WorkstationPackagingCompletionForm[\s\S]*:task-ids="completionGroup\.ids"/)
  assert.doesNotMatch(source, /撤回并重新安排/)
  assert.doesNotMatch(source, /@click="openStockAction\(task, 'issue'\)"/)
  assert.match(form, /\/api\/production\/workstation\/completion\/preview/)
  assert.match(form, /\/api\/production\/workstation\/completion\/submit/)
  assert.match(form, /本次实际完成袋数/)
  assert.match(form, /来源工单数量分配预览/)
  assert.match(form, /workstationGroupMaterialSuggestions/)
  assert.match(form, /@input="touchMaterial\(index\)"/)
})

test('workstation task sections answer current task, next task, and blocked reason', () => {
  const sections = workstationTaskSections([
    {
      job_card_id: 91,
      work_order_no: 'WO-001',
      product_name: '桂花乌龙',
      operation: '包装',
      workstation: '包装工位A',
      status: 'running',
      priority: 9,
      planned_start_at: '2026-06-13 09:00',
      blocking_reason: '',
    },
    {
      job_card_id: 92,
      work_order_no: 'WO-001',
      product_name: '桂花乌龙',
      operation: '贴标',
      workstation: '包装工位A',
      status: 'pending',
      priority: 6,
      planned_start_at: '2026-06-13 09:45',
      blocking_reason: '',
    },
    {
      job_card_id: 93,
      work_order_no: 'WO-002',
      product_name: '日晒瑰夏',
      operation: '烘焙',
      workstation: '布勒 18kg',
      status: 'paused',
      priority: 8,
      blocking_reason: '缺少生豆领料',
    },
  ])

  assert.equal(sections.length, 2)
  const pack = sections.find((section) => section.workstation === '包装工位A')
  assert.equal(pack.currentTask.job_card_id, 91)
  assert.equal(pack.nextTask.job_card_id, 92)
  assert.equal(pack.blockingReason, '')

  const roast = sections.find((section) => section.workstation === '布勒 18kg')
  assert.equal(roast.currentTask.job_card_id, 93)
  assert.equal(roast.nextTask, null)
  assert.equal(roast.blockingReason, '缺少生豆领料')
})

test('workstation sections ignore tasks from completed or cancelled work orders', () => {
  const sections = workstationTaskSections([
    { job_card_id: 1, work_order_id: 1, work_order_status: 'released', workstation: '智烘', status: 'pending' },
    { job_card_id: 2, work_order_id: 2, work_order_status: 'cancelled', workstation: '智烘', status: 'pending' },
    { job_card_id: 3, work_order_id: 3, work_order_status: 'completed', workstation: '智烘', status: 'pending' },
  ])
  assert.equal(sections.length, 1)
  assert.deepEqual(sections[0].tasks.map((row) => row.job_card_id), [1])
})

test('workstation quantity and material state use frozen batch units', () => {
  assert.deepEqual(taskQuantityLines({
    operation: '咖啡烘焙+除石',
    spec_g: 0,
    planned_input_inventory_qty: 3,
    inventory_unit: 'kg',
    planned_output_inventory_qty: 4.994,
    planned_input_qty: 3000,
    operation_total_input_qty: 6000,
  }), ['本批投料 3 kg', '目标产出 2.497 kg'])
  assert.deepEqual(taskQuantityLines({
    operation: '包装',
    spec_g: 227,
    planned_output_inventory_qty: 20,
    inventory_unit: '袋',
    planned_input_qty: 10,
    operation_total_input_qty: 20,
  }), ['规格 227g', '本批生产 10 袋'])
  assert.equal(materialReadinessState({ material_readiness: [{ shortage_g: 0 }], material_data_complete: true }), 'ready')
  assert.equal(materialReadinessState({ material_readiness: [{ shortage_g: 1 }], material_data_complete: true }), 'shortage')
  assert.equal(materialReadinessState({ material_readiness: [], material_data_complete: false }), 'unknown')
  assert.equal(workstationCanOpenIssue({ stock_action: 'issue' }), true)
  assert.equal(workstationCanOpenIssue({ stock_action: 'none', material_readiness_state: 'ready' }), false)
})

test('production task action endpoints stay aligned with workstation action buttons', () => {
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'start'), '/api/job-cards/91/start')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'pause'), '/api/job-cards/91/pause')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'resume'), '/api/job-cards/91/resume')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'complete'), '/api/job-cards/91/complete')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'report_exception'), '/api/production/workstation/tasks/91/exception')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'material_call'), '/api/production/workstation/tasks/91/material-call')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 0 }, 'start'), '')
  assert.equal(productionTaskActionEndpoint({ job_card_id: 91 }, 'unknown'), '')
})

test('production task action failures are explained in Chinese without changing the server state locally', () => {
  assert.equal(
    productionTaskActionErrorMessage(new Error('invalid job card status transition: running -> resume'), 'resume'),
    '当前工序状态不允许继续，请刷新后按最新状态操作',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('job card not found'), 'pause'),
    '工序卡不存在或已失效，请刷新后重试',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('invalid job card action resume from running'), 'resume'),
    '当前工序状态不允许继续，请刷新后按最新状态操作',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('work order must be running before job card start'), 'start'),
    '本任务暂不能开始，请刷新任务状态后重试',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('permission denied'), 'complete'),
    '当前账号没有执行此操作的权限，请联系管理员',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('work order must be released'), 'start'),
    '工单必须先下达后才能执行工序',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('work order is not running'), 'resume'),
    '本任务暂不能开始，请刷新任务状态后重试',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('actual input and output quantity invalid'), 'complete'),
    '实际投入或实际产出数量不正确，请检查后重试',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('实际产出和成品件数只能填写一项'), 'complete'),
    '实际产出和成品件数只能填写一项',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('本任务物料不足：榇巧拼配，需求 875g，WIP 可用 874g，缺口 1g'), 'start'),
    '本任务物料不足：榇巧拼配，需求 875g，WIP 可用 874g，缺口 1g',
  )
  assert.equal(
    productionTaskActionErrorMessage(Object.assign(new Error('database unavailable'), { status: 400, code: 'JC-START-91-20260913123045' }), 'start'),
    '开始失败（错误编号：JC-START-91-20260913123045），请稍后重试；如持续失败请联系管理员',
  )
  assert.equal(
    productionTaskActionErrorMessage(new Error('network unavailable'), 'pause'),
    '暂停失败（错误编号：NETWORK），请稍后重试；如持续失败请联系管理员',
  )
})

test('stale task status conflicts refresh the workstation queue after showing the server reason', () => {
  assert.equal(productionTaskActionNeedsRefresh(new Error('invalid job card action start from running')), true)
  assert.equal(productionTaskActionNeedsRefresh(new Error('invalid job card status transition: running -> resume')), true)
  assert.equal(productionTaskActionNeedsRefresh(new Error('permission denied')), false)

  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')
  assert.match(source, /productionTaskActionNeedsRefresh\(err\)[\s\S]*await load\(\)/)
})

test('production top nav renders count badges for high-frequency production views', () => {
  const items = navItemsWithProductionBadges(productionTopNavItems, {
    productionOverview: { pending: 2, blocked: 1, running: 3 },
    workstationView: { pending: 2, blocked: 1, running: 3 },
    produceRunning: { running: 3 },
  })

  assert.deepEqual(items.slice(0, 4).map((item) => ({
    key: item.key,
    badge: item.badge,
  })), [
    { key: 'workstationView', badge: '待2 阻1 中3' },
    { key: 'productionFlow', badge: '' },
    { key: 'produceRunning', badge: '待0 阻0 中3' },
    { key: 'produceLogs', badge: '' },
  ])
})

test('stock operation context carries WIP prefill parameters from production tasks', () => {
  assert.deepEqual(stockOperationContextParams({
    work_order_id: 88,
    job_card_id: 91,
    running_item_id: 99,
    material_id: 10,
    shortage_g: 600,
  }), {
    tab: 'stockEntries',
    action: 'issue',
    return_source: 'work_order',
    work_order_id: 88,
    job_card_id: 91,
    running_item_id: 99,
    material_id: 10,
    shortage_g: 600,
  })
})

test('workstation view lets wide task rows scroll and single-station filters fill the work area', () => {
  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')

  assert.match(source, /:class="\{\s*'single-station-grid': singleStationLayout\s*\}"/)
  assert.match(source, /const singleStationLayout = computed\(\(\) => visibleSections\.value\.length === 1\)/)
  assert.match(source, /\.station-grid\.single-station-grid\s*\{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\);/s)
  assert.match(source, /\.task-table\s*\{[^}]*overflow-x:\s*auto;[^}]*overflow-y:\s*hidden;[^}]*-webkit-overflow-scrolling:\s*touch;[^}]*overscroll-behavior-inline:\s*contain;/s)
  assert.match(source, /\.task-row\s*\{[^}]*min-width:\s*610px;/s)
  assert.match(source, /@media \(max-width:\s*760px\)\s*\{[\s\S]*\.task-row\s*\{[^}]*grid-template-columns:\s*1fr;[^}]*min-width:\s*0;/)
})

test('workstation action buttons reveal their forms inside the clicked task row', () => {
  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')

  assert.match(source, /<OperationCompletionForm v-if="isFinishPanelForTask\(task\)"/)
  assert.match(source, /class="task-action-panel"/)
  assert.match(source, /function isIssuePanelForTask\(task\)/)
  assert.match(source, /function isFinishPanelForTask\(task\)/)
  assert.match(source, /function openIssue\(task, mode\) \{[\s\S]*finishPanel\.open = false/)
  assert.match(source, /function openFinishPanel\(task\) \{[\s\S]*issue\.open = false/)
  assert.doesNotMatch(source, /<section v-if="issue\.open" class="panel action-panel">/)
  assert.doesNotMatch(source, /<section v-if="finishPanel\.open" class="panel action-panel">/)
  assert.doesNotMatch(source, /isFinishPanelForTask\(task\) &&[^\n]*automatic_completion/)
})

test('roasting task completion is inline and requires only output quantity', () => {
 const source=readFileSync(new URL('../views/WorkstationView.vue',import.meta.url),'utf8')
 const form=readFileSync(new URL('../components/OperationCompletionForm.vue',import.meta.url),'utf8')
 assert.match(source, /OperationCompletionForm v-if="isFinishPanelForTask\(task\)"/)
 assert.match(source, /function isSimplifiedTask\(task\).*烘焙.*除石/)
 assert.doesNotMatch(source, /submitFinishPanel|finishRunningProduction|partial_finish/)
 assert.match(form,/completionQuantityLabel\(preview\.output_unit\)/)
 assert.ok(form.includes('冻结的本批投料和 BOM 自动计算用料'))
 for(const forbidden of ['本单用料','实际投入','实际工时','保存进度','呼叫补料','报异常','核对旧单'])assert.doesNotMatch(form,new RegExp(forbidden))
 assert.match(form,/expected_version|completionPayload/)
})

test('ordinary grinding cards use the simplified completion and work-order actions', () => {
 const source=readFileSync(new URL('../views/WorkstationView.vue',import.meta.url),'utf8')
 assert.match(source,/function isSimplifiedTask\(task\).*研磨.*磨粉/)
 const actions=source.slice(source.indexOf('<template v-if="isSimplifiedTask(task)">'),source.indexOf('<template v-else>',source.indexOf('<template v-if="isSimplifiedTask(task)">')))
 assert.match(actions,/工序完成/)
 assert.match(actions,/查看工单/)
 for(const forbidden of ['物料操作','呼叫补料','报异常','暂停','继续'])assert.doesNotMatch(actions,new RegExp(forbidden))
})

test('workstation renders quantity, ready material state and one responsible employee only', () => {
  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')
  assert.match(source, /taskQuantityLines\(task\)/)
  assert.match(source, /material-state-ready/)
  assert.match(source, /workstationCanOpenIssue\(task\)/)
  assert.doesNotMatch(source, /协作：/)
  assert.doesNotMatch(source, /task\.collaborators/)
})

test('operation, schedule and task staff editors expose one responsible employee', () => {
  for (const relativePath of [
    '../components/ProductionStaffFields.vue',
    '../components/ProductionTaskStaffEditor.vue',
    '../views/ManufacturingOperationsView.vue',
    '../views/ProductionScheduleView.vue',
  ]) {
    const source = readFileSync(new URL(relativePath, import.meta.url), 'utf8')
    assert.doesNotMatch(source, /默认协作人员|协作人员|协作：/)
  }
})

test('stock entry preview keeps production identity and renders no-action state without an empty editable form', () => {
  const source = readFileSync(new URL('../views/StockEntriesView.vue', import.meta.url), 'utf8')
  assert.match(source, /preview\.availability/)
  assert.match(source, /preview\.message/)
  assert.match(source, /stock-preview-state/)
  assert.match(source, /return_navigation/)
})

test('a task missing from the selected queue is resolved from its real status and roster state', () => {
  assert.equal(workstationFocusDisposition({ status: 'pending', canViewAll: true }), 'all_scope')
  assert.equal(workstationFocusDisposition({ status: 'pending', rosterStatus: 'unattended', canViewAll: true }), 'unattended')
  assert.equal(workstationFocusDisposition({ status: 'running', canViewAll: false }), 'outside_scope')
  assert.equal(workstationFocusDisposition({ status: 'completed' }), 'record')
  assert.equal(workstationFocusDisposition({ status: 'cancelled' }), 'record')
  assert.equal(workstationFocusDisposition({}), 'unknown')
  assert.equal(workstationTaskRequiresSchedule({ status: 'pending', roster_status: 'unattended' }), true)
  assert.equal(workstationTaskRequiresSchedule({ status: 'pending' }, { employee_id: 0 }), true)
  assert.equal(workstationTaskRequiresSchedule({ status: 'pending' }, { employee_id: 6 }), false)
  assert.equal(workstationTaskRequiresSchedule({ status: 'running' }), true)
  assert.equal(workstationTaskRequiresSchedule({ status: 'running' }, { employee_id: 6 }), false)

  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')

  assert.match(source, /focus === 'workstation_task'/)
  assert.match(source, /requestedJobCardID/)
  assert.match(source, /selectedWorkstation\.value = matchedTask\.workstation/)
  assert.match(source, /:class="\{ focused: isRequestedTask\(task\) \}"/)
  assert.match(source, /load\(\{ focusRequested: true \}\)/)
  assert.match(source, /options\?\.focusRequested === true \|\| String\(props\.viewParams\?\.focus \|\| ''\) === 'workstation_task'/)
  assert.match(source, /workstationFocusDisposition/)
  assert.match(source, /已从全厂视图加载目标任务/)
  assert.doesNotMatch(source, /该任务已结束或已移出待执行队列/)
  assert.match(source, /目标工序当前状态/)
  assert.match(source, /今日该工位无人值班，请先完成排班/)
  assert.match(source, /:disabled="Boolean\(busyKey\) \|\| loading \|\| isTaskUnattended\(task\)"/)
  assert.match(source, /function isTaskUnattended\(task\)/)
  assert.match(source, /workstationTaskRequiresSchedule\(task, rosterAssignmentForTask\(task\)\)/)
  assert.match(source, /function completionReturnParams\(task\)/)
  assert.doesNotMatch(source, /^\s*focusRequestedTask\(\)\s*$/m)
})

test('roasting completion shows the refreshed published BOM version and returns to the focused workstation', () => {
  const source = readFileSync(new URL('../components/OperationCompletionForm.vue', import.meta.url), 'utf8')
  assert.match(source, /preview\.bom_refresh_pending/)
  assert.match(source, /preview\.effective_bom_version_no/)
  assert.match(source, /preview\.original_bom_version_no/)
  assert.match(source, /returnNavigation:\{key:'workstationView'/)
  assert.match(source, /params:\{\.\.\.props\.returnParams\}/)
})

test('active workstation tasks require a real employee assignment before completion', () => {
  assert.equal(workstationTaskRequiresSchedule({ status: 'pending', roster_status: 'unattended' }), true)
  assert.equal(workstationTaskRequiresSchedule({ status: 'pending' }, { employee_id: 0 }), true)
  assert.equal(workstationTaskRequiresSchedule({ status: 'pending' }, { employee_id: 6 }), false)
  assert.equal(workstationTaskRequiresSchedule({ status: 'running' }), true)
  assert.equal(workstationTaskRequiresSchedule({ status: 'running' }, { employee_id: 6 }), false)
})

test('work-order drawer never routes into a completed operation card', () => {
  const source = readFileSync(new URL('../components/ProductionExecutionHubDrawer.vue', import.meta.url), 'utf8')
  assert.match(source, /enterFirstWorkstation\(\)[\s\S]*find\(\(item\) => !\['completed', 'cancelled'\]\.includes\(item\.status\)\)/)
  assert.match(source, /已完成，查看工序记录/)
  assert.match(source, /work_order_id: Number\(row\.work_order_id \|\| props\.workOrderId \|\| 0\)/)
  assert.match(source, /station_id: Number\(row\.workstation_id \|\| 0\)/)
})

test('completed or cancelled work orders are read-only in the shared detail drawer', () => {
  const source = readFileSync(new URL('../components/ProductionExecutionHubDrawer.vue', import.meta.url), 'utf8')
  assert.match(source, /const workOrderClosed = computed/)
  assert.match(source, /v-if="workOrderClosed" class="muted"/)
  assert.match(source, /该工单已结束，不再开放人员调整、领料或执行动作/)
})

test('workstation completion keeps input and output in the frozen inventory unit', () => {
  assert.equal(productionCompletionOutputQty({
    actualOutputQty: 0,
    finishedUnits: 14,
    inventoryQtyPerSalesUnit: 0.454,
  }), 6.356)
  assert.equal(productionCompletionOutputQty({
    actualOutputQty: 6.2,
    finishedUnits: 0,
    inventoryQtyPerSalesUnit: 0.454,
  }), 6.2)
  assert.throws(
    () => productionCompletionOutputQty({
      actualOutputQty: 6.2,
      finishedUnits: 14,
      inventoryQtyPerSalesUnit: 0.454,
    }),
    /实际产出和成品件数只能填写一项/,
  )
  assert.throws(
    () => productionCompletionOutputQty({
      actualOutputQty: 0,
      finishedUnits: 0,
      inventoryQtyPerSalesUnit: 0.454,
    }),
    /请填写实际产出或成品件数/,
  )
  assert.throws(
    () => productionCompletionOutputQty({
      actualOutputQty: 0,
      finishedUnits: 14,
      inventoryQtyPerSalesUnit: 0,
    }),
    /缺少库存单位换算/,
  )
  assert.throws(
    () => productionCompletionOutputQty({
      actualOutputQty: 0,
      finishedUnits: 14.5,
      inventoryQtyPerSalesUnit: 0.454,
    }),
    /成品件数必须为整数/,
  )
})

test('workstation completion freezes inventory-unit interpretation and hides legacy partial finish', () => {
  assert.deepEqual(productionCompletionMetrics({
    inventoryUnit: 'Kg',
    leftoverQty: 0.2,
    note: '包装抽检',
    warehouse: 'finished_goods',
    finishedUnits: 14,
  }), {
    quantity_basis: 'inventory_unit',
    inventory_unit: 'Kg',
    leftover_qty: 0.2,
    note: '包装抽检',
    finished_units: 14,
  })
  assert.deepEqual(workstationVisibleActions({
    available_actions: ['pause', 'complete', 'partial_finish', 'report_exception', 'material_call'],
  }), ['pause', 'complete', 'report_exception', 'material_call'])
})

test('workstation distinguishes a submitted command whose state refresh failed', () => {
  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')

  assert.match(source, /const refreshed = await load\(\)/)
  assert.match(source, /if \(!refreshed\)/)
  assert.match(source, /已提交，但状态刷新失败，请手动刷新/)
  assert.match(source, /return true/)
  assert.match(source, /return false/)
  assert.doesNotMatch(source, /await load\(\)\s*\n\s*message\.value = `\$\{actionLabel\(action\)\}成功`/)
})

test('employee workstation defaults to personal roster and only administrators see the factory switch', () => {
  const appSource = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')
  const source = readFileSync(new URL('../views/WorkstationView.vue', import.meta.url), 'utf8')

  assert.match(appSource, /:actor="currentActor"/)
  assert.match(source, /actorHasFullViewAccess\(props\.actor\)/)
  assert.match(source, /v-if="canViewAll" class="scope-switch"/)
  assert.match(source, /scope \|\| 'mine'/)
  for (const marker of ['本周排班尚未保存', '今天尚未排班', '今天休息', '今天已上班，暂未负责工位']) {
    assert.ok(source.includes(marker), `missing personal roster state: ${marker}`)
  }
  assert.ok(source.includes('今日工位暂无任务'), 'factory overview must not imply that the administrator owns an unattended workstation')
})
