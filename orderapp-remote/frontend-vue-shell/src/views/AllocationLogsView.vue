<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>分配批次查看</h2>
        <div class="actions">
          <button class="secondary" type="button" @click="goPlan">返回需求汇总</button>
          <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="muted">汇总扣减留痕，按批次查看成品库存分配结果。</div>
    </section>

    <section class="grid">
      <div class="panel">
        <div class="section-title">批次列表</div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>批次</th>
                <th>条目数</th>
                <th>操作者</th>
                <th>时间</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="batch in batches"
                :key="batch.batch_id"
                :class="{ selected: batch.batch_id === batchId }"
                @click="selectBatch(batch.batch_id)">
                <td>{{ batch.batch_id }}</td>
                <td>{{ batch.items }}</td>
                <td>{{ batch.operator_name }}</td>
                <td>{{ batch.created_at }}</td>
              </tr>
              <tr v-if="!batches.length">
                <td colspan="4" class="muted">暂无批次</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="pager">
          <button class="secondary" type="button" :disabled="!hasPrev" @click="changePage(prevPage)">上一页</button>
          <span class="muted">第 {{ page }} 页</span>
          <button class="secondary" type="button" :disabled="!hasNext" @click="changePage(nextPage)">下一页</button>
        </div>
      </div>

      <div class="panel">
        <div class="section-title">批次明细</div>
        <div class="current">{{ batchId || '请选择一个批次' }}</div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>商品</th>
                <th>规格(g)</th>
                <th>需求(g)</th>
                <th>扣减(g)</th>
                <th>缺口(g)</th>
                <th>操作者</th>
                <th>时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in rows" :key="`${row.batch_id}-${row.product}-${row.spec_g}`">
                <td>{{ row.product }}</td>
                <td>{{ row.spec_g }}</td>
                <td>{{ row.need_g }}</td>
                <td>{{ row.deducted_g }}</td>
                <td><b>{{ row.gap_g }}</b></td>
                <td>{{ row.operator_name }}</td>
                <td>{{ row.created_at }}</td>
              </tr>
              <tr v-if="!rows.length">
                <td colspan="7" class="muted">暂无明细</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiGet } from '../api/client'
import { replaceHistoryURL } from '../lib/url-state'

const loading = ref(false)
const error = ref('')
const rows = ref([])
const batches = ref([])
const batchId = ref('')
const page = ref(1)
const perPage = ref(20)
const hasPrev = ref(false)
const hasNext = ref(false)
const prevPage = ref(0)
const nextPage = ref(2)

function applyUrlState() {
  const params = new URL(window.location.href).searchParams
  batchId.value = params.get('batch') || ''
  page.value = Number(params.get('page') || 1)
  if (!Number.isFinite(page.value) || page.value <= 0) page.value = 1
  perPage.value = Number(params.get('per_page') || 20)
  if (!Number.isFinite(perPage.value) || perPage.value <= 0) perPage.value = 20
}

function updateUrl() {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'allocationLogs')
  if (batchId.value) url.searchParams.set('batch', batchId.value)
  else url.searchParams.delete('batch')
  url.searchParams.set('page', String(page.value))
  url.searchParams.set('per_page', String(perPage.value))
  replaceHistoryURL(url)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/produce/allocations', window.location.origin)
    url.searchParams.set('page', String(page.value))
    url.searchParams.set('per_page', String(perPage.value))
    if (batchId.value) url.searchParams.set('batch', batchId.value)
    const data = await apiGet(url.toString())
    rows.value = data.rows || []
    batches.value = data.batches || []
    batchId.value = data.batch_id || ''
    page.value = Number(data.page || 1)
    perPage.value = Number(data.per_page || 20)
    hasPrev.value = Boolean(data.has_prev)
    hasNext.value = Boolean(data.has_next)
    prevPage.value = Number(data.prev_page || 0)
    nextPage.value = Number(data.next_page || page.value + 1)
    updateUrl()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function selectBatch(id) {
  batchId.value = id || ''
  load()
}

function changePage(next) {
  if (!next || next < 1) return
  page.value = next
  batchId.value = ''
  load()
}

function goPlan() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'producePlan' } }))
}

onMounted(() => {
  applyUrlState()
  load()
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; min-width: 0; }
.panel-head { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
.actions, .pager { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.grid { display: grid; grid-template-columns: minmax(360px, .9fr) minmax(520px, 1.4fr); gap: 14px; margin-top: 14px; align-items: start; }
.section-title { font-weight: 700; margin-bottom: 10px; }
.current { color: #444; margin-bottom: 10px; font-size: 13px; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 620px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; }
th { background: #fbfaf8; }
tbody tr { cursor: default; }
tbody tr.selected { background: #f2f7f3; }
button { height: 36px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.secondary { background: #fff; color: #1f1f1f; }
.muted { color: #666; font-size: 13px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; border-radius: 6px; padding: 9px; margin-bottom: 12px; color: #8a1f1f; }
@media (max-width: 1100px) {
  .grid { grid-template-columns: 1fr; }
}
@media (max-width: 900px) {
  .page { padding: 12px; }
  .panel-head { align-items: flex-start; flex-direction: column; }
  table { min-width: 760px; }
}
</style>
