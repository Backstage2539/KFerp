<template>
  <section class="completion-form" aria-label="完成本工序">
    <div class="head"><h3>完成本工序</h3><button type="button" @click="$emit('close')" :disabled="busy || topUpBusy || topUpRetry.uncertain || retry.uncertain">关闭</button></div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <button v-if="refreshNeeded" type="button" :disabled="busy" @click="recheckKeepingDraft">重新核对</button>
    <p v-if="success" class="success" role="status">{{ success }}</p>
    <button v-if="retry.uncertain && batches.length" type="button" :disabled="busy" @click="retryBatchSubmit">重试本次完成</button>
    <template v-if="preview">
      <div class="identity"><strong>{{ preview.product_name }}</strong><span>规格：{{ preview.package_spec_label || '无包装规格' }}</span><span>{{ preview.execution?.receipt_enabled ? '完成后入库' : '交接位置' }}：{{ preview.warehouse_name }}</span></div>
      <div v-if="preview.blocked_reason" class="blocked" role="alert">
        <p>{{ preview.blocked_reason }}</p>
        <button v-if="preview.blocked_code === 'bom_incomplete' && preview.bom_id" type="button" @click="openBom">去 BOM 补齐配料</button>
      </div>
      <p v-if="preview.bom_refresh_pending" class="hint" role="status">本工单尚无实际耗料，将在首次{{ preview.material_consumption_policy === 'bom_start_v1' ? '开工' : '完成' }}时接入已发布的完整配方（{{ preview.effective_bom_version_no }}；原冻结 {{ preview.original_bom_version_no }}）。</p>
      <section v-if="topUpShortages.length || topUpMessage || topUpError || topUpRetry.uncertain || topUpPreview?.blocked_reason" class="wip-top-up" aria-label="WIP 库存使用">
        <p v-if="topUpPreview?.blocked_reason" class="hint">{{ topUpPreview.blocked_reason }}</p>
        <p v-for="item in topUpShortages" :key="item.key">{{ item.name }}：需求 {{ item.required_qty }} {{ item.unit }} · 已投入 {{ item.issued_qty }} {{ item.unit }} · 待补 {{ item.shortage_qty }} {{ item.unit }}</p>
        <p v-if="topUpMessage" class="success" role="status">{{ topUpMessage }}</p>
        <p v-if="topUpError" class="error" role="alert">{{ topUpError }}</p>
        <p v-if="topUpRetry.uncertain" class="hint">尚未收到使用结果，请重试本次操作。</p>
        <button v-if="topUpShortages.length && !topUpOpen" type="button" :disabled="busy || topUpBusy || topUpRefreshNeeded || retry.uncertain" @click="openTopUp">使用 WIP 库存</button>
        <template v-if="topUpOpen && (topUpShortages.length || topUpRetry.uncertain)">
          <p class="hint">选择当前工单需要使用的批次和数量；确认后计入本工单投入，不增加任务数量。</p>
          <div v-for="item in topUpShortages" :key="item.key" class="wip-top-up-material">
            <strong>{{ item.name }} · 待补 {{ item.shortage_qty }} {{ item.unit }}</strong>
            <p v-if="!item.candidates?.length">WIP 暂无可使用批次，请核对库存和占用。</p>
            <div v-for="candidate in item.candidates || []" :key="topUpKey(item,candidate)" class="wip-top-up-candidate">
              <p>{{ candidate.batch_code }} · 货主 {{ candidate.owner_customer_id || '本公司' }} · 可用 {{ candidate.available_qty }} {{ item.unit }} · 质检 {{ topUpQuality(candidate.quality_status) }} · {{ topUpCost(candidate) }}</p>
              <label>本次使用（{{ item.unit }}）<input v-model="topUpQuantities[topUpKey(item,candidate)]" type="number" min="0" :max="candidate.available_qty" step="any" :disabled="topUpBusy || topUpRetry.uncertain || topUpRefreshNeeded || retry.uncertain || !canComplete" /></label>
            </div>
          </div>
          <button type="button" class="primary" :disabled="busy || topUpBusy || topUpRefreshNeeded || retry.uncertain || !canComplete" @click="submitTopUp">{{ topUpBusy ? '正在使用…' : topUpRetry.uncertain ? '重试本次使用' : '确认使用' }}</button>
        </template>
        <button v-if="(topUpOpen || topUpRefreshNeeded || (topUpError && !topUpPreview)) && !topUpRetry.uncertain" type="button" :disabled="topUpBusy || busy || retry.uncertain" @click="recheckTopUp">重新核对 WIP 库存</button>
      </section>
      <div v-if="batchState?.can_initialize" class="batch-init"><p>该工位安排包含多个批次。启用后按批填写投入和产出，已扣用料不会再次出库。</p><button type="button" :disabled="busy" @click="initializeBatches">{{ initRetry.uncertain ? '重试启用' : '启用分批报工' }}</button></div>
      <div v-if="batches.length" class="batch-list" aria-label="执行批次">
        <button v-if="!externalBulk" class="primary" type="button" :disabled="busy || topUpBusy || topUpRetry.uncertain || topUpRefreshNeeded || !canComplete || !!preview.blocked_reason || refreshNeeded || retry.uncertain || !pendingBatches.length" @click="completeAll">完成所有批次</button>
        <div v-for="batch in batches" :key="batch.id" class="batch-row">
          <div><strong>第 {{ batch.batch_no }} 批</strong><small>{{ batch.status==='completed' ? '已完成' : '待完成' }}</small></div>
          <template v-if="batchDrafts[batch.id]">
            <label>投入数量（{{ batch.input_unit || preview.input_unit }}）<input v-model="batchDrafts[batch.id].actual_input_qty" type="number" min="0" step="any" :disabled="batch.status==='completed' || busy || retry.uncertain" /></label>
            <label>实际工时（分钟）<input v-model="batchDrafts[batch.id].actual_minutes" type="number" min="0" step="1" :disabled="batch.status==='completed' || busy || retry.uncertain" /></label>
            <label>产出数量（{{ batch.output_unit || preview.output_unit }}）<input v-model="batchDrafts[batch.id].actual_output_qty" type="number" min="0" step="any" :disabled="batch.status==='completed' || busy || retry.uncertain" /></label>
            <label>备注<input v-model.trim="batchDrafts[batch.id].difference_reason" type="text" placeholder="选填" :disabled="batch.status==='completed' || busy || retry.uncertain" /></label>
            <button v-if="batch.status!=='completed'" type="button" :disabled="busy || topUpBusy || topUpRetry.uncertain || topUpRefreshNeeded || !canComplete || !!preview.blocked_reason || refreshNeeded || retry.uncertain" @click="completeBatch(batch)">批次完成</button>
          </template>
          <small v-if="batch.default_unavailable_reason && batch.status!=='completed'" class="error">{{ batch.default_unavailable_reason }}，请补填实际值</small>
        </div>
      </div>
      <p v-if="batches.length" class="hint">已投入未分配：{{ preview.started_input_qty }} {{ preview.input_unit }}。每批完成后结转对应投入，不重复扣料。</p>
      <p v-else-if="batchState?.reason?.includes('历史汇总')" class="hint">{{ batchState.reason }}</p>
      <fieldset v-if="!batches.length" :disabled="busy || topUpBusy || topUpRetry.uncertain || retry.uncertain || !!success || !!preview.blocked_reason || !!batchState?.can_initialize || (batches.length>0 && !selectedBatch) || batchLoadFailed">
        <label v-if="preview.execution?.reporting_type !== 'incoming'">{{ completionQuantityLabel(preview.output_unit) }}
          <input v-model="draft.actual_output_qty" type="number" min="0" :step="weightUnit(preview.output_unit) ? 'any' : '1'" @input="updateSuggestedMinutes" />
          <small>{{ selectedBatch ? `第 ${selectedBatch.batch_no} 批请填写实际产出` : `本批参考 ${preview.planned_qty} ${preview.output_unit} · 已完成 ${preview.completed_output_qty || 0} ${preview.output_unit}` }}</small>
        </label>
        <template v-if="preview.material_consumption_policy === 'bom_start_v1'">
          <label v-if="preview.execution?.reporting_type !== 'piece'">本次实际处理投入（{{ preview.input_unit }}）<input v-model="draft.actual_input_qty" type="number" min="0" :step="weightUnit(preview.input_unit) ? 'any' : '1'" /><small>本批剩余可处理 {{ preview.input_qty }} {{ preview.input_unit }}</small></label>
          <template v-if="preview.execution?.reporting_type === 'piece'">
            <p>目标 {{ preview.total_planned_qty }} {{ preview.output_unit }} · 已完成 {{ preview.completed_output_qty || 0 }} · 尚需 {{ preview.planned_qty }}</p>
            <div v-if="preview.plan_remainder_eligible === false"><p class="hint">该历史工单未关联可识别生产计划，沿用原余料流程。</p>
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
            </div>
          </template>
          <template v-if="preview.execution?.reporting_type === 'incoming'">
            <label>废料重量（{{ preview.input_unit }}）<input v-model="draft.waste_qty" type="number" min="0" step="any" /></label>
            <p>合格产出：{{ goodOutput }} {{ preview.output_unit }}</p>
          </template>
          <label>实际工时（分钟）<input v-model="draft.actual_minutes" type="number" min="0" step="1" @input="formCache.minutesTouched=true" /></label>
          <label v-if="preview.execution?.reporting_type !== 'piece' && !selectedBatch"><span><input v-model="draft.finish_operation" type="checkbox" /> 本批全部处理结束</span></label>
          <label>备注<textarea v-model.trim="draft.difference_reason" rows="2" /></label>
          <details v-if="preview.started_materials?.length"><summary>已扣用料</summary><p v-for="m in preview.started_materials" :key="m.key">{{ m.name }}：{{ m.qty }} {{ m.unit }} · {{ m.warehouse_name || m.warehouse }}</p></details>
        </template>
      </fieldset>
      <p v-if="preview.material_consumption_policy === 'bom_start_v1'" class="hint">本次报工结转已确认投入的成本，不再扣原料。{{ preview.execution?.reporting_type === 'piece' && preview.plan_remainder_eligible !== false ? '包装余料在本次计划全部包装完成后统一处理。' : '结束批次前须处理剩余投入。' }}</p>
      <p v-else class="hint">系统按冻结的本批投料和 BOM 自动计算用料，完成后自动扣料并{{ preview.execution?.receipt_enabled ? '入库' : '交接' }}。</p>
      <div v-if="!batches.length" class="actions"><button v-if="!success && !batchState?.can_initialize" class="primary" type="button" :disabled="busy || topUpBusy || topUpRetry.uncertain || topUpRefreshNeeded || !canComplete || !!preview.blocked_reason || batchLoadFailed || refreshNeeded || (batches.length>0 && !selectedBatch)" @click="complete">{{ busy ? '处理中…' : retry.uncertain ? '重试本次完成' : selectedBatch ? '完成本批' : '确认工序完成' }}</button></div>
      <details v-if="result?.stock_entry_ids?.length"><summary>关联单据</summary><p v-for="id in result.stock_entry_ids" :key="id">库存单据 #{{ id }}</p></details>
    </template>
    <p v-else>正在读取本批完成数量…</p>
  </section>
</template>
<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { workstationFormCache, clearWorkstationFormCache } from '../lib/workstation-form-cache.js'
import { completionDraft, completionPayload, suggestedCompletionMinutes, completionQuantityLabel, completionRetry, incomingOutput, packagingExpectedRemainder, weightUnit, executionBatchInputQty, executionBatchDraftCache, executionBatchRetryCache } from '../lib/operation-completion.js'
const props=defineProps({jobCardId:{type:Number,required:true},returnParams:{type:Object,default:()=>({})},externalBulk:{type:Boolean,default:false},canComplete:{type:Boolean,default:true}})
const emit=defineEmits(['close','posted','batch-state','top-up-posted'])
const formCache=workstationFormCache('completion',props.jobCardId)
const preview=ref(null),draft=reactive(formCache.draft),busy=ref(false),error=ref(''),success=ref(''),result=ref(null)
const batchState=ref(null),selectedBatch=ref(null),batchLoadFailed=ref(false),refreshNeeded=ref(false)
const batchDrafts=reactive(executionBatchDraftCache(props.jobCardId))
const batches=computed(()=>Array.isArray(batchState.value?.batches)?batchState.value.batches:[])
const pendingBatches=computed(()=>batches.value.filter(b=>b.status==='pending'))
const warehouses=ref([]),remainderTouched=ref(false)
function candidateKey(c){return `${c.batch_code}\u0000${c.warehouse}\u0000${Number(c.owner_customer_id||0)}`}
function selectedCandidate(c){return (draft.selected_unissued_remainders||[]).find(row=>candidateKey(row)===candidateKey(c))}
function toggleCandidate(c,checked){draft.selected_unissued_remainders=(draft.selected_unissued_remainders||[]).filter(row=>candidateKey(row)!==candidateKey(c));if(checked)draft.selected_unissued_remainders.push({batch_code:c.batch_code,warehouse:c.warehouse,owner_customer_id:Number(c.owner_customer_id||0),quantity_g:0})}
const goodOutput=computed(()=>{try{const input=Number(draft.actual_input_qty),waste=Number(draft.waste_qty);return input>=0&&waste>=0&&waste<=input?incomingOutput(input,waste,preview.value?.input_unit,preview.value?.output_unit):'—'}catch{return '—'}})
function updateSuggestedMinutes(){if(preview.value?.execution?.reporting_type==='piece'){if(!formCache.minutesTouched)draft.actual_minutes=suggestedCompletionMinutes(preview.value,draft.actual_output_qty);if(preview.value.plan_remainder_eligible===false){draft.remainder_confirmed=false;if(!remainderTouched.value)draft.actual_remainder_g=packagingExpectedRemainder(preview.value,draft.actual_output_qty)}}}
const retryState=reactive(executionBatchRetryCache(props.jobCardId))
const retry=retryState.retry
const initRetry=reactive(completionRetry())
const topUpCache=workstationFormCache('wip-top-up',props.jobCardId)
const topUpPreview=ref(null),topUpOpen=ref(topUpCache.retry.uncertain),topUpBusy=ref(false),topUpError=ref(''),topUpMessage=ref(''),topUpRefreshNeeded=ref(false)
const topUpQuantities=reactive(topUpCache.draft),topUpRetry=reactive(topUpCache.retry)
const topUpShortages=computed(()=>topUpPreview.value?.supported===false?[]:(topUpPreview.value?.items || []).filter(i=>Number(i.shortage_qty)>0))
function topUpKey(item,candidate){return JSON.stringify([item.key,candidate.batch_code,candidate.warehouse,Number(candidate.owner_customer_id||0)])}
function topUpQuality(value){return ({pass:'合格',unchecked:'未检',hold:'待处理',reject:'不合格'})[value] || value || '未记录'}
function topUpCost(candidate){if(candidate.cost_unavailable || candidate.unit_cost==null)return '成本未记录';return Number(candidate.unit_cost)===0?'账面单价 0（请核对成本）':`账面单价 ${candidate.unit_cost}`}
async function loadTopUp(){topUpBusy.value=true;topUpError.value='';try{const next=await apiGet(`/api/job-cards/${props.jobCardId}/wip-top-up`);topUpPreview.value=next;topUpRefreshNeeded.value=false;if(!topUpRetry.uncertain)topUpRetry.clear()}catch(e){topUpRefreshNeeded.value=!!topUpPreview.value;topUpError.value=`读取 WIP 库存失败：${e.message}`}finally{topUpBusy.value=false}}
async function openTopUp(){topUpOpen.value=true;await loadTopUp()}
async function recheckTopUp(){if(await refresh())await loadTopUp();else{topUpRefreshNeeded.value=true;topUpError.value='工单状态刷新失败，请重新核对'}}
async function submitTopUp(){if(busy.value||topUpBusy.value||topUpRefreshNeeded.value||retry.uncertain||!props.canComplete)return;topUpBusy.value=true;topUpError.value='';topUpMessage.value='';try{let materials=[];if(!topUpRetry.uncertain){for(const item of topUpShortages.value){let total=0;for(const candidate of item.candidates||[]){const raw=topUpQuantities[topUpKey(item,candidate)];if(raw===''||raw==null)continue;const qty=Number(raw);if(!Number.isFinite(qty)||qty<0||qty>Number(candidate.available_qty)+1e-8)throw new Error(`${item.name}使用量不能超过该批次可用量`);if(qty>0){total+=qty;materials.push({key:item.key,batch_code:candidate.batch_code,warehouse:candidate.warehouse,owner_customer_id:Number(candidate.owner_customer_id||0),qty})}}if(total>Number(item.shortage_qty)+1e-8)throw new Error(`${item.name}使用量不能超过本工单缺口`)}if(!materials.length)throw new Error('请填写要使用的 WIP 库存数量')}
 const body=topUpRetry.begin({expected_version:topUpPreview.value.version,materials},()=>crypto.randomUUID());await apiSend(`/api/job-cards/${props.jobCardId}/wip-top-up`,{body});topUpRetry.clear();for(const k of Object.keys(topUpQuantities))delete topUpQuantities[k];topUpOpen.value=false;topUpMessage.value='物料已追加到本工单投入';emit('top-up-posted');if(!await refresh()){topUpRefreshNeeded.value=true;topUpError.value='物料已追加，但工单状态刷新失败，请重新核对';return}await loadTopUp()}catch(e){topUpRetry.fail(e);topUpError.value=topUpRetry.uncertain?'尚未收到使用结果，请重试本次操作，系统不会重复扣料。':e.message;if(!topUpRetry.uncertain&&(e.status===409||/变化|刷新/.test(e.message||'')))topUpRefreshNeeded.value=true}finally{topUpBusy.value=false}}
function syncBatchDrafts(){for(const batch of batches.value){if(batch.status==='completed'){batchDrafts[batch.id]={actual_input_qty:batch.actual_input_qty,actual_output_qty:batch.actual_output_qty,actual_minutes:batch.actual_minutes,difference_reason:batch.remark || ''};continue}if(!batchDrafts[batch.id]){const qty=executionBatchInputQty(batch,preview.value.input_unit);batchDrafts[batch.id]={actual_input_qty:qty ?? '',actual_output_qty:batch.planned_output_qty ?? '',actual_minutes:batch.planned_minutes ?? '',difference_reason:''}}}}
async function loadBatches(){batchState.value=await apiGet(`/api/job-cards/${props.jobCardId}/execution-batches`);syncBatchDrafts();emit('batch-state',batchState.value)}
async function initializeBatches(){busy.value=true;error.value='';try{const body=initRetry.begin({expected_version:batchState.value.version},()=>crypto.randomUUID());batchState.value=await apiSend(`/api/job-cards/${props.jobCardId}/execution-batches`,{body});initRetry.clear();syncBatchDrafts()}catch(e){initRetry.fail(e);error.value=initRetry.uncertain?'未收到启用结果，请刷新后核对，避免重复操作。':e.message}finally{busy.value=false}}
async function recheckKeepingDraft(){busy.value=true;error.value='';try{const [p,state]=await Promise.all([apiGet(`/api/job-cards/${props.jobCardId}/completion-preview`),apiGet(`/api/job-cards/${props.jobCardId}/execution-batches`)]);preview.value=p;batchState.value=state;syncBatchDrafts();refreshNeeded.value=false;retry.clear()}catch(e){error.value=`核对失败：${e.message}`}finally{busy.value=false}}
async function refresh(){busy.value=true;error.value='';try{const p=await apiGet(`/api/job-cards/${props.jobCardId}/completion-preview`);preview.value=p;if(!formCache.initialized){Object.assign(draft,completionDraft(p));formCache.initialized=true}if(!retry.uncertain)retry.clear();if(p.execution?.reporting_type==='piece' && p.plan_remainder_eligible===false){try{const data=await apiGet('/api/stock/warehouses');warehouses.value=(data.rows||[]).filter(w=>w.active!==false&&w.code)}catch{warehouses.value=[]}}if(p.material_consumption_policy==='bom_start_v1'&&!['piece','incoming'].includes(p.execution?.reporting_type)){try{await loadBatches();batchLoadFailed.value=false}catch(e){batchLoadFailed.value=true;error.value=`读取批次失败：${e.message}`}}return true}catch(e){error.value=e.message;return false}finally{busy.value=false}}
function batchPayload(batch){const d=batchDrafts[batch.id];for(const [key,label] of [['actual_input_qty','投入数量'],['actual_minutes','实际工时'],['actual_output_qty','产出数量']]){if(d[key]===''||d[key]===null||d[key]===undefined||!Number.isFinite(Number(d[key]))||Number(d[key])<0)throw new Error(`第 ${batch.batch_no} 批请填写有效的${label}`)}if(!Number.isInteger(Number(d.actual_minutes)))throw new Error(`第 ${batch.batch_no} 批请填写有效工时`);if(Number(d.actual_output_qty)===0&&!String(d.difference_reason||'').trim())throw new Error(`第 ${batch.batch_no} 批产出为零，请填写备注`);return {execution_batch_id:batch.id,actual_input_qty:Number(d.actual_input_qty),actual_output_qty:Number(d.actual_output_qty),actual_minutes:Number(d.actual_minutes),difference_reason:d.difference_reason||'',input_unit:batch.input_unit||preview.value.input_unit,output_unit:batch.output_unit||preview.value.output_unit}}
async function submitBatches(rows,all){if(busy.value||topUpBusy.value||topUpRetry.uncertain||topUpRefreshNeeded.value||!rows.length||!props.canComplete)return;busy.value=true;error.value='';let posted=false;try{const entries=retry.uncertain?[]:rows.map(batchPayload);const body=retry.uncertain?{}:(all?{expected_version:batchState.value.version,batches:entries}:{...entries[0],actual_minutes_explicit:true,expected_version:batchState.value.version});const payload=retry.begin(body,()=>crypto.randomUUID());retryState.rows=rows;retryState.all=all;const data=await apiSend(`/api/job-cards/${props.jobCardId}/${all?'execution-batches/complete-all':'complete'}`,{body:payload});posted=true;retry.clear();success.value='批次记录已保存';try{await loadBatches();if(pendingBatches.value.length)preview.value=await apiGet(`/api/job-cards/${props.jobCardId}/completion-preview`)}catch(e){error.value=`报工已保存，刷新状态失败：${e.message}`;refreshNeeded.value=true}emit('posted',{...data,batchPartial:pendingBatches.value.length>0})}catch(e){if(!posted){retry.fail(e);error.value=retry.uncertain?'尚未收到完成结果，请重试原次操作，系统不会重复扣料。':e.message;if(!retry.uncertain&&(e.status===409||/变化|刷新/.test(e.message||'')))refreshNeeded.value=true}}finally{busy.value=false}}
function completeBatch(batch){submitBatches([batch],false)}
function retryBatchSubmit(){submitBatches(retryState.rows,retryState.all)}
function completeAll(){if(retry.uncertain)retryBatchSubmit();else submitBatches(pendingBatches.value,true)}
defineExpose({completeAll})
function openBom(){window.dispatchEvent(new CustomEvent('kferp:navigate-view',{detail:{key:'productionConfig',params:{tab:'bom',production_bom_id:Number(preview.value?.bom_id||0)},returnNavigation:{key:'workstationView',label:'返回工位任务',params:{...props.returnParams}}}}))}
async function complete(){if(topUpBusy.value||topUpRetry.uncertain||topUpRefreshNeeded.value||!props.canComplete)return;busy.value=true;error.value='';try{if(selectedBatch.value && draft.actual_output_qty!=='' && Number(draft.actual_output_qty)===0 && !String(draft.difference_reason||'').trim())throw new Error('产出为零时请填写原因');const payload=retry.begin({...completionPayload(preview.value,draft,''),...(selectedBatch.value?{execution_batch_id:selectedBatch.value.id}: {})},()=>crypto.randomUUID());const data=await apiSend(`/api/job-cards/${props.jobCardId}/complete`,{body:payload});result.value=data.completion;success.value=data.completion?.message||'工序完成';retry.clear();clearWorkstationFormCache('completion',props.jobCardId);clearWorkstationFormCache('wip-top-up',props.jobCardId);emit('posted',data)}catch(e){retry.fail(e);error.value=retry.uncertain?'尚未收到完成结果，请重试原次操作，系统不会重复扣料。':e.message;if(selectedBatch.value&&!retry.uncertain&&(e.status===409||/变化|刷新/.test(e.message||'')))refreshNeeded.value=true}finally{busy.value=false}}
onMounted(async()=>{await refresh();if(preview.value?.material_consumption_policy==='bom_start_v1' && props.canComplete)await loadTopUp()})
watch(()=>props.canComplete,(enabled,previous)=>{if(enabled&&!previous&&preview.value?.material_consumption_policy==='bom_start_v1'&&!topUpRetry.uncertain)loadTopUp()})
</script>
<style scoped>
.completion-form{grid-column:1/-1;background:#f7faf8;border-top:3px solid #29945e;padding:20px;border-radius:12px;min-width:0}.head,.identity,.actions{display:flex;gap:16px;align-items:center;flex-wrap:wrap}.head{justify-content:space-between}.identity{margin:12px 0}.completion-form label{display:grid;gap:6px;max-width:520px}.completion-form input{border:1px solid #cbd6d0;border-radius:8px;padding:10px;min-width:0;font:inherit}.completion-form small{display:block;color:#64756e;font-size:12px}.completion-form fieldset{border:0;padding:0;min-width:0}.hint{color:#5c7166}.error,.blocked{color:#ae3427}.blocked{padding:12px;background:#fff2f0;border-radius:8px}.success{color:#218450;font-weight:700}button{padding:10px 16px;border:1px solid #cad7cf;border-radius:8px;background:white;font:inherit;cursor:pointer}.primary{background:#2d915e;color:white;border-color:#2d915e}button:disabled{opacity:.5;cursor:default}
.batch-list{display:grid;gap:8px;margin:12px 0}.batch-row{display:grid;grid-template-columns:minmax(76px,.6fr) repeat(3,minmax(105px,1fr)) minmax(130px,1.2fr) auto;gap:10px;align-items:end;padding:12px;background:white;border:1px solid #d7e4dc;border-radius:8px}.batch-row label{font-size:13px}.batch-row input{width:100%;box-sizing:border-box}.batch-row small{margin-top:5px}@media(max-width:760px){.batch-row{grid-template-columns:repeat(2,minmax(0,1fr))}.batch-row>div,.batch-row>button,.batch-row>small{grid-column:1/-1}}.batch-init{padding:12px;background:#eef7f0;border-radius:8px}
.wip-top-up{margin:12px 0;padding:14px;border:1px solid #e5d2a4;border-radius:10px;background:#fff9ed;overflow-wrap:anywhere}.wip-top-up p{margin:8px 0}.wip-top-up-material{padding:10px 0;border-top:1px solid #eadfca}.wip-top-up-candidate{padding:8px 0}.wip-top-up-candidate input{width:min(100%,240px);box-sizing:border-box}.wip-top-up button{margin:6px 8px 0 0}
</style>
