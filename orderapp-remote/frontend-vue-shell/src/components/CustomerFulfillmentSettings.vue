<template>
  <details class="fulfillment-settings" @toggle="onToggle">
    <summary>一件代发履约设置</summary>
    <p>分别启用库存代发、无库存代发。设置保存在此客户下，应用门户模板后继续保留。</p>
    <div v-if="error" class="error" role="alert">{{ error }}</div>
    <div v-if="message" class="ok" role="status">{{ message }}</div>
    <span v-if="loading">正在加载履约设置…</span>
    <template v-else-if="loaded">
      <CustomerShippingPolicies :customer-id="props.customerId" @saved="load" />
      <fieldset v-for="mode in form.modes" :key="mode.mode" :disabled="saving">
        <legend><label class="toggle"><input v-model="mode.enabled" type="checkbox" /> {{ mode.mode === 'stock' ? '库存代发' : '无库存代发' }}</label></legend>
        <template v-if="mode.enabled">
          <label v-if="mode.mode === 'stock'">客户成品仓
            <select v-model="mode.warehouse_code"><option value="">请选择此客户的成品仓</option><option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name || warehouse.code }}</option></select>
          </label>
          <p v-if="mode.mode === 'stock' && !warehouses.length" class="hint">尚无可用客户成品仓，请先在上方开通成品仓。</p>
          <div class="fees-grid">
            <label>每单固定服务费（元）<input v-model="mode.fixed_fee" type="number" min="0" step="0.01" placeholder="免费填写 0" /></label>
            <label>每包装件服务费（元）<input v-model="mode.per_package_fee" type="number" min="0" step="0.01" placeholder="不用填写 0" /></label>
          </div>

          <p class="hint">服务费＝每单固定费＋每包装件费×包装件数。运费按商品净重×数量＋模板包装附加重计算。</p>
        </template>
      </fieldset>
      <p class="hint">无库存代发沿用“商品 → 商品价格表”中该类别已指定的一件代发价格表。</p>
      <div class="actions"><button type="button" @click="save" :disabled="saving">{{ saving ? '保存中…' : '保存履约设置' }}</button><button type="button" class="secondary" @click="load" :disabled="saving">重新载入</button></div>
    </template>
  </details>
</template>
<script setup>
import { ref, watch } from 'vue'
import CustomerShippingPolicies from './CustomerShippingPolicies.vue'
import { apiGet, apiSend } from '../api/client'
import { fulfillmentSettingsForm, fulfillmentSettingsPayload, customerFinishedWarehouses } from '../lib/fulfillment-settings'
const props = defineProps({ customerId: { type: Number, required: true } })
const form = ref(fulfillmentSettingsForm()), warehouses = ref([]), templates = ref([]), error = ref(''), message = ref(''), loading = ref(false), saving = ref(false), loaded = ref(false)
let sequence = 0
watch(() => props.customerId, () => { sequence++; loaded.value = false; error.value = ''; message.value = '' })
function onToggle(event) { if (event.target.open && !loaded.value) load() }
async function load() {
  const request = ++sequence, customerID = props.customerId
  loading.value = true; error.value = ''; message.value = ''
  try {
    const results = await Promise.all([apiGet(`/api/customer-fulfillment/${customerID}/settings`), apiGet(`/api/stock/warehouses?customer_id=${customerID}`)])
    if (request !== sequence) return
    form.value = fulfillmentSettingsForm(results[0]); warehouses.value = customerFinishedWarehouses(results[1].rows, customerID); templates.value = (results[0].shipping_templates || []).filter(r => r.active); loaded.value = true
  } catch (err) { if (request === sequence) error.value = err.message || '履约设置加载失败' }
  finally { if (request === sequence) loading.value = false }
}
async function save() {
  saving.value = true; error.value = ''; message.value = ''
  try { const data = await apiSend(`/api/customer-fulfillment/${props.customerId}/settings`, { method: 'PUT', body: fulfillmentSettingsPayload(form.value) }); form.value = fulfillmentSettingsForm(data); message.value = '此客户履约设置已保存' }
  catch (err) { error.value = err.message || '履约设置保存失败' }
  finally { saving.value = false }
}
</script>
<style scoped>
.fulfillment-settings{margin-top:16px;padding-top:12px;border-top:1px solid #dce5df;min-width:0}summary{cursor:pointer;font-weight:700;color:#245c41}p{font-size:12px;line-height:1.65;color:#627269}fieldset{margin:12px 0;padding:12px;border:1px solid #dce5df;border-radius:8px;min-width:0}label{display:grid;gap:6px;font-size:12px;color:#42584b;margin-bottom:10px}.toggle{display:flex;align-items:center;gap:8px;margin:0}.toggle input{width:16px;height:16px}input,select{width:100%;box-sizing:border-box;padding:8px;border:1px solid #cbd8cf;border-radius:6px;min-width:0}.fees-grid{display:grid;grid-template-columns:1fr 1fr;gap:8px}.actions{display:flex;gap:8px;flex-wrap:wrap}button{border:1px solid #28624a;border-radius:6px;padding:8px 12px;background:#28624a;color:white;cursor:pointer}button:disabled{opacity:.5}.secondary{background:white;color:#28624a}.error,.ok{padding:8px;font-size:12px;border-radius:6px;margin:8px 0}.error{background:#fff1ed;color:#ab2b20}.ok{background:#eef8ef;color:#28624a}@media(max-width:600px){.fees-grid{grid-template-columns:1fr}}
</style>
