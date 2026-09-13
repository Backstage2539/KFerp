<template>
  <section class="roster-workspace" aria-label="生产排班工作区">
    <ProductionReturnLink :source="viewParams.return_navigation" />
    <header class="workspace-header">
      <div>
        <div class="eyebrow">生产管理 · 每周出勤与工位负责人</div>
        <h1>生产排班</h1>
        <p>只安排员工哪天上班，系统自动确定当天每个工位的唯一负责人。</p>
      </div>
      <button class="secondary" type="button" @click="navigate('productionManual')">操作说明</button>
    </header>

    <div class="summary-strip">
      <div class="calendar-mark">周</div>
      <div><strong>{{ weekTitle }}</strong><p>{{ savedSummary }}</p></div>
      <div class="week-actions"><button type="button" @click="moveWeek(-7)">‹ 上一周</button><button type="button" @click="goCurrentWeek">本周</button><button type="button" @click="moveWeek(7)">下一周 ›</button></div>
    </div>

    <div v-if="message" class="notice success" role="status">{{ message }}</div>
    <div v-if="error" class="notice warning" role="alert">{{ error }}</div>

    <div class="workspace-layout">
      <main>
        <nav class="view-tabs" aria-label="排班视图">
          <button type="button" :class="{ active: activeView === 'employees' }" @click="activeView = 'employees'">员工排班</button>
          <button type="button" :class="{ active: activeView === 'workstations' }" @click="activeView = 'workstations'">工位安排</button>
        </nav>

        <section v-if="activeView === 'employees'" class="board-panel">
          <div class="section-heading"><div><h2>员工 × 七天</h2><p>点击单元格切换上班、休息、未排班。未排班不会参与自动派工。</p></div><button class="secondary" type="button" :disabled="loading" @click="copyPreviousWeek">复制上一周</button></div>
          <div class="bulk-bar">
            <label>批量日期<select v-model="bulkDay"><option value="all">整周</option><option v-for="day in days" :key="day.date" :value="day.date">{{ day.label }} {{ day.short }}</option></select></label>
            <label>设置为<select v-model="bulkStatus"><option value="working">上班</option><option value="off">休息</option><option value="unplanned">未排班</option></select></label>
            <button class="secondary" type="button" :disabled="!selectedEmployees.length" @click="applyBulk">应用到已选 {{ selectedEmployees.length }} 人</button>
          </div>
          <div v-if="loading" class="empty">正在读取排班…</div>
          <div v-else class="table-wrap">
            <table class="roster-table">
              <thead><tr><th class="employee-col"><label class="employee-check"><input type="checkbox" :checked="allEmployeesSelected" @change="toggleAllEmployees($event.target.checked)" />员工</label></th><th v-for="day in days" :key="day.date"><strong>{{ day.label }}</strong><small>{{ day.short }}</small></th></tr></thead>
              <tbody><tr v-for="employee in week.employees" :key="employee.id"><th><label class="employee-check"><input v-model="selectedEmployees" type="checkbox" :value="employee.id" />{{ employee.name }}</label></th><td v-for="day in days" :key="day.date" :data-day="day.label"><button type="button" class="attendance-cell" :class="attendanceClass(entryStatus(employee.id, day.date))" @click="cycleEntry(employee.id, day.date)"><strong>{{ attendanceLabel(entryStatus(employee.id, day.date)) }}</strong><small>{{ attendanceHint(entryStatus(employee.id, day.date)) }}</small></button></td></tr><tr v-if="!week.employees.length"><td colspan="8" class="empty">暂无启用的内部员工</td></tr></tbody>
            </table>
          </div>
        </section>

        <section v-else class="board-panel">
          <div class="section-heading"><div><h2>工位 × 七天</h2><p>系统按主负责人和替补顺序自动安排；临时换人只影响当天。</p></div><button class="secondary" type="button" @click="navigate('productionConfig', { tab: 'workstations' })">配置工位人员</button></div>
          <div v-if="loading" class="empty">正在计算工位负责人…</div>
          <div v-else class="table-wrap">
            <table class="station-table">
              <thead><tr><th class="employee-col">工位</th><th v-for="day in days" :key="day.date"><strong>{{ day.label }}</strong><small>{{ day.short }}</small></th></tr></thead>
              <tbody><tr v-for="station in stationRows" :key="station.id"><th><strong>{{ station.name }}</strong><small>{{ station.staffingReady ? '自动派工已配置' : '待补人员配置' }}</small></th><td v-for="day in days" :key="day.date" :data-day="day.label"><div class="owner-cell" :class="workstationOwnerState(assignmentFor(station.id, day.date))"><strong>{{ assignmentFor(station.id, day.date)?.employee_name || '无人值班' }}</strong><small>{{ ownerSourceLabel(assignmentFor(station.id, day.date)) }}</small><select :value="overrideEmployee(station.id, day.date)" @change="setOverride(station.id, day.date, $event.target.value)"><option value="0">恢复自动安排</option><option v-for="person in eligibleWorkingEmployees(station, day.date)" :key="person.id" :value="person.id">临时：{{ person.name }}</option></select><em v-if="assignmentFor(station.id, day.date)?.handover_count">{{ assignmentFor(station.id, day.date).handover_count }} 项执行中任务待交接</em></div></td></tr><tr v-if="!stationRows.length"><td colspan="8" class="empty">暂无启用工位</td></tr></tbody>
            </table>
          </div>
        </section>
      </main>

      <aside class="review-column">
        <section><h2>排班核对</h2><div class="review-card"><span>未排班</span><strong>{{ issueCount('unplanned') }}</strong></div><div class="review-card warning"><span>无人值班</span><strong>{{ unattendedCount }}</strong></div><div class="review-card warning"><span>失效调整</span><strong>{{ issueCount('invalid_override') }}</strong></div><div class="review-card"><span>待交接</span><strong>{{ handoverCount }}</strong></div></section>
        <section v-if="preview" class="preview-panel"><h2>保存前影响</h2><p>将更新 {{ preview.entries.length }} 个出勤单元，影响 {{ preview.affected_workstation_count }} 个工位、{{ preview.affected_task_count }} 项待执行任务。</p><div v-for="issue in preview.issues.slice(0, 8)" :key="`${issue.code}:${issue.workstation_id}:${issue.work_date}`" class="issue-line">{{ issue.message }}</div></section>
        <section class="help-panel"><h2>自动安排顺序</h2><ol><li>当天有效的人工调整</li><li>当天上班的主负责人</li><li>按顺序选择上班的替补</li></ol><p>同一员工负责多个工位属于正常安排。</p></section>
      </aside>
    </div>

    <footer class="roster-footer"><div><strong>{{ footerTitle }}</strong><small>{{ footerHint }}</small></div><button v-if="preview" class="secondary" type="button" @click="preview = null">继续调整</button><button class="primary" type="button" :disabled="saving || !dirty" @click="preview ? saveRoster() : reviewRoster()">{{ preview ? '确认保存排班' : '保存排班' }}</button></footer>
  </section>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import ProductionReturnLink from '../components/ProductionReturnLink.vue'
import { apiGet, apiSend } from '../api/client'
import { formatLocalDateInput } from '../lib/local-date.js'
import { attendanceClass, attendanceLabel, buildWeekDays, copyPreviousWeekEntries, nextAttendance, rosterSavePayload, workstationOwnerState } from '../lib/production-roster.js'

const props = defineProps({ viewParams: { type: Object, default: () => ({}) } })
const loading = ref(false), saving = ref(false), error = ref(''), message = ref('')
const activeView = ref('employees'), preview = ref(null), requestID = ref('')
const week = reactive({ week_start: '', week_end: '', version: 0, employees: [], entries: [], overrides: [], assignments: [], issues: [] })
const workstations = ref([]), selectedEmployees = ref([]), bulkDay = ref('all'), bulkStatus = ref('working'), baseline = ref('')
const days = computed(() => buildWeekDays(week.week_start || formatLocalDateInput(new Date())))
const entries = reactive({}), overrides = reactive({})
const dirty = computed(() => snapshot() !== baseline.value)
const weekTitle = computed(() => `${days.value[0]?.date || '-'} 至 ${days.value[6]?.date || '-'}`)
const stationRows = computed(() => workstations.value.filter(row => row.status === 'active').map(row => ({ id: Number(row.id), name: row.name, primaryEmployeeID: Number(row.primary_employee_id || 0), backupEmployeeIDs: (row.backup_employee_ids || []).map(Number), staffingReady: Boolean(row.staffing_ready) })))
const allEmployeesSelected = computed(() => week.employees.length > 0 && selectedEmployees.value.length === week.employees.length)
const unattendedCount = computed(() => week.assignments.filter(row => row.unattended && !row.override_invalid).length)
const handoverCount = computed(() => week.assignments.reduce((sum, row) => sum + Number(row.handover_count || 0), 0))
const savedSummary = computed(() => dirty.value ? '当前有未保存的排班调整' : `排班版本 V${week.version || 0} · 工位安排随出勤自动更新`)
const footerTitle = computed(() => preview.value
  ? '已完成保存前核对'
  : dirty.value
    ? '有未保存的排班调整'
    : Number(week.version || 0) > 0
      ? '排班已保存'
      : '本周排班尚未保存')
const footerHint = computed(() => preview.value
  ? '确认后立即影响本周工位负责人，执行中任务仍需交接。'
  : Number(week.version || 0) > 0
    ? '保存前会显示受影响工位和任务数量。'
    : '首次保存后，系统才会按本周出勤自动确定工位负责人。')

function key(employeeID, date) { return `${employeeID}:${date}` }
function stationKey(stationID, date) { return `${stationID}:${date}` }
function entryStatus(employeeID, date) { return entries[key(employeeID, date)] || 'unplanned' }
function attendanceHint(status) { return ({ working: '参与派工', off: '当天休息', unplanned: '不参与派工' })[status] }
function cycleEntry(employeeID, date) { entries[key(employeeID, date)] = nextAttendance(entryStatus(employeeID, date)); preview.value = null; requestID.value = '' }
function toggleAllEmployees(checked) { selectedEmployees.value = checked ? week.employees.map(row => row.id) : [] }
function applyBulk() { for (const id of selectedEmployees.value) for (const day of days.value) if (bulkDay.value === 'all' || bulkDay.value === day.date) entries[key(id, day.date)] = bulkStatus.value; preview.value = null; requestID.value = '' }
function assignmentFor(stationID, date) { return week.assignments.find(row => Number(row.workstation_id) === Number(stationID) && row.work_date === date) }
function overrideEmployee(stationID, date) { return Number(overrides[stationKey(stationID, date)] || 0) }
function setOverride(stationID, date, employeeID) { const k = stationKey(stationID, date); if (Number(employeeID)) overrides[k] = Number(employeeID); else delete overrides[k]; preview.value = null; requestID.value = '' }
function eligibleWorkingEmployees(station, date) { const eligible = new Set([station.primaryEmployeeID, ...station.backupEmployeeIDs]); return week.employees.filter(row => eligible.has(Number(row.id)) && entryStatus(row.id, date) === 'working') }
function ownerSourceLabel(row) { if (!row) return '尚未计算'; if (row.override_invalid) return '人工调整已失效'; if (row.unattended) return row.reason || '无人值班'; return ({ override: '人工调整', primary: '主负责人', backup: '替补自动接班' })[row.source] || '自动安排' }
function issueCount(code) { if (code === 'unplanned') return week.employees.reduce((sum, row) => sum + days.value.filter(day => entryStatus(row.id, day.date) === 'unplanned').length, 0); return week.issues.filter(row => row.code === code).length }
function currentEntries() { return week.employees.flatMap(employee => days.value.map(day => ({ employee_id: Number(employee.id), work_date: day.date, status: entryStatus(employee.id, day.date) }))) }
function currentOverrides() { return Object.entries(overrides).map(([k, employeeID]) => { const [workstationID, workDate] = k.split(':'); return { workstation_id: Number(workstationID), work_date: workDate, employee_id: Number(employeeID) } }) }
function snapshot() { return JSON.stringify({ entries: currentEntries(), overrides: currentOverrides().sort((a, b) => `${a.workstation_id}:${a.work_date}`.localeCompare(`${b.workstation_id}:${b.work_date}`)) }) }
function applyWeek(data) { Object.assign(week, data || {}); for (const k of Object.keys(entries)) delete entries[k]; for (const employee of week.employees || []) for (const day of buildWeekDays(week.week_start)) entries[key(employee.id, day.date)] = 'unplanned'; for (const row of week.entries || []) entries[key(row.employee_id, row.work_date)] = row.status; for (const k of Object.keys(overrides)) delete overrides[k]; for (const row of week.overrides || []) overrides[stationKey(row.workstation_id, row.work_date)] = Number(row.employee_id); baseline.value = snapshot() }
async function load(start = week.week_start || props.viewParams.week_start || '') { loading.value = true; error.value = ''; preview.value = null; try { const [data, stationData] = await Promise.all([apiGet(`/api/production-roster${start ? `?week_start=${encodeURIComponent(start)}` : ''}`), apiGet('/api/manufacturing-workstations')]); workstations.value = stationData?.rows || []; applyWeek(data) } catch (err) { error.value = err.message || '排班加载失败' } finally { loading.value = false } }
async function copyPreviousWeek() { const date = new Date(`${week.week_start}T12:00:00`); date.setDate(date.getDate() - 7); try { const previous = await apiGet(`/api/production-roster?week_start=${formatLocalDateInput(date)}`); for (const row of copyPreviousWeekEntries(previous.entries || [], week.week_start)) entries[key(row.employee_id, row.work_date)] = row.status; preview.value = null; message.value = '已复制上一周，请核对后保存' } catch (err) { error.value = err.message } }
function payload() { if (!requestID.value) requestID.value = crypto.randomUUID(); return rosterSavePayload(week, currentEntries(), currentOverrides(), requestID.value) }
async function reviewRoster() { saving.value = true; error.value = ''; try { preview.value = await apiSend('/api/production-roster/preview', { body: payload() }); week.assignments = preview.value.assignments || []; week.issues = preview.value.issues || []; activeView.value = 'workstations' } catch (err) { error.value = err.message || '排班核对失败' } finally { saving.value = false } }
async function saveRoster() { saving.value = true; error.value = ''; try { const saved = await apiSend('/api/production-roster/save', { body: payload() }); applyWeek(saved); preview.value = null; requestID.value = ''; message.value = `本周排班 V${saved.version} 已保存，工位负责人已自动更新` } catch (err) { error.value = err.code === 'version_conflict' ? `${err.message || '排班版本已变化'}；当前调整仍保留，请核对后刷新本周排班` : (err.message || '排班保存失败') } finally { saving.value = false } }
function moveWeek(daysToMove) { if (dirty.value && !window.confirm('当前排班尚未保存，确定切换周次吗？')) return; const date = new Date(`${week.week_start || formatLocalDateInput(new Date())}T12:00:00`); date.setDate(date.getDate() + daysToMove); load(formatLocalDateInput(date)) }
function goCurrentWeek() { load(formatLocalDateInput(new Date())) }
function navigate(view, params = {}) { window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: view, params, returnNavigation: { key: 'productionSchedule', params: { week_start: week.week_start }, label: '返回生产排班' } } })) }
function unload(event) { if (!dirty.value) return; event.preventDefault(); event.returnValue = '' }
onMounted(() => { window.addEventListener('beforeunload', unload); load() })
onBeforeUnmount(() => window.removeEventListener('beforeunload', unload))
</script>

<style scoped>
*{box-sizing:border-box}.roster-workspace{min-height:100%;padding:24px 24px 110px;background:#fff;color:#223d2f;font-size:14px}.workspace-header{display:flex;justify-content:space-between;gap:20px;align-items:flex-start;margin-bottom:18px}.eyebrow{color:#7b887f;font-size:12px;margin-bottom:5px}.workspace-header h1{margin:0;font-size:28px;color:#18372a}.workspace-header p{margin:8px 0 0;color:#748078}.summary-strip{display:flex;align-items:center;gap:14px;padding:14px 17px;border:1px solid #dceee3;border-radius:10px;background:#edf8f1}.calendar-mark{display:grid;place-items:center;width:42px;height:42px;border-radius:11px;background:#2f8f5b;color:#fff;font-weight:800}.summary-strip strong{font-size:18px}.summary-strip p{margin:4px 0 0;color:#657b6b;font-size:12px}.week-actions{margin-left:auto;display:flex}.week-actions button{border:1px solid #cfe0d5;background:#fff;color:#366148;padding:8px 11px}.week-actions button+button{border-left:0}.workspace-layout{display:grid;grid-template-columns:minmax(0,1fr) 290px;gap:22px;margin-top:18px}.view-tabs{display:flex;border-bottom:1px solid #e1e8e3;margin-bottom:16px;gap:22px}.view-tabs button{border:0;border-bottom:3px solid transparent;background:#fff;padding:10px 2px;color:#7b877f;font:inherit}.view-tabs button.active{color:#237748;border-bottom-color:#2f8f5b;font-weight:700}.board-panel{min-width:0}.section-heading{display:flex;justify-content:space-between;gap:14px;align-items:end;margin-bottom:13px}.section-heading h2,.review-column h2{font-size:16px;margin:0}.section-heading p{margin:5px 0 0;color:#7b897f;font-size:12px}.bulk-bar{display:flex;align-items:end;gap:10px;padding:11px;background:#f7faf8;border:1px solid #e1e9e3;border-radius:8px;margin-bottom:12px}.bulk-bar label{display:grid;gap:4px;color:#7b887f;font-size:11px}.bulk-bar select{min-width:120px}.table-wrap{overflow:auto;border:1px solid #e1e7e2;border-radius:8px}.roster-table,.station-table{width:100%;min-width:860px;border-collapse:collapse;table-layout:fixed}.roster-table th,.roster-table td,.station-table th,.station-table td{border-bottom:1px solid #e7ece8;border-right:1px solid #edf0ee;padding:8px;vertical-align:top}.roster-table thead th,.station-table thead th{background:#f4f7f5;text-align:center;color:#65766a}.roster-table thead small,.station-table thead small,.station-table tbody th small{display:block;font-size:11px;color:#8a958d;margin-top:3px}.employee-col{width:135px;text-align:left!important}.employee-check{display:flex;gap:8px;align-items:center;font-weight:600}.employee-check input{accent-color:#2f8f5b}.attendance-cell{width:100%;min-height:54px;border:1px solid transparent;border-radius:7px;background:#f7f8f7;color:#77837b}.attendance-cell strong,.attendance-cell small{display:block}.attendance-cell small{font-size:10px;margin-top:3px}.attendance-cell.working{background:#edf8f1;border-color:#cbe5d4;color:#247847}.attendance-cell.off{background:#f2f3f2;color:#69736d}.attendance-cell.unplanned{background:#fff8ea;border-color:#f0dfba;color:#a87627}.owner-cell{display:grid;gap:4px;min-height:82px;border-radius:7px;padding:7px;background:#edf8f1;color:#247847}.owner-cell small{font-size:10px}.owner-cell select{width:100%;min-width:0;font-size:11px}.owner-cell.unattended,.owner-cell.invalid_override{background:#fff5e5;color:#a46c1e}.owner-cell.invalid_override{border:1px solid #e5b557}.owner-cell em{font-size:10px;color:#a46c1e;font-style:normal}.review-column{border-left:1px solid #e4eae6;padding-left:20px;display:grid;align-content:start;gap:22px}.review-card{display:flex;justify-content:space-between;padding:12px 0;border-bottom:1px solid #e8ece9}.review-card strong{color:#2d8050}.review-card.warning strong{color:#be7c1b}.preview-panel,.help-panel{border:1px solid #dae9de;border-radius:9px;background:#f8fcf9;padding:14px}.preview-panel p,.help-panel p,.help-panel li{font-size:12px;line-height:1.7;color:#6f8074}.help-panel ol{padding-left:18px}.issue-line{padding:6px 0;border-top:1px solid #e7eee9;font-size:11px;color:#9b6a24}.notice{padding:10px 13px;border-radius:8px;margin-top:12px}.notice.success{background:#edf8f1;color:#267b48}.notice.warning{background:#fff7e8;color:#9f6a21}.primary,.secondary,select{min-height:36px;border-radius:7px;padding:7px 11px;font:inherit}.primary{background:#2f8f5b;color:#fff;border:1px solid #2f8f5b}.secondary,select{background:#fff;color:#405b49;border:1px solid #ced9d1}.primary:disabled,.secondary:disabled{opacity:.45}.roster-footer{position:sticky;bottom:0;margin:24px -24px -110px;padding:15px 24px;background:#fffffff5;border-top:1px solid #e1e9e3;display:flex;align-items:center;gap:10px;z-index:8}.roster-footer>div{margin-right:auto}.roster-footer small{display:block;color:#829087;font-size:11px;margin-top:5px}.empty{text-align:center;padding:34px;color:#819087}
@media(max-width:1050px){.workspace-layout{grid-template-columns:1fr}.review-column{border-left:0;padding-left:0;border-top:1px solid #e3e9e5;padding-top:18px;grid-template-columns:repeat(3,1fr)}.review-column>section:first-child{grid-column:1/-1}.review-column>section:first-child{display:grid;grid-template-columns:repeat(4,1fr);gap:10px}.review-column>section:first-child h2{grid-column:1/-1}.review-card{border:1px solid #e5ebe7;border-radius:8px;padding:10px}}
@media(max-width:640px){.roster-workspace{padding:16px 12px 128px}.workspace-header{align-items:flex-start}.workspace-header h1{font-size:24px}.workspace-header p{font-size:12px}.summary-strip{align-items:flex-start;flex-wrap:wrap}.week-actions{width:100%;margin:0}.week-actions button{flex:1}.bulk-bar{flex-wrap:wrap}.bulk-bar label{flex:1}.table-wrap{border:0;overflow:visible}.roster-table,.station-table{min-width:0;display:block}.roster-table thead,.station-table thead{display:none}.roster-table tbody,.station-table tbody{display:grid;gap:12px}.roster-table tr,.station-table tr{display:grid;grid-template-columns:1fr 1fr;border:1px solid #e0e8e2;border-radius:10px;padding:9px}.roster-table th,.station-table th{grid-column:1/-1;border:0;padding:7px 8px;font-size:15px}.roster-table td,.station-table td{border:0;padding:6px}.roster-table td::before,.station-table td::before{content:attr(data-day);display:block;font-size:10px;color:#87938b;margin-bottom:3px}.attendance-cell{min-height:58px}.owner-cell{min-height:96px}.review-column{grid-template-columns:1fr}.review-column>section:first-child{grid-template-columns:1fr 1fr}.roster-footer{margin:20px -12px -128px;padding:12px;flex-wrap:wrap}.roster-footer>div{flex-basis:100%}.roster-footer button{flex:1}}
</style>
