<template>
  <div class="list-pagination-controls" data-pagination-controls="server">
    <div class="pagination-summary">
      共 {{ displayTotal }} 条 / {{ totalPages }} 页，当前第 {{ currentPage }} 页
    </div>
    <div class="pagination-actions">
      <button class="secondary" type="button" @click="go(currentPage - 1)" :disabled="disabled || currentPage <= 1">上一页</button>
      <label>
        <span>跳至</span>
        <input v-model.trim="jumpValue" type="number" min="1" :max="totalPages" @keyup.enter="jump" />
      </label>
      <button class="secondary" type="button" @click="jump" :disabled="disabled">跳转</button>
      <label>
        <span>每页</span>
        <select :value="normalizedPageSize" :disabled="disabled" @change="changePageSize">
          <option v-for="option in pageSizeOptions" :key="option" :value="option">{{ option }} 条</option>
        </select>
      </label>
      <button class="secondary" type="button" @click="go(currentPage + 1)" :disabled="disabled || currentPage >= totalPages">下一页</button>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { clampPage, normalizePageSize, pageCount, PAGE_SIZE_OPTIONS } from '../lib/pagination'

const props = defineProps({
  page: { type: Number, default: 1 },
  pageSize: { type: Number, default: 10 },
  total: { type: Number, default: 0 },
  disabled: { type: Boolean, default: false },
  pageSizeOptions: { type: Array, default: () => PAGE_SIZE_OPTIONS },
})

const emit = defineEmits(['update:page', 'update:pageSize', 'change'])
const jumpValue = ref(String(props.page || 1))

const displayTotal = computed(() => Math.max(0, Number(props.total || 0)))
const normalizedPageSize = computed(() => normalizePageSize(props.pageSize, props.pageSizeOptions))
const totalPages = computed(() => pageCount(displayTotal.value, normalizedPageSize.value))
const currentPage = computed(() => clampPage(props.page, displayTotal.value, normalizedPageSize.value))

watch(currentPage, (value) => {
  jumpValue.value = String(value)
}, { immediate: true })

function emitChange(page, pageSize = normalizedPageSize.value) {
  const nextPage = clampPage(page, displayTotal.value, pageSize)
  emit('update:page', nextPage)
  emit('change', { page: nextPage, pageSize })
}

function go(page) {
  emitChange(page)
}

function jump() {
  emitChange(Number(jumpValue.value || 1))
}

function changePageSize(event) {
  const nextPageSize = normalizePageSize(event?.target?.value, props.pageSizeOptions)
  emit('update:pageSize', nextPageSize)
  emitChange(1, nextPageSize)
}
</script>

<style scoped>
.list-pagination-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-top: 12px;
  color: #333;
}
.pagination-summary { font-size: 13px; color: #5f6368; }
.pagination-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #666;
  font-size: 13px;
}
input, select {
  height: 34px;
  border: 1px solid #cfc8bf;
  border-radius: 6px;
  padding: 6px 8px;
  font: inherit;
  background: #fff;
}
input { width: 72px; }
button {
  min-height: 34px;
  border-radius: 6px;
  border: 1px solid #999;
  padding: 6px 10px;
  font: inherit;
  background: #fff;
  color: #1f1f1f;
  cursor: pointer;
}
button:disabled { cursor: not-allowed; opacity: .55; }
@media (max-width: 700px) {
  .list-pagination-controls { align-items: flex-start; flex-direction: column; }
  .pagination-actions { width: 100%; }
}
</style>
