<template>
  <div ref="workspaceScroll" class="business-group-inline-workspace" data-business-group-inline-workspace>
    <BusinessGroupControls
      :breadcrumb="allLabel"
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

    <div v-if="moveActive" class="business-group-inline-move-prompt" role="status">
      <strong>请选择要移动到的分类</strong>
      <small>点击分类标题立即移动，不再二次确认。全部分类和模板不能作为目标。</small>
    </div>

    <div
      class="business-group-inline-filters"
      :class="{ 'business-group-inline-disabled': moveActive }"
      :aria-disabled="moveActive ? 'true' : 'false'"
      :inert="moveActive || undefined">
      <slot name="filters" />
    </div>

    <div class="business-group-inline-sections">
      <section
        v-for="group in visibleGroups"
        :key="group.key"
        :class="[
          'business-group-inline-section',
          `business-group-inline-section-${groupKind(group)}`,
          { 'business-group-inline-section-collapsed': isCollapsed(group.key) },
        ]"
        :style="groupIndentStyle(group)">
        <header
          class="business-group-inline-heading"
          data-inline-group-header
          :class="{
            targetable: moveActive && isTargetable(group),
            disabled: moveActive && !isTargetable(group),
          }">
          <button
            class="business-group-inline-toggle"
            type="button"
            :disabled="loading || moveActive"
            :aria-label="`${isCollapsed(group.key) ? '展开' : '收起'}${group.label}`"
            @click.stop="toggleGroup(group.key)">
            {{ isCollapsed(group.key) ? '+' : '−' }}
          </button>
          <button
            class="business-group-inline-label"
            type="button"
            :disabled="loading || (moveActive && !isTargetable(group))"
            :title="group.path_label || group.label"
            @click="activateGroup(group)">
            <strong>{{ groupLabel(group) }}</strong>
            <small>{{ groupCount(group) }} {{ countUnit }}</small>
          </button>
        </header>

        <div
          v-if="!group.is_template_group && !isCollapsed(group.key)"
          class="business-group-inline-body"
          :class="{ 'business-group-inline-disabled': moveActive }"
          :aria-disabled="moveActive ? 'true' : 'false'"
          :inert="moveActive || undefined">
          <slot name="group" :group="group" />
        </div>
      </section>
    </div>

    <footer class="business-group-inline-footer">
      <button class="secondary compact-action" type="button" :disabled="loading || moveActive" @click="$emit('manage')">{{ manageLabel }}</button>
      <button class="secondary compact-action" type="button" :disabled="loading || moveActive" @click="$emit('configure')">{{ configureLabel }}</button>
    </footer>
  </div>
</template>

<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import BusinessGroupControls from './BusinessGroupControls.vue'
import { businessGroupHiddenByCollapsedAncestor } from '../lib/business-grouping.js'

const props = defineProps({
  groups: { type: Array, default: () => [] },
  collapsedKeys: { type: Array, default: () => [] },
  moveActive: { type: Boolean, default: false },
  selectedCount: { type: Number, default: 0 },
  canMove: { type: Boolean, default: false },
  loading: { type: Boolean, default: false },
  countUnit: { type: String, default: '个' },
  allLabel: { type: String, default: '全部分类' },
  manageLabel: { type: String, default: '前往分组模板' },
  configureLabel: { type: String, default: '设置分组模板' },
})

const emit = defineEmits(['update:collapsedKeys', 'move', 'cancel', 'target', 'manage', 'configure'])
const workspaceScroll = ref(null)
const moveSnapshot = ref(null)

const normalizedCollapsedKeys = computed(() => (Array.isArray(props.collapsedKeys) ? props.collapsedKeys : [])
  .map((key) => String(key || '').trim())
  .filter(Boolean))
const visibleGroups = computed(() => (Array.isArray(props.groups) ? props.groups : []).filter((group) => (
  !businessGroupHiddenByCollapsedAncestor(props.groups, group, normalizedCollapsedKeys.value)
)))

function isCollapsed(key) {
  return normalizedCollapsedKeys.value.includes(String(key || ''))
}

function toggleGroup(key) {
  if (props.loading || props.moveActive) return
  const normalizedKey = String(key || '')
  const next = new Set(normalizedCollapsedKeys.value)
  if (next.has(normalizedKey)) next.delete(normalizedKey)
  else next.add(normalizedKey)
  emit('update:collapsedKeys', Array.from(next))
}

function isTargetable(group = {}) {
  if (group?.is_template_group || group?.all) return false
  if (group?.unclassified) return true
  return Number(group?.group_id || 0) > 0 && Number(group?.group_item_id || 0) > 0
}

function activateGroup(group = {}) {
  if (props.loading) return
  if (props.moveActive) {
    if (!isTargetable(group)) return
    emit('target', {
      key: group.key,
      label: group.label,
      group_id: Number(group.group_id || 0),
      group_item_id: Number(group.group_item_id || 0),
      unclassified: Boolean(group.unclassified),
    })
    return
  }
  toggleGroup(group.key)
}

function groupKind(group = {}) {
  if (group?.all) return 'all'
  if (group?.is_template_group) return 'template'
  if (group?.unclassified) return 'unclassified'
  return 'category'
}

function groupLabel(group = {}) {
  if (group?.all) return props.allLabel
  return group?.label || group?.path_label || '未命名分类'
}

function groupCount(group = {}) {
  if (group?.is_template_group) return Math.max(0, Number(group?.template_total || group?.total || 0))
  return Math.max(0, Number(group?.total ?? group?.rows?.length ?? 0))
}

function groupIndentStyle(group = {}) {
  if (group?.all || group?.is_template_group || group?.unclassified) return { '--business-group-inline-depth': 0 }
  const hasTemplate = (Array.isArray(props.groups) ? props.groups : []).some((candidate) => (
    candidate?.is_template_group && Number(candidate?.group_id || 0) === Number(group?.group_id || 0)
  ))
  return { '--business-group-inline-depth': Math.max(0, Number(group?.depth || 0)) + (hasTemplate ? 1 : 0) }
}

watch(() => props.groups, (groups) => {
  if (props.moveActive) return
  const valid = new Set((Array.isArray(groups) ? groups : []).map((group) => String(group?.key || '')))
  const next = normalizedCollapsedKeys.value.filter((key) => valid.has(key))
  if (JSON.stringify(next) !== JSON.stringify(normalizedCollapsedKeys.value)) emit('update:collapsedKeys', next)
}, { deep: false })

watch(() => props.moveActive, async (active, previous) => {
  if (active && !previous) {
    moveSnapshot.value = {
      collapsedKeys: [...normalizedCollapsedKeys.value],
      scrollTop: Math.max(0, Number(workspaceScroll.value?.scrollTop || 0)),
    }
    emit('update:collapsedKeys', [])
    await nextTick()
    if (workspaceScroll.value) workspaceScroll.value.scrollTop = 0
    return
  }
  if (!active && previous && moveSnapshot.value) {
    const snapshot = moveSnapshot.value
    emit('update:collapsedKeys', [...snapshot.collapsedKeys])
    await nextTick()
    if (workspaceScroll.value) workspaceScroll.value.scrollTop = snapshot.scrollTop
    moveSnapshot.value = null
  }
})
</script>

<style scoped>
.business-group-inline-workspace { min-width: 0; display: grid; gap: 12px; }
.business-group-inline-move-prompt { display: grid; gap: 3px; padding: 10px 12px; border: 1px solid #bfdbfe; border-radius: 7px; color: #1e3a5f; background: #eff6ff; }
.business-group-inline-move-prompt small { line-height: 1.4; }
.business-group-inline-filters { min-width: 0; }
.business-group-inline-sections { min-width: 0; overflow: hidden; border: 1px solid #ded8d0; border-radius: 8px; background: #fff; }
.business-group-inline-section + .business-group-inline-section { border-top: 1px solid #e5e7eb; }
.business-group-inline-heading { min-height: 46px; display: grid; grid-template-columns: 36px minmax(0, 1fr); align-items: center; padding-left: calc(10px + var(--business-group-inline-depth, 0) * 24px); background: #f7f8fa; color: #263445; }
.business-group-inline-section-template > .business-group-inline-heading { min-height: 50px; background: #eaf0f7; color: #1f3349; }
.business-group-inline-section-all > .business-group-inline-heading { background: #edf3f9; }
.business-group-inline-section-unclassified > .business-group-inline-heading { background: #f4f6f8; }
.business-group-inline-heading.targetable { box-shadow: inset 0 0 0 2px #4f83b6; background: #eaf4ff; }
.business-group-inline-heading.disabled { opacity: .52; }
.business-group-inline-toggle { width: 30px; min-height: 30px; display: grid; place-items: center; padding: 0; border: 0; border-radius: 5px; background: transparent; color: #315b82; font-size: 18px; }
.business-group-inline-label { min-width: 0; min-height: 42px; display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 6px 12px 6px 4px; border: 0; background: transparent; color: inherit; text-align: left; }
.business-group-inline-label strong { min-width: 0; overflow-wrap: anywhere; }
.business-group-inline-label small { flex: 0 0 auto; color: #64748b; font-weight: 400; white-space: nowrap; }
.business-group-inline-body { min-width: 0; padding: 0 12px 12px; background: #fff; }
.business-group-inline-disabled { opacity: .42; filter: grayscale(.35); pointer-events: none; user-select: none; }
.business-group-inline-footer { display: flex; justify-content: flex-end; gap: 8px; flex-wrap: wrap; padding-top: 2px; }
@media (max-width: 760px) {
  .business-group-inline-body { padding-inline: 6px; }
  .business-group-inline-heading { padding-left: calc(4px + var(--business-group-inline-depth, 0) * 16px); }
  .business-group-inline-footer { justify-content: stretch; }
  .business-group-inline-footer button { flex: 1 1 180px; }
}
</style>
