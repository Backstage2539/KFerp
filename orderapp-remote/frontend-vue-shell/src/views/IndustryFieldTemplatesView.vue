<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>行业字段模板</h2>
        <div class="actions">
          <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <div class="grid industry-template-layout">
      <section class="panel industry-template-list-panel">
        <div class="section-title-row">
          <div class="section-title">模板列表</div>
          <button class="primary compact" type="button" @click="newTemplate">新建模板</button>
        </div>
        <div class="template-filters">
          <label>
            <span>搜索模板</span>
            <input v-model.trim="industryTemplateFilters.query" placeholder="搜索模板名" />
          </label>
          <label>
            <span>状态</span>
            <select v-model="industryTemplateFilters.status">
              <option value="active">启用</option>
              <option value="inactive">停用</option>
              <option value="all">全部</option>
            </select>
          </label>
        </div>
        <div class="template-list">
          <button
            v-for="row in filteredIndustryTemplateRows"
            :key="row.id"
            :class="['template-list-row', { active: row.id === form.id }]"
            type="button"
            @click="editTemplate(row)">
            <span>
              <strong>{{ row.name }}</strong>
              <small>{{ row.description || '无说明' }}</small>
            </span>
            <span :class="['pill', row.status]">{{ row.status === 'active' ? '启用' : '停用' }}</span>
            <small>{{ row.fields?.length || 0 }} 个字段 · {{ row.updated_at || '-' }}</small>
          </button>
          <p v-if="!filteredIndustryTemplateRows.length" class="muted list-empty">暂无匹配的行业字段模板</p>
        </div>
      </section>

      <section class="panel editor industry-template-editor-panel">
        <div class="section-title">{{ form.id ? '编辑模板' : '新建模板' }}</div>
        <div class="form-grid">
          <label>
            <span>模板名称</span>
            <input v-model.trim="form.name" placeholder="咖啡烘焙参数 / 服装加工参数 / 鲜果加工参数" />
          </label>
          <label>
            <span>状态</span>
            <select v-model="form.status">
              <option value="active">启用</option>
              <option value="inactive">停用</option>
            </select>
          </label>
        </div>
        <label class="wide">
          <span>说明</span>
          <textarea v-model.trim="form.description" rows="2"></textarea>
        </label>

        <div class="fields-head">
          <div class="section-title">字段定义</div>
          <button class="secondary compact" type="button" @click="addField">新增字段</button>
        </div>
        <div class="field-list">
          <div v-for="(field, index) in form.fields" :key="index" class="field-row">
            <label>
              <span>排序</span>
              <input v-model.number="field.sort_order" type="number" min="1" step="1" />
            </label>
            <label>
              <span>字段键</span>
              <input v-model.trim="field.field_key" placeholder="烘焙度 / 布料损耗率 / 产地" />
            </label>
            <label>
              <span>类型</span>
              <select v-model="field.field_type">
                <option v-for="option in fieldTypeOptions(field)" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
            </label>
            <label class="options">
              <span>{{ field.field_type === 'select' ? '下拉选项' : '默认文本' }}</span>
              <input
                v-model.trim="field.options_text"
                :placeholder="field.field_type === 'select' ? '空格分隔的下拉选项' : '输入默认文本'" />
            </label>
            <button class="text danger" type="button" @click="removeField(index)">删除</button>
          </div>
        </div>
        <div class="footer-actions">
          <button class="primary" type="button" @click="save" :disabled="loading">保存模板</button>
          <button class="secondary danger-outline" type="button" @click="deactivate" :disabled="!form.id || loading">停用</button>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const rows = ref([])
const loading = ref(false)
const error = ref('')
const ok = ref('')
const form = reactive(blankForm())
const industryTemplateFilters = reactive({
  query: '',
  status: 'active',
})

const filteredIndustryTemplateRows = computed(() => {
  const query = String(industryTemplateFilters.query || '').trim().toLowerCase()
  const status = String(industryTemplateFilters.status || 'active')
  return rows.value.filter((row) => {
    if (status !== 'all' && String(row.status || 'active') !== status) return false
    if (!query) return true
    return String(row.name || '').toLowerCase().includes(query)
  })
})

function blankForm() {
  return {
    id: 0,
    name: '',
    industry_key: 'general',
    description: '',
    status: 'active',
    fields: [blankField(1)],
  }
}

function blankField(sortOrder) {
  return {
    field_key: '',
    label: '',
    field_type: 'text',
    unit: '',
    required: false,
    options_json: '[]',
    options_text: '',
    sort_order: sortOrder,
  }
}

function optionsTextFromJSON(raw) {
  try {
    const parsed = JSON.parse(String(raw || '[]'))
    return Array.isArray(parsed) ? parsed.map((item) => String(item || '').trim()).filter(Boolean).join(' ') : ''
  } catch (_) {
    return String(raw || '').trim()
  }
}

function optionsJSONFromText(raw) {
  const seen = new Set()
  const values = String(raw || '').replace(/[,，]+/g, ' ')
    .split(/\s+/)
    .map((item) => item.trim())
    .filter((item) => {
      if (!item || seen.has(item)) return false
      seen.add(item)
      return true
    })
  return JSON.stringify(values)
}

function defaultTextJSON(raw) {
  const value = String(raw || '').trim()
  return JSON.stringify(value ? [value] : [])
}

function fieldTypeOptions(field = {}) {
  const base = [
    { value: 'text', label: '文本' },
    { value: 'select', label: '下拉' },
  ]
  const current = String(field.field_type || '').trim()
  const legacy = {
    textarea: '长文本',
    number: '数字',
    ratio: '比例',
    checkbox: '勾选',
    date: '日期',
  }
  if (current && !base.some((item) => item.value === current)) {
    base.push({ value: current, label: legacy[current] || current })
  }
  return base
}

function normalizeTemplate(row) {
  return {
    ...blankForm(),
    ...row,
    id: Number(row.id || 0),
    fields: (row.fields || []).length ? row.fields.map((field, index) => ({
      ...blankField(index + 1),
      ...field,
      required: !!field.required,
      sort_order: Number(field.sort_order || index + 1),
      options_json: field.options_json || '[]',
      options_text: optionsTextFromJSON(field.options_json || '[]'),
    })) : [blankField(1)],
  }
}

function resetForm(next = blankForm()) {
  Object.assign(form, next)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet('/api/industry-field-templates')
    rows.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function newTemplate() {
  resetForm()
  ok.value = ''
  error.value = ''
}

function editTemplate(row) {
  resetForm(normalizeTemplate(row))
  ok.value = ''
  error.value = ''
}

function addField() {
  form.fields.push(blankField(form.fields.length + 1))
}

function removeField(index) {
  form.fields.splice(index, 1)
  if (!form.fields.length) form.fields.push(blankField(1))
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

async function save() {
  await mutate(async () => {
    const payload = {
      ...form,
      industry_key: 'general',
      fields: form.fields.map((field) => ({
        ...field,
        label: String(field.field_key || '').trim(),
        unit: '',
        required: false,
        options_json: field.field_type === 'select' ? optionsJSONFromText(field.options_text) : defaultTextJSON(field.options_text),
      })),
    }
    const row = await apiSend('/api/industry-field-templates', { body: payload })
    resetForm(normalizeTemplate(row))
    await load()
    ok.value = '已保存行业字段模板'
  })
}

async function deactivate() {
  if (!form.id) return
  await mutate(async () => {
    await apiSend(`/api/industry-field-templates/${form.id}/deactivate`, { body: {} })
    await load()
    const current = rows.value.find((row) => Number(row.id) === Number(form.id))
    if (current) resetForm(normalizeTemplate(current))
    ok.value = '已停用行业字段模板'
  })
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .actions, .fields-head, .footer-actions, .section-title-row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.panel-head { justify-content: space-between; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
.grid { display: grid; grid-template-columns: minmax(280px, 360px) minmax(560px, 1fr); gap: 14px; align-items: start; }
.industry-template-layout { min-height: 560px; }
.industry-template-list-panel, .industry-template-editor-panel { align-self: stretch; }
.industry-template-list-panel { display: flex; flex-direction: column; min-height: 560px; }
.section-title-row { justify-content: space-between; margin-bottom: 10px; }
.section-title-row .section-title { margin-bottom: 0; }
.template-filters { display: grid; grid-template-columns: 1fr 120px; gap: 8px; margin-bottom: 10px; }
.template-list { display: grid; align-content: start; gap: 8px; overflow: auto; padding-right: 2px; }
.template-list-row { min-height: auto; height: auto; border: 1px solid #eee8df; background: #fff; border-radius: 8px; padding: 10px; display: grid; gap: 6px; text-align: left; color: #222; }
.template-list-row.active { border-color: #1f1f1f; background: #f8f7f5; }
.template-list-row strong, .template-list-row small { display: block; }
.template-list-row small { color: #777; line-height: 1.35; }
.list-empty { padding: 18px 8px; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 620px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
td small { display: block; color: #777; margin-top: 3px; }
tbody tr.active { background: #f3f7fb; }
.section-title { font-size: 16px; font-weight: 700; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(180px, 1fr)); gap: 10px; }
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
.wide { display: block; margin-top: 10px; }
.fields-head { justify-content: space-between; margin-top: 14px; }
.field-list { display: grid; gap: 10px; }
.field-row { border: 1px solid #eee8df; border-radius: 8px; padding: 10px; display: grid; grid-template-columns: 70px minmax(180px, 1fr) 120px minmax(220px, 1fr) 72px; gap: 8px; align-items: end; }
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
  .field-row { grid-template-columns: 1fr 1fr; }
}
@media (max-width: 760px) {
  .page { padding: 12px; }
  .field-row { grid-template-columns: 1fr; }
}
</style>
