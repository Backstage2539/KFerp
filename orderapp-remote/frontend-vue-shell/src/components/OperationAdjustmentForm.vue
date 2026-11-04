<template>
<section class="adjustment" aria-label="本批用料调整">
 <div class="head"><h3>本批用料调整</h3><button :disabled="busy||retry.uncertain" @click="$emit('close')">关闭</button></div>
 <p v-if="error" role="alert">{{error}}</p>
 <template v-if="preview">
 <fieldset :disabled="busy||retry.uncertain">
 <label>操作<select v-model="kind" @change="reset"><option value="material_add">补料</option><option value="material_return">退料</option><option value="start_reversal">撤销误开工</option></select></label>
 <template v-if="kind==='material_add'">
 <label>增加的待加工投入（{{preview.input_unit}}）<input type="number" min="0" step="any" v-model.number="actualInput" /></label>
 <label v-for="m in materials" :key="m.key">{{m.name}}（{{m.unit}}） · {{m.warehouse_name||m.warehouse}}<input type="number" min="0" step="any" v-model.number="m.qty" /></label>
 </template>
 <template v-if="kind==='material_return'">
 <label>原投入记录<select v-model.number="inputID" @change="selectInput"><option v-for="i in preview.started_inputs||[]" :value="i.id" :key="i.id">投入 {{i.id}} · 剩余 {{i.remaining_qty}} {{i.unit}}</option></select></label>
 <label>减少的待加工投入（{{preview.input_unit}}）<input type="number" min="0" step="any" v-model.number="actualInput" /></label>
 <label v-for="c in returns" :key="c.id">{{c.name}} · 批次 {{c.batch_code}} · {{c.warehouse}} · 货主 {{c.owner_customer_id||'本公司'}} · 可退 {{c.remaining_qty}} {{c.unit}}<input type="number" min="0" :max="c.remaining_qty" step="1" v-model.number="c.qty" /></label>
 </template>
 <p v-if="kind==='start_reversal'">仅无报工的误开工可撤销。原耗料保留，未结转的净投入按原批次和货主退回。</p>
 <label v-if="kind!=='material_add'"><span><input type="checkbox" v-model="unprocessed" /> 确认所退物料尚未投入加工</span></label>
 <label>原因（必填）<textarea v-model.trim="reason" rows="2" /></label>
 </fieldset>
 <button :disabled="busy||!reason||(kind!=='material_add'&&!unprocessed)" @click="submit">{{retry.uncertain?'重试本次操作':'确认调整'}}</button>
 </template>
</section>
</template>
<script setup>
import {onMounted,reactive,ref} from 'vue'
import {apiGet,apiSend} from '../api/client'
import {completionRetry} from '../lib/operation-completion.js'
const props=defineProps({jobCardId:{type:Number,required:true}}),emit=defineEmits(['close','posted'])
const preview=ref(null),kind=ref('material_add'),materials=ref([]),returns=ref([]),actualInput=ref(0),inputID=ref(0),unprocessed=ref(false),reason=ref(''),busy=ref(false),error=ref(''),retry=reactive(completionRetry())
function selectInput(){const i=(preview.value.started_inputs||[]).find(i=>i.id===inputID.value);actualInput.value=0;returns.value=(i?.components||[]).map(c=>({...c,qty:0}))}
function reset(){actualInput.value=0;unprocessed.value=false;if(kind.value==='material_return'){inputID.value=preview.value.started_inputs?.[0]?.id||0;selectInput()}}
onMounted(async()=>{busy.value=true;try{preview.value=await apiGet(`/api/job-cards/${props.jobCardId}/start-preview`);materials.value=(preview.value.materials||[]).map(m=>({...m,qty:0}))}catch(e){error.value=e.message}finally{busy.value=false}})
async function submit(){busy.value=true;error.value='';try{const body=retry.begin({kind:kind.value,expected_version:preview.value.version,reason:reason.value,actual_input_qty:actualInput.value,input_unit:preview.value.input_unit,source_input_id:inputID.value,unprocessed_confirmed:unprocessed.value,materials:materials.value,returns:returns.value.filter(c=>c.qty>0).map(c=>({id:c.id,qty:c.qty}))},()=>crypto.randomUUID());const result=await apiSend(`/api/job-cards/${props.jobCardId}/${kind.value==='start_reversal'?'start-reversal':'material-adjustments'}`,{body});retry.clear();emit('posted',result)}catch(e){retry.fail(e);error.value=e.message}finally{busy.value=false}}
</script>
<style scoped>
.adjustment{grid-column:1/-1;background:#f7faf8;border:1px solid #c8dacf;padding:20px;border-radius:12px}.head{display:flex;justify-content:space-between}fieldset{border:0;padding:0}label{display:grid;gap:6px;margin:12px 0}input,select,textarea{font:inherit;padding:8px;max-width:30rem}button{padding:10px 16px;cursor:pointer}[role=alert]{color:#a53525}
</style>
