<template>
  <section class="publication-product-picker" aria-label="从已发布价格表选品">
    <header><strong>从已发布价格表选品</strong><button type="button" class="secondary" :disabled="busy" @click="$emit('close')">关闭</button></header>
    <p>按来源表的商品建立新的客户价格表草稿，自动补齐客户商品；不会复制来源价格或覆盖现有价格表。</p>
    <p v-if="error" class="picker-error" role="alert">{{ error }}</p>
    <label>来源公共价格表<select v-model.number="sourceID" :disabled="busy" @change="resetPreview"><option :value="0">请选择已发布价格表</option><option v-for="row in validSources" :key="row.id" :value="Number(row.id)">{{ row.table_name || row.product_type_name || '价格表' }} · {{ row.version }}</option></select></label>
    <label>新价格表名称<input v-model.trim="tableName" :disabled="busy" placeholder="例如 孟豆豆代发固定价" /></label>
    <div><button type="button" class="secondary" :disabled="busy || !source" @click="preview">{{ busy ? '正在处理…' : '查看来源商品' }}</button></div>
    <div v-if="previewSource" class="picker-products"><div class="picker-summary"><label><input type="checkbox" :checked="allSelected" :indeterminate.prop="someSelected" :disabled="busy" @change="selectAll($event.target.checked)" /> 全选商品</label><span>已选 {{ selectedIDs.length }} / {{ products.length }} 款</span></div><div class="picker-list"><label v-for="product in products" :key="product.id"><input type="checkbox" :checked="selectedIDs.includes(product.id)" :disabled="busy" @change="toggleProduct(product.id,$event.target.checked)" /> {{ product.name }}</label></div></div>
    <footer><button type="button" class="secondary" :disabled="busy" @click="$emit('close')">取消</button><button type="button" class="primary" :disabled="busy || !previewSource || !selectedIDs.length || !tableName || !canWrite" @click="create">补齐客户商品并创建草稿</button></footer>
    <small v-if="!canWrite">当前账号没有客户商品编辑权限，只能查看来源商品。</small>
  </section>
</template>
<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client.js'
import { actorHasFullViewAccess } from '../lib/menu-permissions.js'
import { publicationProductPickerError, publicationProducts, publicPublicationURL } from '../lib/publication-product-picker.js'
const props = defineProps({ customerId:{type:Number,required:true}, sources:{type:Array,default:()=>[]}, existingNames:{type:Array,default:()=>[]}, createDraft:{type:Function,required:true} })
const emit = defineEmits(['close','created'])
const sourceID=ref(0),tableName=ref(''),previewSource=ref(null),products=ref([]),selectedIDs=ref([]),busy=ref(false),error=ref(''),canWrite=ref(false)
const validSources=computed(()=>props.sources.filter(row=>row.status==='published' && row.owner_type==='official' && (row.publication_purpose||'factory_supply')==='factory_supply'))
const source=computed(()=>validSources.value.find(row=>Number(row.id)===Number(sourceID.value)))
const allSelected=computed(()=>products.value.length>0 && products.value.every(row=>selectedIDs.value.includes(row.id)))
const someSelected=computed(()=>!allSelected.value && selectedIDs.value.length>0)
function resetPreview(){previewSource.value=null;products.value=[];selectedIDs.value=[];error.value=''}
function toggleProduct(id,checked){if(checked && !selectedIDs.value.includes(id))selectedIDs.value.push(id);else if(!checked)selectedIDs.value=selectedIDs.value.filter(item=>item!==id)}
function selectAll(checked){selectedIDs.value=checked?products.value.map(row=>row.id):[]}
async function readSource(){if(!source.value)throw new Error('请选择已发布的公共价格表');const row=await apiGet(publicPublicationURL(source.value));const issue=publicationProductPickerError(row);if(issue)throw new Error(issue);return row}
async function preview(){busy.value=true;error.value='';try{const row=await readSource();previewSource.value=row;products.value=publicationProducts(row);selectedIDs.value=products.value.map(item=>item.id)}catch(err){resetPreview();error.value=err.message||'读取来源价格表失败'}finally{busy.value=false}}
async function create(){if(!previewSource.value||!selectedIDs.value.length||!canWrite.value)return;const name=tableName.value.trim();if(!name){error.value='请填写新价格表名称';return}if(props.existingNames.some(item=>String(item).trim()===name)){error.value='已有同名价格表，请使用其他名称';return}if(!(props.customerId>0)){error.value='请选择目标客户';return}busy.value=true;error.value='';let copied=false;try{const row=await readSource();const currentIDs=new Set(publicationProducts(row).map(item=>item.id));if(selectedIDs.value.some(id=>!currentIDs.has(id))||Number(row.id)!==Number(previewSource.value.id))throw new Error('来源价格表已变化，请重新查看来源商品');const ids=[...selectedIDs.value];const result=await apiSend('/api/product-settings/customer-catalog/copy',{body:{customer_id:props.customerId,mode:'selected',product_ids:ids}});copied=true;await props.createDraft({source:row,productIds:ids,tableName:name,customerCopy:result});emit('created',{source:row,productIds:ids,tableName:name,customerCopy:result})}catch(err){error.value=(copied?'客户商品已补齐，但草稿未能保存：':'')+(err.message||'创建草稿失败')}finally{busy.value=false}}
onMounted(async()=>{try{const actor=await apiGet('/api/auth/me');canWrite.value=Boolean(actorHasFullViewAccess(actor)||actor.permissions?.includes('*')||actor.permissions?.includes('products.write'))}catch{canWrite.value=false}})
</script>
<style scoped>
.publication-product-picker { display:grid; gap:12px; padding:16px; border:1px solid #d9e2e8; border-radius:8px; background:#fff; } header,.picker-summary,footer { display:flex; justify-content:space-between; align-items:center; gap:12px; } p { margin:0; color:#596b7c; } label:not(.picker-list label) { display:grid; gap:5px; } select,input:not([type=checkbox]) { padding:8px; border:1px solid #cbd5df; border-radius:4px; } .picker-error { color:#b42318; } .picker-list { max-height:240px; overflow:auto; display:grid; gap:8px; padding:10px; border:1px solid #e2e8ef; } .picker-summary { margin-bottom:8px; } .picker-summary label { display:flex; align-items:center; } footer { justify-content:flex-end; } small { color:#596b7c; }
</style>
