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
              <label class="operation-cost">
                <span>计划工序成本</span>
                <input v-model.number="op.planned_operation_cost" type="number" min="0" step="0.0001" placeholder="0" />
              </label>
              <label class="checkbox operation-loss">
                <input v-model="op.records_loss" type="checkbox" />
                <span>记录损耗</span>
              </label>
            </div>
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
    planned_operation_cost: 0,
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
        planned_operation_cost: Number(rest.planned_operation_cost || 0),
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
      planned_operation_cost: Number(op.planned_operation_cost || 0),
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
  const operationData = await apiGet('/api/manufacturing-operations')
  operations.value = operationData?.rows || []
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
.operation-row-fields { display: grid; grid-template-columns: 72px minmax(160px, 1.3fr) minmax(140px, 1fr) minmax(120px, .8fr) 96px; gap: 8px; align-items: end; }
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
