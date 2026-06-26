<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head"><h2>库存批次</h2><button class="secondary" @click="load" :disabled="loading">刷新</button></div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label><span>搜索</span><input v-model.trim="q" placeholder="批次/名称" @keyup.enter="loadPage(1)" /></label>
        <label><span>类型</span><select v-model="itemType"><option value="">全部</option><option value="material">物料</option><option value="finished_product">成品</option></select></label>
        <button class="primary" @click="loadPage(1)" :disabled="loading">查询</button>
      </div>
    </section>
    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead><tr><th>批次号</th><th>类型</th><th>名称</th><th>规格</th><th>来源</th><th>入库库存数量</th><th>剩余库存数量</th><th>成本</th><th>质检</th><th>操作人</th><th>时间</th></tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>{{ row.batch_code }}</td><td>{{ row.item_type }}</td><td>{{ row.item_name }}</td><td>{{ row.spec_g || '-' }}</td>
              <td>{{ row.source_doc_type }} #{{ row.source_doc_id }}</td><td>{{ quantityLabel(row, 'qty') }}</td><td>{{ quantityLabel(row, 'remaining') }}</td>
              <td>{{ money(row.unit_cost) }}</td><td><span class="quality-pill" :class="qualityClass(row.quality_status)">{{ qualityLabel(row.quality_status) }}</span></td><td>{{ row.operator }}</td><td>{{ row.created_at }}</td>
            </tr>
            <tr v-if="!rows.length"><td colspan="11" class="muted">暂无批次</td></tr>
          </tbody>
        </table>
      </div>
      <PaginationControls
        :page="page"
        :page-size="limit"
        :total="total"
        :disabled="loading"
        @change="handlePaginationChange"
      />
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiGet } from '../api/client'
import PaginationControls from '../components/PaginationControls.vue'
import { normalizePageSize, paginationFromApi } from '../lib/pagination'
const rows = ref([])
const q = ref('')
const itemType = ref('')
const loading = ref(false)
const error = ref('')
const page = ref(1)
const limit = ref(50)
const total = ref(0)
const money = (v) => Number(v || 0).toFixed(2)
const qualityLabel = (status) => ({ pass: '通过', hold: '待处理', reject: '不通过', unchecked: '未检' }[status || 'unchecked'] || '未检')
const qualityClass = (status) => `quality-${status || 'unchecked'}`
function quantityLabel(row, prefix) {
  const qtyUnits = Number(row?.[`${prefix}_units`] || 0)
  const qtyG = Number(row?.[`${prefix}_g`] || 0)
  if (qtyUnits && qtyG) return `${qtyUnits.toLocaleString('zh-CN')} 件 / ${qtyG.toLocaleString('zh-CN')}g`
  if (qtyUnits) return `${qtyUnits.toLocaleString('zh-CN')} 件`
  if (qtyG) return `${qtyG.toLocaleString('zh-CN')}g`
  return '-'
}
async function loadPage(nextPage) {
  page.value = Math.max(1, Number(nextPage || 1))
  await load()
}
async function handlePaginationChange({ page: nextPage, pageSize }) {
  limit.value = normalizePageSize(pageSize)
  await loadPage(nextPage)
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/stock/batches', window.location.origin)
    if (q.value) url.searchParams.set('q', q.value)
    if (itemType.value) url.searchParams.set('item_type', itemType.value)
    url.searchParams.set('page', String(page.value))
    url.searchParams.set('limit', String(limit.value))
    const data = await apiGet(url)
    const pagination = paginationFromApi(data)
    rows.value = data.rows || []
    page.value = pagination.page
    limit.value = pagination.pageSize
    total.value = pagination.total
  } catch (err) { error.value = err.message || '加载失败' } finally { loading.value = false }
}
onMounted(load)
</script>

<style scoped>
.page { padding:16px; display:grid; gap:16px; }
.panel { border:1px solid #eee; border-radius:8px; padding:12px; background:#fff; }
.panel-head { display:flex; justify-content:space-between; align-items:center; gap:12px; margin-bottom:12px; }
h2 { margin:0; font-size:18px; }
.filters { display:grid; grid-template-columns:minmax(220px,1fr) 140px 90px; gap:10px; align-items:end; }
label span { display:block; color:#666; font-size:12px; margin-bottom:5px; }
input, select, button { font:inherit; min-height:36px; border-radius:6px; }
input, select { width:100%; border:1px solid #ddd; padding:7px 9px; }
button { padding:8px 12px; cursor:pointer; }
.primary { border:1px solid #111; background:#111; color:#fff; }
.secondary { border:1px solid #999; background:#fff; color:#111; }
.table-wrap { overflow:auto; }
table { width:100%; min-width:1160px; border-collapse:collapse; }
th, td { border-bottom:1px solid #f0f0f0; padding:8px; text-align:left; font-size:13px; }
th { background:#fbfbfb; }
.quality-pill { display:inline-flex; border:1px solid #d1d5db; border-radius:999px; padding:2px 8px; background:#f9fafb; white-space:nowrap; }
.quality-pass { border-color:#bbf7d0; background:#f0fdf4; color:#166534; }
.quality-hold { border-color:#fde68a; background:#fffbeb; color:#92400e; }
.quality-reject { border-color:#fecaca; background:#fef2f2; color:#991b1b; }
.quality-unchecked { border-color:#d1d5db; background:#f9fafb; color:#4b5563; }
.muted { color:#666; text-align:center; }
.error { background:#ffecec; border:1px solid #ffb9b9; border-radius:8px; padding:10px; }
@media (max-width:900px){ .page{padding:12px;} .filters{grid-template-columns:1fr;} }
</style>
