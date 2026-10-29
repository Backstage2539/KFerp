<script setup lang="ts">
import {freightLabel} from '../utils/freight'
import type {FreightSnapshot} from '../api/customerPortal'
import { computed } from 'vue'
import { customerOrderAmounts, type CustomerFeeSummary } from '../utils/customerOrderFees'
const props = defineProps<{ order: { fee_summary?:CustomerFeeSummary; freight?:FreightSnapshot; pay_status?: string; prepayment_amount?: string; grand_total?: string; paid_amount?: string; unpaid_amount?: string } }>()
const amounts = computed(() => customerOrderAmounts(props.order))
const deposit = computed(() => Number(props.order.prepayment_amount || 0) > 0)
</script>
<template>
  <view class="payment-summary">
 <text v-if="!order.fee_summary && Number(order.freight?.service_due_cents||0)>0" class="unpaid">代发服务费待支付 ¥{{(Number(order.freight?.service_due_cents||0)/100).toFixed(2)}}</text>
 <text v-if="order.freight" class="freight">{{freightLabel(order.freight)}}</text>
    <text v-if="!order.fee_summary && order.pay_status === '无需收款'" class="paid">无需收款</text>
    <text v-if="Number(amounts.paid) > 0" class="paid">{{ deposit && Number(amounts.unpaid) > 0 ? '已支付预付款' : '已支付' }} ¥{{ amounts.paid }}</text>
    <text v-if="Number(amounts.unpaid) > 0" class="unpaid">{{ deposit ? '未支付尾款' : '未支付' }} ¥{{ amounts.unpaid }}</text>
  </view>
</template>
<style scoped>
.freight{color:#62766b;font-size:25rpx;width:100%;line-height:1.6}.payment-summary{display:flex;flex-wrap:wrap;gap:12rpx;margin-top:14rpx}.paid,.unpaid{padding:10rpx 14rpx;border-radius:8rpx;font-size:26rpx;font-weight:600}.paid{background:#dcfce7;color:#166534}.unpaid{background:#fee2e2;color:#b91c1c}
</style>
