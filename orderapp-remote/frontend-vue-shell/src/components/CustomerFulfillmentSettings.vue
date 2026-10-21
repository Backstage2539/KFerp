<template>
  <details class="fulfillment-settings" @toggle="onToggle">
    <summary>一件代发履约设置</summary>
    <p>开启需要的代发场景后，在对应区域配置运费和服务费。一次保存全部履约设置。</p>
    <div v-if="error" class="error" role="alert">{{ error }}</div>
    <div v-if="message" class="ok" role="status">{{ message }}</div>
    <span v-if="loading">正在加载履约设置…</span>
    <template v-else-if="loaded">
      <fieldset v-for="mode in form.modes" :key="mode.mode" :disabled="saving">
        <legend><label class="toggle"><input v-model="mode.enabled" type="checkbox" /> {{ mode.mode === 'stock' ? '库存代发' : '无库存代发' }}</label></legend>
        <template v-if="mode.enabled">
          <div v-if="mode.mode === 'stock'" class="warehouse-picker">
            <label>客户成品仓
              <select v-model="mode.warehouse_code"><option value="">请选择此客户的成品仓</option><option v-if="mode.warehouse_code && !warehouses.some(row => row.code === mode.warehouse_code)" :value="mode.warehouse_code" disabled>原仓库不可用，请重新选择</option><option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name || warehouse.code }}</option></select>
            </label>
            <button type="button" class="secondary" @click="warehouseDrawerOpen = true">打开仓库</button>
          </div>
          <p v-if="mode.mode === 'stock' && !warehouses.length" class="hint">尚无可用客户成品仓，可点击“打开仓库”新增或绑定虚拟子仓。</p>
          <div class="fees-grid">
            <label>默认运费模板<select v-model.number="mode.shipping_template_id"><option :value="0">请选择运费模板</option><option v-if="mode.shipping_template_id && !templates.some(row => Number(row.id) === Number(mode.shipping_template_id))" :value="mode.shipping_template_id" disabled>原模板不可用，请重新选择</option><option v-for="template in templates" :key="template.id" :value="template.id" :disabled="!template.active">{{ template.name }} · V{{ template.revision }}{{ template.active ? '' : '（停用）' }}</option></select></label>
            <label>每票固定附加运费（元）<input v-model="mode.surcharge" type="number" min="0" step="0.01" /></label>
          </div>
          <label class="toggle shipping-free"><input v-model="mode.shipping_free" type="checkbox" />客户包邮（免基础运费及附加运费）</label>
          <div class="fees-grid">
            <label>每单固定服务费（元）<input v-model="mode.fixed_fee" type="number" min="0" step="0.01" placeholder="免费填写 0" /></label>
            <label>每包装件服务费（元）<input v-model="mode.per_package_fee" type="number" min="0" step="0.01" placeholder="不用填写 0" /></label>
          </div>
          <p class="hint">服务费＝每单固定费＋每包装件费×包装件数。预估运费不在下单时收取，实际运费到账单出账；包邮不减免服务费。</p>
          <p v-if="mode.mode === 'on_demand'" class="hint">无库存代发沿用“商品 → 商品价格表”中该类别已指定的一件代发价格表，无需配置仓库。</p>
        </template>
      </fieldset>
      <div class="actions"><button type="button" @click="save" :disabled="saving">{{ saving ? '保存中…' : '保存履约设置' }}</button><button type="button" class="secondary" @click="load" :disabled="saving">重新载入</button></div>
    </template>
    <VirtualWarehouseDrawer v-if="warehouseDrawerOpen" :customer-id="props.customerId" @close="warehouseDrawerOpen = false" @changed="handleWarehouseChanged" />
  </details>
</template>
<script setup>
import { ref, watch } from 'vue'
import VirtualWarehouseDrawer from './VirtualWarehouseDrawer.vue'
import { apiGet, apiSend } from '../api/client'
import { fulfillmentSettingsForm, fulfillmentSettingsPayload, customerFinishedWarehouses } from '../lib/fulfillment-settings'
const props = defineProps({ customerId: { type: Number, required: true } })
const emit = defineEmits(['warehouses-changed'])
const form = ref(fulfillmentSettingsForm()), warehouses = ref([]), templates = ref([]), error = ref(''), message = ref(''), loading = ref(false), saving = ref(false), loaded = ref(false), warehouseDrawerOpen = ref(false)
let sequence = 0
watch(() => props.customerId, () => { sequence++; loaded.value = false; warehouseDrawerOpen.value = false; error.value = ''; message.value = ''; form.value = fulfillmentSettingsForm(); warehouses.value = [] })
function onToggle(event) { if (event.target.open && !loaded.value) load() }
async function load() {
  const request = ++sequence, customerID = props.customerId
  loading.value = true; error.value = ''; message.value = ''
  try {
    const results = await Promise.all([apiGet(`/api/customer-fulfillment/${customerID}/settings`), apiGet(`/api/stock/warehouses?customer_id=${customerID}`)])
    if (request !== sequence || customerID !== props.customerId) return
    form.value = fulfillmentSettingsForm(results[0]); warehouses.value = customerFinishedWarehouses(results[1].rows, customerID); templates.value = results[0].shipping_templates || []; loaded.value = true
  } catch (err) { if (request === sequence) error.value = err.message || '履约设置加载失败' }
  finally { if (request === sequence) loading.value = false }
}
async function handleWarehouseChanged(change) {
  const customerID = props.customerId, request = sequence
  try {
    const data = await apiGet(`/api/stock/warehouses?customer_id=${customerID}`)
    if (request !== sequence || customerID !== props.customerId) return
    warehouses.value = customerFinishedWarehouses(data.rows, customerID)
    const stock = form.value.modes.find(row => row.mode === 'stock')
    if (stock && change.action === 'create' && warehouses.value.some(row => row.code === change.warehouse?.code)) stock.warehouse_code = change.warehouse.code
    if (stock?.warehouse_code && !warehouses.value.some(row => row.code === stock.warehouse_code)) { stock.warehouse_code = ''; message.value = '原选仓库已不可用，请重新选择客户成品仓' }
    emit('warehouses-changed', { customerId: customerID, warehouses: data.rows || [] })
  } catch (err) { error.value = err.message || '仓库已保存，但候选刷新失败，请重新打开仓库核对' }
}
async function save() {
  if (saving.value) return
  const customerID = props.customerId, request = sequence
  saving.value = true; error.value = ''; message.value = ''
  try {
    const data = await apiSend(`/api/customer-fulfillment/${customerID}/settings`, { method: 'PUT', body: fulfillmentSettingsPayload(form.value) })
    if (request !== sequence || customerID !== props.customerId) return
    form.value = fulfillmentSettingsForm(data); message.value = '此客户全部履约设置已保存'
  } catch (err) { if (request === sequence && customerID === props.customerId) error.value = err.message || '履约设置保存失败' }
  finally { saving.value = false }
}
</script>
<style scoped>
.fulfillment-settings{margin-top:16px;padding-top:12px;border-top:1px solid #dce5df;min-width:0}summary{cursor:pointer;font-weight:700;color:#245c41}p{font-size:12px;line-height:1.65;color:#627269}fieldset{margin:12px 0;padding:12px;border:1px solid #dce5df;border-radius:8px;min-width:0}label{display:grid;gap:6px;font-size:12px;color:#42584b;margin-bottom:10px}.toggle{display:flex;align-items:center;gap:8px;margin:0}.toggle input{width:16px;height:16px}.shipping-free{margin-bottom:14px;line-height:1.5}input,select{width:100%;box-sizing:border-box;padding:8px;border:1px solid #cbd8cf;border-radius:6px;min-width:0}.fees-grid{display:grid;grid-template-columns:1fr 1fr;gap:8px}.warehouse-picker{display:flex;gap:8px;align-items:end;margin-bottom:10px}.warehouse-picker label{flex:1;margin:0}.warehouse-picker button{flex:none}.actions{display:flex;gap:8px;flex-wrap:wrap}button{border:1px solid #28624a;border-radius:6px;padding:8px 12px;background:#28624a;color:white;cursor:pointer}button:disabled{opacity:.5}.secondary{background:white;color:#28624a}.error,.ok{padding:8px;font-size:12px;border-radius:6px;margin:8px 0}.error{background:#fff1ed;color:#ab2b20}.ok{background:#eef8ef;color:#28624a}@media(max-width:600px){.fees-grid{grid-template-columns:1fr}.warehouse-picker{align-items:stretch;flex-direction:column}}
</style>
