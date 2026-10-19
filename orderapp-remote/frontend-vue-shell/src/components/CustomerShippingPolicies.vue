<template>
 <section class="policies"><h3>客户运费政策</h3><p>三种发货场景分别设置。预估运费下单时不收取；实际账单确认后收取实际运费＋本次冻结的附加运费。包邮免两项运费，代发服务费照常。</p><p v-if="error" class="error" role="alert">{{error}}</p><p v-if="message" class="ok" role="status">{{message}}</p>
  <fieldset v-for="p in policies" :key="p.scene" :disabled="saving"><legend>{{labels[p.scene]}} · {{p.revision?`V${p.revision}`:'未配置'}}</legend><div class="fields"><label>默认运费模板<select v-model.number="p.template_id"><option :value="0">请选择</option><option v-for="t in templates" :key="t.id" :value="t.id" :disabled="!t.active">{{t.name}} · V{{t.revision}}{{t.active?'':'（停用）'}}</option></select></label><label>每票固定附加运费（元）<input v-model="p.surcharge" type="number" min="0" step="0.01" /></label><label class="check"><input v-model="p.free_shipping" type="checkbox" />客户包邮</label></div><button :disabled="saving" @click="save(p)">保存{{labels[p.scene]}}政策</button></fieldset>
 </section>
</template>
<script setup>
import {onMounted,ref,watch} from 'vue'
import {apiGet,apiSend} from '../api/client'
import {explicitMoneyCents} from '../lib/fulfillment-settings'
const props=defineProps({customerId:{type:Number,required:true}}),emit=defineEmits(['saved'])
const labels={sale:'普通发货录单',stock:'库存代发',on_demand:'无库存代发'},policies=ref([]),templates=ref([]),error=ref(''),message=ref(''),saving=ref(false);let sequence=0
async function load(){const version=++sequence;error.value='';try{const [p,t]=await Promise.all([apiGet(`/api/customer-fulfillment/${props.customerId}/shipping-policies`),apiGet('/api/settings/logistics/shipping-templates')]);if(version!==sequence)return;policies.value=p.policies.map(x=>({...x,surcharge:(x.surcharge_cents/100).toFixed(2)}));templates.value=t.templates||[]}catch(e){if(version===sequence)error.value=e.message||'政策加载失败'}}
async function save(p){if(saving.value)return;saving.value=true;error.value='';message.value='';try{const r=await apiSend(`/api/customer-fulfillment/${props.customerId}/shipping-policies/${p.scene}`,{method:'PUT',body:{template_id:p.template_id,surcharge_cents:explicitMoneyCents(p.surcharge),free_shipping:p.free_shipping,expected_revision:p.revision}});Object.assign(p,r);message.value='运费政策已保存，之后的新订单使用新政策';emit('saved')}catch(e){error.value=e.message||'保存失败'}finally{saving.value=false}}
watch(()=>props.customerId,load);onMounted(load)
</script>
<style scoped>
.policies{margin-top:16px}h3{font-size:15px;color:#245c41}p{font-size:12px;color:#63756a;line-height:1.7}fieldset{border:1px solid #dce5df;border-radius:8px;padding:12px;margin:12px 0}legend{font-size:13px;color:#28573b}.fields{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:10px}label{display:grid;gap:6px;font-size:12px}input,select{box-sizing:border-box;width:100%;padding:8px;border:1px solid #cbd9cf;border-radius:6px}.check{display:flex;align-items:center}.check input{width:18px}button{margin-top:10px;padding:8px;border:1px solid #286248;border-radius:6px;background:#286248;color:white}.error{color:#ad3529}.ok{color:#286248}
</style>
