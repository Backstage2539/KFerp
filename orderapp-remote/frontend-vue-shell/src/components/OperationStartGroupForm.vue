<template>
<section class="start-group" aria-label="合并开工确认">
 <div class="head"><h3>合并开工确认</h3><button :disabled="busy||retry.uncertain" @click="$emit('close')">关闭</button></div>
 <p>各来源任务分别确认投入；整组一起开工，任一项失败均不会扣料。</p>
 <p v-if="error" role="alert">{{error}}</p>
 <fieldset v-for="(p,index) in preview?.tasks||[]" :key="p.job_card_id" :disabled="busy||retry.uncertain">
 <legend>工单 {{p.work_order_id}} · {{p.product_name}} · BOM {{p.effective_bom_version_no||p.bom_version_id}}</legend>
 <p>本批计划产出 {{p.planned_qty}} {{p.output_unit}}；可用批次在提交时再次核对。</p>
 <label>本批实际投入（{{p.input_unit}}）<input type="number" min="0" step="any" v-model.number="drafts[index].actual_input_qty" @change="recalculate(index)" /></label>
 <label v-for="m in drafts[index].materials" :key="m.key">{{m.name}}（{{m.unit}}） · {{m.warehouse_name||m.warehouse}} · 货主 {{m.owner_customer_id||'本公司'}}<input type="number" min="0" step="any" v-model.number="m.qty" /><small>可用批次：{{ (m.eligible_batches||[]).map(b=>`${b.batch_code}（${b.available_qty} ${m.unit}）`).join('、') || '暂无' }}</small></label>
 <label>用量差异原因<input v-model.trim="drafts[index].difference_reason" /></label>
 </fieldset>
 <button :disabled="busy||!preview" @click="submit">{{retry.uncertain?'重试本次合并开工':'确认整组开工并扣料'}}</button>
</section>
</template>
<script setup>
import {onMounted,reactive,ref} from 'vue'
import {apiSend} from '../api/client'
import {completionRetry} from '../lib/operation-completion.js'
import {startDraft,startPayload,startSuggestedMaterials} from '../lib/operation-start.js'
const props=defineProps({taskIds:{type:Array,required:true}}),emit=defineEmits(['close','posted'])
const preview=ref(null),drafts=ref([]),busy=ref(false),error=ref(''),retry=reactive(completionRetry())
function recalculate(i){drafts.value[i].materials=startSuggestedMaterials(preview.value.tasks[i],drafts.value[i].actual_input_qty)}
onMounted(async()=>{busy.value=true;try{preview.value=await apiSend('/api/production/workstation/start/preview',{body:{job_card_ids:props.taskIds}});drafts.value=preview.value.tasks.map(startDraft)}catch(e){error.value=e.message}finally{busy.value=false}})
async function submit(){busy.value=true;error.value='';try{const body=retry.begin({expected_version:preview.value.version,items:preview.value.tasks.map((p,i)=>({...startPayload(p,drafts.value[i],''),id:p.job_card_id}))},()=>crypto.randomUUID());const result=await apiSend('/api/production/workstation/start/submit',{body});retry.clear();emit('posted',result)}catch(e){retry.fail(e);error.value=e.message}finally{busy.value=false}}
</script>
<style scoped>
.start-group{padding:20px;background:#f6faf7;border:1px solid #b7cfbf;border-radius:12px;margin-top:15px}.head{display:flex;justify-content:space-between}fieldset{margin:16px 0;border:1px solid #ceddd3;padding:16px}label{display:grid;gap:6px;margin:12px 0}input{font:inherit;padding:8px;max-width:28rem}button{padding:10px 16px;cursor:pointer}[role=alert]{color:#a53525}
</style>
