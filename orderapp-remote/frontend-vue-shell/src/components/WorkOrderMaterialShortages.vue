<template>
  <section class="shortages" aria-label="工单用料">
    <strong>本次用料</strong>
    <p v-if="loading">正在核对 WIP 库存…</p>
    <p v-else-if="error" role="alert">{{ error }}</p>
    <p v-else-if="!rows.length">暂无可确认的 WIP 原料缺口；实际开工时会再次核对。</p>
    <div v-for="row in rows" :key="row.id" class="shortage-task">
      <p v-for="m in row.shortages" :key="m.key">{{ m.name }}：需 {{ m.qty }} {{ m.unit }} · WIP 可用 {{ m.available_qty ?? 0 }} {{ m.unit }} · 待调入 {{ m.shortage_qty }} {{ m.unit }}</p>
      <button type="button" :disabled="loading" @click="open(row)">生成补料转仓单</button>
    </div>
    <WorkstationBatchIssueDrawer v-if="drawer" :task-ids="[drawer.id]" :start-shortages="drawer.shortages" :start-input="drawer.input" :workstation="drawer.station" @close="drawer=null" @posted="posted" @cancelled="posted" />
  </section>
</template>
<script setup>
import { onMounted, ref } from 'vue'
import { apiGet } from '../api/client.js'
import { startPreviewShortages, pendingFirstOperationCards } from '../lib/workstation-batch-issue.js'
import WorkstationBatchIssueDrawer from './WorkstationBatchIssueDrawer.vue'
const props=defineProps({workOrderId:{type:Number,required:true}})
const rows=ref([]),loading=ref(false),error=ref(''),drawer=ref(null)
function entry(card,p){return {id:Number(card.id),station:card.workstation||'',shortages:startPreviewShortages(p),input:{expected_version:p.version,actual_input_qty:p.input_qty,input_unit:p.input_unit,materials:p.materials,difference_reason:''}}}
async function load(){loading.value=true;error.value='';try{const data=await apiGet(`/api/produce/work-orders/${props.workOrderId}`);const candidates=pendingFirstOperationCards(data.job_cards);const results=await Promise.allSettled(candidates.map(async card=>entry(card,await apiGet(`/api/job-cards/${card.id}/start-preview`))));rows.value=results.filter(x=>x.status==='fulfilled'&&x.value.shortages.length).map(x=>x.value);if(results.some(x=>x.status==='rejected'))error.value='部分任务的用料暂无法核对，请到工位重新核对。'}catch(e){error.value=`无法核对用料：${e.message}`}finally{loading.value=false}}
async function open(row){loading.value=true;error.value='';try{const p=await apiGet(`/api/job-cards/${row.id}/start-preview`);const updated=entry({id:row.id,workstation:row.station},p);if(!updated.shortages.length){error.value='库存已变化，目前没有可确认的缺口。';await load();return}drawer.value=updated}catch(e){error.value=`无法核对用料：${e.message}`}finally{loading.value=false}}
async function posted(){drawer.value=null;await load()}
onMounted(load)
</script>
<style scoped>
.shortages{margin:12px 0;padding:12px;border:1px solid #dce3dd;border-radius:8px}.shortages p{margin:8px 0;color:#5c7166}.shortage-task{padding:8px 0;border-top:1px solid #edf0eb}.shortage-task button{background:white;border:1px solid #b6c9bc;border-radius:6px;padding:7px 12px;cursor:pointer}
</style>
