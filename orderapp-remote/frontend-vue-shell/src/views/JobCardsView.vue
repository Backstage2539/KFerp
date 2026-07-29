<template>
  <div class="page">
    <ProductionTopNav v-if="!props.embedded" active-key="jobCards" />

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
            <th>工单</th>
            <th>商品</th>
            <th>BOM/配方</th>
            <th>顺序</th>
            <th>工序</th>
            <th>工序要求</th>
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
            <th>开始时间</th>
            <th>暂停时间</th>
            <th>继续时间</th>
            <th>完成时间</th>
            <th>操作人</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td><button class="link-button work-order-link" type="button" @click="openExecutionHub(row, 'job_card')">{{ row.work_order_no || '工单号缺失' }}</button></td>
            <td>{{ row.product_name || '-' }}</td>
            <td>{{ bomRecipeLabel(row) }}</td>
            <td>{{ row.sequence_no || 1 }}</td>
            <td>{{ operationLabel(row.operation) }}<small v-if="row.records_loss">记录损耗</small></td>
            <td class="requirement-cell">{{ row.process_requirement || '按冻结工艺路线执行' }}</td>
            <td>{{ row.workstation || '-' }}</td>
            <td>{{ row.workstation_capacity_name || '-' }}</td>
            <td><span class="status" :class="statusBadgeClass(row.status)">{{ jobCardStatusLabel(row.status) }}</span></td>
            <td>{{ row.planned_minutes || 0 }}</td>
            <td>{{ money(row.planned_operation_cost) }}</td>
            <td>{{ row.actual_minutes || 0 }}</td>
            <td>{{ money(row.actual_operation_cost) }}</td>
            <td>
              <strong>{{ qty(actualLossQty(row)) }}</strong>
              <small>{{ formatPercent(actualLossRate(row)) }}</small>
            </td>
            <td>{{ row.loss_reason || '-' }}</td>
            <td>{{ row.exception_reason || '-' }}</td>
            <td>{{ row.started_at || '-' }}</td>
            <td>{{ row.paused_at || '-' }}</td>
            <td>{{ row.resumed_at || '-' }}</td>
            <td>{{ row.completed_at || '-' }}</td>
            <td>{{ row.operator || '-' }}</td>
            <td class="row-actions">
              <button class="primary compact" type="button" @click="openWorkstation(row)">进入工位</button>
              <button class="secondary compact" type="button" @click="openExecutionHub(row, 'job_card')">执行枢纽</button>
            </td>
          </tr>
          <tr v-if="!rows.length"><td colspan="22" class="muted">暂无工序卡</td></tr>
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
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #eee;border-radius:8px;padding:12px;background:#fff}.panel-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:12px}h2{margin:0;font-size:18px}.filters{display:grid;grid-template-columns:160px 90px;gap:10px;align-items:end}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,button{font:inherit;min-height:36px;border-radius:6px}select{width:100%;border:1px solid #ddd;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #999;background:#fff;color:#111}.compact{min-height:30px;padding:5px 10px}.link-button{border:0;background:transparent;color:#1d4ed8;padding:0;min-height:0;text-decoration:underline}.work-order-link{font-weight:600}.row-actions{display:flex;gap:6px;flex-wrap:wrap;min-width:170px}.status{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px;background:#f9fafb}.status.info{border-color:#93c5fd;background:#eff6ff;color:#1d4ed8}.status.warning{border-color:#fed7aa;background:#fff7ed;color:#c2410c}.status.success{border-color:#bbf7d0;background:#f0fdf4;color:#15803d}.status.danger{border-color:#fecaca;background:#fef2f2;color:#b91c1c}.status.neutral{border-color:#d1d5db;background:#f9fafb;color:#374151}.table-wrap{overflow:auto}table{width:100%;min-width:1880px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px;vertical-align:top}th{background:#fbfbfb}td small{display:block;color:#6b7280;margin-top:3px}.requirement-cell{min-width:190px;max-width:280px;white-space:normal;line-height:1.45}.muted{color:#666;text-align:center}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}
</style>
