<template>
 <div class="order-freight-estimate">
  <h3>预估运费</h3><p>不计入当前应付，实际运费核对出账后另行收取。</p>
  <div class="fields"><label>收件省份<SearchableSelect v-model="destination.province" :options="provinceOptions" option-value="value" :empty-value="''" placeholder="选择省份" @update:model-value="destination.city='';destination.district=''" /></label><label>收件城市<SearchableSelect v-model="destination.city" :options="cityOptions" option-value="value" :empty-value="''" placeholder="选择城市" @update:model-value="destination.district=''" /></label><label>区县<SearchableSelect v-model="destination.district" :options="districtOptions" option-value="value" :empty-value="''" placeholder="选择区县" /></label></div>
  <details v-if="!customerPortal"><summary>按单调整运费政策</summary><label class="check"><input v-model="overrideEnabled" type="checkbox" />本单调整</label><div v-if="overrideEnabled" class="fields"><label>本单每票附加运费（元）<input v-model="surcharge" type="number" min="0" step="0.01" /></label><label class="check"><input v-model="freeShipping" type="checkbox" />本单包邮</label><label>调整原因<input v-model.trim="reason" required /></label></div></details>
  <p v-if="error" class="error" role="alert">{{error}} <button type="button" @click="preview">重试核算</button></p><p v-else-if="busy">正在核算预估运费…</p><p v-else-if="quote" class="estimate"><strong>{{freightLabel(quote)}}</strong><span v-if="quote.estimate_known"> · {{quote.estimate.chargeable_weight_g}}克；基础 ¥{{money(quote.estimate.base_cents)}}＋附加 ¥{{money(quote.surcharge_cents)}}</span><span v-if="quote.status==='free'&&!quote.estimate_known"> · {{quote.cost_pending_reason}}</span></p><button type="button" class="secondary" :disabled="busy||!customerId" @click="preview">核算预估运费</button>
 </div>
</template>
<script setup>
import {computed,onBeforeUnmount,onMounted,reactive,ref,watch} from 'vue'
import {apiGet,apiSend,pageRequest} from '../api/client'
import {explicitMoneyCents} from '../lib/fulfillment-settings'
import {freightLabel} from '../lib/freight'
import SearchableSelect from './SearchableSelect.vue'
const props=defineProps({customerId:{type:[Number,String],default:0},items:{type:Array,default:()=>[]},customerPortal:{type:Boolean,default:false},address:{type:String,default:''},savedDestination:{type:Object,default:()=>({})}})
const emit=defineEmits(['destination-change'])
const quote=ref(null),error=ref(''),busy=ref(false),regions=ref([]),destination=reactive({province:'',city:'',district:''}),overrideEnabled=ref(false),surcharge=ref('0.00'),freeShipping=ref(false),reason=ref('');let seq=0,timer
const money=c=>(Number(c||0)/100).toFixed(2),options=rs=>rs.map(r=>({value:r.name,label:r.name,py:r.pinyin,pyi:r.initials,code:r.code}))
const provinceOptions=computed(()=>options(regions.value.filter(r=>r.level===1))),province=computed(()=>regions.value.find(r=>r.level===1&&r.name===destination.province)),city=computed(()=>regions.value.find(r=>r.level===2&&r.name===destination.city&&r.code.startsWith(province.value?.code.slice(0,2)||'~'))),cityOptions=computed(()=>options(regions.value.filter(r=>r.level===2&&r.code.startsWith(province.value?.code.slice(0,2)||'~')))),districtOptions=computed(()=>options(regions.value.filter(r=>r.level===3&&r.code.startsWith(city.value?.code.slice(0,4)||'~'))))
function override(){return overrideEnabled.value?{surcharge_cents:explicitMoneyCents(surcharge.value),free_shipping:freeShipping.value,reason:reason.value}:undefined}
async function preview(){if(timer)clearTimeout(timer);const version=++seq;quote.value=null;error.value='';if(!props.customerId||!props.items.length){error.value='选择客户和商品后核算运费';return null}busy.value=true;try{const body={...destination,items:props.items,freight_override:override()};const path=props.customerPortal?'/api/customer-processing/portal/freight/preview':`/api/customer-fulfillment/${props.customerId}/freight/preview`;const q=props.customerPortal?await pageRequest(path,{method:'POST',body}):await apiSend(path,{body});if(version!==seq)return null;quote.value=q;return q}catch(e){if(version===seq)error.value=e.message||'运费核算失败';return null}finally{if(version===seq)busy.value=false}}
function payload(){return {freight_protocol:'deferred_freight_v1',freight_quote_token:quote.value?.token||'',shipping_province:destination.province,shipping_city:destination.city,shipping_district:destination.district,...(overrideEnabled.value?{freight_override:override()}:{}),shipping_amount:'0',payment_shipping_amount:'0'}}
function invalidate(){emit('destination-change',{shipping_province:destination.province,shipping_city:destination.city,shipping_district:destination.district});seq++;busy.value=false;quote.value=null;if(timer)clearTimeout(timer);timer=setTimeout(()=>{void preview()},400)}
watch(()=>[props.customerId,props.items,destination.province,destination.city,destination.district,overrideEnabled.value,surcharge.value,freeShipping.value,reason.value],invalidate,{deep:true})
watch(()=>props.savedDestination,(value)=>Object.assign(destination,{province:value.province||'',city:value.city||'',district:value.district||''}),{immediate:true,deep:true})
watch(()=>props.customerId,()=>{overrideEnabled.value=false;reason.value=''})
onMounted(async()=>{try{const path=props.customerPortal?'/api/customer-processing/portal/shipping-regions':'/api/settings/logistics/regions';const r=props.customerPortal?await pageRequest(path):await apiGet(path);regions.value=r.regions||[]}catch(e){error.value=e.message}})
onBeforeUnmount(()=>{seq++;if(timer)clearTimeout(timer)})
defineExpose({preview,payload})
</script>
<style scoped>
.order-freight-estimate{padding:16px;background:#f3f8f4;border:1px solid #d5e5da;border-radius:8px;margin:8px 0 16px}h3{font-size:15px;margin:0}p{font-size:12px;color:#66796e;line-height:1.7}.fields{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:10px}label{display:grid;gap:6px;font-size:12px}.check{display:flex;align-items:center}.check input{width:16px}input{width:100%;box-sizing:border-box;padding:8px;border:1px solid #cadbce;border-radius:6px}summary{cursor:pointer;font-size:12px;margin:12px 0}button{padding:6px 10px;background:white;color:#286248;border:1px solid #bcd3c5;border-radius:6px;cursor:pointer}.estimate{color:#286248}.error{color:#a83228}
</style>
