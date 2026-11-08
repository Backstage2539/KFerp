<script setup>
import { reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
const emit=defineEmits(['changed'])
const expanded=ref(false),busy=ref(false),error=ref(''),notice=ref(''),revision=ref(0),options=ref({})
const form=reactive({disabled:{},approvals:{}})
const groups={scrap_cost:'报损成本处理',action:'订单操作',production:'生产处置',direct_ship:'关联代发',customer_stock:'客户货权库存',stock:'公司库存',stock_reservation:'库存预留',fee:'费用',funds:'资金去向',return_stock:'退回入库',return_finance:'退回商品款',return_quality:'退回验收',return_ownership:'退回货权'}
const roles={'orders.resolve':'销售负责人','production.resolve':'生产负责人','stock.resolve':'仓库负责人','finance.resolve':'财务负责人'}
async function load(){
 busy.value=true;error.value='';notice.value=''
 try{const data=await apiGet('/api/settings/order-resolution');revision.value=data.revision;options.value=data.options;form.disabled=Object.fromEntries(Object.keys(groups).map(k=>[k,[...(data.policy?.disabled?.[k]||[])]]));form.approvals={...data.policy?.approvals}}
 catch(e){error.value=e.message}finally{busy.value=false}
}
async function show(){expanded.value=!expanded.value;if(expanded.value)await load()}
async function save(){
 if(!revision.value||busy.value)return
 busy.value=true;error.value='';notice.value=''
 try{const out=await apiSend('/api/settings/order-resolution',{body:{revision:revision.value,policy:{disabled:form.disabled,approvals:Object.fromEntries(Object.entries(form.approvals).filter(([,v])=>v))}}});revision.value=out.revision;notice.value='配置已保存，现有未执行处置会重新校验';emit('changed')}
 catch(e){error.value=e.message}finally{busy.value=false}
}
</script>
<template>
 <section class="resolution-policy">
  <button type="button" @click="show">{{ expanded?'收起配置':'订单处理配置（管理员）' }}</button>
  <fieldset v-if="expanded" :disabled="busy">
   <legend>允许的处置方式与审批</legend>
   <p>勾选需要停用的选项。配置不能放宽数量、货权和已发生业务的校验；业务处理页不预选处置。</p>
   <details v-for="(label,kind) in groups" :key="kind"><summary>{{ label }}：停用选项</summary><label v-for="v in options[kind]||[]" :key="v.value"><input v-model="form.disabled[kind]" type="checkbox" :value="v.value" />{{ v.label }}</label></details>
   <h4>额外审批要求</h4><p>各项业务仍须由有权限的岗位确认；可为整笔操作再指定审批岗位。</p>
   <label v-for="v in options.action||[]" :key="v.value">{{ v.label }}<select v-model="form.approvals[v.value]"><option value="">不增加额外审批</option><option v-for="(label,key) in roles" :key="key" :value="key">{{ label }}</option></select></label>
   <p v-if="error" role="alert">{{ error }}</p><p v-if="notice" role="status">{{ notice }}</p>
   <div><button type="button" :disabled="busy" @click="load">重新加载配置</button><button type="button" :disabled="busy||!revision" @click="save">保存配置</button></div>
  </fieldset>
 </section>
</template>
<style scoped>
.resolution-policy{padding:12px 0}fieldset{border:1px solid #cad8cf;border-radius:8px;margin-top:10px;padding:14px;min-width:0}details{padding:8px 0}label{display:flex;align-items:center;gap:8px;margin:8px 0}select{margin-left:auto;padding:8px;max-width:65%}p{font-size:13px;color:#5c7163}button{padding:8px 12px;border:1px solid #b8cbbf;background:white;border-radius:6px;cursor:pointer}button+button{margin-left:8px}[role=alert]{color:#a52c23}
</style>
