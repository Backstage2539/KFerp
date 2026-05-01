<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head"><h2>原料批次</h2><button class="secondary" @click="load" :disabled="loading">刷新</button></div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label><span>搜索</span><input v-model.trim="q" placeholder="批次/物料/供应商" @keyup.enter="load" /></label>
        <label class="check"><input type="checkbox" v-model="activeOnly" /> 只看可用</label>
        <button class="primary" @click="load" :disabled="loading">查询</button>
      </div>
    </section>
    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead><tr><th>批次号</th><th>物料</th><th>供应商</th><th>入库单</th><th>入库(g)</th><th>剩余(g)</th><th>成本</th><th>库存状态</th><th>质检</th><th>时间</th><th>备注</th></tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>{{ row.batch_code }}</td><td>{{ row.material_name }}</td><td>{{ row.supplier || '-' }}</td><td>#{{ row.receipt_id }}</td>
              <td>{{ row.qty_g }}</td><td>{{ row.remaining_g }}</td><td>{{ money(row.unit_cost) }}</td><td>{{ row.status }}</td><td><span class="quality-pill" :class="qualityClass(row.quality_status)">{{ qualityLabel(row.quality_status) }}</span></td><td>{{ row.received_at }}</td><td>{{ row.note }}</td>
            </tr>
            <tr v-if="!rows.length"><td colspan="11" class="muted">暂无原料批次</td></tr>
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
const activeOnly = ref(false)
const loading = ref(false)
const error = ref('')
const money = (v) => Number(v || 0).toFixed(2)
const qualityLabel = (status) => ({ pass: '通过', hold: '待处理', reject: '不通过', unchecked: '未检' }[status || 'unchecked'] || '未检')
const qualityClass = (status) => `quality-${status || 'unchecked'}`
async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/stock/material-batches', window.location.origin)
    if (q.value) url.searchParams.set('q', q.value)
    if (activeOnly.value) url.searchParams.set('active_only', '1')
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
.filters { display:grid; grid-template-columns:minmax(220px,1fr) 120px 90px; gap:10px; align-items:end; }
.check { align-self:center; font-size:14px; color:#333; }
label span { display:block; color:#666; font-size:12px; margin-bottom:5px; }
input, button { font:inherit; min-height:36px; border-radius:6px; }
input[type=text], input:not([type]) { width:100%; border:1px solid #ddd; padding:7px 9px; }
button { padding:8px 12px; cursor:pointer; }
.primary { border:1px solid #111; background:#111; color:#fff; }
.secondary { border:1px solid #999; background:#fff; color:#111; }
.table-wrap { overflow:auto; }
table { width:100%; min-width:1060px; border-collapse:collapse; }
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
