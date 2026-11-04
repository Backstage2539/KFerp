<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { apiGet, apiSend, pageRequest } from '../api/client'
import { recipientAddressSnapshot } from '../lib/customer-stock-entry'
import { regionDestination } from '../lib/order-freight'
import SearchableSelect from './SearchableSelect.vue'

const props = defineProps({ customerId: { type: [Number, String], required: true }, currentRecipient: { type: Object, default: null }, apiBase: { type: String, default: "" }, requireSaved: { type: Boolean, default: false }, requireValidRegion: { type: Boolean, default: false }, customerPortal: { type: Boolean, default: false } })
const emit = defineEmits(['select'])
const open = ref(false)
const loading = ref(false)
const saving = ref(false)
const rows = ref([])
const query = ref('')
const error = ref('')
const paste = ref('')
const editing = ref(false)
const form = reactive(emptyForm())
const regions = ref([])
const regionLoading = ref(false)
let originalRegionParts = []
let originalRegionValid = false
let loadVersion = 0
const baseURL = computed(() => props.apiBase || `/api/customer-portal/admin/customers/${Number(props.customerId)}/recipient-addresses`)
const filteredRows = computed(() => {
  const search = query.value.trim().toLowerCase()
  return rows.value.filter(row => !search || [row.recipient_name, row.phone, row.company, row.province, row.city, row.district, row.detail_address].join(' ').toLowerCase().includes(search))
})
function emptyForm() { return { id: 0, revision: 0, recipient_name: '', phone: '', company: '', province: '', city: '', district: '', detail_address: '', is_default: false } }
async function load() {
  const version = ++loadVersion
  if (Number(props.customerId) <= 0) return
  loading.value = true
  error.value = ''
  try { const data = await apiGet(baseURL.value); if (version === loadVersion) rows.value = data.rows || [] }
  catch (cause) { if (version === loadVersion) error.value = cause.message || '地址簿加载失败' }
  finally { if (version === loadVersion) loading.value = false }
}
const regionOptions = computed(() => regions.value.filter(row => row.level === 3).map(row => ({ id: row.code, name: row.path || row.name, label: row.alias, code: row.code, py: row.pinyin, pyi: row.initials })))
function matchingRegion(address) {
  const fields = ['province','city','district']
  if (fields.some(key => !String(address?.[key] || '').trim())) return ''
  const map = new Map(regions.value.map(row => [row.code, row]))
  const matches = regions.value.filter(row => {
    if (row.level !== 3) return false
    const city = map.get(row.parent_code), province = map.get(city?.parent_code)
    return [province,city,row].every((region,index) => region && [region.name,region.alias].includes(String(address[fields[index]] || '').trim()))
  })
  return matches.length === 1 ? matches[0].code : ''
}
const regionCode = computed(() => matchingRegion(form))
async function loadRegions() {
  if (!props.requireValidRegion || regionLoading.value || regions.value.length) return
  regionLoading.value = true
  try { const result = props.customerPortal ? await pageRequest('/api/customer-processing/portal/direct-ship/shipping-regions') : await apiGet(`/api/customer-fulfillment/${Number(props.customerId)}/direct-ship/shipping-regions`); regions.value = result.regions || []; if (!regions.value.length) throw new Error('地区列表为空'); error.value = '' }
  catch (cause) { regions.value = []; error.value = cause.message || '地区列表加载失败，请重试' }
  finally { regionLoading.value = false }
}
function chooseRegion(code) {
  if (!code) { form.province = ''; form.city = ''; form.district = ''; return }
  const destination = regionDestination(regions.value, code)
  if (!destination.province || !destination.city || !destination.district) return
  const selectedNames = regions.value.filter(row => [destination.province_code,destination.city_code,destination.district_code].includes(row.code)).flatMap(row => [row.name,row.alias])
  const details = originalRegionParts.filter((part,index) => part && !selectedNames.includes(part) && (!regions.value.some(row => row.name === part || row.alias === part) || index === 2 && !originalRegionValid) && !form.detail_address.includes(part))
  if (details.length) form.detail_address = [...new Set(details), form.detail_address].filter(Boolean).join(' ')
  Object.assign(form, { province: destination.province, city: destination.city, district: destination.district })
  error.value = ''
}
function show() { open.value = true; editing.value = false; query.value = ''; void load(); void loadRegions() }
function edit(row) { Object.assign(form, emptyForm(), row || {}); originalRegionParts = [form.province,form.city,form.district].map(value => String(value || '').trim()); originalRegionValid = Boolean(matchingRegion(form)); paste.value = ''; editing.value = true; error.value = '' }
function createFromOrder() {
  const current = props.currentRecipient || {}
  edit({ recipient_name: current.receiver_name || '', phone: current.receiver_phone || '', company: current.receiver_company || '', province: current.province || '', city: current.city || '', district: current.district || '', detail_address: current.detail_address || current.receiver_address || '', is_default: !rows.value.length })
}
function use(row) { if (props.requireSaved && !Number(row.id)) { error.value = '请先保存地址再使用'; return }; if (!row.recipient_name?.trim() || !row.phone?.trim() || !row.detail_address?.trim()) { error.value = '请填写收件人、电话和详细地址'; return }; emit('select', recipientAddressSnapshot(row), row); open.value = false }
async function parse() {
  if (!paste.value.trim() || saving.value) return
  saving.value = true
  try { const parsed = await apiSend('/api/customer-recipient/parse', { body: { text: paste.value } }); Object.assign(form, { recipient_name: parsed.recipient_name || form.recipient_name, phone: parsed.phone || form.phone, province: parsed.province || '', city: parsed.city || '', district: parsed.district || '', detail_address: parsed.detail_address || parsed.address || form.detail_address }); originalRegionParts = [form.province,form.city,form.district].map(value => String(value || '').trim()); originalRegionValid = Boolean(matchingRegion(form)) }
  catch (cause) { error.value = cause.message || '解析失败' }
  finally { saving.value = false }
}
async function save(useAfterSave) {
  if (saving.value) return
  if (!form.recipient_name.trim() || !form.phone.trim() || !form.detail_address.trim()) { error.value = '请填写收件人、电话和详细地址'; return }
  const regionChanged = [form.province,form.city,form.district].some((value,index) => String(value || '').trim() !== originalRegionParts[index])
  if (props.requireValidRegion && regionChanged && !matchingRegion(form)) { error.value = '请选择有效的省市区，并核对详细地址'; return }
  saving.value = true
  error.value = ''
  const customerID = Number(props.customerId)
  try {
    const row = await apiSend(`${baseURL.value}${form.id ? `/${form.id}` : ''}`, { method: form.id ? 'PUT' : 'POST', body: { ...form, expected_revision: form.revision } })
    if (customerID !== Number(props.customerId)) return
    editing.value = false
    if (useAfterSave) use(row)
    else await load()
  } catch (cause) { error.value = cause.message || '地址保存失败' }
  finally { saving.value = false }
}
async function remove(row) {
  if (!window.confirm(`删除 ${row.recipient_name} 的收件地址？`)) return
  try { await apiSend(`${baseURL.value}/${row.id}?revision=${row.revision}`, { method: 'DELETE' }); await load() }
  catch (cause) { error.value = cause.message || '删除失败' }
}
watch(() => props.customerId, () => { loadVersion += 1; rows.value = []; open.value = false; editing.value = false })
defineExpose({ show })
</script>

<template>
  <button class="secondary address-book-button" type="button" :disabled="Number(customerId) <= 0" @click="show">选择 / 维护发货地址簿</button>
  <Teleport to="body">
    <div v-if="open" class="address-overlay" @click.self="open = false">
      <section class="address-dialog" role="dialog" aria-modal="true" aria-label="客户发货地址簿">
        <header><div><h3>客户发货地址簿</h3><p>与该客户的小程序共用。收件信息独立保存，订单保留地址快照。</p></div><button type="button" class="secondary" @click="open = false">返回录单</button></header>
        <div v-if="!editing" class="address-toolbar"><input v-model="query" type="search" placeholder="搜索姓名、电话、门店或地址" /><button type="button" class="primary" @click="createFromOrder">新增地址</button></div>
        <button v-if="requireValidRegion && !regions.length && !editing" type="button" class="secondary" :disabled="regionLoading" @click="loadRegions">{{ regionLoading ? '正在加载地区…' : '重试加载地区' }}</button>
        <p v-if="loading">正在载入地址簿…</p>
        <div v-if="!editing" class="address-list">
          <article v-for="row in filteredRows" :key="row.id"><div><strong>{{ row.recipient_name }} · {{ row.phone }} {{ row.is_default ? '（默认）' : '' }}</strong><p>{{ row.company }}</p><p>{{ row.province }}{{ row.city }}{{ row.district }}{{ row.detail_address }}</p></div><div class="address-actions"><button type="button" class="primary" @click="use(row)">使用地址</button><button type="button" class="secondary" @click="edit(row)">编辑</button><button type="button" class="secondary" @click="remove(row)">删除</button></div></article>
          <p v-if="!loading && !filteredRows.length">暂无匹配地址，可新增地址并保留已录商品。</p>
        </div>
        <form v-else class="address-form" @submit.prevent="save(true)">
          <label class="wide">粘贴收件信息<textarea v-model="paste" rows="2" placeholder="粘贴收件人、电话、地址" /></label><button type="button" class="secondary" :disabled="saving || !paste.trim()" @click="parse">解析收件信息</button>
          <label>收件人<input v-model.trim="form.recipient_name" required /></label><label>电话<input v-model.trim="form.phone" required /></label><label>公司 / 门店<input v-model.trim="form.company" /></label>
          <template v-if="requireValidRegion"><label class="wide">省市区<SearchableSelect :model-value="regionCode" :options="regionOptions" option-value="id" :empty-value="''" :disabled="regionLoading || !regions.length" placeholder="搜索区县，选择完整省市区" @update:model-value="chooseRegion" /></label><p class="wide">当前地区：{{ [form.province, form.city, form.district].filter(Boolean).join(' / ') || '尚未选择' }}。请选择行政区，街道、古城等请保留在详细地址。</p><button v-if="!regions.length" type="button" class="secondary" :disabled="regionLoading" @click="loadRegions">{{ regionLoading ? '正在加载地区…' : '重试加载地区' }}</button></template>
          <template v-else><label>省<input v-model.trim="form.province" /></label><label>市<input v-model.trim="form.city" /></label><label>区 / 县<input v-model.trim="form.district" /></label></template><label class="wide">详细地址<input v-model.trim="form.detail_address" required /></label>
          <label class="check"><input v-model="form.is_default" type="checkbox" />设为默认</label><div class="address-actions wide"><button type="button" class="secondary" @click="editing = false">取消编辑</button><button v-if="!requireSaved" type="button" class="secondary" :disabled="saving" @click="use(form)">仅本单使用</button><button class="primary" type="submit" :disabled="saving">保存并使用</button></div>
        </form>
        <p v-if="error" class="error" role="alert">{{ error }}</p>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.address-book-button{justify-self:start}.address-overlay{position:fixed;inset:0;z-index:2200;background:#0b211a70;display:grid;place-items:center;padding:20px}.address-dialog{width:min(900px,100%);max-height:90vh;overflow:auto;background:#fff;border-radius:14px;padding:22px;box-sizing:border-box;color:#203c30}.address-dialog header,.address-toolbar,.address-actions{display:flex;justify-content:space-between;align-items:center;gap:10px}.address-dialog h3{margin:0}.address-dialog p{margin:6px 0;color:#64746b;font-size:13px}.address-toolbar{margin:18px 0}.address-toolbar input{flex:1}.address-list{display:grid;gap:12px}.address-list article{display:flex;justify-content:space-between;gap:16px;padding:15px;border:1px solid #dce7df;border-radius:8px}.address-form{display:grid;grid-template-columns:repeat(3,1fr);gap:12px;margin-top:16px}.address-form label{display:grid;gap:6px;font-size:13px}.address-form .check{display:flex;align-items:center}.wide{grid-column:1/-1}.address-dialog input:not([type=checkbox]),.address-dialog textarea{width:100%;box-sizing:border-box;padding:9px;border:1px solid #cbd9d0;border-radius:6px}.address-dialog button{padding:8px 12px;border:1px solid #c8d8ce;border-radius:6px;cursor:pointer;white-space:nowrap}.primary{background:#28624a;color:#fff}.secondary{background:#fff;color:#315844}.error{color:#b42318!important}.address-actions{flex-wrap:wrap;justify-content:flex-end}@media(max-width:650px){.address-overlay{padding:8px}.address-dialog header,.address-list article{align-items:stretch;flex-direction:column}.address-form{grid-template-columns:1fr}.address-toolbar{align-items:stretch;flex-direction:column}}
</style>
