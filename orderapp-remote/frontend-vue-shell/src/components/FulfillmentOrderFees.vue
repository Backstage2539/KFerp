<template>
  <div class="fulfillment-fees" aria-label="代发费用明细">
    <div><span>商品货款</span><strong>¥{{ money(summary.goods) }}</strong><small v-if="mode === 'stock'">原单已购库存，本次不重复收取货款</small></div>
    <div><span>代发服务费</span><strong>¥{{ money(summary.service) }}</strong><small v-if="summary.serviceDue != null">{{ summary.serviceDue > 0 ? `待收 ¥${money(summary.serviceDue)}` : '服务费已结清' }} · 已结 ¥{{ money(Math.max(0, summary.service - summary.serviceDue)) }}</small></div>
    <div><span>预估运费</span><strong>{{ summary.estimated == null ? '待核算' : `¥${money(summary.estimated)}` }}</strong><small>实际运费按快递账单独立出账</small></div>
    <div><span>本次运费应收</span><strong>¥{{ money(summary.shipping) }}</strong></div>
    <div><span>实际运费</span><strong>{{ summary.actual == null ? '待快递账单' : `¥${money(summary.actual)}` }}</strong><small v-if="summary.billed != null">已出账 ¥{{ money(summary.billed) }}</small></div>
    <div><span>本单费用合计</span><strong>¥{{ money(summary.total) }}</strong></div>
    <p v-if="fees.shipping_template_revision" class="wide">沿用原单运费模板 V{{ fees.shipping_template_revision }}<template v-if="fees.chargeable_weight_g"> · 计费重 {{ fees.chargeable_weight_g }} 克</template><template v-if="fees.packaging_weight_g != null"> · 每票包装 {{ fees.packaging_weight_g }} 克</template></p>
  </div>
</template>
<script setup>
import { computed } from 'vue'
import { fulfillmentFeeSummary } from '../lib/fulfillment-order-edit'
const props = defineProps({ fees: { type: Object, default: () => ({}) }, freightView: { type: Object, default: () => ({}) }, goodsCents: { type: Number, default: 0 }, mode: { type: String, default: '' } })
const summary = computed(() => fulfillmentFeeSummary(props.fees, props.goodsCents, props.freightView))
const money = cents => (Number(cents || 0) / 100).toFixed(2)
</script>
<style scoped>
.fulfillment-fees { display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;padding:12px;background:#f1f7f3;border:1px solid #d5e5da;border-radius:8px;width:100%;box-sizing:border-box; }
.fulfillment-fees>div { display:grid;gap:5px;min-width:0; }.fulfillment-fees span,.fulfillment-fees small,.fulfillment-fees p { color:#5e7065;font-size:12px;line-height:1.5; }.fulfillment-fees strong { color:#245b42;font-size:16px; }.wide { grid-column:1/-1;margin:0; }
@media(max-width:640px){.fulfillment-fees{grid-template-columns:repeat(2,minmax(0,1fr));}}
</style>
