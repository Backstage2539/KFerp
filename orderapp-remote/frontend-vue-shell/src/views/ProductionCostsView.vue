<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head"><h2>生产成本</h2><button class="secondary" @click="load" :disabled="loading">刷新</button></div>
      <div v-if="error" class="error">{{ error }}</div>
    </section>
    <section class="panel table-wrap">
      <table>
        <thead><tr><th>时间</th><th>生产批次</th><th>商品</th><th>物料成本</th><th>工序成本</th><th>总成本</th><th>成品(g)</th><th>元/kg</th></tr></thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id"><td>{{ row.created_at }}</td><td>{{ row.batch_id }}</td><td>{{ row.product_name }}</td><td>{{ money(row.material_cost) }}</td><td>{{ money(row.operation_cost) }}</td><td>{{ money(row.total_cost) }}</td><td>{{ row.finished_g }}</td><td>{{ money(row.unit_cost_per_kg) }}</td></tr>
          <tr v-if="!rows.length"><td colspan="8" class="muted">暂无成本记录</td></tr>
        </tbody>
      </table>
    </section>
  </div>
</template>
<script setup>
import { onMounted, ref } from 'vue'
import { apiGet } from '../api/client'
const rows = ref([]), loading = ref(false), error = ref('')
const money = (v) => Number(v || 0).toFixed(2)
async function load(){ loading.value=true; error.value=''; try{ const data=await apiGet('/api/produce/costs'); rows.value=data.rows||[] }catch(err){ error.value=err.message||'加载失败' }finally{ loading.value=false } }
onMounted(load)
</script>
<style scoped>
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #eee;border-radius:8px;padding:12px;background:#fff}.panel-head{display:flex;justify-content:space-between;align-items:center;gap:12px}h2{margin:0;font-size:18px}button{font:inherit;min-height:36px;border-radius:6px;padding:8px 12px;cursor:pointer}.secondary{border:1px solid #999;background:#fff;color:#111}.table-wrap{overflow:auto}table{width:100%;min-width:860px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px}th{background:#fbfbfb}.muted{color:#666;text-align:center}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}
</style>
