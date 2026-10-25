<template>
  <section class="receipt-form" aria-label="确认数量并入库">
    <header><div><h3>确认数量并入库</h3><p>{{ workOrderNo }} · 在当前工单直接办理</p></div><button type="button" :disabled="busy || uncertain" @click="$emit('close')">收起</button></header>
    <p v-if="busy && !row" role="status">正在读取本单数量和用料…</p>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="warehouseError" class="error" role="alert">仓库未加载：{{ warehouseError }}，请重试读取后再入库。</p>
    <button v-if="!row || warehouseError" type="button" :disabled="busy || uncertain" @click="load">重新读取</button>
    <form v-if="row" @submit.prevent="submit">
      <div class="identity"><strong>{{ row.output_name || row.product_name }}</strong><span>规格：{{ row.package_spec_label || '无包装规格' }}</span><span>目标仓库：{{ row.target_warehouse_name }}</span></div>
      <p>原完成 {{ row.reported_qty }} {{ row.output_unit }} · 已入库 {{ row.received_qty }} {{ row.output_unit }}；请填写现场实际数量。</p>
      <p v-if="row.review_blocked_reason" class="error" role="alert">{{ row.review_blocked_reason }}</p>
      <fieldset :disabled="busy || uncertain || !!row.review_blocked_reason">
        <div class="fields"><label>{{ row.output_unit === '袋' ? '实际完成袋数' : `实际完成数量（${row.output_unit}）` }}<input v-model="draft.actual_output_qty" type="number" min="0" :step="weightUnit(row.output_unit) ? 'any' : '1'" placeholder="填写实际数量" required @input="suggestMaterials" /></label><label>备注<input v-model="draft.reason" placeholder="数量确认或差异说明" required /></label></div>
        <p v-if="draft.actual_output_qty !== ''">本次补入库 {{ Math.max(0, Number(draft.actual_output_qty) - Number(row.received_qty || 0)) }} {{ row.output_unit }}；已有入库不会重复增加。</p>
        <div v-if="row.correction_cards?.length > 1" class="fields"><label v-for="(card, index) in row.correction_cards" :key="card.id">{{ card.operation }}第 {{ index + 1 }} 批实际数量（{{ row.output_unit }}）<input v-model="draft.card_quantities[card.id]" type="number" min="0" :step="weightUnit(row.output_unit) ? 'any' : '1'" required /></label></div>
        <details :open="materialsNeedInput"><summary>本单用料（可修改实际用量）</summary><p>按确认数量带入冻结配方用料；已有耗料会抵扣。请核实实际用量及来源仓。</p><div v-for="material in draft.materials" :key="material.reservation_id" class="material"><strong>{{ material.name }}<small>已有耗料 {{ material.consumed_qty }} {{ material.unit }}</small></strong><label>累计实际用量（{{ material.unit }}）<input v-model="material.qty" type="number" min="0" :step="weightUnit(material.unit) ? 'any' : '1'" required @input="material.edited = true" /></label><label>来源仓<select v-model="material.warehouse"><option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}</option></select></label></div></details>
      </fieldset>
      <button class="primary" :disabled="busy || !!warehouseError || !!row.review_blocked_reason">{{ busy ? '正在入库…' : uncertain ? '重试本次入库' : '确认并入库' }}</button>
    </form>
  </section>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { weightUnit } from '../lib/operation-completion.js'
import { legacyReviewDraft, legacyReviewPayload } from '../lib/production-records.js'

const props = defineProps({ workOrderId: { type: Number, required: true }, workOrderNo: { type: String, default: '' } })
const emit = defineEmits(['close', 'posted', 'locked'])
const row = ref(null), draft = ref(null), warehouses = ref([]), busy = ref(false), uncertain = ref(false), error = ref(''), warehouseError = ref('')
let attempt = null
const materialsNeedInput = computed(() => draft.value?.materials.some(material => material.qty === '') || false)
watch(() => busy.value || uncertain.value, value => emit('locked', value), { flush: 'sync' })

async function load() {
  if (busy.value || uncertain.value) return
  busy.value = true; error.value = ''; warehouseError.value = ''
  const [preview, locations] = await Promise.allSettled([
    apiSend('/api/production/legacy-review/preview', { body: { work_order_ids: [props.workOrderId] } }),
    apiGet('/api/stock/warehouses'),
  ])
  if (preview.status === 'fulfilled') {
    row.value = (preview.value.rows || []).find(item => Number(item.work_order_id) === props.workOrderId) || null
    if (!row.value) error.value = '本单没有待入库的旧完成记录，请刷新工单列表查看最新状态。'
    else draft.value ||= legacyReviewDraft(row.value)
  } else error.value = preview.reason.message || '读取本单入库数量失败'
  if (locations.status === 'fulfilled') {
    const data = locations.value
    warehouses.value = (Array.isArray(data) ? data : data.rows || data.warehouses || []).filter(item => item.active !== false)
  } else warehouseError.value = locations.reason.message || '仓库加载失败'
  busy.value = false
}

function suggestMaterials() {
  const output = Number(draft.value.actual_output_qty), planned = Number(row.value.planned_qty)
  for (const material of draft.value.materials) {
    if (material.edited) continue
    if (draft.value.actual_output_qty === '' || !Number.isFinite(output) || output < 0 || !(planned > 0)) { material.qty = ''; continue }
    const suggested = Number(row.value.materials.find(item => item.reservation_id === material.reservation_id)?.qty || 0) * output / planned
    material.qty = weightUnit(material.unit) ? Number(suggested.toFixed(6)) : Math.ceil(suggested)
  }
}

async function submit() {
  if (busy.value || warehouseError.value || row.value?.review_blocked_reason) return
  error.value = ''
  try {
    if (!attempt) attempt = { items: [legacyReviewPayload(row.value, draft.value, crypto.randomUUID())] }
  } catch (err) { error.value = err.message; return }
  busy.value = true
  try {
    const data = await apiSend('/api/production/legacy-review/submit', { body: attempt })
    const result = (data.rows || []).find(item => Number(item.work_order_id) === props.workOrderId)
    if (!result) throw new Error('未收到本单入库结果')
    if (!result.succeeded) { attempt = null; uncertain.value = false; error.value = result.error || '入库未完成，请核对后重试'; return }
    attempt = null; uncertain.value = false; busy.value = false
    emit('posted', result)
  } catch (err) {
    uncertain.value = !err.status || err.status >= 500
    if (!uncertain.value) attempt = null
    error.value = uncertain.value ? '尚未收到入库结果，填写内容已保留，请重试本次入库，不会重复记账。' : err.message
  } finally { busy.value = false }
}
onMounted(load)
</script>

<style scoped>
.receipt-form{padding:20px;background:#f1f8f3;border:1px solid #aed0bc;border-radius:12px;color:#203c31;text-align:left}.receipt-form header{display:flex;justify-content:space-between;align-items:flex-start;gap:16px}.receipt-form h3{margin:0}.receipt-form p{margin:10px 0;color:#62766c}.receipt-form .identity{display:flex;gap:20px;flex-wrap:wrap;margin:15px 0}.receipt-form fieldset{border:0;padding:0}.receipt-form .fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px;margin:12px 0}.receipt-form label{display:grid;gap:6px}.receipt-form input,.receipt-form select{padding:10px;border:1px solid #c6d3cb;border-radius:8px;font:inherit;min-width:0;background:white}.receipt-form button{padding:10px 16px;border:1px solid #b8cfc0;border-radius:8px;font:inherit;background:white;cursor:pointer}.receipt-form .primary{background:#2d915e;color:white;margin-top:16px;min-width:180px}.receipt-form button:disabled{opacity:.5;cursor:default}.receipt-form .error{color:#a13624}.receipt-form details{margin:14px 0}.receipt-form summary{cursor:pointer;font-weight:600}.receipt-form .material{display:grid;grid-template-columns:1fr 1fr 1fr;gap:14px;margin:14px 0;align-items:center}.receipt-form small{display:block;font-weight:normal;color:#62766c}@media(max-width:700px){.receipt-form .fields,.receipt-form .material{grid-template-columns:1fr}}
</style>
