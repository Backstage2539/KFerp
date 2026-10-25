<template>
  <section class="completion-form" aria-label="完成本工序">
    <div class="head"><h3>完成本工序</h3><button type="button" @click="$emit('close')" :disabled="busy || retry.uncertain">关闭</button></div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="saved" class="success" role="status">{{ saved }}</p>
    <p v-if="success" class="success" role="status">{{ success }}</p>
    <template v-if="preview">
      <div class="identity"><strong>{{ preview.product_name }}</strong><span>规格：{{ preview.package_spec_label || '无包装规格' }}</span><span>{{ preview.execution.receipt_enabled ? '完成后入库' : '交接位置' }}：{{ preview.warehouse_name }}</span></div>
      <p v-if="preview.blocked_reason" class="error">{{ preview.blocked_reason }}</p>
      <fieldset :disabled="busy || retry.uncertain || !!success">
        <div class="fields">
          <label>{{ completionQuantityLabel(preview.output_unit) }}<input v-model="draft.actual_output_qty" type="number" min="0" :step="weightUnit(preview.output_unit) ? 'any' : '1'" @input="outputChanged" /><small>本工序剩余参考 {{ preview.planned_qty }} {{ preview.output_unit }} · 当前可完成 {{ preview.max_output_qty ?? preview.planned_qty }} {{ preview.output_unit }} · 已完成 {{ preview.completed_output_qty || 0 }} {{ preview.output_unit }}</small></label>
          <label>实际投入（{{ preview.input_unit }}）<input v-model="draft.actual_input_qty" type="number" min="0" :step="weightUnit(preview.input_unit) ? 'any' : '1'" @input="inputEdited = true" /></label>
          <label>实际工时（分钟，可选）<input v-model="draft.actual_minutes" type="number" min="0" step="1" /><small>未填写时，标准工时成本暂估</small></label>
          <label>产量差异原因<input v-model="draft.difference_reason" placeholder="超产、少产或无产出的实际原因" /></label>
        </div>
        <label class="finish-choice"><input v-model="draft.finish_operation" type="checkbox" /> 本工序已全部做完 <small v-if="!draft.finish_operation">完成本批后，剩余数量继续保留待办</small><small v-else-if="Number(draft.actual_output_qty) !== Number(preview.planned_qty)">少产或超产结单时请填写差异原因</small></label>
        <details v-if="draft.materials?.length" open><summary>本工序新增用料</summary><div v-for="row in draft.materials" :key="row.reservation_id" class="material"><span>{{ row.name }}<small>{{ row.warehouse_name }} · 可用 {{ row.available_qty ?? row.qty }} {{ row.unit }}<template v-if="row.shortage_qty > 0"> · 尚缺 {{ row.shortage_qty }} {{ row.unit }}</template></small></span><label>实际用量（{{ row.unit }}）<input v-model="row.qty" type="number" min="0" :step="weightUnit(row.unit) ? 'any' : '1'" @input="row.edited = true" /></label></div></details>
        <details v-if="preview.predecessor_batches?.length"><summary>前序批次与实际位置</summary><p v-for="row in preview.predecessor_batches" :key="row.id">{{ row.operation }} · {{ row.remaining }} {{ row.unit }} · {{ row.warehouse_name }}</p></details>
      </fieldset>
      <p class="hint">保存进度不改变库存；完成本批后自动用料，并记录交接或入库。前序原料不会重复扣减。</p>
      <div class="actions"><button v-if="!success && !retry.uncertain" type="button" :disabled="busy || !preview.enabled" @click="saveProgress">保存进度</button><button v-if="!success" class="primary" type="button" :disabled="busy || !!preview.blocked_reason" @click="complete">{{ retry.uncertain ? '重试原次完成' : (draft.finish_operation ? '完成并' : '完成本批并') + (preview.execution.receipt_enabled ? '入库' : '交接') }}</button><button v-if="!success && !retry.uncertain" type="button" :disabled="busy" @click="refresh">刷新建议</button><button v-if="success" class="primary" type="button" @click="$emit('next')">继续处理下一工序</button></div>
      <details v-if="result?.stock_entry_ids?.length"><summary>关联单据</summary><p v-for="id in result.stock_entry_ids" :key="id">库存单据 #{{ id }}</p></details>
    </template>
    <p v-else>正在核对完成数量与用料…</p>
  </section>
</template>
<script setup>
import { onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { completionDraft, completionPayload, progressPayload, completionQuantityLabel, completionRetry, suggestedMaterialQty, suggestedInputQty, weightUnit } from '../lib/operation-completion.js'
const props=defineProps({jobCardId:{type:Number,required:true}})
const emit=defineEmits(['close','posted','next'])
const saved=ref(''),inputEdited=ref(false)
const preview=ref(null),draft=reactive({materials:[]}),busy=ref(false),error=ref(''),success=ref(''),result=ref(null)
const retry=reactive(completionRetry())
async function refresh(){busy.value=true;error.value='';try{const p=await apiGet(`/api/job-cards/${props.jobCardId}/completion-preview`);const previous=preview.value;preview.value=p;if(!previous)Object.assign(draft,completionDraft(p));else{draft.materials=p.materials.map(row=>{const entered=draft.materials.find(m=>m.reservation_id===row.reservation_id);return {...row,...(entered?{qty:entered.qty,edited:true}:{})}})}retry.clear()}catch(e){error.value=e.message}finally{busy.value=false}}
function suggestUsage(){if(!preview.value?.planned_qty)return;for(const row of draft.materials){if(row.edited)continue;const original=preview.value.materials.find(x=>x.reservation_id===row.reservation_id);if(original)row.qty=suggestedMaterialQty(preview.value,original,Number(draft.actual_output_qty))}}
function outputChanged(){const qty=Number(draft.actual_output_qty);if(!inputEdited.value)draft.actual_input_qty=suggestedInputQty(preview.value,qty);draft.finish_operation=qty>=Number(preview.value?.planned_qty||0);suggestUsage()}
async function saveProgress(){busy.value=true;error.value='';try{const payload=progressPayload(preview.value,draft);const p=await apiSend(`/api/job-cards/${props.jobCardId}/progress`,{body:payload});preview.value=p;error.value='';saved.value='进度已保存，库存未改变';success.value='';}catch(e){error.value=e.message}finally{busy.value=false}}
async function complete(){busy.value=true;error.value='';try{const payload=retry.begin(completionPayload(preview.value,draft,''),()=>crypto.randomUUID());const data=await apiSend(`/api/job-cards/${props.jobCardId}/complete`,{body:payload});result.value=data.completion;success.value=data.completion?.message || '完成记录已保存';retry.clear();emit('posted',data)}catch(e){retry.fail(e);error.value=retry.uncertain?'尚未收到完成结果。输入已保留，请重试原次完成，系统不会重复扣料。':e.message}finally{busy.value=false}}
onMounted(refresh)
</script>
<style scoped>
.completion-form{grid-column:1/-1;background:#f7faf8;border-top:3px solid #29945e;padding:20px;border-radius:12px;min-width:0}.head,.identity,.actions{display:flex;gap:16px;align-items:center;flex-wrap:wrap}.head{justify-content:space-between}.identity{margin:12px 0}.fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px}label{display:flex;flex-direction:column;gap:6px}.finish-choice{flex-direction:row;align-items:center;margin:14px 0}.finish-choice input{width:auto}input{border:1px solid #cbd6d0;border-radius:8px;padding:10px;min-width:0;font:inherit}small{display:block;color:#64756e;font-size:12px}fieldset{border:0;padding:0;min-width:0}.material{display:grid;grid-template-columns:1fr 1fr;gap:16px;align-items:center;padding:10px 0}details{margin:15px 0}.hint{color:#5c7166}.error{color:#ae3427}.success{color:#218450;font-weight:700}button{padding:10px 16px;border:1px solid #cad7cf;border-radius:8px;background:white;font:inherit;cursor:pointer}.primary{background:#2d915e;color:white;border-color:#2d915e}button:disabled{opacity:.5;cursor:default}@media(max-width:640px){.fields,.material{grid-template-columns:1fr}}
</style>
