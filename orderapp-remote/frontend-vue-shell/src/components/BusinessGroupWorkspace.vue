<template>
  <div class="business-group-workspace" data-business-group-workspace>
    <aside class="business-group-category-panel" aria-label="分类结构">
      <div class="business-group-category-head">
        <strong>分类结构</strong>
        <small v-if="!moveActive">点击分类浏览右侧列表</small>
      </div>
      <div v-if="moveActive" class="business-group-move-prompt" role="status">
        <strong>请选择要移动到的分类</strong>
        <small>点击大类、小类或未分类后立即移动，不再二次确认。</small>
      </div>
      <div ref="treeScroll" class="business-group-tree-scroll">
        <div class="business-group-category-tree" role="tree">
          <div
            v-for="node in visibleNodes"
            :key="node.key"
            :class="[
              'business-group-tree-node',
              `business-group-tree-node-${node.kind}`,
              {
                active: !moveActive && node.key === selectedKey,
                targetable: moveActive && node.targetable,
                disabled: moveActive && !node.targetable,
              },
            ]"
            :style="{ '--business-group-tree-depth': node.tree_depth }"
            role="treeitem"
            :aria-level="Number(node.tree_depth || 0) + 1"
            :aria-expanded="node.expandable ? isExpanded(node.key) : undefined"
            :aria-selected="!moveActive && node.key === selectedKey">
            <button
              v-if="node.expandable"
              class="business-group-tree-toggle"
              type="button"
              :aria-label="`${isExpanded(node.key) ? '收起' : '展开'}${node.label}`"
              @click.stop="toggleNode(node.key)">
              {{ isExpanded(node.key) ? '−' : '+' }}
            </button>
            <span v-else class="business-group-tree-toggle-placeholder"></span>
            <button
              class="business-group-tree-label"
              type="button"
              :disabled="loading || (moveActive && !node.targetable)"
              @click="selectNode(node)">
              <span>{{ node.label }}</span>
              <small>{{ node.count }} {{ countUnit }}</small>
            </button>
          </div>
        </div>
      </div>
      <footer class="business-group-category-footer">
        <button class="secondary compact-action" type="button" :disabled="loading || moveActive" @click="$emit('manage')">{{ manageLabel }}</button>
        <button class="secondary compact-action" type="button" :disabled="loading || moveActive" @click="$emit('configure')">{{ configureLabel }}</button>
      </footer>
    </aside>

    <section class="business-group-main-panel">
      <BusinessGroupControls
        :breadcrumb="breadcrumb"
        :selected-count="selectedCount"
        :can-move="canMove"
        :move-active="moveActive"
        :loading="loading"
        @move="$emit('move')"
        @cancel="$emit('cancel')">
        <template #extra-actions>
          <slot name="toolbar-extra" />
        </template>
      </BusinessGroupControls>
      <div
        :class="{ 'business-group-list-disabled': moveActive }"
        :aria-disabled="moveActive ? 'true' : 'false'"
        :inert="moveActive || undefined">
        <slot />
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import BusinessGroupControls from './BusinessGroupControls.vue'
import {
  beginBusinessGroupMoveState,
  businessGroupCategoryBreadcrumb,
  businessGroupCategoryTreeNodes,
  restoreBusinessGroupMoveState,
} from '../lib/business-grouping'

const props = defineProps({
  groups: { type: Array, default: () => [] },
  modelValue: { type: String, default: 'business-group-all' },
  moveActive: { type: Boolean, default: false },
  selectedCount: { type: Number, default: 0 },
  canMove: { type: Boolean, default: false },
  loading: { type: Boolean, default: false },
  countUnit: { type: String, default: '个' },
  allLabel: { type: String, default: '全部分类' },
  manageLabel: { type: String, default: '前往分组模板' },
  configureLabel: { type: String, default: '设置分组模板' },
})

const emit = defineEmits(['update:modelValue', 'move', 'cancel', 'target', 'manage', 'configure'])
const treeScroll = ref(null)
const expandedKeys = ref([])
const moveState = ref(null)
const initialized = ref(false)

const nodes = computed(() => businessGroupCategoryTreeNodes(props.groups, { allLabel: props.allLabel }))
const nodesByKey = computed(() => new Map(nodes.value.map((node) => [node.key, node])))
const selectedKey = computed(() => nodesByKey.value.has(props.modelValue) ? props.modelValue : 'business-group-all')
const breadcrumb = computed(() => businessGroupCategoryBreadcrumb(nodes.value, selectedKey.value))

const visibleNodes = computed(() => {
  const expanded = new Set(expandedKeys.value)
  const visible = []
  for (const node of nodes.value) {
    let parentKey = node.parent_key
    let show = true
    const visited = new Set()
    while (parentKey && !visited.has(parentKey)) {
      visited.add(parentKey)
      if (!expanded.has(parentKey)) {
        show = false
        break
      }
      parentKey = nodesByKey.value.get(parentKey)?.parent_key || ''
    }
    if (show) visible.push(node)
  }
  return visible
})

function isExpanded(key) {
  return expandedKeys.value.includes(key)
}

function toggleNode(key) {
  const next = new Set(expandedKeys.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expandedKeys.value = Array.from(next)
}

function selectNode(node) {
  if (props.loading) return
  if (props.moveActive) {
    if (!node.targetable) return
    emit('target', {
      key: node.key,
      label: node.label,
      group_id: Number(node.group_id || 0),
      group_item_id: Number(node.group_item_id || 0),
      unclassified: node.kind === 'unclassified',
    })
    return
  }
  emit('update:modelValue', node.key)
}

watch(nodes, (nextNodes) => {
  const validKeys = new Set(nextNodes.map((node) => node.key))
  const expandableKeys = nextNodes.filter((node) => node.expandable).map((node) => node.key)
  if (!initialized.value) {
    expandedKeys.value = expandableKeys
    initialized.value = true
  } else {
    expandedKeys.value = expandedKeys.value.filter((key) => validKeys.has(key))
    if (!expandedKeys.value.includes('business-group-all')) expandedKeys.value.unshift('business-group-all')
  }
  if (!validKeys.has(props.modelValue)) emit('update:modelValue', 'business-group-all')
}, { immediate: true })

watch(() => props.moveActive, async (active, previous) => {
  if (active && !previous) {
    moveState.value = beginBusinessGroupMoveState({
      expandedKeys: expandedKeys.value,
      selectedKey: selectedKey.value,
      scrollTop: Number(treeScroll.value?.scrollTop || 0),
    }, nodes.value)
    expandedKeys.value = [...moveState.value.expandedKeys]
    await nextTick()
    if (treeScroll.value) treeScroll.value.scrollTop = 0
    return
  }
  if (!active && previous && moveState.value?.snapshot) {
    const restored = restoreBusinessGroupMoveState(moveState.value)
    moveState.value = restored
    expandedKeys.value = [...restored.expandedKeys]
    emit('update:modelValue', restored.selectedKey)
    await nextTick()
    if (treeScroll.value) treeScroll.value.scrollTop = restored.scrollTop
    moveState.value = null
  }
})
</script>

<style scoped>
.business-group-workspace { display: grid; grid-template-columns: minmax(210px, 260px) minmax(0, 1fr); gap: 12px; align-items: start; }
.business-group-category-panel { min-width: 0; max-height: min(720px, calc(100vh - 220px)); display: flex; flex-direction: column; overflow: hidden; border: 1px solid #d8e2ee; border-radius: 8px; background: #f8fafc; }
.business-group-category-head { display: grid; gap: 2px; padding: 11px 12px; border-bottom: 1px solid #d8e2ee; color: #334155; background: #edf3f9; }
.business-group-category-head small { color: #64748b; }
.business-group-move-prompt { display: grid; gap: 3px; padding: 10px 12px; border-bottom: 1px solid #bfdbfe; color: #1e3a5f; background: #eff6ff; }
.business-group-move-prompt small { line-height: 1.4; }
.business-group-tree-scroll { min-height: 180px; max-height: min(610px, calc(100vh - 340px)); overflow-y: auto; overscroll-behavior: contain; padding: 6px; }
.business-group-category-tree { display: grid; gap: 3px; }
.business-group-tree-node { display: grid; grid-template-columns: 24px minmax(0, 1fr); align-items: center; padding-left: calc(var(--business-group-tree-depth, 0) * 14px); border-radius: 6px; color: #334155; }
.business-group-tree-node-template { background: #e7eef7; color: #243b53; font-weight: 700; }
.business-group-tree-node-category { background: #f0f5fa; }
.business-group-tree-node-unclassified { background: #f5f7fa; }
.business-group-tree-node-all { background: #dde8f3; color: #1f3349; font-weight: 700; }
.business-group-tree-node.active { box-shadow: inset 0 0 0 2px #4f83b6; background: #dbeafe; }
.business-group-tree-node.targetable { box-shadow: inset 0 0 0 1px #7ba7d1; background: #eaf4ff; }
.business-group-tree-node.disabled { opacity: .52; }
.business-group-tree-toggle, .business-group-tree-toggle-placeholder { width: 24px; height: 30px; display: grid; place-items: center; padding: 0; border: 0; background: transparent; color: #315b82; }
.business-group-tree-label { min-width: 0; min-height: 34px; display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 5px 8px; border: 0; background: transparent; color: inherit; text-align: left; }
.business-group-tree-label span { min-width: 0; overflow-wrap: anywhere; }
.business-group-tree-label small { flex: 0 0 auto; color: #64748b; font-weight: 400; white-space: nowrap; }
.business-group-category-footer { position: sticky; bottom: 0; display: grid; grid-template-columns: 1fr; gap: 7px; margin-top: auto; padding: 9px; border-top: 1px solid #d8e2ee; background: #f8fafc; }
.business-group-main-panel { min-width: 0; display: grid; gap: 10px; }
.business-group-list-disabled { opacity: .42; filter: grayscale(.35); pointer-events: none; user-select: none; }
@media (max-width: 980px) {
  .business-group-workspace { grid-template-columns: 1fr; }
  .business-group-category-panel { max-height: none; }
  .business-group-tree-scroll { max-height: 320px; }
  .business-group-category-footer { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
