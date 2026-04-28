<template>
  <div class="page">
    <section class="toolbar">
      <div>
        <h2>生产质检</h2>
        <p>原料、生产工单和成品批次的检查结果</p>
      </div>
      <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
    </section>

    <div v-if="message" class="notice">{{ message }}</div>
    <div v-if="error" class="error">{{ error }}</div>

    <section class="form-band">
      <div class="section-title">新增质检记录</div>
      <div class="form-grid">
        <label>
          <span>质检范围</span>
          <select v-model="form.scope">
            <option value="raw_material">原料</option>
            <option value="work_order">生产工单</option>
            <option value="finished_batch">成品批次</option>
          </select>
        </label>
        <label>
          <span>单据/批次号</span>
          <input v-model.trim="form.reference_no" placeholder="WO-0000000020" />
        </label>
        <label>
          <span>物料/产品</span>
          <input v-model.trim="form.item_name" placeholder="孟连水洗5T批次" />
        </label>
        <label>
          <span>检查结果</span>
          <select v-model="form.result">
            <option value="pass">通过</option>
            <option value="hold">待处理</option>
            <option value="reject">不合格</option>
          </select>
        </label>
      </div>
      <div class="note-grid">
        <label>
          <span>指标 JSON</span>
          <textarea v-model.trim="form.metrics_json" rows="3" placeholder='{"水分":"10.5%","色值":"正常"}'></textarea>
        </label>
        <label>
          <span>备注</span>
          <textarea v-model.trim="form.note" rows="3" placeholder="首锅杯测通过"></textarea>
        </label>
      </div>
      <div class="actions">
        <button class="primary" type="button" @click="save" :disabled="saving">保存质检</button>
      </div>
    </section>

    <section class="form-band">
      <div class="section-title">质检记录</div>
      <div class="filters">
        <select v-model="filters.scope" @change="load">
          <option value="">全部范围</option>
          <option value="raw_material">原料</option>
          <option value="work_order">生产工单</option>
          <option value="finished_batch">成品批次</option>
        </select>
        <select v-model="filters.result" @change="load">
          <option value="">全部结果</option>
          <option value="pass">通过</option>
          <option value="hold">待处理</option>
          <option value="reject">不合格</option>
        </select>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>时间</th>
              <th>范围</th>
              <th>单据/批次号</th>
              <th>物料/产品</th>
              <th>结果</th>
              <th>指标</th>
              <th>备注</th>
              <th>操作人</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>{{ row.created_at }}</td>
              <td>{{ scopeLabel(row.scope) }}</td>
              <td>{{ row.reference_no }}</td>
              <td>{{ row.item_name }}</td>
              <td><strong>{{ resultLabel(row.result) }}</strong></td>
              <td class="mono">{{ row.metrics_json }}</td>
              <td>{{ row.note }}</td>
              <td>{{ row.operator }}</td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="8" class="empty">暂无质检记录</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const message = ref('')
const rows = ref([])

const filters = reactive({
  scope: '',
  result: '',
})

const form = reactive({
  scope: 'work_order',
  reference_no: '',
  item_name: '',
  result: 'pass',
  metrics_json: '',
  note: '',
})

function scopeLabel(value) {
  return {
    raw_material: '原料',
    work_order: '生产工单',
    finished_batch: '成品批次',
  }[value] || value
}

function resultLabel(value) {
  return {
    pass: '通过',
    hold: '待处理',
    reject: '不合格',
  }[value] || value
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams()
    if (filters.scope) params.set('scope', filters.scope)
    if (filters.result) params.set('result', filters.result)
    const qs = params.toString()
    const data = await apiGet(`/api/produce/quality-inspections${qs ? `?${qs}` : ''}`)
    rows.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  error.value = ''
  message.value = ''
  try {
    await apiSend('/api/produce/quality-inspections', {
      body: {
        scope: form.scope,
        reference_type: form.scope,
        reference_no: form.reference_no,
        item_name: form.item_name,
        result: form.result,
        metrics_json: form.metrics_json || '{}',
        note: form.note,
      },
    })
    message.value = '质检记录已保存'
    form.reference_no = ''
    form.item_name = ''
    form.metrics_json = ''
    form.note = ''
    await load()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; display: grid; gap: 14px; }
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}
h2 { margin: 0 0 4px; font-size: 20px; }
p { margin: 0; color: #666; }
.form-band { border: 1px solid #e7e0d8; border-radius: 8px; padding: 12px; }
.section-title { font-size: 18px; font-weight: 700; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.note-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-top: 10px; }
label { display: flex; flex-direction: column; gap: 5px; }
span { font-size: 12px; color: #666; }
input, select, textarea, button { font: inherit; }
input, select, textarea {
  width: 100%;
  border: 1px solid #cfc8bf;
  border-radius: 6px;
  padding: 8px 9px;
  background: #fff;
}
textarea { resize: vertical; }
button {
  height: 34px;
  border-radius: 6px;
  border: 1px solid #222;
  padding: 0 10px;
  cursor: pointer;
  white-space: nowrap;
}
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #202020; color: #fff; }
.secondary { background: #fff; color: #202020; }
.actions, .filters { display: flex; gap: 8px; margin-top: 10px; flex-wrap: wrap; }
.filters select { max-width: 180px; }
.notice, .error { border-radius: 6px; padding: 9px 10px; }
.notice { border: 1px solid #b7d9b7; background: #f0fff0; color: #246024; }
.error { border: 1px solid #e0b0b0; background: #fff3f3; color: #8a1f1f; }
.table-wrap { overflow: auto; border: 1px solid #eee8df; border-radius: 8px; }
table { width: 100%; min-width: 1080px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; }
.empty { color: #666; text-align: center; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .toolbar { align-items: flex-start; }
  .form-grid, .note-grid { grid-template-columns: 1fr; }
}
</style>
