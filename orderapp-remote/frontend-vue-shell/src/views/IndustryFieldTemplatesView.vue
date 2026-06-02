<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>行业字段模板</h2>
        <div class="actions">
          <button class="secondary" type="button" @click="newTemplate">新建</button>
          <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <div class="grid industry-template-layout">
      <section class="panel table-wrap">
        <div class="section-title">模板列表</div>
        <table>
          <thead>
            <tr>
              <th>模板</th>
              <th>行业键</th>
              <th>状态</th>
              <th>字段数</th>
              <th>更新时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id" :class="{ active: row.id === form.id }" @click="editTemplate(row)">
              <td><strong>{{ row.name }}</strong><small>{{ row.description || '-' }}</small></td>
              <td>{{ row.industry_key }}</td>
              <td><span :class="['pill', row.status]">{{ row.status === 'active' ? '启用' : '停用' }}</span></td>
              <td>{{ row.fields?.length || 0 }}</td>
              <td>{{ row.updated_at }}</td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="5" class="muted">暂无行业字段模板</td>
            </tr>
          </tbody>
        </table>
      </section>

      <section class="panel editor">
        <div class="section-title">{{ form.id ? '编辑模板' : '新建模板' }}</div>
        <div class="form-grid">
          <label>
            <span>模板名称</span>
            <input v-model.trim="form.name" placeholder="咖啡烘焙参数 / 服装加工参数 / 鲜果加工参数" />
          </label>
          <label>
            <span>行业键</span>
            <input v-model.trim="form.industry_key" placeholder="coffee / apparel / fruit" />
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
              <input v-model.trim="field.field_key" placeholder="roast_degree / cloth_loss_rate" />
            </label>
            <label>
              <span>显示名</span>
              <input v-model.trim="field.label" placeholder="烘焙度 / 布料损耗率" />
            </label>
            <label>
              <span>类型</span>
              <select v-model="field.field_type">
                <option v-for="option in fieldTypeOptions(field)" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
            </label>
            <label>
              <span>单位</span>
              <input v-model.trim="field.unit" placeholder="% / g / min" />
            </label>
            <label class="checkbox">
              <input v-model="field.required" type="checkbox" />
              <span>必填</span>
            </label>
            <label class="options">
              <span>下拉预设</span>
              <textarea v-model.trim="field.options_text" rows="2" placeholder="浅烘, 中烘, 深烘"></textarea>
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
import { onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const rows = ref([])
const loading = ref(false)
const error = ref('')
const ok = ref('')
const form = reactive(blankForm())

function blankForm() {
  return {
    id: 0,
    name: '',
    industry_key: '',
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
    return Array.isArray(parsed) ? parsed.map((item) => String(item || '').trim()).filter(Boolean).join(', ') : ''
  } catch (_) {
    return String(raw || '').trim()
  }
}

function optionsJSONFromText(raw) {
  const seen = new Set()
  const values = String(raw || '')
    .split(/[,，]/)
    .map((item) => item.trim())
    .filter((item) => {
      if (!item || seen.has(item)) return false
      seen.add(item)
      return true
    })
  return JSON.stringify(values)
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
      fields: form.fields.map((field) => ({
        ...field,
        options_json: optionsJSONFromText(field.options_text),
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
.panel-head, .actions, .fields-head, .footer-actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.panel-head { justify-content: space-between; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
.grid { display: grid; grid-template-columns: minmax(380px, .85fr) minmax(560px, 1.15fr); gap: 14px; align-items: start; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 620px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
td small { display: block; color: #777; margin-top: 3px; }
tbody tr.active { background: #f3f7fb; }
.section-title { font-size: 16px; font-weight: 700; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: repeat(3, minmax(160px, 1fr)); gap: 10px; }
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
.field-row { border: 1px solid #eee8df; border-radius: 8px; padding: 10px; display: grid; grid-template-columns: 70px repeat(4, minmax(110px, 1fr)) 80px; gap: 8px; align-items: end; }
.field-row .options { grid-column: span 5; }
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
  .field-row { grid-template-columns: 1fr 1fr; }
  .field-row .options { grid-column: span 2; }
}
@media (max-width: 760px) {
  .page { padding: 12px; }
  .field-row { grid-template-columns: 1fr; }
  .field-row .options { grid-column: span 1; }
}
</style>
