<template>
  <div class="batch-editor">
    <label><span>每批数量</span><select :value="isCustom ? 'custom' : 'standard'" :disabled="disabled || !capacity" @change="changeMode($event.target.value)"><option value="standard">标准 {{ quantity(capacity?.batch_size_qty ?? split.batch_size_qty) }}{{ split.batch_size_unit }}/批</option><option value="custom">本次自定义</option></select></label>
    <label v-if="isCustom"><span>自定义每批（{{ split.batch_size_unit }}）</span><input :value="split.custom_batch_size_qty" type="number" min="0.001" step="0.001" :disabled="disabled" @input="changeCustom($event.target.value)" @blur="commitCustom" @keydown.enter.prevent="commitCustom" /></label>
    <label><span>批次</span><input :value="split.batch_input ? split.planned_batch_count : metrics.planned_batch_count" type="number" min="1" step="1" :disabled="disabled" @input="changeBatch($event.target.value)" /></label>
    <div v-if="recommendation" class="batch-advice">
      <template v-if="recommendation.batch_count > 0">
        <span>按其余安排计算，建议 {{ recommendation.batch_count }} 批（{{ quantity(recommendation.planned_qty) }}{{ recommendation.unit }}），{{ variance(recommendation.diff_qty, recommendation.unit) }}。</span>
        <button type="button" :disabled="disabled" @click="useRecommendation">采用建议</button>
        <span v-if="recommendation.lower_batch_count > 0">选择 {{ recommendation.lower_batch_count }} 批（{{ quantity(recommendation.lower_planned_qty) }}{{ recommendation.unit }}）则{{ variance(recommendation.lower_diff_qty, recommendation.unit) }}。</span>
      </template>
      <span v-else>其他工位已覆盖本工序目标，仍可继续安排正整数批次。</span>
      <small>批次可自行调整；不足或超排仅作提示，不阻止继续。{{ isCustom ? '自定义数量仅用于本次安排。' : '' }}</small>
    </div>
    <div v-if="showPresetPrompt" class="preset-prompt">
      <span>是否将 {{ quantity(split.custom_batch_size_qty) }}{{ split.batch_size_unit }}/批保存为该工位的常用产能？</span>
      <button type="button" :disabled="disabled || presetSaving" @click="savePreset">{{ presetSaving ? '正在保存…' : '是，保存为常用产能' }}</button>
      <button type="button" :disabled="presetSaving" @click="declinePreset">否，仅本次使用</button>
    </div>
    <p v-if="presetMessage" class="preset-message" role="status">{{ presetMessage }}</p>
    <p v-if="presetError" class="preset-error" role="alert">{{ presetError }}，本次自定义安排仍保留。<button type="button" :disabled="presetSaving || disabled" @click="savePreset">重试保存常用产能</button></p>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { apiSend } from '../api/client'
import { applyCapacityBatchSnapshot, capacityBatchRecommendation, capacitySplitInputError, plannedCapacitySplitMetrics, withCapacityBatchCount, withCustomCapacityBatchSize } from '../lib/produce-plan'

const props = defineProps({
  split: { type: Object, required: true }, rows: { type: Array, default: () => [] }, item: { type: Object, default: () => ({}) },
  capacities: { type: Array, default: () => [] }, disabled: { type: Boolean, default: false }, canSavePreset: { type: Boolean, default: false },
})
const emit = defineEmits(['update', 'preset-saved'])
const capacity = computed(() => props.capacities.find(row => Number(row.id) === Number(props.split.workstation_capacity_id)))
const isCustom = computed(() => props.split.custom_batch_size_qty != null)
const metrics = computed(() => plannedCapacitySplitMetrics(props.split))
const recommendation = computed(() => capacityBatchRecommendation(props.split, props.rows, props.item))
const presetSaving = ref(false)
const presetError = ref('')
const presetMessage = ref('')
const answeredPresets = ref(new Set())
const committedPreset = ref('')
const customEdited = ref(false)
const presetKey = computed(() => `${props.split.workstation_capacity_id}:${Number(props.split.custom_batch_size_qty)}`)
const showPresetPrompt = computed(() => !props.disabled && props.canSavePreset && isCustom.value && !!capacity.value && !capacitySplitInputError({ ...props.split, batch_input: false }) && committedPreset.value === presetKey.value && !answeredPresets.value.has(presetKey.value))
watch(presetKey, () => { presetError.value = ''; presetMessage.value = '' })
function quantity(value) { return Number(value || 0).toLocaleString('zh-CN', { maximumFractionDigits: 3 }) }
function variance(value, unit) { return value > 0 ? `超排 ${quantity(value)}${unit}` : value < 0 ? `不足 ${quantity(-value)}${unit}` : '刚好满足目标' }
function changeCustom(value, edited = true) { if (edited && String(value) !== String(props.split.custom_batch_size_qty)) customEdited.value = true; emit('update', withCustomCapacityBatchSize(props.split, value, props.rows, props.item, capacity.value)) }
function commitCustom() {
  if (customEdited.value && isCustom.value && !capacitySplitInputError({ ...props.split, batch_input: false })) committedPreset.value = presetKey.value
  customEdited.value = false
}
function changeBatch(value) { emit('update', withCapacityBatchCount(props.split, value, capacity.value)) }
function changeMode(mode) {
  committedPreset.value = ''; customEdited.value = false
  if (mode === 'custom') changeCustom(props.split.batch_size_qty || capacity.value?.batch_size_qty || '', false)
  else if (capacity.value) emit('update', applyCapacityBatchSnapshot({ ...props.split, custom_batch_size_qty: null }, capacity.value))
}
function useRecommendation() {
  if (recommendation.value?.batch_count > 0) changeBatch(recommendation.value.batch_count)
}
function declinePreset() { answeredPresets.value.add(presetKey.value); presetError.value = '' }
async function savePreset() {
  if (props.disabled || presetSaving.value || !props.canSavePreset || !capacity.value || !isCustom.value || capacitySplitInputError({ ...props.split, batch_input: false })) return
  const key = presetKey.value
  presetSaving.value = true
  presetError.value = ''
  try {
    const result = await apiSend(`/api/manufacturing-workstation-capacities/${props.split.workstation_capacity_id}/batch-presets`, { method: 'POST', body: { batch_size_qty: Number(props.split.custom_batch_size_qty) } })
    emit('preset-saved', result.capacity)
    answeredPresets.value.add(key)
    if (key === presetKey.value) {
      presetMessage.value = result.created ? '已保存为常用产能。本次安排保持不变，下次可直接选择。' : '已有相同常用产能，可在产能列表中选择。本次安排保持不变。'
    }
  } catch (err) { if (key === presetKey.value) presetError.value = err.message || '保存常用产能失败' }
  finally { presetSaving.value = false }
}
</script>

<style scoped>
.batch-editor{display:grid;grid-template-columns:minmax(170px,1fr) minmax(100px,.6fr) minmax(80px,.4fr);gap:10px;grid-column:1/-1;border-top:1px solid #e2eae5;padding-top:12px}.batch-editor label{display:grid;gap:4px;align-content:start}.batch-editor label>span{font-size:11px;color:#677772}.batch-editor select,.batch-editor input{width:100%;box-sizing:border-box;min-height:36px;border:1px solid #cfdad5;border-radius:7px;padding:6px 8px;background:#fff;color:#162f29;font:inherit}.batch-advice,.preset-prompt{grid-column:1/-1;display:flex;flex-wrap:wrap;align-items:center;gap:7px;font-size:12px;line-height:1.6;color:#52645e}.batch-advice small{flex-basis:100%;color:#677772}.batch-editor button{border:1px solid #b9d7c5;border-radius:6px;background:#f0f8f3;color:#14653e;padding:5px 9px;font-size:12px;cursor:pointer}.batch-editor button:disabled{opacity:.45;cursor:not-allowed}.preset-prompt{padding:10px;background:#f3f7f4;border-radius:7px}.preset-prompt>span{flex-basis:100%}.preset-message,.preset-error{grid-column:1/-1;margin:0;font-size:12px;line-height:1.6}.preset-message{color:#14653e}.preset-error{color:#a54022}.preset-error button{margin-left:8px}@media(max-width:600px){.batch-editor{grid-template-columns:1fr}.batch-editor label{grid-column:1}}
</style>
