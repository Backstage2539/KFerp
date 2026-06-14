<template>
  <div class="page production-overview">
    <ProductionTopNav active-key="productionOverview" />

    <section class="toolbar">
      <div>
        <h2>生产视图</h2>
        <p>{{ overview.date || todayText }} · {{ overview.total_tasks || 0 }} 个生产任务</p>
      </div>
      <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
    </section>

    <div v-if="message" class="notice">{{ message }}</div>
    <div v-if="error" class="error">{{ error }}</div>

    <div class="section-title overview-title">今日生产总览</div>
    <section class="summary-grid">
      <article v-for="row in statusSummary" :key="row.key" class="metric">
        <span>{{ row.label }}</span>
        <strong>{{ row.count }}</strong>
      </article>
      <article v-if="!statusSummary.length" class="metric">
        <span>暂无任务</span>
        <strong>0</strong>
      </article>
    </section>

    <section class="panel quick-actions">
      <div>
        <div class="section-title">关键操作</div>
        <p class="muted">从总览直接进入下一步处理页面。</p>
      </div>
      <div class="actions quick-action-buttons">
        <button type="button" @click="openView('workOrders')">打开工单</button>
        <button type="button" @click="openView('stockOperations')">打开库存作业</button>
        <button type="button" @click="openView('qualityInspections')">打开质检</button>
        <button type="button" @click="openFirstAssignment" :disabled="!tasks.length">分配工位 / 调整优先级</button>
      </div>
    </section>

    <div class="overview-layout">
      <section class="panel">
        <div class="section-title-row">
          <div class="section-title">待处理</div>
          <span class="muted">{{ pendingTasks.length }} 项</span>
        </div>
        <div class="task-list">
          <article v-for="task in pendingTasks" :key="taskKey(task)" class="task-row">
            <div class="task-main">
              <strong>{{ taskTitle(task) }}</strong>
              <span>{{ task.work_order_no || '-' }} · {{ task.workstation || task.work_center || '未分配工位' }}</span>
              <small>{{ task.readiness_label || '待处理' }} · {{ task.planned_start_at || '未排时间' }} · P{{ task.priority || 0 }} · {{ task.next_handler || '-' }}</small>
            </div>
            <div class="actions">
              <button type="button" @click="openWorkOrder(task)">工单</button>
              <button type="button" @click="openAssignment(task)">分配</button>
              <button type="button" @click="openQuality(task)">质检</button>
            </div>
          </article>
          <p v-if="!pendingTasks.length" class="muted">暂无待处理任务</p>
        </div>
      </section>

      <section class="panel">
        <div class="section-title-row">
          <div class="section-title">执行中</div>
          <span class="muted">{{ runningTasks.length }} 项</span>
        </div>
        <div class="task-list">
          <article v-for="task in runningTasks" :key="taskKey(task)" class="task-row running">
            <div class="task-main">
              <strong>{{ taskTitle(task) }}</strong>
              <span>{{ task.work_order_no || '-' }} · {{ task.workstation || task.work_center || '未分配工位' }}</span>
              <small>{{ task.readiness_label || '执行中' }} · {{ task.planned_start_at || '未排时间' }} · {{ task.next_handler || '-' }}</small>
            </div>
            <div class="actions">
              <button type="button" @click="openWorkOrder(task)">工单</button>
              <button type="button" @click="openStockOperations(task)">库存作业</button>
              <button type="button" @click="openAssignment(task)">优先级</button>
            </div>
          </article>
          <p v-if="!runningTasks.length" class="muted">暂无执行中任务</p>
        </div>
      </section>

      <section class="panel attention-panel">
        <div class="section-title-row">
          <div class="section-title">异常</div>
          <span class="muted">{{ blockedTasks.length }} 项</span>
        </div>
        <div class="task-list">
          <article v-for="task in blockedTasks" :key="taskKey(task)" class="task-row blocked">
            <div class="task-main">
              <strong>{{ task.blocking_reason || '待处理异常' }}</strong>
              <span>{{ taskTitle(task) }} · {{ task.work_order_no || '-' }}</span>
              <small>{{ task.next_handler || '-' }} 处理 · {{ task.workstation || task.work_center || '未分配工位' }}</small>
            </div>
            <div class="actions">
              <button type="button" @click="openWorkOrder(task)">工单</button>
              <button type="button" @click="openStockOperations(task)">库存作业</button>
              <button type="button" @click="openQuality(task)">质检</button>
            </div>
          </article>
          <p v-if="!blockedTasks.length" class="muted">暂无异常阻塞</p>
        </div>
      </section>

      <aside class="panel side-panel">
        <div class="section-title">工位负载</div>
        <div class="load-list">
          <article v-for="row in workstationLoad" :key="row.workstation" class="load-row">
            <div>
              <strong>{{ row.workstation }}</strong>
              <span>{{ row.total_tasks }} 项 · {{ row.load_minutes || 0 }} 分钟</span>
            </div>
            <div class="load-counts">
              <span>待 {{ row.pending_tasks || 0 }}</span>
              <span>中 {{ row.running_tasks || 0 }}</span>
              <span :class="{ warn: row.blocked_tasks }">阻 {{ row.blocked_tasks || 0 }}</span>
            </div>
            <small>当前：{{ row.current_task || '-' }}</small>
            <small>下一件：{{ row.next_task || '-' }}</small>
            <small v-if="row.blocking_reason" class="warn-text">{{ row.blocking_reason }}</small>
          </article>
          <p v-if="!workstationLoad.length" class="muted">暂无工位负载</p>
        </div>

        <div class="section-title priority-title">优先级</div>
        <div class="tag-list">
          <span v-for="row in prioritySummary" :key="row.key">P{{ priorityNumber(row.label) }} · {{ row.count }}</span>
          <span v-if="!prioritySummary.length">暂无</span>
        </div>

        <div class="section-title priority-title">阻塞原因</div>
        <div class="tag-list">
          <span v-for="row in blockedSummary" :key="row.key">{{ row.label }} · {{ row.count }}</span>
          <span v-if="!blockedSummary.length">暂无</span>
        </div>
      </aside>
    </div>

    <section v-if="assignment.open" class="panel assignment-panel">
      <div class="section-title-row">
        <div>
          <div class="section-title">分配工位 / 调整优先级</div>
          <p class="muted">{{ assignment.title }}</p>
        </div>
        <button type="button" class="secondary" @click="assignment.open = false">关闭</button>
      </div>
      <div class="form-grid">
        <label><span>工位/工作中心</span><input v-model.trim="assignment.work_center" /></label>
        <label><span>负责人</span><input v-model.trim="assignment.assigned_to" /></label>
        <label><span>优先级</span><input v-model.number="assignment.priority" type="number" min="0" /></label>
        <label><span>备注</span><input v-model.trim="assignment.note" /></label>
        <button class="primary" type="button" @click="saveAssignment" :disabled="saving">保存</button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { assignProductionSchedule, fetchProductionWorkstationOverview } from '../api/production.js'
import ProductionTopNav from '../components/ProductionTopNav.vue'
import { stockOperationContextParams, taskTitle } from '../lib/production-workstation.js'

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const message = ref('')
const todayText = new Date().toISOString().slice(0, 10)
const overview = ref({
  total_tasks: 0,
  status_summary: [],
  blocked_summary: [],
  priority_summary: [],
  workstation_load: [],
  tasks: [],
})
const assignment = reactive({
  open: false,
  title: '',
  work_order_id: 0,
  job_card_id: 0,
  work_center: '',
  assigned_to: '',
  priority: 0,
  note: '',
})

const tasks = computed(() => overview.value.tasks || [])
const statusSummary = computed(() => overview.value.status_summary || [])
const blockedSummary = computed(() => overview.value.blocked_summary || [])
const prioritySummary = computed(() => overview.value.priority_summary || [])
const workstationLoad = computed(() => overview.value.workstation_load || [])
const pendingTasks = computed(() => tasks.value.filter((task) => task.status_label === '待处理'))
const runningTasks = computed(() => tasks.value.filter((task) => task.status_label === '执行中'))
const blockedTasks = computed(() => tasks.value.filter((task) => task.is_blocked || task.status_label === '异常'))

function taskKey(task) {
  return `${task.job_card_id || 0}:${task.work_order_id || 0}`
}

function priorityNumber(label) {
  return String(label || '').replace(/^P/, '')
}

function openView(key, params = {}) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key, params } }))
}

function openWorkOrder(task) {
  openView('workOrders', { work_order_id: task.work_order_id })
}

function openStockOperations(task) {
  openView('stockOperations', stockOperationContextParams(task))
}

function openQuality(task) {
  openView('qualityInspections', { work_order_id: task.work_order_id, job_card_id: task.job_card_id })
}

function openAssignment(task) {
  assignment.open = true
  assignment.title = `${taskTitle(task)} · ${task.work_order_no || ''}`
  assignment.work_order_id = Number(task.work_order_id || 0)
  assignment.job_card_id = Number(task.job_card_id || 0)
  assignment.work_center = task.work_center || task.workstation || ''
  assignment.assigned_to = task.assigned_to || task.next_handler || ''
  assignment.priority = Number(task.priority || 0)
  assignment.note = task.scheduling_note || ''
}

function openFirstAssignment() {
  const task = tasks.value[0]
  if (task) openAssignment(task)
}

async function saveAssignment() {
  saving.value = true
  error.value = ''
  message.value = ''
  try {
    await assignProductionSchedule({
      work_order_id: assignment.work_order_id,
      job_card_id: assignment.job_card_id,
      work_center: assignment.work_center,
      assigned_to: assignment.assigned_to,
      priority: Number(assignment.priority || 0),
      note: assignment.note,
    })
    assignment.open = false
    message.value = '已保存排程分配'
    await load()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    overview.value = await fetchProductionWorkstationOverview({ limit: 500 })
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.page { padding: 20px; color: #252525; }
.toolbar, .panel {
  border: 1px solid #e2ded7;
  border-radius: 8px;
  background: #fff;
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
  margin-bottom: 14px;
}
h2 { margin: 0; font-size: 22px; line-height: 1.25; letter-spacing: 0; }
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
.notice, .error {
  padding: 10px 12px;
  border-radius: 8px;
  margin-bottom: 12px;
}
.notice { border: 1px solid #b7dfc4; background: #effaf2; color: #175c2f; }
.error { border: 1px solid #efb9b9; background: #fff2f2; color: #9d2424; }
.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 14px;
}
.metric {
  border: 1px solid #e2ded7;
  border-radius: 8px;
  background: #fff;
  padding: 12px;
}
.metric span { display: block; color: #666; font-size: 13px; }
.metric strong { display: block; margin-top: 6px; font-size: 28px; line-height: 1; }
.overview-title { margin: 0 0 8px; }
.quick-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.quick-action-buttons { justify-content: flex-end; }
.overview-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1fr) 320px;
  gap: 12px;
  align-items: start;
}
.panel { padding: 14px; min-width: 0; }
.section-title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 10px;
}
.section-title { font-weight: 800; font-size: 16px; line-height: 1.25; }
.muted { color: #777; font-size: 13px; }
.task-list, .load-list {
  display: grid;
  gap: 10px;
}
.task-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 10px;
  border: 1px solid #ebe7df;
  border-radius: 8px;
  padding: 10px;
  background: #fff;
}
.task-row.running { border-color: #bdd8f0; }
.task-row.blocked { border-color: #edc4c4; background: #fffafa; }
.task-main { display: grid; gap: 3px; min-width: 0; }
.task-main strong, .task-main span, .task-main small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.task-main span { color: #444; }
.task-main small { color: #777; font-size: 12px; }
.actions { display: flex; gap: 6px; flex-wrap: wrap; }
.actions button { min-height: 30px; font-size: 12px; padding: 4px 8px; }
.side-panel { align-self: stretch; }
.load-row {
  display: grid;
  gap: 6px;
  border: 1px solid #ebe7df;
  border-radius: 8px;
  padding: 10px;
}
.load-row div:first-child {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.load-row span, .load-row small { color: #666; font-size: 12px; }
.load-counts { display: flex; gap: 6px; flex-wrap: wrap; }
.load-counts span, .tag-list span {
  border: 1px solid #ded8d0;
  border-radius: 999px;
  padding: 3px 7px;
  background: #faf9f7;
}
.warn, .warn-text { color: #9d2424 !important; }
.priority-title { margin-top: 16px; }
.tag-list { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 8px; }
.tag-list span { color: #444; font-size: 12px; }
.assignment-panel { margin-top: 12px; }
.form-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 10px;
  align-items: end;
}
label { display: grid; gap: 5px; color: #555; font-size: 13px; }
input {
  min-height: 34px;
  border: 1px solid #cfc8bf;
  border-radius: 8px;
  padding: 6px 8px;
  font: inherit;
}

@media (max-width: 1180px) {
  .overview-layout { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .side-panel { grid-column: 1 / -1; }
  .form-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 720px) {
  .page { padding: 14px; }
  .toolbar, .quick-actions { align-items: stretch; flex-direction: column; }
  .quick-action-buttons { justify-content: flex-start; }
  .summary-grid, .overview-layout, .form-grid { grid-template-columns: 1fr; }
}
</style>
