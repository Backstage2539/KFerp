<template>
  <article class="creator-node" :class="[`kind-${data.module.kind}`, { selected }]" :aria-label="data.module.name">
    <Handle
      v-for="(port, index) in inputPorts"
      :id="port.id"
      :key="`in-${port.id}`"
      type="target"
      :position="Position.Left"
      :style="handleStyle(index, inputPorts.length)" />
    <span
      v-for="port in recipePorts"
      :key="`label-${port.id}`"
      class="creator-node-input-label"
      :style="recipePortStyle(port.id)">{{ port.label }}</span>
    <Handle
      v-if="Number(data.module.workflow_version || 1) < 2"
      id="__prerequisite"
      type="target"
      :position="Position.Left"
      class="creator-node-prerequisite-handle"
      :style="{ top: '96%' }" />
    <div class="creator-node-icon">
      <component :is="iconFor(data.module.kind)" :size="23" stroke-width="1.8" />
    </div>
    <div class="creator-node-copy">
      <strong>{{ data.label || data.module.name }}</strong>
      <span>{{ nodeSummary(data.module, data.config) }}</span>
      <small v-if="data.countLabel">{{ data.countLabel }}</small>
      <small v-else-if="data.module.kind === 'material'" class="creator-node-tags">外购 · 自制</small>
    </div>
    <span class="creator-node-more" aria-hidden="true">···</span>
    <Handle
      v-for="(port, index) in data.module.outputs"
      :id="port.id"
      :key="`out-${port.id}`"
      type="source"
      :position="Position.Right"
      :style="handleStyle(index, data.module.outputs.length)" />
    <Handle
      v-if="Number(data.module.workflow_version || 1) < 2"
      id="__prerequisite"
      type="source"
      :position="Position.Right"
      class="creator-node-prerequisite-handle"
      :style="{ top: '96%' }" />
  </article>
</template>

<script setup>
import { Handle, Position } from '@vue-flow/core'
import { computed } from 'vue'
import {
  IconBox,
  IconPackage,
  IconSitemap,
  IconRoute,
  IconSend,
  IconShoppingCart,
  IconTags,
} from '@tabler/icons-vue'

const props = defineProps({
  id: { type: String, required: true },
  data: { type: Object, required: true },
  selected: { type: Boolean, default: false },
})

const recipePorts = computed(() => (props.data.recipeInputs || []).filter((port) => !port.add))
const inputPorts = computed(() => {
  const moduleInputs = props.data.module.inputs || []
  if (Number(props.data.module.workflow_version || 1) < 3 || props.data.module.kind !== 'bom') return moduleInputs
  return [...moduleInputs.filter((port) => port.id !== 'components'), ...recipePorts.value, ...(props.data.recipeInputs || []).filter((port) => port.add)]
})

const icons = {
  product: IconBox,
  material: IconPackage,
  bom: IconSitemap,
  process: IconRoute,
  publish: IconSend,
  purchase: IconShoppingCart,
  pricing: IconTags,
}

function iconFor(kind) {
  return icons[kind] || IconBox
}

function nodeSummary(module, config = {}) {
  if (Number(module.workflow_version || 1) >= 2) {
    const role = config.data_role
    if (module.kind === 'material') return role === 'output' ? 'BOM产出 · 自动生成物料' : '配方物料入口 · 选择或新建'
    if (module.kind === 'product') return role === 'output' ? 'BOM产出 · 商品与多规格' : '商品规格入口 · 引用已发布规格'
    if (module.kind === 'bom') return `BOM组装 · ${config.output_type === 'product' ? '产出商品' : '产出物料'}`
    if (module.kind === 'process') return '选择有效工艺路线'
    if (module.kind === 'purchase') return '物料购入 · 采购单与后续收货'
  }
  const summaries = {
    product: '新建 / 选择已有',
    material: '原料与自制半成品',
    bom: '关联产出 · 配方 · 多规格',
    process: '选择有效工艺路线',
    publish: '发布 BOM · 设置默认',
    purchase: '创建采购单 · 到货收货',
    pricing: '试算 · 保存草稿 · 发布',
  }
  return summaries[module.kind] || module.description
}

function handleStyle(index, count) {
  return { top: `${((index + 1) / (count + 1)) * 100}%` }
}

function recipePortStyle(portID) {
  const index = inputPorts.value.findIndex((port) => port.id === portID)
  return { top: `${((index + 1) / (inputPorts.value.length + 1)) * 100}%` }
}
</script>

<style scoped>
.creator-node {
  position: relative;
  display: flex;
  align-items: center;
  gap: 13px;
  box-sizing: border-box;
  width: 300px;
  min-height: 104px;
  border: 1px solid #dbe4ec;
  border-radius: 10px;
  padding: 16px 19px;
  color: #203149;
  background: #fff;
  box-shadow: 0 3px 12px #24364d0b;
  cursor: grab;
}

.creator-node:active { cursor: grabbing; }
.creator-node-input-label { position: absolute; left: -9px; z-index: 2; max-width: 112px; overflow: hidden; padding: 2px 5px; border-radius: 4px; color: #62738a; background: #f7f9fc; font-size: 9px; text-overflow: ellipsis; white-space: nowrap; transform: translate(-100%, -50%); pointer-events: none; }
.creator-node-icon {
  display: grid;
  flex: 0 0 40px;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 9px;
  color: #21804f;
  background: #e8f5ed;
}
.creator-node-copy { display: grid; flex: 1; min-width: 0; gap: 5px; }
.creator-node-copy strong {
  overflow: hidden;
  color: #203149;
  font-size: 14px;
  font-weight: 650;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.creator-node-copy span { overflow: hidden; color: #748197; font-size: 11px; line-height: 1.4; text-overflow: ellipsis; white-space: nowrap; }
.creator-node-copy small { color: #278052; font-size: 10px; }
.creator-node-copy .creator-node-tags { color: #8090a2; }
.creator-node-more { align-self: flex-start; color: #8795a6; font-size: 15px; letter-spacing: 1px; line-height: 1; }
.creator-node.kind-material .creator-node-icon { color: #3579ae; background: #eaf3fb; }
.creator-node.kind-bom .creator-node-icon { color: #7858a7; background: #f1ecf8; }
.creator-node.kind-process .creator-node-icon { color: #b27724; background: #fbf3e4; }
.creator-node.kind-publish .creator-node-icon { color: #2f8357; background: #e8f5ed; }
.creator-node.kind-purchase .creator-node-icon { color: #2876a7; background: #eaf3fa; }
.creator-node.kind-pricing .creator-node-icon { color: #b26548; background: #fbefea; }
@media (max-width: 720px) {
  .creator-node { width: 258px; min-height: 88px; gap: 9px; padding: 12px 14px; }
  .creator-node-icon { flex-basis: 34px; width: 34px; height: 34px; }
}
</style>
