<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head"><h2>库存批次</h2><button class="secondary" @click="load" :disabled="loading">刷新</button></div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label><span>搜索</span><input v-model.trim="q" placeholder="批次/名称" @keyup.enter="load" /></label>
        <label><span>类型</span><select v-model="itemType"><option value="">全部</option><option value="material">物料</option><option value="finished_product">成品</option></select></label>
        <button class="primary" @click="load" :disabled="loading">查询</button>
      </div>
    </section>
    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead><tr><th>批次号</th><th>类型</th><th>名称</th><th>规格</th><th>来源</th><th>数量(g)</th><th>剩余(g)</th><th>件数</th><th>成本</th><th>操作人</th><th>时间</th></tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>{{ row.batch_code }}</td><td>{{ row.item_type }}</td><td>{{ row.item_name }}</td><td>{{ row.spec_g || '-' }}</td>
              <td>{{ row.source_doc_type }} #{{ row.source_doc_id }}</td><td>{{ row.qty_g }}</td><td>{{ row.remaining_g }}</td><td>{{ row.qty_units }}</td>
              <td>{{ money(row.unit_cost) }}</td><td>{{ row.operator }}</td><td>{{ row.created_at }}</td>
            </tr>
            <tr v-if="!rows.length"><td colspan="11" class="muted">暂无批次</td></tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiGet } from '../api/client'
const rows = ref([])
const q = ref('')
const itemType = ref('')
const loading = ref(false)
const error = ref('')
const money = (v) => Number(v || 0).toFixed(2)
async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/stock/batches', window.location.origin)
    if (q.value) url.searchParams.set('q', q.value)
    if (itemType.value) url.searchParams.set('item_type', itemType.value)
    const data = await apiGet(url)
    rows.value = data.rows || []
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
table { width:100%; min-width:1080px; border-collapse:collapse; }
th, td { border-bottom:1px solid #f0f0f0; padding:8px; text-align:left; font-size:13px; }
th { background:#fbfbfb; }
.muted { color:#666; text-align:center; }
.error { background:#ffecec; border:1px solid #ffb9b9; border-radius:8px; padding:10px; }
@media (max-width:900px){ .page{padding:12px;} .filters{grid-template-columns:1fr;} }
</style>
