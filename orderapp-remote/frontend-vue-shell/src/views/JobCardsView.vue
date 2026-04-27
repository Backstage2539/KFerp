<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head"><h2>工序卡</h2><button class="secondary" @click="load" :disabled="loading">刷新</button></div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters"><label><span>状态</span><select v-model="status"><option value="">全部</option><option value="running">running</option><option value="completed">completed</option><option value="cancelled">cancelled</option></select></label><button class="primary" @click="load">查询</button></div>
    </section>
    <section class="panel table-wrap">
      <table>
        <thead><tr><th>ID</th><th>工单</th><th>工序</th><th>工位</th><th>状态</th><th>开始</th><th>完成</th><th>操作人</th></tr></thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id"><td>#{{ row.id }}</td><td>#{{ row.work_order_id }}</td><td>{{ row.operation }}</td><td>{{ row.workstation }}</td><td>{{ row.status }}</td><td>{{ row.started_at }}</td><td>{{ row.completed_at || '-' }}</td><td>{{ row.operator }}</td></tr>
          <tr v-if="!rows.length"><td colspan="8" class="muted">暂无工序卡</td></tr>
        </tbody>
      </table>
    </section>
  </div>
</template>
<script setup>
import { onMounted, ref } from 'vue'
import { apiGet } from '../api/client'
const rows = ref([]), status = ref(''), loading = ref(false), error = ref('')
async function load(){ loading.value=true; error.value=''; try{ const url=new URL('/api/produce/job-cards', window.location.origin); if(status.value) url.searchParams.set('status', status.value); const data=await apiGet(url); rows.value=data.rows||[] }catch(err){ error.value=err.message||'加载失败' }finally{ loading.value=false } }
onMounted(load)
</script>
<style scoped>
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #eee;border-radius:8px;padding:12px;background:#fff}.panel-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:12px}h2{margin:0;font-size:18px}.filters{display:grid;grid-template-columns:160px 90px;gap:10px;align-items:end}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,button{font:inherit;min-height:36px;border-radius:6px}select{width:100%;border:1px solid #ddd;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #999;background:#fff;color:#111}.table-wrap{overflow:auto}table{width:100%;min-width:820px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px}th{background:#fbfbfb}.muted{color:#666;text-align:center}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}
</style>
