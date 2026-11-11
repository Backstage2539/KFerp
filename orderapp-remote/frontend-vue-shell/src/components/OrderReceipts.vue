<template>
  <details class="drawer-section receipts" @toggle="opened">
    <summary>逐笔收款</summary>
    <p v-if="loading" role="status">正在读取收款信息…</p>
    <p v-if="error" role="alert" class="error">{{ error }}</p>
    <p v-if="!canWrite && !actorIdentity">当前账号身份无法确认，无法安全登记收款。</p>
    <div v-if="pending" role="alert" class="warning">上次登记结果尚未确认：{{ pending.receipt_date }} · ¥{{ money(pending.actual_cents) }}。请重试原次登记，系统不会重复收款。</div>
    <template v-if="canWrite && preview && !pending && !result">
      <p>订单应收 ¥{{ money(preview.total_cents) }} · 已收 ¥{{ money(preview.paid_cents) }} · 待收 ¥{{ money(preview.due_cents) }}</p>
      <div v-if="preview.due_cents > 0 && !preview.error" class="fields">
        <label>本次到账金额（元）<input v-model="amount" type="number" min="0" step="0.01" :disabled="busy" /></label>
        <label>实际到账日期<input v-model="date" type="date" :disabled="busy" /></label>
        <label>收款方式<select v-model="method" :disabled="busy"><option value="">请选择</option><option v-for="item in methods" :key="item" :value="item">{{ item }}</option></select></label>
        <label>收款凭证（可选）<input type="file" accept="image/*,.pdf" :disabled="busy" @change="upload" /><small>{{ voucherName || '未上传' }}</small></label>
        <button type="button" class="primary" :disabled="busy" @click="submit">登记收款</button>
      </div>
      <p v-else-if="preview.error">{{ preview.error }}</p>
    </template>
    <button v-if="pending" type="button" class="secondary" :disabled="busy" @click="retry(false)">重试原次登记</button>
    <p v-if="result" role="status">登记成功：{{ result.receipt_no }}。 <button type="button" @click="again">继续登记</button></p>
    <h4>本订单逐笔收款</h4>
    <p v-if="loaded && !rows.length">暂无本订单录单或逐笔登记的收款。合并收款请查看“合并收款记录”，账单收款请查看账单。</p>
    <article v-for="row in rows" :key="row.receipt_id">
      <strong>{{ row.receipt_no }}</strong>
      <p>实际到账日期：{{ row.receipt_date || '未记录' }} · {{ row.payment_method }} · {{ row.actor }}</p>
      <p>本次到账 ¥{{ money(row.actual_cents) }} · 应收总额 ¥{{ money(row.receivable_cents) }}</p>
      <p v-if="row.payment_voucher_asset_id">本次收款凭证已关联</p>
    </article>
  </details>
</template>
<script setup>
import { ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { orderReceiptMethodOptions as methods } from '../lib/order-entry'
import { orderReceiptRequest, receiptPendingKey, receiptKeepPending } from '../lib/order-receipt.js'
const props = defineProps({ orderId: { type: Number, required: true }, actorIdentity: { type: String, default: '' }, canWrite: { type: Boolean, default: false } })
const emit = defineEmits(['updated'])
const loading=ref(false), busy=ref(false), loaded=ref(false), rows=ref([]), preview=ref(null), error=ref(''), result=ref(null)
const amount=ref(''), date=ref(''), method=ref(''), voucherID=ref(0), voucherName=ref(''), pending=ref(null)
const storageKey = props.actorIdentity ? receiptPendingKey(props.actorIdentity, props.orderId) : ''
function money(n) { return (Number(n || 0)/100).toFixed(2) }
async function load() {
 loading.value=true; error.value=''
 try {
  const jobs=[apiGet(`/api/orders/${props.orderId}/receipts`)]
  if (props.canWrite) jobs.push(apiSend('/api/orders/status-batch/preview',{body:{order_ids:[props.orderId]}}))
  const [history, details] = await Promise.all(jobs)
  rows.value=Array.isArray(history.receipts)?history.receipts:[]; preview.value=details?.orders?.[0] || null; loaded.value=true
 } catch(e) { error.value=e.message || '读取收款信息失败' } finally {loading.value=false}
}
function opened(e) {
 if (!e.target.open || loaded.value || loading.value) return
 try { const raw=storageKey && localStorage.getItem(storageKey); if(raw) pending.value=JSON.parse(raw) } catch { error.value='无法读取待确认记录，请联系管理员核对后再登记'; return }
 load()
}
async function upload(e) {
 const file=e.target.files?.[0]; if(!file)return
 busy.value=true; error.value=''
 try {const body=new FormData();body.append('file',file);const out=await apiSend('/api/order/payment-vouchers',{body});if(!out.asset?.id)throw new Error('凭证上传未完成');voucherID.value=Number(out.asset.id);voucherName.value=out.asset.filename||file.name}
 catch(err){error.value=err.message||'凭证上传失败';voucherID.value=0;voucherName.value=''} finally{busy.value=false;e.target.value=''}
}
async function submit() {
 error.value=''
 try {
  const key=globalThis.crypto?.randomUUID?.() || `receipt-${Date.now()}-${Math.random().toString(36).slice(2)}`
  const body=orderReceiptRequest({orderId:props.orderId,revision:preview.value?.edit_revision,dueCents:preview.value?.due_cents,amount:amount.value,date:date.value,method:method.value,key,voucherID:voucherID.value})
  localStorage.setItem(storageKey,JSON.stringify(body)); pending.value=body
  await retry(true)
 } catch(err) {error.value=err.message || '登记收款失败'}
}
async function retry(firstAttempt = false) {
 if(!pending.value)return
 busy.value=true; error.value=''
 try {
  const data=await apiSend(`/api/orders/${props.orderId}/receipts`,{body:pending.value})
  localStorage.removeItem(storageKey);pending.value=null;result.value=data;emit('updated');await load()
 } catch(err) {
  if (receiptKeepPending(err.status,err.not_committed,firstAttempt)) error.value = `登记结果未确认，请重试原次登记或联系管理员核对；不要另建收款。${err.message ? `（${err.message}）` : ''}`
  else { try { localStorage.removeItem(storageKey); pending.value=null; error.value=err.message || '收款未提交，请核对后重试'; await load() } catch { error.value='无法清除待确认记录，请联系管理员核对' } }
 } finally {busy.value=false}
}
function again(){result.value=null;amount.value='';date.value='';method.value='';voucherID.value=0;voucherName.value='';load()}
</script>
<style scoped>
.receipts summary { cursor:pointer; font-weight:600 }
.receipts article { margin-top:10px; padding:10px; border:1px solid #dce6df; border-radius:6px }
.receipts p { margin:6px 0 }
.fields { display:flex; flex-wrap:wrap; align-items:end; gap:10px }
.fields label { display:grid; gap:4px }
.warning,.error { color:#a52b20 }
</style>
