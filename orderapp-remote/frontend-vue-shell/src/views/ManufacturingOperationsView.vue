<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>工序</h2>
        <div class="actions">
          <button class="secondary" type="button" @click="newOperation">新建工序</button>
          <button class="secondary" type="button" @click="loadOperations" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <div class="grid master-data-layout">
      <section class="panel master-list-panel operation-list-panel">
        <div class="section-title">工序列表</div>
        <table>
          <thead>
            <tr><th>工序</th><th>状态</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in operations" :key="row.id" :class="{ active: row.id === form.id }" @click="editOperation(row)">
              <td>
                <strong>{{ row.name }}</strong>
                <small>{{ row.code || '无编码' }}</small><small :class="{ 'staff-warning': !row.staffing_ready }">{{ row.staffing_ready ? '人员配置完整' : '待补人员配置' }}</small>
                <small>{{ row.updated_at || '-' }}</small>
              </td>
              <td class="master-status">
                <span :class="['pill', row.status]">{{ statusLabel(row.status) }}</span>
                <button class="text danger" type="button" :disabled="row.status === 'inactive'" @click.stop="deactivateOperation(row)">停用</button>
              </td>
            </tr>
            <tr v-if="!operations.length"><td colspan="2" class="muted">暂无工序</td></tr>
          </tbody>
        </table>
      </section>

      <section class="panel editor master-editor-panel operation-editor-panel" data-ownership="工序不决定工时">
        <div class="section-title">{{ form.id ? '编辑工序' : '新建工序' }}</div>
        <div class="form-grid">
          <label><span>工序名称</span><input v-model.trim="form.name" placeholder="烘焙 / 研磨 / 包装" /></label>
          <label><span>编码</span><input v-model.trim="form.code" placeholder="ROAST / PACK" /></label>
          <label>
            <span>状态</span>
            <select v-model="form.status">
              <option value="active">启用</option>
              <option value="inactive">停用</option>
            </select>
          </label>
        </div>
        <section class="operation-staff-section">
          <h3>工序人员</h3><p>先设置谁可以执行，再选择默认负责人。新任务会自动带入，调度仍可临时更换。</p>
          <label><span>搜索可执行员工</span><input v-model.trim="employeeSearch" placeholder="按员工姓名搜索" /></label>
          <div class="eligible-grid"><label v-for="person in filteredEmployees" :key="person.id" :class="{ selected: form.eligible_employee_ids.includes(person.id) }"><input v-model="form.eligible_employee_ids" type="checkbox" :value="person.id" /><span>{{ person.name }}</span></label></div>
          <p v-if="!employees.length" class="staff-warning">暂无启用员工，请先在员工管理中维护。</p>
          <ProductionStaffFields defaults :model-value="{ assigned_employee_id: form.default_employee_id }" :employees="eligibleEmployees" :disabled="loading" @update:model-value="updateDefaultStaff" />
          <p class="staff-help">以上为默认人员，仅用于后续排程带入，不会覆盖已保存的任务安排。</p>
        </section>
        <label class="wide"><span>备注</span><textarea v-model.trim="form.note" rows="3"></textarea></label>
        <div class="footer-actions">
          <button class="primary" type="button" @click="saveOperation" :disabled="loading">保存工序</button>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import ProductionStaffFields from '../components/ProductionStaffFields.vue'
import { apiGet, apiSend } from '../api/client'

const props = defineProps({ viewParams: { type: Object, default: () => ({}) } })
const employees = ref([]), employeeSearch = ref('')
const filteredEmployees = computed(() => employees.value.filter(row => row.name.includes(employeeSearch.value)))
const eligibleEmployees = computed(() => employees.value.filter(row => form.eligible_employee_ids.includes(Number(row.id))))
const loading = ref(false)
const error = ref('')
const ok = ref('')
const operations = ref([])
const form = reactive(blankOperation())

function blankOperation() {
  return { id: 0, eligible_employee_ids: [], default_employee_id: 0, name: '', code: '', status: 'inactive', default_minutes: 0, standard_operation_cost: 0, note: '' }
}

function resetForm(next = blankOperation()) {
  Object.assign(form, next)
}

function statusLabel(status) {
  return status === 'inactive' ? '停用' : '启用'
}

async function loadOperations() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet('/api/manufacturing-operations')
    operations.value = data?.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function newOperation() {
  resetForm()
  error.value = ''
  ok.value = ''
}

function editOperation(row) {
  resetForm({
    id: Number(row.id || 0), eligible_employee_ids: [...(row.eligible_employee_ids || [])], default_employee_id: Number(row.default_employee_id || 0),
    name: row.name || '',
    code: row.code || '',
    status: row.status === 'inactive' ? 'inactive' : 'active',
    default_minutes: Number(row.default_minutes || 0),
    standard_operation_cost: Number(row.standard_operation_cost || 0),
    note: row.note || '',
  })
  error.value = ''
  ok.value = ''
}

async function mutate(action) {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await action()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    loading.value = false
  }
}

async function saveOperation() {
  if (!form.name.trim()) {
    error.value = '请填写工序名称'
    return
  }
  await mutate(async () => {
    const saved = await apiSend('/api/manufacturing-operations', { body: { ...form, default_minutes: 0, standard_operation_cost: 0 } })
    editOperation(saved)
    await loadOperations()
    ok.value = '已保存工序'
  })
}

async function deactivateOperation(row) {
  const id = Number(row?.id || 0)
  if (!id) return
  await mutate(async () => {
    await apiSend(`/api/manufacturing-operations/${id}/deactivate`, { body: {} })
    await loadOperations()
    if (form.id === id) form.status = 'inactive'
    ok.value = '已停用工序'
  })
}

function updateDefaultStaff(value) { form.default_employee_id = value.assigned_employee_id }
watch(() => [...form.eligible_employee_ids], ids => { if (!ids.includes(form.default_employee_id)) form.default_employee_id = 0 })
async function focusOperation() { const row = operations.value.find(row => Number(row.id) === Number(props.viewParams.operation_id)); if (row) editOperation(row) }
watch(() => props.viewParams.operation_id, focusOperation)
onMounted(async () => { await loadOperations(); try { const data = await apiGet('/api/company/employees'); employees.value = (data.rows || data.employees || data || []).filter(row => row.active !== false).map(row => ({ ...row, id: Number(row.id) })) } catch (err) { error.value = err.message } await focusOperation() })
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .actions, .footer-actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.panel-head { justify-content: space-between; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
.master-data-layout { display: grid; grid-template-columns: minmax(300px, 360px) minmax(0, 1fr); gap: 14px; align-items: start; }
.master-list-panel, .master-editor-panel { min-width: 0; }
.master-list-panel { overflow: auto; }
.master-editor-panel { min-width: 0; overflow: hidden; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select, textarea { width: 100%; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
input, select { height: 38px; }
textarea { resize: vertical; }
button { min-height: 36px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #2f8f5b; border-color: #2f8f5b; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; border-color: #999; }
.text { border: 0; background: transparent; color: #1f4f82; padding: 0; }
.text.danger { color: #9d2626; }
table { width: 100%; min-width: 0; border-collapse: collapse; table-layout: fixed; }
th:last-child, td:last-child { width: 86px; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
td small { display: block; color: #777; margin-top: 3px; }
tbody tr.active { background: #eef8f1; }
.section-title { font-size: 16px; font-weight: 700; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.form-grid label, .wide { min-width: 0; }
.wide { display: block; margin-top: 10px; }
.footer-actions { justify-content: flex-end; margin-top: 14px; }
.pill { display: inline-flex; border: 1px solid #d1d5db; border-radius: 999px; padding: 2px 8px; background: #f9fafb; white-space: nowrap; }
.pill.inactive { border-color: #e1b6b6; color: #8a1f1f; background: #fff0f0; }
.master-status { display: flex; flex-direction: column; align-items: flex-start; gap: 6px; }
.master-status .text { min-height: auto; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
@media (max-width: 760px) {
  .master-data-layout, .form-grid { grid-template-columns: 1fr; }
}
.operation-staff-section{margin-top:18px;padding:16px;background:#f5fbf7;border:1px solid #d5e7db;border-radius:10px}.operation-staff-section h3{margin:0;font-size:16px;color:#244b36}.operation-staff-section p{font-size:12px;line-height:1.7;color:#718578}.eligible-grid{display:flex;flex-wrap:wrap;gap:8px;max-height:180px;overflow:auto;margin:12px 0 18px}.eligible-grid label{display:flex;align-items:center;gap:7px;border:1px solid #d5ded8;background:#fff;padding:8px 10px;border-radius:8px;cursor:pointer}.eligible-grid label.selected{background:#e8f6ed;border-color:#91c3a3}.eligible-grid label span{margin:0;color:#45654f;font-size:13px}.eligible-grid input{width:15px;height:15px;accent-color:#2f8f5b}.staff-warning{color:#b17a2c!important}.operation-staff-section .staff-help{margin-bottom:0}.operation-editor-panel{border-color:#dce6df}
</style>
