<template>
  <div class="records-page">
    <ProductionTopNav v-if="!embedded" active-key="productionAcceptance" />
    <ProductionReturnLink :source="viewParams.return_navigation" />
    <header><div><h2>生产记录与核对</h2><p>完成工序即自动用料、交接或入库。这里查看记录、核对旧单和更正实际数量。</p></div><div class="actions"><button @click="navigate('productionManual')">操作帮助</button><button :disabled="busy || uncertain" @click="load">刷新</button></div></header>
    <p v-if="error" class="error" role="alert">{{ error }}</p><p v-if="message" class="success" role="status">{{ message }}</p>
    <nav aria-label="生产记录"><button :class="{active:tab==='records'}" :disabled="uncertain" @click="tab='records'">完成记录</button><button :class="{active:tab==='legacy'}" :disabled="uncertain" @click="tab='legacy'">旧单核对 {{ legacy.length || '' }}</button></nav>
    <section v-if="tab==='records'">
      <p v-if="!busy && !records.length" class="empty">暂无自动完成记录。旧工单请在“旧单核对”中核实。</p>
      <article v-for="row in records" :key="`${row.id}:${row.warehouse}`">
        <div class="record-head"><div><strong>{{ row.product_name }}</strong><p>规格：{{ row.package_spec_label || '无包装规格' }} · {{ row.operation }}</p><p>{{ row.work_order_no }} · {{ row.final ? '已入' : '交接位置' }}{{ row.warehouse_name }}</p></div><div><strong>{{ row.quantity }} {{ row.unit }}</strong><p>{{ row.operator }} · {{ row.created_at }}</p><small v-if="row.estimated_cost">工时成本暂估</small></div><button v-if="!row.legacy" :disabled="busy || uncertain" @click="openCorrection(row)">更正</button><button @click="openWorkOrder(row)">查看工单</button></div>
        <details><summary>数量、成本与关联单据</summary><p>原始数量 {{ row.original_qty }} {{ row.unit }}；当前数量 {{ row.quantity }} {{ row.unit }}</p><div v-for="entry in (row.details || [])" :key="entry.id" class="event"><strong>{{ entry.kind==='correct' ? '更正记录' : '完成记录' }} #{{ entry.id }}</strong><p>{{ entry.operator }} · {{ entry.created_at }} · {{ entry.reason }}</p><p v-if="entry.material_cost != null">材料成本 {{ entry.material_cost }} · 工序成本 {{ entry.operation_cost }}</p><button v-for="id in documentIDs(entry)" :key="id" @click="navigate('stockOperations',{tab:'stockEntries',stock_entry_id:id})">库存单据 #{{ id }}</button></div></details>
        <form v-if="editing?.id===row.id" @submit.prevent="correct(row)" class="edit">
          <fieldset :disabled="busy || uncertain"><label>正确的完成数量（{{ row.unit }}）<input v-model="correction.quantity" type="number" min="0" :step="weightUnit(row.unit)?'any':1" required /></label><label>更正原因<input v-model="correction.reason" required /></label><template v-if="row.correction_cards?.length>1"><label v-for="card in row.correction_cards" :key="card.id">{{ card.operation }} #{{ card.id }} 正确数量<input v-model="correction.card_quantities[card.id]" type="number" min="0" :step="weightUnit(row.unit)?'any':1" required /></label></template></fieldset><p>原始记录保留，保存时同步调整库存差额和相关成本。已被下游使用的数量会列出冲突。</p><button class="primary" :disabled="busy">{{ uncertain ? '重试原次更正' : '保存更正' }}</button><button v-if="!uncertain" type="button" :disabled="busy" @click="editing=null">取消</button>
        </form>
      </article>
    </section>
    <section v-else>
      <p class="notice">请填写现场核实的真实完成数量及累计实际用料；系统会抵扣已有记录，只补缺少的部分。</p>
      <p v-if="!busy && !legacy.length" class="empty">没有待核对的已完成旧工单。</p>
      <article v-for="row in legacy" :key="row.work_order_id">
        <div class="record-head"><label class="selection"><input v-model="selected" type="checkbox" :value="row.work_order_id" :disabled="busy || uncertain || !!row.review_blocked_reason" /><strong>{{ row.output_name || row.product_name }}</strong></label><div><p>规格：{{ row.package_spec_label || '无包装规格' }} · 目标仓：{{ row.target_warehouse_name }}</p><p>{{ row.work_order_no }} · 原完成 {{ row.reported_qty }} {{ row.output_unit }} · 已入库 {{ row.received_qty }} {{ row.output_unit }}</p></div><button @click="openWorkOrder(row)">查看工单</button></div>
        <p v-if="row.review_blocked_reason" class="error">{{ row.review_blocked_reason }}</p>
        <template v-if="selected.includes(row.work_order_id)">
          <fieldset :disabled="busy || uncertain"><div class="fields"><label>现场核实完成数量（{{ row.output_unit }}）<input v-model="drafts[row.work_order_id].actual_output_qty" type="number" min="0" :step="weightUnit(row.output_unit)?'any':1" placeholder="请现场核实后填写" /></label><label>核对原因<input v-model="drafts[row.work_order_id].reason" placeholder="记录核实依据和差异原因" /></label></div>
            <div v-if="row.correction_cards?.length>1" class="fields"><label v-for="card in row.correction_cards" :key="card.id">{{ card.operation }} #{{ card.id }} 实际数量<input v-model="drafts[row.work_order_id].card_quantities[card.id]" type="number" min="0" :step="weightUnit(row.output_unit)?'any':1" /></label></div>
            <h4>累计实际用料</h4><div v-for="material in drafts[row.work_order_id].materials" :key="material.reservation_id" class="material"><span>{{ material.name }}<small>已有耗料 {{ material.consumed_qty }} {{ material.unit }}</small></span><label>核实总用量（{{ material.unit }}）<input v-model="material.qty" type="number" min="0" :step="weightUnit(material.unit)?'any':1" placeholder="请填写实际用量" /></label><label>来源仓<select v-model="material.warehouse"><option v-for="w in warehouses" :key="w.code" :value="w.code">{{ w.name }}</option></select></label></div>
          </fieldset>
        </template>
        <p v-if="results[row.work_order_id]" :class="results[row.work_order_id].succeeded ? 'success':'error'">{{ resultText(results[row.work_order_id]) }}</p>
      </article>
      <div v-if="selected.length || uncertain" class="submit-bar"><button class="primary" :disabled="busy" @click="reconcile">{{ uncertain ? '重试原次核对' : `核对并补齐记录（${selected.length} 张）` }}</button></div>
      <section v-if="Object.values(results).some(r=>r.succeeded)"><h3>本次已完成</h3><p v-for="row in Object.values(results).filter(r=>r.succeeded)" :key="row.work_order_id" class="success">工单 #{{ row.work_order_id }}：{{ resultText(row) }}</p></section>
    </section>
  </div>
</template>
<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import ProductionTopNav from '../components/ProductionTopNav.vue'
import ProductionReturnLink from '../components/ProductionReturnLink.vue'
import { completionRetry, weightUnit } from '../lib/operation-completion.js'
import { correctionPayload, legacyReviewDraft, legacyReviewPayload } from '../lib/production-records.js'
const props=defineProps({embedded:Boolean,viewParams:{type:Object,default:()=>({})}})
const records=ref([]),legacy=ref([]),warehouses=ref([]),selected=ref([]),drafts=reactive({}),results=reactive({}),tab=ref('records'),busy=ref(false),error=ref(''),message=ref(''),editing=ref(null),correction=reactive({quantity:'',reason:''}),uncertain=ref(false)
const correctionAttempt=completionRetry()
let reviewAttempt=null
const workID=()=>Number(props.viewParams.work_order_id)||0
async function load(){busy.value=true;error.value='';try{const [a,b,w]=await Promise.all([apiGet(`/api/production/records?work_order_id=${workID()}`),apiSend('/api/production/legacy-review/preview',{body:{work_order_ids:workID()?[workID()]:[]}}),apiGet('/api/warehouses')]);records.value=a.rows||[];legacy.value=b.rows||[];warehouses.value=(Array.isArray(w)?w:w.rows||w.warehouses||[]);for(const row of legacy.value)drafts[row.work_order_id] ||= legacyReviewDraft(row);selected.value=selected.value.filter(id=>legacy.value.some(row=>row.work_order_id===id));}catch(e){error.value=e.message}finally{busy.value=false}}
function openCorrection(row){editing.value=row;correction.quantity=row.quantity;correction.reason='';correction.card_quantities=Object.fromEntries((row.correction_cards||[]).map(c=>[c.id,c.actual_output_qty]));correctionAttempt.clear();error.value=''}
async function correct(row){busy.value=true;error.value='';try{const body=correctionAttempt.begin(correctionPayload(row,correction,''),()=>crypto.randomUUID());await apiSend(`/api/produce/job-cards/${row.job_card_id}/correct-output`,{body});correctionAttempt.clear();uncertain.value=false;editing.value=null;message.value='更正已保存，库存差额和成本已同步记录';await load()}catch(e){correctionAttempt.fail(e);uncertain.value=correctionAttempt.uncertain;error.value=uncertain.value?'尚未收到结果，输入已保留，请重试原次更正。':e.message}finally{busy.value=false}}
async function reconcile(){busy.value=true;error.value='';try{if(!reviewAttempt)reviewAttempt={items:legacy.value.filter(row=>selected.value.includes(row.work_order_id)).map(row=>legacyReviewPayload(row,drafts[row.work_order_id],crypto.randomUUID()))};if(!reviewAttempt.items.length)throw new Error('请选择已核实的工单');const data=await apiSend('/api/production/legacy-review/submit',{body:reviewAttempt});for(const row of data.rows||[])results[row.work_order_id]=row;const success=(data.rows||[]).filter(row=>row.succeeded);message.value=`已核对 ${success.length} 张工单；${(data.rows||[]).length-success.length} 张需继续处理`;selected.value=selected.value.filter(id=>!success.some(row=>row.work_order_id===id));reviewAttempt=null;uncertain.value=false;await load()}catch(e){uncertain.value=!!reviewAttempt&&(!e.status||e.status>=500);if(!uncertain.value)reviewAttempt=null;error.value=uncertain.value?'尚未收到核对结果，请重试原次核对，已成功部分不会重复记账。':e.message}finally{busy.value=false}}
function documentIDs(entry){return [...new Set([...(entry.result?.stock_entry_ids||[]),entry.result?.stock_entry_id].filter(Boolean))]}
function resultText(row){return row.succeeded?`${row.completion?.message} · ${row.completion?.output_qty} ${row.completion?.output_unit} · 单据 ${(row.completion?.stock_entry_ids||[]).join('、')}`:row.error}
function navigate(key,params={}){window.dispatchEvent(new CustomEvent('kferp:navigate-view',{detail:{key,params,returnNavigation:{key:'productionAcceptance',label:'返回生产记录',params:props.viewParams}}}))}
function openWorkOrder(row){navigate('workOrders',{work_order_id:row.work_order_id})}
watch(()=>props.viewParams.work_order_id,()=>{if(!uncertain.value)load()})
onMounted(load)
</script>
<style scoped>
.records-page{padding:20px;color:#203c31;background:#f6f8f7;min-height:100%}header,.record-head,.actions{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap}h2{margin:0}p{margin:8px 0;color:#62766c}nav{display:flex;gap:8px;margin:20px 0}article{background:white;border:1px solid #dce4df;border-radius:14px;padding:22px;margin:16px 0}button{background:white;border:1px solid #c6d3cb;border-radius:8px;padding:10px 16px;font:inherit;cursor:pointer}button:disabled{opacity:.5;cursor:default}.active,.primary{background:#2d915e;border-color:#2d915e;color:white}.error{color:#ab3f29}.success{color:#21824b}.notice,.edit{background:#edf5ef;border-radius:10px;padding:18px}.fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:18px}label{display:flex;flex-direction:column;gap:6px}input,select{min-width:0;border:1px solid #c6d3cb;border-radius:8px;padding:10px;font:inherit}fieldset{border:0;padding:0;min-width:0}.selection{flex-direction:row;align-items:center}.material{display:grid;grid-template-columns:1fr 1fr 1fr;align-items:center;gap:16px;margin:12px 0}small{display:block;color:#687b71}details{margin-top:16px}.event{padding:12px;border-bottom:1px solid #e8eeeb}.event button{margin-right:8px}.submit-bar{position:sticky;bottom:0;padding:14px;background:#f6f8f7}.empty{padding:32px;text-align:center}@media(max-width:700px){.fields,.material{grid-template-columns:1fr}.records-page{padding:12px}}
</style>
