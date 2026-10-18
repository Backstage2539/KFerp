<template>
  <details class="fee-adjustment" @toggle="onToggle"><summary>代发费用明细及调整</summary>
    <p v-if="loading">正在读取费用…</p><p v-if="error" class="error" role="alert">{{ error }}</p><p v-if="message" class="ok" role="status">{{ message }}</p>
    <form v-if="quote" @submit.prevent="save"><p>{{ quote.fulfillment_mode === 'stock' ? '库存代发' : '无库存代发' }} · {{ quote.package_qty }} 个包装件 · 计费重 {{ quote.chargeable_weight_g }} 克</p><p>系统试算：服务费 ¥{{ money(quote.calculated_service_fee_cents) }}，运费 ¥{{ money(quote.calculated_shipping_fee_cents) }}；运费模板 V{{ quote.shipping_template_revision }}</p><div class="fields"><label>服务费（元）<input v-model="service" type="number" min="0" step="0.01" required /></label><label>运费（元）<input v-model="shipping" type="number" min="0" step="0.01" required /></label></div><label>调整原因<textarea v-model.trim="reason" placeholder="填写本次费用调整原因" required /></label><button type="submit" :disabled="saving || !reason">{{ saving ? '保存中…' : '保存费用调整' }}</button></form>
  </details>
</template>
<script setup>
import { ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { explicitMoneyCents } from '../lib/fulfillment-settings'
const props = defineProps({ customerId: { type: Number, required: true }, orderId: { type: Number, required: true } })
const emit = defineEmits(['saved'])
const quote = ref(null), service = ref(''), shipping = ref(''), reason = ref(''), loading = ref(false), saving = ref(false), error = ref(''), message = ref('')
let sequence = 0
const money = cents => (Number(cents || 0) / 100).toFixed(2)
watch(() => [props.customerId, props.orderId], () => { sequence++; quote.value = null; error.value = ''; message.value = '' })
function assign(data) { quote.value = data; service.value = money(data.service_fee_cents); shipping.value = money(data.shipping_fee_cents); reason.value = '' }
function onToggle(event) { if (event.target.open && !quote.value) load() }
const url = () => `/api/customer-fulfillment/${props.customerId}/orders/${props.orderId}/fees`
async function load() { const request = ++sequence; loading.value = true; error.value = ''; try { const data = await apiGet(url()); if (request === sequence) assign(data) } catch (err) { if (request === sequence) error.value = err.message || '费用读取失败' } finally { if (request === sequence) loading.value = false } }
async function save() { saving.value = true; error.value = ''; message.value = ''; try { const data = await apiSend(url(), { method: 'PUT', body: { expected_quote_token: quote.value.fee_quote_token, service_fee_cents: explicitMoneyCents(service.value), shipping_fee_cents: explicitMoneyCents(shipping.value), reason: reason.value } }); assign(data); message.value = '费用调整已保存并记录操作日志'; emit('saved') } catch (err) { error.value = err.message || '费用调整失败' } finally { saving.value = false } }
</script>
<style scoped>
.fee-adjustment{padding:10px 0;font-size:12px;min-width:260px}summary{cursor:pointer;color:#28624a;font-weight:700}p{color:#66796c;line-height:1.6}.fields{display:grid;grid-template-columns:1fr 1fr;gap:12px}label{display:grid;gap:6px;margin:10px 0}input,textarea{box-sizing:border-box;width:100%;border:1px solid #ccd9cf;border-radius:6px;padding:8px}textarea{min-height:70px}button{padding:8px 12px;background:#28624a;color:#fff;border:0;border-radius:6px;cursor:pointer}button:disabled{opacity:.5}.error{color:#ae3024}.ok{color:#28624a}
</style>
