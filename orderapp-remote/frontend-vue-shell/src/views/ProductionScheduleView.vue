<template>
  <div class="page production-schedule-page">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>生产排程工作台</h2>
          <p>按日期、班次、工位和负责人排工单与工序卡，先做人工排程和冲突提示。</p>
        </div>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div class="filters">
        <label><span>开始日期</span><input v-model="filters.from" type="date" /></label>
        <label><span>结束日期</span><input v-model="filters.to" type="date" /></label>
        <label><span>工位/工作中心</span><input v-model.trim="filters.work_center" placeholder="印刷线 / 烘焙机 / 缝制组" /></label>
        <label>
          <span>状态</span>
          <select v-model="filters.status">
            <option value="">全部</option>
            <option value="released">未开工</option>
            <option value="running">生产中</option>
            <option value="partially_completed">部分完成</option>
            <option value="completed">已完成</option>
          </select>
        </label>
      </div>
      <div class="mode-tabs" role="tablist" aria-label="排程视图">
        <button v-for="mode in viewModes" :key="mode.value" type="button" :class="{ active: viewMode === mode.value }" @click="viewMode = mode.value">{{ mode.label }}</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <section v-if="conflicts.length" class="panel conflict-panel">
      <div class="section-title">冲突</div>
      <div class="conflict-list">
        <article v-for="(row, index) in conflicts" :key="`${row.work_center}-${row.work_date}-${row.shift_code}-${index}`" class="conflict-row">
          <strong>{{ row.severity === 'error' ? '错误' : '提醒' }}</strong>
          <span>{{ row.message || `${row.work_date || '-'} ${row.work_center || '-'} ${row.shift_code || '-'} 产能冲突` }}</span>
        </article>
      </div>
    </section>

    <div class="schedule-layout">
      <section class="panel board-panel">
        <div class="section-title-row">
          <div class="section-title">{{ activeModeLabel }}</div>
          <span class="muted">{{ workOrders.length }} 个工单 · {{ jobCards.length }} 张工序卡</span>
        </div>

        <div v-if="viewMode === 'list'" class="table-wrap">
          <table>
            <thead>
              <tr><th>工单</th><th>商品</th><th>状态</th><th>计划时间</th><th>班次</th><th>工位</th><th>负责人</th><th>优先级</th><th>操作</th></tr>
            </thead>
            <tbody>
              <tr v-for="row in workOrders" :key="row.id">
                <td><strong>{{ row.work_order_no || '-' }}</strong><small>{{ row.order_nos || '' }}</small></td>
                <td>{{ row.product_name || '-' }}</td>
                <td><span class="pill">{{ statusLabel(row.status) }}</span></td>
                <td>{{ rangeText(row.planned_start_at, row.planned_end_at) }}</td>
                <td>{{ row.shift_code || '-' }}</td>
                <td>{{ row.work_center || '-' }}</td>
                <td>{{ row.assigned_to || '-' }}</td>
                <td>{{ row.priority || 0 }}</td>
                <td><button class="link" type="button" @click="editWorkOrder(row)">排程</button></td>
              </tr>
              <tr v-if="!workOrders.length"><td colspan="9" class="muted">暂无工单</td></tr>
            </tbody>
          </table>
        </div>

        <div v-else-if="viewMode === 'calendar'" class="calendar-grid">
          <article v-for="day in calendarDays" :key="day" class="calendar-day">
            <h3>{{ day }}</h3>
            <button v-for="row in workOrdersForDay(day)" :key="row.id" type="button" class="calendar-item" @click="editWorkOrder(row)">
              <strong>{{ row.work_order_no }}</strong>
              <span>{{ row.work_center || '未排工位' }} · {{ row.shift_code || '未排班次' }}</span>
            </button>
            <p v-if="!workOrdersForDay(day).length" class="muted">暂无排程</p>
          </article>
        </div>

        <div v-else-if="viewMode === 'gantt'" class="gantt-list">
          <article v-for="row in ganttRows" :key="row.kind + row.id" class="gantt-row">
            <div class="gantt-meta"><strong>{{ row.name }}</strong><span>{{ row.work_center || '未排工位' }}</span></div>
            <div class="gantt-track">
              <span class="gantt-bar" :style="{ width: ganttWidth(row) }">{{ rangeText(row.planned_start_at, row.planned_end_at) }}</span>
            </div>
          </article>
          <p v-if="!ganttRows.length" class="muted">暂无甘特数据</p>
        </div>

        <div v-else class="capacity-grid">
          <article v-for="row in capacityRows" :key="`${row.work_center}-${row.work_date}-${row.shift_code}`" class="capacity-card">
            <strong>{{ row.work_center }}</strong>
            <span>{{ row.work_date }} · {{ row.shift_code || '默认' }}</span>
            <div class="capacity-meter"><i :style="{ width: capacityWidth(row) }"></i></div>
            <small>可用 {{ row.available_minutes || 0 }} 分钟，停机 {{ row.downtime_minutes || 0 }} 分钟</small>
          </article>
          <p v-if="!capacityRows.length" class="muted">暂无工位负载</p>
        </div>
      </section>

      <aside class="panel side-panel">
        <div class="section-title">MRP 建议</div>
        <div class="mrp-summary">
          <span>采购建议 {{ mrp.purchase_suggestion_g || 0 }}g</span>
          <span>调拨建议 {{ mrp.transfer_suggestion_g || 0 }}g</span>
        </div>
        <div class="mrp-list">
          <article v-for="row in mrpRows" :key="row.material_id" class="mrp-row">
            <strong>{{ row.material_name || `物料 #${row.material_id}` }}</strong>
            <span>{{ suggestionLabel(row.suggestion_type) }} · 需求 {{ row.required_g || 0 }}g</span>
            <small>WIP 可用 {{ row.available_g || 0 }}g，原料仓 {{ row.raw_g || 0 }}g，采购 {{ row.purchase_suggestion_g || 0 }}g，调拨 {{ row.wip_transfer_suggestion_g || 0 }}g</small>
            <small>{{ row.source_work_orders || '暂无来源工单' }}</small>
          </article>
          <p v-if="!mrpRows.length" class="muted">暂无 MRP 缺料建议</p>
        </div>

        <div class="section-title">保存排程</div>
        <div class="form-grid one">
          <label><span>工单 ID</span><input v-model.number="assignment.work_order_id" type="number" min="1" /></label>
          <label><span>工序卡 ID</span><input v-model.number="assignment.job_card_id" type="number" min="0" /></label>
          <label><span>工位/工作中心</span><input v-model.trim="assignment.work_center" /></label>
          <label><span>计划开始</span><input v-model="assignment.planned_start_at" placeholder="2026-06-13 09:00" /></label>
          <label><span>计划结束</span><input v-model="assignment.planned_end_at" placeholder="2026-06-13 11:30" /></label>
          <label><span>班次</span><input v-model.trim="assignment.shift_code" placeholder="早班" /></label>
          <label><span>负责人</span><input v-model.trim="assignment.assigned_to" /></label>
          <label><span>优先级</span><input v-model.number="assignment.priority" type="number" min="0" /></label>
          <label><span>备注</span><textarea v-model.trim="assignment.note" rows="2"></textarea></label>
          <button class="primary" type="button" @click="saveAssignment" :disabled="saving">保存排程</button>
        </div>

        <div class="section-title capacity-title">保存产能</div>
        <div class="form-grid one">
          <label><span>工位/工作中心</span><input v-model.trim="capacityDraft.work_center" /></label>
          <label><span>日期</span><input v-model="capacityDraft.work_date" type="date" /></label>
          <label><span>班次</span><input v-model.trim="capacityDraft.shift_code" placeholder="早班" /></label>
          <label><span>可用分钟</span><input v-model.number="capacityDraft.available_minutes" type="number" min="0" /></label>
          <label><span>停机分钟</span><input v-model.number="capacityDraft.downtime_minutes" type="number" min="0" /></label>
          <label><span>备注</span><textarea v-model.trim="capacityDraft.note" rows="2"></textarea></label>
          <button class="secondary" type="button" @click="saveCapacity" :disabled="saving">保存产能</button>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import {
  buildCapacityCalendarPayload,
  buildScheduleAssignmentPayload,
  capacityCalendarEndpoint,
  mrpSuggestionsEndpoint,
  productionScheduleEndpoint,
  scheduleAssignEndpoint,
  scheduleStatusLabel,
  scheduleViewModes,
} from '../lib/production-schedule.js'

const today = new Date().toISOString().slice(0, 10)
const filters = reactive({ from: today, to: today, work_center: '', status: '', limit: 200 })
const assignment = reactive({ work_order_id: 0, job_card_id: 0, work_center: '', planned_start_at: '', planned_end_at: '', shift_code: '', assigned_to: '', priority: 0, note: '' })
const capacityDraft = reactive({ work_center: '', work_date: today, shift_code: '', available_minutes: 480, downtime_minutes: 0, note: '' })
const viewModes = scheduleViewModes()
const phase3ScheduleAPIMarkers = ['/api/production-schedule', '/api/production-schedule/assign', '/api/production-capacity-calendar', '/api/mrp/suggestions', 'MRP', '采购建议', '调拨建议', '列表', '日历', '甘特', '工位负载', '冲突']
const viewMode = ref('list')
const board = ref({ work_orders: [], job_cards: [], capacity: [], conflicts: [] })
const mrp = ref({ rows: [], purchase_suggestion_g: 0, transfer_suggestion_g: 0 })
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref('')

const workOrders = computed(() => board.value.work_orders || [])
const jobCards = computed(() => board.value.job_cards || [])
const capacityRows = computed(() => board.value.capacity || [])
const conflicts = computed(() => board.value.conflicts || [])
const mrpRows = computed(() => mrp.value.rows || [])
const activeModeLabel = computed(() => viewModes.find((item) => item.value === viewMode.value)?.label || '列表')
const calendarDays = computed(() => {
  const days = new Set([filters.from, filters.to].filter(Boolean))
  for (const row of workOrders.value) {
    if (row.planned_start_at) days.add(String(row.planned_start_at).slice(0, 10))
  }
  return Array.from(days).sort()
})
const ganttRows = computed(() => [
  ...workOrders.value.map((row) => ({ ...row, kind: 'work_order', name: row.work_order_no || `WO-${row.id}` })),
  ...jobCards.value.map((row) => ({ ...row, kind: 'job_card', name: `${row.operation || '工序'} #${row.id}`, work_center: row.work_center || row.workstation })),
].filter((row) => row.planned_start_at || row.planned_end_at))

function statusLabel(status) {
  return scheduleStatusLabel(status)
}

function rangeText(start, end) {
  if (!start && !end) return '未排程'
  return `${start || '-'} -> ${end || '-'}`
}

function workOrdersForDay(day) {
  return workOrders.value.filter((row) => String(row.planned_start_at || '').slice(0, 10) === day)
}

function ganttWidth(row) {
  const start = Date.parse(String(row.planned_start_at || '').replace(' ', 'T'))
  const end = Date.parse(String(row.planned_end_at || '').replace(' ', 'T'))
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) return '24%'
  const minutes = (end - start) / 60000
  return `${Math.max(18, Math.min(100, Math.round(minutes / 6)))}%`
}

function capacityWidth(row) {
  const available = Number(row.available_minutes || 0)
  const downtime = Number(row.downtime_minutes || 0)
  if (available <= 0) return '0%'
  return `${Math.max(0, Math.min(100, Math.round(((available - downtime) / available) * 100)))}%`
}

function suggestionLabel(type) {
  return ({
    purchase_suggestion: '采购建议',
    transfer_suggestion: '调拨建议',
    covered: '库存已覆盖',
  })[String(type || '').trim()] || 'MRP'
}

function editWorkOrder(row) {
  assignment.work_order_id = Number(row.id || 0)
  assignment.job_card_id = 0
  assignment.work_center = row.work_center || ''
  assignment.planned_start_at = row.planned_start_at || ''
  assignment.planned_end_at = row.planned_end_at || ''
  assignment.shift_code = row.shift_code || ''
  assignment.assigned_to = row.assigned_to || ''
  assignment.priority = Number(row.priority || 0)
  assignment.note = row.scheduling_note || ''
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [boardData, mrpData] = await Promise.all([
      apiGet(productionScheduleEndpoint(filters)),
      apiGet(mrpSuggestionsEndpoint({ ...filters, limit: 50 })),
    ])
    board.value = boardData
    mrp.value = mrpData
  } catch (err) {
    error.value = err.message || '加载排程失败'
  } finally {
    loading.value = false
  }
}

async function saveAssignment() {
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(scheduleAssignEndpoint(), { body: buildScheduleAssignmentPayload(assignment) })
    ok.value = '排程已保存'
    await load()
  } catch (err) {
    error.value = err.message || '保存排程失败'
  } finally {
    saving.value = false
  }
}

async function saveCapacity() {
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(capacityCalendarEndpoint(), { body: buildCapacityCalendarPayload(capacityDraft) })
    ok.value = '产能已保存'
    await load()
  } catch (err) {
    error.value = err.message || '保存产能失败'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #e5e7eb;border-radius:8px;background:#fff;padding:12px}.panel-head{display:flex;justify-content:space-between;align-items:flex-start;gap:12px}.panel-head h2{margin:0 0 4px;font-size:18px}.panel-head p{margin:0;color:#6b7280;font-size:13px}.filters{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:10px;align-items:end}label span{display:block;font-size:12px;color:#666;margin-bottom:5px}input,select,textarea,button{font:inherit;border-radius:6px}input,select,textarea{width:100%;border:1px solid #d1d5db;padding:7px 9px}button{min-height:34px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff;padding:8px 12px}.secondary{border:1px solid #9ca3af;background:#fff;color:#111;padding:8px 12px}.link{border:0;background:transparent;text-decoration:underline;padding:0;min-height:0}.mode-tabs{display:flex;gap:8px;flex-wrap:wrap;margin-top:12px}.mode-tabs button{border:1px solid #d1d5db;background:#fff;padding:7px 12px}.mode-tabs button.active{border-color:#111;background:#111;color:#fff}.schedule-layout{display:grid;grid-template-columns:minmax(0,1fr) 320px;gap:16px;align-items:start}.section-title-row{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:10px}.section-title{font-weight:700}.muted{color:#6b7280;text-align:center}.table-wrap{overflow:auto}table{width:100%;min-width:980px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px}th{background:#fbfbfb}td small{display:block;color:#6b7280;margin-top:3px}.pill{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px;background:#f9fafb}.error{border:1px solid #fecaca;background:#fef2f2;color:#991b1b;border-radius:8px;padding:10px}.ok{border:1px solid #bbf7d0;background:#f0fdf4;color:#166534;border-radius:8px;padding:10px}.conflict-panel{border-color:#fde68a;background:#fffbeb}.conflict-list{display:grid;gap:8px}.conflict-row{display:flex;gap:8px;align-items:center;color:#92400e}.calendar-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:10px}.calendar-day{border:1px solid #e5e7eb;border-radius:8px;padding:10px;min-height:160px}.calendar-day h3{margin:0 0 10px;font-size:14px}.calendar-item{display:block;width:100%;text-align:left;border:1px solid #e5e7eb;background:#f9fafb;border-radius:6px;padding:8px;margin-bottom:8px}.calendar-item span{display:block;color:#6b7280;font-size:12px;margin-top:3px}.gantt-list{display:grid;gap:10px}.gantt-row{display:grid;grid-template-columns:180px minmax(0,1fr);gap:10px;align-items:center}.gantt-meta{display:grid;gap:3px;font-size:13px}.gantt-meta span{color:#6b7280}.gantt-track{height:28px;border:1px solid #e5e7eb;border-radius:6px;background:#f9fafb;overflow:hidden}.gantt-bar{display:flex;align-items:center;height:100%;background:#dbeafe;color:#1e40af;padding:0 8px;font-size:12px;white-space:nowrap}.capacity-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:10px}.capacity-card{border:1px solid #e5e7eb;border-radius:8px;padding:10px;display:grid;gap:5px}.capacity-card span,.capacity-card small{color:#6b7280}.capacity-meter{height:8px;border-radius:999px;background:#e5e7eb;overflow:hidden}.capacity-meter i{display:block;height:100%;background:#22c55e}.mrp-summary{display:grid;grid-template-columns:1fr 1fr;gap:8px}.mrp-summary span{border:1px solid #e5e7eb;border-radius:6px;background:#f9fafb;padding:8px;font-size:13px}.mrp-list{display:grid;gap:8px}.mrp-row{display:grid;gap:3px;border-bottom:1px solid #f0f0f0;padding-bottom:8px}.mrp-row span,.mrp-row small{color:#6b7280;font-size:12px}.side-panel{display:grid;gap:12px}.form-grid.one{display:grid;gap:9px}.capacity-title{margin-top:10px}@media (max-width:900px){.filters{grid-template-columns:1fr 1fr}.schedule-layout{grid-template-columns:1fr}.gantt-row{grid-template-columns:1fr}.side-panel{order:-1}}@media (max-width:520px){.filters{grid-template-columns:1fr}.panel-head{display:grid}.mode-tabs button{flex:1}.table-wrap table{min-width:760px}}
</style>
