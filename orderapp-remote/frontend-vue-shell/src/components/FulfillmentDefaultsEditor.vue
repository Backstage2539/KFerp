<template>
 <section class="defaults-editor">
  <p class="default-impact">共享默认按场景维护。保存后，使用默认的字段会随新规则生效；客户自定义项、仓库与历史订单保持原值。</p>
  <p v-if="loading" role="status">正在加载默认配置…</p><p v-if="error" role="alert" class="error">{{ error }}</p><p v-if="message" role="status" class="success">{{ message }}</p>
  <form v-if="loaded" @submit.prevent="save">
   <p class="version">当前默认 V{{ revision }} · {{ inheritingCustomers }} 位客户正在使用继承配置</p>
   <fieldset v-for="mode in form.modes" :key="mode.mode" :disabled="saving"><legend>{{ mode.mode === 'stock' ? '库存代发' : '现做现发' }}</legend>
    <div class="default-grid"><label>每单固定服务费（元）<input v-model="mode.fixed_fee" inputmode="decimal" placeholder="免费请填写 0" /></label><label>每包装件服务费（元）<input v-model="mode.per_package_fee" inputmode="decimal" placeholder="免费请填写 0" /></label></div>
    <label>运费模板<select v-model.number="mode.shipping_template_id"><option :value="0">继续使用客户档案／系统物流默认</option><option v-for="template in templates" :key="template.id" :value="template.id" :disabled="!template.active">{{ template.name }} · V{{ template.revision }}{{ template.active ? '' : '（停用）' }}</option></select></label>
    <label>每票固定附加运费（元）<input v-model="mode.surcharge" inputmode="decimal" /></label>
    <label class="checkbox"><input v-model="mode.shipping_free" type="checkbox" />客户包邮（免基础运费及附加运费）</label>
   </fieldset>
   <p class="default-impact">本次保存将更新上述 {{ inheritingCustomers }} 位客户的继承字段。已存在订单继续使用保存时的费用快照。</p>
   <div class="actions"><button type="submit" :disabled="saving">{{ saving ? '保存中…' : '保存共享默认配置' }}</button><button type="button" class="secondary" :disabled="saving" @click="load">重新载入</button></div>
  </form>
 </section>
</template>
<script setup>
import { onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { fulfillmentSettingsForm, explicitMoneyCents } from '../lib/fulfillment-settings.js'
const emit=defineEmits(['saved'])
const revision=ref(0),inheritingCustomers=ref(0),form=ref(fulfillmentSettingsForm()),templates=ref([]),loaded=ref(false),loading=ref(false),saving=ref(false),error=ref(''),message=ref('')
function assign(data){revision.value=Number(data.revision||0);inheritingCustomers.value=Number(data.inheriting_customers||0);form.value=fulfillmentSettingsForm(data);loaded.value=true}
async function load(){loading.value=true;error.value='';message.value='';try{const [data,list]=await Promise.all([apiGet('/api/settings/logistics/fulfillment-defaults'),apiGet('/api/settings/logistics/shipping-templates')]);assign(data);templates.value=list.templates||[]}catch(e){error.value=e.message||'默认配置加载失败'}finally{loading.value=false}}
async function save(){if(saving.value)return;saving.value=true;error.value='';message.value='';try{
 const modes=form.value.modes.map(m=>({mode:m.mode,fee_configured:true,fixed_fee_cents:explicitMoneyCents(m.fixed_fee),per_package_fee_cents:explicitMoneyCents(m.per_package_fee),shipping_template_id:Number(m.shipping_template_id||0),shipping_free:m.shipping_free===true,shipping_policy:{surcharge_cents:explicitMoneyCents(m.surcharge)}}))
 const data=await apiSend('/api/settings/logistics/fulfillment-defaults',{method:'PUT',body:{expected_revision:revision.value,modes}});assign({...data,inheriting_customers:inheritingCustomers.value});message.value='共享默认配置已保存';emit('saved',data)
}catch(e){error.value=e.message||'默认配置保存失败'}finally{saving.value=false}}
onMounted(load)
</script>
<style scoped>
.defaults-editor{color:#142033}.default-impact{padding:12px;background:#f7f9fb;font-size:12px;color:#718097;line-height:1.8;border-radius:7px}.version{font-size:12px;color:#718097}fieldset{border:1px solid #e1e7ee;border-radius:8px;padding:16px;margin:18px 0}legend{font-size:14px;font-weight:600}label{display:grid;gap:7px;font-size:12px;margin:10px 0;color:#526174}input,select{width:100%;box-sizing:border-box;border:1px solid #e1e7ee;border-radius:6px;background:white;padding:9px;font:inherit}.default-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px}.checkbox{display:flex;align-items:center;gap:8px}.checkbox input{width:16px}.actions{display:flex;gap:10px;margin-top:20px}button{border:1px solid #278456;border-radius:7px;background:#278456;color:#fff;padding:10px 15px;font:inherit;font-size:13px;cursor:pointer}.secondary{background:white;color:#278456}.error{color:#ad3c2a;background:#fff2ed;padding:12px;font-size:13px}.success{color:#278456;background:#e8f6ed;padding:12px}button:disabled{opacity:.5;cursor:default}
</style>
