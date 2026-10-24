<template>
  <details class="operation-balances" @toggle="opened">
    <summary>工序中的物料与实际位置</summary>
    <p>这些数量正在生产中，不计入可发货商品库存。</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <button type="button" :disabled="busy" @click="load">刷新</button>
    <div class="scroll"><table><thead><tr><th>商品 / 工单</th><th>来源工序</th><th>实际位置</th><th>待后序使用</th><th>货主</th></tr></thead><tbody><tr v-for="row in visible" :key="row.id"><td>{{ row.product_name }}<small>{{ row.work_order_no }} · 批次 #{{ row.id }}</small></td><td>{{ row.operation }}</td><td>{{ row.warehouse_name }}</td><td>{{ row.remaining }} {{ row.unit }}</td><td>{{ row.owner_customer_name || (row.owner_customer_id ? `客户 #${row.owner_customer_id}` : '工厂自有') }}</td></tr></tbody></table></div>
    <p v-if="!busy && !visible.length">当前仓库没有待交接数量。</p>
  </details>
</template>
<script setup>
import { computed, ref } from 'vue'
import { apiGet } from '../api/client'
const props=defineProps({warehouse:{type:String,default:''}})
const rows=ref([]),error=ref(''),busy=ref(false),loaded=ref(false)
const visible=computed(()=>rows.value.filter(row=>!props.warehouse || row.warehouse===props.warehouse))
async function load(){busy.value=true;error.value='';try{const data=await apiGet('/api/production/process-balances');rows.value=data.rows||[];loaded.value=true}catch(e){error.value=e.message}finally{busy.value=false}}
function opened(event){if(event.target.open&&!loaded.value)load()}
</script>
<style scoped>
.operation-balances{margin:16px 0;padding:18px;background:#eef6f0;border:1px solid #cdded2;border-radius:12px}summary{cursor:pointer;font-weight:700}p,small{color:#62776a}small{display:block}.scroll{overflow-x:auto}table{width:100%;border-collapse:collapse;text-align:left}th,td{padding:12px;border-bottom:1px solid #d6e4db}button{border:1px solid #c5d3c9;background:white;border-radius:8px;padding:8px 16px}
</style>
