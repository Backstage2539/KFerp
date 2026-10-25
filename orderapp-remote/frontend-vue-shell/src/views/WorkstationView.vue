<template>
  <div class="page workstation-view">

    <ProductionReturnLink :source="viewParams.return_navigation" />
    <section v-if="lastGroupResult" class="panel group-result"><strong>合并入库完成：{{ lastGroupResult.completed_qty }} {{ lastGroupResult.output_unit }}</strong><p v-for="item in lastGroupResult.items" :key="item.job_card.id">{{ item.work_order.work_order_no }} · 入库 {{ item.completion?.output_qty || 0 }} {{ item.completion?.output_unit }} · 单据 {{ item.completion?.stock_entry_ids?.join('、') || '无' }}</p><p>{{ lastGroupResult.remaining_qty > 0 ? `剩余 ${lastGroupResult.remaining_qty} 袋继续待办` : '本组任务已完成' }}</p><button class="secondary" @click="lastGroupResult=null">收起</button></section>
    <section v-if="lastAutomaticResult" class="panel"><strong>{{ lastAutomaticResult.message }} · {{ lastAutomaticResult.output_qty }} {{ lastAutomaticResult.output_unit }}</strong><p>关联单据：{{ lastAutomaticResult.stock_entry_ids?.join('、') || '工序交接记录' }}</p><button class="secondary" @click="lastAutomaticResult=null">收起</button></section>
    <section class="toolbar">
      <div>
        <h2>{{ scope === 'mine' ? '我的今日工位' : '全厂工位视图' }}</h2>
        <p>{{ todayRoster.date || overview.date || '今天' }} · {{ visibleSections.length }} 个工位 · {{ visibleTaskCount }} 个任务</p>
      </div>
      <div class="toolbar-actions">
        <div v-if="canViewAll" class="scope-switch"><button type="button" :class="{ active: scope === 'mine' }" @click="switchScope('mine')">我的今日工位</button><button type="button" :class="{ active: scope === 'all' }" @click="switchScope('all')">全厂总览</button></div>
        <label>
          <span>工位</span>
          <select v-model="selectedWorkstation">
            <option value="">全部工位</option>
            <option v-for="section in sections" :key="section.workstation" :value="section.workstation">{{ section.workstation }}</option>
          </select>
        </label>
        <button class="secondary" type="button" @click="openScheduling">查看生产排班</button>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
    </section>

    <div v-if="message" class="notice">{{ message }}</div>
    <div v-if="error" class="error">{{ error }}</div>
    <section v-if="requestedTaskNotice" class="requested-task-notice" role="status">
      <strong>{{ requestedTaskNotice.operation }} · {{ requestedTaskNotice.work_order_no }} · {{ requestedTaskNotice.status_label }}</strong>
      <p>{{ requestedTaskNotice.message }}</p>
      <button v-if="requestedTaskNotice.disposition === 'unattended'" type="button" class="secondary" @click="openScheduling">去安排今日工位</button>
      <button v-if="requestedTaskNotice.work_order_id" type="button" class="secondary" @click="openExecutionHub(requestedTaskNotice, 'job_card')">查看工单</button>
    </section>

    <section class="station-grid" :class="{ 'single-station-grid': singleStationLayout }">
      <article v-for="section in visibleSections" :key="section.workstation" class="station-panel">
        <div class="station-head">
          <div>
            <h3>{{ section.workstation }}</h3>
            <p>{{ loadStatusLabel(stationLoad(section).load_status) }} · 队列 {{ stationLoad(section).queue_count || section.tasks.length }} · 阻塞 {{ stationLoad(section).blocked_count || 0 }} · 预计 {{ stationLoad(section).estimated_minutes || 0 }} 分钟</p>
            <p class="station-owner" :class="{ unattended: !section.assignment?.employee_id }">今日负责人：{{ section.assignment?.employee_name || '无人值班' }} · {{ rosterSourceLabel(section.assignment) }}</p>
          </div>
          <div class="station-head-actions"><span v-if="section.blockingReason" class="blocker">{{ section.blockingReason }}</span><button v-if="section.assignment?.handover_count" class="handover" type="button" :disabled="busyKey !== ''" @click="handoverStation(section)">接手工位（{{ section.assignment.handover_count }}）</button></div>
        </div>

        <div class="answer-grid">
          <div class="answer-block current">
            <span>当前任务 · 现在做</span>
            <strong>{{ taskTitle(section.currentTask) }}</strong>
            <small>{{ taskMeta(section.currentTask) }}</small>
          </div>
          <div class="answer-block next">
            <span>下一件</span>
            <strong>{{ taskTitle(section.nextTask) }}</strong>
            <small>{{ taskMeta(section.nextTask) }}</small>
          </div>
          <div class="answer-block blocked" :class="{ empty: !section.blockingReason }">
            <span>当前待办</span>
            <strong>{{ section.blockingReason || '无阻塞' }}</strong>
            <small>{{ section.blockingReason ? `待协同岗位：${nextHandler(section)}` : '可继续执行' }}</small>
          </div>
        </div>

        <div v-if="!section.tasks.length" class="no-task">{{ scope === 'mine' ? '今日负责此工位，暂无任务' : '今日工位暂无任务' }}</div>
        <button v-else-if="!selectedWorkstation" class="enter-station primary" type="button" @click="selectedWorkstation = section.workstation">进入本工位</button>

        <div v-if="selectedWorkstation && issueTasks(section).length" class="batch-issue-toolbar">
          <span>已选 {{ issueSelectionState(issueTasks(section), selectedIssueTasks).count }} 个任务</span>
          <button type="button" class="primary" :disabled="!issueSelectionState(issueTasks(section), selectedIssueTasks).count || loading" @click="openBatchIssue(section)">合并领料</button>
          <small>多选可领任务后，同料汇总、一次领到 WIP</small>
        </div>
        <div v-if="selectedWorkstation && packagingGroups(section).length" class="packaging-groups">
          <article v-for="group in packagingGroups(section)" :key="group.key" class="packaging-group-card">
            <div class="packaging-group-head"><div><h4>{{ group.product_name }} · {{ group.spec_g }}g袋装 · {{ quantityText(group.remaining_output_qty) }} {{ group.inventory_unit }} · {{ group.tasks.length }}张工单</h4><p>目标仓：{{ warehouseLabel(group.target_warehouse_name || group.tasks[0]?.target_warehouse) }} · 已完成 {{ quantityText(group.completed_output_qty) }} / {{ quantityText(group.planned_output_qty) }} {{ group.inventory_unit }}</p></div><div class="packaging-group-actions"><button type="button" class="secondary" :disabled="loading || busyKey !== '' || !groupHasShortage(group)" @click="openBatchIssueForTasks(group.tasks)">补齐物料</button><button type="button" class="primary" :disabled="loading || busyKey !== ''" @click="openGroupCompletion(group)">完成并入库</button></div></div>
            <div class="group-material-table"><div class="material-row material-head"><span>物料</span><span>需求</span><span>WIP本组可用</span><span>待补</span><span>来源仓可补</span></div><div v-for="material in group.materials" :key="material.identity" class="material-row"><strong>{{ material.material_name }}</strong><span>{{ materialQuantity(material, 'required') }}</span><span>{{ materialQuantity(material, 'wip') }}</span><span :class="{ shortage: materialHasShortage(material) }">{{ materialQuantity(material, 'shortage') }}</span><span>{{ sourceAvailabilityLabel(group, material) }}</span></div></div>
            <details class="group-source-orders"><summary>查看 {{ group.tasks.length }} 张来源工单</summary><div v-for="task in group.tasks" :key="taskKey(task)">{{ task.work_order_no }} · {{ task.customer_name || '客户未指定' }} · {{ quantityText(task.planned_output_inventory_qty) }} {{ task.inventory_unit }} <button type="button" class="text-button" @click="openExecutionHub(task, 'job_card')">查看工单</button></div></details>
            <WorkstationPackagingCompletionForm v-if="completionGroup.open && completionGroup.key === group.key" :key="completionGroup.key" :task-ids="completionGroup.ids" @close="completionGroup.open = false" @posted="groupCompleted" />
          </article>
        </div>
        <div v-if="selectedWorkstation && ungroupedTasks(section).length" class="task-table">
          <div class="task-row header">
            <span><input type="checkbox" aria-label="全选当前工位可领料任务" :checked="issueSelectionState(issueTasks(section), selectedIssueTasks).checked" :indeterminate="issueSelectionState(issueTasks(section), selectedIssueTasks).indeterminate" :disabled="!issueSelectionState(issueTasks(section), selectedIssueTasks).total" @change="selectedIssueTasks = toggleIssueSelection(issueTasks(section), selectedIssueTasks)" /> 任务</span>
            <span>状态</span>
            <span>执行人</span>
            <span>动作</span>
          </div>
          <div v-for="task in ungroupedTasks(section)"
            :key="taskKey(task)"
            class="task-row"
            :class="{ focused: isRequestedTask(task) }"
            :data-task-key="taskKey(task)">
            <div class="task-title">
              <strong><input v-if="!isSimplifiedTask(task)" v-model="selectedIssueTasks" type="checkbox" :value="Number(task.job_card_id)" :disabled="!workstationCanOpenIssue(task)" :aria-label="`选择任务 ${task.work_order_no} ${task.job_card_id}`" />{{ taskTitle(task) }}</strong>
              <small>{{ taskBatchLabel(task) }} · {{ task.work_order_no || '-' }} · P{{ task.priority || 0 }}</small>
              <small v-for="line in taskQuantityLines(task)" :key="line" class="task-quantity">{{ line }}</small>
              <small>工序要求：{{ task.process_requirement || '按冻结工艺路线执行' }}</small>
              <details v-if="task.material_readiness?.length" class="material-readiness" :class="`material-state-${materialReadinessState(task)}`">
                <summary>{{ materialSummary(task) }}</summary>
                <div class="material-row material-head"><span>物料名称</span><span>需求</span><span>WIP 可用</span><span>缺口</span></div>
                <div v-for="material in task.material_readiness" :key="material.reservation_id || material.material_id" class="material-row"><strong>{{ material.material_name }}</strong><span>{{ materialQuantity(material, 'required') }}</span><span>{{ materialQuantity(material, 'wip') }}</span><span :class="{ shortage: materialHasShortage(material) }">{{ materialQuantity(material, 'shortage') }}</span></div>
              </details>
              <div v-else-if="!task.has_predecessor && materialReadinessState(task) === 'unknown'" class="material-readiness material-state-unknown">用料待核对</div>
            </div>
            <span class="pill" :class="statusClass(task)">{{ task.status_label || task.status || '-' }}</span>
            <span class="task-staff">{{ task.assigned_to || task.roster_employee_name || '无人值班' }}<small v-if="task.pending_handover">排班已换人，待接手工位</small><small v-else-if="task.blocking_reason">待协同岗位：{{ task.next_handler || '-' }}</small></span>
            <div class="actions">
              <template v-if="isSimplifiedTask(task)">
                <button type="button" class="primary" :disabled="Boolean(busyKey) || loading || isTaskUnattended(task)" @click="openFinishPanel(task)">工序完成</button>
                <small v-if="isTaskUnattended(task)" class="schedule-required">今日无人值班，请先完成排班</small>
                <button type="button" class="secondary" @click="openExecutionHub(task, 'job_card')">查看工单</button>
              </template>
              <template v-else>
              <button type="button" class="secondary" @click="openExecutionHub(task, 'job_card')">查看工单</button>
              <button v-if="workstationCanOpenIssue(task)" type="button" class="primary" @click="openPicking(task)">领料</button>
              <button v-if="task.readiness_label === '待质检'" type="button" class="secondary" @click="openQuality(task)">查看质检</button>
              <details v-if="isFirstOperationTask(task) && task.status === 'running'" class="material-actions"><summary>物料操作</summary><button type="button" @click="openStockAction(task, 'consume')">耗料</button><button type="button" @click="openStockAction(task, 'return')">退料</button></details>
              <button
                v-for="action in workstationVisibleActions(task)"
                :key="action"
                type="button"
                :class="{ primary: action === 'start' || action === 'complete' }"
                :disabled="Boolean(busyKey) || loading"
                @click="handleTaskAction(task, action)"
              >
                {{ actionLabel(action) }}
              </button>
              </template>
            </div>
            <div
              v-if="taskFeedback.taskKey === taskKey(task) && (taskFeedback.message || taskFeedback.error)"
              class="task-feedback"
              :class="{ error: Boolean(taskFeedback.error) }">
              {{ taskFeedback.error || taskFeedback.message }}
            </div>
            <div v-if="isIssuePanelForTask(task)" class="task-action-panel">
              <div class="section-title-row">
                <div>
                  <div class="section-title">{{ issue.mode === 'material_call' ? '呼叫补料' : '报异常' }}</div>
                  <p class="muted">{{ issue.title }}</p>
                </div>
                <button class="secondary" type="button" @click="closeIssue">关闭</button>
              </div>
              <label>
                <span>{{ issue.mode === 'material_call' ? '补料说明' : '异常原因' }}</span>
                <textarea v-model.trim="issue.note" rows="3"></textarea>
              </label>
              <button class="primary" type="button" @click="submitIssue" :disabled="busyKey !== ''">提交</button>
            </div>

            <OperationCompletionForm v-if="isFinishPanelForTask(task) && isSimplifiedTask(task)" :key="task.job_card_id" :job-card-id="Number(task.job_card_id)" :return-params="completionReturnParams(task)" @close="closeFinishPanel" @posted="automaticPosted" />
            <OperationCompletionForm v-if="isFinishPanelForTask(task) && !isSimplifiedTask(task) && task.automatic_completion" :key="task.job_card_id" :job-card-id="Number(task.job_card_id)" :return-params="completionReturnParams(task)" @close="closeFinishPanel" @posted="automaticPosted" />
          </div>
        </div>
      </article>
      <div v-if="!visibleSections.length" class="empty-state">
        <strong>{{ emptyState.title }}</strong>
        <span>{{ emptyState.hint }}</span>
      </div>
    </section>
    <WorkstationBatchIssueDrawer v-if="batchIssue.open" :task-ids="batchIssue.ids" :workstation="selectedWorkstation" :return-params="{scope}" @close="batchIssue.open = false" @posted="load" />
    <ProductionExecutionHubDrawer
      :open="executionHub.open"
      :work-order-id="executionHub.workOrderId"
      :focus="executionHub.focus"
      :view-params="{ ...(props.viewParams || {}), work_order_id: executionHub.workOrderId, job_card_id: executionHub.jobCardId, focus: executionHub.focus }"
      @close="executionHub.open = false"
      @updated="load" />
  </div>
</template>

<script setup>
import OperationCompletionForm from '../components/OperationCompletionForm.vue'
import WorkstationBatchIssueDrawer from '../components/WorkstationBatchIssueDrawer.vue'
import WorkstationPackagingCompletionForm from '../components/WorkstationPackagingCompletionForm.vue'
import { issueSelectionState, toggleIssueSelection } from '../lib/workstation-batch-issue.js'
import ProductionReturnLink from '../components/ProductionReturnLink.vue'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { fetchProductionWorkstationOverview, runProductionTaskAction } from '../api/production.js'
import { apiGet, apiSend } from '../api/client'
import { actorHasFullViewAccess } from '../lib/menu-permissions.js'
import ProductionExecutionHubDrawer from '../components/ProductionExecutionHubDrawer.vue'
import {
  productionTaskActionEndpoint,
  productionTaskActionErrorMessage,
  materialReadinessState,
  taskTitle,
  taskQuantityLines,
  workstationCanOpenIssue,
  workstationVisibleActions,
  workstationTaskSections,
  workstationPackagingGroups,
  workstationPackagingMaterialIdentity,
  workstationFocusDisposition,
  workstationTaskRequiresSchedule,
} from '../lib/production-workstation.js'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
  actor: { type: Object, default: null },
})

const selectedIssueTasks = ref([])
const batchIssue = reactive({open: false, ids: []})
const completionGroup = reactive({ open: false, key: '', ids: [] })
const lastGroupResult = ref(null)
const packagingSourceAvailability = ref(new Map())
let packagingSourceAvailabilityRevision = 0
function groupedTasks(section) { return workstationPackagingGroups(section.tasks).flatMap(group => group.tasks) }
function ungroupedTasks(section) { const ids = new Set(groupedTasks(section).map(task => Number(task.job_card_id))); return section.tasks.filter(task => !ids.has(Number(task.job_card_id))) }
function isSimplifiedTask(task) { return Boolean(task?.automatic_completion) || /烘焙|烘烤|除石/.test(String(task?.operation || task?.operation_name || task?.task_name || '')) }
function issueTasks(section) { return ungroupedTasks(section).filter(task => !isSimplifiedTask(task)) }
function packagingGroups(section) { return workstationPackagingGroups(section.tasks) }
function openBatchIssueForTasks(tasks) { batchIssue.ids = tasks.filter(workstationCanOpenIssue).map(task => Number(task.job_card_id)); batchIssue.open = batchIssue.ids.length > 0 }
function openBatchIssue(section) { openBatchIssueForTasks(issueTasks(section).filter(task => selectedIssueTasks.value.includes(Number(task.job_card_id)))) }
function openGroupCompletion(group) { completionGroup.key = group.key; completionGroup.ids = group.tasks.map(task => Number(task.job_card_id)); completionGroup.open = true; batchIssue.open = false }
async function groupCompleted(result) { lastGroupResult.value = result; completionGroup.open = false; message.value = `已完成并入库 ${result.completed_qty} ${result.output_unit}`; await load() }
function quantityText(value) { return Number(value || 0).toLocaleString('zh-CN', { maximumFractionDigits: 3 }) }
function sourceAvailabilityKey(groupKey, materialIdentity) { return `${groupKey}:${materialIdentity}` }
function sourceAvailabilityLabel(group, material) {
  const row = packagingSourceAvailability.value.get(sourceAvailabilityKey(group.key, material.identity))
  if (!row) return '正在查询…'
  if (row.error) return '暂不可查询'
  if (row.g > 0) return `${quantityText(row.g)}g`
  return `${quantityText(row.units)}${material.unit || '件'}`
}
function warehouseLabel(code) { const key = String(code || ''); return warehouseNames.value.get(key) || ({ finished_goods: '成品仓', packaging: '包材仓', raw_materials: '原料仓', wip: 'WIP在制仓' })[key] || (key ? `仓库名待加载（${key}）` : '待确认') }
async function loadWarehouseNames() { try { const data = await apiGet('/api/stock/warehouses'); const rows = Array.isArray(data) ? data : (data.rows || data.warehouses || []); warehouseNames.value = new Map(rows.filter(row => row?.code).map(row => [String(row.code), String(row.name || row.code)])) } catch { warehouseNames.value = new Map() } }
async function loadPackagingSourceAvailability(tasks) {
  const revision = ++packagingSourceAvailabilityRevision
  const groups = workstationPackagingGroups(tasks)
  const values = new Map()
  for (const group of groups) for (const material of group.materials) values.set(sourceAvailabilityKey(group.key, material.identity), { g: 0, units: 0 })
  packagingSourceAvailability.value = values
  await Promise.all(groups.map(async group => {
    try {
      const data = await apiSend('/api/production/workstation/material-issue/preview', { body: { job_card_ids: group.tasks.map(task => Number(task.job_card_id)) } })
      for (const row of data.groups || []) {
        const identity = workstationPackagingMaterialIdentity({ item_type: row.item_type, item_id: row.item_id, material_id: row.item_id, bom_spec_id: row.bom_spec_id, owner_customer_id: row.owner_customer_id, unit: row.unit, spec_g: row.spec_g })
        const material = group.materials.find(item => item.identity === identity)
        if (!material) continue
        const key = sourceAvailabilityKey(group.key, identity)
        const availability = values.get(key) || { g: 0, units: 0 }
        if (row.quantity_basis === 'weight') availability.g += Number(row.available || 0)
        else availability.units += Number(row.available || 0)
        values.set(key, availability)
      }
    } catch {
      for (const material of group.materials) values.set(sourceAvailabilityKey(group.key, material.identity), { error: true })
    }
  }))
  if (revision === packagingSourceAvailabilityRevision) packagingSourceAvailability.value = new Map(values)
}
const loading = ref(false)
const busyKey = ref('')
const error = ref('')
const message = ref('')
const selectedWorkstationValue = ref('')
const selectedWorkstation = computed({ get: () => selectedWorkstationValue.value, set: value => { selectedWorkstationValue.value = value } })
const overview = ref({ tasks: [] })
const todayRoster = ref({ assignments: [] })
const warehouseNames = ref(new Map())
const canViewAll = computed(() => actorHasFullViewAccess(props.actor))
const scope = ref(String(props.viewParams?.scope || 'mine') === 'all' && canViewAll.value ? 'all' : 'mine')
const issue = reactive({ open: false, mode: '', title: '', task: null, note: '' })
const executionHub = reactive({ open: false, workOrderId: 0, jobCardId: 0, focus: '' })
const requestedJobCardID = computed(() => Number(props.viewParams?.job_card_id || 0))
const taskFeedback = reactive({ taskKey: '', message: '', error: '' })
const finishPanel = reactive({open:false,task:null})
const requestedTaskNotice = ref(null)
let overviewLoadRevision = 0

const tasks = computed(() => overview.value.tasks || [])
async function convertedOldOrder(){closeFinishPanel();message.value='核对已保存，后续工序可直接完成';await load()}
const lastAutomaticResult=ref(null)
async function automaticPosted(data){message.value=data.completion?.message || '完成记录已保存';lastAutomaticResult.value={...data.completion};finishPanel.open=false;await load()}
watch(selectedWorkstation, () => { selectedIssueTasks.value = []; batchIssue.open = false })
watch(tasks, rows => { const available = new Set(rows.filter(workstationCanOpenIssue).map(task => Number(task.job_card_id))); selectedIssueTasks.value = selectedIssueTasks.value.filter(id => available.has(id)) })
const sections = computed(() => {
  const taskSections = workstationTaskSections(tasks.value)
  const byName = new Map(taskSections.map(section => [section.workstation, section]))
  for (const assignment of todayRoster.value.assignments || []) {
    const section = byName.get(assignment.workstation) || { workstation: assignment.workstation, tasks: [], currentTask: null, nextTask: null, blockingReason: '' }
    section.assignment = assignment
    byName.set(assignment.workstation, section)
  }
  for (const section of byName.values()) if (!section.assignment) section.assignment = assignmentForSection(section)
  return Array.from(byName.values()).sort((a, b) => a.workstation.localeCompare(b.workstation, 'zh-Hans-CN'))
})
const workstationLoad = computed(() => overview.value.workstation_load || [])
const visibleSections = computed(() => selectedWorkstation.value ? sections.value.filter((section) => section.workstation === selectedWorkstation.value) : sections.value)
const visibleTaskCount = computed(() => visibleSections.value.reduce((total, section) => total + section.tasks.length, 0))
const singleStationLayout = computed(() => visibleSections.value.length === 1)
const emptyState = computed(() => {
  if (scope.value !== 'mine') return { title: '暂无工位任务', hint: '当前筛选范围内没有待执行或执行中的任务。' }
  if (Number(todayRoster.value.roster_version || 0) === 0) return { title: '本周排班尚未保存', hint: '请先完成本周生产排班，再查看今天负责的工位。' }
  if (todayRoster.value.attendance === 'off') return { title: '今天休息', hint: '今天不参与工位自动安排。' }
  if (todayRoster.value.attendance === 'unplanned') return { title: '今天尚未排班', hint: '请联系排班人员确认今天上班或休息。' }
  return { title: '今天已上班，暂未负责工位', hint: '如需临时换岗，请由排班人员调整当天工位负责人。' }
})

function loadStatusLabel(value) {
  return ({
    overloaded: '超负荷',
    blocked: '有待办',
    busy: '繁忙',
    normal: '正常',
    idle: '空闲',
  })[String(value || '').toLowerCase()] || '正常'
}

function taskKey(task) {
  return `${task.job_card_id || 0}:${task.work_order_id || 0}`
}

function isRequestedTask(task) {
  return requestedJobCardID.value > 0 && Number(task?.job_card_id || 0) === requestedJobCardID.value
}

const taskStatusLabels = { pending: '待执行', ready: '可开始', released: '待执行', running: '执行中', paused: '已暂停', blocked: '待处理', completed: '已完成', cancelled: '已取消' }
async function focusRequestedTask(options = {}) {
  const focus = String(props.viewParams?.focus || '')
  requestedTaskNotice.value = null
  if (focus !== 'workstation_task' || requestedJobCardID.value <= 0) return false
  const matchedTask = tasks.value.find((task) => Number(task?.job_card_id || 0) === requestedJobCardID.value)
  if (matchedTask) {
    selectedWorkstation.value = matchedTask.workstation
    const rosterStatus = isTaskUnattended(matchedTask) ? 'unattended' : matchedTask.roster_status
    const disposition = workstationFocusDisposition({ task: matchedTask, rosterStatus, canViewAll: canViewAll.value })
    const workOrderNo = matchedTask.work_order_no || `工单 #${matchedTask.work_order_id || props.viewParams?.work_order_id || ''}`
    const base = { ...matchedTask, work_order_no: workOrderNo, status_label: matchedTask.status_label || taskStatusLabels[matchedTask.status] || matchedTask.status || '状态未知', disposition }
    if (disposition === 'record') {
      openExecutionHub(matchedTask, 'job_card')
      requestedTaskNotice.value = { ...base, message: '该工序已结束，已打开工单记录。' }
    } else if (disposition === 'unattended') {
      requestedTaskNotice.value = { ...base, message: `今日该工位无人值班，请先完成排班。目标工序当前状态：${base.status_label}。` }
    } else if (options.scopeRetry === true) {
      message.value = '已从全厂视图加载目标任务'
      requestedTaskNotice.value = { ...base, message: `目标工序当前状态：${base.status_label}。` }
    }
    window.requestAnimationFrame(() => {
      document.querySelector(`[data-task-key="${taskKey(matchedTask)}"]`)?.scrollIntoView({ block: 'center', behavior: 'smooth' })
    })
    return true
  }
  if (canViewAll.value && scope.value === 'mine' && options.scopeRetry !== true) {
    scope.value = 'all'
    await load({ focusRequested: true, scopeRetry: true })
    return false
  }
  const workOrderID = Number(props.viewParams?.work_order_id || 0)
  if (workOrderID <= 0) {
    error.value = '缺少工单编号，无法确认目标工序状态'
    return false
  }
  try {
    const detail = await apiGet(`/api/produce/work-orders/${workOrderID}`)
    if (options.loadRevision !== overviewLoadRevision || requestedJobCardID.value !== Number(props.viewParams?.job_card_id || 0)) return false
    const operations = detail?.execution_hub?.operation_progress || []
    const operation = operations.find(row => Number(row.job_card_id || 0) === requestedJobCardID.value)
    if (!operation) {
      error.value = '工单中没有找到这个工序卡，请核对工单链接'
      return false
    }
    const rosterStatus = isTaskUnattended(operation) ? 'unattended' : operation.roster_status
    const disposition = workstationFocusDisposition({ task: operation, rosterStatus, canViewAll: canViewAll.value })
    const workOrderNo = detail?.execution_hub?.header?.work_order_no || `工单 #${workOrderID}`
    const base = { ...operation, work_order_id: workOrderID, work_order_no: workOrderNo, status_label: taskStatusLabels[operation.status] || operation.status || '状态未知', disposition }
    if (disposition === 'record') {
      executionHub.workOrderId = workOrderID
      executionHub.jobCardId = requestedJobCardID.value
      executionHub.focus = 'job_card'
      executionHub.open = true
      requestedTaskNotice.value = { ...base, message: '该工序已结束，已打开只读工单记录。' }
    } else if (disposition === 'unattended') {
      requestedTaskNotice.value = { ...base, message: '今日该工位无人值班，请先完成排班；排班完成后刷新即可继续。' }
    } else if (disposition === 'outside_scope') {
      requestedTaskNotice.value = { ...base, message: '工序仍未结束，但当前账号无权查看该工位任务。请联系排班负责人或使用有查看权限的账号。' }
    } else if (disposition === 'all_scope') {
      selectedWorkstation.value = String(operation.workstation || selectedWorkstation.value || '')
      requestedTaskNotice.value = { ...base, message: `目标工序当前状态：${base.status_label}。已尝试全厂视图，但待执行列表未返回该卡，请刷新工单状态。` }
    } else {
      requestedTaskNotice.value = { ...base, message: `目标工序当前状态：${base.status_label}。请刷新后核对。` }
    }
    return false
  } catch (err) {
    const message = String(err?.message || '')
    if (/403|permission|forbidden|权限/i.test(message)) error.value = '当前账号无权查看该工单状态，请联系管理员或排班负责人'
    else if (/404|not found|不存在/i.test(message)) error.value = '工单不存在或已删除，请检查工单链接'
    else error.value = message || '读取工单状态失败，请刷新后重试'
    return false
  }
}

function updateTaskFeedback(task, { message: nextMessage = '', error: nextError = '' } = {}) {
  taskFeedback.taskKey = taskKey(task)
  taskFeedback.message = nextMessage
  taskFeedback.error = nextError
}

function taskMeta(task) {
  if (!task) return '-'
  return `${taskQuantityLines(task).join(' · ')} · ${task.work_order_no || '-'} · ${task.next_handler || task.assigned_to || '-'} · ${task.planned_start_at || '未排时间'}`
}

function taskBatchLabel(task) {
  return `第 ${Number(task?.batch_index || 1)} 批 / 共 ${Number(task?.batch_count || 1)} 批`
}

function materialHasShortage(material) {
  return Number(material?.shortage_g || 0) > 0 || Number(material?.shortage_units || 0) > 0
}

function groupHasShortage(group) {
  return (group?.materials || []).some(materialHasShortage)
}

function materialSummary(task) {
  if (materialReadinessState(task) === 'unknown') return '用料待核对'
  const rows = task?.material_readiness || []
  const shortages = rows.filter(materialHasShortage).length
  return shortages ? `缺料 ${shortages} 项，展开查看` : `本次用料 ${rows.length} 项，WIP 已齐套`
}

function materialQuantity(material, kind) {
  const g = Number(material?.[`${kind === 'wip' ? 'wip_available' : kind}_g`] || 0)
  const units = Number(material?.[`${kind === 'wip' ? 'wip_available' : kind}_units`] || 0)
  if (g > 0) return `${g.toLocaleString('zh-CN')}g`
  return `${units.toLocaleString('zh-CN')}${material?.unit || '件'}`
}

function nextHandler(section) {
  return section.tasks.find((task) => task.blocking_reason)?.next_handler || '现场主管'
}

function stationLoad(section) {
  return workstationLoad.value.find((row) => row.workstation === section.workstation) || {}
}

function statusClass(task) {
  if (task.status_label === '异常') return 'danger'
  if (task.status_label === '执行中') return 'running'
  return ''
}

function actionLabel(action) {
  return {
    start: '开始本任务',
    pause: '暂停',
    resume: '继续',
    complete: '完成本工序',
    report_exception: '报异常',
    material_call: '呼叫补料',
  }[action] || action
}

function openScheduling() { window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'productionSchedule', params: { work_center: selectedWorkstation.value }, returnNavigation: { key: 'workstationView', params: { ...props.viewParams }, label: '返回工位视图' } } })) }

function completionReturnParams(task) {
  return {
    ...props.viewParams,
    scope: scope.value,
    workstation: task.workstation,
    workstation_id: Number(task.workstation_id || 0),
    station_id: Number(task.workstation_id || 0),
    work_order_id: Number(task.work_order_id || 0),
    job_card_id: Number(task.job_card_id || 0),
    focus: 'workstation_task',
  }
}

function assignmentForSection(section) { const task = section.tasks[0]; return task ? { workstation_id: task.workstation_id, workstation: section.workstation, employee_id: task.roster_employee_id, employee_name: task.roster_employee_name, source: task.roster_source, unattended: task.roster_status === 'unattended', override_invalid: task.roster_status === 'invalid_override', handover_count: section.tasks.filter(row => row.pending_handover).length } : null }
function rosterAssignmentForTask(task) { return (todayRoster.value.assignments || []).find(row => (Number(task?.workstation_id || 0) > 0 && Number(row.workstation_id || 0) === Number(task.workstation_id)) || (task?.workstation && row.workstation === task.workstation)) || null }
function isTaskUnattended(task) {
  return workstationTaskRequiresSchedule(task, rosterAssignmentForTask(task))
}
function rosterSourceLabel(row) { if (!row) return '尚未排班'; if (row.override_invalid) return '临时调整已失效'; if (row.unattended) return row.reason || '请先排班'; return ({ override: '临时调整', primary: '主负责人', backup: '替补接班' })[row.source] || '自动安排' }
async function switchScope(next) { scope.value = next; selectedWorkstation.value = ''; await load() }
async function handoverStation(section) { if (!section.assignment?.employee_id) return; busyKey.value = `handover:${section.assignment.workstation_id}`; error.value = ''; try { const result = await apiSend('/api/production-roster/handover', { body: { workstation_id: Number(section.assignment.workstation_id), work_date: todayRoster.value.date, employee_id: Number(section.assignment.employee_id), expected_version: Number(todayRoster.value.roster_version || 0), request_id: crypto.randomUUID() } }); message.value = `已接手 ${section.workstation}，共 ${result.job_card_ids?.length || 0} 项执行中任务`; await load() } catch (err) { error.value = err.message || '工位交接失败' } finally { busyKey.value = '' } }

function openPicking(task) {
  openStockAction(task, 'issue')
}

function isFirstOperationTask(task) {
  const sequences = tasks.value.filter((row) => Number(row.work_order_id) === Number(task.work_order_id)).map((row) => Number(row.sequence_no || 0)).filter((value) => value > 0)
  return sequences.length > 0 && Number(task.sequence_no || 0) === Math.min(...sequences)
}

function openStockAction(task, action) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'stockOperations', params: { tab: 'stockEntries', action, return_source: 'workstation_task', work_order_id: task.work_order_id, work_order_no: task.work_order_no, job_card_id: task.job_card_id }, returnNavigation: { key: 'workstationView', params: { scope: scope.value, workstation: selectedWorkstation.value || task.workstation, work_order_id: task.work_order_id, job_card_id: task.job_card_id, focus: 'workstation_task' }, label: '返回工位' } } }))
}

function openQuality(task) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'qualityInspections', params: { work_order_id: task.work_order_id, job_card_id: task.job_card_id, reference_no: task.work_order_no } } }))
}

function sameTask(a, b) {
  return Boolean(a && b) && taskKey(a) === taskKey(b)
}

function isIssuePanelForTask(task) {
  return issue.open && sameTask(issue.task, task)
}

function isFinishPanelForTask(task) {
  return finishPanel.open && sameTask(finishPanel.task, task)
}

function closeIssue() {
  issue.open = false
  issue.task = null
}

function closeFinishPanel() {
  finishPanel.open = false
  finishPanel.task = null
}

function openIssue(task, mode) {
  finishPanel.open = false
  finishPanel.task = null
  issue.open = true
  issue.mode = mode
  issue.task = task
  issue.title = `${taskTitle(task)} · ${task.work_order_no || ''}`
  issue.note = mode === 'material_call' ? '' : (task.blocking_reason || '')
}

function openFinishPanel(task) {
  if (isTaskUnattended(task)) {
    message.value = '今日该工位无人值班，请先完成排班'
    return
  }
  issue.open = false
  issue.task = null
  finishPanel.open = true
  finishPanel.task = task

}

function openExecutionHub(task, focus = 'job_card') {
  const id = Number(task?.work_order_id || 0)
  if (!id) return
  executionHub.workOrderId = id
  executionHub.jobCardId = Number(task?.job_card_id || 0)
  executionHub.focus = focus
  executionHub.open = true
}

async function handleTaskAction(task, action) {
  if (action === 'report_exception' || action === 'material_call') {
    openIssue(task, action)
    return
  }
  if (action === 'complete') {
    openFinishPanel(task)
    return
  }
  const endpoint = productionTaskActionEndpoint(task, action)
  if (!endpoint) return
  busyKey.value = `${task.job_card_id}:${action}`
  error.value = ''
  message.value = ''
  updateTaskFeedback(task)
  try {
    await runProductionTaskAction(endpoint, {})
    const refreshed = await load()
    if (!refreshed) {
      const explanation = `${actionLabel(action)}已提交，但状态刷新失败，请手动刷新`
      error.value = explanation
      updateTaskFeedback(task, { error: explanation })
      return
    }
    message.value = `${actionLabel(action)}成功`
    updateTaskFeedback(task, { message: `${actionLabel(action)}成功，状态已刷新` })
  } catch (err) {
    const explanation = productionTaskActionErrorMessage(err, action)
    error.value = explanation
    updateTaskFeedback(task, { error: explanation })
  } finally {
    busyKey.value = ''
  }
}

async function submitIssue() {
  const task = issue.task
  const mode = issue.mode
  const endpoint = productionTaskActionEndpoint(task, mode)
  if (!endpoint) return
  busyKey.value = `${task.job_card_id}:${mode}`
  error.value = ''
  message.value = ''
  updateTaskFeedback(task)
  try {
    const payload = mode === 'material_call' ? { note: issue.note } : { exception_reason: issue.note }
    await runProductionTaskAction(endpoint, payload)
    closeIssue()
    const submittedMessage = mode === 'material_call' ? '呼叫补料' : '上报异常'
    const refreshed = await load()
    if (!refreshed) {
      const explanation = `${submittedMessage}已提交，但状态刷新失败，请手动刷新`
      error.value = explanation
      updateTaskFeedback(task, { error: explanation })
      return
    }
    message.value = `${submittedMessage}成功`
    updateTaskFeedback(task, { message: `${submittedMessage}成功，状态已刷新` })
  } catch (err) {
    const explanation = productionTaskActionErrorMessage(err, mode)
    error.value = explanation
    updateTaskFeedback(task, { error: explanation })
  } finally {
    busyKey.value = ''
  }
}

async function load(options = {}) {
  const revision = ++overviewLoadRevision
  loading.value = true
  if (options?.focusRequested === true) error.value = ''
  try {
    const [overviewData, rosterData] = await Promise.all([fetchProductionWorkstationOverview({ limit: 500, scope: scope.value }), apiGet(`/api/production-roster/today?scope=${scope.value}`)])
    if (revision !== overviewLoadRevision) return false
    overview.value = overviewData
    todayRoster.value = rosterData
    void loadPackagingSourceAvailability(overviewData.tasks || [])
    if (selectedWorkstation.value && !sections.value.some((section) => section.workstation === selectedWorkstation.value)) {
      selectedWorkstation.value = ''
    }
    if (options?.focusRequested === true || String(props.viewParams?.focus || '') === 'workstation_task') await focusRequestedTask({ ...options, loadRevision: revision })
    return true
  } catch (err) {
    if (revision === overviewLoadRevision) error.value = err.message || '加载失败'
    return false
  } finally {
    if (revision === overviewLoadRevision) loading.value = false
  }
}

watch(() => [props.viewParams?.focus, props.viewParams?.work_order_id, props.viewParams?.job_card_id, props.viewParams?.workstation, props.viewParams?.workstation_id, props.viewParams?.station_id, props.viewParams?.scope], async () => {
  const requestedScope = String(props.viewParams?.scope || '')
  if (requestedScope === 'mine' || (requestedScope === 'all' && canViewAll.value)) scope.value = requestedScope
  selectedWorkstation.value = String(props.viewParams?.workstation || tasks.value.find(row => Number(row.workstation_id || 0) === Number(props.viewParams?.workstation_id || props.viewParams?.station_id || 0))?.workstation || '')
  finishPanel.open = false
  finishPanel.task = null
  executionHub.open = false
  executionHub.workOrderId = 0
  executionHub.jobCardId = 0
  requestedTaskNotice.value = null
  if (String(props.viewParams?.focus || '') === 'workstation_task') await load({ focusRequested: true })
}, { flush: 'post' })

onMounted(async () => {
  void loadWarehouseNames()
  selectedWorkstation.value = String(props.viewParams?.workstation || '')
  await load({ focusRequested: true })
  const focus = String(props.viewParams?.focus || '')
  if (focus === 'workstation_task') return
  const id = Number(props.viewParams?.work_order_id || 0)
  if (!id) return
  executionHub.workOrderId = id
  executionHub.jobCardId = Number(props.viewParams?.job_card_id || 0)
  executionHub.focus = focus || (executionHub.jobCardId ? 'job_card' : 'summary')
  executionHub.open = true
})
</script>

<style scoped>
.packaging-groups{display:grid;gap:16px;margin:18px 0}.packaging-group-card{border:1px solid #d7e2da;border-radius:12px;padding:18px;background:#fff}.packaging-group-head{display:flex;justify-content:space-between;align-items:flex-start;gap:14px}.packaging-group-head h4{margin:0 0 6px;font-size:18px;color:#203b2c}.packaging-group-head p,.group-source-orders{color:#69776f}.packaging-group-actions{display:flex;gap:8px;flex-wrap:wrap}.group-material-table{margin-top:14px}.group-material-table .material-row{grid-template-columns:minmax(130px,1fr) repeat(4,minmax(74px,.55fr))}.group-source-orders{margin-top:12px}.group-source-orders summary{cursor:pointer}.group-source-orders>div{padding:7px 0}.text-button{border:0;background:none;color:#267349;text-decoration:underline;cursor:pointer}.group-result{padding:16px}.group-result p{margin:.3rem 0}
.batch-issue-toolbar{display:flex;align-items:center;gap:14px;flex-wrap:wrap;margin:18px 0 12px}.batch-issue-toolbar small{color:#6c786f}.task-table input[type=checkbox]{width:16px;height:16px;vertical-align:middle;margin:0 8px 0 0;accent-color:#315740}

.page { padding: 20px; color: #252525; }
.toolbar, .panel, .station-panel {
  border: 1px solid #e2ded7;
  border-radius: 8px;
  background: #fff;
}
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 16px;
  margin-bottom: 14px;
}
h2, h3 { margin: 0; line-height: 1.25; letter-spacing: 0; }
h2 { font-size: 22px; }
h3 { font-size: 18px; }
p { margin: 4px 0 0; color: #666; }
button {
  min-height: 34px;
  border: 1px solid #cfc8bf;
  border-radius: 8px;
  background: #fff;
  padding: 6px 10px;
  color: #222;
  cursor: pointer;
  font: inherit;
}
button.primary { background: #2f8f5b; border-color: #2f8f5b; color: #fff; }
button.secondary { background: #f8f7f5; }
button:disabled { opacity: .55; cursor: not-allowed; }
.toolbar-actions {
  display: flex;
  align-items: end;
  gap: 10px;
  flex-wrap: wrap;
}
.scope-switch{display:flex;border:1px solid #cfdad3;border-radius:8px;overflow:hidden}.scope-switch button{border:0;border-radius:0}.scope-switch button.active{background:#e9f6ed;color:#217546;font-weight:700}.station-owner{color:#26814d!important;font-weight:700}.station-owner.unattended{color:#b36f19!important}.station-head-actions{display:flex;align-items:flex-start;gap:7px;flex-wrap:wrap;justify-content:flex-end}.handover{background:#fff5e5;color:#9a641d;border-color:#e8c890}.no-task{padding:22px;border:1px dashed #cfded4;border-radius:8px;background:#f7fbf8;color:#5e7967;text-align:center}
label { display: grid; gap: 5px; color: #555; font-size: 13px; }
select, input, textarea {
  min-height: 34px;
  border: 1px solid #cfc8bf;
  border-radius: 8px;
  padding: 6px 8px;
  background: #fff;
  font: inherit;
}
textarea { resize: vertical; }
.notice, .error {
  padding: 10px 12px;
  border-radius: 8px;
  margin-bottom: 12px;
}
.notice { border: 1px solid #b7dfc4; background: #effaf2; color: #175c2f; }
.error { border: 1px solid #efb9b9; background: #fff2f2; color: #9d2424; }
.station-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.station-grid.single-station-grid { grid-template-columns: minmax(0, 1fr); }
.station-panel { padding: 14px; min-width: 0; }
.station-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.blocker {
  align-self: start;
  border: 1px solid #efb9b9;
  border-radius: 999px;
  padding: 4px 8px;
  background: #fff2f2;
  color: #9d2424;
  font-size: 12px;
}
.answer-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 12px;
}
.answer-block {
  display: grid;
  gap: 5px;
  min-height: 94px;
  border: 1px solid #ebe7df;
  border-radius: 8px;
  padding: 10px;
}
.answer-block span { color: #777; font-size: 12px; }
.answer-block strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.answer-block small { color: #666; line-height: 1.35; }
    .answer-block.blocked:not(.empty) { border-color: #efb9b9; background: #fffafa; }
    .enter-station { width: 100%; margin-bottom: 12px; }
    .task-table {
  display: grid;
  border: 1px solid #ebe7df;
  border-radius: 8px;
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
  overscroll-behavior-inline: contain;
}
.task-row {
  display: grid;
  grid-template-columns: minmax(180px, 1.4fr) 80px minmax(155px, .7fr) minmax(180px, 1.2fr);
  min-width: 610px;
  gap: 10px;
  align-items: center;
  padding: 10px;
  border-top: 1px solid #ebe7df;
}
.task-staff{display:grid;gap:5px;align-content:start;min-width:0}.task-staff small{display:block;font-size:12px;color:#657269;line-height:1.6}
.task-row.header {
  border-top: 0;
  background: #faf9f7;
  color: #666;
  font-size: 13px;
  font-weight: 700;
}
.task-row.focused {
  background: #fffbea;
  box-shadow: inset 3px 0 0 #d97706;
}
.task-title { display: grid; gap: 3px; min-width: 0; }
.task-title strong, .task-title small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.task-title small { color: #777; }
.task-title .task-quantity { color:#294d39;font-weight:650; }
.task-title>details{margin-top:5px}.material-readiness summary{cursor:pointer;color:#a85a08;font-size:12px}.material-row{display:grid;grid-template-columns:minmax(130px,1fr) repeat(3,minmax(74px,.55fr));gap:7px;padding:6px 0;border-top:1px solid #eee;font-size:12px}.material-row.material-head{color:#707a75}.material-row .shortage{color:#b85d0a;font-weight:700}.bulk-assignment{display:grid;grid-template-columns:minmax(240px,1fr) minmax(180px,260px) auto;gap:12px;align-items:end}.bulk-assignment p{font-size:12px}.assignment-panel{grid-template-columns:minmax(180px,280px) auto auto;align-items:end}
.material-readiness.material-state-ready summary{color:#23824d;font-weight:700}.material-readiness.material-state-shortage summary{color:#b85d0a;font-weight:700}.material-state-unknown{margin-top:5px;color:#7b8580;font-size:12px}
.pill {
  justify-self: start;
  border: 1px solid #d8d2c8;
  border-radius: 999px;
  padding: 3px 8px;
  background: #faf9f7;
  font-size: 12px;
}
.pill.running { border-color: #bdd8f0; background: #eef7ff; }
.pill.danger { border-color: #efb9b9; background: #fff2f2; color: #9d2424; }
.actions { display: flex; gap: 6px; flex-wrap: wrap; }
.actions button { min-height: 30px; padding: 4px 8px; font-size: 12px; }
.material-actions{position:relative}.material-actions summary{list-style:none;border:1px solid #cfc8bf;border-radius:8px;padding:5px 8px;cursor:pointer;font-size:12px}.material-actions button{display:block;width:100%;margin-top:4px;background:#fff}
.task-feedback {
  grid-column: 1 / -1;
  border: 1px solid #b7dfc4;
  border-radius: 6px;
  padding: 7px 9px;
  background: #effaf2;
  color: #175c2f;
  font-size: 13px;
}
.task-feedback.error {
  border-color: #efb9b9;
  background: #fff2f2;
  color: #9d2424;
}
.empty-state {
  grid-column: 1 / -1;
  display: grid;
  gap: 6px;
  border: 1px solid #e2ded7;
  border-radius: 8px;
  padding: 18px;
  color: #777;
  background: #fff;
}
.empty-state strong { color: #222; font-size: 16px; }
.task-action-panel {
  grid-column: 1 / -1;
  display: grid;
  gap: 10px;
  margin-top: 4px;
  padding: 12px;
  border: 1px solid #ebe7df;
  border-radius: 8px;
  background: #fffdf8;
}
.section-title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}
.section-title { font-weight: 800; font-size: 16px; line-height: 1.25; }
.muted { color: #777; font-size: 13px; }
.form-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  align-items: end;
}
.form-grid .span-2 { grid-column: span 2; }

@media (max-width: 1100px) {
  .station-grid { grid-template-columns: 1fr; }
}

@media (max-width: 760px) {
  .page { padding: 14px; }
  .toolbar, .station-head { align-items: stretch; flex-direction: column; }
  .answer-grid, .form-grid { grid-template-columns: 1fr; }
  .form-grid .span-2 { grid-column: auto; }
  .task-row { grid-template-columns: 1fr; min-width: 0; align-items: start; }
  .task-row.header { display: none; }
  .bulk-assignment,.assignment-panel,.material-row{grid-template-columns:1fr}.material-row.material-head{display:none}
}
</style>
