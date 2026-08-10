<script setup lang="ts">
import { ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { fetchEmployeeOrders, type EmployeeOrder } from '../../api/customerPortal'
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

async function load() {
  rememberEmployeeOrderListQuery(q.value)
  loading.value = true
  error.value = ''
  try {
    const response = await fetchEmployeeOrders(session.token, q.value)
    rows.value = employeeOrderNavigationRows(response.rows)
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '订单加载失败'
  } finally {
    loading.value = false
  }
}

function rememberListQuery() {
  rememberEmployeeOrderListQuery(q.value)
}
onShow(() => void load())
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
    <view class="search"><input v-model="q" placeholder="订单号 / 客户" confirm-type="search" @confirm="load" /><button @tap="load">查询</button></view>
    <text v-if="loading" class="state">加载中...</text>
    <text v-else-if="error" class="state error">{{ error }}</text>
    <view v-else-if="rows.length" class="list">
      <template v-for="row in rows" :key="row.id || row.order_no">
        <navigator v-if="row.detail_url" class="card" hover-class="card-active" :url="row.detail_url" @tap="rememberListQuery">
          <view class="line"><text class="no">{{ row.order_no }}</text><text>¥{{ row.grand_total || '0.00' }}</text></view>
          <text class="customer">{{ row.customer }}</text>
          <text class="meta">{{ row.order_date }} · {{ row.pay_status }} · {{ row.ship_status }} · {{ row.process_status }}</text>
          <text class="open-hint">查看完整订单 ›</text>
        </navigator>
        <view v-else class="card card-disabled">
          <view class="line"><text class="no">{{ row.order_no }}</text><text>¥{{ row.grand_total || '0.00' }}</text></view>
          <text class="customer">{{ row.customer }}</text>
          <text class="meta">{{ row.order_date }} · {{ row.pay_status }} · {{ row.ship_status }} · {{ row.process_status }}</text>
          <text class="open-hint invalid-hint">订单编号异常，无法查看</text>
        </view>
      </template>
    </view>
    <text v-else class="state">没有找到订单</text>

    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter :revealed="pullUpBrandRevealed" />
    </view>
  </view>
</template>

<style scoped>
.page{min-height:100vh;padding:28rpx;background:#f5f7f6;box-sizing:border-box}.search{display:flex;gap:16rpx;margin-bottom:22rpx}.search input{flex:1;background:#fff;border:1rpx solid #dfe7e2;border-radius:12rpx;padding:18rpx}.search button{margin:0;background:#28624a;color:#fff;font-size:28rpx}.list{display:flex;flex-direction:column;gap:18rpx}.card{padding:24rpx;background:#fff;border:1rpx solid #dfe7e2;border-radius:16rpx}.card-active{background:#eef5f1}.card-disabled{background:#fafafa}.line{display:flex;justify-content:space-between}.no{font-weight:800}.customer{display:block;margin-top:12rpx}.meta{display:block;margin-top:10rpx;color:#69766f;font-size:24rpx}.open-hint{display:block;margin-top:14rpx;color:#28624a;font-size:23rpx;text-align:right}.invalid-hint{color:#b42318}.state{display:block;padding:60rpx;text-align:center;color:#69766f}.error{color:#b42318}
</style>
