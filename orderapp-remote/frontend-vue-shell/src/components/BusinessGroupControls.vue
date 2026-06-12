<template>
  <div class="business-group-controls" data-business-group-controls>
    <button class="secondary compact-action" type="button" @click="$emit('manage')">{{ manageLabel }}</button>
    <label>
      <span>{{ templateLabel }}</span>
      <select :value="Number(modelValue || 0)" @change="$emit('update:modelValue', Number($event.target.value || 0))">
        <option :value="0">选择分组模板</option>
        <option v-for="option in templateOptions" :key="option.id" :value="Number(option.id || 0)">
          {{ option.label }}
        </option>
      </select>
    </label>
    <template v-if="selectedTemplate">
      <button class="secondary compact-action" type="button" :disabled="!canMove || loading" @click="$emit('move')">
        {{ moveLabel }}
      </button>
      <label>
        <span>目标分类</span>
        <select :value="Number(moveModelValue || 0)" :disabled="!canSelectTargetEffective || loading" @change="$emit('update:moveModelValue', Number($event.target.value || 0))">
          <option :value="0">未分类</option>
          <option v-for="option in moveOptions" :key="option.key || option.id" :value="Number(option.group_item_id || 0)">
            {{ option.label }}
          </option>
        </select>
      </label>
      <span class="muted left">已选 {{ selectedCount }} 个可移动对象</span>
    </template>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: { type: Number, default: 0 },
  moveModelValue: { type: Number, default: 0 },
  templateOptions: { type: Array, default: () => [] },
  moveOptions: { type: Array, default: () => [] },
  selectedTemplate: { type: Object, default: null },
  selectedCount: { type: Number, default: 0 },
  canMove: { type: Boolean, default: false },
  canSelectTarget: { type: Boolean, default: null },
  loading: { type: Boolean, default: false },
  templateLabel: { type: String, default: '选择分组模板' },
  manageLabel: { type: String, default: '前往分组模板' },
  moveLabel: { type: String, default: '移动到分类' },
})

const canSelectTargetEffective = computed(() => props.canSelectTarget === null ? props.canMove : props.canSelectTarget)

defineEmits(['update:modelValue', 'update:moveModelValue', 'manage', 'move'])
</script>
