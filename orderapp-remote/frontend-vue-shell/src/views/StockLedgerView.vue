<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>库存流水</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label><span>搜索</span><input v-model.trim="filters.q" placeholder="物料/商品/批次" @keyup.enter="loadPage(1)" /></label>
        <label><span>类型</span><select v-model="filters.item_type"><option value="">全部</option><option value="material">物料</option><option value="finished_product">成品</option></select></label>
        <label><span>来源</span><input v-model.trim="filters.source_doc_type" placeholder="production_run" @keyup.enter="loadPage(1)" /></label>
        <button class="primary" type="button" @click="loadPage(1)" :disabled="loading">查询</button>
      </div>
    </section>
    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead><tr><th>时间</th><th>类型</th><th>名称</th><th>规格</th><th>仓库</th><th>来源</th><th>批次</th><th>变化(g)</th><th>结余(g)</th><th>变化(件)</th><th>结余(件)</th><th>操作人</th></tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>{{ row.created_at }}</td><td>{{ itemTypeText(row.item_type) }}</td><td>{{ row.item_name }}</td><td>{{ row.spec_g || '-' }}</td><td>{{ row.warehouse }}</td>
              <td>{{ row.source_doc_type }} #{{ row.source_doc_id }}</td><td>{{ row.source_batch_code || row.source_batch_id || '-' }}</td>
              <td :class="{ neg: row.qty_change_g < 0, pos: row.qty_change_g > 0 }">{{ row.qty_change_g }}</td><td>{{ row.qty_after_g }}</td>
              <td>{{ row.qty_change_units }}</td><td>{{ row.qty_after_units }}</td><td>{{ row.operator }}</td>
            </tr>
            <tr v-if="!rows.length"><td colspan="12" class="muted">暂无流水</td></tr>
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
import { onMounted, reactive, ref } from 'vue'
import { apiGet } from '../api/client'
import PaginationControls from '../components/PaginationControls.vue'
import { normalizePageSize, paginationFromApi } from '../lib/pagination'

const rows = ref([])
const loading = ref(false)
const error = ref('')
const page = ref(1)
const limit = ref(50)
const total = ref(0)
const filters = reactive({ q: '', item_type: '', source_doc_type: '' })

function itemTypeText(v) {
  if (v === 'material') return '物料'
  if (v === 'finished_product') return '成品'
  return v || '-'
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
    const url = new URL('/api/stock/ledger', window.location.origin)
    for (const [key, val] of Object.entries(filters)) if (val) url.searchParams.set(key, val)
    url.searchParams.set('page', String(page.value))
    url.searchParams.set('limit', String(limit.value))
    const data = await apiGet(url)
    const pagination = paginationFromApi(data)
    rows.value = data.rows || []
    page.value = pagination.page
    limit.value = pagination.pageSize
    total.value = pagination.total
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.page { padding: 16px; display: grid; gap: 16px; }
.panel { border: 1px solid #eee; border-radius: 8px; padding: 12px; background: #fff; }
.panel-head { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 12px; }
h2 { margin: 0; font-size: 18px; }
.filters { display: grid; grid-template-columns: minmax(220px,1fr) 140px 160px 90px; gap: 10px; align-items: end; }
label span { display:block; color:#666; font-size:12px; margin-bottom:5px; }
input, select, button { font: inherit; min-height: 36px; border-radius: 6px; }
input, select { width:100%; border:1px solid #ddd; padding:7px 9px; }
button { padding: 8px 12px; cursor:pointer; }
.primary { border:1px solid #111; background:#111; color:#fff; }
.secondary { border:1px solid #999; background:#fff; color:#111; }
.table-wrap { overflow:auto; }
table { width:100%; min-width:1160px; border-collapse:collapse; }
th, td { border-bottom:1px solid #f0f0f0; padding:8px; text-align:left; font-size:13px; }
th { background:#fbfbfb; }
.muted { color:#666; text-align:center; }
.error { background:#ffecec; border:1px solid #ffb9b9; border-radius:8px; padding:10px; }
.neg { color:#b42318; } .pos { color:#067647; }
@media (max-width: 900px) { .page { padding:12px; } .filters { grid-template-columns:1fr; } }
</style>
