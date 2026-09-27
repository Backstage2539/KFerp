<template>
  <article class="creator-node" :class="[`kind-${data.module.kind}`, { selected }]" :aria-label="data.module.name">
    <Handle
      v-for="(port, index) in data.module.inputs"
      :id="port.id"
      :key="`in-${port.id}`"
      type="target"
      :position="Position.Left"
      :style="handleStyle(index, data.module.inputs.length)" />
    <Handle
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
      <span>{{ nodeSummary(data.module) }}</span>
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
      id="__prerequisite"
      type="source"
      :position="Position.Right"
      class="creator-node-prerequisite-handle"
      :style="{ top: '96%' }" />
  </article>
</template>

<script setup>
import { Handle, Position } from '@vue-flow/core'
import {
  IconBox,
  IconPackage,
  IconSitemap,
  IconRoute,
  IconSend,
  IconShoppingCart,
  IconTags,
} from '@tabler/icons-vue'

defineProps({
  id: { type: String, required: true },
  data: { type: Object, required: true },
  selected: { type: Boolean, default: false },
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

function nodeSummary(module) {
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
</script>
