<template>
  <div class="page workstation-view">

    <ProductionReturnLink :source="viewParams.return_navigation" />
    <section class="toolbar">
      <div>
        <h2>工位视图</h2>
        <p>{{ visibleSections.length }} 个工位 · {{ visibleTaskCount }} 个任务</p>
      </div>
      <div class="toolbar-actions">
        <label>
          <span>工位</span>
          <select v-model="selectedWorkstation">
            <option value="">全部工位</option>
            <option v-for="section in sections" :key="section.workstation" :value="section.workstation">{{ section.workstation }}</option>
          </select>
        </label>
        <button class="secondary" type="button" @click="openScheduling">安排人员与时间</button>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
    </section>

    <div v-if="message" class="notice">{{ message }}</div>
    <div v-if="error" class="error">{{ error }}</div>

    <section class="station-grid" :class="{ 'single-station-grid': singleStationLayout }">
      <article v-for="section in visibleSections" :key="section.workstation" class="station-panel">
        <div class="station-head">
          <div>
            <h3>{{ section.workstation }}</h3>
            <p>{{ loadStatusLabel(stationLoad(section).load_status) }} · 队列 {{ stationLoad(section).queue_count || section.tasks.length }} · 阻塞 {{ stationLoad(section).blocked_count || 0 }} · 预计 {{ stationLoad(section).estimated_minutes || 0 }} 分钟</p>
          </div>
          <span v-if="section.blockingReason" class="blocker">{{ section.blockingReason }}</span>
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

        <button v-if="!selectedWorkstation" class="enter-station primary" type="button" @click="selectedWorkstation = section.workstation">进入本工位</button>

        <div v-if="selectedWorkstation" class="task-table">
          <div class="task-row header">
            <span>任务</span>
            <span>状态</span>
            <span>执行人</span>
            <span>动作</span>
          </div>
          <div v-for="task in section.tasks"
            :key="taskKey(task)"
            class="task-row"
            :class="{ focused: isRequestedTask(task) }"
            :data-task-key="taskKey(task)">
            <div class="task-title">
              <strong>{{ taskTitle(task) }}</strong>
              <small>{{ taskBatchLabel(task) }} · {{ task.work_order_no || '-' }} · P{{ task.priority || 0 }}</small>
              <small v-for="line in taskQuantityLines(task)" :key="line" class="task-quantity">{{ line }}</small>
              <small>工序要求：{{ task.process_requirement || '按冻结工艺路线执行' }}</small>
              <details v-if="task.material_readiness?.length" class="material-readiness" :class="`material-state-${materialReadinessState(task)}`">
                <summary>{{ materialSummary(task) }}</summary>
                <div class="material-row material-head"><span>物料名称</span><span>需求</span><span>WIP 可用</span><span>缺口</span></div>
                <div v-for="material in task.material_readiness" :key="material.reservation_id || material.material_id" class="material-row"><strong>{{ material.material_name }}</strong><span>{{ materialQuantity(material, 'required') }}</span><span>{{ materialQuantity(material, 'wip') }}</span><span :class="{ shortage: materialHasShortage(material) }">{{ materialQuantity(material, 'shortage') }}</span></div>
              </details>
              <div v-else-if="materialReadinessState(task) === 'unknown'" class="material-readiness material-state-unknown">用料待核对</div>
            </div>
            <span class="pill" :class="statusClass(task)">{{ task.status_label || task.status || '-' }}</span>
            <span class="task-staff">{{ task.assigned_to || '待配置负责人' }}<small v-if="task.blocking_reason">待协同岗位：{{ task.next_handler || '-' }}</small></span>
            <div class="actions">
              <button type="button" class="secondary" @click="openExecutionHub(task, 'job_card')">查看工单</button>
              <button v-if="!task.assigned_to" type="button" class="secondary" @click="claimTask(task)">领取任务</button>
              <button type="button" class="secondary" @click="openTaskAssignment(task)">{{ task.assigned_to ? '调整人员' : '分配人员' }}</button>
              <button v-if="workstationCanOpenIssue(task)" type="button" class="primary" @click="openPicking(task)">领料</button>
              <button v-if="task.readiness_label === '待质检'" type="button" class="secondary" @click="openQuality(task)">查看质检</button>
              <details v-if="isFirstOperationTask(task) && (workstationCanOpenIssue(task) || task.status === 'running')" class="material-actions"><summary>物料操作</summary><button v-if="workstationCanOpenIssue(task)" type="button" @click="openStockAction(task, 'issue')">领料</button><button v-if="task.status === 'running'" type="button" @click="openStockAction(task, 'consume')">耗料</button><button v-if="task.status === 'running'" type="button" @click="openStockAction(task, 'return')">退料</button></details>
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
            </div>
            <div v-if="assignmentTask && sameTask(assignmentTask, task)" class="task-action-panel">
              <ProductionTaskStaffEditor ref="staffEditors" :job-card-id="Number(task.job_card_id)" :work-order-id="Number(task.work_order_id)" @saved="assignmentTask = null; load()" @cancel="assignmentTask = null" />
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

            <div v-if="isFinishPanelForTask(task)" class="task-action-panel">
              <div class="section-title-row">
                <div>
                  <div class="section-title">完成本工序</div>
                  <p class="muted">{{ finishPanel.title }}</p>
                </div>
                <button class="secondary" type="button" @click="closeFinishPanel">关闭</button>
              </div>
              <div class="form-grid">
                <label><span>实际分钟</span><input v-model.number="finishPanel.actual_minutes" type="number" min="0" /></label>
                <label><span>实际投入（{{ finishPanel.inventory_unit || '-' }}）</span><input v-model.number="finishPanel.actual_input_qty" type="number" min="0" step="any" /></label>
                <label><span>实际产出（{{ finishPanel.inventory_unit || '-' }}）</span><input v-model.number="finishPanel.actual_output_qty" type="number" min="0" step="any" :disabled="Number(finishPanel.finished_units || 0) > 0" /></label>
                <label><span>成品件数（件）</span><input v-model.number="finishPanel.finished_units" type="number" min="0" step="1" :disabled="Number(finishPanel.actual_output_qty || 0) > 0" /><small>实际产出或成品件数二选一</small></label>
                <label><span>余料（{{ finishPanel.inventory_unit || '-' }}）</span><input v-model.number="finishPanel.leftover_qty" type="number" min="0" step="any" /></label>
                <label><span>损耗原因</span><input v-model.trim="finishPanel.loss_reason" /></label>
                <label><span>异常原因</span><input v-model.trim="finishPanel.exception_reason" /></label>
                <label class="span-2"><span>备注</span><input v-model.trim="finishPanel.note" /></label>
                <button class="primary" type="button" @click="submitFinishPanel" :disabled="busyKey !== ''">
                  完成本工序
                </button>
              </div>
            </div>
          </div>
        </div>
      </article>
      <p v-if="!visibleSections.length" class="empty-state">暂无工位任务</p>
    </section>
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
import ProductionReturnLink from '../components/ProductionReturnLink.vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { fetchProductionWorkstationOverview, runProductionTaskAction } from '../api/production.js'
import { apiGet, apiSend } from '../api/client'
import ProductionExecutionHubDrawer from '../components/ProductionExecutionHubDrawer.vue'
import ProductionTaskStaffEditor from '../components/ProductionTaskStaffEditor.vue'
import {
  productionCompletionMetrics,
  productionCompletionOutputQty,
  productionTaskActionEndpoint,
  productionTaskActionErrorMessage,
  materialReadinessState,
  taskTitle,
  taskQuantityLines,
  workstationCanOpenIssue,
  workstationVisibleActions,
  workstationTaskSections,
} from '../lib/production-workstation.js'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
})

const loading = ref(false)
const busyKey = ref('')
const error = ref('')
const message = ref('')
const staffEditors = ref([])
function staffCanLeave() { return (Array.isArray(staffEditors.value) ? staffEditors.value : [staffEditors.value]).filter(Boolean).every(editor => editor.canLeave()) }
const selectedWorkstationValue = ref('')
const selectedWorkstation = computed({ get: () => selectedWorkstationValue.value, set: value => { if (value !== selectedWorkstationValue.value && !staffCanLeave()) return; selectedWorkstationValue.value = value; assignmentTask.value = null } })
const overview = ref({ tasks: [] })
const assignmentTask = ref(null)
const issue = reactive({ open: false, mode: '', title: '', task: null, note: '' })
const executionHub = reactive({ open: false, workOrderId: 0, jobCardId: 0, focus: '' })
const requestedJobCardID = computed(() => Number(props.viewParams?.job_card_id || 0))
const taskFeedback = reactive({ taskKey: '', message: '', error: '' })
const finishPanel = reactive({
  open: false,
  title: '',
  task: null,
  inventory_unit: '',
  actual_minutes: 0,
  actual_input_qty: 0,
  actual_output_qty: 0,
  finished_units: 0,
  leftover_qty: 0,
  loss_reason: '',
  exception_reason: '',
  note: '',
})

const tasks = computed(() => overview.value.tasks || [])
const sections = computed(() => workstationTaskSections(tasks.value))
const workstationLoad = computed(() => overview.value.workstation_load || [])
const visibleSections = computed(() => selectedWorkstation.value ? sections.value.filter((section) => section.workstation === selectedWorkstation.value) : sections.value)
const visibleTaskCount = computed(() => visibleSections.value.reduce((total, section) => total + section.tasks.length, 0))
const singleStationLayout = computed(() => visibleSections.value.length === 1)

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

function focusRequestedTask() {
  const focus = String(props.viewParams?.focus || '')
  if (focus !== 'workstation_task' || requestedJobCardID.value <= 0) return false
  const matchedTask = tasks.value.find((task) => Number(task?.job_card_id || 0) === requestedJobCardID.value)
  if (!matchedTask) {
    error.value = '该任务已结束或已移出待执行队列，已为你打开工单查看结束状态'
    const workOrderID = Number(props.viewParams?.work_order_id || 0)
    if (workOrderID > 0) {
      executionHub.workOrderId = workOrderID
      executionHub.jobCardId = requestedJobCardID.value
      executionHub.focus = 'job_card'
      executionHub.open = true
    }
    return false
  }
  selectedWorkstation.value = matchedTask.workstation
  window.requestAnimationFrame(() => {
    document.querySelector(`[data-task-key="${taskKey(matchedTask)}"]`)?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  })
  return true
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

function openTaskAssignment(task) { if (staffCanLeave()) assignmentTask.value = task }
function openScheduling() { window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'productionSchedule', params: { work_center: selectedWorkstation.value }, returnNavigation: { key: 'workstationView', params: { ...props.viewParams }, label: '返回工位视图' } } })) }

async function claimTask(task) {
  const endpoint = productionTaskActionEndpoint(task, 'claim')
  if (!endpoint) return
  busyKey.value = `${task.job_card_id}:claim`
  error.value = ''
  try { await apiSend(endpoint, { body: {} }); await load(); message.value = '任务已领取' } catch (err) { error.value = err.message || '领取任务失败' } finally { busyKey.value = '' }
}

function openPicking(task) {
  openStockAction(task, 'issue')
}

function isFirstOperationTask(task) {
  const sequences = tasks.value.filter((row) => Number(row.work_order_id) === Number(task.work_order_id)).map((row) => Number(row.sequence_no || 0)).filter((value) => value > 0)
  return sequences.length > 0 && Number(task.sequence_no || 0) === Math.min(...sequences)
}

function openStockAction(task, action) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'stockOperations', params: { tab: 'stockEntries', action, return_source: 'workstation_task', work_order_id: task.work_order_id, work_order_no: task.work_order_no, job_card_id: task.job_card_id }, returnNavigation: { key: 'workstationView', params: { workstation: selectedWorkstation.value, work_order_id: task.work_order_id, job_card_id: task.job_card_id, focus: 'workstation_task' }, label: '返回工位视图' } } }))
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
  issue.open = false
  issue.task = null
  finishPanel.open = true
  finishPanel.task = task
  finishPanel.title = `${taskTitle(task)} · ${task.work_order_no || ''}`
  finishPanel.inventory_unit = String(task.inventory_unit || '').trim()
  finishPanel.actual_minutes = Number(task.actual_minutes || 0)
  finishPanel.actual_input_qty = Number(task.actual_input_qty || task.planned_input_inventory_qty || 0)
  finishPanel.actual_output_qty = Number(task.actual_output_qty || 0)
  finishPanel.finished_units = Number(task.actual_finished_units || 0)
  finishPanel.leftover_qty = Number(task.leftover_qty || 0)
  finishPanel.loss_reason = task.loss_reason || ''
  finishPanel.exception_reason = task.exception_reason || ''
  finishPanel.note = task.note || ''
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

async function submitFinishPanel() {
  const task = finishPanel.task
  if (!task) return
  busyKey.value = `${task.job_card_id}:complete`
  error.value = ''
  message.value = ''
  updateTaskFeedback(task)
  try {
    const finishedUnits = Number(finishPanel.finished_units || 0)
    const actualOutputQty = productionCompletionOutputQty({
      actualOutputQty: finishPanel.actual_output_qty,
      finishedUnits,
      inventoryQtyPerSalesUnit: task.inventory_qty_per_sales_unit,
    })
    const endpoint = productionTaskActionEndpoint(task, 'complete')
    await runProductionTaskAction(endpoint, {
      actual_minutes: Number(finishPanel.actual_minutes || 0),
      actual_input_qty: Number(finishPanel.actual_input_qty || 0),
      actual_output_qty: actualOutputQty,
      loss_reason: finishPanel.loss_reason || '',
      exception_reason: finishPanel.exception_reason || '',
      metrics_json: productionCompletionMetrics({
        inventoryUnit: finishPanel.inventory_unit,
        leftoverQty: finishPanel.leftover_qty,
        note: finishPanel.note,
        finishedUnits,
      }),
    })
    closeFinishPanel()
    const refreshed = await load()
    if (!refreshed) {
      const explanation = '完成本工序已提交，但状态刷新失败，请手动刷新'
      error.value = explanation
      updateTaskFeedback(task, { error: explanation })
      return
    }
    message.value = '完成本工序成功'
    updateTaskFeedback(task, { message: message.value })
  } catch (err) {
    const explanation = productionTaskActionErrorMessage(err, 'complete')
    error.value = explanation
    updateTaskFeedback(task, { error: explanation })
  } finally {
    busyKey.value = ''
  }
}

async function load(options = {}) {
  loading.value = true
  error.value = ''
  try {
    overview.value = await fetchProductionWorkstationOverview({ limit: 500 })
    if (selectedWorkstation.value && !sections.value.some((section) => section.workstation === selectedWorkstation.value)) {
      selectedWorkstation.value = ''
    }
    if (options?.focusRequested === true) focusRequestedTask()
    return true
  } catch (err) {
    error.value = err.message || '加载失败'
    return false
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
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
  border: 1px solid #e2ded7;
  border-radius: 8px;
  padding: 18px;
  color: #777;
  background: #fff;
}
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
