<template>
  <details class="operation-material-config" @toggle="opened">
    <summary>物料使用工序与来源仓</summary>
    <p>每个组件只在所选工序扣一次；默认从工序来源仓使用。此处调整只影响新工单。</p>
    <p v-if="error" role="alert" class="error">{{ error }}</p><p v-if="message" role="status">{{ message }}</p>
    <div v-for="row in config.rows" :key="row.key" class="row"><strong>{{ row.name }}</strong><label>使用工序<select v-model="row.binding.node_key"><option value="">请选择工序</option><option v-for="node in config.nodes" :key="node.key" :value="node.key">{{ node.sequence }} · {{ node.name }}</option></select></label><label>来源仓<select v-model="row.binding.source_warehouse"><option value="">沿用工序默认仓</option><option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}</option></select></label></div>
    <div class="actions"><button type="button" :disabled="busy || !config.rows.length" @click="save">保存用料归属</button><button type="button" :disabled="busy" @click="load">刷新</button></div>
  </details>
</template>
<script setup>
import { reactive,ref,watch } from 'vue'
import { apiGet,apiSend } from '../api/client'
const props=defineProps({versionId:{type:Number,required:true},variantId:{type:Number,default:0}})
const config=reactive({rows:[],nodes:[],version:''}),warehouses=ref([]),busy=ref(false),loaded=ref(false),error=ref(''),message=ref('')
async function opened(event){if(event.target.open&&!loaded.value)await load()}
async function load(){if(!props.versionId)return;busy.value=true;error.value='';message.value='';try{const [result,stock]=await Promise.all([apiGet(`/api/production-bom-versions/${props.versionId}/operation-materials?variant_id=${props.variantId}`),apiGet('/api/warehouses')]);Object.assign(config,result);warehouses.value=(stock.rows||stock||[]).filter(row=>row.active!==false);loaded.value=true}catch(e){error.value=e.message}finally{busy.value=false}}
async function save(){busy.value=true;error.value='';message.value='';try{Object.assign(config,await apiSend(`/api/production-bom-versions/${props.versionId}/operation-materials`,{method:'PUT',body:{variant_id:props.variantId,expected_version:config.version,bindings:Object.fromEntries(config.rows.map(row=>[row.key,row.binding]))}}));message.value='用料归属已保存，新工单按此冻结。'}catch(e){error.value=e.message}finally{busy.value=false}}
watch(()=>[props.versionId,props.variantId],()=>{loaded.value=false;config.rows=[];load()})
</script>
<style scoped>
.operation-material-config{border:1px solid #d5dfd9;border-radius:8px;padding:14px;margin:16px 0}summary{font-weight:600;cursor:pointer}.row{display:grid;grid-template-columns:1fr 1fr 1fr;gap:12px;align-items:center;padding:10px 0}label{display:flex;flex-direction:column;gap:5px}select{padding:8px;min-width:0;max-width:100%;border:1px solid #ccd6cf;border-radius:6px}.actions{display:flex;gap:10px}.error{color:#b73728}@media(max-width:640px){.row{grid-template-columns:1fr}}
</style>
