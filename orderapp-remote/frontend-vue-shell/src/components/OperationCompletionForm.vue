<template>
  <section class="completion-form" aria-label="完成本工序">
    <div class="head"><h3>完成本工序</h3><button type="button" @click="$emit('close')" :disabled="busy || retry.uncertain">关闭</button></div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="success" class="success" role="status">{{ success }}</p>
    <template v-if="preview">
      <div class="identity"><strong>{{ preview.product_name }}</strong><span>规格：{{ preview.package_spec_label || '无包装规格' }}</span><span>{{ preview.execution?.receipt_enabled ? '完成后入库' : '交接位置' }}：{{ preview.warehouse_name }}</span></div>
      <div v-if="preview.blocked_reason" class="blocked" role="alert">
        <p>{{ preview.blocked_reason }}</p>
        <button v-if="preview.bom_id" type="button" @click="openBom">去 BOM 补齐配料</button>
      </div>
      <p v-if="preview.bom_refresh_pending" class="hint" role="status">本工单尚无实际耗料，将在首次完成时接入已发布的完整配方（V{{ preview.effective_bom_version_id }}；原冻结 V{{ preview.original_bom_version_id }}）。</p>
      <fieldset :disabled="busy || retry.uncertain || !!success || !!preview.blocked_reason">
        <label>{{ completionQuantityLabel(preview.output_unit) }}
          <input v-model="draft.actual_output_qty" type="number" min="0" :step="weightUnit(preview.output_unit) ? 'any' : '1'" />
          <small>本批参考 {{ preview.planned_qty }} {{ preview.output_unit }} · 已完成 {{ preview.completed_output_qty || 0 }} {{ preview.output_unit }}</small>
        </label>
      </fieldset>
      <p class="hint">系统按冻结的本批投料和 BOM 自动计算用料，完成后自动扣料并{{ preview.execution?.receipt_enabled ? '入库' : '交接' }}。</p>
      <div class="actions"><button v-if="!success" class="primary" type="button" :disabled="busy || !!preview.blocked_reason" @click="complete">{{ busy ? '处理中…' : retry.uncertain ? '重试本次完成' : '确认工序完成' }}</button></div>
      <details v-if="result?.stock_entry_ids?.length"><summary>关联单据</summary><p v-for="id in result.stock_entry_ids" :key="id">库存单据 #{{ id }}</p></details>
    </template>
    <p v-else>正在读取本批完成数量…</p>
  </section>
</template>
<script setup>
import { onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { completionDraft, completionPayload, completionQuantityLabel, completionRetry, weightUnit } from '../lib/operation-completion.js'
const props=defineProps({jobCardId:{type:Number,required:true},returnParams:{type:Object,default:()=>({})}})
const emit=defineEmits(['close','posted'])
const preview=ref(null),draft=reactive({}),busy=ref(false),error=ref(''),success=ref(''),result=ref(null)
const retry=reactive(completionRetry())
async function refresh(){busy.value=true;error.value='';try{const p=await apiGet(`/api/job-cards/${props.jobCardId}/completion-preview`);preview.value=p;Object.assign(draft,completionDraft(p));retry.clear()}catch(e){error.value=e.message}finally{busy.value=false}}
function openBom(){window.dispatchEvent(new CustomEvent('kferp:navigate-view',{detail:{key:'productionConfig',params:{tab:'bom',production_bom_id:Number(preview.value?.bom_id||0)},returnNavigation:{key:'workstationView',label:'返回工位任务',params:{...props.returnParams}}}}))}
async function complete(){busy.value=true;error.value='';try{const payload=retry.begin(completionPayload(preview.value,draft,''),()=>crypto.randomUUID());const data=await apiSend(`/api/job-cards/${props.jobCardId}/complete`,{body:payload});result.value=data.completion;success.value=data.completion?.message||'工序完成';retry.clear();emit('posted',data)}catch(e){retry.fail(e);error.value=retry.uncertain?'尚未收到完成结果，请重试原次操作，系统不会重复扣料。':e.message}finally{busy.value=false}}
onMounted(refresh)
</script>
<style scoped>
.completion-form{grid-column:1/-1;background:#f7faf8;border-top:3px solid #29945e;padding:20px;border-radius:12px;min-width:0}.head,.identity,.actions{display:flex;gap:16px;align-items:center;flex-wrap:wrap}.head{justify-content:space-between}.identity{margin:12px 0}.completion-form label{display:grid;gap:6px;max-width:520px}.completion-form input{border:1px solid #cbd6d0;border-radius:8px;padding:10px;min-width:0;font:inherit}.completion-form small{display:block;color:#64756e;font-size:12px}.completion-form fieldset{border:0;padding:0;min-width:0}.hint{color:#5c7166}.error,.blocked{color:#ae3427}.blocked{padding:12px;background:#fff2f0;border-radius:8px}.success{color:#218450;font-weight:700}button{padding:10px 16px;border:1px solid #cad7cf;border-radius:8px;background:white;font:inherit;cursor:pointer}.primary{background:#2d915e;color:white;border-color:#2d915e}button:disabled{opacity:.5;cursor:default}
</style>
