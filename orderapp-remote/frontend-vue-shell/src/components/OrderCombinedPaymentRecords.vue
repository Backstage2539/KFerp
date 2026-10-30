<template>
  <details class="drawer-section combined-payments" @toggle="onToggle">
    <summary>合并收款记录</summary>
    <p v-if="loading" role="status">正在读取收款记录…</p>
    <p v-else-if="error" role="alert">{{ error }} <button type="button" @click="load">重试</button></p>
    <p v-else-if="loaded && !payments.length">暂无合并收款记录</p>
    <article v-for="payment in payments" :key="payment.batch_id">
      <strong>合并收款 #{{ payment.batch_id }}</strong>
      <p>{{ payment.created_at }} · {{ payment.payment_method }} · {{ payment.actor }}</p>
      <p>本次实际收款 ¥{{ money(payment.actual_cents) }} · 订单待收合计 ¥{{ money(payment.due_cents) }}</p>
      <p>本订单分配 ¥{{ money(ownAllocation(payment)) }} · 超额待分配 ¥{{ money(payment.unallocated_cents) }}</p>
      <p v-if="payment.payment_voucher_asset_id">本次收款凭证已关联</p>
      <small>待分配金额属于整笔合并收款，不自动抵扣其他订单。</small>
    </article>
  </details>
</template>

<script setup>
import { ref, watch } from 'vue'
import { apiGet } from '../api/client'

const props = defineProps({ orderId: { type: Number, required: true } })
const payments = ref([]), loading = ref(false), loaded = ref(false), error = ref('')
let sequence = 0
function money(cents) { return (Number(cents || 0) / 100).toFixed(2) }
function ownAllocation(payment) { return (payment.allocations || []).filter(row => Number(row.order_id) === props.orderId).reduce((sum, row) => sum + Number(row.amount_cents || 0), 0) }
async function load() {
  const id = props.orderId, current = ++sequence
  if (!id) return
  loading.value = true; error.value = ''
  try {
    const result = await apiGet(`/api/orders/${id}/combined-payments`)
    if (current !== sequence || id !== props.orderId) return
    payments.value = Array.isArray(result?.payments) ? result.payments : []
    loaded.value = true
  } catch (cause) {
    if (current === sequence) error.value = cause.status === 403 ? '当前账号无权查看收款记录' : (cause.message || '读取收款记录失败')
  } finally { if (current === sequence) loading.value = false }
}
function onToggle(event) { if (event.target.open && !loaded.value && !loading.value) load() }
watch(() => props.orderId, () => { sequence++; payments.value = []; loading.value = false; loaded.value = false; error.value = '' })
</script>

<style scoped>
.combined-payments summary { cursor: pointer; font-weight: 600; }
.combined-payments article { margin-top: 10px; padding: 10px; border: 1px solid #dce6df; border-radius: 6px; }
.combined-payments p { margin: 6px 0; }
.combined-payments small { color: #66756e; }
</style>
