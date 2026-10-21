<template>
  <section class="policies panel">
    <h3>普通发货录单运费配置</h3>
    <p>选择客户后维护普通发货政策。代发运费在客户履约配置中单独维护；预估运费不在下单时收取。</p>
    <label class="customer-picker"><span>客户</span><SearchableSelect v-model="customerID" :options="customers" :disabled="saving" :option-label="customerLabel" placeholder="搜索客户名 / 公司 / 手机号" /></label>
    <p v-if="error" class="error" role="alert">{{ error }}</p><p v-if="message" class="ok" role="status">{{ message }}</p>
    <p v-if="!customerID">请先选择需要配置普通发货运费的客户。</p>
    <p v-else-if="loading">正在加载运费政策…</p>
    <fieldset v-else-if="policy" :disabled="saving">
      <legend>普通发货录单 · {{ policy.revision ? `V${policy.revision}` : '未配置' }}</legend>
      <div class="fields">
        <label>默认运费模板<select v-model.number="policy.template_id"><option :value="0">请选择运费模板</option><option v-if="policy.template_id && !templates.some(row => Number(row.id) === Number(policy.template_id))" :value="policy.template_id" disabled>原模板不可用，请重新选择</option><option v-for="template in templates" :key="template.id" :value="template.id" :disabled="!template.active">{{ template.name }} · V{{ template.revision }}{{ template.active ? '' : '（停用）' }}</option></select></label>
        <label>每票固定附加运费（元）<input v-model="policy.surcharge" type="number" min="0" step="0.01" /></label>
        <label class="check"><input v-model="policy.free_shipping" type="checkbox" />客户包邮</label>
      </div>
      <button class="primary" type="button" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存普通发货政策' }}</button>
    </fieldset>
  </section>
</template>
<script setup>
import { onMounted, ref, watch } from 'vue'
import SearchableSelect from './SearchableSelect.vue'
import { apiGet, apiSend } from '../api/client'
import { fetchAllCustomerOptions } from '../api/view-context'
import { explicitMoneyCents } from '../lib/fulfillment-settings'
const customerID = ref(0), customers = ref([]), policy = ref(null), templates = ref([]), error = ref(''), message = ref(''), loading = ref(false), saving = ref(false)
let sequence = 0
function customerLabel(row) { return [row.name, row.company_name, row.phone].filter(Boolean).join(' · ') }
async function load() {
  const request = ++sequence, id = Number(customerID.value)
  policy.value = null; error.value = ''; message.value = ''
  if (!id) { loading.value = false; return }
  loading.value = true
  try {
    const [data, templateData] = await Promise.all([apiGet(`/api/customer-fulfillment/${id}/shipping-policies`), apiGet('/api/settings/logistics/shipping-templates')])
    if (request !== sequence || id !== Number(customerID.value)) return
    const row = (data.policies || []).find(row => row.scene === 'sale') || { scene: 'sale', template_id: 0, surcharge_cents: 0, free_shipping: false, revision: 0 }
    policy.value = { ...row, surcharge: (Number(row.surcharge_cents || 0) / 100).toFixed(2) }; templates.value = templateData.templates || []
  } catch (err) { if (request === sequence) error.value = err.message || '普通发货政策加载失败' }
  finally { if (request === sequence) loading.value = false }
}
async function save() {
  if (saving.value || !policy.value || !customerID.value) return
  const request = sequence, id = Number(customerID.value)
  saving.value = true; error.value = ''; message.value = ''
  try {
    const row = policy.value
    if (!row.free_shipping && !row.template_id) throw new Error('请选择运费模板，或明确选择包邮')
    const result = await apiSend(`/api/customer-fulfillment/${id}/shipping-policies/sale`, { method: 'PUT', body: { template_id: row.template_id, surcharge_cents: explicitMoneyCents(row.surcharge), free_shipping: row.free_shipping === true, expected_revision: row.revision } })
    if (request !== sequence || id !== Number(customerID.value)) return
    policy.value = { ...result, surcharge: (Number(result.surcharge_cents || 0) / 100).toFixed(2) }; message.value = '普通发货政策已保存，之后的新订单使用新政策'
  } catch (err) { if (request === sequence) error.value = err.message || '普通发货政策保存失败' }
  finally { saving.value = false }
}
watch(customerID, load)
onMounted(async () => { customers.value = await fetchAllCustomerOptions() })
</script>
<style scoped>
.policies{padding:16px;border:1px solid #e0e8e2;border-radius:8px;background:#fff;min-width:0}h3{font-size:16px;color:#245c41;margin:0 0 8px}p{font-size:12px;color:#63756a;line-height:1.7}.customer-picker{max-width:600px}fieldset{border:1px solid #dce5df;border-radius:8px;padding:12px;margin:14px 0 0;min-width:0}legend{font-size:13px;color:#28573b}.fields{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:10px}label{display:grid;gap:6px;font-size:12px;min-width:0}input,select{box-sizing:border-box;width:100%;padding:8px;border:1px solid #cbd9cf;border-radius:6px;min-width:0}.check{display:flex;align-items:center}.check input{width:18px}button{margin-top:12px;padding:9px 12px;border:1px solid #286248;border-radius:6px;background:#286248;color:white;cursor:pointer}button:disabled{opacity:.5}.error{color:#ad3529}.ok{color:#286248}
</style>
