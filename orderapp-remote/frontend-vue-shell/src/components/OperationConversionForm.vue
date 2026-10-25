<template>
 <section class="conversion">
  <div class="head"><h3>核对旧工单，继续生产</h3><button :disabled="busy || uncertain" @click="$emit('close')">关闭</button></div>
  <p>一次核实已完成工序的实际数量、用料和位置。保存后，后序使用已交接物料，完成时自动记账。</p>
  <p v-if="error" class="error" role="alert">{{ error }}</p>
  <p v-if="warehouseError" class="error" role="alert">仓库未加载：{{ warehouseError }}。核对提交已禁用，请刷新后重试。</p>
  <template v-if="preview">
   <strong>{{ preview.output_name || preview.product_name }} · {{ preview.package_spec_label }} · {{ preview.work_order_no }}</strong>
   <p v-if="preview.review_blocked_reason" class="error">{{ preview.review_blocked_reason }}</p>
   <fieldset :disabled="busy || uncertain">
    <article v-for="node in nodes" :key="node.sequence"><strong>{{ node.operation }}</strong><div class="fields"><label>产出单位<input v-model="node.execution.output_unit" /></label><label>新增物料默认来源仓<select v-model="node.execution.source_warehouse"><option v-for="w in warehouses" :key="w.code" :value="w.code">{{ w.name }}</option></select></label><label><span><input v-model="node.execution.receipt_enabled" type="checkbox" :disabled="node.sequence===lastSequence" /> 完成后入库</span><select v-if="node.execution.receipt_enabled" v-model="node.execution.receipt_warehouse"><option v-if="node.sequence===lastSequence" value="">沿用工单：{{ preview.target_warehouse_name }}</option><option v-for="w in warehouses" :key="w.code" :value="w.code">{{ w.name }}</option></select></label></div></article>
    <article v-for="card in cards.filter(c=>c.status==='completed')" :key="card.id"><strong>核实 {{ card.operation }} #{{ card.id }}</strong><p>原投入 {{ card.original_input }}，原产出 {{ card.original_output }}。旧数值只供对照，不自动采用。</p><div class="fields"><label>实际投入<input v-model="card.input_qty" type="number" min="0" step="any" /></label><label>投入单位<input v-model="card.input_unit" /></label><label>实际产出（{{ nodeFor(card).execution.output_unit }}）<input v-model="card.output_qty" type="number" min="0" :step="weightUnit(nodeFor(card).execution.output_unit)?'any':1" /></label></div></article>
    <h4>核实已发生的累计用料</h4><article v-for="m in materials" :key="m.reservation_id"><strong>{{ m.name }}</strong><small>已有耗料 {{ m.consumed_qty }} {{ m.unit }}；未使用的物料填写 0</small><div class="fields"><label>使用工序<select v-model="m.node_key"><option value="">请选择</option><option v-for="node in nodes" :key="node.execution.node_key" :value="node.execution.node_key">{{ node.operation }}</option></select></label><label>累计实际用量（{{ m.unit }}）<input v-model="m.qty" type="number" min="0" :step="weightUnit(m.unit)?'any':1" /></label><label>实际来源仓<select v-model="m.warehouse"><option v-for="w in warehouses" :key="w.code" :value="w.code">{{ w.name }}</option></select></label></div></article>
    <label>核对原因<input v-model="reason" placeholder="记录核实依据和差异原因" /></label>
   </fieldset>
   <button class="primary" :disabled="busy || !!preview.review_blocked_reason || !!warehouseError" @click="submit">{{ uncertain ? '重试原次核对' : '保存核对并继续生产' }}</button>
  </template>
 </section>
</template>
<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { completionRetry, weightUnit } from '../lib/operation-completion.js'
const props=defineProps({workOrderId:{type:Number,required:true}})
const emit=defineEmits(['close','converted'])
const preview=ref(null),cards=ref([]),nodes=ref([]),materials=ref([]),warehouses=ref([]),reason=ref(''),error=ref(''),warehouseError=ref(''),busy=ref(false),uncertain=ref(false)
const retry=completionRetry()
const lastSequence=computed(()=>Math.max(0,...nodes.value.map(n=>n.sequence)))
const nodeFor=card=>nodes.value.find(n=>n.sequence===card.sequence)
function number(value,unit){if(value===''||value==null)throw new Error('请填写现场核实的数量');const n=Number(value);if(!Number.isFinite(n)||n<0)throw new Error('数量必须大于等于零');if(!weightUnit(unit)&&!Number.isInteger(n))throw new Error(`${unit}数必须为整数`);return n}
async function load(){busy.value=true;error.value='';warehouseError.value='';try{const p=await apiGet(`/api/production/work-orders/${props.workOrderId}/conversion-preview`);preview.value=p;cards.value=p.cards.map(c=>({...c,input_qty:'',output_qty:''}));nodes.value=p.cards.filter((c,i,a)=>a.findIndex(n=>n.sequence===c.sequence)===i).map(c=>({...c,execution:{...c.execution}}));materials.value=p.materials.map(m=>({...m,qty:'',node_key:''}))}catch(e){error.value=e.message}try{const w=await apiGet('/api/stock/warehouses');warehouses.value=(Array.isArray(w)?w:w.rows||w.warehouses||[]).filter(row=>row.active!==false)}catch(e){warehouses.value=[];warehouseError.value=e.message||'仓库加载失败'}finally{busy.value=false}}
async function submit(){busy.value=true;error.value='';try{if(!reason.value.trim())throw new Error('请填写核对原因');const payload={work_order_id:props.workOrderId,expected_version:preview.value.version,reason:reason.value.trim(),cards:cards.value.map(c=>({...c,execution:nodeFor(c).execution,input_qty:c.status==='completed'?number(c.input_qty,c.input_unit):null,output_qty:c.status==='completed'?number(c.output_qty,nodeFor(c).execution.output_unit):null})),materials:materials.value.map(m=>({...m,qty:number(m.qty,m.unit)}))};const body=retry.begin(payload,()=>crypto.randomUUID());const data=await apiSend('/api/production/operation-conversion',{body});retry.clear();uncertain.value=false;emit('converted',data)}catch(e){retry.fail(e);uncertain.value=retry.uncertain;error.value=uncertain.value?'尚未收到核对结果，请重试原次核对。':e.message}finally{busy.value=false}}
onMounted(load)
</script>
<style scoped>
.conversion{grid-column:1/-1;background:#eff6f1;border:1px solid #c9dacd;padding:20px;border-radius:12px}.head{display:flex;justify-content:space-between;align-items:center}p,small{color:#5e7466}small{display:block}.fields{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px;margin:10px 0}fieldset{border:0;padding:0;min-width:0}label{display:flex;flex-direction:column;gap:6px}input,select{min-width:0;padding:9px;border:1px solid #bfd0c4;border-radius:7px;font:inherit}article{margin:12px 0;padding:14px;background:white;border-radius:10px}button{padding:10px 16px;border:1px solid #bfd0c4;border-radius:8px;background:white;font:inherit}.primary{background:#2d915e;color:white;margin-top:14px}.error{color:#a83b2c}@media(max-width:700px){.fields{grid-template-columns:1fr}}
</style>
