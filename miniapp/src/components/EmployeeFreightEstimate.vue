<script setup lang="ts">
import {ref,computed,watch,onBeforeUnmount,onMounted} from 'vue'
import {freightLabel} from '../utils/freight'
import {previewEmployeeFreight,resolveEmployeeShippingDestination,fetchEmployeeFreightRegions,type FreightSnapshot} from '../api/customerPortal'
import {emptyFreightState,freightAmount,freightOverride,regionDestination,searchOrderRegions,type FreightState,type FreightRegion} from '../utils/orderFreight'
const props=defineProps<{token:string;customerId:number;modelValue?:FreightState;address?:string;orderId?:number;expectedToken?:string;locked?:boolean;storedQuote?:FreightSnapshot;items:{product_id:number;bom_spec_id?:number;bom_variant_id?:number;qty:number}[]}>()
const emit=defineEmits<{(e:'update:modelValue',v:FreightState):void;(e:'amount-change',v:number):void}>()
const state=ref<FreightState>(props.modelValue||emptyFreightState()),quote=ref<FreightSnapshot|undefined>(props.storedQuote),busy=ref(false),error=ref(''),query=ref(''),searchOpen=ref(false),regions=ref<FreightRegion[]>([])
let addressTask:Promise<void>=Promise.resolve()
let sequence=0,addressSequence=0,alive=true,timer:ReturnType<typeof setTimeout>|undefined
const regionLabel=computed(()=>[state.value.destination.province,state.value.destination.city,state.value.destination.district].filter(Boolean).join(' / ')),matches=computed(()=>searchOrderRegions(regions.value,query.value).slice(0,80))
function update(patch:Partial<FreightState>){state.value={...state.value,...patch};emit('update:modelValue',state.value);try{emit('amount-change',freightAmount(state.value))}catch{}}
function inputAmount(event:Event){const e=event as unknown as {detail:{value:string}};update({manual:e.detail.value})}
function toggleFree(event:Event){const e=event as unknown as {detail:{value:boolean}};update({free:e.detail.value})}
function selectRegion(code:string){addressSequence++;update({destination:regionDestination(regions.value,code),source:'manual',address:props.address||''});query.value='';searchOpen.value=false;error.value=''}
async function resolveAddress(){const current=++addressSequence;if(props.locked||!props.address||state.value.address===props.address&&state.value.destination.province)return;try{const r=await resolveEmployeeShippingDestination(props.token,props.address);if(!alive||current!==addressSequence)return;if(r.status==='resolved'){update({destination:r.destination,address:props.address,source:'auto'})}else{update({destination:emptyFreightState().destination,address:props.address,source:'auto'});error.value='收件地区无法唯一识别，请搜索选择'}}catch(e){if(alive&&current===addressSequence)error.value=e instanceof Error?e.message:'地址识别失败'}}
async function preview(){await addressTask;if(timer)clearTimeout(timer);if(props.locked)return props.storedQuote||null;const current=++sequence;quote.value=undefined;error.value='';if(!props.customerId||!props.items.length){busy.value=false;return null}busy.value=true;try{const q=await previewEmployeeFreight(props.token,{customer_id:props.customerId,freight_protocol:'order_freight_v2',order_id:props.orderId||0,expected_freight_token:props.expectedToken||'',...state.value.destination,items:props.items,freight_override:freightOverride(state.value)});if(!alive||current!==sequence)return null;if(state.value.free==null && q.policy?.free_shipping)update({free:true});quote.value=q;return q}catch(e){if(alive&&current===sequence)error.value=e instanceof Error?e.message:'运费核算失败';return null}finally{if(alive&&current===sequence)busy.value=false}}
function payload(){return {freight_protocol:'order_freight_v2',freight_quote_token:quote.value?.token||'',expected_freight_token:props.expectedToken||'',shipping_province:state.value.destination.province,shipping_city:state.value.destination.city,shipping_district:state.value.destination.district,freight_override:freightOverride(state.value),shipping_amount:freightAmount(state.value)}}
function invalidate(){if(props.locked){quote.value=props.storedQuote;return;}sequence++;quote.value=undefined;busy.value=false;if(timer)clearTimeout(timer);if(!props.locked)timer=setTimeout(()=>void preview(),450)}
watch(()=>props.modelValue,v=>{if(v&&JSON.stringify(v)!==JSON.stringify(state.value))state.value=v},{deep:true})
watch(()=>[props.customerId,props.items,state.value,props.expectedToken],invalidate,{deep:true})
watch(()=>props.customerId,(next,previous)=>{if(previous>0&&next!==previous){addressSequence++;update(emptyFreightState());addressTask=resolveAddress()}})
watch(()=>props.address,()=>addressTask=resolveAddress(),{immediate:true})
onMounted(async()=>{try{const r=await fetchEmployeeFreightRegions(props.token);if(alive)regions.value=r.regions||[]}catch(e){if(alive)error.value=e instanceof Error?e.message:'地区加载失败'}if(props.customerId&&props.items.length)invalidate()})
onBeforeUnmount(()=>{alive=false;sequence++;addressSequence++;if(timer)clearTimeout(timer)})
defineExpose({preview,payload})
</script>
<template>
 <view class="estimate-card"><text class="title">运费</text>
 <text v-if="locked" class="hint">运费已出账或已确认实际费用，收费规则已锁定。</text>
 <template v-else>
  <view class="field" @tap="searchOpen=!searchOpen">{{regionLabel||'选择收件省市区'}} ›</view>
  <view v-if="searchOpen"><input v-model="query" class="field" placeholder="搜索省市区，如孟连县" /><scroll-view scroll-y class="region-options"><view v-for="r in matches" :key="r.code" class="region-row" @tap="selectRegion(r.code)">{{r.path||r.name}}</view><text v-if="!matches.length" class="hint">没有符合条件的地区</text></scroll-view></view>
  <text class="label">本单运费（元，留空自动）</text><input :value="state.manual" :disabled="!!state.free" type="digit" class="field" placeholder="留空自动" @input="inputAmount" />
  <view class="switch"><text>包邮</text><switch :checked="!!state.free" @change="toggleFree" /></view>
 </template>
 <text v-if="error" class="error">{{error}}</text><text v-else-if="busy" class="hint">正在核算运费…</text><text v-else-if="quote" class="amount">{{freightLabel(quote)}}</text><text v-else class="hint">{{customerId&&items.length?'准备核算运费…':'添加商品后自动计算'}}</text>
 <text v-if="!state.free && !state.manual" class="hint">自动预估不计当前应付，实际运费核对出账后另收。</text><text v-else-if="!state.free" class="hint">手填运费计入本单应付，实际快递账单仅核对成本。</text>
 <text v-if="quote && !quote.estimate_known && quote.cost_pending_reason" class="hint">{{quote.cost_pending_reason}}</text>
 <button v-if="error&&!locked" @tap="preview">重试核算</button></view>
</template>
<style scoped>
.estimate-card{padding:24rpx;background:#eff6f0;border:1px solid #d5e5d9;border-radius:14rpx;margin:24rpx 0}.title{font-size:30rpx;font-weight:700;color:#244c36}.field{margin-top:16rpx;padding:20rpx;background:white;border:1px solid #cbd9cf;border-radius:10rpx;font-size:28rpx}.label{display:block;margin-top:20rpx;font-size:26rpx}.hint,.error,.amount{display:block;font-size:25rpx;line-height:1.7;margin:12rpx 0}.hint{color:#718276}.error{color:#b13b2f}.amount{color:#286248;font-weight:600}.switch{display:flex;align-items:center;justify-content:space-between;font-size:26rpx;margin:10rpx 0}button{font-size:26rpx;color:#286248;background:white;border:1px solid #cbd9cf;margin-top:14rpx}.region-options{max-height:420rpx}.region-row{padding:20rpx 8rpx;border-bottom:1px solid #d5e5d9;font-size:26rpx}
</style>
