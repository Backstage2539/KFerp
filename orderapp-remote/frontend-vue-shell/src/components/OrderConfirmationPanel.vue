<template>
  <section class="confirmation-panel">
    <p v-if="status"><strong>{{ orderConfirmationLabel(status?.confirmation_status) }}</strong><span v-if="status?.confirmation_reason"> · {{ status.confirmation_reason }}</span></p>
    <p v-if="status">生产：{{ status.process_status || '待生产' }} · 发货：{{ status.ship_status || '待发货' }}</p>
    <p v-if="error" role="alert" class="error">{{ error }}</p>
    <p v-if="message" role="status">{{ message }}</p>
    <template v-if="!editing">
      <button v-if="status?.can_edit" type="button" :disabled="busy" @click="editing = true">修改订单</button>
      <span v-else-if="status?.edit_block_reason">{{ status.edit_block_reason }}</span>
      <template v-if="status?.can_confirm && !customerPortal">
        <button type="button" :disabled="busy" @click="review('accepted')">确认订单</button>
        <input v-model="reason" :disabled="busy" placeholder="拒绝原因" aria-label="拒绝原因" />
        <button type="button" :disabled="busy" @click="review('rejected')">拒绝</button>
      </template>
    </template>
    <template v-else>
      <button type="button" @click="editing = false">取消修改</button>
      <OrderEntryView :key="orderId" embedded fulfillment-mode :edit-id="orderId" :customer-portal="customerPortal" :customer-context-id="customerId" :portal-service="portalService" @saved="saved" />
    </template>
    <details class="confirmation-help">
      <summary>订单确认与修改说明</summary>
      <p>保存后使用原订单号进入待确认，由客户负责人或管理员确认接单。拒绝新订单后可修改重提；拒绝已接单订单的修改会恢复上次接单内容。</p>
      <p>待确认时暂停生产、库存占用和发货。修改前需解除未开工计划及库存占用，开始生产或发货后不能修改。页面可见时每 15 秒更新状态，保留正在编辑的内容。</p>
    </details>
    <details v-if="status?.confirmation_history?.length">
      <summary>订单确认记录</summary>
      <details v-for="row in status.confirmation_history" :key="row.revision">
        <summary>版本 {{ row.revision }} · {{ orderConfirmationLabel(row.status) }} · {{ row.reviewed_by || row.actor }} {{ row.reason }}</summary>
        <p>订单日期：{{ row.order_date?.slice(0, 10) }} · 金额：{{ row.grand_total }}</p>
        <p v-for="(item, index) in row.items" :key="index">{{ item.item_name }} · {{ item.spec }} · {{ item.qty }} {{ item.unit }} · 单价 {{ item.unit_price }}</p>
      </details>
    </details>
  </section>
</template>
<script setup>
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { apiGet, apiSend } from '../api/client.js'
import OrderEntryView from '../views/OrderEntryView.vue'
import { orderConfirmationLabel, visibleOrderRefresh } from '../lib/order-confirmation.js'
const props = defineProps({ orderId: { type: Number, required: true }, customerId: Number, customerPortal: Boolean, portalService: { type: String, default: 'direct_ship' } })
const emit = defineEmits(['updated'])
const status = ref(null), editing = ref(false), busy = ref(false), error = ref(''), message = ref(''), reason = ref('')
let revision = 0, stop
async function load() {
  const token = ++revision
  try { const data = await apiGet(`/api/orders/${props.orderId}/confirmation`); if (token === revision) { status.value = data; error.value = '' } }
  catch (err) { if (token === revision) error.value = err.message || '读取订单状态失败' }
}
async function review(decision) {
  if (busy.value) return
  if (decision === 'rejected' && !reason.value.trim()) { error.value = '请填写拒绝原因'; return }
  busy.value = true; error.value = ''; message.value = ''
  try {
    await apiSend(`/api/orders/${props.orderId}/confirmation`, { body: { revision: status.value.confirmation_revision, decision, reason: decision === 'rejected' ? reason.value.trim() : '' } })
    message.value = decision === 'accepted' ? '订单已接单' : '已拒绝，订单内容和状态已更新'
    reason.value = ''; await load(); emit('updated')
  } catch (err) { error.value = err.message || '确认失败，请刷新后重试' }
  finally { busy.value = false }
}
async function saved() { editing.value = false; message.value = '订单已保存，待负责人或管理员确认'; await load(); emit('updated') }
watch(() => props.orderId, () => { editing.value = false; status.value = null; reason.value = ''; message.value = ''; load() }, { immediate: true })
onMounted(() => { stop = visibleOrderRefresh(load) })
onBeforeUnmount(() => { revision++; stop?.() })
</script>
<style scoped>
.confirmation-panel { padding: 12px; border: 1px solid #d4dfd8; border-radius: 8px; margin-bottom: 14px; }
button, input { margin: 4px; max-width: 100%; }
.error { color: #b3261e; }
</style>
