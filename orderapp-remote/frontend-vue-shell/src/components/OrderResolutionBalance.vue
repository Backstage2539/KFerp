<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { apiSend } from '../api/client'
import { blankResolutionStep, resolutionStepPayload } from '../lib/order-resolution'
const props=defineProps({orderId:Number,balances:{type:Array,default:()=>[]},targets:{type:Array,default:()=>[]},options:Object,permissions:Object,history:{type:Array,default:()=>[]}})
const emit=defineEmits(['changed'])
const form=reactive(blankResolutionStep()),sourceID=ref(0),busy=ref(false),error=ref(''),expanded=ref(false)
const allowed=computed(()=>Boolean(props.permissions?.['finance.resolve']))
const choices=computed(()=>(props.options?.funds||[]).filter(v=>['refund','offset'].includes(v.value)))
const total=computed(()=>props.balances.reduce((sum,v)=>sum+Number(v.available_cents),0))
let key=''
watch([()=>props.balances,()=>props.targets],()=>{key=''}, {deep:true})
watch([form,sourceID],()=>{key=''}, {deep:true,flush:'sync'})
async function submit(){
 if(!allowed.value||busy.value)return
 error.value=''
 const source=props.balances.find(v=>Number(v.id)===Number(sourceID.value))
 if(!source){error.value='请选择客户余额来源';return}
 const target=props.targets.find(v=>Number(v.id)===Number(form.target_id))
 if(form.choice==='offset'&&!target){error.value='请选择具体应收';return}
 busy.value=true
 try{
  if(!key)key=crypto.randomUUID()
  await apiSend(`/api/orders/${props.orderId}/resolution/balance`,{body:{...resolutionStepPayload(form,key),source_entry_id:source.id,expected_available_cents:source.available_cents,expected_target_due_cents:form.choice==='offset'?target.available_cents:0}})
  Object.assign(form,blankResolutionStep());sourceID.value=0;expanded.value=false;emit('changed')
 }catch(e){error.value=e.message}finally{busy.value=false}
}
</script>
<template>
 <section v-if="allowed" class="balance-panel">
  <h3>客户余额 · 可用 ¥{{ (total/100).toFixed(2) }}</h3>
  <p>来源为已确认留存的款项。使用余额不改写原收款；选择实际退款或抵扣应收后分别登记。</p>
  <button v-if="balances.length" type="button" @click="expanded=!expanded">{{ expanded?'收起':'处理客户余额' }}</button>
  <fieldset v-if="expanded" :disabled="busy">
   <label>余额来源<select v-model.number="sourceID"><option :value="0">请选择来源</option><option v-for="v in balances" :key="v.id" :value="v.id">{{ v.order_no }} · {{ v.posting_date }} · 可用 ¥{{ (v.available_cents/100).toFixed(2) }}</option></select></label>
   <label>处理方式<select v-model="form.choice"><option value="">请选择</option><option v-for="v in choices" :key="v.value" :value="v.value" :disabled="v.disabled">{{ v.label }}{{ v.disabled?`（${v.reason}）`:'' }}</option></select></label>
   <label>金额（元）<input v-model="form.amount" inputmode="decimal" placeholder="填写实际金额" /></label>
   <label v-if="form.choice==='offset'">抵扣应收<select v-model.number="form.target_id"><option :value="0">请选择同客户应收</option><option v-for="v in targets" :key="v.id" :value="v.id">{{ v.label }} · 待收 ¥{{ (v.available_cents/100).toFixed(2) }}</option></select></label>
   <template v-if="form.choice==='refund'"><label>实际付款账户<input v-model="form.account" /></label><label>付款凭据<input v-model="form.reference" /></label><p>先完成实际付款再登记；系统不会代为转账。</p></template>
   <label>实际处理日期<input v-model="form.posting_date" type="date" /></label><label>说明<textarea v-model="form.note" rows="2" /></label>
   <p v-if="error" role="alert" class="error">{{ error }}</p><button type="button" :disabled="busy||!sourceID||!form.choice" @click="submit">确认使用客户余额</button>
  </fieldset>
  <details v-if="history.length"><summary>余额使用记录（最近 100 笔）</summary><article v-for="v in history" :key="v.id"><strong>{{ v.choice==='offset'?'抵扣应收':'实际退款' }} ¥{{ (v.amount_cents/100).toFixed(2) }}</strong><p>来源 {{ v.order_no }} · {{ v.posting_date }} · {{ v.actor }}</p><p v-if="v.target_order_no">抵扣 {{ v.target_order_no }}</p><p v-if="v.reference">凭据 {{ v.reference }} · {{ v.account }}</p><p>{{ v.note }}</p></article></details>
 </section>
</template>
<style scoped>
.balance-panel{background:#fff;padding:16px;border:1px solid #d6e0d9;border-radius:10px}.balance-panel p{font-size:13px;color:#62776a}details{margin-top:12px}article{padding:12px 0;border-bottom:1px solid #d6e0d9}fieldset{border:0;padding:12px 0;display:grid;gap:12px;min-width:0}label{display:flex;flex-direction:column;gap:6px}input,select,textarea{box-sizing:border-box;width:100%;padding:9px;border:1px solid #bccdc1;border-radius:6px;font:inherit;background:white;color:inherit}button{padding:9px 14px;border:1px solid #b8cbbf;border-radius:6px;background:white;color:#285e49;cursor:pointer}button:disabled{opacity:.5;cursor:default}.balance-panel .error{color:#ab3026}
</style>
