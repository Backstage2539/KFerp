<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  fetchCustomerInventory,
  fetchCustomerInventoryBatches,
  type CustomerInventoryBatch,
  type CustomerInventorySummary,
} from '../api/customerPortal'

const props = defineProps<{ token: string }>()
const loading = ref(false)
const errorMessage = ref('')
const rows = ref<CustomerInventorySummary[]>([])
const selected = ref<CustomerInventorySummary | null>(null)
const batches = ref<CustomerInventoryBatch[]>([])

async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    rows.value = (await fetchCustomerInventory(props.token)).rows || []
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '客户库存加载失败'
  } finally {
    loading.value = false
  }
}

async function openItem(item: CustomerInventorySummary) {
  selected.value = item
  batches.value = []
  try {
    batches.value = (await fetchCustomerInventoryBatches(props.token, item.product_id, item.spec_g)).rows || []
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '库存批次加载失败'
  }
}

function submitProduction(item: CustomerInventorySummary) {
  uni.navigateTo({ url: `/pages/service/service?key=processing&product_id=${item.product_id}&spec_g=${item.spec_g}` })
}

function productionDate(item: CustomerInventoryBatch): string {
  return item.production_date || '历史库存，暂无生产日期'
}

onMounted(() => { void load() })
</script>

<template>
  <view class="workspace">
    <view class="panel">
      <text class="title">我的库存</text>
      <text class="hint">仅展示绑定到当前客户的成品仓库存，不显示成本、供应商或操作人。</text>
      <text v-if="loading" class="hint">加载中...</text>
      <view v-for="item in rows" :key="`${item.product_id}:${item.spec_g}`" class="inventory" @tap="openItem(item)">
        <view class="head"><text class="name">{{ item.product_name }}</text><text class="available">可用 {{ item.available_qty }}</text></view>
        <text class="hint">{{ item.sku_code || `SKU ${item.product_id}` }} · 规格 {{ item.spec_g }}g · 预留 {{ item.reserved_qty }}</text>
        <text class="hint">{{ item.warehouses.join('、') }}</text>
      </view>
      <text v-if="!loading && !rows.length" class="hint">当前客户绑定成品仓暂无库存</text>
    </view>

    <view v-if="selected" class="panel">
      <view class="head"><text class="title">库存详情</text><button class="secondary compact" @tap="selected = null">收起</button></view>
      <text class="name">{{ selected.product_name }} · {{ selected.spec_g }}g</text>
      <view v-for="batch in batches" :key="`${batch.batch_id}:${batch.warehouse}`" class="batch">
        <text>批次 {{ batch.batch_no || batch.batch_id }}</text>
        <text class="hint">生产日期：{{ productionDate(batch) }}</text>
        <text class="hint">所在仓库：{{ batch.warehouse }}</text>
        <text class="hint">入库时间：{{ batch.inbound_at || '暂无记录' }}</text>
        <text class="hint">可用 {{ batch.available_qty }} · 已预留 {{ batch.reserved_qty }} · 质量状态 {{ batch.quality_status || '正常' }}</text>
      </view>
      <text v-if="!batches.length" class="hint">暂无可追溯批次</text>
      <button class="primary" @tap="submitProduction(selected)">提交生产工单</button>
    </view>
    <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
  </view>
</template>

<style scoped>
.workspace,.panel,.inventory,.batch{display:flex;flex-direction:column;gap:14rpx}.panel{padding:24rpx;margin-bottom:20rpx;border:1rpx solid #e6e0d8;border-radius:8rpx;background:#fff}.title{font-size:30rpx;font-weight:900}.name{font-size:27rpx;font-weight:800}.hint{color:#707070;font-size:23rpx;line-height:1.5}.inventory,.batch{padding:16rpx;border:1rpx solid #eee;border-radius:8rpx}.head{display:flex;align-items:center;justify-content:space-between;gap:14rpx}.available{color:#28624a;font-weight:800}.primary,.secondary{min-height:76rpx;margin:0;border-radius:8rpx}.primary{background:#2b2118;color:#fff}.secondary{background:#fff;border:1rpx solid #ddd}.compact{min-height:60rpx;padding:0 18rpx;font-size:22rpx}.error{display:block;padding:18rpx;color:#b42318}
</style>
