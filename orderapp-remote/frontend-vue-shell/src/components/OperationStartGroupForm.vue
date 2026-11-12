<template>
<section class="start-group" aria-label="合并开工确认">
 <div class="head"><h3>合并开工确认</h3><button :disabled="busy||retry.uncertain" @click="$emit('close')">关闭</button></div>
 <p>各工单分别确认投入与客户工序备注；一起开工，分别扣料。任一项失败均不会扣料。</p>
 <p v-if="error" role="alert">{{error}}</p>
 <button type="button" :disabled="busy||retry.uncertain" @click="refreshPreview">刷新核对</button>
 <small>刷新保留各单已填投入和备注；确认后按最新库存与版本开工。</small>
 <fieldset v-for="(p,index) in preview?.tasks||[]" :key="p.job_card_id" :class="{failed:taskHasError(p)}" :disabled="busy||retry.uncertain">
 <legend>{{sourceTask(p).work_order_no || `工单 ${p.work_order_id}`}} · {{p.product_name}} · {{sourceTask(p).spec_label || '规格未记录'}}</legend>
 <div class="source-orders" v-if="sourceTask(p).related_orders?.length"><div v-for="order in sourceTask(p).related_orders" :key="`${order.order_id}:${order.order_item_id}`"><strong>{{order.order_no}} · {{order.customer_name || '客户未记录'}}</strong><span v-if="order.late_association"> · 后关联</span><p v-if="order.original_order_note">原订单备注：{{order.original_order_note}}</p><p v-if="order.original_item_note">原商品备注：{{order.original_item_note}}</p><p v-for="(note,noteIndex) in order.operation_notes || []" :key="noteIndex">{{note.operation_name}}：{{note.note}}</p></div></div>
 <p>本批计划产出 {{p.planned_qty}} {{p.output_unit}}；可用批次在提交时再次核对。</p>
 <label>本批实际投入（{{p.input_unit}}）<input type="number" min="0" step="any" v-model.number="drafts[index].actual_input_qty" @change="recalculate(index)" /></label>
 <label v-for="m in drafts[index].materials" :key="m.key">{{m.name}}（{{m.unit}}） · {{m.warehouse_name||m.warehouse}} · 货主 {{m.owner_customer_id||'本公司'}}<input type="number" min="0" step="any" v-model.number="m.qty" /><small>可用批次：{{ (m.eligible_batches||[]).map(b=>`${b.batch_code}（${b.available_qty} ${m.unit}）`).join('、') || '暂无' }}</small><small v-if="materialShortage(m)>0" class="shortage">WIP 还缺 {{materialShortage(m)}} {{m.unit}}，请补齐物料后刷新核对。</small></label>
 <label>备注<input v-model.trim="drafts[index].difference_reason" /></label>
 </fieldset>
 <button :disabled="busy||!isCurrentPreview" @click="submit">{{retry.uncertain?'重试本次合并开工':'确认整组开工并扣料'}}</button>
</section>
</template>
<script setup>
import {computed,onBeforeUnmount,reactive,ref,watch} from 'vue'
import {apiSend} from '../api/client'
import {completionRetry} from '../lib/operation-completion.js'
import {startDraft,startPayload,startSuggestedMaterials,refreshStartDraft} from '../lib/operation-start.js'
const props=defineProps({taskIds:{type:Array,required:true},sourceTasks:{type:Array,default:()=>[]}}),emit=defineEmits(['close','posted','lock-change'])
const preview=ref(null),previewChecked=ref(false),drafts=ref([]),busy=ref(false),submitting=ref(false),error=ref(''),retry=reactive(completionRetry())
const selectionKey=computed(()=>idsKey(props.taskIds))
const isCurrentPreview=computed(()=>previewChecked.value && Boolean(preview.value) && idsKey(preview.value.tasks.map(task=>task.job_card_id))===selectionKey.value)
let previewRequestVersion=0
function idsKey(ids){return [...ids].map(Number).sort((a,b)=>a-b).join(',')}
function taskHasError(p){return error.value.includes(`工序卡 ${p.job_card_id}：`) || Boolean(sourceTask(p).work_order_no && error.value.includes(sourceTask(p).work_order_no))}
function sourceTask(p){return props.sourceTasks.find(task=>Number(task.job_card_id)===Number(p.job_card_id)) || {}}
function recalculate(i){drafts.value[i].materials=startSuggestedMaterials(preview.value.tasks[i],drafts.value[i].actual_input_qty)}
function materialShortage(m){return m.warehouse==='wip'?Math.max(0,Number((Number(m.qty||0)-Number(m.available_qty||0)).toFixed(6))):0}
function materialIdentity(m){return [m.key,m.material_id||0,m.product_id||0,m.bom_spec_id||0,m.bom_variant_id||0,m.unit,m.warehouse,m.owner_customer_id||0].join(':')}
function refreshedDraft(task,old){
 if(!old)return startDraft(task)
 const oldMaterials=new Map((old.draft.materials||[]).map(m=>[materialIdentity(m),m]))
 if(task.input_unit!==old.task.input_unit || oldMaterials.size!==(task.materials||[]).length || (task.materials||[]).some(m=>!oldMaterials.has(materialIdentity(m)))){
  throw new Error(`工序卡 ${task.job_card_id}：用料清单或单位已变化（${(old.draft.materials||[]).map(m=>m.name).join('、') || old.task.input_unit}），原输入已保留；请核对后调整所选工单。`)
 }
 return refreshStartDraft(task,old.draft)
}
function refreshPreview(){if(!busy.value&&!retry.uncertain)void loadPreview(true)}
async function loadPreview(preserve=false){
 const version=++previewRequestVersion,selected=selectionKey.value,ids=[...props.taskIds]
 const keep=preserve && preview.value && idsKey(preview.value.tasks.map(task=>task.job_card_id))===selected
 const old=new Map(keep?preview.value.tasks.map((task,i)=>[task.job_card_id,{task,draft:drafts.value[i]}]):[])
 if(!keep){preview.value=null;drafts.value=[]}
 previewChecked.value=false;retry.clear();error.value='';busy.value=true
 try{
  const result=await apiSend('/api/production/workstation/start/preview',{body:{job_card_ids:ids}})
  if(version!==previewRequestVersion || selected!==selectionKey.value)return
  if(idsKey((result.tasks||[]).map(task=>task.job_card_id))!==selected)throw new Error('所选工单已有不可用项，预览与当前选择不一致；已填输入已保留，请重新核对所选工单。')
  const updated=result.tasks.map(task=>{
   if(task.blocked_reason)throw new Error(`工序卡 ${task.job_card_id}：${task.blocked_reason}；已填输入已保留。`)
   return refreshedDraft(task,old.get(task.job_card_id))
  })
  preview.value=result;drafts.value=updated;previewChecked.value=true
 }catch(e){if(version===previewRequestVersion)error.value=e.message}
 finally{if(version===previewRequestVersion)busy.value=false}
}
watch(selectionKey,()=>loadPreview(),{immediate:true})
watch(()=>submitting.value||retry.uncertain,value=>emit('lock-change',value),{flush:'sync'})
onBeforeUnmount(()=>{++previewRequestVersion;emit('lock-change',false)})
async function submit(){
 if(busy.value || !isCurrentPreview.value)return
 busy.value=true;submitting.value=true;error.value=''
 const submittedIDs=preview.value.tasks.map(task=>task.job_card_id)
 try{
  const body=retry.begin({expected_version:preview.value.version,items:preview.value.tasks.map((p,i)=>({...startPayload(p,drafts.value[i],''),id:p.job_card_id}))},()=>crypto.randomUUID())
  const result=await apiSend('/api/production/workstation/start/submit',{body})
  retry.clear();emit('posted',result,submittedIDs)
 }catch(e){retry.fail(e);if(e.status&&!retry.uncertain)previewChecked.value=false;error.value=e.message}
 finally{submitting.value=false;busy.value=false}
}
</script>
<style scoped>
.start-group{padding:20px;background:#f6faf7;border:1px solid #b7cfbf;border-radius:12px;margin-top:15px}.head{display:flex;justify-content:space-between}fieldset{margin:16px 0;border:1px solid #ceddd3;padding:16px}fieldset.failed{border-color:#bb4b38;background:#fff5f2}.source-orders p{margin:.35rem 0;color:#5b665e}label{display:grid;gap:6px;margin:12px 0}input{font:inherit;padding:8px;max-width:28rem}button{padding:10px 16px;cursor:pointer}[role=alert],.shortage{color:#a53525}
</style>
