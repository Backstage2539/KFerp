<template>
  <div v-if="open" class="drawer-mask" @click.self="$emit('close')">
    <aside class="execution-hub" :data-focus="focusState.section" aria-label="生产执行枢纽">
      <div class="drawer-head">
        <div>
          <div class="eyebrow">生产执行枢纽</div>
          <h2>{{ header.work_order_no || workOrderLabel }}</h2>
          <p>{{ header.product_name || '-' }} · {{ header.spec_g || 0 }}g · 计划 {{ formatG(header.planned_g) }}</p>
        </div>
        <button class="secondary compact" type="button" @click="$emit('close')">关闭</button>
      </div>

      <div v-if="loading" class="notice">加载中</div>
      <div v-else-if="error" class="error">{{ error }}</div>
      <template v-else>
        <section class="summary-grid">
          <div><span>工单状态</span><strong>{{ header.status || '-' }}</strong></div>
          <div><span>BOM / 路线</span><strong>{{ hub.bom_summary || '-' }}</strong><small>{{ hub.route_summary || '-' }}</small></div>
          <div><span>负责人 / 工位</span><strong>{{ assignment.assigned_to || readiness.next_handler || '-' }}</strong><small>{{ assignment.work_center || '未分配工位' }}</small></div>
          <div><span>成本</span><strong>{{ money(hub.cost_summary?.total_cost) }}</strong></div>
        </section>

        <section class="readiness-panel" :class="readinessTone">
          <div>
            <div class="section-title">执行 readiness</div>
            <p>{{ readinessText }}</p>
          </div>
          <div class="readiness-flags">
            <span>开始 {{ readiness.can_start ? '可执行' : '受阻' }}</span>
            <span>完成 {{ readiness.can_complete ? '可执行' : '受阻' }}</span>
            <span>下一处理人 {{ readiness.next_handler || '-' }}</span>
          </div>
          <div v-if="readiness.blocking_reasons?.length" class="reason-list">
            <article v-for="reason in readiness.blocking_reasons" :key="reason.code">
              <strong>{{ reason.label }}</strong>
              <span>{{ reason.next_handler || readiness.next_handler || '-' }}</span>
            </article>
          </div>
        </section>

        <section class="action-row">
          <button
            v-for="action in contextActions"
            :key="action.key"
            type="button"
            :disabled="action.disabled"
            :class="{ primary: action.key === readiness.suggested_action || action.key === 'productionIssue' }"
            :title="action.reason || action.label"
            @click="navigate(action)">
            {{ action.label }}
          </button>
        </section>

        <section class="status-grid">
          <article class="wip-card" :class="{ shortage: wipHasShortage }">
            <div class="section-title">{{ wipHasShortage ? 'WIP库存不足' : 'WIP 状态' }}</div>
            <strong>{{ wipStatus.status || (wipHasShortage ? '库存不足' : '-') }}</strong>
            <div v-if="wipStatus.materials?.length" class="wip-materials">
              <div v-for="row in wipStatus.materials" :key="row.material_id || row.material_name">
                <strong>{{ row.material_name || row.name || `物料 ${row.material_id || '-'}` }}</strong>
                <span>需求 {{ materialQuantity(row, 'required_qty') }}</span>
                <span>可用 {{ materialQuantity(row, 'available_qty') }}</span>
                <span :class="{ 'danger-text': materialShortage(row) > 0 }">缺口 {{ materialQuantity(row, 'shortage_qty') }}</span>
              </div>
            </div>
            <p v-else>需求 {{ formatG(wipStatus.required_g) }} · 可用 {{ formatG(wipStatus.available_g ?? wipStatus.reserved_g) }} · 缺口 {{ formatG(wipStatus.shortage_g) }}</p>
            <small v-if="wipStatus.blocking_reason">{{ wipStatus.blocking_reason }}</small>
            <button v-if="wipHasShortage && productionIssueAction" class="primary compact issue-action" type="button" @click="navigate(productionIssueAction)">生产领料</button>
          </article>
          <article>
            <div class="section-title">质检状态</div>
            <strong>{{ qualityStatus.status || '-' }}</strong>
            <p>{{ qualityStatus.reference_no || header.work_order_no || '-' }} · {{ qualityStatus.result || '-' }}</p>
            <small v-if="qualityStatus.note">{{ qualityStatus.note }}</small>
          </article>
        </section>

        <section>
          <div class="section-title">工序进度</div>
          <div class="operation-list">
            <article v-for="row in hub.operation_progress || []" :key="row.job_card_id || row.sequence_no" :class="{ focused: focusState.job_card_id === row.job_card_id }">
              <strong>{{ row.sequence_no || '-' }}. {{ row.operation || '工序' }}</strong>
              <span>{{ row.status_label || row.status || '-' }}</span>
              <small>{{ row.workstation || '未分配工位' }} · {{ row.assigned_to || row.operator || '-' }} · {{ row.planned_minutes || 0 }} 分钟</small>
              <em v-if="row.blocking_reason">{{ row.blocking_reason }}</em>
            </article>
            <p v-if="!(hub.operation_progress || []).length" class="muted">暂无工序进度</p>
          </div>
        </section>

        <section>
          <div class="section-title-row">
            <div class="section-title">追溯 timeline</div>
            <div class="filter-tabs">
              <button
                v-for="item in filters"
                :key="item.key"
                type="button"
                :class="{ active: timelineFilter === item.key }"
                @click="timelineFilter = item.key">
                {{ item.label }}
              </button>
            </div>
          </div>
          <div class="timeline">
            <article v-for="item in visibleTimeline" :key="`${item.type}-${item.ref_type}-${item.ref_id}-${item.title}`">
              <span>{{ item.type }}</span>
              <strong>{{ item.title }}</strong>
              <small>{{ item.at || '-' }} · {{ item.summary || '-' }}</small>
            </article>
            <p v-if="!visibleTimeline.length" class="muted">暂无追溯记录</p>
          </div>
        </section>
      </template>
    </aside>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { apiGet } from '../api/client'
import {
  buildExecutionHubActions,
  buildExecutionHubFocus,
  executionHubTimelineFilters,
  filterExecutionHubTimeline,
  readinessBadgeTone,
} from '../lib/production-execution-hub'

const props = defineProps({
  open: { type: Boolean, default: false },
  workOrderId: { type: Number, default: 0 },
  focus: { type: String, default: '' },
  viewParams: { type: Object, default: () => ({}) },
})

defineEmits(['close'])

const loading = ref(false)
const error = ref('')
const detail = ref({})
const timelineFilter = ref('all')
const filters = executionHubTimelineFilters()

const hub = computed(() => detail.value.execution_hub || {})
const header = computed(() => hub.value.header || detail.value.work_order || {})
const assignment = computed(() => hub.value.workstation_assignment || {})
const readiness = computed(() => hub.value.readiness || {})
const wipStatus = computed(() => hub.value.wip_status || {})
const qualityStatus = computed(() => hub.value.quality_status || {})
const readinessTone = computed(() => readinessBadgeTone(readiness.value))
const readinessText = computed(() => {
  if (readiness.value.blocking_reasons?.length) return readiness.value.blocking_reasons.map((row) => row.label).join(' / ')
  if (readiness.value.can_start) return '可开始生产'
  if (readiness.value.can_complete) return '可完工入库'
  return '查看下一处理动作'
})
const focusState = computed(() => buildExecutionHubFocus({ ...(props.viewParams || {}), focus: props.focus }))
const fallbackActions = computed(() => buildExecutionHubActions({ ...hub.value, work_order: header.value, job_cards: detail.value.job_cards }))
const contextActions = computed(() => {
  const actions = hub.value.context_actions?.length ? hub.value.context_actions : fallbackActions.value
  return actions.map((action) => {
    let params = action.params || {}
    if (action.view === 'stockOperations') {
      params = { ...(fallbackActions.value.find((fallback) => fallback.key === action.key)?.params || params) }
      if (focusState.value.section === 'job_card' && focusState.value.job_card_id) params.job_card_id = focusState.value.job_card_id
    }
    return { ...action, params, disabled: Boolean(action.disabled) }
  })
})
const productionIssueAction = computed(() => contextActions.value.find((action) => action.key === 'productionIssue'))
const wipHasShortage = computed(() => {
  if (Number(wipStatus.value.shortage_qty || wipStatus.value.shortage_g || wipStatus.value.shortage_units || 0) > 0) return true
  return (wipStatus.value.materials || []).some((row) => materialShortage(row) > 0)
})
const visibleTimeline = computed(() => filterExecutionHubTimeline(hub.value.trace_timeline || [], timelineFilter.value))
const workOrderLabel = computed(() => '工单')

function formatG(value) {
  return `${Number(value || 0).toLocaleString('zh-CN')}g`
}

function money(value) {
  return Number(value || 0).toFixed(2)
}

function materialShortage(row = {}) {
  return Number(row.shortage_qty ?? row.shortage_g ?? row.shortage_units ?? 0)
}

function materialQuantity(row = {}, field) {
  let value = Number(row[field] ?? 0)
  let unit = String(row.inventory_unit || '').trim()
  if (!value && field === 'required_qty') value = Number(row.required_g || row.required_units || 0)
  if (!value && field === 'available_qty') value = Number(row.available_g || row.available_units || 0)
  if (!value && field === 'shortage_qty') value = Number(row.shortage_g || row.shortage_units || 0)
  if (!unit) unit = Number(row[`${field.replace('_qty', '')}_units`] || 0) > 0 ? '件' : 'g'
  return `${value.toLocaleString('zh-CN')} ${unit}`
}

function navigate(action) {
  if (!action?.view || action.disabled) return
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: action.view, params: action.params || {} } }))
}

async function load() {
  if (!props.open || !props.workOrderId) return
  loading.value = true
  error.value = ''
  try {
    detail.value = await apiGet(`/api/produce/work-orders/${props.workOrderId}`)
  } catch (err) {
    error.value = err.message || '加载执行枢纽失败'
  } finally {
    loading.value = false
  }
}

watch(() => [props.open, props.workOrderId], load, { immediate: true })
</script>

<style scoped>
.drawer-mask{position:fixed;inset:0;background:rgba(17,24,39,.28);z-index:60;display:flex;justify-content:flex-end}.execution-hub{width:min(980px,94vw);height:100%;overflow:auto;background:#fff;padding:18px;box-shadow:-14px 0 30px rgba(15,23,42,.18);display:grid;align-content:start;gap:14px}.drawer-head,.section-title-row{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.drawer-head{border-bottom:1px solid #e5e7eb;padding-bottom:12px}.eyebrow{font-size:12px;color:#6b7280}.drawer-head h2{margin:2px 0 4px;font-size:20px}.drawer-head p{margin:0;color:#6b7280}.compact{min-height:30px;padding:5px 10px}.secondary{border:1px solid #9ca3af;background:#fff;color:#111}.primary{border:1px solid #111;background:#111;color:#fff}button{font:inherit;border-radius:6px;padding:8px 12px;min-height:34px;cursor:pointer}button:disabled{opacity:.55;cursor:not-allowed}.summary-grid,.status-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.status-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.summary-grid div,.status-grid article{border:1px solid #e5e7eb;border-radius:8px;padding:10px;background:#fff}.status-grid article.wip-card.shortage{border-color:#fca5a5;background:#fef2f2}.summary-grid span{display:block;color:#6b7280;font-size:12px}.summary-grid strong,.status-grid strong{display:block;margin-top:4px}.summary-grid small,.status-grid small{display:block;color:#6b7280;margin-top:4px}.wip-materials{display:grid;gap:6px;margin-top:8px}.wip-materials>div{display:grid;grid-template-columns:minmax(130px,1.4fr) repeat(3,minmax(86px,1fr));gap:8px;align-items:center;border-top:1px solid #fecaca;padding-top:6px}.wip-materials strong{margin:0}.wip-materials span{color:#4b5563;font-size:12px}.danger-text{color:#b91c1c!important;font-weight:700}.issue-action{margin-top:10px}.readiness-panel{border:1px solid #e5e7eb;border-radius:8px;padding:12px;display:grid;gap:10px}.readiness-panel.danger{border-color:#fecaca;background:#fef2f2}.readiness-panel.warning{border-color:#fde68a;background:#fffbeb}.readiness-panel.success{border-color:#bbf7d0;background:#f0fdf4}.readiness-panel p{margin:4px 0 0}.readiness-flags,.action-row,.filter-tabs{display:flex;flex-wrap:wrap;gap:8px}.readiness-flags span{border:1px solid #d1d5db;border-radius:999px;padding:3px 8px;background:#fff;font-size:12px}.reason-list{display:grid;gap:8px}.reason-list article{border:1px solid #f1f5f9;border-radius:6px;background:#fff;padding:8px;display:flex;justify-content:space-between;gap:10px}.section-title{font-weight:700}.operation-list,.timeline{display:grid;gap:8px}.operation-list article,.timeline article{border:1px solid #eef2f7;border-radius:8px;padding:9px;background:#fff}.operation-list article.focused{border-color:#111;box-shadow:0 0 0 1px #111 inset}.operation-list strong,.timeline strong{display:block}.operation-list small,.timeline small{display:block;color:#6b7280;margin-top:3px}.operation-list em{display:block;color:#b91c1c;font-style:normal;margin-top:3px}.filter-tabs button{border:1px solid #d1d5db;background:#fff}.filter-tabs button.active{border-color:#111;background:#111;color:#fff}.timeline span{font-size:12px;color:#2563eb}.muted{color:#6b7280;text-align:center}.notice{border:1px solid #bfdbfe;background:#eff6ff;border-radius:8px;padding:10px}.error{border:1px solid #fecaca;background:#fef2f2;border-radius:8px;padding:10px;color:#991b1b}@media (max-width:760px){.execution-hub{width:100vw}.summary-grid,.status-grid{grid-template-columns:1fr}.wip-materials>div{grid-template-columns:1fr 1fr}.drawer-head,.section-title-row{display:grid}}
</style>
