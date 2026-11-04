<template>
  <details class="operation-material-config" @toggle="opened">
    <summary>物料来源仓</summary>
    <p>来源仓可选；未指定时沿用首道工序默认仓，再使用 WIP。此处调整只影响新工单。</p>
    <p v-if="error" role="alert" class="error">{{ error }}</p><p v-if="warehouseError" role="alert" class="error">仓库未加载：{{ warehouseError }}。刷新成功前不能保存仓库配置。</p><p v-if="message" role="status">{{ message }}</p>
    <div v-for="row in config.rows" :key="row.key" class="row"><strong>{{ row.name }}</strong><label>来源仓<select v-model="row.binding.source_warehouse"><option value="">沿用首道工序默认仓</option><option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}</option></select></label></div>
    <div class="actions"><button type="button" :disabled="busy || !config.rows.length || !!warehouseError" @click="save">保存来源仓</button><button type="button" :disabled="busy" @click="load">刷新</button></div>
  </details>
</template>
<script setup>
import { reactive,ref,watch } from 'vue'
import { apiGet,apiSend } from '../api/client'
import {materialSourcesPayload} from '../lib/material-sources.js'
const props=defineProps({versionId:{type:Number,required:true},variantId:{type:Number,default:0}})
const config=reactive({rows:[],nodes:[],version:''}),warehouses=ref([]),busy=ref(false),loaded=ref(false),error=ref(''),warehouseError=ref(''),message=ref('')
async function opened(event){if(event.target.open&&!loaded.value)await load()}
async function load(){if(!props.versionId)return;busy.value=true;error.value='';warehouseError.value='';message.value='';try{const result=await apiGet(`/api/production-bom-versions/${props.versionId}/material-sources?variant_id=${props.variantId}`);Object.assign(config,result);loaded.value=true}catch(e){error.value=e.message}try{const stock=await apiGet('/api/stock/warehouses');warehouses.value=(stock.rows||stock||[]).filter(row=>row.active!==false)}catch(e){warehouses.value=[];warehouseError.value=e.message||'仓库加载失败'}finally{busy.value=false}}
async function save(){busy.value=true;error.value='';message.value='';try{Object.assign(config,await apiSend(`/api/production-bom-versions/${props.versionId}/material-sources`,{method:'PUT',body:materialSourcesPayload(config,props.variantId)}));message.value='来源仓已保存，新工单按此冻结。'}catch(e){error.value=e.message}finally{busy.value=false}}
watch(()=>[props.versionId,props.variantId],()=>{loaded.value=false;config.rows=[];load()})
</script>
<style scoped>
.operation-material-config{border:1px solid #d5dfd9;border-radius:8px;padding:14px;margin:16px 0}summary{font-weight:600;cursor:pointer}.row{display:grid;grid-template-columns:1fr 2fr;gap:12px;align-items:center;padding:10px 0}label{display:flex;flex-direction:column;gap:5px}select{padding:8px;min-width:0;max-width:100%;border:1px solid #ccd6cf;border-radius:6px}.actions{display:flex;gap:10px}.error{color:#b73728}@media(max-width:640px){.row{grid-template-columns:1fr}}
</style>
