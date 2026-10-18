<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { onHide } from '@dcloudio/uni-app'
import { useSessionStore } from '../stores/session'
import { fetchOrderDetailsText } from '../api/orderDetailsCopy'
import { createOrderDetailsCopyAction, writeOrderDetailsClipboard } from '../utils/orderDetailsCopy'
const props = defineProps<{ orderId: number; employee?: boolean }>()
const session = useSessionStore()
const busy = ref(false)
let alive = true
const context = () => JSON.stringify([alive, session.token, session.currentCustomerID, session.accountType, session.employeeID, props.orderId, props.employee])
const action = createOrderDetailsCopyAction({ context, fetchDetails: id => fetchOrderDetailsText(session.token, id, props.employee), writeText: writeOrderDetailsClipboard })
function reset() { action.invalidate(); busy.value = false }
watch(context, reset, { flush: 'sync' })
onHide(reset)
onBeforeUnmount(() => { alive = false; reset() })
async function copy() {
  if (busy.value) return
  busy.value = true
  const scope = context()
  const result = await action.copy(props.orderId)
  busy.value = action.busy
  if (!alive || scope !== context()) return
  if (result.status === 'copied') uni.showToast({ title: '订单详情已复制', icon: 'success' })
  if (result.status === 'error') uni.showToast({ title: result.message || '复制失败，请重试', icon: 'none' })
}
</script>
<template>
  <button class="order-details-copy" :loading="busy" :disabled="busy || !orderId" @tap.stop="copy">{{ busy ? '正在复制…' : '复制订单详情' }}</button>
</template>
<style scoped>
.order-details-copy{display:inline-block;margin:12rpx 0;padding:10rpx 18rpx;background:#f0f7f3;color:#28624a;border-radius:10rpx;font-size:26rpx;line-height:1.6;vertical-align:middle}.order-details-copy::after{border:1rpx solid #d0e0d6}.order-details-copy[disabled]{opacity:.55}
</style>
