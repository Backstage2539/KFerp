<script setup>
import { ref, watch } from 'vue'
import { logisticsWeightGrams } from '../lib/logistics-weight.js'
const props = defineProps({ modelValue: { type: [Number, String], default: null }, inventoryUnit: { type: String, default: '' }, disabled: Boolean })
const emit = defineEmits(['update:modelValue'])
const unit = ref('kg')
const error = ref('')
const displayedQuantity = () => props.modelValue == null || props.modelValue === '' ? '' : Number(props.modelValue) / (unit.value === 'kg' ? 1000 : 1)
const quantity = ref(displayedQuantity())
let lastEmittedValue
watch(() => props.modelValue, value => {
  if (Object.is(value, lastEmittedValue)) { lastEmittedValue = undefined; return }
  quantity.value = displayedQuantity()
})
function changeUnit(value) {
  unit.value = value
  quantity.value = displayedQuantity()
  error.value = ''
}
function updateQuantity(value) {
  error.value = ''
  quantity.value = value
  try {
    lastEmittedValue = logisticsWeightGrams({ logistics_weight_qty: value, logistics_weight_unit: unit.value })
    emit('update:modelValue', lastEmittedValue)
  } catch (cause) {
    error.value = cause.message
    lastEmittedValue = null
    emit('update:modelValue', null)
  }
}
</script>
<template>
  <label class="logistics-weight-editor">
    <span>物流重量</span>
    <span class="logistics-weight-input"><span>1 {{ inventoryUnit || '库存单位' }} =</span><input :value="quantity" type="number" :min="unit === 'kg' ? 0.000000001 : 0.000001" :max="unit === 'kg' ? 1000000 : 1000000000" step="any" :disabled="disabled" placeholder="手工填写" aria-label="每库存单位物流重量" @input="updateQuantity($event.target.value)" /><select :value="unit" :disabled="disabled" aria-label="物流重量单位" @change="changeUnit($event.target.value)"><option value="kg">KG</option><option value="g">G</option></select></span>
    <small v-if="error" class="weight-error" role="alert">{{ error }}</small>
    <small v-else>每个商品规格发布前必填，用于运费核算。</small>
  </label>
</template>
<style scoped>
.logistics-weight-editor{display:grid;gap:5px;min-width:0}.logistics-weight-input{display:flex;align-items:center;gap:6px;min-width:0}.logistics-weight-input>span{flex-shrink:0}.logistics-weight-input input{min-width:70px;width:100%;flex:1}.logistics-weight-input select{width:70px;flex-shrink:0}.logistics-weight-editor small{font-size:12px;line-height:1.4;color:#687568}.logistics-weight-editor .weight-error{color:#b42318}
</style>
