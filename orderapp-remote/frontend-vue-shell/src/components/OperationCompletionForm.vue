<template>
  <section class="completion-form" aria-label="完成本工序">
    <div class="head"><h3>完成本工序</h3><button type="button" @click="$emit('close')" :disabled="busy || retry.uncertain">关闭</button></div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="success" class="success" role="status">{{ success }}</p>
    <template v-if="preview">
      <div class="identity"><strong>{{ preview.product_name }}</strong><span>规格：{{ preview.package_spec_label || '无包装规格' }}</span><span>{{ preview.execution?.receipt_enabled ? '完成后入库' : '交接位置' }}：{{ preview.warehouse_name }}</span></div>
      <div v-if="preview.blocked_reason" class="blocked" role="alert">
        <p>{{ preview.blocked_reason }}</p>
        <button v-if="preview.blocked_code === 'bom_incomplete' && preview.bom_id" type="button" @click="openBom">去 BOM 补齐配料</button>
      </div>
      <p v-if="preview.bom_refresh_pending" class="hint" role="status">本工单尚无实际耗料，将在首次{{ preview.material_consumption_policy === 'bom_start_v1' ? '开工' : '完成' }}时接入已发布的完整配方（{{ preview.effective_bom_version_no }}；原冻结 {{ preview.original_bom_version_no }}）。</p>
      <fieldset :disabled="busy || retry.uncertain || !!success || !!preview.blocked_reason">
        <label v-if="preview.execution?.reporting_type !== 'incoming'">{{ completionQuantityLabel(preview.output_unit) }}
          <input v-model="draft.actual_output_qty" type="number" min="0" :step="weightUnit(preview.output_unit) ? 'any' : '1'" @input="updatePackagingSuggestion" />
          <small>本批参考 {{ preview.planned_qty }} {{ preview.output_unit }} · 已完成 {{ preview.completed_output_qty || 0 }} {{ preview.output_unit }}</small>
        </label>
        <template v-if="preview.material_consumption_policy === 'bom_start_v1'">
          <label v-if="preview.execution?.reporting_type !== 'piece'">本次实际处理投入（{{ preview.input_unit }}）<input v-model="draft.actual_input_qty" type="number" min="0" :step="weightUnit(preview.input_unit) ? 'any' : '1'" /><small>本批剩余可处理 {{ preview.input_qty }} {{ preview.input_unit }}</small></label>
          <template v-if="preview.execution?.reporting_type === 'piece'">
            <p>目标 {{ preview.total_planned_qty }} {{ preview.output_unit }} · 已完成 {{ preview.completed_output_qty || 0 }} · 尚需 {{ preview.planned_qty }}</p>
            <p>预计余料 {{ packagingExpectedRemainder(preview,draft.actual_output_qty) }} g；请称重后确认。</p>
            <label>实际余料（g）<input v-model="draft.actual_remainder_g" type="number" min="0" step="1" @input="remainderTouched=true;draft.remainder_confirmed=false" /></label>
            <label>余料去向<select v-model="draft.remainder_warehouse"><option value="wip">WIP在制仓</option><option v-for="w in warehouses.filter(row => row.code !== 'wip')" :key="w.code" :value="w.code">{{ w.name }}</option></select></label>
            <label><span><input v-model="draft.remainder_confirmed" type="checkbox" /> 已称重并确认余料</span></label>
            <div v-if="preview.packaging_unissued_candidates?.length">
              <strong>同批次未领可用库存（共享）</strong><p>这些库存不自动计入本单余料。仅移动明确勾选并填写数量的批次。</p>
              <div v-for="candidate in preview.packaging_unissued_candidates" :key="candidateKey(candidate)">
                <label><span><input type="checkbox" :checked="!!selectedCandidate(candidate)" @change="toggleCandidate(candidate,$event.target.checked)" /> {{ candidate.batch_code }} · {{ candidate.warehouse }} · 货主 {{ candidate.owner_customer_id || '本公司' }} · 可用 {{ candidate.available_g }} g</span></label>
                <label v-if="selectedCandidate(candidate)">确认数量（g）<input v-model.number="selectedCandidate(candidate).quantity_g" type="number" min="1" :max="candidate.available_g" step="1" /></label>
              </div>
            </div>
          </template>
          <template v-if="preview.execution?.reporting_type === 'incoming'">
            <label>废料重量（{{ preview.input_unit }}）<input v-model="draft.waste_qty" type="number" min="0" step="any" /></label>
            <p>合格产出：{{ goodOutput }} {{ preview.output_unit }}</p>
          </template>
          <label>实际工时（分钟）<input v-model="draft.actual_minutes" type="number" min="0" step="1" /></label>
          <label v-if="preview.execution?.reporting_type !== 'piece'"><span><input v-model="draft.finish_operation" type="checkbox" /> 本批全部处理结束</span></label>
          <label>数量差异或损耗原因<textarea v-model.trim="draft.difference_reason" rows="2" /></label>
          <details v-if="preview.started_materials?.length"><summary>已扣用料</summary><p v-for="m in preview.started_materials" :key="m.key">{{ m.name }}：{{ m.qty }} {{ m.unit }} · {{ m.warehouse_name || m.warehouse }}</p></details>
        </template>
      </fieldset>
      <p v-if="preview.material_consumption_policy === 'bom_start_v1'" class="hint">本次报工结转已确认投入的成本，不再扣原料。结束批次前须处理剩余投入。</p>
      <p v-else class="hint">系统按冻结的本批投料和 BOM 自动计算用料，完成后自动扣料并{{ preview.execution?.receipt_enabled ? '入库' : '交接' }}。</p>
      <div class="actions"><button v-if="!success" class="primary" type="button" :disabled="busy || !!preview.blocked_reason" @click="complete">{{ busy ? '处理中…' : retry.uncertain ? '重试本次完成' : '确认工序完成' }}</button></div>
      <details v-if="result?.stock_entry_ids?.length"><summary>关联单据</summary><p v-for="id in result.stock_entry_ids" :key="id">库存单据 #{{ id }}</p></details>
    </template>
    <p v-else>正在读取本批完成数量…</p>
  </section>
</template>
<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { completionDraft, completionPayload, completionQuantityLabel, completionRetry, incomingOutput, packagingExpectedRemainder, weightUnit } from '../lib/operation-completion.js'
const props=defineProps({jobCardId:{type:Number,required:true},returnParams:{type:Object,default:()=>({})}})
const emit=defineEmits(['close','posted'])
const preview=ref(null),draft=reactive({}),busy=ref(false),error=ref(''),success=ref(''),result=ref(null)
const warehouses=ref([]),remainderTouched=ref(false)
const goodOutput=computed(()=>{try{const input=Number(draft.actual_input_qty),waste=Number(draft.waste_qty);return input>=0&&waste>=0&&waste<=input?incomingOutput(input,waste,preview.value?.input_unit,preview.value?.output_unit):'—'}catch{return '—'}})
function updatePackagingSuggestion(){if(preview.value?.execution?.reporting_type!=='piece')return;draft.remainder_confirmed=false;if(!remainderTouched.value)draft.actual_remainder_g=packagingExpectedRemainder(preview.value,draft.actual_output_qty)}
function candidateKey(c){return `${c.batch_code}\u0000${c.warehouse}\u0000${Number(c.owner_customer_id||0)}`}
function selectedCandidate(c){return (draft.selected_unissued_remainders||[]).find(row=>candidateKey(row)===candidateKey(c))}
function toggleCandidate(c,checked){draft.selected_unissued_remainders=(draft.selected_unissued_remainders||[]).filter(row=>candidateKey(row)!==candidateKey(c));if(checked)draft.selected_unissued_remainders.push({batch_code:c.batch_code,warehouse:c.warehouse,owner_customer_id:Number(c.owner_customer_id||0),quantity_g:0})}
const retry=reactive(completionRetry())
async function refresh(){busy.value=true;error.value='';try{const p=await apiGet(`/api/job-cards/${props.jobCardId}/completion-preview`);preview.value=p;Object.assign(draft,completionDraft(p));retry.clear();if(p.execution?.reporting_type==='piece'){try{const data=await apiGet('/api/stock/warehouses');warehouses.value=(data.rows||[]).filter(w=>w.active!==false&&w.code)}catch{warehouses.value=[]}}}catch(e){error.value=e.message}finally{busy.value=false}}
function openBom(){window.dispatchEvent(new CustomEvent('kferp:navigate-view',{detail:{key:'productionConfig',params:{tab:'bom',production_bom_id:Number(preview.value?.bom_id||0)},returnNavigation:{key:'workstationView',label:'返回工位任务',params:{...props.returnParams}}}}))}
async function complete(){busy.value=true;error.value='';try{const payload=retry.begin(completionPayload(preview.value,draft,''),()=>crypto.randomUUID());const data=await apiSend(`/api/job-cards/${props.jobCardId}/complete`,{body:payload});result.value=data.completion;success.value=data.completion?.message||'工序完成';retry.clear();emit('posted',data)}catch(e){retry.fail(e);error.value=retry.uncertain?'尚未收到完成结果，请重试原次操作，系统不会重复扣料。':e.message}finally{busy.value=false}}
onMounted(refresh)
</script>
<style scoped>
.completion-form{grid-column:1/-1;background:#f7faf8;border-top:3px solid #29945e;padding:20px;border-radius:12px;min-width:0}.head,.identity,.actions{display:flex;gap:16px;align-items:center;flex-wrap:wrap}.head{justify-content:space-between}.identity{margin:12px 0}.completion-form label{display:grid;gap:6px;max-width:520px}.completion-form input{border:1px solid #cbd6d0;border-radius:8px;padding:10px;min-width:0;font:inherit}.completion-form small{display:block;color:#64756e;font-size:12px}.completion-form fieldset{border:0;padding:0;min-width:0}.hint{color:#5c7166}.error,.blocked{color:#ae3427}.blocked{padding:12px;background:#fff2f0;border-radius:8px}.success{color:#218450;font-weight:700}button{padding:10px 16px;border:1px solid #cad7cf;border-radius:8px;background:white;font:inherit;cursor:pointer}.primary{background:#2d915e;color:white;border-color:#2d915e}button:disabled{opacity:.5;cursor:default}
</style>
