<template>
  <section class="capacity-workspace">
    <header class="workspace-header">
      <button class="back-button" type="button" @click="$emit('back')">← 返回生产计划</button>
      <div class="header-main">
        <div>
          <div class="eyebrow">生产计划 · 产能拆分</div>
          <div class="title-row">
            <h1>{{ detail.plan_no || '生产计划' }}</h1>
            <span class="status-pill">{{ isDraft ? '草稿' : '只读' }}</span>
            <span v-if="dirty" class="unsaved-pill">未保存</span>
          </div>
          <p>按本次计划冻结的工艺路线安排工位和产量，工序名称随计划实际配置展示。</p>
        </div>
        <button class="secondary" type="button" :disabled="loading" @click="$emit('refresh')">重新计算</button>
      </div>
      <div class="plan-summary">
        <article><span>计划内容</span><strong>{{ productSummary }}</strong><small>{{ itemCount }} 项生产任务</small></article>
        <article><span>本次工序</span><strong>{{ groups.length }} 个</strong><small>{{ groups.map((group) => group.operation).join(' → ') || '暂无工序' }}</small></article>
        <article><span>来源订单</span><strong>{{ orderCount }} 张</strong><small>每条任务保留订单数量</small></article>
        <article><span>拆分状态</span><strong>{{ readinessTitle }}</strong><small>{{ readinessHint }}</small></article>
      </div>
    </header>

    <div v-if="loading" class="workspace-state">正在加载本次计划的工序和产能…</div>
    <div v-else-if="error && !groups.length" class="workspace-state error-state">{{ error }}</div>
    <div v-else class="workspace-body">
      <main class="workspace-main">
        <section class="operation-overview">
          <div>
            <span class="section-index">工序</span>
            <div><h2>本次计划涉及 {{ groups.length }} 个工序</h2><p>先选择工序，再逐项核对任务数量和工位安排。</p></div>
          </div>
          <nav class="operation-tabs" aria-label="本次计划工序">
            <button
              v-for="(group, index) in groups"
              :key="group.key"
              type="button"
              :class="{ active: group.key === activeGroupKey }"
              @click="activeGroupKey = group.key"
            >
              <span>{{ String(index + 1).padStart(2, '0') }}</span>
              <strong>{{ group.operation }}</strong>
              <em :class="statusTone(groupStatus(group))">{{ statusLabel(groupStatus(group)) }}</em>
            </button>
          </nav>
        </section>

        <section v-if="activeGroup" class="operation-section">
          <div class="operation-heading">
            <div><span class="operation-seq">{{ activeGroupIndex }}</span><div><h2>{{ activeGroup.operation }}</h2><p>{{ activeGroup.tasks.length }} 项任务，以下名称和顺序来自本计划的工艺快照。</p></div></div>
            <button class="secondary" type="button" :disabled="saving || !isDraft" @click="autoGroup(activeGroup)">自动安排本工序</button>
          </div>

          <article v-for="task in activeGroup.tasks" :key="task.key" class="task-card">
            <div class="task-head">
              <div>
                <span class="task-kind">{{ taskKind(task.item) }}</span>
                <h3>{{ task.name }} <b v-if="task.spec_label">· {{ task.spec_label }}</b></h3>
                <p>{{ task.sources.length ? `${task.sources.length} 条来源任务` : task.order_nos.length ? `${task.order_nos.length} 张来源订单` : '备货任务' }}</p>
              </div>
              <span :class="['task-status', statusTone(task.status)]">{{ statusLabel(task.status) }}</span>
            </div>

            <div class="quantity-summary">
              <article><span>计划数量</span><strong>{{ task.required_label }}</strong></article>
              <article><span>已安排</span><strong>{{ task.arranged_label }}</strong></article>
              <article :class="statusTone(task.status)"><span>还需安排</span><strong>{{ task.remaining_label }}</strong></article>
            </div>

            <div v-if="task.sources.length || task.order_nos.length" class="source-orders">
              <div class="subsection-title"><strong>来源订单</strong><span>订单任务不会因商品相同而丢失</span></div>
              <table v-if="task.sources.length">
                <thead><tr><th>订单号</th><th>客户</th><th>任务数量</th></tr></thead>
                <tbody>
                  <tr v-for="source in task.sources" :key="source.key"><td>{{ source.order_no || '-' }}</td><td>{{ source.customer_name || '-' }}</td><td>{{ source.quantity_label }}</td></tr>
                </tbody>
              </table>
              <div v-else class="order-chips"><span v-for="orderNo in task.order_nos" :key="orderNo">{{ orderNo }}</span></div>
            </div>

            <div class="allocation-section">
              <div class="subsection-title">
                <div><strong>工位产能</strong><span>可拆到多个工位，数量实时汇总</span></div>
                <div class="allocation-actions">
                  <button type="button" :disabled="saving || !isDraft" @click="$emit('auto', task.item, task.operation)">自动拆分</button>
                  <button type="button" :disabled="saving || !isDraft" @click="$emit('add', task.item, task.operation)">添加工位</button>
                </div>
              </div>
              <div v-for="split in taskSplits(task)" :key="split.local_key || split.id" class="allocation-row">
                <label class="capacity-field"><span>工位 / 产能</span><select :value="split.workstation_capacity_id || 0" :disabled="saving || !isDraft" @change="changeCapacity(split, $event)"><option :value="0">请选择适用产能</option><option v-for="capacity in taskCapacities(task)" :key="capacity.id" :value="capacity.id">{{ capacityLabel(capacity) }}</option></select></label>
                <label class="quantity-field"><span>承担产量（{{ split.batch_size_unit || task.quantity_unit }}）</span><input :value="split.planned_qty" type="number" min="0" :step="quantityStep(split)" :disabled="saving || !isDraft" @input="changeQuantity(split, $event)" /></label>
                <div class="metric"><span>批次</span><strong>{{ metrics(split).planned_batch_count }}</strong></div>
                <div class="metric"><span>预计用时</span><strong>{{ metrics(split).planned_minutes }} 分钟</strong></div>
                <div class="metric"><span>工序成本</span><strong>¥{{ metrics(split).planned_operation_cost }}</strong></div>
                <button class="remove-button" type="button" :disabled="saving || !isDraft" @click="$emit('remove', split)">删除</button>
                <div v-if="batchCards(split).length" class="batch-summary">
                  <span v-for="batch in batchCards(split)" :key="`${split.local_key}-${batch.label}`" :class="{ underfilled: batch.underfilled }">{{ batch.label }} {{ quantityLabel(batch.planned_qty, batch.batch_size_unit) }}<em v-if="batch.underfilled">尾批</em></span>
                </div>
              </div>
              <div v-if="!taskSplits(task).length" class="empty-allocation">尚未安排工位。保存草稿允许暂缺，确认安排前必须补齐。</div>
            </div>
          </article>
        </section>

        <section v-else class="workspace-state">本计划没有可拆分的工序，请先检查工艺路线。</section>
      </main>

      <aside class="review-panel">
        <div class="review-title"><span class="section-index">核对</span><div><h2>拆分核对</h2><p>按任务逐项判断，合计相同也不能抵消单项差异。</p></div></div>
        <div v-if="previewLoading" class="review-loading">正在重新计算…</div>
        <div v-if="previewError" class="review-error">{{ previewError }}</div>
        <div class="review-list">
          <article v-for="group in groups" :key="`review-${group.key}`">
            <div class="review-operation"><strong>{{ group.operation }}</strong><span :class="statusTone(groupStatus(group))">{{ statusLabel(groupStatus(group)) }}</span></div>
            <div v-for="task in group.tasks" :key="`review-${task.key}`" class="review-task"><div><strong>{{ task.name }}</strong><small>{{ task.spec_label || taskKind(task.item) }}</small></div><div><span>{{ task.arranged_label }} / {{ task.required_label }}</span><em :class="statusTone(task.status)">{{ statusLabel(task.status) }}</em></div></div>
          </article>
        </div>
        <label v-if="readiness.over_count" class="over-ack"><input v-model="overAcknowledged" type="checkbox" /><span>我已核对 {{ readiness.over_count }} 项超排数量，确认按当前数量保存。</span></label>
        <div class="source-check">
          <strong>用料与来源</strong>
          <p v-if="sourceIssueCount">仍有 {{ sourceIssueCount }} 项来源或供应问题；可先保存拆分，返回计划详情继续处理。</p>
          <p v-else>当前计划没有来源仓库或供应缺口阻断。</p>
        </div>
        <div class="review-note"><strong>确认安排的含义</strong><p>确认只保存本次产能拆分并返回生产计划，不会提交计划，也不会生成工单。</p></div>
      </aside>
    </div>

    <footer class="workspace-footer">
      <div><strong>{{ footerTitle }}</strong><span>{{ footerHint }}</span></div>
      <div class="footer-actions">
        <button class="secondary" type="button" :disabled="saving || !dirty" @click="$emit('save')">{{ saving ? '正在保存…' : '保存草稿' }}</button>
        <button class="primary" type="button" :disabled="saving || previewLoading || !readiness.can_confirm" @click="$emit('confirm')">确认安排</button>
      </div>
    </footer>
  </section>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import {
  applicableOperationCapacities,
  buildProductionPlanCapacityGroups,
  operationSplitPreviewStatusLabel,
  operationSplitPreviewStatusTone,
  plannedCapacitySplitMetrics,
  productionPlanCapacityReadiness,
  productionPlanSplitBatchCards,
} from '../lib/produce-plan'

const props = defineProps({
  detail: { type: Object, required: true },
  rows: { type: Array, default: () => [] },
  capacities: { type: Array, default: () => [] },
  preview: { type: Object, default: null },
  loading: { type: Boolean, default: false },
  previewLoading: { type: Boolean, default: false },
  saving: { type: Boolean, default: false },
  error: { type: String, default: '' },
  previewError: { type: String, default: '' },
  dirty: { type: Boolean, default: false },
})
const emit = defineEmits(['back', 'save', 'confirm', 'refresh', 'auto', 'add', 'remove', 'capacity-change', 'quantity-change'])

const groups = computed(() => buildProductionPlanCapacityGroups(props.detail, props.preview || {}))
const activeGroupKey = ref('')
const overAcknowledged = ref(false)
watch(groups, (next) => { if (!next.some((group) => group.key === activeGroupKey.value)) activeGroupKey.value = next[0]?.key || '' }, { immediate: true })
watch(() => props.preview, () => { overAcknowledged.value = false })
const activeGroup = computed(() => groups.value.find((group) => group.key === activeGroupKey.value) || groups.value[0] || null)
const activeGroupIndex = computed(() => String(Math.max(0, groups.value.findIndex((group) => group.key === activeGroup.value?.key)) + 1).padStart(2, '0'))
const readiness = computed(() => productionPlanCapacityReadiness(props.preview || {}, overAcknowledged.value))
const isDraft = computed(() => String(props.detail?.status || '') === 'draft')
const productNames = computed(() => [...new Set((props.detail?.items || []).map((item) => item.output_name || item.product_name).filter(Boolean))])
const productSummary = computed(() => productNames.value.slice(0, 2).join('、') || '备货计划')
const itemCount = computed(() => (props.detail?.items || []).length)
const orderCount = computed(() => new Set((props.detail?.items || []).flatMap((item) => (item.demand_sources || []).map((row) => row.order_no).concat(String(item.order_nos || '').split(','))).map((value) => String(value || '').trim()).filter(Boolean)).size)
const sourceIssueCount = computed(() => (props.detail?.readiness?.issues || []).filter((issue) => ['source', 'supply'].includes(String(issue.category || ''))).length)
const readinessTitle = computed(() => readiness.value.can_confirm ? '可以确认' : readiness.value.short_count ? `${readiness.value.short_count} 项待补` : readiness.value.requires_over_acknowledgement ? '超排待核对' : '正在核对')
const readinessHint = computed(() => readiness.value.can_confirm ? '所有任务已覆盖' : '草稿仍可保存')
const footerTitle = computed(() => props.dirty ? '当前拆分尚未保存' : readiness.value.can_confirm ? '拆分已保存并满足确认条件' : '拆分已保存，仍有任务待安排')
const footerHint = computed(() => readiness.value.short_count ? `${readiness.value.short_count} 项任务不足；可保存草稿，补齐后再确认。` : readiness.value.requires_over_acknowledgement ? '勾选超排核对后可以确认。' : '确认后返回生产计划详情，计划仍保持草稿。')

function statusLabel(status) { return operationSplitPreviewStatusLabel(status) }
function statusTone(status) { return operationSplitPreviewStatusTone(status) }
function taskKind(item) { return String(item?.output_type || 'product') === 'material' ? '自制物料' : '商品任务' }
function operationIdentity(row = {}) { return { seq: Number(row.seq || row.operation_seq || row.sequence_no || 0), id: Number(row.operation_id || row.id || 0), name: String(row.operation || row.name || '').trim() } }
function splitMatches(split, operation) { const left = operationIdentity(split); const right = operationIdentity(operation); if (left.id > 0 && right.id > 0) return left.id === right.id; if (left.seq > 0 && right.seq > 0) return left.seq === right.seq; return left.name === right.name }
function taskSplits(task) { return props.rows.filter((split) => Number(split.production_plan_item_id || 0) === Number(task.item.id || 0) && splitMatches(split, task.operation)) }
function taskCapacities(task) { return applicableOperationCapacities(task.operation, props.capacities) }
function metrics(split) { return plannedCapacitySplitMetrics(split) }
function batchCards(split) { return productionPlanSplitBatchCards(split) }
function quantityStep(split) { return ['g', '克'].includes(String(split.batch_size_unit || '').trim().toLowerCase()) ? 1 : 0.001 }
function quantityLabel(value, unit = '') { const number = Number(value || 0); return `${Number(number.toFixed(3)).toLocaleString('zh-CN', { maximumFractionDigits: 3 })}${unit || ''}` }
function capacityLabel(capacity) { return `${capacity.name || '未命名产能'} · ${quantityLabel(capacity.batch_size_qty, capacity.batch_size_unit)} / 批 · ${capacity.standard_minutes || 0} 分钟` }
function changeCapacity(split, event) { emit('capacity-change', split, Number(event.target.value || 0)) }
function changeQuantity(split, event) { emit('quantity-change', split, Number(event.target.value || 0)) }
function groupStatus(group) { const statuses = group.tasks.map((task) => task.status); if (statuses.some((status) => ['short', 'missing'].includes(status))) return 'short'; if (statuses.some((status) => status === 'over')) return 'over'; return statuses.length ? 'matched' : 'missing' }
function autoGroup(group) { for (const task of group.tasks) emit('auto', task.item, task.operation) }
</script>

<style scoped>
.capacity-workspace{--ink:#162f29;--muted:#677772;--line:#dde6e1;--green:#16834f;--green-dark:#11653e;--green-soft:#eaf6ef;--amber:#c66a0b;--amber-soft:#fff7e9;min-height:100vh;background:#f7f9f7;color:var(--ink);padding-bottom:82px}.workspace-header{position:sticky;top:0;z-index:12;padding:18px 28px;background:rgba(255,255,255,.97);border-bottom:1px solid var(--line);box-shadow:0 6px 22px rgba(22,47,41,.05)}.back-button{border:0;background:none;color:#1669b2;font-weight:700;padding:0;margin-bottom:12px;cursor:pointer}.header-main,.title-row,.operation-overview>div,.operation-heading,.operation-heading>div,.task-head,.subsection-title,.allocation-actions,.review-title,.review-operation,.workspace-footer{display:flex;align-items:center}.header-main{justify-content:space-between;gap:20px}.eyebrow,.section-index,.task-kind{font-size:11px;font-weight:800;letter-spacing:.06em;color:#71817c}.title-row{gap:10px;margin:4px 0}.title-row h1{font-size:25px;margin:0}.header-main p,.operation-overview p,.operation-heading p{margin:0;color:var(--muted);font-size:13px}.status-pill,.unsaved-pill,.task-status{padding:5px 9px;border-radius:6px;font-size:12px;font-weight:800}.status-pill{background:#eef2f0}.unsaved-pill{background:#fff0d8;color:#92580b}.plan-summary{display:grid;grid-template-columns:1.4fr .8fr .8fr 1fr;gap:10px;margin-top:16px;padding:13px 16px;border-radius:10px;background:linear-gradient(90deg,#edf8f2,#f5faf7)}.plan-summary article{display:grid;gap:2px;min-width:0;padding-right:12px;border-right:1px solid #dcebe3}.plan-summary article:last-child{border:0}.plan-summary span,.plan-summary small{font-size:11px;color:var(--muted)}.plan-summary strong{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.workspace-body{display:grid;grid-template-columns:minmax(0,1fr) 310px;gap:20px;max-width:1480px;margin:0 auto;padding:22px 28px}.workspace-main{display:grid;gap:16px}.operation-overview,.operation-section,.review-panel{background:#fff;border:1px solid var(--line);border-radius:12px;box-shadow:0 7px 24px rgba(22,47,41,.04)}.operation-overview{padding:18px 20px}.operation-overview>div{align-items:flex-start;gap:10px}.operation-overview h2,.operation-heading h2,.review-title h2{margin:0 0 4px;font-size:18px}.section-index,.operation-seq{display:grid;place-items:center;min-width:34px;height:26px;border-radius:6px;background:var(--green-soft);color:var(--green);font-size:12px;font-weight:850}.operation-tabs{display:flex;gap:8px;margin-top:15px;overflow-x:auto}.operation-tabs button{display:grid;grid-template-columns:auto 1fr auto;align-items:center;gap:8px;min-width:180px;padding:10px 12px;border:1px solid var(--line);border-radius:8px;background:#fff;color:var(--ink);text-align:left;cursor:pointer}.operation-tabs button.active{border-color:#7fc39f;background:var(--green-soft);box-shadow:inset 3px 0 var(--green)}.operation-tabs button>span{display:grid;place-items:center;width:24px;height:24px;border-radius:50%;background:#edf2ef;color:var(--green);font-size:11px}.operation-tabs em,.review-operation span,.review-task em{font-size:11px;font-style:normal;font-weight:800}.matched{color:var(--green-dark)}.short,.missing{color:#b85c0a}.over{color:#8a4b12}.operation-section{padding:20px}.operation-heading{justify-content:space-between;gap:18px;padding-bottom:16px;border-bottom:1px solid var(--line)}.operation-heading>div{gap:11px}.operation-heading p{font-size:12px}.task-card{margin-top:16px;padding:17px;border:1px solid var(--line);border-radius:11px;background:#fff}.task-head{justify-content:space-between;gap:20px}.task-head h3{font-size:16px;margin:4px 0}.task-head h3 b{font-weight:650;color:#52645e}.task-head p{margin:0;font-size:11px;color:var(--muted)}.task-status.matched{background:var(--green-soft)}.task-status.short,.task-status.missing,.task-status.over{background:var(--amber-soft)}.quantity-summary{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px;margin:14px 0}.quantity-summary article{display:grid;gap:3px;padding:10px 12px;border-radius:8px;background:#f4f7f5}.quantity-summary span{font-size:11px;color:var(--muted)}.quantity-summary strong{font-size:17px}.subsection-title{justify-content:space-between;gap:12px;margin-bottom:9px}.subsection-title>div{display:grid;gap:2px}.subsection-title span{font-size:11px;color:var(--muted);margin-left:8px}.source-orders{padding:12px;border:1px solid #e6ece8;border-radius:8px;background:#fbfcfb}.source-orders table{width:100%;border-collapse:collapse;font-size:12px}.source-orders th,.source-orders td{padding:7px 9px;border-top:1px solid #e9eeeb;text-align:left}.source-orders th{color:#687873;background:#f1f4f2}.order-chips{display:flex;flex-wrap:wrap;gap:6px}.order-chips span{padding:5px 8px;border-radius:5px;background:#eef3f0;font-size:12px}.allocation-section{margin-top:14px}.allocation-actions{gap:6px}.allocation-actions button{border:0;background:none;color:#14724a;font-weight:750;cursor:pointer}.allocation-row{display:grid;grid-template-columns:minmax(220px,1.4fr) minmax(135px,.7fr) 70px 105px 100px 48px;align-items:end;gap:9px;padding:12px;border:1px solid var(--line);border-radius:9px;margin-top:8px;background:#fcfdfc}.allocation-row label,.metric{display:grid;gap:4px}.allocation-row label>span,.metric span{font-size:10px;color:var(--muted)}select,input{width:100%;box-sizing:border-box;min-height:36px;border:1px solid #cfdad5;border-radius:7px;background:#fff;padding:6px 8px;color:var(--ink)}.metric{padding-bottom:4px}.metric strong{font-size:12px}.remove-button{height:36px;border:0;background:none;color:#a33e31;font-weight:700;cursor:pointer}.batch-summary{grid-column:1/-1;display:flex;flex-wrap:wrap;gap:6px;padding-top:5px}.batch-summary>span{padding:4px 7px;border-radius:5px;background:#edf4f0;color:#3f5c51;font-size:10px}.batch-summary>span.underfilled{background:var(--amber-soft);color:#9a580b}.batch-summary em{font-style:normal;margin-left:4px}.empty-allocation{padding:12px;border:1px dashed #d8c7a9;border-radius:8px;background:#fffaf1;color:#8a5a20;font-size:12px}.review-panel{position:sticky;top:207px;align-self:start;padding:18px}.review-title{align-items:flex-start;gap:9px}.review-title p{margin:0;color:var(--muted);font-size:11px;line-height:1.5}.review-loading,.review-error{margin-top:12px;padding:9px;border-radius:7px;background:#f4f7f5;font-size:12px}.review-error{background:#fff3ed;color:#a54022}.review-list{display:grid;gap:10px;margin-top:14px}.review-list>article{padding:11px;border:1px solid var(--line);border-radius:8px}.review-operation{justify-content:space-between;padding-bottom:7px;border-bottom:1px solid #edf1ef;font-size:13px}.review-task{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:8px;padding-top:8px}.review-task>div{display:grid;gap:2px}.review-task>div:last-child{text-align:right}.review-task strong,.review-task span{font-size:11px}.review-task small{color:var(--muted);font-size:10px}.over-ack{display:flex;align-items:flex-start;gap:8px;margin-top:12px;padding:10px;border-radius:8px;background:var(--amber-soft);font-size:11px;line-height:1.45}.over-ack input{width:auto;min-height:auto;margin-top:2px}.source-check,.review-note{margin-top:12px;padding:11px;border-radius:8px;background:#f3f6f4}.source-check p,.review-note p{margin:5px 0 0;color:var(--muted);font-size:11px;line-height:1.5}.workspace-footer{position:sticky;bottom:0;z-index:14;justify-content:space-between;gap:20px;padding:13px 28px;background:rgba(255,255,255,.98);border-top:1px solid var(--line);box-shadow:0 -7px 22px rgba(22,47,41,.08)}.workspace-footer>div:first-child{display:grid;gap:2px}.workspace-footer span{font-size:11px;color:var(--muted)}.footer-actions{display:flex;gap:9px}button.primary,button.secondary{min-height:38px;padding:0 15px;border-radius:8px;font-weight:750;cursor:pointer}button.primary{border:1px solid var(--green);background:var(--green);color:#fff}button.secondary{border:1px solid #cbd8d2;background:#fff;color:var(--ink)}button:disabled{opacity:.45;cursor:not-allowed}.workspace-state{padding:42px;text-align:center;color:var(--muted)}.error-state{color:#a54022}
@media(max-width:1120px){.workspace-body{grid-template-columns:1fr}.review-panel{position:relative;top:auto;order:-1}.allocation-row{grid-template-columns:1fr 1fr repeat(3,90px) 48px}.plan-summary{grid-template-columns:repeat(2,1fr)}.plan-summary article:nth-child(2){border:0}}
@media(max-width:760px){.workspace-header,.workspace-body,.workspace-footer{padding-left:15px;padding-right:15px}.header-main,.workspace-footer{align-items:flex-start;flex-direction:column}.plan-summary,.quantity-summary,.allocation-row{grid-template-columns:1fr}.plan-summary article{border:0;border-bottom:1px solid #dcebe3;padding-bottom:7px}.operation-tabs button{min-width:150px}.allocation-row>*{grid-column:1}.footer-actions{width:100%}.footer-actions button{flex:1}.capacity-workspace{padding-bottom:125px}}
</style>
