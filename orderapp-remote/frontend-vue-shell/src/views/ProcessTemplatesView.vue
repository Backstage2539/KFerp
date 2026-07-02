<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>工艺路线</h2>
        <div class="actions">
          <button class="secondary" type="button" @click="newRoute">新建路线</button>
          <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
      <div class="filters">
        <label>
          <span>状态</span>
          <select v-model="filters.status">
            <option value="">全部</option>
            <option value="active">已发布</option>
            <option value="draft">草稿</option>
            <option value="inactive">停用</option>
          </select>
        </label>
        <button class="primary" type="button" @click="loadRoutes">筛选</button>
      </div>
    </section>

    <div class="grid process-route-layout">
      <section class="panel route-list-panel">
        <div class="section-title">路线列表</div>
        <table>
          <thead>
            <tr>
              <th>路线</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in routes" :key="row.id" :class="{ active: row.id === form.id }" @click="editRoute(row)">
              <td>
                <strong>{{ row.name }}</strong>
                <small>#{{ row.id }} · {{ row.operations?.length || 0 }} 道工序</small>
                <small>{{ row.updated_at || '-' }}</small>
              </td>
              <td><span :class="['pill', row.status]">{{ statusLabel(row.status) }}</span></td>
            </tr>
            <tr v-if="!routes.length">
              <td colspan="2" class="muted">暂无工艺路线</td>
            </tr>
          </tbody>
        </table>
      </section>

      <section class="panel editor route-editor-panel">
        <div class="section-title">{{ form.id ? '编辑路线' : '新建路线' }}</div>
        <div class="form-grid">
          <label>
            <span>路线名称</span>
            <input v-model.trim="form.name" placeholder="例如 标准烘焙 / 包装 / 缝制" />
          </label>
          <label>
            <span>状态</span>
            <select v-model="form.status">
              <option value="draft">草稿</option>
              <option value="active">已发布</option>
              <option value="inactive">停用</option>
            </select>
          </label>
        </div>
        <label class="wide">
          <span>备注</span>
          <textarea v-model.trim="form.note" rows="2"></textarea>
        </label>

        <div class="operations-head">
          <div class="section-title">路线工序</div>
          <button class="secondary compact" type="button" @click="addOperation">新增工序</button>
        </div>
        <div class="operation-list">
          <div v-for="(op, index) in form.operations" :key="index" class="operation-row">
            <div class="operation-row-fields">
              <label class="operation-seq">
                <span>顺序</span>
                <input v-model.number="op.seq" type="number" min="1" step="1" />
              </label>
              <label class="operation-select">
                <span>工序</span>
                <SearchableSelect
                  v-model="op.operation_id"
                  :options="activeOperations"
                  :option-label="optionLabel"
                  :option-meta="operationMeta"
                  :option-value="optionNumericValue"
                  placeholder="选择工序"
                  empty-text="暂无工序"
                  @select="applyOperation(index, $event)" />
              </label>
              <label class="operation-name">
                <span>工序名称快照</span>
                <input v-model.trim="op.operation" placeholder="烘焙 / 裁剪 / 包装" />
              </label>
              <label class="standard-capacity-select">
                <span>标准成本默认产能</span>
                <select v-model.number="op.standard_cost_capacity_id">
                  <option :value="0">未设置</option>
                  <option v-for="capacity in standardCostCapacityOptions(op)" :key="capacity.id" :value="Number(capacity.id || 0)">
                    {{ standardCostCapacityOptionLabel(capacity) }}
                  </option>
                </select>
              </label>
              <label class="checkbox operation-loss">
                <input v-model="op.records_loss" type="checkbox" />
                <span>记录损耗</span>
              </label>
            </div>
            <p class="standard-capacity-summary">{{ standardCostCapacitySummary(op) }}</p>
            <div class="operation-quality">
              <label>
                <span>质检项</span>
                <textarea v-model.trim="op.quality_checklist_text" rows="2" placeholder="每行一个质检项，例如：外观&#10;重量"></textarea>
              </label>
              <button class="text danger operation-delete" type="button" @click="removeOperation(index)">删除</button>
            </div>
          </div>
        </div>

        <div class="footer-actions">
          <button class="primary" type="button" @click="saveRoute" :disabled="loading">保存草稿</button>
          <button class="secondary" type="button" @click="publishRoute" :disabled="!form.id || loading">发布</button>
          <button class="secondary danger-outline" type="button" @click="deactivateRoute" :disabled="!form.id || loading">停用</button>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import SearchableSelect from '../components/SearchableSelect.vue'

const loading = ref(false)
const error = ref('')
const ok = ref('')
const routes = ref([])
const operations = ref([])
const workstationCapacities = ref([])
const filters = reactive({ status: '' })
const form = reactive(blankRoute())

const activeOperations = computed(() => operations.value.filter((row) => row.status === 'active'))

function blankRoute() {
  return {
    id: 0,
    name: '',
    status: 'draft',
    default_equipment: '',
    default_minutes: 0,
    note: '',
    operations: [blankOperation(1)],
  }
}

function blankOperation(seq) {
  return {
    seq,
    operation_id: 0,
    operation: '',
    standard_cost_capacity_id: 0,
    records_loss: false,
    quality_checklist_json: '[]',
    quality_checklist_text: '',
  }
}

function optionLabel(option) {
  return option?.name || ''
}

function optionNumericValue(option) {
  return Number(option?.id || 0)
}

function operationMeta(option) {
  const parts = []
  if (option?.code) parts.push(option.code)
  return parts.join(' / ')
}

function statusLabel(status) {
  if (status === 'active') return '已发布'
  if (status === 'inactive') return '停用'
  return '草稿'
}

function qualityChecklistTextFromJSON(value) {
  if (Array.isArray(value)) {
    return value.map((item) => String(item || '').trim()).filter(Boolean).join('\n')
  }
  const text = String(value || '').trim()
  if (!text || text === '[]') return ''
  try {
    const parsed = JSON.parse(text)
    if (Array.isArray(parsed)) {
      return parsed.map((item) => String(item || '').trim()).filter(Boolean).join('\n')
    }
  } catch (_) {
    return text
  }
  return text
}

function qualityChecklistJSONFromText(value) {
  const items = String(value || '')
    .split(/\n|,|，|;|；/)
    .map((item) => item.trim())
    .filter(Boolean)
  return JSON.stringify(items)
}

function routePayload() {
  return {
    ...form,
    status: form.status || 'draft',
    default_equipment: '',
    default_minutes: 0,
    operations: (form.operations || []).map((op) => {
      const { quality_checklist_text: qualityChecklistText, ...rest } = op
      return {
        seq: Number(rest.seq || 0),
        operation_id: Number(rest.operation_id || 0),
        operation: rest.operation || '',
        standard_cost_capacity_id: Number(rest.standard_cost_capacity_id || 0),
        records_loss: !!rest.records_loss,
        quality_checklist_json: qualityChecklistJSONFromText(qualityChecklistText),
      }
    }),
  }
}

function resetForm(next = blankRoute()) {
  Object.assign(form, next)
}

function normalizeRoute(row) {
  return {
    ...blankRoute(),
    ...row,
    id: Number(row.id || 0),
    default_equipment: '',
    default_minutes: Number(row.default_minutes || 0),
    operations: (row.operations || []).length ? row.operations.map((op, index) => ({
      ...blankOperation(index + 1),
      ...op,
      seq: Number(op.seq || index + 1),
      operation_id: Number(op.operation_id || 0),
      operation: op.operation || '',
      standard_cost_capacity_id: Number(op.standard_cost_capacity_id || 0),
      records_loss: !!op.records_loss,
      quality_checklist_json: op.quality_checklist_json || '[]',
      quality_checklist_text: qualityChecklistTextFromJSON(op.quality_checklist_json || '[]'),
    })) : [blankOperation(1)],
  }
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    await Promise.all([loadManufacturingMasterData(), loadRoutes()])
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadManufacturingMasterData() {
  const [operationData, capacityData] = await Promise.all([
    apiGet('/api/manufacturing-operations'),
    apiGet('/api/manufacturing-workstation-capacities'),
  ])
  operations.value = operationData?.rows || []
  workstationCapacities.value = capacityData?.rows || []
}

async function loadRoutes() {
  const url = new URL('/api/process-routes', window.location.origin)
  if (filters.status) url.searchParams.set('status', filters.status)
  const data = await apiGet(url)
  routes.value = data.rows || []
}

function newRoute() {
  resetForm()
  error.value = ''
  ok.value = ''
}

function editRoute(row) {
  resetForm(normalizeRoute(row))
  error.value = ''
  ok.value = ''
}

function addOperation() {
  form.operations.push(blankOperation(form.operations.length + 1))
}

function removeOperation(index) {
  form.operations.splice(index, 1)
  if (!form.operations.length) form.operations.push(blankOperation(1))
}

function applyOperation(index, option) {
  const op = form.operations[index]
  if (!op || !option) return
  op.operation_id = Number(option.id || 0)
  op.operation = option.name || op.operation
  if (!standardCostCapacityOptions(op).some((capacity) => Number(capacity.id || 0) === Number(op.standard_cost_capacity_id || 0))) {
    op.standard_cost_capacity_id = 0
  }
}

function standardCostCapacityOptions(op = {}) {
  const operationID = Number(op.operation_id || 0)
  if (!operationID) return []
  return workstationCapacities.value.filter((capacity) => {
    if (String(capacity.status || 'active') !== 'active') return false
    const ids = Array.isArray(capacity.applicable_operation_ids) ? capacity.applicable_operation_ids.map((id) => Number(id || 0)) : []
    return ids.includes(operationID)
  })
}

function standardCostCapacityOptionLabel(capacity = {}) {
  const parts = [capacity.name || capacity.code || '标准产能']
  if (capacity.workstation) parts.push(capacity.workstation)
  if (Number(capacity.batch_size_qty || 0) > 0) parts.push(`${Number(capacity.batch_size_qty || 0)}${capacity.batch_size_unit || ''}`)
  if (Number(capacity.standard_minutes || 0) > 0) parts.push(`${Number(capacity.standard_minutes || 0)}分钟`)
  return parts.join(' · ')
}

function selectedStandardCostCapacity(op = {}) {
  const id = Number(op.standard_cost_capacity_id || 0)
  if (!id) return null
  return standardCostCapacityOptions(op).find((capacity) => Number(capacity.id || 0) === id) || null
}

function standardCostCapacitySummary(op = {}) {
  const options = standardCostCapacityOptions(op)
  const selected = selectedStandardCostCapacity(op)
  if (selected) {
    const hourlyRate = Number(selected.hourly_rate || 0)
    const minutes = Number(selected.standard_minutes || 0)
    const outputQty = Number(selected.batch_size_qty || 0)
    const outputUnit = selected.batch_size_unit || ''
    return `小时成本 × 标准分钟 / 60 / 标准产出 = ${hourlyRate.toFixed(2)} × ${minutes} / 60 / ${outputQty}${outputUnit}`
  }
  if (!Number(op.operation_id || 0)) return '先选择工序后再选择标准成本默认产能。'
  if (options.length === 1) return `未设置；标准成本试算可按唯一匹配产能「${standardCostCapacityOptionLabel(options[0])}」折算。`
  if (options.length > 1) return '未设置默认；价格试算会提示「请为工艺路线工序设置标准成本默认产能」。'
  return '当前工序暂无启用适用产能；请先维护标准产能。'
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

async function saveRoute() {
  await mutate(async () => {
    const row = await apiSend('/api/process-routes', { body: routePayload() })
    resetForm(normalizeRoute(row))
    await loadRoutes()
    ok.value = '已保存工艺路线'
  })
}

async function publishRoute() {
  if (!form.id) return
  await mutate(async () => {
    await apiSend(`/api/process-routes/${form.id}/publish`, { body: {} })
    await loadRoutes()
    const current = routes.value.find((row) => Number(row.id) === Number(form.id))
    if (current) resetForm(normalizeRoute(current))
    ok.value = '已发布工艺路线'
  })
}

async function deactivateRoute() {
  if (!form.id) return
  await mutate(async () => {
    await apiSend(`/api/process-routes/${form.id}/deactivate`, { body: {} })
    await loadRoutes()
    const current = routes.value.find((row) => Number(row.id) === Number(form.id))
    if (current) resetForm(normalizeRoute(current))
    ok.value = '已停用工艺路线'
  })
}

onMounted(loadAll)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .actions, .filters, .operations-head, .footer-actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.panel-head { justify-content: space-between; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
.process-route-layout { display: grid; grid-template-columns: minmax(300px, 360px) minmax(0, 1fr); gap: 14px; align-items: start; }
.route-list-panel, .route-editor-panel { min-width: 0; }
.route-list-panel { overflow: auto; }
.route-editor-panel { min-width: 0; overflow: hidden; }
.filters { align-items: end; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select, textarea { width: 100%; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
input, select { height: 38px; }
textarea { resize: vertical; }
button { min-height: 36px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; border-color: #999; }
.compact { min-height: 30px; padding: 4px 10px; }
.danger-outline { border-color: #a33; color: #8a1f1f; }
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
.form-grid label, .wide, .operation-row label, .operation-quality label { min-width: 0; }
.wide { display: block; margin-top: 10px; }
.operations-head { justify-content: space-between; margin-top: 14px; }
.operation-list { display: grid; gap: 10px; }
.operation-row { border: 1px solid #eee8df; border-radius: 8px; padding: 10px; display: grid; gap: 10px; }
.operation-row-fields { display: grid; grid-template-columns: 72px minmax(150px, 1.1fr) minmax(160px, 1fr) minmax(190px, 1.2fr) 96px; gap: 8px; align-items: end; }
.standard-capacity-summary { margin: 0; color: #666; font-size: 12px; line-height: 1.5; }
.operation-quality { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 10px; align-items: end; }
.operation-delete { justify-self: end; min-height: 38px; }
.checkbox { display: flex; align-items: center; gap: 6px; min-height: 38px; }
.checkbox input { width: auto; height: auto; }
.checkbox span { margin: 0; }
.footer-actions { justify-content: flex-end; margin-top: 14px; }
.pill { display: inline-flex; border: 1px solid #d1d5db; border-radius: 999px; padding: 2px 8px; background: #f9fafb; white-space: nowrap; }
.pill.active { border-color: #b7d2b7; color: #27602e; background: #f2fbf2; }
.pill.inactive { border-color: #e1b6b6; color: #8a1f1f; background: #fff0f0; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
@media (max-width: 1100px) {
  .operation-row-fields { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
@media (max-width: 760px) {
  .page { padding: 12px; }
  .process-route-layout, .form-grid, .operation-row-fields, .operation-quality { grid-template-columns: 1fr; }
  .operation-delete { justify-self: start; }
}
</style>
