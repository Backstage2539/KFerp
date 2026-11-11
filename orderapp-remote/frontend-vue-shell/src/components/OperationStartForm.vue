<template>
  <section class="start-form" aria-label="确认本批开工用料">
    <div class="head"><h3>确认本批开工用料</h3><button type="button" :disabled="locked || busy || retry.uncertain" @click="$emit('close')">关闭</button></div>
    <p v-if="error" role="alert" class="error">{{ error }}</p>
    <p v-if="!preview">正在核对本批配方和库存…</p>
    <template v-else>
      <p>{{ preview.product_name }} · BOM {{ preview.effective_bom_version_no || preview.bom_version_id }} · {{ preview.operation_name }}</p>
      <p>本批计划产出 {{ preview.planned_qty }} {{ preview.output_unit }}。批次按冻结预留和可用批次顺序扣减，确认时重新核对。</p>
      <p v-if="preview.blocked_reason" role="alert" class="error">{{ preview.blocked_reason }}</p>
      <p v-if="preview.predecessor_required">本工序承接前序产出，不再扣本 BOM 原料。</p>
      <div v-if="preview.incoming_allocations?.length" class="warning" role="status">
        <p>按前序实际产出核对各工位来料，首次开工后冻结：</p>
        <p v-for="allocation in preview.incoming_allocations" :key="allocation.job_card_id">{{ allocation.workstation || `任务 ${allocation.job_card_id}` }}：{{ allocation.quantity }} {{ allocation.unit }}</p>
      </div>
      <p v-if="preview.bom_refresh_pending">确认开工后锁定配方 {{ preview.effective_bom_version_no }}。</p>
      <div v-if="startShortages.length" class="warning">
        <p v-for="m in startShortages" :key="m.key">本批开工缺口：{{ m.name }} {{ m.shortage_qty }} {{ m.unit }}</p>
        <p>补料成功后将重新读取库存，已填写的投入数量和差异原因会保留，请重新核对。</p>
        <button type="button" :disabled="locked || busy || retry.uncertain" @click="openIssue">补齐物料</button>
      </div>
      <fieldset :disabled="locked || busy || retry.uncertain || !!preview.blocked_reason">
        <label>本批实际投入（{{ preview.input_unit }}）<input v-model.number="draft.actual_input_qty" type="number" min="0" step="any" @change="recalculate" /></label>
        <div v-for="(m,index) in draft.materials" :key="m.key" class="material">
          <label>{{ m.name }}（{{ m.unit }}）<input v-model.number="draft.materials[index].qty" type="number" min="0" step="any" /></label>
          <span>扣料仓：{{ m.warehouse_name || m.warehouse }} · 货主：{{ m.owner_customer_id || '本公司' }} · 可用 {{ m.available_qty ?? 0 }}</span>
          <p class="batches">可用批次：{{ (m.eligible_batches || []).map(b => `${b.batch_code}（${b.available_qty} ${m.unit}）`).join('、') || '暂无可用批次' }}</p>
        </div>
        <label>用量差异原因<textarea v-model.trim="draft.difference_reason" rows="2" /></label>
      </fieldset>
      <button type="button" class="primary" :disabled="locked || busy || !!preview.blocked_reason" @click="submit">{{ busy ? '正在开工…' : retry.uncertain ? '重试本次开工' : preview.predecessor_required ? '确认开工' : '确认开工并扣料' }}</button>
    </template>
  </section>
</template>
<script setup>
import {computed,onMounted,reactive,ref,watch} from 'vue'
import {apiGet,apiSend} from '../api/client'
import {completionRetry} from '../lib/operation-completion.js'
import {startDraft,startPayload,startSuggestedMaterials,refreshStartDraft} from '../lib/operation-start.js'
const props=defineProps({jobCardId:{type:Number,required:true},locked:{type:Boolean,default:false},refreshToken:{type:Number,default:0}})
const emit=defineEmits(['close','posted','issue'])
const preview=ref(null),draft=reactive({materials:[],actual_input_qty:0,difference_reason:''}),busy=ref(false),error=ref(''),retry=reactive(completionRetry())
const startShortages=computed(()=>preview.value?.predecessor_required?[]:(draft.materials||[]).filter(m=>m.warehouse==='wip' && Number.isFinite(Number(m.available_qty ?? 0)) && Number(m.qty)>Number(m.available_qty ?? 0)+1e-8).map(m=>({...m,shortage_qty:Number((Number(m.qty)-Number(m.available_qty ?? 0)).toFixed(6))})))
function openIssue(){if(!props.locked&&!busy.value&&!retry.uncertain&&startShortages.value.length)emit('issue',startShortages.value,{expected_version:preview.value.version,actual_input_qty:draft.actual_input_qty,input_unit:preview.value.input_unit,materials:draft.materials.map(m=>({...m})),difference_reason:draft.difference_reason})}
function recalculate(){draft.materials=startSuggestedMaterials(preview.value,draft.actual_input_qty)}
async function load(preserve=false){busy.value=true;error.value='';try{const next=await apiGet(`/api/job-cards/${props.jobCardId}/start-preview`);const updated=preserve&&preview.value?refreshStartDraft(next,draft):startDraft(next);preview.value=next;Object.assign(draft,updated)}catch(e){error.value=e.message}finally{busy.value=false}}
async function submit(){if(props.locked||busy.value)return;busy.value=true;error.value='';try{const payload=retry.begin(startPayload(preview.value,draft,''),()=>crypto.randomUUID());const result=await apiSend(`/api/job-cards/${props.jobCardId}/start`,{body:payload});retry.clear();emit('posted',result)}catch(e){retry.fail(e);error.value=retry.uncertain?'尚未收到开工结果，请重试原次操作，系统不会重复扣料。':e.message}finally{busy.value=false}}
watch(()=>props.refreshToken,()=>load(true))
onMounted(()=>load())
</script>
<style scoped>
.start-form{grid-column:1/-1;background:#f7faf8;border-top:3px solid #29945e;padding:20px;border-radius:12px;min-width:0}.head{display:flex;justify-content:space-between;gap:12px}.start-form fieldset{border:0;padding:0}.start-form label{display:grid;gap:6px;margin:10px 0;max-width:600px}.start-form input,.start-form textarea{padding:10px;border:1px solid #cbd6d0;border-radius:8px;font:inherit;min-width:0}.material{padding:8px 0;border-bottom:1px solid #d5dfd9}.material span{font-size:13px;color:#5c7166}.error{color:#ae3427}.warning{margin:12px 0;padding:12px 16px;border:1px solid #ead8ad;border-radius:8px;background:#fff9ed;color:#875915}.warning p{margin:6px 0;font-size:13px;line-height:1.5}.warning button{background:#fff;color:#79510d}button{padding:10px 16px;border:1px solid #cad7cf;border-radius:8px;cursor:pointer}.primary{margin-top:16px;background:#2d915e;color:#fff}.start-form button:disabled{opacity:.5;cursor:default}
</style>
