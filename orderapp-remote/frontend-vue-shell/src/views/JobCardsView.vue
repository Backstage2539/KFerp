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
            <option v-for="option in statusOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
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
            <th>顺序</th>
            <th>工序</th>
            <th>工位</th>
            <th>工位产能</th>
            <th>状态</th>
            <th>计划分钟</th>
            <th>计划工序成本</th>
            <th>实际分钟</th>
            <th>实际工序成本</th>
            <th>实际损耗</th>
            <th>损耗原因</th>
            <th>异常原因</th>
            <th>开始</th>
            <th>暂停/继续</th>
            <th>完成</th>
            <th>操作人</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td>#{{ row.id }}</td>
            <td>#{{ row.work_order_id }}</td>
            <td>{{ row.sequence_no || 1 }}</td>
            <td>{{ operationLabel(row.operation) }}<small v-if="row.records_loss">记录损耗</small></td>
            <td>{{ row.workstation }}</td>
            <td>{{ row.workstation_capacity_name || '-' }}</td>
            <td><span class="status" :class="statusBadgeClass(row.status)">{{ jobCardStatusLabel(row.status) }}</span></td>
            <td>{{ row.planned_minutes || 0 }}</td>
            <td>{{ money(row.planned_operation_cost) }}</td>
            <td><input v-model.number="draftFor(row).actual_minutes" type="number" min="0" step="1" /></td>
            <td>{{ actualOperationCost(row) }}</td>
            <td>
              <strong>{{ qty(actualLossQty(row)) }}</strong>
              <small>{{ formatPercent(actualLossRate(row)) }}</small>
            </td>
            <td><input v-model.trim="draftFor(row).loss_reason" placeholder="损耗原因" /></td>
            <td><input v-model.trim="draftFor(row).exception_reason" placeholder="可选" /></td>
            <td>{{ row.started_at }}</td>
            <td>
              <small>暂停 {{ row.paused_at || '-' }}</small>
              <small>继续 {{ row.resumed_at || '-' }}</small>
            </td>
            <td>{{ row.completed_at || '-' }}</td>
            <td>{{ row.operator }}</td>
            <td class="row-actions">
              <button class="primary compact" @click="runJobCardAction(row, 'start')" :disabled="!canRunJobCardAction(row, 'start') || loading">开始</button>
              <button class="secondary compact" @click="runJobCardAction(row, 'pause')" :disabled="!canRunJobCardAction(row, 'pause') || loading">暂停</button>
              <button class="secondary compact" @click="runJobCardAction(row, 'resume')" :disabled="!canRunJobCardAction(row, 'resume') || loading">继续</button>
              <button class="primary compact" @click="runJobCardAction(row, 'complete')" :disabled="!canRunJobCardAction(row, 'complete') || loading">完成</button>
              <button class="secondary compact" @click="saveActuals(row)" :disabled="loading">保存实际</button>
            </td>
          </tr>
          <tr v-if="!rows.length"><td colspan="19" class="muted">暂无工序卡</td></tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { buildJobCardActionPayload, canRunJobCardAction, jobCardActionEndpoint, jobCardStatusLabel, jobCardStatusOptions } from '../lib/manufacturing-execution'
import { formatPercent } from '../lib/manufacturing-loss'

const rows = ref([])
const status = ref('')
const loading = ref(false)
const error = ref('')
const drafts = ref({})
const statusOptions = jobCardStatusOptions()

function buildDraft(row) {
  return {
    planned_input_qty: Number(row.planned_input_qty || 0),
    actual_input_qty: Number(row.actual_input_qty || 0),
    actual_output_qty: Number(row.actual_output_qty || 0),
    actual_minutes: Number(row.actual_minutes || 0),
    loss_reason: row.loss_reason || '',
    exception_reason: row.exception_reason || '',
    metrics_json: row.metrics_json || '{}',
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

function money(value) {
  return Number(value || 0).toFixed(2)
}

function actualLossQty(row) {
  return Math.max(0, Number(row.actual_loss_qty || 0))
}

function actualLossRate(row) {
  return Math.max(0, Number(row.actual_loss_rate || 0))
}

function actualOperationCost(row) {
  const draft = draftFor(row)
  const minutes = Number(draft.actual_minutes || 0)
  const hourlyRate = Number(row.hourly_rate || 0)
  if (minutes > 0 && hourlyRate > 0) return money((minutes / 60) * hourlyRate)
  return money(row.actual_operation_cost || 0)
}

function operationLabel(operation) {
  if (operation === 'roast') return '生产'
  return operation || '-'
}

function statusBadgeClass(statusValue) {
  return {
    pending: 'neutral',
    ready: 'info',
    running: 'warning',
    paused: 'warning',
    completed: 'success',
    cancelled: 'danger',
  }[String(statusValue || '').trim()] || 'neutral'
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
        actual_minutes: Number(draft.actual_minutes || 0),
        loss_reason: draft.loss_reason || '',
        exception_reason: draft.exception_reason || '',
        metrics_json: metricsPayload(draft.metrics_json),
      },
    })
    await load()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    loading.value = false
  }
}

async function runJobCardAction(row, action) {
  const endpoint = jobCardActionEndpoint(row, action)
  if (!endpoint) return
  loading.value = true
  error.value = ''
  try {
    await apiSend(endpoint, { body: buildJobCardActionPayload(draftFor(row)) })
    await load()
  } catch (err) {
    error.value = err.message || '工序状态更新失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #eee;border-radius:8px;padding:12px;background:#fff}.panel-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:12px}h2{margin:0;font-size:18px}.filters{display:grid;grid-template-columns:160px 90px;gap:10px;align-items:end}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,button,input{font:inherit;min-height:36px;border-radius:6px}select,input{width:100%;border:1px solid #ddd;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #999;background:#fff;color:#111}.compact{min-height:30px;padding:5px 10px}.row-actions{display:flex;gap:6px;flex-wrap:wrap;min-width:260px}.status{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px;background:#f9fafb}.status.info{border-color:#93c5fd;background:#eff6ff;color:#1d4ed8}.status.warning{border-color:#fed7aa;background:#fff7ed;color:#c2410c}.status.success{border-color:#bbf7d0;background:#f0fdf4;color:#15803d}.status.danger{border-color:#fecaca;background:#fef2f2;color:#b91c1c}.status.neutral{border-color:#d1d5db;background:#f9fafb;color:#374151}.table-wrap{overflow:auto}table{width:100%;min-width:1240px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px;vertical-align:top}th{background:#fbfbfb}td small{display:block;color:#6b7280;margin-top:3px}.muted{color:#666;text-align:center}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}
</style>
