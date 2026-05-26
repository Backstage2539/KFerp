<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>工艺模板</h2>
        <div class="actions">
          <button class="secondary" type="button" @click="newTemplate">新建</button>
          <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
      <div class="filters">
        <label>
          <span>SKU</span>
          <SearchableSelect
            v-model="filters.product_id"
            :options="products"
            :option-label="optionLabel"
            :option-meta="productMeta"
            :option-value="optionNumericValue"
            placeholder="全部 SKU"
            empty-text="暂无 SKU" />
        </label>
        <label>
          <span>状态</span>
          <select v-model="filters.status">
            <option value="">全部</option>
            <option value="active">已发布</option>
            <option value="draft">草稿</option>
            <option value="inactive">停用</option>
          </select>
        </label>
        <button class="primary" type="button" @click="loadTemplates">筛选</button>
      </div>
    </section>

    <div class="grid">
      <section class="panel table-wrap">
        <div class="section-title">模板列表</div>
        <table>
          <thead>
            <tr>
              <th>模板</th>
              <th>SKU</th>
              <th>BOM版本</th>
              <th>行业模板</th>
              <th>状态</th>
              <th>工序</th>
              <th>更新时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in templates" :key="row.id" :class="{ active: row.id === form.id }" @click="editTemplate(row)">
              <td><strong>{{ row.name }}</strong><small>#{{ row.id }}</small></td>
              <td>{{ row.product_name || '-' }}</td>
              <td>{{ row.bom_version_no || '-' }}</td>
              <td>{{ row.industry_template_name || '-' }}</td>
              <td><span :class="['pill', row.status]">{{ statusLabel(row.status) }}</span></td>
              <td>{{ row.operations?.length || 0 }}</td>
              <td>{{ row.updated_at }}</td>
            </tr>
            <tr v-if="!templates.length">
              <td colspan="7" class="muted">暂无工艺模板</td>
            </tr>
          </tbody>
        </table>
      </section>

      <section class="panel editor">
        <div class="section-title">{{ form.id ? '编辑模板' : '新建模板' }}</div>
        <div class="form-grid">
          <label>
            <span>模板名称</span>
            <input v-model.trim="form.name" placeholder="例如 标准烘焙/裁剪缝制/鲜果去皮包装" />
          </label>
          <label>
            <span>绑定 SKU</span>
            <SearchableSelect
              v-model="form.product_id"
              :options="products"
              :option-label="optionLabel"
              :option-meta="productMeta"
              :option-value="optionNumericValue"
              placeholder="选择 SKU"
              empty-text="暂无 SKU"
              @select="loadBomVersions(optionNumericValue($event))" />
          </label>
          <label>
            <span>BOM版本</span>
            <select v-model.number="form.bom_version_id" :disabled="!form.product_id">
              <option :value="0">当前 BOM</option>
              <option v-for="version in bomVersions" :key="version.id" :value="version.id">
                {{ version.version_no }} · {{ version.status }}
              </option>
            </select>
          </label>
          <label>
            <span>行业字段模板</span>
            <select v-model.number="form.industry_template_id">
              <option :value="0">不绑定</option>
              <option v-for="item in activeIndustryTemplates" :key="item.id" :value="item.id">
                {{ item.name }}
              </option>
            </select>
          </label>
          <label>
            <span>默认设备</span>
            <input v-model.trim="form.default_equipment" placeholder="例如 Probat / 裁床 / 去皮机" />
          </label>
          <label>
            <span>默认工时(分钟)</span>
            <input v-model.number="form.default_minutes" type="number" min="0" step="1" />
          </label>
        </div>
        <label class="wide">
          <span>关键参数 JSON</span>
          <textarea v-model.trim="form.key_params_json" rows="3" placeholder='{"roast_level":"medium","cutting_method":"laser"}'></textarea>
        </label>
        <label class="wide">
          <span>备注</span>
          <textarea v-model.trim="form.note" rows="2"></textarea>
        </label>

        <div class="operations-head">
          <div class="section-title">工艺路线</div>
          <button class="secondary compact" type="button" @click="addOperation">新增工序</button>
        </div>
        <div class="operation-list">
          <div v-for="(op, index) in form.operations" :key="index" class="operation-row">
            <label>
              <span>顺序</span>
              <input v-model.number="op.seq" type="number" min="1" step="1" />
            </label>
            <label>
              <span>工序</span>
              <input v-model.trim="op.operation" placeholder="烘焙/裁剪/去皮/包装" />
            </label>
            <label>
              <span>工位</span>
              <input v-model.trim="op.workstation" placeholder="roaster/cutting/packing" />
            </label>
            <label>
              <span>设备</span>
              <input v-model.trim="op.default_equipment" />
            </label>
            <label>
              <span>分钟</span>
              <input v-model.number="op.default_minutes" type="number" min="0" step="1" />
            </label>
            <label class="checkbox">
              <input v-model="op.records_loss" type="checkbox" />
              <span>记录损耗</span>
            </label>
            <label class="json-field">
              <span>参数字段 JSON</span>
              <textarea v-model.trim="op.parameter_schema_json" rows="2" placeholder='{"temperature":{"type":"number","unit":"C"}}'></textarea>
            </label>
            <label class="json-field">
              <span>质检项 JSON</span>
              <textarea v-model.trim="op.quality_checklist_json" rows="2" placeholder='["外观","重量"]'></textarea>
            </label>
            <button class="text danger" type="button" @click="removeOperation(index)">删除</button>
          </div>
        </div>

        <div class="footer-actions">
          <button class="primary" type="button" @click="saveTemplate" :disabled="loading">保存草稿</button>
          <button class="secondary" type="button" @click="publishTemplate" :disabled="!form.id || loading">发布</button>
          <button class="secondary danger-outline" type="button" @click="deactivateTemplate" :disabled="!form.id || loading">停用</button>
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
const templates = ref([])
const products = ref([])
const industryTemplates = ref([])
const bomVersions = ref([])
const filters = reactive({ product_id: 0, status: '' })
const form = reactive(blankForm())

const activeIndustryTemplates = computed(() => industryTemplates.value.filter((row) => row.status === 'active'))

function blankForm() {
  return {
    id: 0,
    name: '',
    product_id: 0,
    bom_version_id: 0,
    industry_template_id: 0,
    status: 'draft',
    default_equipment: '',
    default_minutes: 0,
    key_params_json: '{}',
    note: '',
    operations: [blankOperation(1)],
  }
}

function blankOperation(seq) {
  return {
    seq,
    operation: '',
    workstation: '',
    default_equipment: '',
    default_minutes: 0,
    records_loss: false,
    parameter_schema_json: '{}',
    quality_checklist_json: '[]',
  }
}

function optionLabel(option) {
  return option?.name || ''
}

function optionNumericValue(option) {
  return Number(option?.id || 0)
}

function productMeta(option) {
  const parts = []
  parts.push(Number(option?.customer_id || 0) ? `客户 #${option.customer_id}` : '公共SKU')
  if (option?.product_kind) parts.push(option.product_kind)
  return parts.join(' / ')
}

function statusLabel(status) {
  if (status === 'active') return '已发布'
  if (status === 'inactive') return '停用'
  return '草稿'
}

function resetForm(next = blankForm()) {
  Object.assign(form, next)
}

function normalizeTemplate(row) {
  return {
    ...blankForm(),
    ...row,
    id: Number(row.id || 0),
    product_id: Number(row.product_id || 0),
    bom_version_id: Number(row.bom_version_id || 0),
    industry_template_id: Number(row.industry_template_id || 0),
    default_minutes: Number(row.default_minutes || 0),
    key_params_json: row.key_params_json || '{}',
    operations: (row.operations || []).length ? row.operations.map((op, index) => ({
      ...blankOperation(index + 1),
      ...op,
      seq: Number(op.seq || index + 1),
      default_minutes: Number(op.default_minutes || 0),
      records_loss: !!op.records_loss,
      parameter_schema_json: op.parameter_schema_json || '{}',
      quality_checklist_json: op.quality_checklist_json || '[]',
    })) : [blankOperation(1)],
  }
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [productData, industryData] = await Promise.all([
      apiGet('/api/bom/products'),
      apiGet('/api/industry-field-templates'),
    ])
    products.value = productData || []
    industryTemplates.value = industryData.rows || []
    await loadTemplates()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadTemplates() {
  const url = new URL('/api/process-templates', window.location.origin)
  if (filters.product_id) url.searchParams.set('product_id', String(filters.product_id))
  if (filters.status) url.searchParams.set('status', filters.status)
  const data = await apiGet(url)
  templates.value = data.rows || []
}

async function loadBomVersions(productID) {
  const id = Number(productID || form.product_id || 0)
  bomVersions.value = []
  if (!id) return
  try {
    const data = await apiGet(`/api/bom/versions?product_id=${id}`)
    bomVersions.value = data.rows || []
  } catch {
    bomVersions.value = []
  }
}

function newTemplate() {
  resetForm()
  bomVersions.value = []
  error.value = ''
  ok.value = ''
}

async function editTemplate(row) {
  resetForm(normalizeTemplate(row))
  await loadBomVersions(form.product_id)
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

async function saveTemplate() {
  await mutate(async () => {
    const row = await apiSend('/api/process-templates', { body: { ...form, status: form.status || 'draft' } })
    resetForm(normalizeTemplate(row))
    await loadTemplates()
    ok.value = '已保存工艺模板'
  })
}

async function publishTemplate() {
  if (!form.id) return
  await mutate(async () => {
    await apiSend(`/api/process-templates/${form.id}/publish`, { body: {} })
    await loadTemplates()
    const current = templates.value.find((row) => Number(row.id) === Number(form.id))
    if (current) resetForm(normalizeTemplate(current))
    ok.value = '已发布工艺模板'
  })
}

async function deactivateTemplate() {
  if (!form.id) return
  await mutate(async () => {
    await apiSend(`/api/process-templates/${form.id}/deactivate`, { body: {} })
    await loadTemplates()
    const current = templates.value.find((row) => Number(row.id) === Number(form.id))
    if (current) resetForm(normalizeTemplate(current))
    ok.value = '已停用工艺模板'
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
.grid { display: grid; grid-template-columns: minmax(420px, .9fr) minmax(560px, 1.1fr); gap: 14px; align-items: start; }
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
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 820px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
td small { display: block; color: #777; margin-top: 3px; }
tbody tr.active { background: #f3f7fb; }
.section-title { font-size: 16px; font-weight: 700; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(220px, 1fr)); gap: 10px; }
.wide { display: block; margin-top: 10px; }
.operations-head { justify-content: space-between; margin-top: 14px; }
.operation-list { display: grid; gap: 10px; }
.operation-row { border: 1px solid #eee8df; border-radius: 8px; padding: 10px; display: grid; grid-template-columns: 72px repeat(4, minmax(110px, 1fr)) 100px; gap: 8px; align-items: end; }
.operation-row .json-field { grid-column: span 3; }
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
@media (max-width: 1180px) {
  .grid, .form-grid { grid-template-columns: 1fr; }
  .operation-row { grid-template-columns: 1fr 1fr; }
  .operation-row .json-field { grid-column: span 2; }
}
@media (max-width: 760px) {
  .page { padding: 12px; }
  .operation-row { grid-template-columns: 1fr; }
  .operation-row .json-field { grid-column: span 1; }
}
</style>
