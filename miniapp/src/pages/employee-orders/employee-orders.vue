<script setup lang="ts">
import {orderProgressLabel} from '../../utils/orderProgress'
import OrderDetailsCopyButton from '../../components/OrderDetailsCopyButton.vue'
import PaymentSummary from '../../components/PaymentSummary.vue'
import EmployeeOrderListStatus from '../../components/EmployeeOrderListStatus.vue'
import { ref } from 'vue'
import { MiniRequestError } from '../../api/client'
import { onShow, onReachBottom, onShareAppMessage, onShareTimeline } from '@dcloudio/uni-app'
import {
  defaultMiniappShare,
  defaultMiniappTimelineShare,
  refreshMiniappShareMenu,
} from '../../utils/miniappShare'
import { fetchEmployeeOrderPage, fetchEmployeeOrderDetail, type EmployeeOrder } from '../../api/customerPortal'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { useSessionStore } from '../../stores/session'
import {
  employeeOrderListQuery,
  employeeOrderNavigationRows,
  rememberEmployeeOrderListQuery,
  type EmployeeOrderNavigationRow,
} from '../../utils/employeeOrderDetail'

import { copyOrderURL } from '../../utils/prepayment'
function copyOrder(id: number) {
  const url = copyOrderURL(id)
  if (url) { rememberListQuery(); uni.navigateTo({ url }) }
}

const session = useSessionStore()
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
const q = ref(employeeOrderListQuery())
const rows = ref<EmployeeOrderNavigationRow<EmployeeOrder>[]>([])
const loading = ref(false)
const error = ref('')

const nextCursor = ref('')
const hasNext = ref(false)
let loaded = false
let activeQuery = ''
let retryAppend = false
let sequence = 0
let returningOrderID = 0
let loadedToken = ''

async function load(append = false) {
  if (append && (loading.value || !hasNext.value || q.value.trim() !== activeQuery)) return
  const requestSequence = ++sequence
  const query = q.value.trim()
  rememberEmployeeOrderListQuery(query)
  loading.value = true
  error.value = ''
  try {
    const response = await fetchEmployeeOrderPage(session.token, query, append ? nextCursor.value : '')
    if (requestSequence !== sequence) return
    const incoming = employeeOrderNavigationRows(response.rows)
    const existingIDs = new Set(append ? rows.value.map(row => row.id) : [])
    rows.value = append ? [...rows.value, ...incoming.filter(row => !existingIDs.has(row.id))] : incoming
    nextCursor.value = response.next_cursor || ''
    hasNext.value = response.has_next
    activeQuery = query
    loadedToken = session.token
    loaded = true
  } catch (cause) {
    if (requestSequence === sequence) {
      retryAppend = append
      error.value = cause instanceof Error ? cause.message : '订单加载失败'
    }
  } finally {
    if (requestSequence === sequence) loading.value = false
  }
}

async function refreshRow(id: number) {
  try {
    const response = await fetchEmployeeOrderDetail(session.token, id)
    const order = response.order
    if (order.is_void) {
      rows.value = rows.value.filter(row => row.id !== id)
      return
    }
    rows.value = rows.value.map(row => row.id === id ? {
      ...row,
      pay_status: order.pay_status,
      ship_status: order.ship_status,
      process_status: order.process_status,
      fulfillment_progress: order.fulfillment_progress,
      prepayment_amount: order.prepayment_amount,
      paid_amount: order.paid_amount,
      unpaid_amount: order.unpaid_amount,
      delivery_status: order.delivery_status,
    } : row)
  } catch (cause) {
    if (cause instanceof MiniRequestError && [403, 404].includes(cause.statusCode)) {
      rows.value = rows.value.filter(row => row.id !== id)
      return
    }
    uni.showToast({ title: '订单刷新失败，请重新查询', icon: 'none' })
  }
}

function rememberListQuery(id = 0) {
  rememberEmployeeOrderListQuery(q.value)
  returningOrderID = id
}
onShow(() => {
  if (loadedToken && loadedToken !== session.token) {
    rows.value = []
    loaded = false
    hasNext.value = false
    returningOrderID = 0
  }
  if (!loaded) void load()
  else if (returningOrderID) {
    const id = returningOrderID
    returningOrderID = 0
    void refreshRow(id)
  }
})
onReachBottom(() => void load(true))

onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
onShow(() => { void refreshMiniappShareMenu() })
</script>

<template>
  <view
    class="page pull-up-brand-page"
    @touchstart="handlePullUpBrandTouchStart"
    @touchmove="handlePullUpBrandTouchMove"
    @touchend="handlePullUpBrandTouchEnd"
    @touchcancel="handlePullUpBrandTouchCancel"
  >
    <EnvironmentBadge />
    <view class="search"><input v-model="q" placeholder="订单号 / 客户" confirm-type="search" @confirm="load(false)" /><button @tap="load(false)">查询</button></view>
    <text v-if="loading && !rows.length" class="state">加载中...</text>
    <text v-else-if="error && !rows.length" class="state error" @tap="load(false)">{{ error }}，点击重试</text>
    <view v-else-if="rows.length" class="list">
      <template v-for="row in rows" :key="row.id || row.order_no">
        <view v-if="row.detail_url" class="card">
        <navigator hover-class="card-active" :url="row.detail_url" @tap="rememberListQuery(row.id)">
          <view class="line"><text class="no">{{ row.order_no }}</text><text>¥{{ row.grand_total || '0.00' }}</text></view>
          <text class="customer">{{ row.customer }}</text>
          <text class="meta">{{ row.order_date }} · {{ orderProgressLabel(row) }}</text>
          <text class="open-hint">查看完整订单 ›</text>
        </navigator>
        <EmployeeOrderListStatus :order="row" @changed="refreshRow" />
        <PaymentSummary :order="row" />
        <OrderDetailsCopyButton :order-id="row.id" employee />
        <button class="copy-order" @tap.stop="copyOrder(row.id)">复制订单</button>
        </view>
        <view v-else class="card card-disabled">
          <view class="line"><text class="no">{{ row.order_no }}</text><text>¥{{ row.grand_total || '0.00' }}</text></view>
          <text class="customer">{{ row.customer }}</text>
          <text class="meta">{{ row.order_date }} · {{ orderProgressLabel(row) }}</text>
          <text class="open-hint invalid-hint">订单编号异常，无法查看</text>
        </view>
      </template>
    </view>
    <text v-else-if="!loading" class="state">没有找到订单</text>
    <text v-if="rows.length && loading" class="state">加载中...</text>
    <text v-if="rows.length && error" class="state error" @tap="load(retryAppend)">{{ error }}，点击重试</text>

    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter :revealed="pullUpBrandRevealed" />
    </view>
  </view>
</template>

<style scoped>
.copy-order{margin:16rpx 0 0 auto;padding:8rpx 0;width:auto;line-height:1.5;background:transparent;color:#2563eb;font-size:26rpx;text-align:right}.copy-order::after{border:0}
.page{min-height:100vh;padding:28rpx;background:#f5f7f6;box-sizing:border-box}.search{display:flex;gap:16rpx;margin-bottom:22rpx}.search input{flex:1;background:#fff;border:1rpx solid #dfe7e2;border-radius:12rpx;padding:18rpx}.search button{margin:0;background:#28624a;color:#fff;font-size:28rpx}.list{display:flex;flex-direction:column;gap:18rpx}.card{padding:24rpx;background:#fff;border:1rpx solid #dfe7e2;border-radius:16rpx}.card-active{background:#eef5f1}.card-disabled{background:#fafafa}.line{display:flex;justify-content:space-between}.no{font-weight:800}.customer{display:block;margin-top:12rpx}.meta{display:block;margin-top:10rpx;color:#69766f;font-size:24rpx}.open-hint{display:block;margin-top:14rpx;color:#28624a;font-size:23rpx;text-align:right}.invalid-hint{color:#b42318}.state{display:block;padding:60rpx;text-align:center;color:#69766f}.error{color:#b42318}
</style>
