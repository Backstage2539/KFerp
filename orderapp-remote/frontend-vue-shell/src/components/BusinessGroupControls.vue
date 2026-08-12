<template>
  <div class="business-group-controls" data-business-group-controls>
    <nav class="business-group-breadcrumb" aria-label="当前分类">{{ breadcrumb || '全部分类' }}</nav>
    <div class="business-group-control-actions">
      <button
        :class="moveActive ? 'secondary compact-action' : 'primary compact-action'"
        type="button"
        :disabled="loading || (!moveActive && !canMove)"
        @click="$emit(moveActive ? 'cancel' : 'move')">
        {{ moveActive ? '取消移动' : moveLabel }}
      </button>
      <span class="muted left">已选 {{ selectedCount }} 个可移动对象</span>
      <slot name="extra-actions" />
    </div>
  </div>
</template>

<script setup>
defineProps({
  breadcrumb: { type: String, default: '全部分类' },
  selectedCount: { type: Number, default: 0 },
  canMove: { type: Boolean, default: false },
  moveActive: { type: Boolean, default: false },
  loading: { type: Boolean, default: false },
  moveLabel: { type: String, default: '移动到分类' },
})

defineEmits(['move', 'cancel'])
</script>

<style scoped>
.business-group-controls { display: grid; gap: 8px; padding-bottom: 10px; border-bottom: 1px solid #e2e8f0; }
.business-group-breadcrumb { color: #475569; font-size: 13px; line-height: 1.5; overflow-wrap: anywhere; }
.business-group-control-actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.muted { color: #64748b; }
.left { text-align: left; }
</style>
