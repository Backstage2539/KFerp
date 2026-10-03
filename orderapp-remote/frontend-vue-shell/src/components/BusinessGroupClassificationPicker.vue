<template>
  <div class="classification-picker">
    <input :value="query" aria-label="搜索分类" placeholder="搜索分类名称或完整路径" @input="query = $event.target.value" />
    <div class="classification-actions">
      <button type="button" aria-label="全部展开分类" @click="collapsedKeys = []">全部展开</button>
      <button type="button" aria-label="全部收缩分类" :disabled="Boolean(query.trim())" @click="collapsedKeys = allBranches">全部收缩</button>
    </div>
    <div class="classification-tree" role="group" aria-label="可选分类">
      <button class="classification-unassigned" type="button" :aria-pressed="!Number(value.classification_group_id)" @click="$emit('select', null)">未分类</button>
      <div v-for="option in options" :key="option.key" class="classification-row" :style="{ paddingLeft: `${option.depth * 20}px` }">
        <button v-if="option.hasChildren" class="classification-toggle" type="button" :aria-label="`${option.expanded ? '收起' : '展开'}${option.label}`" :aria-expanded="option.expanded" :disabled="Boolean(query.trim())" @click="toggle(option.key)">{{ option.expanded ? '▾' : '▸' }}</button>
        <span v-else class="classification-toggle" aria-hidden="true"></span>
        <button class="classification-select" type="button" :aria-label="`选择 ${option.path_label}`" :aria-pressed="Number(value.classification_group_id) === option.group_id && Number(value.classification_item_id || 0) === option.group_item_id" :title="option.path_label" @click="$emit('select', option)">{{ option.label }}</button>
      </div>
      <p v-if="!options.length">{{ query.trim() ? '没有匹配的分类' : '当前功能没有已启用的分类' }}</p>
    </div>
  </div>
</template>
<script setup>
import { computed, ref } from 'vue'
import { businessGroupClassificationTreeOptions } from '../lib/business-grouping.js'
const props = defineProps({ groups: {type:Array,default:()=>[]}, selectedTemplateIDs: {type:Array,default:()=>[]}, usageKey: {type:String,default:''}, value: {type:Object,default:()=>({})} })
defineEmits(['select'])
const query = ref('')
const collapsedKeys = ref([])
const base = computed(() => ({ selectedTemplateIDs: props.selectedTemplateIDs, usageKey: props.usageKey }))
const options = computed(() => businessGroupClassificationTreeOptions(props.groups, {...base.value, query: query.value, collapsedKeys: collapsedKeys.value}))
const allBranches = computed(() => businessGroupClassificationTreeOptions(props.groups, base.value).filter(row => row.hasChildren).map(row => row.key))
function toggle(key) { collapsedKeys.value = collapsedKeys.value.includes(key) ? collapsedKeys.value.filter(id => id !== key) : [...collapsedKeys.value, key] }
</script>
<style scoped>
.classification-picker { display:flex; flex-direction:column; gap:12px; min-height:0; }
.classification-picker input { box-sizing:border-box; width:100%; min-height:40px; padding:9px 12px; border:1px solid #d9e3ef; border-radius:8px; color:#26384f; font:inherit; }
.classification-picker input:focus { outline:2px solid #a8d3b8; outline-offset:1px; }
.classification-actions { display:flex; gap:8px; }
.classification-actions button { padding:6px 12px; border:1px solid #d9e3ef; border-radius:6px; background:#f7fafc; color:#38516b; font:inherit; cursor:pointer; }
.classification-tree { overflow:auto; border:1px solid #dce5ec; border-radius:8px; }
.classification-row { display:flex; align-items:center; min-height:44px; border-top:1px solid #e8edf3; background:#f9fbfd; }
.classification-toggle { display:inline-grid; place-items:center; width:32px; flex:0 0 32px; height:36px; border:0; background:transparent; color:#496a85; font-size:18px; cursor:pointer; }
.classification-select,.classification-unassigned { flex:1; min-width:0; padding:10px 12px; border:0; background:transparent; color:#26384f; font:inherit; text-align:left; cursor:pointer; overflow-wrap:anywhere; }
.classification-unassigned { width:100%; }
button[aria-pressed="true"] { background:#e8f5ed; color:#247448; font-weight:600; }
button:hover:not(:disabled) { background:#eff6f3; }
button:focus-visible { outline:2px solid #40875e; outline-offset:-2px; }
button:disabled { opacity:.5; cursor:default; }
.classification-tree p { padding:12px; color:#6b7d90; }
</style>
