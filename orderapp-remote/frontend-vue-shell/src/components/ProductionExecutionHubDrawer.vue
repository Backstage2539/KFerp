<template>
  <div v-if="open" class="drawer-mask" @click.self="requestClose()">
    <aside class="work-order-detail" aria-label="工单详情">
      <header class="detail-head">
        <div>
          <button class="back-link" type="button" @click="requestClose()">← 返回工单列表</button>
          <div class="eyebrow">工单详情</div>
          <h2>{{ header.work_order_no || workOrderLabel }}</h2>
          <p>工单用于分配任务和汇总状态，执行动作在对应工位完成。</p>
        </div>
        <button class="secondary" type="button" @click="requestClose()">关闭</button>
      </header>

      <div v-if="loading" class="notice">正在加载工单状态…</div>
      <template v-else>
        <div v-if="message" class="notice success">{{ message }}</div>
        <div v-if="error" class="notice danger">{{ error }}</div>

        <section class="product-hero">
          <div class="product-icon">●</div>
          <div><strong>{{ typedOutputLabel }}</strong><p>{{ header.work_order_no || '-' }} · {{ formatTargetQuantity(header) }}</p></div>
          <span class="status-pill" :class="statusTone(header.status)">{{ statusLabel(header.status) }}</span>
        </section>

        <section class="summary-grid">
          <div><span>工序进度</span><strong>{{ completedOperations }}/{{ operations.length }}</strong><small>{{ currentOperationText }}</small></div>
          <div><span>调度负责人</span><strong>{{ header.assigned_to || '未指定' }}</strong><small>协调整张工单，不代替任务执行人</small></div>
          <div><span>入库状态</span><strong>{{ receiptStatus }}</strong><small>{{ receiptQuantityText }}</small></div>
          <div><span>当前待办</span><strong>{{ todoItems.length }} 项</strong><small>{{ firstTodoLabel }}</small></div>
        </section>

        <div class="detail-layout">
          <main class="operation-column">
            <div class="section-head"><div><span class="step-no">01</span><strong>工序进度</strong></div><span class="muted">按工序及拆分批次执行</span></div>
            <div class="operation-list">
              <article v-for="row in operations" :key="row.job_card_id" class="operation-card" :class="statusTone(row.status)">
                <div class="operation-sequence">{{ row.sequence_no || '-' }}</div>
                <div class="operation-main">
                  <div class="operation-title"><strong>{{ row.operation || '工序' }}</strong><span>{{ batchLabel(row) }}</span><em>{{ row.status_label || statusLabel(row.status) }}</em></div>
                  <p>{{ row.workstation || '未分配工位' }} · 负责人：{{ row.assigned_to || '待配置负责人' }}</p>
                  <details v-if="isOperationRecorded(row)" class="record-details">
                    <summary>工序记录</summary>
                    <div class="record-grid"><span>实际投入 <strong>{{ quantity(row.actual_input_qty) }}</strong></span><span>实际产出 <strong>{{ quantity(row.actual_output_qty) }}</strong></span><span>耗时 <strong>{{ row.actual_minutes || 0 }} 分钟</strong></span><span>损耗 <strong>{{ quantity(row.actual_loss_qty) }}</strong></span></div>
                  </details>
                  <ProductionTaskStaffEditor ref="staffEditors" v-if="!workOrderClosed && assignmentJobCardID === row.job_card_id" :job-card-id="Number(row.job_card_id)" :work-order-id="Number(header.work_order_id)" @saved="assignmentJobCardID = 0; load(); emit('updated')" @cancel="assignmentJobCardID = 0" />
                  <div class="operation-actions">
                    <span v-if="workOrderClosed" class="muted">{{ statusLabel(header.status) }}，任务只读</span>
                    <template v-else><button v-if="!['completed','cancelled'].includes(row.status)" class="secondary" type="button" @click="openAssignment(row)">{{ row.assigned_to ? '调整人员' : '分配任务' }}</button><button class="secondary" type="button" @click="enterWorkstation(row)">进入工位</button></template>
                  </div>
                </div>
              </article>
              <div v-if="!operations.length" class="empty">暂无工序任务，请先完善工序拆分。</div>
            </div>

            <div class="section-head record-heading"><div><span class="step-no">02</span><strong>工序记录</strong></div><span class="muted">原“工序卡”记录已合入这里</span></div>
            <p class="record-help">展开已执行任务，可查看人员、投入、产出、耗时和损耗。旧工序卡链接仍可定位到对应任务。</p>
            <details class="folded"><summary>冻结 BOM 与工艺路线</summary><p>{{ hub.bom_summary || '-' }}</p><p>{{ hub.route_summary || '-' }}</p></details>
            <details class="folded"><summary>配方、成本与订单追溯</summary><p>成本 {{ money(hub.cost_summary?.total_cost) }} · 关联订单 {{ header.order_nos || '-' }}</p><div v-for="item in hub.trace_timeline || []" :key="`${item.type}-${item.ref_id}-${item.at}`" class="trace-row"><strong>{{ item.title }}</strong><span>{{ item.at || '-' }} · {{ item.summary || '-' }}</span></div></details>
          </main>

          <aside class="todo-column">
            <div class="section-head"><div><span class="step-no warning">!</span><strong>当前待办</strong></div></div>
            <article v-for="todo in todoItems" :key="todo.key" class="todo-card" :class="todo.tone">
              <strong>{{ todo.label }}</strong><p>{{ todo.detail }}</p>
              <details v-if="todo.key === 'wip_shortage' && materialRows.length" class="material-details"><summary>查看缺料明细</summary><div v-for="material in materialRows" :key="material.id || material.material_id"><strong>{{ material.material_name }}</strong><span>需求 {{ materialQty(material, 'required') }} · WIP 可用 {{ materialQty(material, 'available') }} · 缺口 {{ materialQty(material, 'shortage') }}</span></div></details>
              <button type="button" @click="runTodo(todo)">{{ todo.actionLabel }}</button>
            </article>
            <article v-if="workOrderClosed" class="todo-card"><strong>{{ statusLabel(header.status) }}，工单只读</strong><p>该工单已结束，不再开放人员调整、领料或执行动作。</p></article>
            <article v-else-if="!todoItems.length" class="todo-card ready"><strong>当前无阻塞</strong><p>工位可按任务状态继续执行。</p><button type="button" @click="enterFirstWorkstation">进入工位</button></article>
            <div class="quality-summary"><span>质检</span><strong>{{ qualityLabel }}</strong><small>{{ qualityStatus.note || '按工序或批次记录检查结果' }}</small></div>
          </aside>
        </div>
      </template>
    </aside>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import ProductionTaskStaffEditor from './ProductionTaskStaffEditor.vue'
import { executionHubOutputLabel } from '../lib/production-execution-hub'

const props = defineProps({ open: { type: Boolean, default: false }, workOrderId: { type: Number, default: 0 }, focus: { type: String, default: '' }, viewParams: { type: Object, default: () => ({}) } })
const emit = defineEmits(['close', 'updated'])
const loading = ref(false)
const error = ref('')
const message = ref('')
const detail = ref({})
const assignmentJobCardID = ref(0)

const hub = computed(() => detail.value.execution_hub || {})
const header = computed(() => hub.value.header || detail.value.work_order || {})
const operations = computed(() => (hub.value.operation_progress || []).map((row, index, rows) => ({ ...row, batch_index: rows.filter((item, itemIndex) => item.sequence_no === row.sequence_no && itemIndex <= index).length, batch_count: rows.filter((item) => item.sequence_no === row.sequence_no).length })))
const qualityStatus = computed(() => hub.value.quality_status || {})
const workOrderClosed = computed(() => ['completed', 'cancelled'].includes(String(header.value.status || '').toLowerCase()))
const typedOutputLabel = computed(() => executionHubOutputLabel({ ...hub.value, header: header.value }))
const completedOperations = computed(() => operations.value.filter((row) => row.status === 'completed').length)
const currentOperationText = computed(() => operations.value.find((row) => !['completed', 'cancelled'].includes(row.status))?.operation || '工序已结束')
const finishedReceipts = computed(() => hub.value.finished_receipts || [])
const materialRows = computed(() => hub.value.wip_status?.materials || [])
const receiptStatus = computed(() => header.value.status === 'completed' ? '已入库' : finishedReceipts.value.length ? '部分入库' : operations.value.length && completedOperations.value === operations.value.length ? '待入库' : '等待报工')
const receiptQuantityText = computed(() => finishedReceipts.value.length ? `${finishedReceipts.value.length} 笔入库记录` : `目标 ${formatTargetQuantity(header.value)}`)
const qualityLabel = computed(() => ({ unchecked: '未检查', pass: '通过', hold: '待处理', reject: '不合格', blocked: '已冻结' }[qualityStatus.value.status] || qualityStatus.value.result || '未检查'))
const todoItems = computed(() => {
  const rows = []
  if (workOrderClosed.value) return rows
  const unassigned = operations.value.filter((row) => !row.assigned_to)
  if (unassigned.length) rows.push({ key: 'assignment', label: `待分配 ${unassigned.length} 项任务`, detail: '为具体工序批次选择执行人，不改变调度负责人。', actionLabel: '分配任务', tone: 'warning', row: unassigned[0] })
  for (const reason of hub.value.readiness?.blocking_reasons || []) {
    if (reason.code === 'workstation_unassigned') continue
    rows.push({ key: reason.code, label: reason.label, detail: `待协同岗位：${reason.next_handler || '现场主管'}`, actionLabel: todoActionLabel(reason.code), tone: reason.severity === 'blocked' ? 'warning' : 'neutral', reason })
  }
  if (receiptStatus.value === '待入库' || receiptStatus.value === '部分入库') rows.push({ key: 'receipt', label: receiptStatus.value, detail: '核对已报工产出、质检状态和本次入库数量。', actionLabel: '去完工入库', tone: 'ready' })
  return rows
})
const firstTodoLabel = computed(() => todoItems.value[0]?.label || '可进入工位继续执行')
const workOrderLabel = computed(() => props.workOrderId ? `工单 #${props.workOrderId}` : '工单')

function statusLabel(value) { return ({ released: '待执行', running: '生产中', partially_completed: '部分完成', completed: '已完成', cancelled: '已取消', pending: '待处理', ready: '可开始', paused: '已暂停' }[String(value || '')] || String(value || '待处理')) }
function statusTone(value) { if (value === 'completed') return 'ready'; if (value === 'running') return 'running'; if (value === 'cancelled') return 'danger'; return 'waiting' }
function batchLabel(row) { return `第 ${row.batch_index || 1} 批 / 共 ${row.batch_count || 1} 批` }
function quantity(value) { return Number(value || 0).toLocaleString('zh-CN', { maximumFractionDigits: 3 }) }
function money(value) { return `¥${Number(value || 0).toFixed(2)}` }
function formatTargetQuantity(row) { const value = Number(row.output_qty || row.planned_units || row.planned_output_g || row.planned_g || 0); return `${quantity(value)} ${row.output_unit || (row.planned_units ? '件' : 'g')}` }
function isOperationRecorded(row) { return Boolean(row.started_at || row.completed_at || row.actual_minutes || row.actual_input_qty || row.actual_output_qty) }
function materialQty(row, kind) { const g = Number(row?.[`${kind}_g`] || 0); if (g > 0) return `${quantity(g)}g`; const units = Number(row?.[`${kind}_units`] || 0); return `${quantity(units)}${row?.unit || '件'}` }
function todoActionLabel(code) { return ({ wip_shortage: '去领料', quality_freeze: '查看质检', prior_operation_incomplete: '进入前序工位', job_cards_incomplete: '进入工位', schedule_risk: '调整排程' }[code] || '查看处理') }
const staffEditors = ref([])
function staffCanLeave() { return (Array.isArray(staffEditors.value) ? staffEditors.value : [staffEditors.value]).filter(Boolean).every(editor => editor.canLeave()) }
function requestClose() { if (staffCanLeave()) emit('close') }
function navigate(key, params = {}) { if (!staffCanLeave()) return; window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key, params: { work_order_id: header.value.work_order_id, ...params }, returnNavigation: { key: 'workOrders', label: '返回工单详情', params: { work_order_id: header.value.work_order_id } } } })); emit('close') }
function enterWorkstation(row) { navigate('workstationView', { job_card_id: row.job_card_id, focus: 'workstation_task' }) }
function enterFirstWorkstation() { const row = operations.value.find((item) => !['completed', 'cancelled'].includes(item.status)) || operations.value[0]; if (row) enterWorkstation(row) }
function openAssignment(row) { if (!row || !staffCanLeave()) return; assignmentJobCardID.value = row.job_card_id;  }

function runTodo(todo) {
  if (todo.key === 'assignment') { openAssignment(todo.row); return }
  if (todo.key === 'receipt') { navigate('productionAcceptance'); return }
  const link = todo.reason?.related_links?.[0]
  if (link?.view) { navigate(link.view, link.params || {}); return }
  enterFirstWorkstation()
}
async function load() {
  if (!props.workOrderId) return
  loading.value = true; error.value = ''
  try {
    detail.value = await apiGet(`/api/produce/work-orders/${props.workOrderId}`)
    const requested = Number(props.viewParams?.job_card_id || 0); if (props.focus === 'assignment') openAssignment(operations.value.find((row) => row.job_card_id === requested) || operations.value.find((row) => !row.assigned_to) || operations.value[0])
  } catch (err) { error.value = err.message || '加载工单详情失败' } finally { loading.value = false }
}
watch(() => [props.open, props.workOrderId], ([isOpen]) => { if (isOpen) load() }, { immediate: true })
</script>

<style scoped>
/* Previous compact declarations are retained below for source-history compatibility.
*{box-sizing:border-box}.drawer-mask{position:fixed;inset:0;z-index:60;background:rgba(28,34,31,.26);display:flex;justify-content:flex-end}.work-order-detail{width:min(1120px,96vw);height:100%;overflow:auto;background:#f6f8f7;color:#18342b;padding:22px}.detail-head{display:flex;justify-content:space-between;gap:18px;align-items:flex-start;margin-bottom:16px}.back-link{border:0;background:transparent;color:#24704a;padding:0;margin-bottom:14px}.eyebrow{font-size:12px;color:#6b7b75;margin-bottom:4px}.detail-head h2{margin:0;font-size:28px}.detail-head p{margin:6px 0 0;color:#68766f}.secondary,.primary,.todo-card button{min-height:36px;border-radius:8px;padding:7px 12px;font:inherit;cursor:pointer}.secondary{border:1px solid #c8d0cc;background:#fff;color:#27463b}.primary,.todo-card button{border:1px solid #2f8f5b;background:#2f8f5b;color:#fff}.notice{padding:11px 13px;border:1px solid #e0c58f;border-radius:9px;background:#fff8e8;margin-bottom:12px}.notice.success{border-color:#acd4ba;background:#eff9f2}.notice.danger{border-color:#efb1aa;background:#fff1ef;color:#9e3328}.product-hero{display:flex;align-items:center;gap:14px;padding:18px;border:1px solid #dce9e2;border-radius:12px;background:linear-gradient(90deg,#eaf7ef,#f7fbf9);margin-bottom:14px}.product-icon{width:42px;height:42px;border-radius:50%;display:grid;place-items:center;background:#784d2c;color:#b17c52}.product-hero strong{font-size:20px}.product-hero p{margin:5px 0 0;color:#637169}.status-pill{margin-left:auto;border:1px solid #d7ddd9;border-radius:999px;padding:5px 10px;background:#fff}.status-pill.running{color:#24704a;border-color:#acd4ba}.status-pill.danger{color:#9e3328;border-color:#efb1aa}.summary-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px;margin-bottom:16px}.summary-grid>div{display:grid;gap:5px;border:1px solid #e0e5e2;border-radius:10px;background:#fff;padding:12px}.summary-grid span,.summary-grid small,.muted{font-size:12px;color:#738079}.summary-grid strong{font-size:17px}.detail-layout{display:grid;grid-template-columns:minmax(0,1fr) 300px;gap:18px}.operation-column,.todo-column{border:1px solid #e0e5e2;border-radius:12px;background:#fff;padding:16px}.section-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:12px}.section-head>div{display:flex;align-items:center;gap:9px}.step-no{width:30px;height:30px;display:grid;place-items:center;border-radius:50%;background:#2f8f5b;color:#fff;font-size:12px}.step-no.warning{background:#d77b12}.operation-list{display:grid;gap:10px}.operation-card{display:grid;grid-template-columns:38px 1fr;gap:12px;padding:13px;border:1px solid #e2e7e4;border-radius:10px}.operation-card.running{border-color:#83bd99;box-shadow:inset 3px 0 #2f8f5b}.operation-card.waiting{border-color:#efd4a5}.operation-sequence{width:34px;height:34px;border-radius:50%;background:#edf6f0;color:#24704a;display:grid;place-items:center;font-weight:700}.operation-title{display:flex;gap:9px;align-items:center;flex-wrap:wrap}.operation-title span{font-size:12px;color:#66736d}.operation-title em{font-size:12px;font-style:normal;border-radius:999px;padding:3px 8px;background:#f5f6f5}.operation-main p{margin:5px 0;color:#64716b}.operation-actions{display:flex;gap:8px;margin-top:10px}.assignment-panel{display:flex;gap:8px;align-items:end;padding:10px;background:#fff8e8;border-radius:8px;margin-top:10px}.assignment-panel label{display:grid;gap;gap:4px;min-width:220px}.assignment-panel span{font-size:12px;color:#6d7772}.assignment-panel select{min-height:36px;border:1px solid #cfd6d2;border-radius:7px;background:#fff;padding:6px 8px}.record-details,.folded{margin-top:9px;border-top:1px solid #edf0ee;padding-top:8px}.record-details summary,.folded summary{cursor:pointer;color:#24704a;font-weight:600}.record-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:8px;margin-top:8px;font-size:12px}.record-heading{margin-top:18px}.record-help{color:#6f7b75;margin-top:-5px}.folded{border:1px solid #e4e8e6;border-radius:9px;padding:11px;margin-top:10px}.folded p{color:#64716b}.trace-row{display:grid;gap:3px;border-top:1px solid #eee;padding:7px 0;font-size:12px}.todo-column{align-self:start;display:grid;gap:10px}.todo-card{border:1px solid #edcf9f;border-radius:10px;padding:12px;background:#fff9ed}.todo-card p{margin:5px 0 10px;color:#775c35;font-size:13px}.todo-card button{background:#fff;border-color:#d89a44;color:#9b5c08}.todo-card.ready{border-color:#acd4ba;background:#eff9f2}.todo-card.ready button{border-color:#2f8f5b;color:#24704a}.quality-summary{display:grid;gap:5px;border-top:1px solid #e5e9e7;padding-top:12px}.quality-summary span,.quality-summary small{font-size:12px;color:#738079}.empty{padding:18px;text-align:center;color:#738079}
.material-details{margin:8px 0}.material-details summary{cursor:pointer;color:#9b5c08;font-size:12px}.material-details div{display:grid;gap:2px;border-top:1px solid #efdcb9;padding:7px 0}.material-details div span{font-size:11px;color:#775c35}
@media(max-width:900px){.work-order-detail{width:100vw;padding:14px}.summary-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.detail-layout{grid-template-columns:1fr}.todo-column{order:-1}.record-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:560px){.detail-head{display:grid}.summary-grid{grid-template-columns:1fr}.product-hero{align-items:flex-start;flex-wrap:wrap}.status-pill{margin-left:0}.operation-card{grid-template-columns:1fr}.assignment-panel{display:grid}.assignment-panel label{min-width:0}.operation-actions{flex-wrap:wrap}}
.assignment-panel label{gap:4px}
*/
* { box-sizing: border-box; }
.drawer-mask { position: fixed; inset: 0; z-index: 60; display: flex; justify-content: flex-end; background: rgba(28, 34, 31, .26); }
.work-order-detail { width: min(1120px, 96vw); height: 100%; overflow: auto; padding: 22px; color: #18342b; background: #f6f8f7; }
.detail-head, .section-head, .operation-title, .operation-actions, .assignment-panel { display: flex; }
.detail-head { align-items: flex-start; justify-content: space-between; gap: 18px; margin-bottom: 16px; }
.detail-head h2 { margin: 0; font-size: 28px; }
.detail-head p { margin: 6px 0 0; color: #68766f; }
.back-link { padding: 0; margin-bottom: 14px; border: 0; color: #24704a; background: transparent; }
.eyebrow, .muted, .summary-grid span, .summary-grid small, .quality-summary span, .quality-summary small { color: #738079; font-size: 12px; }
.primary, .secondary, .todo-card button { min-height: 36px; padding: 7px 12px; border-radius: 8px; font: inherit; cursor: pointer; }
.primary { border: 1px solid #2f8f5b; color: #fff; background: #2f8f5b; }
.secondary { border: 1px solid #c8d0cc; color: #27463b; background: #fff; }
.notice { padding: 11px 13px; margin-bottom: 12px; border: 1px solid #e0c58f; border-radius: 9px; background: #fff8e8; }
.notice.success { border-color: #acd4ba; background: #eff9f2; }
.notice.danger { border-color: #efb1aa; color: #9e3328; background: #fff1ef; }
.product-hero { display: flex; align-items: center; gap: 14px; padding: 18px; margin-bottom: 14px; border: 1px solid #dce9e2; border-radius: 12px; background: linear-gradient(90deg, #eaf7ef, #f7fbf9); }
.product-icon { display: grid; place-items: center; width: 42px; height: 42px; border-radius: 50%; color: #b17c52; background: #784d2c; }
.product-hero strong { font-size: 20px; }
.product-hero p { margin: 5px 0 0; color: #637169; }
.status-pill { padding: 5px 10px; margin-left: auto; border: 1px solid #d7ddd9; border-radius: 999px; background: #fff; }
.status-pill.running { border-color: #acd4ba; color: #24704a; }
.status-pill.danger { border-color: #efb1aa; color: #9e3328; }
.summary-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; margin-bottom: 16px; }
.summary-grid > div { display: grid; gap: 5px; padding: 12px; border: 1px solid #e0e5e2; border-radius: 10px; background: #fff; }
.summary-grid strong { font-size: 17px; }
.detail-layout { display: grid; grid-template-columns: minmax(0, 1fr) 300px; gap: 18px; }
.operation-column, .todo-column { padding: 16px; border: 1px solid #e0e5e2; border-radius: 12px; background: #fff; }
.section-head { align-items: center; justify-content: space-between; margin-bottom: 12px; }
.section-head > div { display: flex; align-items: center; gap: 9px; }
.step-no { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 50%; color: #fff; background: #2f8f5b; font-size: 12px; }
.step-no.warning { background: #d77b12; }
.operation-list, .todo-column { display: grid; gap: 10px; }
.operation-card { display: grid; grid-template-columns: 38px 1fr; gap: 12px; padding: 13px; border: 1px solid #e2e7e4; border-radius: 10px; }
.operation-card.running { border-color: #83bd99; box-shadow: inset 3px 0 #2f8f5b; }
.operation-card.waiting { border-color: #efd4a5; }
.operation-sequence { display: grid; place-items: center; width: 34px; height: 34px; border-radius: 50%; color: #24704a; background: #edf6f0; font-weight: 700; }
.operation-title { flex-wrap: wrap; align-items: center; gap: 9px; }
.operation-title span, .operation-title em { font-size: 12px; }
.operation-title span { color: #66736d; }
.operation-title em { padding: 3px 8px; border-radius: 999px; background: #f5f6f5; font-style: normal; }
.operation-main p { margin: 5px 0; color: #64716b; }
.operation-actions { flex-wrap: wrap; gap: 8px; margin-top: 10px; }
.assignment-panel { align-items: end; gap: 8px; padding: 10px; margin-top: 10px; border-radius: 8px; background: #fff8e8; }
.assignment-panel label { display: grid; gap: 4px; min-width: 220px; }
.assignment-panel span { color: #6d7772; font-size: 12px; }
.assignment-panel select { min-height: 36px; padding: 6px 8px; border: 1px solid #cfd6d2; border-radius: 7px; background: #fff; }
.record-details { padding-top: 8px; margin-top: 9px; border-top: 1px solid #edf0ee; }
.record-details summary, .folded summary { color: #24704a; font-weight: 600; cursor: pointer; }
.record-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; margin-top: 8px; font-size: 12px; }
.record-heading { margin-top: 18px; }
.record-help { margin-top: -5px; color: #6f7b75; }
.folded { padding: 11px; margin-top: 10px; border: 1px solid #e4e8e6; border-radius: 9px; }
.folded p { color: #64716b; }
.trace-row { display: grid; gap: 3px; padding: 7px 0; border-top: 1px solid #eee; font-size: 12px; }
.todo-column { align-self: start; }
.todo-card { padding: 12px; border: 1px solid #edcf9f; border-radius: 10px; background: #fff9ed; }
.todo-card p { margin: 5px 0 10px; color: #775c35; font-size: 13px; }
.todo-card button { border-color: #d89a44; color: #9b5c08; background: #fff; }
.todo-card.ready { border-color: #acd4ba; background: #eff9f2; }
.todo-card.ready button { border-color: #2f8f5b; color: #24704a; }
.material-details { margin: 8px 0; }
.material-details summary { color: #9b5c08; font-size: 12px; cursor: pointer; }
.material-details div { display: grid; gap: 2px; padding: 7px 0; border-top: 1px solid #efdcb9; }
.material-details div span { color: #775c35; font-size: 11px; }
.quality-summary { display: grid; gap: 5px; padding-top: 12px; border-top: 1px solid #e5e9e7; }
.empty { padding: 18px; color: #738079; text-align: center; }
@media (max-width: 900px) {
  .work-order-detail { width: 100vw; padding: 14px; }
  .summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .detail-layout { grid-template-columns: 1fr; }
  .todo-column { order: -1; }
  .record-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 560px) {
  .detail-head, .assignment-panel { display: grid; }
  .summary-grid { grid-template-columns: 1fr; }
  .product-hero { flex-wrap: wrap; align-items: flex-start; }
  .status-pill { margin-left: 0; }
  .operation-card { grid-template-columns: 1fr; }
  .assignment-panel label { min-width: 0; }
}
</style>
