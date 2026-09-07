<script setup lang="ts">
import { computed } from 'vue'
import { paymentSummary } from '../utils/prepayment'
const props = defineProps<{ order: { pay_status?: string; prepayment_amount?: string; grand_total?: string; paid_amount?: string; unpaid_amount?: string } }>()
const amounts = computed(() => paymentSummary(props.order.pay_status || '', Number(props.order.prepayment_amount || 0), Number(props.order.grand_total || 0)))
const deposit = computed(() => Number(props.order.prepayment_amount || 0) > 0)
</script>
<template>
  <view class="payment-summary">
    <text v-if="Number(amounts.paid) > 0" class="paid">{{ deposit && Number(amounts.unpaid) > 0 ? '已支付预付款' : '已支付' }} ¥{{ amounts.paid }}</text>
    <text v-if="Number(amounts.unpaid) > 0" class="unpaid">{{ deposit ? '未支付尾款' : '未支付' }} ¥{{ amounts.unpaid }}</text>
  </view>
</template>
<style scoped>
.payment-summary{display:flex;flex-wrap:wrap;gap:12rpx;margin-top:14rpx}.paid,.unpaid{padding:10rpx 14rpx;border-radius:8rpx;font-size:26rpx;font-weight:600}.paid{background:#dcfce7;color:#166534}.unpaid{background:#fee2e2;color:#b91c1c}
</style>
