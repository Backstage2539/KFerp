<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head"><h2>工序卡</h2><button class="secondary" @click="load" :disabled="loading">刷新</button></div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters"><label><span>状态</span><select v-model="status"><option value="">全部</option><option value="running">running</option><option value="completed">completed</option><option value="cancelled">cancelled</option></select></label><button class="primary" @click="load">查询</button></div>
    </section>
    <section class="panel table-wrap">
      <table>
        <thead><tr><th>ID</th><th>工单</th><th>顺序</th><th>工序</th><th>工位</th><th>状态</th><th>计划投入</th><th>实际投入</th><th>实际产出</th><th>实际损耗</th><th>损耗率</th><th>异常原因</th><th>操作人</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td>#{{ row.id }}</td>
            <td>#{{ row.work_order_id }}<small>{{ row.started_at }} / {{ row.completed_at || '-' }}</small></td>
            <td>{{ row.sequence_no || 1 }}</td>
            <td>{{ row.operation }}<small v-if="row.records_loss">记录损耗</small></td>
            <td>{{ row.workstation || '-' }}</td>
            <td>{{ row.status }}</td>
            <td><input v-model.number="row.planned_input_qty" type="number" min="0" step="0.01" /></td>
            <td><input v-model.number="row.actual_input_qty" type="number" min="0" step="0.01" /></td>
            <td><input v-model.number="row.actual_output_qty" type="number" min="0" step="0.01" /></td>
            <td><input v-model.number="row.actual_loss_qty" type="number" min="0" step="0.01" /></td>
            <td>{{ percent(row.actual_loss_rate) }}</td>
            <td><input v-model.trim="row.exception_reason" placeholder="异常原因" /></td>
            <td>{{ row.operator }}</td>
            <td><button class="secondary compact" type="button" @click="saveMetrics(row)">保存</button></td>
          </tr>
          <tr v-if="!rows.length"><td colspan="14" class="muted">暂无工序卡</td></tr>
        </tbody>
      </table>
    </section>
  </div>
</template>
<script setup>
import { onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
const rows = ref([]), status = ref(''), loading = ref(false), error = ref('')
function percent(value){ const n=Number(value||0); return n ? `${(n*100).toFixed(2)}%` : '-' }
async function load(){ loading.value=true; error.value=''; try{ const url=new URL('/api/produce/job-cards', window.location.origin); if(status.value) url.searchParams.set('status', status.value); const data=await apiGet(url); rows.value=(data.rows||[]).map((row)=>({ ...row, planned_input_qty:Number(row.planned_input_qty||0), actual_input_qty:Number(row.actual_input_qty||0), actual_output_qty:Number(row.actual_output_qty||0), actual_loss_qty:Number(row.actual_loss_qty||0), actual_loss_rate:Number(row.actual_loss_rate||0) })) }catch(err){ error.value=err.message||'加载失败' }finally{ loading.value=false } }
async function saveMetrics(row){ loading.value=true; error.value=''; try{ const saved=await apiSend(`/api/produce/job-cards/${row.id}/metrics`, { body: { planned_input_qty:Number(row.planned_input_qty||0), actual_input_qty:Number(row.actual_input_qty||0), actual_output_qty:Number(row.actual_output_qty||0), actual_loss_qty:Number(row.actual_loss_qty||0), exception_reason:row.exception_reason||'', metrics_json:row.metrics_json||'{}' } }); Object.assign(row, saved) }catch(err){ error.value=err.message||'保存失败' }finally{ loading.value=false } }
onMounted(load)
</script>
<style scoped>
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #eee;border-radius:8px;padding:12px;background:#fff}.panel-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:12px}h2{margin:0;font-size:18px}.filters{display:grid;grid-template-columns:160px 90px;gap:10px;align-items:end}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,button{font:inherit;min-height:36px;border-radius:6px}select{width:100%;border:1px solid #ddd;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #999;background:#fff;color:#111}.table-wrap{overflow:auto}table{width:100%;min-width:1500px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px}th{background:#fbfbfb}.muted{color:#666;text-align:center}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}
td small{display:block;color:#777;margin-top:3px}td input{width:120px;min-height:32px;border:1px solid #ddd;border-radius:6px;padding:5px 7px;font:inherit}.compact{min-height:30px;padding:5px 10px}
</style>
