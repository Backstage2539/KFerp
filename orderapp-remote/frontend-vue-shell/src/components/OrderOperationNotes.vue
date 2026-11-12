<template>
  <details class="operation-notes" :open="opened" @toggle="onToggle">
    <summary>工序备注<span v-if="notes.length"> · {{ notes.length }} 条</span></summary>
    <div class="notes-body">
      <p class="hint">选择需要说明的工序，备注会随订单传到对应工位。</p>
      <p v-if="contextMismatch && !validationError" class="warning">商品、规格或客户已变化，已填备注已保留；请核对后确认用于当前商品。</p>
      <p v-if="validationError" class="error" role="alert">{{ validationError }}</p>
      <p v-if="error" class="error" role="alert">{{ error }} <button type="button" @click="loadOptions" :disabled="loading">重试</button></p>
      <p v-else-if="loading" class="hint">正在读取可选工序…</p>
      <p v-else-if="!optionsUrl" class="hint">请先选择商品与规格。</p>
      <p v-else-if="loaded && !options.length" class="hint">当前商品暂无可选工序。</p>
      <div v-for="(note, index) in notes" :key="index" class="note-row">
        <label>
          <span>工序</span>
          <select :value="optionKey(note)" :disabled="disabled" @change="changeTarget(index, $event.target.value)">
            <option v-if="!findOption(note)" :value="optionKey(note)">原工序：{{ note.operation_name || '未记录' }}</option>
            <option v-for="option in options" :key="optionKey(option)" :value="optionKey(option)" :disabled="usedByOther(option, index)">{{ option.output_name }} · {{ option.operation_name }}</option>
          </select>
          <small v-if="loaded && !findOption(note)" class="warning">原工序不在当前商品的可选范围内，请重新选择；已填备注已保留。</small>
        </label>
        <label class="note-text">
          <span>备注</span>
          <textarea :value="note.note" :disabled="disabled" rows="2" maxlength="1000" required placeholder="填写该工序的要求" @input="updateNote(index, $event.target.value)"></textarea>
        </label>
        <button type="button" class="remove" :disabled="disabled" @click="removeNote(index)">删除</button>
      </div>
      <button type="button" class="add" :disabled="disabled || loading || !remainingOptions.length || notes.length >= 100" @click="addNote">添加工序备注</button>
      <button v-if="contextMismatch" type="button" :disabled="disabled || loading || !allNotesAvailable" @click="confirmContext">确认用于当前商品</button>
    </div>
  </details>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { apiGet } from '../api/client'

const props = defineProps({
  modelValue: { type: Array, default: undefined },
  optionsUrl: { type: String, default: '' },
  context: { type: String, default: '' },
  contextKey: { type: String, default: '' },
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'update:context'])
const notes = computed(() => Array.isArray(props.modelValue) ? props.modelValue : [])
const opened = ref(notes.value.length > 0)
const options = ref([])
const loading = ref(false)
const loaded = ref(false)
const error = ref('')
const validationError = ref('')
const currentContext = computed(() => props.contextKey || props.optionsUrl)
const contextMismatch = computed(() => notes.value.length > 0 && Boolean(props.context) && props.context !== currentContext.value)
let requestVersion = 0

function optionKey(option) {
  return [option.output_type, option.output_id, Number(option.bom_spec_id || 0), Number(option.bom_variant_id || 0), option.route_id, option.step_id, option.operation_id].join(':')
}
function findOption(note) { return options.value.find(option => optionKey(option) === optionKey(note)) }
function usedByOther(option, index) { return notes.value.some((note, i) => i !== index && optionKey(note) === optionKey(option)) }
const remainingOptions = computed(() => options.value.filter(option => !usedByOther(option, -1)))
const allNotesAvailable = computed(() => loaded.value && notes.value.every(note => findOption(note) && String(note.note || '').trim()))
function selectedNote(option, note = '') {
  return { output_type: option.output_type, output_id: option.output_id, bom_spec_id: Number(option.bom_spec_id || 0), bom_variant_id: Number(option.bom_variant_id || 0), route_id: option.route_id, step_id: option.step_id, operation_id: option.operation_id, operation_name: option.operation_name, note }
}
function addNote() {
  if (props.disabled || notes.value.length >= 100 || !remainingOptions.value.length) return
  if (!notes.value.length) emit('update:context', currentContext.value)
  emit('update:modelValue', [...notes.value, selectedNote(remainingOptions.value[0])])
}
function changeTarget(index, key) {
  const option = options.value.find(item => optionKey(item) === key)
  if (props.disabled || !option || usedByOther(option, index)) return
  emit('update:modelValue', notes.value.map((note, i) => i === index ? selectedNote(option, note.note) : { ...note }))
}
function updateNote(index, value) {
  if (props.disabled) return
  emit('update:modelValue', notes.value.map((note, i) => ({ ...note, ...(i === index ? { note: value } : {}) })))
}
function removeNote(index) {
  if (props.disabled) return
  emit('update:modelValue', notes.value.filter((_, i) => i !== index).map(note => ({ ...note })))
}
function confirmContext() {
  if (props.disabled || !allNotesAvailable.value) return
  emit('update:context', currentContext.value)
  validationError.value = ''
}
async function validate() {
  validationError.value = ''
  if (!notes.value.length) return true
  const validatingContext = currentContext.value
  const validatingURL = props.optionsUrl
  opened.value = true
  if (contextMismatch.value) {
    validationError.value = '商品、规格或客户已变化，请先核对工序备注并确认用于当前商品。'
    return false
  }
  await loadOptions()
  if (validatingContext !== currentContext.value || validatingURL !== props.optionsUrl || contextMismatch.value) {
    validationError.value = '商品、规格或客户已变化，请重新核对工序备注。'
    return false
  }
  if (!loaded.value || error.value) {
    validationError.value = '无法核验工序备注，请重试；已填内容已保留。'
    return false
  }
  if (!allNotesAvailable.value) {
    validationError.value = '请为每条备注选择当前商品的工序，并填写备注内容。'
    return false
  }
  return true
}
defineExpose({ validate })
async function loadOptions() {
  const version = ++requestVersion
  const url = props.optionsUrl
  error.value = ''
  loaded.value = false
  if (!url) { options.value = []; loading.value = false; return }
  loading.value = true
  try {
    const result = await apiGet(url)
    if (version !== requestVersion) return
    options.value = result.options || []
    loaded.value = true
  } catch (err) {
    if (version === requestVersion) error.value = err.message || '工序读取失败，请重试'
  } finally {
    if (version === requestVersion) loading.value = false
  }
}
function onToggle(event) {
  opened.value = event.target.open
  if (opened.value && !loaded.value && !loading.value) void loadOptions()
}
watch(() => props.optionsUrl, () => {
  ++requestVersion
  options.value = []
  loaded.value = false
  loading.value = false
  error.value = ''
  if (opened.value) void loadOptions()
})
onMounted(() => {
  if (notes.value.length && !props.context) emit('update:context', currentContext.value)
  if (opened.value) void loadOptions()
})
</script>

<style scoped>
.operation-notes { min-width: 0; border: 1px solid #d9e5df; border-radius: 8px; padding: 10px 12px; }
summary { cursor: pointer; color: #245d43; font-weight: 600; }
.notes-body { display: grid; gap: 12px; margin-top: 10px; }
.hint, .error { margin: 0; font-size: 13px; }
.hint { color: #60766a; }
.error, .warning { color: #a44522; }
.note-row { display: grid; grid-template-columns: minmax(160px, 1fr) minmax(180px, 2fr) auto; gap: 10px; align-items: start; }
label { display: grid; gap: 6px; min-width: 0; font-size: 14px; }
select, textarea { width: 100%; min-width: 0; box-sizing: border-box; border: 1px solid #c6d8ce; border-radius: 6px; padding: 8px; font: inherit; color: inherit; background: #fff; }
textarea { resize: vertical; }
button { width: fit-content; border: 1px solid #c6d8ce; border-radius: 6px; padding: 8px 12px; background: #fff; color: #246a45; cursor: pointer; }
button:disabled { opacity: .55; cursor: default; }
.remove { margin-top: 25px; }
@media (max-width: 680px) {
  .note-row { grid-template-columns: minmax(0, 1fr) auto; padding-top: 10px; border-top: 1px solid #e3ebe6; }
  .note-row > label { grid-column: 1 / -1; }
  .remove { margin: 0; grid-column: 2; }
}
</style>
