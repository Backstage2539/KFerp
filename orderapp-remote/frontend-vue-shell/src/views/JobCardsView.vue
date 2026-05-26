<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>工序卡</h2>
        <button class="secondary" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label>
          <span>状态</span>
          <select v-model="status">
            <option value="">全部</option>
            <option value="running">running</option>
            <option value="completed">completed</option>
            <option value="cancelled">cancelled</option>
          </select>
        </label>
        <button class="primary" @click="load">查询</button>
      </div>
    </section>

    <section class="panel table-wrap">
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>工单</th>
            <th>工序</th>
            <th>工位</th>
            <th>状态</th>
            <th>计划投入</th>
            <th>实际投入</th>
            <th>实际产出</th>
            <th>实际损耗</th>
            <th>异常原因</th>
            <th>开始</th>
            <th>完成</th>
            <th>操作人</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td>#{{ row.id }}</td>
            <td>#{{ row.work_order_id }}</td>
            <td>{{ operationLabel(row.operation) }}</td>
            <td>{{ row.workstation }}</td>
            <td>{{ row.status }}</td>
            <td><input v-model.number="draftFor(row).planned_input_qty" type="number" min="0" step="0.001" /></td>
            <td><input v-model.number="draftFor(row).actual_input_qty" type="number" min="0" step="0.001" /></td>
            <td><input v-model.number="draftFor(row).actual_output_qty" type="number" min="0" step="0.001" /></td>
            <td>
              <strong>{{ qty(actualLossQty(row)) }}</strong>
              <small>{{ formatPercent(actualLossRate(row)) }}</small>
            </td>
            <td><input v-model.trim="draftFor(row).exception_reason" placeholder="可选" /></td>
            <td>{{ row.started_at }}</td>
            <td>{{ row.completed_at || '-' }}</td>
            <td>{{ row.operator }}</td>
            <td><button class="secondary compact" @click="saveActuals(row)" :disabled="loading">保存实际</button></td>
          </tr>
          <tr v-if="!rows.length"><td colspan="14" class="muted">暂无工序卡</td></tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { formatPercent } from '../lib/manufacturing-loss'

const rows = ref([])
const status = ref('')
const loading = ref(false)
const error = ref('')
const drafts = ref({})

function buildDraft(row) {
  return {
    planned_input_qty: Number(row.planned_input_qty || 0),
    actual_input_qty: Number(row.actual_input_qty || 0),
    actual_output_qty: Number(row.actual_output_qty || 0),
    exception_reason: row.exception_reason || '',
  }
}

function draftFor(row) {
  const id = String(row.id)
  if (!drafts.value[id]) drafts.value[id] = buildDraft(row)
  return drafts.value[id]
}

function qty(value) {
  const n = Number(value || 0)
  return n ? n.toLocaleString('zh-CN', { maximumFractionDigits: 3 }) : '-'
}

function actualLossQty(row) {
  const draft = draftFor(row)
  return Math.max(0, Number(draft.actual_input_qty || 0) - Number(draft.actual_output_qty || 0))
}

function actualLossRate(row) {
  const input = Number(draftFor(row).actual_input_qty || 0)
  return input > 0 ? actualLossQty(row) / input : 0
}

function operationLabel(operation) {
  if (operation === 'roast') return '生产'
  return operation || '-'
}

function metricsPayload(value) {
  if (!value) return {}
  if (typeof value === 'object') return value
  try {
    return JSON.parse(value)
  } catch {
    return {}
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/produce/job-cards', window.location.origin)
    if (status.value) url.searchParams.set('status', status.value)
    const data = await apiGet(url)
    rows.value = data.rows || []
    const next = {}
    for (const row of rows.value) next[String(row.id)] = buildDraft(row)
    drafts.value = next
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function saveActuals(row) {
  loading.value = true
  error.value = ''
  try {
    const draft = draftFor(row)
    await apiSend(`/api/produce/job-cards/${row.id}/actuals`, {
      body: {
        planned_input_qty: Number(draft.planned_input_qty || 0),
        actual_input_qty: Number(draft.actual_input_qty || 0),
        actual_output_qty: Number(draft.actual_output_qty || 0),
        exception_reason: draft.exception_reason || '',
        metrics_json: metricsPayload(row.metrics_json),
      },
    })
    await load()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #eee;border-radius:8px;padding:12px;background:#fff}.panel-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:12px}h2{margin:0;font-size:18px}.filters{display:grid;grid-template-columns:160px 90px;gap:10px;align-items:end}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,button,input{font:inherit;min-height:36px;border-radius:6px}select,input{width:100%;border:1px solid #ddd;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #999;background:#fff;color:#111}.compact{min-height:30px;padding:5px 10px}.table-wrap{overflow:auto}table{width:100%;min-width:1320px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px;vertical-align:top}th{background:#fbfbfb}td small{display:block;color:#6b7280;margin-top:3px}.muted{color:#666;text-align:center}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}
</style>
