<template>
  <div class="batch-mask" @click.self="close">
    <section class="batch-dialog" role="dialog" aria-modal="true" aria-labelledby="batch-status-title">
      <header class="batch-head">
        <div><h3 id="batch-status-title">批量修改付款和发货状态</h3><p>共 {{ selectedRows.length }} 张订单。每张订单单独校验和保存，失败不会影响其他订单。</p></div>
        <button class="secondary" type="button" :disabled="busy" @click="close">关闭</button>
      </header>
      <div class="batch-targets">
        <label>付款状态<select v-model.number="payStatusID" :disabled="busy || submitted"><option :value="0">不修改</option><option v-for="option in payStatuses" :key="option.id" :value="option.id">{{ option.name }}</option></select></label>
        <label>发货状态<select v-model.number="shipStatusID" :disabled="busy || submitted"><option :value="0">不修改</option><option v-for="option in shipStatuses" :key="option.id" :value="option.id">{{ option.name }}</option></select></label>
      </div>
      <p class="hint">请核对每单资料。预付款和已收款等状态需要相应收款资料；发货状态受订单履约和物流信息限制。</p>
      <p v-if="loading" role="status">正在读取所选订单…</p>
      <p v-if="error" role="alert" class="error">{{ error }} <button v-if="!drafts.length" class="secondary" type="button" :disabled="busy" @click="readOrders(selectedRows.map(row => Number(row.id)))">重新读取</button></p>
      <div v-if="drafts.length" class="batch-orders">
        <article v-for="draft in drafts" :key="draft.order_id" class="batch-order">
          <div class="order-heading">
            <strong>{{ draft.order_no || `订单 #${draft.order_id}` }}</strong>
            <span>{{ customerName(draft.order_id) }}</span>
            <span :class="{ error: draft.result === 'failed', ok: draft.result === 'updated' || draft.result === 'unchanged' }">{{ batchStatusResultLabel(draft.result) }}</span>
          </div>
          <p class="current">{{ isDone(draft) ? '修改前付款' : '已保存付款' }}：{{ statusName(payStatuses, draft.pay_status_id) }}{{ draft.saved.payment_method ? ` / ${draft.saved.payment_method}` : '' }}；发货：{{ draft.delivery_mode === 'customer_stock' ? '留厂库存，无需发货' : statusName(shipStatuses, draft.ship_status_id) }}</p>
          <p class="current">{{ isDone(draft) ? '修改前资料' : '已保存资料' }}：预付款 {{ displayAmount(draft.saved.prepayment_amount) }} 元；实收货款 {{ displayAmount(draft.saved.payment_goods_amount) }} 元；实收运费 {{ displayAmount(draft.saved.payment_shipping_amount) }} 元；凭证 {{ draft.saved.payment_voucher_filename || (draft.saved.payment_voucher_asset_id ? '已保存' : '未上传') }}</p>
          <p v-if="draft.resultError" class="error" role="alert">{{ draft.resultError }}</p>
          <button v-if="draft.uploadError" class="secondary" type="button" :disabled="busy" @click="draft.uploadError = false; draft.resultError = ''">不更换凭证</button>
          <div v-if="payStatusID && !draft.error && !isDone(draft) && draft.settlement_reason !== 'free_sample'" class="payment-fields">
            <label>收款方式<select v-model="draft.payment_method" :disabled="busy"><option value="">请选择</option><option v-for="method in methods" :key="method" :value="method">{{ method }}</option></select></label>
            <label>预付款（元）<input v-model="draft.prepayment_amount" type="number" min="0" step="0.01" :disabled="busy" /></label>
            <label>实收货款（元）<input v-model="draft.payment_goods_amount" type="number" min="0" step="0.01" :disabled="busy" /></label>
            <label>实收运费（元）<input v-model="draft.payment_shipping_amount" type="number" min="0" step="0.01" :disabled="busy" /></label>
            <label class="voucher">收款凭证<input type="file" accept="image/*,.pdf" :disabled="busy" @change="uploadVoucher(draft, $event)" /><small>{{ draft.uploading ? '上传中…' : (draft.payment_voucher_filename || (draft.payment_voucher_asset_id ? '已保存' : '未上传')) }}</small></label>
          </div>
        </article>
      </div>
      <footer class="batch-actions">
        <button v-if="hasFailed" class="secondary" type="button" :disabled="busy" title="更新保存版本和资料，保留手动修改的输入" @click="reloadFailures">重新读取失败订单</button>
        <button class="secondary" type="button" :disabled="busy" @click="close">{{ submitted ? '完成' : '取消' }}</button>
        <button class="primary" type="button" :disabled="busy || loading || !drafts.length || (!payStatusID && !shipStatusID) || !hasPending" @click="submit">{{ submitting ? '保存中…' : submitted ? '重试未成功订单' : '确认修改' }}</button>
      </footer>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiSend } from '../api/client'
import { orderReceiptMethodOptions as methods } from '../lib/order-entry'
import { batchStatusRequest, batchStatusResultLabel, makeBatchStatusDraft, refreshBatchStatusDraft } from '../lib/order-batch-status'

const props = defineProps({ selectedRows: { type: Array, required: true }, payStatuses: { type: Array, required: true }, shipStatuses: { type: Array, required: true } })
const emit = defineEmits(['close', 'updated'])
const drafts = ref([])
const payStatusID = ref(0)
const shipStatusID = ref(0)
const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const submitted = ref(false)
const busy = computed(() => loading.value || submitting.value || drafts.value.some(d => d.uploading))
const hasFailed = computed(() => drafts.value.some(d => d.result === 'failed'))
const hasPending = computed(() => drafts.value.some(d => !isDone(d) && !d.error && !d.requiresReload && !d.uploadError))
function isDone(draft) { return draft.result === 'updated' || draft.result === 'unchanged' }
function displayAmount(value) { return String(value ?? '').trim() || '0' }
function statusName(options, id) { return options.find(o => Number(o.id) === Number(id))?.name || '未设置' }
function customerName(id) { return props.selectedRows.find(row => Number(row.id) === Number(id))?.customer || '' }
function close() { if (!busy.value) emit('close') }

async function readOrders(ids, replace = false) {
  loading.value = true
  error.value = ''
  try {
    const response = await apiSend('/api/orders/status-batch/preview', { body: { order_ids: ids } })
    const received = new Map((response.orders || []).map(order => [Number(order.order_id), makeBatchStatusDraft(order)]))
    const next = ids.map(id => received.get(Number(id)) || makeBatchStatusDraft({ order_id: id, error: '未能读取订单，请重试' }))
    drafts.value = replace ? drafts.value.map(old => received.has(Number(old.order_id)) ? refreshBatchStatusDraft(old, received.get(Number(old.order_id))) : old) : next
  } catch (cause) {
    error.value = cause.message || '读取订单失败，请重试'
  } finally { loading.value = false }
}
function reloadFailures() { return readOrders(drafts.value.filter(d => d.result === 'failed').map(d => Number(d.order_id)), true) }
async function uploadVoucher(draft, event) {
  const file = event.target.files?.[0]
  if (!file) return
  draft.uploading = true
  draft.uploadError = false
  draft.resultError = ''
  try {
    const body = new FormData()
    body.append('file', file)
    const response = await apiSend('/api/order/payment-vouchers', { body })
    if (!response.asset?.id) throw new Error('凭证上传未完成，请重试')
    draft.payment_voucher_asset_id = Number(response.asset.id)
    draft.payment_voucher_filename = response.asset.filename || file.name
  } catch (cause) { draft.uploadError = true; draft.resultError = (cause.message || '凭证上传失败') + '，请重新上传或选择不更换凭证' }
  finally { draft.uploading = false; event.target.value = '' }
}
async function submit() {
  error.value = ''
  const payload = []
  for (const draft of drafts.value) {
    if (isDone(draft) || draft.error || draft.requiresReload || draft.uploadError) continue
    try { payload.push(batchStatusRequest(draft, payStatusID.value, shipStatusID.value)); draft.resultError = '' }
    catch (cause) { draft.result = 'failed'; draft.resultError = cause.message }
  }
  submitted.value = true
  if (!payload.length) return
  submitting.value = true
  try {
    const response = await apiSend('/api/orders/status-batch', { body: { orders: payload } })
    const results = new Map((response.results || []).map(result => [Number(result.order_id), result]))
    for (const item of payload) {
      const draft = drafts.value.find(d => Number(d.order_id) === item.order_id)
      const result = results.get(item.order_id)
      const knownStatus = ['updated', 'unchanged', 'failed'].includes(result?.status)
      draft.result = knownStatus ? result.status : 'failed'
      draft.requiresReload = !knownStatus
      draft.resultError = knownStatus ? (result.error || '') : '未收到处理结果，请重新读取后确认'
    }
    emit('updated')
  } catch (cause) {
    error.value = cause.message || '提交失败，请重新读取订单后再试'
    for (const item of payload) {
      const draft = drafts.value.find(d => Number(d.order_id) === item.order_id)
      draft.result = 'failed'
      draft.requiresReload = true
      draft.resultError = '提交结果未确认，请重新读取订单后再重试'
    }
  } finally { submitting.value = false }
}
onMounted(() => readOrders(props.selectedRows.map(row => Number(row.id))))
</script>

<style scoped>
.batch-mask { position: fixed; inset: 0; z-index: 130; background: rgba(14, 25, 20, .48); display: flex; align-items: center; justify-content: center; padding: 20px; }
.batch-dialog { background: #fff; border-radius: 12px; box-shadow: 0 18px 50px #0003; width: min(1080px, 100%); max-height: 92vh; display: flex; flex-direction: column; padding: 20px; gap: 12px; overflow: auto; }
.batch-head, .order-heading, .batch-actions { display: flex; align-items: center; gap: 12px; justify-content: space-between; }
.batch-head h3, .batch-head p, .current, .hint { margin: 0; }
.batch-head p, .current, .hint, small { color: #66756e; font-size: 13px; }
.batch-targets, .payment-fields { display: flex; flex-wrap: wrap; gap: 12px; }
.batch-targets label, .payment-fields label { display: flex; flex-direction: column; gap: 5px; font-size: 13px; min-width: 140px; }
.payment-fields label { flex: 1 1 140px; }
input, select { border: 1px solid #ccd8d0; border-radius: 5px; padding: 7px; min-width: 0; }
input[type=file] { max-width: 230px; }
.batch-orders { overflow: auto; display: grid; gap: 10px; }
.batch-order { border: 1px solid #dce6df; border-radius: 8px; padding: 12px; display: grid; gap: 8px; }
.order-heading { justify-content: flex-start; flex-wrap: wrap; }
.order-heading span:last-child { margin-left: auto; }
.error { color: #b42318; }
.ok { color: #187144; }
.batch-actions { justify-content: flex-end; }
button:disabled { opacity: .55; cursor: not-allowed; }
</style>
