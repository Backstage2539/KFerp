<template>
  <div class="page workstation-view">
    <ProductionTopNav active-key="workstationView" />

    <section class="toolbar">
      <div>
        <h2>工位视图</h2>
        <p>{{ visibleSections.length }} 个工位 · {{ tasks.length }} 个任务</p>
      </div>
      <div class="toolbar-actions">
        <label>
          <span>工位</span>
          <select v-model="selectedWorkstation">
            <option value="">全部工位</option>
            <option v-for="section in sections" :key="section.workstation" :value="section.workstation">{{ section.workstation }}</option>
          </select>
        </label>
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
            <p>{{ stationLoad(section).load_status || 'normal' }} · 队列 {{ stationLoad(section).queue_count || section.tasks.length }} · 阻塞 {{ stationLoad(section).blocked_count || 0 }} · 预计 {{ stationLoad(section).estimated_minutes || 0 }} 分钟</p>
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
            <span>阻塞原因 / 不能做原因</span>
            <strong>{{ section.blockingReason || '无阻塞' }}</strong>
            <small>{{ section.blockingReason ? nextHandler(section) : '可继续执行' }}</small>
          </div>
        </div>

        <div class="task-table">
          <div class="task-row header">
            <span>任务</span>
            <span>状态</span>
            <span>负责人</span>
            <span>动作</span>
          </div>
          <div v-for="task in section.tasks" :key="taskKey(task)" class="task-row">
            <div class="task-title">
              <strong>{{ taskTitle(task) }}</strong>
              <small>{{ task.work_order_no || '-' }} · P{{ task.priority || 0 }}</small>
            </div>
            <span class="pill" :class="statusClass(task)">{{ task.status_label || task.status || '-' }}</span>
            <span>{{ task.next_handler || task.assigned_to || '-' }}</span>
            <div class="actions">
              <button type="button" class="secondary" @click="openExecutionHub(task, 'job_card')">详情</button>
              <button
                v-for="action in task.available_actions || []"
                :key="action"
                type="button"
                :class="{ primary: action === 'start' || action === 'complete' }"
                :disabled="busyKey === `${task.job_card_id}:${action}`"
                @click="handleTaskAction(task, action)"
              >
                {{ actionLabel(action) }}
              </button>
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
                  <div class="section-title">{{ finishPanel.mode === 'partial_finish' ? '部分完成' : '完成本工序' }}</div>
                  <p class="muted">{{ finishPanel.title }}</p>
                </div>
                <button class="secondary" type="button" @click="closeFinishPanel">关闭</button>
              </div>
              <div class="form-grid">
                <label><span>投料(g)</span><input v-model.number="finishPanel.consumed_input_g" type="number" min="0" /></label>
                <label><span>成品件数</span><input v-model.number="finishPanel.finished_units" type="number" min="0" /></label>
                <label><span>余料(g)</span><input v-model.number="finishPanel.finished_loose_g" type="number" min="0" /></label>
                <label><span>入库仓</span><input v-model.trim="finishPanel.warehouse" /></label>
                <label class="span-2"><span>异常/备注</span><input v-model.trim="finishPanel.note" /></label>
                <button class="primary" type="button" @click="submitFinishPanel" :disabled="busyKey !== ''">
                  {{ finishPanel.mode === 'partial_finish' ? '记录部分完成' : '完成本工序' }}
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
      @close="executionHub.open = false" />
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { fetchProductionWorkstationOverview, finishRunningProduction, runProductionTaskAction } from '../api/production.js'
import ProductionExecutionHubDrawer from '../components/ProductionExecutionHubDrawer.vue'
import ProductionTopNav from '../components/ProductionTopNav.vue'
import { productionTaskActionEndpoint, taskTitle, workstationTaskSections } from '../lib/production-workstation.js'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
})

const loading = ref(false)
const busyKey = ref('')
const error = ref('')
const message = ref('')
const selectedWorkstation = ref('')
const overview = ref({ tasks: [] })
const issue = reactive({ open: false, mode: '', title: '', task: null, note: '' })
const executionHub = reactive({ open: false, workOrderId: 0, jobCardId: 0, focus: '' })
const finishPanel = reactive({
  open: false,
  mode: '',
  title: '',
  task: null,
  finished_units: 0,
  finished_loose_g: 0,
  consumed_input_g: 0,
  warehouse: 'finished_goods',
  note: '',
})

const tasks = computed(() => overview.value.tasks || [])
const sections = computed(() => workstationTaskSections(tasks.value))
const workstationLoad = computed(() => overview.value.workstation_load || [])
const visibleSections = computed(() => selectedWorkstation.value ? sections.value.filter((section) => section.workstation === selectedWorkstation.value) : sections.value)
const singleStationLayout = computed(() => visibleSections.value.length === 1)

function taskKey(task) {
  return `${task.job_card_id || 0}:${task.work_order_id || 0}`
}

function taskMeta(task) {
  if (!task) return '-'
  return `${task.work_order_no || '-'} · ${task.next_handler || task.assigned_to || '-'} · ${task.planned_start_at || '未排时间'}`
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
    start: '开始',
    pause: '暂停',
    resume: '继续',
    complete: '完成本工序',
    partial_finish: '部分完成',
    report_exception: '报异常',
    material_call: '呼叫补料',
  }[action] || action
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

function openFinishPanel(task, mode) {
  issue.open = false
  issue.task = null
  finishPanel.open = true
  finishPanel.mode = mode
  finishPanel.task = task
  finishPanel.title = `${taskTitle(task)} · ${task.work_order_no || ''}`
  finishPanel.finished_units = 0
  finishPanel.finished_loose_g = 0
  finishPanel.consumed_input_g = Number(task.planned_g || task.planned_output_g || 0)
  finishPanel.warehouse = 'finished_goods'
  finishPanel.note = task.blocking_reason || ''
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
  if (action === 'complete' || action === 'partial_finish') {
    openFinishPanel(task, action)
    return
  }
  const endpoint = productionTaskActionEndpoint(task, action)
  if (!endpoint) return
  busyKey.value = `${task.job_card_id}:${action}`
  error.value = ''
  message.value = ''
  try {
    await runProductionTaskAction(endpoint, {})
    message.value = `${actionLabel(action)}已提交`
    await load()
  } catch (err) {
    error.value = err.message || '操作失败'
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
  try {
    const payload = mode === 'material_call' ? { note: issue.note } : { exception_reason: issue.note }
    await runProductionTaskAction(endpoint, payload)
    closeIssue()
    message.value = mode === 'material_call' ? '已呼叫补料' : '已上报异常'
    await load()
  } catch (err) {
    error.value = err.message || '提交失败'
  } finally {
    busyKey.value = ''
  }
}

async function submitFinishPanel() {
  const task = finishPanel.task
  if (!task) return
  const mode = finishPanel.mode
  busyKey.value = `${task.job_card_id}:${mode}`
  error.value = ''
  message.value = ''
  try {
    if (mode === 'partial_finish') {
      if (!task.running_item_id) throw new Error('缺少生产中项目，无法记录部分完成')
      await finishRunningProduction({
        id: Number(task.running_item_id),
        finished_units: Number(finishPanel.finished_units || 0),
        finished_loose_g: Number(finishPanel.finished_loose_g || 0),
        consumed_input_g: Number(finishPanel.consumed_input_g || 0),
        partial: true,
        warehouse: finishPanel.warehouse || 'finished_goods',
      })
      message.value = '已记录部分完成'
    } else {
      const endpoint = productionTaskActionEndpoint(task, 'complete')
      const actualOutputG = Number(task.spec_g || 0) * Number(finishPanel.finished_units || 0) + Number(finishPanel.finished_loose_g || 0)
      await runProductionTaskAction(endpoint, {
        actual_input_qty: Number(finishPanel.consumed_input_g || 0),
        actual_output_qty: actualOutputG,
        exception_reason: finishPanel.note,
      })
      message.value = '完成本工序已提交'
    }
    closeFinishPanel()
    await load()
  } catch (err) {
    error.value = err.message || '完成操作失败'
  } finally {
    busyKey.value = ''
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    overview.value = await fetchProductionWorkstationOverview({ limit: 500 })
    if (selectedWorkstation.value && !sections.value.some((section) => section.workstation === selectedWorkstation.value)) {
      selectedWorkstation.value = ''
    }
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
  const id = Number(props.viewParams?.work_order_id || 0)
  if (id > 0) {
    executionHub.workOrderId = id
    executionHub.jobCardId = Number(props.viewParams?.job_card_id || 0)
    executionHub.focus = props.viewParams?.focus || (executionHub.jobCardId ? 'job_card' : 'summary')
    executionHub.open = true
  }
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
button.primary { background: #1f1f1f; border-color: #1f1f1f; color: #fff; }
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
  grid-template-columns: minmax(180px, 1.4fr) 90px 110px minmax(180px, 1.2fr);
  min-width: 610px;
  gap: 10px;
  align-items: center;
  padding: 10px;
  border-top: 1px solid #ebe7df;
}
.task-row.header {
  border-top: 0;
  background: #faf9f7;
  color: #666;
  font-size: 13px;
  font-weight: 700;
}
.task-title { display: grid; gap: 3px; min-width: 0; }
.task-title strong, .task-title small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.task-title small { color: #777; }
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
}
</style>
