<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>工位/设备</h2>
        <div class="actions">
          <button class="secondary" type="button" @click="newWorkstation">新建工位/设备</button>
          <button class="secondary" type="button" @click="loadWorkstations" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <div class="grid master-data-layout">
      <section class="panel master-list-panel workstation-list-panel">
        <div class="section-title">工位/设备列表</div>
        <table>
          <thead>
            <tr><th>工位/设备</th><th>状态</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in workstations" :key="row.id" :class="{ active: row.id === form.id }" @click="editWorkstation(row)">
              <td>
                <strong>{{ row.name }}</strong>
                <small>#{{ row.id }} · {{ row.code || '无编码' }}</small>
                <small>小时成本合计 {{ Number(row.hourly_rate || 0).toFixed(2) }} · {{ row.updated_at || '-' }}</small>
              </td>
              <td class="master-status">
                <span :class="['pill', row.status]">{{ statusLabel(row.status) }}</span>
                <button class="text danger" type="button" :disabled="row.status === 'inactive'" @click.stop="deactivateWorkstation(row)">停用</button>
              </td>
            </tr>
            <tr v-if="!workstations.length"><td colspan="2" class="muted">暂无工位/设备</td></tr>
          </tbody>
        </table>
      </section>

      <section class="panel editor master-editor-panel workstation-editor-panel">
        <div class="section-title">{{ form.id ? '编辑工位/设备' : '新建工位/设备' }}</div>
        <div class="form-grid">
          <label><span>工位/设备名称</span><input v-model.trim="form.name" placeholder="烘焙机 / 包装台 / 质检台" /></label>
          <label><span>编码</span><input v-model.trim="form.code" placeholder="ROASTER-01 / PACK-01" /></label>
          <label><span>机器成本/小时</span><input v-model.number="form.machine_hourly_cost" type="number" min="0" step="0.01" /></label>
          <label><span>人工成本/小时</span><input v-model.number="form.labor_hourly_cost" type="number" min="0" step="0.01" /></label>
          <label><span>其他成本/小时</span><input v-model.number="form.overhead_hourly_cost" type="number" min="0" step="0.01" /></label>
          <label><span>小时成本合计</span><input :value="workstationHourlyRate.toFixed(2)" type="text" readonly /></label>
          <label>
            <span>状态</span>
            <select v-model="form.status">
              <option value="active">启用</option>
              <option value="inactive">停用</option>
            </select>
          </label>
        </div>
        <div class="wide operation-checks">
          <span>适用工序</span>
          <div v-if="activeOperations.length" class="operation-check-grid">
            <label v-for="operation in activeOperations" :key="operation.id" class="operation-checkbox">
              <input v-model="form.applicable_operation_ids" type="checkbox" :value="Number(operation.id)" />
              <span>{{ operation.name }}</span>
            </label>
          </div>
          <div v-else class="muted inline-muted">暂无启用工序</div>
          <small>生产计划自动拆分会按这里筛选该工位下的产能；工位产能本身不再维护适用工序。</small>
        </div>
        <label class="wide"><span>备注</span><textarea v-model.trim="form.note" rows="3"></textarea></label>
        <div class="footer-actions">
          <button class="primary" type="button" @click="saveWorkstation" :disabled="loading">保存工位/设备</button>
        </div>

        <div class="capacity-panel">
          <div class="operations-head">
            <div class="section-title">工位产能</div>
            <button class="secondary compact" type="button" @click="newCapacity" :disabled="!form.id">新增产能</button>
          </div>
          <div v-if="!form.id" class="muted inline-muted">先选择或保存工位/设备</div>
          <table v-else class="capacity-table">
            <thead>
              <tr><th>工位产能</th><th>批量</th><th>标准分钟/批</th><th>继承小时成本</th><th>状态</th></tr>
            </thead>
            <tbody>
              <tr v-for="row in capacitiesForSelectedWorkstation" :key="row.id" :class="{ active: row.id === capacityForm.id }" @click="editCapacity(row)">
                <td>
                  <strong>{{ row.name }}</strong>
                  <small>{{ row.code || '无编码' }}</small>
                </td>
                <td>{{ Number(row.batch_size_qty || 0) }} {{ row.batch_size_unit || '' }}</td>
                <td>{{ row.standard_minutes || 0 }}</td>
                <td>{{ Number(row.hourly_rate || 0).toFixed(2) }}</td>
                <td>
                  <span :class="['pill', row.status]">{{ statusLabel(row.status) }}</span>
                  <button class="text danger" type="button" :disabled="row.status === 'inactive'" @click.stop="deactivateCapacity(row)">停用</button>
                </td>
              </tr>
              <tr v-if="!capacitiesForSelectedWorkstation.length"><td colspan="5" class="muted">暂无工位产能</td></tr>
            </tbody>
          </table>

          <div v-if="form.id" class="capacity-form">
            <label><span>产能名称</span><input v-model.trim="capacityForm.name" placeholder="布勒 18kg / 智烘 3kg" /></label>
            <label><span>编码</span><input v-model.trim="capacityForm.code" placeholder="BUHLER-18KG" /></label>
            <label><span>标准批量</span><input v-model.number="capacityForm.batch_size_qty" type="number" min="0" step="0.001" /></label>
            <label><span>单位</span><input v-model.trim="capacityForm.batch_size_unit" placeholder="kg / g / 件" /></label>
            <label><span>标准分钟/批</span><input v-model.number="capacityForm.standard_minutes" type="number" min="0" step="1" /></label>
            <label><span>继承工位小时成本</span><input :value="workstationHourlyRate.toFixed(2)" type="text" readonly /></label>
            <label>
              <span>状态</span>
              <select v-model="capacityForm.status">
                <option value="active">启用</option>
                <option value="inactive">停用</option>
              </select>
            </label>
            <label><span>排序</span><input v-model.number="capacityForm.sort_order" type="number" step="1" /></label>
            <label class="wide"><span>备注</span><textarea v-model.trim="capacityForm.note" rows="2"></textarea></label>
            <div class="footer-actions">
              <button class="primary" type="button" @click="saveCapacity" :disabled="loading || !form.id">保存工位产能</button>
            </div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const loading = ref(false)
const error = ref('')
const ok = ref('')
const workstations = ref([])
const workstationCapacities = ref([])
const operations = ref([])
const form = reactive(blankWorkstation())
const capacityForm = reactive(blankCapacity())

const capacitiesForSelectedWorkstation = computed(() => workstationCapacities.value.filter((row) => Number(row.workstation_id || 0) === Number(form.id || 0)))
const activeOperations = computed(() => operations.value.filter((row) => String(row.status || 'active') === 'active'))
const workstationHourlyRate = computed(() => Number((Number(form.machine_hourly_cost || 0) + Number(form.labor_hourly_cost || 0) + Number(form.overhead_hourly_cost || 0)).toFixed(2)))

function blankWorkstation() {
  return { id: 0, name: '', code: '', status: 'active', default_minutes: 0, machine_hourly_cost: 0, labor_hourly_cost: 0, overhead_hourly_cost: 0, hourly_rate: 0, applicable_operation_ids: [], note: '' }
}

function blankCapacity() {
  return {
    id: 0,
    workstation_id: 0,
    name: '',
    code: '',
    status: 'active',
    batch_size_qty: 0,
    batch_size_unit: 'kg',
    standard_minutes: 0,
    production_capacity: 1,
    sort_order: 0,
    note: '',
  }
}

function resetForm(next = blankWorkstation()) {
  Object.assign(form, {
    ...next,
    applicable_operation_ids: Array.isArray(next.applicable_operation_ids) ? next.applicable_operation_ids.map((id) => Number(id || 0)).filter((id) => id > 0) : [],
  })
}

function resetCapacity(next = blankCapacity()) {
  Object.assign(capacityForm, next)
}

function statusLabel(status) {
  return status === 'inactive' ? '停用' : '启用'
}

async function loadWorkstations() {
  loading.value = true
  error.value = ''
  try {
    const [data, capacityData, operationData] = await Promise.all([
      apiGet('/api/manufacturing-workstations'),
      apiGet('/api/manufacturing-workstation-capacities'),
      apiGet('/api/manufacturing-operations'),
    ])
    workstations.value = data?.rows || []
    workstationCapacities.value = capacityData?.rows || []
    operations.value = operationData?.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function newWorkstation() {
  resetForm()
  resetCapacity()
  error.value = ''
  ok.value = ''
}

function editWorkstation(row) {
  resetForm({
    id: Number(row.id || 0),
    name: row.name || '',
    code: row.code || '',
    status: row.status === 'inactive' ? 'inactive' : 'active',
    default_minutes: Number(row.default_minutes || 0),
    machine_hourly_cost: Number(row.machine_hourly_cost || 0),
    labor_hourly_cost: Number(row.labor_hourly_cost || 0),
    overhead_hourly_cost: Number(row.overhead_hourly_cost || 0),
    hourly_rate: Number(row.hourly_rate || 0),
    applicable_operation_ids: Array.isArray(row.applicable_operation_ids) ? row.applicable_operation_ids : [],
    note: row.note || '',
  })
  resetCapacity({ ...blankCapacity(), workstation_id: Number(row.id || 0) })
  error.value = ''
  ok.value = ''
}

function newCapacity() {
  resetCapacity({ ...blankCapacity(), workstation_id: Number(form.id || 0) })
}

function editCapacity(row) {
  resetCapacity({
    ...blankCapacity(),
    ...row,
    id: Number(row.id || 0),
    workstation_id: Number(row.workstation_id || form.id || 0),
    batch_size_qty: Number(row.batch_size_qty || 0),
    standard_minutes: Number(row.standard_minutes || 0),
    production_capacity: Number(row.production_capacity || 1),
    sort_order: Number(row.sort_order || 0),
  })
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

async function saveWorkstation() {
  if (!form.name.trim()) {
    error.value = '请填写工位/设备名称'
    return
  }
  await mutate(async () => {
    const saved = await apiSend('/api/manufacturing-workstations', {
      body: {
        ...form,
        default_minutes: 0,
        machine_hourly_cost: Number(form.machine_hourly_cost || 0),
        labor_hourly_cost: Number(form.labor_hourly_cost || 0),
        overhead_hourly_cost: Number(form.overhead_hourly_cost || 0),
        hourly_rate: workstationHourlyRate.value,
        applicable_operation_ids: form.applicable_operation_ids.map((id) => Number(id || 0)).filter((id) => id > 0),
      },
    })
    editWorkstation(saved)
    await loadWorkstations()
    ok.value = '已保存工位/设备'
  })
}

async function deactivateWorkstation(row) {
  const id = Number(row?.id || 0)
  if (!id) return
  await mutate(async () => {
    await apiSend(`/api/manufacturing-workstations/${id}/deactivate`, { body: {} })
    await loadWorkstations()
    if (form.id === id) form.status = 'inactive'
    ok.value = '已停用工位/设备'
  })
}

async function saveCapacity() {
  if (!form.id) return
  if (!capacityForm.name.trim()) {
    error.value = '请填写工位产能名称'
    return
  }
  await mutate(async () => {
    const saved = await apiSend('/api/manufacturing-workstation-capacities', {
      body: {
        ...capacityForm,
        workstation_id: Number(form.id || 0),
        batch_size_qty: Number(capacityForm.batch_size_qty || 0),
        standard_minutes: Number(capacityForm.standard_minutes || 0),
        hourly_rate: 0,
        production_capacity: Number(capacityForm.production_capacity || 1),
        sort_order: Number(capacityForm.sort_order || 0),
      },
    })
    editCapacity(saved)
    await loadWorkstations()
    ok.value = '已保存工位产能'
  })
}

async function deactivateCapacity(row) {
  const id = Number(row?.id || 0)
  if (!id) return
  await mutate(async () => {
    await apiSend(`/api/manufacturing-workstation-capacities/${id}/deactivate`, { body: {} })
    await loadWorkstations()
    if (capacityForm.id === id) capacityForm.status = 'inactive'
    ok.value = '已停用工位产能'
  })
}

onMounted(loadWorkstations)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .actions, .footer-actions, .operations-head { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.panel-head { justify-content: space-between; margin-bottom: 12px; }
.operations-head { justify-content: space-between; margin-top: 14px; }
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
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; border-color: #999; }
.compact { min-height: 30px; padding: 4px 10px; }
.text { border: 0; background: transparent; color: #1f4f82; padding: 0; }
.text.danger { color: #9d2626; }
table { width: 100%; min-width: 0; border-collapse: collapse; table-layout: fixed; }
th:last-child, td:last-child { width: 86px; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
td small { display: block; color: #777; margin-top: 3px; }
tbody tr.active { background: #f3f7fb; }
.section-title { font-size: 16px; font-weight: 700; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.form-grid label, .wide { min-width: 0; }
.wide { display: block; margin-top: 10px; }
.capacity-panel { border-top: 1px solid #eee8df; margin-top: 16px; padding-top: 12px; }
.capacity-table { margin-top: 8px; }
.capacity-table th:last-child, .capacity-table td:last-child { width: 120px; }
.capacity-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; margin-top: 12px; }
.capacity-form .wide { grid-column: 1 / -1; margin-top: 0; }
.operation-checks > span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
.operation-checks small { display: block; color: #777; margin-top: 6px; }
.operation-check-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 8px; }
.operation-checkbox { display: flex; align-items: center; gap: 6px; border: 1px solid #ddd7ce; border-radius: 6px; padding: 7px 9px; margin: 0; }
.operation-checkbox input { width: 16px; height: 16px; padding: 0; flex: 0 0 auto; }
.operation-checkbox span { margin: 0; color: #171717; font-size: 14px; }
.inline-muted { border: 1px dashed #ddd2c7; border-radius: 6px; padding: 10px; }
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
  .master-data-layout, .form-grid, .capacity-form { grid-template-columns: 1fr; }
}
</style>
