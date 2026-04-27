<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head"><h2>生产工单</h2><button class="secondary" @click="load" :disabled="loading">刷新</button></div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters"><label><span>状态</span><select v-model="status"><option value="">全部</option><option value="running">running</option><option value="completed">completed</option><option value="cancelled">cancelled</option></select></label><button class="primary" @click="load">查询</button></div>
    </section>
    <section class="panel table-wrap">
      <table>
        <thead><tr><th>工单号</th><th>批次</th><th>商品</th><th>规格</th><th>计划(g)</th><th>状态</th><th>实际成本</th><th>创建</th><th>完成</th></tr></thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id"><td>{{ row.work_order_no }}</td><td>{{ row.batch_id }}</td><td>{{ row.product_name }}</td><td>{{ row.spec_g }}</td><td>{{ row.planned_g }}</td><td>{{ row.status }}</td><td>{{ money(row.actual_cost) }}</td><td>{{ row.created_at }}</td><td>{{ row.completed_at || '-' }}</td></tr>
          <tr v-if="!rows.length"><td colspan="9" class="muted">暂无工单</td></tr>
        </tbody>
      </table>
    </section>
  </div>
</template>
<script setup>
import { onMounted, ref } from 'vue'
import { apiGet } from '../api/client'
const rows = ref([])
const status = ref('')
const loading = ref(false)
const error = ref('')
const money = (v) => Number(v || 0).toFixed(2)
async function load() {
  loading.value = true; error.value = ''
  try {
    const url = new URL('/api/produce/work-orders', window.location.origin)
    if (status.value) url.searchParams.set('status', status.value)
    const data = await apiGet(url)
    rows.value = data.rows || []
  } catch (err) { error.value = err.message || '加载失败' } finally { loading.value = false }
}
onMounted(load)
</script>
<style scoped>
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #eee;border-radius:8px;padding:12px;background:#fff}.panel-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:12px}h2{margin:0;font-size:18px}.filters{display:grid;grid-template-columns:160px 90px;gap:10px;align-items:end}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,button{font:inherit;min-height:36px;border-radius:6px}select{width:100%;border:1px solid #ddd;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #999;background:#fff;color:#111}.table-wrap{overflow:auto}table{width:100%;min-width:960px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px}th{background:#fbfbfb}.muted{color:#666;text-align:center}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}
</style>
