<script setup lang="ts">
import { ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { fetchEmployeeOrders, type EmployeeOrder } from '../../api/customerPortal'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import { useSessionStore } from '../../stores/session'

const session = useSessionStore()
const q = ref('')
const rows = ref<EmployeeOrder[]>([])
const loading = ref(false)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    rows.value = (await fetchEmployeeOrders(session.token, q.value)).rows || []
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '订单加载失败'
  } finally {
    loading.value = false
  }
}
onShow(() => void load())
</script>

<template>
  <view class="page">
    <EnvironmentBadge />
    <view class="search"><input v-model="q" placeholder="订单号 / 客户" confirm-type="search" @confirm="load" /><button @tap="load">查询</button></view>
    <text v-if="loading" class="state">加载中...</text>
    <text v-else-if="error" class="state error">{{ error }}</text>
    <view v-else-if="rows.length" class="list">
      <view v-for="row in rows" :key="row.id" class="card">
        <view class="line"><text class="no">{{ row.order_no }}</text><text>¥{{ row.grand_total || '0.00' }}</text></view>
        <text class="customer">{{ row.customer }}</text>
        <text class="meta">{{ row.order_date }} · {{ row.pay_status }} · {{ row.ship_status }} · {{ row.process_status }}</text>
      </view>
    </view>
    <text v-else class="state">没有找到订单</text>
  </view>
</template>

<style scoped>
.page{min-height:100vh;padding:28rpx;background:#f5f7f6;box-sizing:border-box}.search{display:flex;gap:16rpx;margin-bottom:22rpx}.search input{flex:1;background:#fff;border:1rpx solid #dfe7e2;border-radius:12rpx;padding:18rpx}.search button{margin:0;background:#28624a;color:#fff;font-size:28rpx}.list{display:flex;flex-direction:column;gap:18rpx}.card{padding:24rpx;background:#fff;border:1rpx solid #dfe7e2;border-radius:16rpx}.line{display:flex;justify-content:space-between}.no{font-weight:800}.customer{display:block;margin-top:12rpx}.meta{display:block;margin-top:10rpx;color:#69766f;font-size:24rpx}.state{display:block;padding:60rpx;text-align:center;color:#69766f}.error{color:#b42318}
</style>
