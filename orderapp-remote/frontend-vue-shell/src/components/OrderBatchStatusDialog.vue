<template>
  <div class="batch-mask" @click.self="close">
    <section class="batch-dialog" role="dialog" aria-modal="true" aria-labelledby="batch-status-title">
      <header class="batch-head">
        <div><h3 id="batch-status-title">批量修改付款和发货状态</h3><p>共 {{ selectedRows.length }} 张订单。{{ paymentMode === 'combined' ? '合并付款及发货修改整批校验、一起保存。' : '每张订单单独校验和保存，失败不会影响其他订单。' }}</p></div>
        <button class="secondary" type="button" :disabled="busy" @click="close">关闭</button>
      </header>
      <div class="batch-modes" role="group" aria-label="付款处理方式">
        <label><input v-model="paymentMode" type="radio" value="individual" :disabled="busy || submitted || combinedUncertain || !!combinedResult" /> 逐单修改</label>
        <label><input v-model="paymentMode" type="radio" value="combined" :disabled="busy || submitted || combinedUncertain || !!combinedResult" /> 合并付款</label>
      </div>
      <div class="batch-targets">
        <label v-if="paymentMode === 'individual'">付款状态<select v-model.number="payStatusID" :disabled="busy || submitted"><option :value="0">不修改</option><option v-for="option in payStatuses" :key="option.id" :value="option.id">{{ option.name }}</option></select></label>
        <label>发货状态<select v-model.number="shipStatusID" :disabled="busy || submitted || combinedUncertain || !!combinedResult"><option :value="0">不修改</option><option v-for="option in shipStatuses" :key="option.id" :value="option.id">{{ option.name }}</option></select></label>
      </div>
      <p class="hint">{{ paymentMode === 'combined' ? '仅支持同一客户。已付款订单不重复收款；选择发货状态时，任一订单校验失败则整批不保存。' : '请核对每单资料。预付款和已收款等状态需要相应收款资料；发货状态受订单履约和物流信息限制。' }}</p>
      <p v-if="loading" role="status">正在读取所选订单…</p>
      <p v-if="error" role="alert" class="error">{{ error }} <button v-if="!drafts.length" class="secondary" type="button" :disabled="busy" @click="readOrders(selectedRows.map(row => Number(row.id)))">重新读取</button></p>
      <section v-if="paymentMode === 'combined'" class="combined-payment" aria-label="合并付款信息">
        <div v-if="combinedUncertain" class="combined-warning" role="alert">上次合并付款结果尚未确认，请重试原次操作。原订单：{{ pendingOrderIDs }}；本次收款 ¥{{ formatCents(pendingCombined?.actual_cents) }}；待收合计 ¥{{ formatCents(pendingCombined?.expected_due_cents) }}。重试会使用原订单及原发货设置。</div>
        <div v-if="combinedResult" class="ok" role="status">合并付款已完成。订单：{{ resultOrderIDs }}；本次收款 ¥{{ formatCents(combinedResult.actual_cents) }}，结清 ¥{{ formatCents(combinedResult.due_cents) }}，待分配 ¥{{ formatCents(combinedResult.unallocated_cents) }}。</div>
        <div v-if="!combinedUncertain && !combinedResult" class="combined-totals"><span>订单总应收 <strong>¥{{ formatCents(combinedSummary.totalCents) }}</strong></span><span>已有收款 <strong>¥{{ formatCents(combinedSummary.paidCents) }}</strong></span><span>本次待收合计 <strong>¥{{ formatCents(combinedSummary.dueCents) }}</strong></span><span>本次收款订单 {{ combinedSummary.payableCount }} 单</span></div>
        <p v-if="combinedSummary.error && !combinedUncertain && !combinedResult" class="error" role="alert">{{ combinedSummary.error }}</p>
        <p v-if="combinedStorageError && !combinedResult" class="error" role="alert">{{ combinedStorageError }}</p>
        <div v-if="!combinedResult" class="payment-fields">
          <label>本次实际收款（元）<input v-model="combinedActual" type="number" min="0" step="0.01" :disabled="busy || combinedUncertain" /></label>
          <label>收款方式<select v-model="combinedMethod" :disabled="busy || combinedUncertain"><option value="">请选择</option><option v-for="method in methods" :key="method" :value="method">{{ method }}</option></select></label>
          <label class="voucher">收款凭证（可选）<input type="file" accept="image/*,.pdf" :disabled="busy || combinedUncertain" @change="uploadCombinedVoucher" /><small>{{ combinedVoucherUploading ? '上传中…' : (combinedVoucherName || '未上传') }}</small></label>
        </div>
        <p v-if="combinedVoucherError" class="error" role="alert">{{ combinedVoucherError }} <button class="secondary" type="button" :disabled="busy" @click="clearCombinedVoucher">不使用凭证</button></p>
        <p v-if="combinedAmountError && !combinedUncertain && !combinedResult" class="error" role="alert">{{ combinedAmountError }}</p>
        <p v-if="combinedExcess > 0 && !combinedUncertain && !combinedResult" class="hint">超出待收的 ¥{{ formatCents(combinedExcess) }} 将记录为待分配，不自动抵扣其他订单。</p>
        <p v-if="combinedNeedsRefresh && !combinedUncertain" class="error">请重新读取所有订单并核对金额后再提交。 <button class="secondary" type="button" :disabled="busy" @click="readOrders(selectedRows.map(row => Number(row.id)))">重新读取</button></p>
      </section>
      <div v-if="drafts.length && !combinedUncertain && !combinedResult" class="batch-orders">
        <article v-for="draft in drafts" :key="draft.order_id" class="batch-order">
          <div class="order-heading">
            <strong>{{ draft.order_no || `订单 #${draft.order_id}` }}</strong>
            <span>{{ customerName(draft.order_id) }}</span>
            <span :class="{ error: draft.result === 'failed', ok: draft.result === 'updated' || draft.result === 'unchanged' }">{{ batchStatusResultLabel(draft.result) }}</span>
          </div>
          <p class="current">{{ isDone(draft) ? '修改前付款' : '已保存付款' }}：{{ statusName(payStatuses, draft.pay_status_id) }}{{ draft.saved.payment_method ? ` / ${draft.saved.payment_method}` : '' }}；发货：{{ draft.delivery_mode === 'customer_stock' ? '留厂库存，无需发货' : statusName(shipStatuses, draft.ship_status_id) }}</p>
          <p v-if="paymentMode === 'combined'" class="current">应收 ¥{{ formatCents(draft.total_cents) }} · 已收 ¥{{ formatCents(draft.paid_cents) }} · 待收 ¥{{ formatCents(draft.due_cents) }} <span v-if="draft.fully_paid">· 已付款，不重复收款</span></p>
          <p v-else class="current">{{ isDone(draft) ? '修改前资料' : '已保存资料' }}：预付款 {{ displayAmount(draft.saved.prepayment_amount) }} 元；实收货款 {{ displayAmount(draft.saved.payment_goods_amount) }} 元；实收运费 {{ displayAmount(draft.saved.payment_shipping_amount) }} 元；凭证 {{ draft.saved.payment_voucher_filename || (draft.saved.payment_voucher_asset_id ? '已保存' : '未上传') }}</p>
          <p v-if="draft.resultError" class="error" role="alert">{{ draft.resultError }}</p>
          <button v-if="draft.uploadError" class="secondary" type="button" :disabled="busy" @click="keepVoucher(draft)">不更换凭证</button>
          <div v-if="paymentMode === 'individual' && payStatusID && !draft.error && !isDone(draft) && draft.settlement_reason !== 'free_sample'" class="payment-fields">
            <label>收款方式<select v-model="draft.payment_method" :disabled="busy"><option value="">请选择</option><option v-for="method in methods" :key="method" :value="method">{{ method }}</option></select></label>
            <label>预付款（元）<input v-model="draft.prepayment_amount" type="number" min="0" step="0.01" :disabled="busy" /></label>
            <label>实收货款（元）<input v-model="draft.payment_goods_amount" type="number" min="0" step="0.01" :disabled="busy" /></label>
            <label>实收运费（元）<input v-model="draft.payment_shipping_amount" type="number" min="0" step="0.01" :disabled="busy" /></label>
            <label class="voucher">收款凭证<input type="file" accept="image/*,.pdf" :disabled="busy" @change="uploadVoucher(draft, $event)" /><small>{{ draft.uploading ? '上传中…' : (draft.payment_voucher_filename || (draft.payment_voucher_asset_id ? '已保存' : '未上传')) }}</small></label>
          </div>
        </article>
      </div>
      <footer class="batch-actions">
        <button v-if="paymentMode === 'individual' && hasFailed" class="secondary" type="button" :disabled="busy" title="更新保存版本和资料，保留手动修改的输入" @click="reloadFailures">重新读取失败订单</button>
        <button class="secondary" type="button" :disabled="busy" @click="close">{{ submitted || combinedResult ? '完成' : '取消' }}</button>
        <button v-if="paymentMode === 'combined' && !combinedResult" class="primary" type="button" :disabled="busy || loading || !!combinedStorageError || (!combinedUncertain && (!drafts.length || !!combinedSummary.error || combinedSummary.dueCents <= 0 || !combinedActual || !!combinedAmountError || !combinedMethod || combinedNeedsRefresh || !!combinedVoucherError))" @click="submitCombined">{{ submitting ? '保存中…' : combinedUncertain ? '重试原次合并付款' : '确认合并付款' }}</button>
        <button v-if="paymentMode === 'individual'" class="primary" type="button" :disabled="busy || loading || !drafts.length || (!payStatusID && !shipStatusID) || !hasPending" @click="submit">{{ submitting ? '保存中…' : submitted ? '重试未成功订单' : '确认修改' }}</button>
      </footer>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiSend } from '../api/client'
import { orderReceiptMethodOptions as methods } from '../lib/order-entry'
import { batchStatusRequest, batchStatusResultLabel, makeBatchStatusDraft, refreshBatchStatusDraft } from '../lib/order-batch-status'
import { combinedPaymentRequest, combinedPaymentSummary, parseCombinedMoneyCents } from '../lib/combined-order-payment'

const props = defineProps({ selectedRows: { type: Array, required: true }, payStatuses: { type: Array, required: true }, shipStatuses: { type: Array, required: true } })
const emit = defineEmits(['close', 'updated'])
const drafts = ref([])
const payStatusID = ref(0)
const shipStatusID = ref(0)
const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const submitted = ref(false)
const paymentMode = ref('individual')
const combinedActual = ref(''), combinedMethod = ref(''), combinedVoucherID = ref(0), combinedVoucherName = ref('')
const combinedVoucherUploading = ref(false), combinedVoucherError = ref('')
const combinedStorageError = ref('')
const combinedNeedsRefresh = ref(false), combinedUncertain = ref(false), combinedResult = ref(null), pendingCombined = ref(null)
const resultOrderIDs = ref('')
const pendingStorageKey = 'kferp.order-combined-payment.pending'
const combinedSummary = computed(() => combinedPaymentSummary(drafts.value))
const combinedAmountError = computed(() => {
  if (!String(combinedActual.value).trim()) return ''
  try {
    return parseCombinedMoneyCents(combinedActual.value) < combinedSummary.value.dueCents ? '实际收款不足待收合计，不能确认付款' : ''
  } catch (cause) { return cause.message }
})
const combinedExcess = computed(() => {
  try { return Math.max(0, parseCombinedMoneyCents(combinedActual.value) - combinedSummary.value.dueCents) } catch { return 0 }
})
const pendingOrderIDs = computed(() => (pendingCombined.value?.orders || []).map(row => `#${row.order_id}`).join('、'))
const busy = computed(() => loading.value || submitting.value || combinedVoucherUploading.value || drafts.value.some(d => d.uploading))
const hasFailed = computed(() => drafts.value.some(d => d.result === 'failed'))
const hasPending = computed(() => drafts.value.some(d => !isDone(d) && !d.error && !d.requiresReload && !d.uploadError))
function isDone(draft) { return draft.result === 'updated' || draft.result === 'unchanged' }
function displayAmount(value) { return String(value ?? '').trim() || '0' }
function formatCents(value) { return (Number.isSafeInteger(Number(value)) ? Number(value) / 100 : 0).toFixed(2) }
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
    combinedNeedsRefresh.value = false
  } catch (cause) {
    error.value = cause.message || '读取订单失败，请重试'
  } finally { loading.value = false; restorePendingCombined() }
}
function reloadFailures() { return readOrders(drafts.value.filter(d => d.result === 'failed').map(d => Number(d.order_id)), true) }
function keepVoucher(draft) { draft.uploadError = false; draft.uploadErrorMessage = ''; draft.resultError = draft.error || '' }
async function uploadVoucher(draft, event) {
  const file = event.target.files?.[0]
  if (!file) return
  draft.uploading = true
  draft.uploadError = false
  draft.uploadErrorMessage = ''
  draft.resultError = ''
  try {
    const body = new FormData()
    body.append('file', file)
    const response = await apiSend('/api/order/payment-vouchers', { body })
    if (!response.asset?.id) throw new Error('凭证上传未完成，请重试')
    draft.payment_voucher_asset_id = Number(response.asset.id)
    draft.payment_voucher_filename = response.asset.filename || file.name
  } catch (cause) { draft.uploadError = true; draft.uploadErrorMessage = (cause.message || '凭证上传失败') + '，请重新上传或选择不更换凭证'; draft.resultError = draft.uploadErrorMessage }
  finally { draft.uploading = false; event.target.value = '' }
}

function combinedStorage() {
  try {
    if (typeof window === 'undefined' || !window.sessionStorage) throw new Error('unavailable')
    return window.sessionStorage
  } catch { throw new Error('浏览器无法保存本次付款的重试记录，请检查浏览器存储设置') }
}

function restorePendingCombined() {
  if (combinedResult.value || pendingCombined.value) return
  let raw
  try {
    raw = combinedStorage().getItem(pendingStorageKey)
    combinedStorageError.value = ''
  } catch {
    combinedStorageError.value = '浏览器存储不可用，无法安全保存付款重试记录；请检查浏览器设置。'
    return
  }
  if (!raw) return
  try {
    const pending = JSON.parse(raw)
    if (!pending || !Array.isArray(pending.orders) || !pending.orders.length || !pending.idempotency_key || !Number.isSafeInteger(pending.actual_cents) || !Number.isSafeInteger(pending.expected_due_cents)) throw new Error('invalid')
    pendingCombined.value = pending
    combinedUncertain.value = true
    paymentMode.value = 'combined'
    combinedActual.value = formatCents(pending.actual_cents)
    combinedMethod.value = pending.payment_method || ''
    combinedVoucherID.value = Number(pending.payment_voucher_asset_id || 0)
    combinedVoucherName.value = combinedVoucherID.value ? '原次操作已上传' : ''
    shipStatusID.value = Number(pending.ship_status_id || 0)
  } catch (cause) {
    // A malformed saved record must not be silently discarded: it may describe
    // a payment whose response was lost. Block new combined payments.
    combinedStorageError.value = '无法读取上次未确认付款记录，请先联系管理员核对订单及收款记录。'
  }
}

async function uploadCombinedVoucher(event) {
  const file = event.target.files?.[0]
  if (!file || combinedUncertain.value) return
  combinedVoucherUploading.value = true
  combinedVoucherError.value = ''
  try {
    const body = new FormData()
    body.append('file', file)
    const response = await apiSend('/api/order/payment-vouchers', { body })
    if (!response.asset?.id) throw new Error('凭证上传未完成')
    combinedVoucherID.value = Number(response.asset.id)
    combinedVoucherName.value = response.asset.filename || file.name
  } catch (cause) {
    combinedVoucherError.value = (cause.message || '凭证上传失败') + '，请重新上传或选择不使用凭证'
  } finally { combinedVoucherUploading.value = false; event.target.value = '' }
}

function clearCombinedVoucher() { combinedVoucherID.value = 0; combinedVoucherName.value = ''; combinedVoucherError.value = '' }

async function submitCombined() {
  if (busy.value || combinedResult.value || combinedVoucherError.value || combinedStorageError.value || (combinedNeedsRefresh.value && !combinedUncertain.value)) return
  error.value = ''
  const wasUncertain = combinedUncertain.value
  let request = pendingCombined.value
  if (!request) {
    try {
      const key = globalThis.crypto?.randomUUID?.() || `combined-${Date.now()}-${Math.random().toString(36).slice(2)}`
      request = combinedPaymentRequest(drafts.value, { actual: combinedActual.value, method: combinedMethod.value, key, voucherID: combinedVoucherID.value, shipStatusID: shipStatusID.value })
      combinedStorage().setItem(pendingStorageKey, JSON.stringify(request))
      pendingCombined.value = request
    } catch (cause) { error.value = cause.message || '无法准备付款'; return }
  }
  submitting.value = true
  try {
    const response = await apiSend('/api/orders/status-batch/combined-payment', { body: request })
    if (!Number.isSafeInteger(Number(response?.batch_id)) || Number(response.batch_id) <= 0 || !Number.isSafeInteger(response.actual_cents) || !Number.isSafeInteger(response.due_cents) || !Number.isSafeInteger(response.unallocated_cents)) throw new Error('未收到完整收款结果')
    resultOrderIDs.value = request.orders.map(row => `#${row.order_id}`).join('、')
    combinedResult.value = response
    combinedUncertain.value = false
    pendingCombined.value = null
    try { combinedStorage().removeItem(pendingStorageKey) } catch { /* A replay of the same key remains safe. */ }
    emit('updated')
    await readOrders(props.selectedRows.map(row => Number(row.id)))
    if (error.value) error.value = `收款已保存，但订单刷新失败：${error.value}。请关闭后重新读取。`
  } catch (cause) {
    if (!wasUncertain && Number(cause?.status) >= 400 && Number(cause?.status) < 500) {
      try { combinedStorage().removeItem(pendingStorageKey) } catch { /* leave the same key for safe review */ }
      pendingCombined.value = null
      combinedNeedsRefresh.value = true
      error.value = cause.message || '付款未保存，请重新读取订单'
    } else {
      combinedUncertain.value = true
      error.value = '未能确认本次付款结果，请重试原次合并付款；系统不会重复入账。' + (cause.message ? ` ${cause.message}` : '')
    }
  } finally { submitting.value = false }
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
.batch-targets, .payment-fields, .batch-modes, .combined-totals { display: flex; flex-wrap: wrap; gap: 12px; }
.batch-modes label { display: flex; align-items: center; gap: 5px; font-size: 13px; }
.combined-payment { border: 1px solid #cde5d5; background: #f6fbf7; border-radius: 8px; padding: 14px; display: grid; gap: 10px; }
.combined-payment p { margin: 0; }
.combined-totals { align-items: center; justify-content: space-between; font-size: 13px; color: #435d4c; }
.combined-totals strong { font-size: 17px; color: #187144; }
.combined-warning { color: #8a5200; background: #fff4d6; border-radius: 6px; padding: 10px; font-size: 13px; }
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
