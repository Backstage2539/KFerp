<template>
  <div ref="root" class="searchable-select" :class="{ open, disabled }">
    <div class="select-control">
      <input
        :value="query"
        :placeholder="placeholder"
        :disabled="disabled"
        type="search"
        autocomplete="off"
        role="combobox"
        :aria-expanded="open"
        :aria-controls="listboxId"
        @focus="openMenu"
        @input="handleInput"
        @keydown="handleKeydown"
      />
      <button
        class="select-toggle"
        type="button"
        :disabled="disabled"
        aria-label="展开候选项"
        @mousedown.prevent
        @click="toggleMenu"
      >v</button>
    </div>
    <div v-if="open" :id="listboxId" class="select-menu" role="listbox">
      <button
        v-for="(option, index) in filteredOptions"
        :key="optionKey(option)"
        class="select-option"
        :class="{ selected: isSelected(option), active: index === activeIndex }"
        type="button"
        role="option"
        :aria-selected="isSelected(option)"
        @mousedown.prevent="choose(option)"
      >
        <strong>{{ labelOf(option) }}</strong>
        <small v-if="metaOf(option)">{{ metaOf(option) }}</small>
      </button>
      <div v-if="!filteredOptions.length" class="select-empty">{{ emptyText }}</div>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { defaultOptionLabel, filterSearchableOptions } from '../lib/searchable-select'

const props = defineProps({
  modelValue: { type: [Number, String], default: 0 },
  options: { type: Array, default: () => [] },
  optionLabel: { type: Function, default: defaultOptionLabel },
  optionMeta: { type: Function, default: () => '' },
  optionValue: { type: [Function, String], default: 'id' },
  placeholder: { type: String, default: '请选择' },
  emptyText: { type: String, default: '没有匹配项' },
  emptyValue: { type: [Number, String], default: 0 },
  maxOptions: { type: Number, default: 80 },
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'select'])
const root = ref(null)
const open = ref(false)
const query = ref('')
const activeIndex = ref(0)
const listboxId = `searchable-select-${Math.random().toString(36).slice(2)}`

const selectedOption = computed(() => {
  const current = String(props.modelValue ?? '')
  if (current === '' || current === String(props.emptyValue)) return null
  return (props.options || []).find((option) => String(valueOf(option)) === current) || null
})

const filteredOptions = computed(() => (
  filterSearchableOptions(props.options, query.value, labelOf).slice(0, props.maxOptions)
))

function labelOf(option) {
  return String(props.optionLabel(option) || '').trim()
}

function metaOf(option) {
  return String(props.optionMeta(option) || '').trim()
}

function valueOf(option) {
  if (typeof props.optionValue === 'function') return props.optionValue(option)
  return option?.[props.optionValue]
}

function optionKey(option) {
  return String(valueOf(option) ?? labelOf(option))
}

function isSelected(option) {
  return String(valueOf(option)) === String(props.modelValue)
}

function syncQueryToSelection() {
  query.value = selectedOption.value ? labelOf(selectedOption.value) : ''
}

function openMenu(event) {
  if (props.disabled) return
  open.value = true
  if (event?.target?.select) event.target.select()
}

function closeMenu() {
  open.value = false
  syncQueryToSelection()
}

function toggleMenu() {
  if (props.disabled) return
  if (open.value) {
    closeMenu()
    return
  }
  if (selectedOption.value && query.value === labelOf(selectedOption.value)) query.value = ''
  open.value = true
}

function handleInput(event) {
  query.value = event.target.value
  open.value = true
  const selectedLabel = selectedOption.value ? labelOf(selectedOption.value) : ''
  if (selectedOption.value && query.value !== selectedLabel) {
    emit('update:modelValue', props.emptyValue)
  }
}

function choose(option) {
  const value = valueOf(option)
  query.value = labelOf(option)
  open.value = false
  emit('update:modelValue', value)
  emit('select', option)
}

function handleKeydown(event) {
  if (event.key === 'Escape') {
    closeMenu()
    return
  }
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    open.value = true
    activeIndex.value = Math.min(activeIndex.value + 1, Math.max(filteredOptions.value.length - 1, 0))
    return
  }
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    open.value = true
    activeIndex.value = Math.max(activeIndex.value - 1, 0)
    return
  }
  if (event.key === 'Enter' && open.value && filteredOptions.value[activeIndex.value]) {
    event.preventDefault()
    choose(filteredOptions.value[activeIndex.value])
  }
}

function handleOutsidePointer(event) {
  if (!root.value || root.value.contains(event.target)) return
  closeMenu()
}

watch(selectedOption, () => {
  if (!open.value) syncQueryToSelection()
}, { immediate: true })

watch(filteredOptions, () => {
  activeIndex.value = filteredOptions.value.length ? 0 : -1
})

onMounted(() => {
  document.addEventListener('pointerdown', handleOutsidePointer)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handleOutsidePointer)
})
</script>

<style scoped>
.searchable-select { position: relative; width: 100%; }
.select-control { position: relative; display: flex; align-items: stretch; width: 100%; }
.select-control input {
  width: 100%;
  height: 38px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  padding: 7px 34px 7px 9px;
  font: inherit;
  background: #fff;
}
.select-toggle {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 30px;
  height: 30px;
  min-height: 30px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: #374151;
  cursor: pointer;
  font: inherit;
  line-height: 1;
}
.select-toggle:hover { background: #f3f4f6; }
.select-menu {
  position: absolute;
  z-index: 30;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  max-height: 280px;
  overflow: auto;
  border: 1px solid #d1d5db;
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 14px 30px rgba(15, 23, 42, .16);
  padding: 6px;
}
.select-option {
  display: grid;
  gap: 2px;
  width: 100%;
  min-height: 38px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #111827;
  padding: 8px;
  text-align: left;
  cursor: pointer;
  font: inherit;
}
.select-option small { color: #6b7280; font-size: 12px; }
.select-option:hover, .select-option.active { background: #f3f6fb; }
.select-option.selected { background: #eef5ff; color: #174ea6; }
.select-empty { padding: 12px; color: #6b7280; font-size: 13px; }
.disabled { opacity: .7; }
</style>
