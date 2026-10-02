<template>
  <div class="pc-simple-name-editor">
    <div class="pc-name-add-actions">
      <button type="button" aria-label="添加文本" @click="append('text')">＋ 文本</button>
      <button type="button" aria-label="添加变量" @click="append('variable')">＋ 变量</button>
    </div>
    <div v-for="(part, index) in parts" :key="index" class="pc-name-piece">
      <span class="pc-name-piece-label">{{ index + 1 }} · {{ part.type === 'variable' ? '变量' : '文本' }}</span>
      <input v-if="part.type === 'text'" :aria-label="`文本 ${index + 1}`" :value="part.value" placeholder="输入文本" @input="change(index, { value: $event.target.value })" />
      <div v-else class="pc-name-variable-field">
        <button v-if="part.variable_id && editing !== index" type="button" class="pc-name-selected-variable" @click="editing = index">{{ variableName(part.variable_id) }} <span>更换</span></button>
        <template v-else>
          <input :aria-label="`搜索变量 ${index + 1}`" :value="queries[index] || ''" placeholder="输入变量名，搜索或新增" @input="queries[index] = $event.target.value" />
          <div class="pc-name-variable-results">
            <button v-for="variable in matches(index)" :key="variable.id" type="button" :aria-label="`选择变量 ${variable.name}`" @click="choose(index, variable.id)">{{ variable.name }}</button>
            <button v-if="canCreate(index)" type="button" :aria-label="`新建变量 ${queries[index].trim()}`" @click="emit('create-variable', { name: queries[index].trim(), index }); editing = -1">＋ 新建“{{ queries[index].trim() }}”</button>
          </div>
        </template>
      </div>
      <button type="button" class="pc-name-remove" :aria-label="`移除片段 ${index + 1}`" @click="remove(index)">×</button>
    </div>
    <div class="pc-name-live-preview"><span>生成名称预览</span><strong aria-label="生成名称预览">{{ preview.value || '添加文本或变量后，这里显示完整名称' }}</strong></div>
    <details v-if="usedVariables.length" class="pc-name-samples">
      <summary>试填变量，查看名称效果</summary>
      <label v-for="variable in usedVariables" :key="variable.id"><span>{{ variable.name }} · 仅用于预览</span><input :value="samples[variable.id] ?? ''" :placeholder="variable.default_value || `填写${variable.name}示例值`" @input="emit('sample', variable.id, $event.target.value)" /></label>
    </details>
  </div>
</template>
<script setup>
import { computed, ref } from 'vue'
import { filterWorkflowVariables, renderNamePreview } from '../lib/product-creator-variables.js'
const props = defineProps({ parts: { type: Array, default: () => [] }, variables: { type: Array, default: () => [] }, samples: { type: Object, default: () => ({}) } })
const emit = defineEmits(['update', 'create-variable', 'sample'])
const queries = ref({})
const editing = ref(-1)
const preview = computed(() => renderNamePreview(props.parts, props.variables, props.samples))
const usedVariables = computed(() => props.variables.filter(v => props.parts.some(p => p.variable_id === v.id)))
const variableName = id => props.variables.find(v => v.id === id)?.name || '变量已失效，请更换'
const matches = index => filterWorkflowVariables(queries.value[index], props.variables)
const canCreate = index => queries.value[index]?.trim() && !props.variables.some(v => v.name.toLocaleLowerCase() === queries.value[index].trim().toLocaleLowerCase())
function append(type) { emit('update', [...props.parts, type === 'text' ? { type, value: '' } : { type, variable_id: '' }]) }
function change(index, patch) { emit('update', props.parts.map((p, i) => i === index ? { ...p, ...patch } : { ...p })) }
function choose(index, id) { change(index, { variable_id: id }); editing.value = -1 }
function remove(index) { emit('update', props.parts.filter((_, i) => i !== index)); queries.value = {}; editing.value = -1 }
</script>
<style scoped>
.pc-simple-name-editor { display: grid; gap: 12px; min-width: 0; }
.pc-name-add-actions { display: flex; gap: 12px; }
.pc-name-add-actions button, .pc-name-variable-results button { border: 1px solid #dce5ed; border-radius: 7px; padding: 8px 12px; color: #247950; background: #fff; cursor: pointer; font: inherit; }
.pc-name-piece { display: grid; grid-template-columns: minmax(0,1fr) 30px; gap: 6px; min-width: 0; }
.pc-name-piece-label { grid-column: 1 / -1; color: #6c7f95; font-size: 12px; }
input, .pc-name-selected-variable { box-sizing: border-box; width: 100%; min-width: 0; border: 1px solid #d8e2ed; border-radius: 7px; padding: 10px 12px; font: inherit; color: #203149; background: #fff; }
input:focus { outline: 2px solid #66ac88; outline-offset: 1px; }
.pc-name-selected-variable { display: flex; justify-content: space-between; gap: 8px; text-align: left; color: #287147; background: #f2f9f5; cursor: pointer; }
.pc-name-selected-variable span { font-size: 12px; white-space: nowrap; color: #748197; }
.pc-name-variable-field { min-width: 0; }
.pc-name-variable-results { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 6px; max-height: 160px; overflow-y: auto; }
.pc-name-remove { align-self: start; border: 0; padding: 9px 4px; color: #7a8a9d; font-size: 20px; background: none; cursor: pointer; }
.pc-name-live-preview { display: grid; gap: 6px; padding: 12px; border: 1px solid #dce9e1; border-radius: 8px; background: #f5faf7; overflow-wrap: anywhere; }
.pc-name-live-preview span, .pc-name-samples { color: #748197; font-size: 12px; }
.pc-name-live-preview strong { color: #2d6447; line-height: 1.6; }
.pc-name-samples summary { cursor: pointer; }
.pc-name-samples label { display: grid; gap: 5px; margin-top: 10px; }
</style>
