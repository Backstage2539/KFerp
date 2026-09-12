<template>
  <div class="page">
    <ProductionTopNav v-if="!props.embedded" active-key="jobCards" />

    <section class="panel">
      <div class="panel-head">
        <div><h2>工序记录</h2><p>工序卡已合入工单详情；这里保留只读兼容入口，执行请进入工位。</p></div>
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
            <th>商品 / 工单</th>
            <th>工序批次 / BOM</th>
            <th>状态 / 执行人</th>
            <th>工序记录</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td><strong>{{ row.product_name || '-' }}</strong><button class="link-button work-order-link" type="button" @click="openExecutionHub(row, 'job_card')">{{ row.work_order_no || '工单号缺失' }}</button></td>
            <td><strong>{{ operationLabel(row.operation) }}</strong><small>第 {{ row.sequence_no || 1 }} 道 · {{ row.workstation || '未分配工位' }}</small><small>{{ bomRecipeLabel(row) }}</small></td>
            <td><span class="status" :class="statusBadgeClass(row.status)">{{ jobCardStatusLabel(row.status) }}</span><small>执行人：{{ row.assigned_to || '未分配' }}</small><small>{{ row.operator ? `报工：${row.operator}` : '' }}</small></td>
            <td>
              <details class="record-details">
                <summary>{{ row.completed_at || row.started_at ? '查看记录' : '尚未执行' }}</summary>
                <div class="record-grid">
                  <span>工序要求<strong>{{ row.process_requirement || '按冻结工艺路线执行' }}</strong></span>
                  <span>工位产能<strong>{{ row.workstation_capacity_name || '-' }}</strong></span>
                  <span>计划分钟<strong>{{ row.planned_minutes || 0 }}</strong></span>
                  <span>实际分钟<strong>{{ row.actual_minutes || 0 }}</strong></span>
                  <span>投入数量<strong>{{ qty(row.actual_input_qty) }}</strong></span>
                  <span>产出数量<strong>{{ qty(row.actual_output_qty) }}</strong></span>
                  <span>计划工序成本<strong>{{ money(row.planned_operation_cost) }}</strong></span>
                  <span>实际工序成本<strong>{{ money(row.actual_operation_cost) }}</strong></span>
                  <span>实际损耗<strong>{{ qty(actualLossQty(row)) }} · {{ formatPercent(actualLossRate(row)) }}</strong></span>
                  <span>损耗原因<strong>{{ row.loss_reason || '-' }}</strong></span>
                  <span>异常原因<strong>{{ row.exception_reason || '-' }}</strong></span>
                </div>
              </details>
            </td>
            <td class="row-actions">
              <button class="primary compact" type="button" @click="openWorkstation(row)">进入工位</button>
              <button class="secondary compact" type="button" @click="openExecutionHub(row, 'job_card')">查看工单</button>
            </td>
          </tr>
          <tr v-if="!rows.length"><td colspan="5" class="muted">暂无工序记录</td></tr>
        </tbody>
      </table>
    </section>

    <ProductionExecutionHubDrawer
      :open="executionHub.open"
      :work-order-id="executionHub.workOrderId"
      :focus="executionHub.focus"
      :view-params="{ ...(props.viewParams || {}), work_order_id: executionHub.workOrderId, job_card_id: executionHub.jobCardId, focus: executionHub.focus }"
      @close="executionHub.open = false"
      @updated="load" />
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiGet } from '../api/client'
import ProductionExecutionHubDrawer from '../components/ProductionExecutionHubDrawer.vue'
import ProductionTopNav from '../components/ProductionTopNav.vue'
import { jobCardStatusLabel, jobCardStatusOptions } from '../lib/manufacturing-execution'
import { formatPercent } from '../lib/manufacturing-loss'

const props = defineProps({
  embedded: { type: Boolean, default: false },
  viewParams: { type: Object, default: () => ({}) },
})

const rows = ref([])
const status = ref('')
const loading = ref(false)
const error = ref('')
const executionHub = ref({ open: false, workOrderId: 0, jobCardId: 0, focus: '' })
const statusOptions = jobCardStatusOptions()

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

function operationLabel(operation) {
  if (operation === 'roast') return '生产'
  return operation || '-'
}

function bomRecipeLabel(row) {
  const bomID = Number(row?.bom_version_id || 0)
  return bomID > 0 ? `BOM版本 #${bomID}` : '默认 BOM/配方'
}

function openExecutionHub(row, focus = 'job_card') {
  const id = Number(row?.work_order_id || props.viewParams?.work_order_id || 0)
  if (!id) return
  executionHub.value = {
    open: true,
    workOrderId: id,
    jobCardId: Number(row?.id || row?.job_card_id || props.viewParams?.job_card_id || 0),
    focus,
  }
}

function openWorkstation(row) {
  const workOrderID = Number(row?.work_order_id || 0)
  const jobCardID = Number(row?.id || row?.job_card_id || 0)
  if (!workOrderID || !jobCardID) return
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: 'workstationView',
      params: {
        work_order_id: workOrderID,
        job_card_id: jobCardID,
        focus: 'workstation_task',
      },
      returnNavigation: {
        key: 'jobCards',
        params: {
          work_order_id: workOrderID,
          job_card_id: jobCardID,
          focus: 'job_card',
        },
      },
    },
  }))
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

async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/produce/job-cards', window.location.origin)
    if (status.value) url.searchParams.set('status', status.value)
    const data = await apiGet(url)
    rows.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
  if (Number(props.viewParams?.work_order_id || 0) > 0) {
    openExecutionHub({ work_order_id: Number(props.viewParams.work_order_id), job_card_id: Number(props.viewParams?.job_card_id || 0) }, props.viewParams?.focus || 'job_card')
  }
})
</script>

<style scoped>
.page{padding:16px;display:grid;gap:16px;background:#f7f8fa}.panel{border:1px solid #e2e7e4;border-radius:10px;padding:14px;background:#fff}.panel-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:12px}h2{margin:0;font-size:20px}.panel-head p{margin:4px 0 0;color:#6b7280;font-size:13px}.filters{display:grid;grid-template-columns:160px 90px;gap:10px;align-items:end}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,button{font:inherit;min-height:36px;border-radius:7px}select{width:100%;border:1px solid #d5dcd8;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #2f8f5b;background:#2f8f5b;color:#fff}.secondary{border:1px solid #b9c4be;background:#fff;color:#27463b}.compact{min-height:30px;padding:5px 10px}.link-button{display:block;border:0;background:transparent;color:#24704a;padding:3px 0 0;min-height:0;text-decoration:underline}.work-order-link{font-weight:600}.row-actions{display:flex;gap:6px;flex-wrap:wrap;min-width:170px}.status{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px;background:#f9fafb}.status.info{border-color:#93c5fd;background:#eff6ff;color:#1d4ed8}.status.warning{border-color:#efd4a5;background:#fff8e8;color:#9b5c08}.status.success{border-color:#bbf7d0;background:#f0fdf4;color:#15803d}.status.danger{border-color:#fecaca;background:#fef2f2;color:#b91c1c}.status.neutral{border-color:#d1d5db;background:#f9fafb;color:#374151}.table-wrap{overflow:auto}table{width:100%;min-width:780px;border-collapse:collapse}th,td{border-bottom:1px solid #edf0ee;padding:10px;text-align:left;font-size:13px;vertical-align:top}th{background:#f7f9f8}td small{display:block;color:#6b7280;margin-top:3px}.record-details summary{cursor:pointer;color:#24704a;white-space:nowrap}.record-grid{display:grid;grid-template-columns:repeat(2,minmax(120px,1fr));gap:7px;margin-top:8px;min-width:360px}.record-grid span{display:grid;gap:2px;color:#6b7280;font-size:11px}.record-grid strong{color:#273b34;font-size:12px}.muted{color:#666;text-align:center}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}@media(max-width:760px){.page{padding:12px}.panel-head{align-items:stretch;flex-direction:column}.table-wrap{overflow:visible}table,thead,tbody,tr,th,td{display:block}thead{display:none}table{min-width:0}tr{border:1px solid #e2e7e4;border-radius:9px;margin-bottom:9px;padding:8px}td{border:0;padding:5px}.record-grid{grid-template-columns:1fr;min-width:0}}
</style>
