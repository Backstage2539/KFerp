<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  fetchCustomerBillDetail,
  fetchCustomerBills,
  type CustomerBillDetail,
  type CustomerBillSummary,
} from '../api/customerPortal'

const props = defineProps<{ token: string }>()
const loading = ref(false)
const errorMessage = ref('')
const rows = ref<CustomerBillSummary[]>([])
const selected = ref<CustomerBillDetail | null>(null)

const billingStatusLabels: Record<string, string> = {
  confirmed: '待付款',
  paid: '已付款',
  reversed: '已冲销',
  adjusted: '已调整',
}

const billingBasisLabels: Record<string, string> = {
  actual_input_kg: '实际投入（kg）',
  actual_output_kg: '实际产出（kg）',
  actual_minutes: '实际工序时长（分钟）',
  actual_units: '实际完成件数',
  fixed_per_work_order: '每张工单固定费',
  factory_material_actual_cost: '工厂提供物料实际成本',
  manual_adjustment: '人工调整',
  reversal: '原账单冲销',
}

function billingStatusLabel(status?: string): string {
  const value = String(status || '').trim()
  return billingStatusLabels[value] || value || '待付款'
}

function billingBasisLabel(basis?: string): string {
  const value = String(basis || '').trim()
  return billingBasisLabels[value] || value || '计费依据'
}

async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    rows.value = (await fetchCustomerBills(props.token)).rows || []
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '费用账单加载失败'
  } finally {
    loading.value = false
  }
}

async function openBill(item: CustomerBillSummary) {
  try {
    selected.value = (await fetchCustomerBillDetail(props.token, item.id)).bill
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '账单明细加载失败'
  }
}

onMounted(() => { void load() })
</script>

<template>
  <view class="workspace">
    <view class="panel">
      <text class="title">费用账单</text>
      <text class="hint">只显示 ERP 已确认并推送的代加工账单。</text>
      <text v-if="loading" class="hint">加载中...</text>
      <view v-for="item in rows" :key="item.id" class="bill" @tap="openBill(item)">
        <view class="head"><text class="name">{{ item.settlement_no }}</text><text class="amount">¥{{ item.total_amount }}</text></view>
        <text class="hint">{{ billingStatusLabel(item.status) }} · 关联 {{ item.work_order_count }} 张工单</text>
        <text v-if="item.summary" class="hint">{{ item.summary }}</text>
      </view>
      <text v-if="!loading && !rows.length" class="hint">暂无已推送账单</text>
    </view>

    <view v-if="selected" class="panel">
      <view class="head"><text class="title">账单明细</text><button class="secondary compact" @tap="selected = null">收起</button></view>
      <text class="name">{{ selected.settlement_no }} · ¥{{ selected.total_amount }} · {{ billingStatusLabel(selected.status) }}</text>
      <text class="subtitle">关联工单</text>
      <view v-for="workOrder in selected.work_orders" :key="workOrder.work_order_id" class="detail">
        <text>{{ workOrder.work_order_no }} · {{ workOrder.product_name }}</text>
        <text class="hint">完工时间 {{ workOrder.completed_at || '暂无' }}</text>
      </view>
      <text class="subtitle">费用项目</text>
      <view v-for="(line, index) in selected.lines" :key="`${line.work_order_id}:${line.fee_type}:${index}`" class="detail">
        <view class="head"><text>{{ line.fee_name }}</text><text class="amount">¥{{ line.amount }}</text></view>
        <text class="hint">计费依据：{{ billingBasisLabel(line.basis) }}</text>
        <text class="hint">计费数量：{{ line.base_quantity }} · 单价：¥{{ line.unit_price }}</text>
      </view>
    </view>
    <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
  </view>
</template>

<style scoped>
.workspace,.panel,.bill,.detail{display:flex;flex-direction:column;gap:14rpx}.panel{padding:24rpx;margin-bottom:20rpx;border:1rpx solid #e6e0d8;border-radius:8rpx;background:#fff}.title{font-size:30rpx;font-weight:900}.subtitle,.name{font-size:27rpx;font-weight:800}.hint{color:#707070;font-size:23rpx;line-height:1.5}.bill,.detail{padding:16rpx;border:1rpx solid #eee;border-radius:8rpx}.head{display:flex;align-items:center;justify-content:space-between;gap:14rpx}.amount{font-weight:900;color:#5d3b17}.secondary{min-height:60rpx;margin:0;padding:0 18rpx;border:1rpx solid #ddd;border-radius:8rpx;background:#fff}.compact{font-size:22rpx}.error{display:block;padding:18rpx;color:#b42318}
</style>
