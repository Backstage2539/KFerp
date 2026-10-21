<template>
 <div class="order-freight-estimate">
  <h3>运费</h3>
  <p v-if="locked">运费已出账或已确认实际费用，收费规则已锁定。</p>
  <template v-else>
   <label v-if="!customerPortal">收件地区<input v-model="query" placeholder="搜索省市区，如孟连县" @focus="searchOpen=true" /></label>
   <div v-if="customerPortal" class="fields"><label>省份<SearchableSelect :model-value="destination.province" :options="provinceOptions" option-value="value" :empty-value="''" @update:model-value="update({destination:{province:$event,city:'',district:''}})" /></label><label>城市<SearchableSelect :model-value="destination.city" :options="cityOptions" option-value="value" :empty-value="''" @update:model-value="update({destination:{...destination,city:$event,district:''}})" /></label><label>区县<SearchableSelect :model-value="destination.district" :options="districtOptions" option-value="value" :empty-value="''" @update:model-value="update({destination:{...destination,district:$event}})" /></label></div>
   <p>{{regionLabel || '正在识别客户收件地址，可搜索选择省市区'}}</p>
   <div v-if="searchOpen && query.trim()" class="region-options"><button v-for="r in matches" :key="r.code" type="button" @click="selectRegion(r.code)">{{r.path||r.name}}</button><p v-if="!matches.length">没有符合条件的地区</p></div>
   <div v-if="!customerPortal" class="fields"><label>本单运费（元，留空自动）<input :value="state.manual" :disabled="!!state.free" type="text" inputmode="decimal" placeholder="留空自动" @input="update({manual:$event.target.value})" /></label><label class="check"><input type="checkbox" :checked="!!state.free" @change="update({free:$event.target.checked})" />包邮</label></div>
  </template>
  <p v-if="error" class="error" role="alert">{{error}} <button v-if="!locked" type="button" @click="preview">重试核算</button></p><p v-else-if="busy">正在核算运费…</p><p v-else-if="quote" class="estimate"><strong>{{freightLabel(quote)}}</strong><span v-if="!quote.estimate_known && quote.cost_pending_reason"> · 快递成本待核</span></p><p v-else>{{customerId&&items.length?'准备核算运费…':'添加商品后自动计算'}}</p>
  <p v-if="!state.free && !state.manual">自动预估不计当前应付，实际运费核对出账后另收。</p><p v-else-if="!state.free">手填运费计入本单应付，实际快递账单仅核对成本。</p>
 </div>
</template>
<script setup>
import {computed,onBeforeUnmount,onMounted,ref,watch} from 'vue'
import SearchableSelect from './SearchableSelect.vue'
import {apiGet,apiSend,pageRequest} from '../api/client'
import {freightLabel} from '../lib/freight'
import {emptyFreightState,freightAmount,freightOverride,regionDestination,searchOrderRegions} from '../lib/order-freight'
const props=defineProps({modelValue:{type:Object,default:()=>emptyFreightState()},customerId:{type:[Number,String],default:0},items:{type:Array,default:()=>[]},customerPortal:{type:Boolean,default:false},address:{type:String,default:''},orderId:{type:Number,default:0},expectedToken:{type:String,default:''},locked:{type:Boolean,default:false},storedQuote:{type:Object,default:null},savedDestination:{type:Object,default:()=>({})}})
const emit=defineEmits(['update:modelValue','destination-change','amount-change'])
const state=ref({...emptyFreightState(),...props.modelValue}),destination=computed(()=>state.value.destination),quote=ref(props.storedQuote),error=ref(''),busy=ref(false),regions=ref([]),query=ref(''),searchOpen=ref(false)
let addressTask=Promise.resolve()
let seq=0,addressSeq=0,timer,alive=true
const matches=computed(()=>searchOrderRegions(regions.value,query.value).slice(0,80)),regionLabel=computed(()=>[destination.value.province,destination.value.city,destination.value.district].filter(Boolean).join(' / '))
const options=rows=>rows.map(r=>({value:r.name,label:r.name,py:r.pinyin,pyi:r.initials})),provinceOptions=computed(()=>options(regions.value.filter(r=>r.level===1))),province=computed(()=>regions.value.find(r=>r.level===1&&r.name===destination.value.province)),city=computed(()=>regions.value.find(r=>r.level===2&&r.name===destination.value.city&&r.code.startsWith(province.value?.code.slice(0,2)||'~'))),cityOptions=computed(()=>options(regions.value.filter(r=>r.level===2&&r.code.startsWith(province.value?.code.slice(0,2)||'~')))),districtOptions=computed(()=>options(regions.value.filter(r=>r.level===3&&r.code.startsWith(city.value?.code.slice(0,4)||'~'))))
function update(patch){state.value={...state.value,...patch};emit('update:modelValue',state.value);emit('destination-change',{shipping_province:destination.value.province,shipping_city:destination.value.city,shipping_district:destination.value.district});try{emit('amount-change',freightAmount(state.value))}catch{}}
function selectRegion(code){addressSeq++;update({destination:regionDestination(regions.value,code),source:'manual',address:props.address});query.value='';searchOpen.value=false;error.value=''}
async function resolveAddress(){const current=++addressSeq;if(props.customerPortal||props.locked||!props.address||state.value.address===props.address&&destination.value.province)return;try{const r=props.customerPortal?null:await apiSend('/api/order/shipping-destination/resolve',{body:{address:props.address}});if(!alive||current!==addressSeq)return;if(r?.status==='resolved'){update({destination:r.destination,address:props.address,source:'auto'})}else{update({destination:emptyFreightState().destination,address:props.address,source:'auto'});error.value='收件地区无法唯一识别，请搜索选择'}}catch(e){if(alive&&current===addressSeq)error.value=e.message||'地址识别失败'}}
async function preview(){await addressTask;if(timer)clearTimeout(timer);if(props.locked)return props.storedQuote;const current=++seq;quote.value=null;error.value='';if(!props.customerId||!props.items.length){busy.value=false;return null}busy.value=true;try{const body={...destination.value,items:props.items,...(!props.customerPortal?{freight_protocol:'order_freight_v2',customer_id:Number(props.customerId),order_id:props.orderId,expected_freight_token:props.expectedToken,freight_override:freightOverride(state.value)}:{})};const q=props.customerPortal?await pageRequest('/api/customer-processing/portal/freight/preview',{method:'POST',body}):await apiSend('/api/order/freight/preview',{body});if(!alive||current!==seq)return null;if(state.value.free==null && q.policy?.free_shipping)update({free:true});quote.value=q;return q}catch(e){if(alive&&current===seq)error.value=e.message||'运费核算失败';return null}finally{if(alive&&current===seq)busy.value=false}}
function payload(){return {freight_protocol:props.customerPortal?'deferred_freight_v1':'order_freight_v2',freight_quote_token:quote.value?.token||'',expected_freight_token:props.expectedToken,shipping_province:destination.value.province,shipping_city:destination.value.city,shipping_district:destination.value.district,...(!props.customerPortal?{freight_override:freightOverride(state.value)}:{}),shipping_amount:String(props.customerPortal?0:freightAmount(state.value))}}
function invalidate(){if(props.locked){quote.value=props.storedQuote;return;}seq++;quote.value=null;busy.value=false;if(timer)clearTimeout(timer);if(!props.locked)timer=setTimeout(()=>void preview(),400)}
watch(()=>props.modelValue,v=>{if(JSON.stringify(v)!==JSON.stringify(state.value))state.value={...emptyFreightState(),...v}},{deep:true})
watch(()=>[props.customerId,props.items,state.value,props.expectedToken],invalidate,{deep:true})
watch(()=>props.customerId,(next,previous)=>{if(previous>0&&Number(next)!==Number(previous)){addressSeq++;update(emptyFreightState());addressTask=resolveAddress()}})
watch(()=>props.savedDestination,v=>{if(props.customerPortal&&v?.province&&[v.province,v.city,v.district].join('|')!==[destination.value.province,destination.value.city,destination.value.district].join('|'))update({destination:{...emptyFreightState().destination,...v}})},{immediate:true,deep:true})
watch(()=>props.address,()=>addressTask=resolveAddress(),{immediate:true})
onMounted(async()=>{try{const r=props.customerPortal?await pageRequest('/api/customer-processing/portal/shipping-regions'):await apiGet('/api/order/freight/regions');if(alive)regions.value=r.regions||[]}catch(e){if(alive)error.value=e.message}if(props.customerId&&props.items.length)invalidate()})
onBeforeUnmount(()=>{alive=false;seq++;addressSeq++;if(timer)clearTimeout(timer)})
defineExpose({preview,payload})
</script>
<style scoped>
.order-freight-estimate{padding:16px;background:#f3f8f4;border:1px solid #d5e5da;border-radius:8px;margin:8px 0 16px}h3{font-size:15px;margin:0}p{font-size:12px;color:#66796e;line-height:1.7}.fields{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:10px}label{display:grid;gap:6px;font-size:12px}.check{display:flex;align-items:center}.check input{width:16px}input{width:100%;box-sizing:border-box;padding:8px;border:1px solid #cadbce;border-radius:6px}button{padding:8px 10px;background:white;color:#286248;border:1px solid #bcd3c5;border-radius:6px;cursor:pointer}.estimate{color:#286248}.error{color:#a83228}.region-options{display:grid;gap:6px;max-height:240px;overflow:auto}.region-options button{text-align:left}
</style>
