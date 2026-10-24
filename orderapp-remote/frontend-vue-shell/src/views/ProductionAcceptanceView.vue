<template>
  <div class="page finished-receipts-page">
    <ProductionTopNav v-if="!props.embedded" active-key="productionAcceptance" />

    <section class="panel page-head">
      <div><h2>完工入库</h2><p>相同商品、规格和目标仓可在一次操作中合并入库；单据仍按工单保留库存追溯。</p></div>
      <div class="row-actions"><button class="secondary" type="button" @click="openManual">操作帮助</button><button class="secondary" type="button" :disabled="loading || saving" @click="load">刷新</button></div>
    </section>

    <section class="summary-grid">
      <button v-for="item in summary" :key="item.key" type="button" :class="['summary-card', { active: status === item.key }]" @click="selectStatus(item.key)">
        <span>{{ item.label }}</span><strong>{{ item.count }}</strong>
      </button>
    </section>

    <div v-if="message" class="notice success">{{ message }}</div>
    <div v-if="error" class="notice danger">{{ error }}</div>
    <div v-if="warehouseWarning" class="notice warning">{{ warehouseWarning }}</div>
    <div v-if="attempt?.items.some(item => !item.succeeded)" class="notice warning">
      有入库请求尚未确认。重试会沿用原请求，不重复入库。
      <button class="secondary" :disabled="saving" @click="postAttempt">重试原请求</button>
      <button class="secondary" :disabled="saving" @click="resetAttempt">刷新后重新核对</button>
    </div>

    <section class="panel receipt-list">
      <div class="list-head"><strong>入库任务</strong><span>{{ filteredGroups.length }} 组 · {{ filteredRows.length }} 张工单</span></div>
      <article v-for="group in filteredGroups" :key="group.key" class="receipt-row">
        <div class="product">
          <strong>{{ outputTitle(group) }}</strong>
          <span>每袋规格：{{ receiptSpecLabel(group) }}</span>
          <span>{{ group.rows.length > 1 ? `${group.rows.length} 张来源工单` : (group.rows[0]?.work_order_no || '工单') }} · 目标仓 {{ group.warehouse_name }}</span>
        </div>
        <div><span>已报工</span><strong>{{ qty(group.reported_qty) }} {{ group.output_unit || '' }}</strong></div>
        <div><span>已入库</span><strong>{{ qty(group.received_qty) }} {{ group.output_unit || '' }}</strong></div>
        <div><span>本次可入库</span><strong>{{ qty(group.processable_qty) }} {{ group.output_unit || '' }}</strong></div>
        <div><span>质量</span><strong :class="{ blocked: group.blocked_count > 0 }">{{ qualityLabel(group) }}</strong></div>
        <div class="row-actions">
          <button v-if="group.processable_qty > 0" class="primary" type="button" :disabled="saving" @click="openReceipt(group)">{{ group.rows.length > 1 ? '合并入库' : '办理入库' }}</button>
          <button class="secondary" type="button" @click="toggleSources(group)">{{ expandedGroupKey === group.key ? '收起来源' : '查看工单' }}</button>
        </div>

        <div v-if="expandedGroupKey === group.key || group.blocked_count > 0" class="source-list">
          <div v-for="row in group.rows" :key="row.work_order_id" class="source-row">
            <strong>{{ row.work_order_no }}</strong>
            <span>已报 {{ qty(row.reported_qty) }} · 已入 {{ qty(row.received_qty) }} · 可入 {{ qty(row.available_qty) }} {{ row.output_unit || '' }}</span>
            <span v-if="row.blocked_reason || row.quality_blocked" class="blocked">{{ row.blocked_reason || '质检阻塞，本次不自动过账' }}</span>
            <button v-if="row.correction_cards?.length && !row.correction_blocked_reason" class="secondary compact" :disabled="saving" @click="openCorrection(row)">更正报工</button>
            <span v-else-if="row.blocked_reason && row.correction_blocked_reason">{{ row.correction_blocked_reason }}</span>
            <button class="secondary compact" type="button" @click="openWorkOrder(row)">打开工单</button>
          </div>
        </div>

        <div v-if="selectedGroupKey === group.key" class="receipt-form">
          <div class="form-title"><div><strong>{{ group.rows.length > 1 ? '核对合并入库' : '核对本次入库' }}</strong><p>数量按工单顺序分配；提交后仍分别生成工单关联单据。</p></div><button class="secondary" type="button" :disabled="saving" @click="selectedGroupKey = ''">关闭</button></div>
          <div class="form-grid">
            <label v-if="group.output_type === 'material'"><span>入库方式</span><select v-model="form.completion_mode"><option value="partial">本次部分入库</option><option value="final">最后一次入库</option></select></label>
            <label><span>{{ group.output_unit === '袋' ? '本次入库袋数' : `本次入库数量（${group.output_unit || '-'}）` }}</span><input v-model.number="form.quantity" type="number" min="0" :max="group.processable_qty" :step="quantityStep(group)" /></label>
            <label><span>每袋规格</span><input :value="receiptSpecLabel(group)" disabled /></label>
            <label><span>目标仓库</span><input :value="group.warehouse_name" disabled /></label>
            <label v-if="group.rows.length === 1" class="wide"><span>本次实际投入（g，可选）</span><input v-model.number="form.consumed_input_g" type="number" min="0" step="1" /></label>
          </div>
          <div class="source-list">
            <div v-for="item in finishedReceiptBatchPlan(group, form.quantity)" :key="item.work_order_id" class="source-row"><strong>{{ item.work_order_no }}</strong><span>本次分配 {{ qty(item.quantity) }} {{ group.output_unit }}</span></div>
          </div>
          <div class="source-note"><span>备注（自动记录来源工单）</span><strong>{{ plannedSourceWorkOrders(group).join('、') || '请选择可入库工单' }}</strong></div>
          <div class="impact">数量异常或质检阻塞项不会过账；已成功部分和失败工单会分别列出，刷新后可继续处理。增量入库不会重复扣料或重复计成本。</div>
          <button class="primary submit" type="button" :disabled="saving || !Number(form.quantity || 0)" @click="submitReceipt(group)">{{ saving ? '正在过账…' : (group.rows.length > 1 ? '确认合并入库' : '确认入库') }}</button>
        </div>
      </article>
      <div v-if="correction.row" class="receipt-form">
        <div class="form-title"><strong>更正报工 · {{ correction.row.work_order_no }}</strong><button class="secondary" :disabled="saving" @click="correction.row = null">关闭</button></div>
        <p>{{ outputTitle(correction.row) }} · 每袋规格 {{ receiptSpecLabel(correction.row) }} · {{ correction.row.target_warehouse_name }}</p>
        <div class="form-grid">
          <label><span>来源末工序</span><select v-model.number="correction.cardID" :disabled="!!correction.pending"><option v-for="card in correction.row.correction_cards" :key="card.id" :value="card.id">{{ card.operation }} #{{ card.id }} · 原报 {{ card.actual_output_qty }} {{ correction.row.output_unit }}</option></select></label>
          <label><span>核实后的累计报工数量（{{ correction.row.output_unit }}）</span><input v-model.number="correction.quantity" :disabled="!!correction.pending" type="number" min="0" :step="quantityStep(correction.row)" placeholder="请按现场核实填写" /></label>
          <label><span>更正原因</span><input v-model="correction.reason" :disabled="!!correction.pending" placeholder="说明误报及核实情况" /></label>
        </div>
        <p>仅更正所选工序的累计产出，保存更正记录；保存后再办理入库。</p>
        <button class="primary" :disabled="saving" @click="submitCorrection">{{ correction.pending ? '重试原更正请求' : '保存更正' }}</button>
      </div>
      <div v-if="!filteredGroups.length" class="empty">当前没有{{ statusLabel(status) }}任务</div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import ProductionTopNav from '../components/ProductionTopNav.vue'
import { completionNoteForWorkOrder, createReceiptAttempt, runReceiptAttempt, receiptCompletionMode, receiptSpecLabel, finishedReceiptBatchPlan, groupFinishedReceiptTasks } from '../lib/production-acceptance.js'

const props = defineProps({ embedded: { type: Boolean, default: false }, viewParams: { type: Object, default: () => ({}) } })
const rows = ref([])
const warehouses = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const message = ref('')
const warehouseWarning = ref('')
const status = ref('pending')
const selectedGroupKey = ref('')
const expandedGroupKey = ref('')
const attempt = ref(null)
const correction = reactive({ row: null, cardID: 0, quantity: '', reason: '', pending: null })
const form = reactive({ completion_mode: 'final', quantity: 0, consumed_input_g: 0 })

const summary = computed(() => [
  { key: 'pending', label: '待入库（含部分）', count: rows.value.filter((row) => ['pending', 'partial'].includes(row.receipt_status)).length },
  { key: 'partial', label: '部分入库', count: rows.value.filter((row) => row.receipt_status === 'partial').length },
  { key: 'received', label: '已入库', count: rows.value.filter((row) => row.receipt_status === 'received').length },
])
const filteredRows = computed(() => rows.value.filter((row) => (status.value === 'pending' ? ['pending', 'partial'].includes(row.receipt_status) : row.receipt_status === status.value)))
const filteredGroups = computed(() => groupFinishedReceiptTasks(filteredRows.value, warehouses.value))

function openManual() { window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'productionManual', returnNavigation: { key: 'productionAcceptance', label: '返回完工入库' } } })) }
function qty(value) { return Number(value || 0).toLocaleString('zh-CN', { maximumFractionDigits: 3 }) }
function outputTitle(row) { return row.output_name || '未命名产出' }
function statusLabel(value) { return ({ pending: '待入库', partial: '部分入库', received: '已入库' }[value] || '') }
function selectStatus(value) { status.value = value; selectedGroupKey.value = ''; error.value = ''; message.value = '' }
function qualityLabel(group) {
  if (group.blocked_count && group.processable_qty > 0) return `${group.blocked_count} 张工单待处理，其余可入库`
  if (group.blocked_count) return '需要处理'
  return ({ pass: '已放行', unchecked: '未检查', ready: '可入库' }[group.quality_status] || '可入库')
}
function quantityStep(group) { return ['kg', '千克', '公斤'].includes(String(group.output_unit || '').trim().toLowerCase()) ? '0.001' : (['g', '克'].includes(String(group.output_unit || '').trim().toLowerCase()) ? '1' : '1') }
function plannedSourceWorkOrders(group) { return finishedReceiptBatchPlan(group, Number(form.quantity || 0)).map((item) => item.work_order_no) }
function toggleSources(group) { expandedGroupKey.value = expandedGroupKey.value === group.key ? '' : group.key }
function openReceipt(group) {
  if (attempt.value?.items.some(item => !item.succeeded)) { error.value = '请先重试原请求，或刷新后重新核对已有入库结果'; return }
  selectedGroupKey.value = group.key
  form.completion_mode = group.output_type === 'material' && group.rows[0]?.receipt_status === 'partial' ? 'partial' : 'final'
  form.quantity = Number(group.processable_qty || 0)
  form.consumed_input_g = 0
}
function openWorkOrder(row) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: {
    key: 'workOrders',
    params: { work_order_id: row.work_order_id, focus: 'receipt' },
    returnNavigation: { key: 'productionAcceptance', label: '返回完工入库', params: { ...props.viewParams, work_order_id: row.work_order_id } },
  } }))
}
function requestID(row) { return globalThis.crypto?.randomUUID?.() || `receipt-${row.work_order_id}-${Date.now()}` }
function receiptPayload(row, quantity, note, includeConsumedInput) {
  const unit = String(row.output_unit || '').trim().toLowerCase()
  const weightFactor = ['kg', '千克', '公斤'].includes(unit) ? 1000 : (['g', '克'].includes(unit) ? 1 : 0)
  const totalG = weightFactor ? Math.round(quantity * weightFactor) : 0
  const totalUnits = weightFactor ? 0 : Math.round(quantity)
  return {
    completion_mode: row.output_type === 'material' ? form.completion_mode : receiptCompletionMode(row, quantity),
    expected_receipt_version: row.receipt_version,
    request_id: requestID(row),
    finished_qty_g: totalG,
    finished_loose_g: totalG,
    finished_qty_units: totalUnits,
    finished_units: totalUnits,
    consumed_input_g: includeConsumedInput ? Math.round(Number(form.consumed_input_g || 0)) : 0,
    note,
  }
}

async function submitReceipt(group) {
  const quantity = Number(form.quantity || 0)
  if (quantity <= 0) { error.value = '本次入库数量必须大于 0'; return }
  if (quantity > Number(group.processable_qty || 0)) { error.value = `本次数量不能超过当前可入库 ${qty(group.processable_qty)} ${group.output_unit || ''}`; return }
  if (!['kg', '千克', '公斤', 'g', '克'].includes(String(group.output_unit || '').trim().toLowerCase()) && !Number.isInteger(quantity)) {
    error.value = '按件/袋/盒入库时数量必须为整数'
    return
  }
  const plan = finishedReceiptBatchPlan(group, quantity)
  if (!plan.length) { error.value = '没有可入库的工单'; return }

  attempt.value = createReceiptAttempt(group, quantity, (row, item) => receiptPayload(row, item.quantity, completionNoteForWorkOrder(row), group.rows.length === 1))
  await postAttempt()
}
async function postAttempt() {
  if (!attempt.value || saving.value) return
  saving.value = true; error.value = ''; message.value = ''
  try {
    await runReceiptAttempt(attempt.value, (row, body) => apiSend(`/api/produce/work-orders/${row.work_order_id}/complete`, { body }))
    const succeeded = attempt.value.items.filter(item => item.succeeded)
    const failed = attempt.value.items.filter(item => !item.succeeded)
    const entries = [...new Set(succeeded.flatMap(item => (item.result?.stock_entries || []).map(entry => entry.entry_no)))]
    message.value = succeeded.length ? `本次入库成功 ${qty(succeeded.reduce((sum, item) => sum + item.quantity, 0))} ${attempt.value.unit}，处理 ${succeeded.length} 张工单${entries.length ? `；关联单据 ${entries.join('、')}` : ''}` : ''
    selectedGroupKey.value = ''
    await load()
    error.value = [error.value, ...failed.map(item => `${item.work_order_no}（${item.reason}）`)].filter(Boolean).join('；')
  } finally { saving.value = false }
}
async function resetAttempt() {
  await load()
  if (!error.value) { attempt.value = null; message.value = '已刷新，请核对各工单累计已入库量后填写新一次入库数量' }
}
function openCorrection(row) {
  if (correction.pending) { error.value = '请先重试尚未确认的更正请求'; return }
  correction.row = row; correction.cardID = row.correction_cards.length === 1 ? row.correction_cards[0].id : 0
  correction.quantity = ''; correction.reason = ''; error.value = ''
}
async function submitCorrection() {
  if (saving.value || !correction.row) return
  const row = correction.row
  if (!correction.pending) {
    const quantity = Number(correction.quantity)
    if (!correction.cardID || correction.quantity === '' || !Number.isFinite(quantity) || quantity < 0 || !correction.reason.trim() || (quantityStep(row) === '1' && !Number.isInteger(quantity))) {error.value = '请选择来源工序，填写核实后的数量和更正原因'; return}
    correction.pending = { id: correction.cardID, body: { actual_output_qty: quantity, reason: correction.reason.trim(), expected_version: row.receipt_version, request_id: requestID(row) } }
  }
  saving.value = true; error.value = ''
  try {
    await apiSend(`/api/produce/job-cards/${correction.pending.id}/correct-output`, { body: correction.pending.body })
    correction.pending = null; correction.row = null
    await load(); message.value = '报工已更正并记录操作日志，请核对本次入库袋数后办理入库'
  } catch (err) {
    error.value = err.message || '更正结果未确认，请重试原请求'
    if (err.status >= 400 && err.status < 500) correction.pending = null
  } finally { saving.value = false }
}

async function loadWarehouseNames(items) {
  const customerIDs = [...new Set(items.map((row) => Number(row.customer_id || 0)))]
  const results = await Promise.allSettled(customerIDs.map((customerID) => apiGet(customerID > 0 ? `/api/stock/warehouses?customer_id=${customerID}` : '/api/stock/warehouses')))
  warehouses.value = results.flatMap((result) => result.status === 'fulfilled' ? (result.value.rows || []) : [])
  warehouseWarning.value = results.some((result) => result.status === 'rejected') ? '部分目标仓名称暂未加载，请刷新后核对仓库资料。' : ''
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet('/api/produce/finished-receipts?status=all')
    rows.value = data.rows || []
    if (rows.value.some(row => !row.target_warehouse_name)) await loadWarehouseNames(rows.value)
    const requested = Number(props.viewParams?.work_order_id || 0)
    if (requested) {
      const row = rows.value.find((item) => Number(item.work_order_id) === requested)
      if (row) {
        status.value = row.receipt_status === 'received' ? 'received' : 'pending'
        const group = filteredGroups.value.find(group => group.rows.some(source => Number(source.work_order_id) === requested))
        if (row.receipt_status !== 'received' && group) openReceipt(group)
      }
    }
  } catch (err) { error.value = err.message || '加载完工入库任务失败' } finally { loading.value = false }
}
onMounted(load)
</script>

<style scoped>
*{box-sizing:border-box}.page{padding:16px;display:grid;gap:14px;background:#f7f8fa;color:#19342b}.panel{border:1px solid #e0e5e2;border-radius:11px;background:#fff;padding:15px}.page-head,.list-head,.form-title{display:flex;justify-content:space-between;align-items:flex-start;gap:12px}.page-head h2{margin:0;font-size:22px}.page-head p,.form-title p{margin:5px 0 0;color:#6b7771;font-size:13px}.primary,.secondary{min-height:36px;border-radius:8px;padding:7px 12px;font:inherit;cursor:pointer}.primary{border:1px solid #2f8f5b;background:#2f8f5b;color:#fff}.secondary{border:1px solid #bec9c3;background:#fff;color:#28483c}button:disabled{opacity:.5;cursor:not-allowed}.summary-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px}.summary-card{display:grid;gap:5px;text-align:left;border:1px solid #e0e5e2;border-radius:10px;background:#fff;padding:13px;color:#617069}.summary-card strong{font-size:22px;color:#19342b}.summary-card.active{border-color:#2f8f5b;background:#eff9f2}.notice{padding:11px 13px;border-radius:9px}.notice.success{border:1px solid #acd4ba;background:#eff9f2;color:#24623d}.notice.danger{border:1px solid #efb1aa;background:#fff1ef;color:#9e3328}.notice.warning{border:1px solid #f0d69d;background:#fff8e8;color:#8a570c}.receipt-list{display:grid;gap:8px}.list-head{padding-bottom:8px;border-bottom:1px solid #edf0ee}.list-head span{color:#6b7771}.receipt-row{display:grid;grid-template-columns:minmax(210px,1.4fr) repeat(4,minmax(100px,.65fr)) minmax(160px,.8fr);gap:12px;align-items:center;border:1px solid #e3e8e5;border-radius:10px;padding:12px}.receipt-row>div{display:grid;gap:4px}.receipt-row span{font-size:12px;color:#718079}.product span{font-size:12px}.blocked{color:#b7640d}.row-actions{display:flex!important;gap:7px;flex-wrap:wrap}.compact{min-height:30px;padding:4px 8px;font-size:12px}.source-list{grid-column:1/-1;display:grid;gap:7px;padding:10px;border-radius:8px;background:#f7faf8}.source-row{display:flex;gap:10px;align-items:center;flex-wrap:wrap}.source-row strong{min-width:170px}.source-row span{font-size:12px}.receipt-form{grid-column:1/-1;padding:14px;border-radius:9px;background:#f7faf8;border-top:3px solid #2f8f5b}.form-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px;margin:12px 0}.form-grid label{display:grid;gap:5px}.form-grid label span,.source-note span{font-size:12px;color:#617069}.form-grid input,.form-grid select{width:100%;min-height:36px;border:1px solid #cbd5cf;border-radius:7px;padding:7px 9px;background:#fff;font:inherit}.form-grid .wide{grid-column:span 2}.source-note{display:grid;gap:5px;margin:8px 0 10px}.source-note strong{overflow-wrap:anywhere}.impact{padding:9px 10px;border-radius:7px;background:#fff8e8;color:#8a570c;font-size:12px;margin-bottom:10px}.submit{width:100%}.empty{text-align:center;padding:26px;color:#6f7b75}@media(max-width:980px){.receipt-row{grid-template-columns:repeat(3,minmax(0,1fr))}.product{grid-column:span 2}.form-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:620px){.page{padding:12px}.summary-grid,.receipt-row,.form-grid{grid-template-columns:1fr}.product,.form-grid .wide{grid-column:auto}.page-head,.form-title{display:grid}.row-actions{width:100%}.source-row{align-items:flex-start;flex-direction:column}}
</style>
