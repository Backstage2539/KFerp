<script setup lang="ts">
import {ref,watch,onBeforeUnmount} from 'vue'
import {freightLabel} from '../utils/freight'
import {previewEmployeeFreight,type FreightSnapshot} from '../api/customerPortal'
const props=defineProps<{token:string;customerId:number;destination?:string[];items:{product_id:number;bom_spec_id?:number;bom_variant_id?:number;qty:number}[]}>()
const emit=defineEmits<{(e:'update:destination',v:string[]):void}>()
const destination=ref<string[]>(props.destination||[]),quote=ref<FreightSnapshot|null>(null),busy=ref(false),error=ref(''),adjust=ref(false),surcharge=ref('0.00'),free=ref(false),reason=ref('');let sequence=0,timer:ReturnType<typeof setTimeout>|undefined
function override(){if(!adjust.value)return undefined;const n=Number(surcharge.value);if(!String(surcharge.value).trim()||!Number.isFinite(n)||n<0||Math.round(n*100)!==n*100)throw new Error('附加运费需为非负金额，最多两位小数');return {surcharge_cents:Math.round(n*100),free_shipping:free.value,reason:reason.value}}
function toggleAdjustment(event:Event){adjust.value=Boolean((event as unknown as {detail:{value:boolean}}).detail.value)}
function toggleFree(event:Event){free.value=Boolean((event as unknown as {detail:{value:boolean}}).detail.value)}
function selectRegion(event:{detail:{value:string[]}}){destination.value=event.detail.value;emit('update:destination',destination.value)}
async function preview(){if(timer)clearTimeout(timer);const current=++sequence;quote.value=null;error.value='';if(!props.customerId||!props.items.length){error.value='选择客户及商品后核算运费';return null}busy.value=true;try{const q=await previewEmployeeFreight(props.token,{customer_id:props.customerId,province:destination.value[0]||'',city:destination.value[1]||'',district:destination.value[2]||'',items:props.items,freight_override:override()});if(current!==sequence)return null;quote.value=q;return q}catch(e){if(current===sequence)error.value=e instanceof Error?e.message:'运费核算失败';return null}finally{if(current===sequence)busy.value=false}}
function payload(){return {freight_protocol:'deferred_freight_v1',freight_quote_token:quote.value?.token||'',shipping_province:destination.value[0]||'',shipping_city:destination.value[1]||'',shipping_district:destination.value[2]||'',...(adjust.value?{freight_override:override()}:{}),shipping_amount:0}}
watch(()=>[props.customerId,props.items,destination.value,adjust.value,surcharge.value,free.value,reason.value],()=>{sequence++;quote.value=null;busy.value=false;if(timer)clearTimeout(timer);timer=setTimeout(()=>void preview(),450)},{deep:true})
watch(()=>props.destination,v=>{if(v&&v.join('|')!==destination.value.join('|'))destination.value=[...v]})
watch(()=>props.customerId,(next,previous)=>{adjust.value=false;reason.value='';if(previous>0&&next!==previous){destination.value=[];emit('update:destination',[])}else if(props.destination?.length)destination.value=[...props.destination]})
onBeforeUnmount(()=>{sequence++;if(timer)clearTimeout(timer)})
defineExpose({preview,payload})
</script>
<template>
 <view class="estimate-card"><text class="title">预估运费</text><picker mode="region" :value="destination" @change="selectRegion"><view class="field">{{destination.length?destination.join(' / '):'选择收件省市区'}} ›</view></picker><text class="hint">不计入当前应付，实际运费核对出账后另收。</text>
 <view class="switch"><text>本单调整运费政策</text><switch :checked="adjust" @change="toggleAdjustment" /></view><view v-if="adjust"><input v-model="surcharge" type="digit" class="field" placeholder="每票附加运费（元）" /><view class="switch"><text>本单包邮</text><switch :checked="free" @change="toggleFree" /></view><input v-model="reason" class="field" placeholder="填写调整原因" /></view>
 <text v-if="error" class="error">{{error}}</text><text v-else-if="busy" class="hint">正在核算…</text><text v-else-if="quote" class="amount">{{freightLabel(quote)}}</text><text v-if="quote && !quote.estimate_known && quote.cost_pending_reason" class="hint">{{quote.cost_pending_reason}}</text><button :disabled="busy" @tap="preview">核算预估运费</button></view>
</template>
<style scoped>
.estimate-card{padding:24rpx;background:#eff6f0;border:1px solid #d5e5d9;border-radius:14rpx;margin:24rpx 0}.title{font-size:30rpx;font-weight:700;color:#244c36}.field{margin-top:16rpx;padding:20rpx;background:white;border:1px solid #cbd9cf;border-radius:10rpx;font-size:28rpx}.hint,.error,.amount{display:block;font-size:25rpx;line-height:1.7;margin:12rpx 0}.hint{color:#718276}.error{color:#b13b2f}.amount{color:#286248;font-weight:600}.switch{display:flex;align-items:center;justify-content:space-between;font-size:26rpx;margin:10rpx 0}button{font-size:26rpx;color:#286248;background:white;border:1px solid #cbd9cf;margin-top:14rpx}
</style>
