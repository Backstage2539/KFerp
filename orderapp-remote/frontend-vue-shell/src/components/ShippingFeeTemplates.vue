<template>
 <section class="shipping-fee-panel">
  <div class="heading"><div><h2>运费模板</h2><p>整单一票计重，包装附加重只加一次。新报价使用保存后的版本，历史订单保留原规则。</p></div><button @click="newTemplate">新增模板</button></div>
  <p v-if="error" class="error" role="alert">{{error}}</p><p v-if="message" class="ok" role="status">{{message}}</p>
  <div class="layout">
   <aside><button v-for="t in templates" :key="t.id" class="template" :class="{selected:form.id===t.id}" @click="edit(t)"><strong>{{t.name}}</strong><span>V{{t.revision}} · {{t.active?'启用':'停用'}} · {{t.groups?.length?'重量分组':'旧首续重'}}</span></button></aside>
   <form @submit.prevent="save">
    <div class="fields"><label>模板名称<input v-model.trim="form.name" required /></label><label>每票包装附加重（克）<input v-model.number="form.packaging_weight_g" type="number" min="0" step="1" required /></label><label class="check"><input v-model="form.active" type="checkbox" />启用</label></div>
    <div class="fields"><label>物流公司<select v-model.number="form.carrier_id" @change="form.logistics_product_id=0"><option :value="0">请选择</option><option v-for="c in companies.filter(c=>c.active!==false)" :key="c.id" :value="c.id">{{c.name}}</option></select></label><label>物流产品<select v-model.number="form.logistics_product_id"><option :value="0">请选择</option><option v-for="p in products" :key="p.id" :value="p.id">{{p.name}}</option></select></label></div>
    <template v-if="!legacy">
     <p class="hint">地区目录：{{regionVersion}}（固定版本）。更具体的市／区县价优先；同层地区不能属于多个分组。</p>
     <div v-for="(g,i) in form.groups" :key="i" class="group">
      <div class="heading"><label>地区分组<input v-model.trim="g.name" placeholder="如 一区" required /></label><div class="actions"><button type="button" class="secondary" @click="copyGroup(i)">复制分组</button><button type="button" class="remove" @click="form.groups.splice(i,1)">删除</button></div></div>
      <FreightRegionSelect v-model="g.region_codes" :regions="regions" />
      <div class="weight-heading"><h3>重量价格</h3><select :value="g.tiers.length?'tiers':'legacy'" @change="switchMode(g,$event.target.value)"><option value="tiers">重量档位＋末档续重</option><option value="legacy">首重／续重</option></select></div>
      <div v-if="g.tiers.length" class="tiers"><label v-for="(tier,j) in g.tiers" :key="j">{{j===0?'不超过':'至'}}<div><input v-model.number="tier.upper_weight_g" aria-label="档位上限克数" type="number" min="1" step="1" />克</div><div><input v-model="tier.fee" aria-label="档位价格元" type="number" min="0" step="0.01" />元<button type="button" class="remove" @click="g.tiers.splice(j,1)">×</button></div></label><button type="button" class="secondary" @click="g.tiers.push({upper_weight_g:(g.tiers.at(-1)?.upper_weight_g||0)+1000,fee:''})">添加档位</button></div>
      <div v-else class="fields"><label>首重（克）<input v-model.number="g.first_weight_g" type="number" min="1" /></label><label>首费（元）<input v-model="g.first_fee" type="number" min="0" step="0.01" /></label></div>
      <div class="fields"><label>{{g.tiers.length?'超过末档后，每续重':'每续重'}}（克）<input v-model.number="g.additional_weight_g" type="number" min="1" step="1" /></label><label>续重费用（元）<input v-model="g.additional_fee" type="number" min="0" step="0.01" /></label></div><p class="hint">不足一个续重单位向上取整。</p>
     </div>
     <button type="button" class="secondary" @click="form.groups.push(emptyFreightGroup())">添加地区分组</button>
     <p class="coverage">尚未整省覆盖：{{uncovered.map(r=>r.name).join('、')||'无'}}。仅配置市区例外的省份仍可能有未覆盖地区，未覆盖不自动套价。</p>
    </template>
    <template v-else><p class="hint">旧首重／续重规则，保留原有文本匹配。新地区分组请新增或复制模板后配置。</p><div v-for="(r,i) in form.rules" :key="i" class="group"><h3>{{r.province}} {{r.city}} {{r.district}}</h3><div class="fields"><label>首重（克）<input v-model.number="r.first_weight_g" type="number" min="1" /></label><label>首费（元）<input v-model="r.first_fee" type="number" min="0" step="0.01" /></label><label>续重（克）<input v-model.number="r.additional_weight_g" type="number" min="1" /></label><label>续费（元）<input v-model="r.additional_fee" type="number" min="0" step="0.01" /></label></div></div></template>
    <div v-if="versions.length" class="group"><h3>历史版本（只读）</h3><p v-for="v in versions" :key="v.revision">V{{v.revision}} · {{v.name}} · {{v.groups?.length||v.rules?.length}}组 <button type="button" class="secondary" @click="edit(v);copyTemplate()">复制此版本为新模板</button></p></div>
    <div class="actions"><button type="submit" :disabled="saving||loading">{{saving?'保存中…':'保存模板'}}</button><button v-if="form.id" type="button" class="secondary" @click="copyTemplate">复制模板</button><span v-if="form.id">当前 V{{form.revision}}</span><button v-if="form.id" type="button" class="secondary" @click="loadVersions">历史版本</button></div>
   </form>
  </div>
 </section>
</template>
<script setup>
import {computed,onMounted,ref} from 'vue'
import {apiGet,apiSend} from '../api/client'
import {emptyFreightGroup,freightTemplateForm,freightTemplatePayload,remainingCoverage} from '../lib/freight.js'
import {shippingTemplateForm,shippingTemplatePayload} from '../lib/fulfillment-settings.js'
import FreightRegionSelect from './FreightRegionSelect.vue'
const props=defineProps({companies:{type:Array,default:()=>[]}})
const versions=ref([])
const templates=ref([]),regions=ref([]),regionVersion=ref(''),form=ref(freightTemplateForm()),legacy=ref(false),error=ref(''),message=ref(''),saving=ref(false),loading=ref(false)
const products=computed(()=>props.companies.find(c=>Number(c.id)===Number(form.value.carrier_id))?.products?.filter(p=>p.active!==false)||[]),uncovered=computed(()=>remainingCoverage(regions.value,form.value.groups||[]))
function edit(t){versions.value=[];legacy.value=!!t.rules?.length&&!t.groups?.length;form.value=legacy.value?{...shippingTemplateForm(t),carrier_id:t.carrier_id||0,logistics_product_id:t.logistics_product_id||0}:freightTemplateForm(t);error.value='';message.value=''}
function newTemplate(){edit({})}
function copyTemplate(){const copy=JSON.parse(JSON.stringify(form.value));copy.id=0;copy.revision=0;copy.name+=' 副本';form.value=copy}
function copyGroup(i){const g=JSON.parse(JSON.stringify(form.value.groups[i]));g.name+=' 副本';g.region_codes=[];form.value.groups.push(g)}
function switchMode(g,mode){g.tiers=mode==='tiers'?emptyFreightGroup().tiers:[]}
async function load(){loading.value=true;error.value='';try{const [t,r]=await Promise.all([apiGet('/api/settings/logistics/shipping-templates'),apiGet('/api/settings/logistics/regions')]);templates.value=t.templates||[];regions.value=r.regions||[];regionVersion.value=r.version}catch(e){error.value=e.message||'加载失败'}finally{loading.value=false}}
async function loadVersions(){try{const data=await apiGet(`/api/settings/logistics/shipping-templates/${form.value.id}/versions`);versions.value=data.versions||[]}catch(e){error.value=e.message}}
async function save(){saving.value=true;error.value='';message.value='';try{const p=legacy.value?{...shippingTemplatePayload(form.value),carrier_id:form.value.carrier_id,logistics_product_id:form.value.logistics_product_id}:freightTemplatePayload(form.value);const saved=await apiSend(`/api/settings/logistics/shipping-templates${p.id?`/${p.id}`:''}`,{method:p.id?'PUT':'POST',body:p});await load();edit(saved);message.value='模板已保存，新报价使用此版本'}catch(e){error.value=e.message||'保存失败'}finally{saving.value=false}}
onMounted(load)
</script>
<style scoped>
.shipping-fee-panel{margin-top:18px;padding:20px;border:1px solid #dce5df;border-radius:8px;background:white;color:#243d30}.heading,.actions,.weight-heading{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}.heading{margin-bottom:16px}.heading p,.hint,.coverage{font-size:12px;color:#687c6e;line-height:1.7}.layout{display:grid;grid-template-columns:210px minmax(0,1fr);gap:24px}.layout aside{display:flex;flex-direction:column;gap:8px;border-right:1px solid #e4ebe6;padding-right:12px}.template{display:grid;gap:6px;text-align:left;background:white;color:#28573b;white-space:normal}.template span{font-size:12px;color:#74827a}.template.selected{background:#edf6f0}.fields{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px;margin-bottom:14px}label{display:grid;gap:6px;font-size:13px}input,select{box-sizing:border-box;width:100%;min-width:0;padding:9px;border:1px solid #cbd9cf;border-radius:6px}.check{display:flex;align-items:center}.check input{width:18px}.group{padding:16px;border:1px solid #dce5df;border-radius:8px;margin:16px 0}.tiers{display:flex;gap:12px;align-items:flex-end;flex-wrap:wrap;margin:12px 0}.tiers label{width:140px}.tiers label div{display:flex;align-items:center;gap:5px}.weight-heading{margin:16px 0 8px}.weight-heading select{width:auto}h2,h3{margin:0}h2{font-size:20px}h3{font-size:15px}button{padding:8px 12px;border:1px solid #286248;border-radius:6px;background:#286248;color:white;cursor:pointer}button:disabled{opacity:.5}.secondary{background:white;color:#286248}.remove{background:white;color:#a93830;border-color:#e1c4bd}.actions{justify-content:flex-start;margin-top:16px}.error,.ok{padding:10px;font-size:13px}.error{background:#fff1ee;color:#ab2b20}.ok{background:#edf6f0;color:#286248}@media(max-width:900px){.layout{grid-template-columns:1fr}.layout aside{border:0;display:grid;grid-template-columns:1fr 1fr}}
</style>
