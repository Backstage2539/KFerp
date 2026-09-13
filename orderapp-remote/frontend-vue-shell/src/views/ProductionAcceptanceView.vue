<template>
  <div class="page finished-receipts-page">
    <ProductionTopNav v-if="!props.embedded" active-key="productionAcceptance" />

    <section class="panel page-head">
      <div><h2>完工入库</h2><p>核对末道工序报工、质量状态和增量入库数量，再完成过账。</p></div>
      <button class="secondary" type="button" :disabled="loading" @click="load">刷新</button>
    </section>

    <section class="summary-grid">
      <button v-for="item in summary" :key="item.key" type="button" :class="['summary-card', { active: status === item.key }]" @click="status = item.key">
        <span>{{ item.label }}</span><strong>{{ item.count }}</strong>
      </button>
    </section>

    <div v-if="message" class="notice success">{{ message }}</div>
    <div v-if="error" class="notice danger">{{ error }}</div>

    <section class="panel receipt-list">
      <div class="list-head"><strong>入库任务</strong><span>{{ filteredRows.length }} 项</span></div>
      <article v-for="row in filteredRows" :key="row.work_order_id" class="receipt-row">
        <div class="product"><strong>{{ outputTitle(row) }}</strong><span>{{ row.work_order_no }} · 目标仓 {{ row.target_warehouse || '按工单冻结仓库' }}</span></div>
        <div><span>已报工</span><strong>{{ qty(row.reported_qty) }} {{ row.output_unit || '' }}</strong></div>
        <div><span>已入库</span><strong>{{ qty(row.received_qty) }} {{ row.output_unit || '' }}</strong></div>
        <div><span>本次可入库</span><strong>{{ qty(row.available_qty) }} {{ row.output_unit || '' }}</strong></div>
        <div><span>质量</span><strong :class="{ blocked: row.quality_blocked }">{{ qualityLabel(row) }}</strong></div>
        <div class="row-actions">
          <button v-if="row.receipt_status !== 'received'" class="primary" type="button" :disabled="row.quality_blocked || Number(row.available_qty || 0) <= 0" @click="openReceipt(row)">办理入库</button>
          <button class="secondary" type="button" @click="openWorkOrder(row)">查看工单</button>
        </div>
        <div v-if="selected?.work_order_id === row.work_order_id" class="receipt-form">
          <div class="form-title"><div><strong>核对本次入库</strong><p>默认带入冻结目标仓和未入库报工数量。</p></div><button class="secondary" type="button" @click="selected = null">关闭</button></div>
          <div class="form-grid">
            <label><span>入库方式</span><select v-model="form.completion_mode" :disabled="row.output_type !== 'material'"><option value="partial">本次部分入库</option><option value="final">最后一次入库</option></select></label>
            <label><span>本次入库（{{ row.output_unit || '-' }}）</span><input v-model.number="form.quantity" type="number" min="0" step="any" /></label>
            <label><span>本次实际投入（g，可选）</span><input v-model.number="form.consumed_input_g" type="number" min="0" step="1" /></label>
            <label><span>目标仓</span><input :value="row.target_warehouse || '按工单冻结仓库'" disabled /></label>
            <label class="wide"><span>备注</span><input v-model.trim="form.note" placeholder="批次、差异或结清说明" /></label>
          </div>
          <div class="impact">过账后只记录本次增量；已确认的耗料和成本不会重复扣除。</div>
          <button class="primary" type="button" :disabled="saving" @click="submitReceipt(row)">{{ form.completion_mode === 'partial' ? '确认部分入库' : '确认最后入库' }}</button>
        </div>
      </article>
      <div v-if="!filteredRows.length" class="empty">当前没有{{ statusLabel(status) }}任务</div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import ProductionTopNav from '../components/ProductionTopNav.vue'

const props = defineProps({ embedded: { type: Boolean, default: false }, viewParams: { type: Object, default: () => ({}) } })
const rows = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const message = ref('')
const status = ref('pending')
const selected = ref(null)
const form = reactive({ completion_mode: 'final', quantity: 0, consumed_input_g: 0, note: '' })

const summary = computed(() => [
  { key: 'pending', label: '待入库', count: rows.value.filter((row) => row.receipt_status === 'pending').length },
  { key: 'partial', label: '部分入库', count: rows.value.filter((row) => row.receipt_status === 'partial').length },
  { key: 'received', label: '已入库', count: rows.value.filter((row) => row.receipt_status === 'received').length },
])
const filteredRows = computed(() => rows.value.filter((row) => row.receipt_status === status.value))

function qty(value) { return Number(value || 0).toLocaleString('zh-CN', { maximumFractionDigits: 3 }) }
function outputTitle(row) { const spec = Number(row.spec_g || 0) > 0 ? ` · ${row.spec_g}g` : ''; return `${row.output_name || '未命名产出'}${spec}` }
function statusLabel(value) { return ({ pending: '待入库', partial: '部分入库', received: '已入库' }[value] || '') }
function qualityLabel(row) { if (row.quality_blocked) return '质检阻塞'; return ({ pass: '已放行', unchecked: '未检查', ready: '可入库' }[row.quality_status] || '可入库') }
function openReceipt(row) { selected.value = row; form.completion_mode = row.output_type === 'material' && row.receipt_status === 'partial' ? 'partial' : 'final'; form.quantity = Number(row.available_qty || 0); form.consumed_input_g = 0; form.note = '' }
function openWorkOrder(row) { window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'workOrders', params: { work_order_id: row.work_order_id, focus: 'receipt' } } })) }
function requestID(row) { return globalThis.crypto?.randomUUID?.() || `receipt-${row.work_order_id}-${Date.now()}` }
function receiptPayload(row) {
  const quantity = Number(form.quantity || 0)
  const unit = String(row.output_unit || '').trim().toLowerCase()
  const weightFactor = ['kg', '千克', '公斤'].includes(unit) ? 1000 : (['g', '克'].includes(unit) ? 1 : 0)
  return {
    completion_mode: row.output_type === 'material' ? form.completion_mode : 'final',
    request_id: requestID(row),
    finished_qty_g: weightFactor ? Math.round(quantity * weightFactor) : 0,
    finished_loose_g: weightFactor ? Math.round(quantity * weightFactor) : 0,
    finished_qty_units: weightFactor ? 0 : Math.round(quantity),
    finished_units: weightFactor ? 0 : Math.round(quantity),
    consumed_input_g: Math.round(Number(form.consumed_input_g || 0)),
    note: form.note || '',
  }
}
async function submitReceipt(row) {
  if (Number(form.quantity || 0) <= 0) { error.value = '本次入库数量必须大于 0'; return }
  saving.value = true; error.value = ''; message.value = ''
  try {
    await apiSend(`/api/produce/work-orders/${row.work_order_id}/complete`, { body: receiptPayload(row) })
    message.value = form.completion_mode === 'partial' ? '部分入库已过账' : '最后一次入库已过账'
    selected.value = null
    await load()
  } catch (err) { error.value = err.message || '完工入库失败' } finally { saving.value = false }
}
async function load() {
  loading.value = true; error.value = ''
  try {
    const data = await apiGet('/api/produce/finished-receipts?status=all')
    rows.value = data.rows || []
    const requested = Number(props.viewParams?.work_order_id || 0)
    if (requested) {
      const row = rows.value.find((item) => Number(item.work_order_id) === requested)
      if (row) { status.value = row.receipt_status; if (row.receipt_status !== 'received') openReceipt(row) }
    }
  } catch (err) { error.value = err.message || '加载完工入库任务失败' } finally { loading.value = false }
}
onMounted(load)
</script>

<style scoped>
*{box-sizing:border-box}.page{padding:16px;display:grid;gap:14px;background:#f7f8fa;color:#19342b}.panel{border:1px solid #e0e5e2;border-radius:11px;background:#fff;padding:15px}.page-head,.list-head,.form-title{display:flex;justify-content:space-between;align-items:flex-start;gap:12px}.page-head h2{margin:0;font-size:22px}.page-head p,.form-title p{margin:5px 0 0;color:#6b7771;font-size:13px}.primary,.secondary{min-height:36px;border-radius:8px;padding:7px 12px;font:inherit;cursor:pointer}.primary{border:1px solid #2f8f5b;background:#2f8f5b;color:#fff}.secondary{border:1px solid #bec9c3;background:#fff;color:#28483c}button:disabled{opacity:.5;cursor:not-allowed}.summary-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px}.summary-card{display:grid;gap:5px;text-align:left;border:1px solid #e0e5e2;border-radius:10px;background:#fff;padding:13px;color:#617069}.summary-card strong{font-size:22px;color:#19342b}.summary-card.active{border-color:#2f8f5b;background:#eff9f2}.notice{padding:11px 13px;border-radius:9px}.notice.success{border:1px solid #acd4ba;background:#eff9f2;color:#24623d}.notice.danger{border:1px solid #efb1aa;background:#fff1ef;color:#9e3328}.receipt-list{display:grid;gap:8px}.list-head{padding-bottom:8px;border-bottom:1px solid #edf0ee}.list-head span{color:#6b7771}.receipt-row{display:grid;grid-template-columns:minmax(210px,1.4fr) repeat(4,minmax(100px,.65fr)) minmax(160px,.8fr);gap:12px;align-items:center;border:1px solid #e3e8e5;border-radius:10px;padding:12px}.receipt-row>div{display:grid;gap:4px}.receipt-row span{font-size:12px;color:#718079}.product span{font-size:12px}.blocked{color:#b7640d}.row-actions{display:flex!important;gap:7px;flex-wrap:wrap}.receipt-form{grid-column:1/-1;padding:14px;border-radius:9px;background:#f7faf8;border-top:3px solid #2f8f5b}.form-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px;margin:12px 0}.form-grid label{display:grid;gap:5px}.form-grid label span{font-size:12px}.form-grid input,.form-grid select{width:100%;min-height:36px;border:1px solid #cbd5cf;border-radius:7px;padding:7px 9px;background:#fff;font:inherit}.form-grid .wide{grid-column:span 2}.impact{padding:9px 10px;border-radius:7px;background:#fff8e8;color:#8a570c;font-size:12px;margin-bottom:10px}.empty{text-align:center;padding:26px;color:#6f7b75}@media(max-width:980px){.receipt-row{grid-template-columns:repeat(3,minmax(0,1fr))}.product{grid-column:span 2}.form-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:620px){.page{padding:12px}.summary-grid,.receipt-row,.form-grid{grid-template-columns:1fr}.product,.form-grid .wide{grid-column:auto}.page-head,.form-title{display:grid}.row-actions{width:100%}}
</style>
