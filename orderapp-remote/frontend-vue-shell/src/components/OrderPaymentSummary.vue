<script setup>
import { computed } from 'vue'
const props = defineProps({ order: { type: Object, required: true } })
const deposit = computed(() => Number(props.order.prepayment_amount || 0))
const paid = computed(() => /已付款|已收款|已支付/.test(props.order.pay_status || '') ? Number(props.order.grand_total || 0) : deposit.value)
const unpaid = computed(() => Math.max(0, Number(props.order.grand_total || 0) - paid.value))
</script>
<template>
  <div class="payment-summary">
    <span v-if="paid > 0" class="paid">{{ deposit > 0 && unpaid > 0 ? '已支付预付款' : '已支付' }} ¥{{ paid.toFixed(2) }}</span>
    <span v-if="unpaid > 0" class="unpaid">{{ deposit > 0 ? '未支付尾款' : '未支付' }} ¥{{ unpaid.toFixed(2) }}</span>
  </div>
</template>
<style scoped>
.payment-summary{display:flex;flex-wrap:wrap;gap:6px;margin-top:6px;font-size:13px}.paid,.unpaid{padding:4px 8px;border-radius:5px;font-weight:600}.paid{background:#dcfce7;color:#166534}.unpaid{background:#fee2e2;color:#b91c1c}
</style>
