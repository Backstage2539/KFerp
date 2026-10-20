<script setup>
import {explicitMoneyCents} from '../lib/fulfillment-settings'
import {freightLabel} from '../lib/freight'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { readFormDraft, saveFormDraft, clearFormDraft } from '../lib/form-draft-cache'
import { enabledFulfillmentModes, stockSupplySummary } from '../lib/customer-stock-entry'
import SearchableSelect from './SearchableSelect.vue'
import CustomerRecipientPicker from './CustomerRecipientPicker.vue'
import OrderEntryView from '../views/OrderEntryView.vue'

const props = defineProps({ customerId: { type: [Number, String], required: true }, customerLabel: { type: String, default: '' }, customerPortal: { type: Boolean, default: false } })
const emit = defineEmits(['saved'])
const settings = ref(null)
const catalog = ref({ product_families: [], price_tables: [] })
const mode = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const preview = ref(null)
const topup = ref(null)
const override=reactive({enabled:false,surcharge:'0.00',free:false,reason:''})
const form = reactive({ order_date: new Date().toLocaleDateString('en-CA', { timeZone: 'Asia/Shanghai' }), receiver_name: '', receiver_phone: '', receiver_address: '', receiver_company: '', province: '', city: '', district: '', note: '' })
const lines = ref([newLine()])
const requestID = ref(newRequestID())
let version = 0
let previewVersion = 0
let loadCycle = 0
const baseURL = computed(() => props.customerPortal ? "/api/customer-processing/portal" : `/api/customer-fulfillment/${Number(props.customerId)}`)
const settingsURL = computed(() => props.customerPortal ? `${baseURL.value}/direct-ship/settings` : `${baseURL.value}/settings`)
const addressURL = computed(() => props.customerPortal ? `${baseURL.value}/recipient-addresses` : "")
const modes = computed(() => enabledFulfillmentModes(settings.value))
const activeMode = computed(() => modes.value.find(row => row.mode === mode.value))
const families = computed(() => catalog.value.product_families || [])
const goodsAmount = computed(() => Number(preview.value?.total_amount || 0))
const serviceAmount = computed(() => Number(preview.value?.fees?.service_fee_cents || 0) / 100)
const freightAmount = computed(() => Number(preview.value?.fees?.shipping_fee_cents || 0) / 100)
const totalAmount = computed(() => goodsAmount.value + serviceAmount.value + freightAmount.value)
function modeLabel(value) { return value === 'stock' ? '库存代发' : '无库存代发' }
function money(value) { return Number(value || 0).toFixed(2) }
function newRequestID() { return `erp-ds-${Date.now()}-${Math.random().toString(36).slice(2, 12)}` }
function newLine() { return { key: newRequestID(), family_key: '', spec_key: '', qty: 1 } }
function familyKey(row) { return String(row.parent_product_id || row.product_id || row.name || '') }
function familyLabel(row) { return row.customer_product_display_name || row.alias_name || row.name || row.parent_product_name || '' }
function specKey(row) { return `${row.sku_id || row.product_id || 0}:${row.bom_spec_id || 0}:${row.bom_variant_id || 0}:${row.spec_g || row.unit_bean_g || row.net_content_qty || 0}:${row.sales_unit || ''}` }
function specLabel(row) { return row.spec_name || row.spec_label || row.sku_name || `${row.unit_bean_g || row.net_content_qty || 0}g` }
function familyFor(line) { return families.value.find(row => familyKey(row) === line.family_key) }
function specFor(line) { return familyFor(line)?.specs?.find(row => specKey(row) === line.spec_key) }
function selectFamily(line, family) { const spec = family?.specs?.find(row => row.is_default || row.is_default_sku) || family?.specs?.[0]; line.family_key = familyKey(family); line.spec_key = spec ? specKey(spec) : ''; invalidate() }
function quantityFor(line) { return stockSupplySummary(specFor(line), line.qty) }
function draftKey(customerID = props.customerId, scenario = mode.value) { return `customer-direct-ship:${Number(customerID)}:${scenario || 'unselected'}` }
function saveDraft(customerID = props.customerId, scenario = mode.value) { if (Number(customerID) > 0) { saveFormDraft(draftKey(customerID, scenario), { form: { ...form }, lines: lines.value, request_id: requestID.value }); if (scenario) saveFormDraft(`customer-direct-ship:${Number(customerID)}:selection`, { mode: scenario }) } }
function restoreDraft() { const draft = readFormDraft(draftKey()); if (draft) { Object.assign(form, draft.form || {}); lines.value = draft.lines?.length ? draft.lines : [newLine()]; requestID.value = draft.request_id || newRequestID() } }
function invalidate() { previewVersion += 1; preview.value = null; saveDraft() }
function selectRecipient(snapshot, address) { Object.assign(form, snapshot, { receiver_address: address.detail_address || '', province: address.province || '', city: address.city || '', district: address.district || '' }); invalidate() }
async function loadCatalog() {
  const current = ++version
  if (!mode.value) { catalog.value = { product_families: [], price_tables: [] }; return }
  loading.value = true
  try { const value = await apiGet(`${baseURL.value}/direct-ship/catalog?fulfillment_mode=${mode.value}`); if (current === version) catalog.value = value }
  catch (cause) { if (current === version) { catalog.value = { product_families: [], price_tables: [] }; error.value = cause.message || '商品加载失败' } }
  finally { if (current === version) loading.value = false }
}
async function load() {
  const cycle = ++loadCycle
  const customerID = Number(props.customerId)
  loading.value = true; error.value = ''; preview.value = null; topup.value = null
  try { const value = await apiGet(settingsURL.value); if (cycle !== loadCycle || customerID !== Number(props.customerId)) return; settings.value = value; const restoredMode = readFormDraft(`customer-direct-ship:${customerID}:selection`)?.mode; const chosen = mode.value || restoredMode; mode.value = modes.value.some(row => row.mode === chosen) ? chosen : modes.value.length === 1 ? modes.value[0].mode : ''; restoreDraft(); await loadCatalog() }
  catch (cause) { if (cycle === loadCycle) error.value = cause.message || '履约设置加载失败' }
  finally { if (cycle === loadCycle) loading.value = false }
}
async function changeMode(next) { if (saving.value || mode.value === next) return; saveDraft(); mode.value = next; lines.value = [newLine()]; requestID.value = newRequestID(); preview.value = null; error.value = ''; restoreDraft(); await loadCatalog() }
function buildItems() { return lines.value.map(line => { const spec = specFor(line); return { product_id: Number(spec?.sku_id || spec?.product_id || 0), bom_spec_id: Number(spec?.bom_spec_id || 0), bom_variant_id: Number(spec?.bom_variant_id || 0), inventory_unit: spec?.inventory_unit || '', sales_unit: spec?.sales_unit || '', spec_g: Number(spec?.bom_spec_id || 0) > 0 ? 0 : Number(spec?.spec_g || spec?.unit_bean_g || (spec?.net_content_unit === 'kg' ? Number(spec?.net_content_qty) * 1000 : spec?.net_content_qty) || 0), qty: Number(line.qty) } }) }
function command() { return { ...(override.enabled?{freight_override:{surcharge_cents:explicitMoneyCents(override.surcharge),free_shipping:override.free,reason:override.reason}}:{}), fulfillment_mode: mode.value, idempotency_key: requestID.value, order_date: form.order_date, recipient_name: form.receiver_name.trim(), recipient_phone: form.receiver_phone.trim(), province: form.province.trim(), city: form.city.trim(), district: form.district.trim(), detail_address: form.receiver_address.trim(), recipient_company: form.receiver_company.trim(), items: buildItems(), note: form.note.trim() } }
function validation() { if (!activeMode.value) return modes.value.length ? '请选择库存代发或无库存代发场景' : '请先在履约设置中启用代发场景'; if (!form.receiver_name.trim() || !form.receiver_phone.trim() || !form.receiver_address.trim()) return '请填写完整收件信息'; if (!lines.value.length || buildItems().some(item => !item.product_id || !Number.isInteger(item.qty) || item.qty <= 0)) return '请完整选择商品规格，数量须为正整数'; return '' }
async function check() {
  const problem = validation(); if (problem) { error.value = problem; return null }
  const current = ++previewVersion
  loading.value = true; error.value = ''
  try { const result = await apiSend(`${baseURL.value}/direct-ship/preview`, { body: command() }); if (current !== previewVersion) return null; preview.value = result; return result }
  catch (cause) { error.value = cause.message || '核价失败'; return null }
  finally { loading.value = false }
}
async function submit() {
  if (saving.value) return
  const checked = await check(); if (!checked) return
  if (!checked.can_submit) { error.value = checked.needs_topup ? '购买权益不足，请先补购留厂订单' : '当前订单无法提交，请查看库存缺口'; return }
  saving.value = true
  const submittedCustomerID = Number(props.customerId)
  try {
    const saved = await apiSend(`${baseURL.value}/direct-ship/requests`, { body: { ...command(), freight_protocol:'deferred_freight_v1', price_quote_token: checked.price_quote_token, fee_quote_token: checked.fee_quote_token || checked.fees?.fee_quote_token } })
    if (submittedCustomerID !== Number(props.customerId)) return
    notice.value = `代发订单已提交：${saved.order_no || saved.request_no || saved.id || ''}`
    clearFormDraft(draftKey()); lines.value = [newLine()]; form.note = ''; requestID.value = newRequestID(); preview.value = null; emit('saved', saved); await loadCatalog()
  } catch (cause) { error.value = cause.message || '提交失败'; saveDraft() }
  finally { saving.value = false }
}
function startTopup() {
  saveDraft()
  const seen = new Set()
  const items = buildItems()
  const gaps = lines.value.map((line,index) => { const spec = specFor(line); const item = items[index]; const key = `${item.product_id}:${item.bom_spec_id}:${item.bom_variant_id}:${item.spec_g}`; if (seen.has(key)) return { qty: 0 }; seen.add(key); const shortage = preview.value?.shortages?.find(row => Number(row.product_id) === item.product_id && Number(row.bom_spec_id || 0) === item.bom_spec_id && Number(row.bom_variant_id || 0) === item.bom_variant_id && (item.bom_spec_id > 0 || Number(row.spec_g || 0) === item.spec_g)); const gap = shortage ? Math.max(0, Number(shortage.qty) - Number(shortage.available_qty || 0)) : quantityFor(line).gap; return { ...item, product_name: familyLabel(familyFor(line) || {}), spec: specLabel(spec || {}), unit: spec?.inventory_unit || '件', qty: gap } }).filter(row => row.qty > 0)
  if (!gaps.length) { error.value = '当前没有待补购数量，请重新核价'; return }
  topup.value = { warehouse_code: activeMode.value?.warehouse_code || '', items: gaps }
}
async function topupSaved() { topup.value = null; notice.value = '补购留厂订单已保存；实际入客户仓后才能发货。原代发草稿已保留。'; await loadCatalog(); await check() }
onMounted(load)
onBeforeUnmount(() => { saveDraft(); version += 1; previewVersion += 1; loadCycle += 1 })
watch(() => props.customerId, async (next, previous) => { saveDraft(previous); version += 1; previewVersion += 1; mode.value = ''; Object.assign(form, { receiver_name: '', receiver_phone: '', receiver_address: '', receiver_company: '', province: '', city: '', district: '', note: '' }); lines.value = [newLine()]; requestID.value = newRequestID(); await load() })
</script>

<template>
  <div class="direct-ship-form">
    <div v-if="topup" class="topup-panel"><div class="form-head"><h4>补购留厂订单</h4><button type="button" class="secondary" @click="topup = null">返回代发草稿</button></div><p>只带入超出已购权益的缺口，按正常销售价格表购买。保存后返回原代发草稿。</p><OrderEntryView :key="`${customerId}:topup`" embedded :customer-portal="customerPortal" :fulfillment-mode="customerPortal" portal-service="product_order" :customer-context-id="customerId" :customer-context-label="customerLabel" :delivery-prefill="topup" @saved="topupSaved" /></div>
    <template v-else>
      <div class="mode-choices"><button v-for="option in modes" :key="option.mode" type="button" :class="{ selected: mode === option.mode }" :disabled="loading || saving" @click="changeMode(option.mode)">{{ modeLabel(option.mode) }}</button><span v-if="!loading && !modes.length">请在门户客户配置的履约设置中启用库存代发或无库存代发。</span></div>
      <p v-if="mode === 'stock'" class="explanation">从绑定客户仓选择已购商品。实物优先，待入库权益可预订；超权益先补购。商品货款不重复收取。</p>
      <p v-if="mode === 'on_demand'" class="explanation">每个商品规格使用代发价格表中的唯一固定单价，任意正数量均可下单，另计代发服务费和运费。</p>
      <fieldset v-if="mode" class="entry-grid" :disabled="saving">
        <label>订单日期<input v-model="form.order_date" type="date" @change="invalidate" /></label><label v-if="mode === 'stock'">客户仓库<input :value="activeMode?.warehouse_code || '尚未绑定客户仓库'" readonly /></label>
        <div class="wide"><CustomerRecipientPicker :customer-id="customerId" :current-recipient="form" :api-base="addressURL" @select="selectRecipient" /></div>
        <label>收件人<input v-model.trim="form.receiver_name" @input="invalidate" /></label><label>电话<input v-model.trim="form.receiver_phone" @input="invalidate" /></label><label>门店 / 公司<input v-model.trim="form.receiver_company" @input="invalidate" /></label>
        <label>省<input v-model.trim="form.province" placeholder="运费匹配省份" @input="invalidate" /></label><label>市<input v-model.trim="form.city" @input="invalidate" /></label><label>区 / 县<input v-model.trim="form.district" @input="invalidate" /></label><label class="wide">详细地址<input v-model.trim="form.receiver_address" @input="invalidate" /></label>
        <div v-for="(line,index) in lines" :key="line.key" class="line wide"><label>商品<SearchableSelect v-model="line.family_key" :options="families" :option-label="familyLabel" :option-value="familyKey" placeholder="搜索商品" @select="selectFamily(line,$event)" /></label><label>规格<select v-model="line.spec_key" @change="invalidate"><option value="">请选择规格</option><option v-for="spec in familyFor(line)?.specs || []" :key="specKey(spec)" :value="specKey(spec)">{{ specLabel(spec) }}</option></select></label><label>数量<input v-model.number="line.qty" type="number" min="1" step="1" @input="invalidate" /></label><button type="button" class="secondary" :disabled="saving" @click="lines.splice(index,1); invalidate()">删除</button><div v-if="mode === 'stock' && specFor(line)" class="supply wide"><span>实物可用 {{ quantityFor(line).physical }}</span><span>待入库 {{ quantityFor(line).pending }}</span><span>待入库已预订 {{ quantityFor(line).reserved }}</span><span>本单待补购 {{ quantityFor(line).gap }}</span></div></div>
        <button type="button" class="secondary" :disabled="saving" @click="lines.push(newLine()); invalidate()">新增商品</button><label class="wide">备注<textarea v-model.trim="form.note" rows="2" @input="invalidate" /></label>
        <details v-if="!props.customerPortal" class="wide"><summary>本单运费调整</summary><label><input v-model="override.enabled" type="checkbox" @change="invalidate" />本单调整附加运费／包邮</label><template v-if="override.enabled"><label>附加运费（元）<input v-model="override.surcharge" type="number" min="0" step="0.01" @input="invalidate" /></label><label><input v-model="override.free" type="checkbox" @change="invalidate" />本单包邮</label><label>调整原因<input v-model.trim="override.reason" @input="invalidate" /></label></template></details>
        <div v-if="preview" class="quote wide"><span>商品货款 ¥{{ money(goodsAmount) }}</span><span>代发服务费 ¥{{ money(serviceAmount) }}</span><span>{{preview.fees?.freight?freightLabel(preview.fees.freight):`运费 ¥${money(freightAmount)}`}}</span><span v-if="preview.fees?.freight">预估不计当前应付，实际运费出账后另收</span><strong>本次应付 ¥{{ money(totalAmount) }}</strong><span v-if="mode === 'stock' && !preview.stock_ready" class="wide">已购待入库商品将预订权益，实际入库后才能发货。</span></div>
        <div v-if="preview?.needs_topup" class="shortage wide">超出已购权益，待补购 {{ preview.purchase_gap_qty || 0 }} 件。<button type="button" class="secondary" @click="startTopup">保存草稿并补购</button></div>
        <div class="form-actions wide"><button type="button" class="secondary" :disabled="loading || saving" @click="check">核对库存与费用</button><button type="button" class="primary" :disabled="loading || saving || Boolean(preview && !preview.can_submit)" @click="submit">{{ saving ? '提交中…' : '提交代发订单' }}</button></div>
      </fieldset>
    </template>
    <p v-if="error" class="error" role="alert">{{ error }}</p><p v-if="notice" class="notice">{{ notice }}</p>
  </div>
</template>

<style scoped>
.direct-ship-form,.entry-grid{display:grid;gap:12px}.entry-grid{border:0;padding:0;margin:0;min-width:0;grid-template-columns:repeat(3,minmax(0,1fr))}.direct-ship-form label{display:grid;gap:6px;font-size:13px}.wide{grid-column:1/-1}.direct-ship-form input,.direct-ship-form select,.direct-ship-form textarea{width:100%;box-sizing:border-box;min-height:38px;padding:8px;border:1px solid #cbd8d0;border-radius:6px;background:#fff}.mode-choices,.form-actions,.form-head,.supply,.quote{display:flex;gap:12px;align-items:center;flex-wrap:wrap}.mode-choices button,.direct-ship-form button{padding:9px 14px;border:1px solid #cbd9d0;border-radius:6px;cursor:pointer}.mode-choices .selected,.primary{background:#28624a;color:#fff}.secondary{background:#fff;color:#315844}.explanation{margin:0;padding:12px;border-radius:7px;background:#f0f7f2;color:#496453;font-size:13px;line-height:1.6}.line{display:grid;grid-template-columns:2fr 1fr 1fr auto;gap:10px;align-items:end;padding:12px;border:1px solid #dfe8e2;border-radius:8px}.supply{padding-top:10px;color:#65766c;font-size:12px}.quote{justify-content:space-between;padding:14px;background:#edf5f0;border-radius:8px}.quote strong{color:#1c6544}.shortage,.error{color:#b42318;background:#fff4f1;padding:10px;border-radius:6px}.notice{color:#28624a}.form-head{justify-content:space-between}.topup-panel>p{font-size:13px;color:#64756a}@media(max-width:900px){.entry-grid{border:0;padding:0;margin:0;min-width:0;grid-template-columns:1fr 1fr}.line{grid-template-columns:1fr 1fr}.line>label:first-child{grid-column:1/-1}}@media(max-width:560px){.entry-grid{border:0;padding:0;margin:0;min-width:0;grid-template-columns:1fr}.form-actions button{flex:1}}
</style>
