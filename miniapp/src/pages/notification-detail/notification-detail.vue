<script setup lang="ts">
import { ref } from 'vue'
import { onHide, onLoad, onShow, onShareAppMessage, onShareTimeline } from '@dcloudio/uni-app'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { defaultMiniappShare, defaultMiniappTimelineShare, refreshMiniappShareMenu } from '../../utils/miniappShare'
import { isAuthenticationExpiredRequestError, miniRequest } from '../../api/client'
import { useSessionStore } from '../../stores/session'
import { loginRouteFor } from '../../utils/loginReturn'
import { formatNoticeCents, notificationDetailAPI, notificationDetailRoute, parseNotificationDetail, type NotificationDetailTarget } from '../../utils/notificationDetail'

type Receipt = { receipt_no: string, receipt_date: string, actual_cents: number, receivable_cents: number }
type Bill = { bill_no: string, period_from: string, period_to: string, amount_cents: number }
const session = useSessionStore()
const { pullUpBrandRevealed, handlePullUpBrandTouchStart, handlePullUpBrandTouchMove, handlePullUpBrandTouchEnd, handlePullUpBrandTouchCancel } = usePullUpBrandGesture()
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
const target = ref<NotificationDetailTarget | null>(null)
const receipt = ref<Receipt | null>(null), bill = ref<Bill | null>(null)
const loading = ref(false), error = ref('')
let generation = 0

function login() {
  if (!target.value) return
  uni.reLaunch({ url: loginRouteFor(notificationDetailRoute(target.value.type, target.value.id)) })
}
async function load() {
  const current = target.value
  const run = ++generation
  receipt.value = null; bill.value = null; error.value = ''
  if (!current) { error.value = '通知链接无效'; return }
  if (!session.token) { login(); return }
  loading.value = true
  try {
    const result = await miniRequest<Receipt | Bill>(notificationDetailAPI(current.type, current.id), { token: session.token })
    if (run !== generation) return
    if (current.type === 'receipt') receipt.value = result as Receipt
    else bill.value = result as Bill
  } catch (cause) {
    if (run !== generation) return
    if (isAuthenticationExpiredRequestError(cause)) { session.clearSession(); login(); return }
    error.value = cause instanceof Error ? cause.message : '无法读取通知详情'
  } finally { if (run === generation) loading.value = false }
}
onLoad((query) => {
  const keys = Object.keys(query || {})
  target.value = keys.every(key => key === 'type' || key === 'id') ? parseNotificationDetail(query?.type, query?.id) : null
})
onShow(() => { void refreshMiniappShareMenu(); void load() })
onHide(() => { generation++; receipt.value = null; bill.value = null; loading.value = false })
</script>

<template>
  <view class="page pull-up-brand-page" @touchstart="handlePullUpBrandTouchStart" @touchmove="handlePullUpBrandTouchMove" @touchend="handlePullUpBrandTouchEnd" @touchcancel="handlePullUpBrandTouchCancel">
    <EnvironmentBadge />
    <text class="title">{{ target?.type === 'receipt' ? '收款详情' : '账单详情' }}</text>
    <text v-if="loading">正在读取详情…</text>
    <view v-else-if="error" class="error">{{ error }}<button v-if="target" @tap="load">重试</button></view>
    <view v-else-if="receipt" class="card">
      <text>收款单号：{{ receipt.receipt_no }}</text>
      <text>实际到账日期：{{ receipt.receipt_date || '未记录' }}</text>
      <text>关联应收总额：¥{{ formatNoticeCents(receipt.receivable_cents) }}</text>
      <text>本次到账：¥{{ formatNoticeCents(receipt.actual_cents) }}</text>
    </view>
    <view v-else-if="bill" class="card">
      <text>账单编号：{{ bill.bill_no }}</text>
      <text>账期：{{ bill.period_from }} 至 {{ bill.period_to }}</text>
      <text>出账金额：¥{{ formatNoticeCents(bill.amount_cents) }}</text>
    </view>
    <view class="pull-up-brand-footer-anchor"><PullUpBrandFooter :revealed="pullUpBrandRevealed" /></view>
  </view>
</template>

<style scoped>
.page{display:flex;flex-direction:column;gap:24rpx;padding:32rpx;min-height:100vh;background:#f6f7f8;box-sizing:border-box}.title{font-size:38rpx;font-weight:600}.card{display:flex;flex-direction:column;gap:20rpx;background:#fff;border-radius:18rpx;padding:28rpx}.error{color:#b42323}
</style>
