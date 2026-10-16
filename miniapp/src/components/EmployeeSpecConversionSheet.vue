<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { fetchEmployeeSpecConversions, quoteEmployeeSpecConversion, type EmployeeOrderDraftItem, type EmployeeOrderProductFamily, type SpecConversionCatalog, type SpecConversionQuote } from '../api/customerPortal'
import { conversionTargets, conversionDescription } from '../utils/specConversion'
const props = defineProps<{ item:EmployeeOrderDraftItem;family?:EmployeeOrderProductFamily;customerId:number;token:string }>()
const emit = defineEmits<{ close:[];apply:[quote:SpecConversionQuote] }>()
const catalog=ref<SpecConversionCatalog>({groups:[],specs:[]}), sourceID=ref(Number(props.item.spec_conversion?.source_bom_spec_id || props.item.bom_spec_id || 0)), rowKey=ref(props.item.selected_price_row_key || ''), groupID=ref(props.item.spec_conversion?.group_id || 0), targetID=ref(props.item.spec_conversion ? Number(props.item.bom_spec_id) : 0), quote=ref<SpecConversionQuote>(),error=ref(''),busy=ref(false)
let sequence=0, catalogSequence=0, timer:ReturnType<typeof setTimeout> | undefined
const sources=computed(()=>[...new Map((catalog.value.source_tiers || []).map(s=>[s.bom_spec_id,s])).values()])
const source=computed(()=>sources.value.find(s=>Number(s.bom_spec_id)===sourceID.value))
const tiers=computed(()=> (catalog.value.source_tiers || []).filter(t=>t.bom_spec_id===sourceID.value))
const groups=computed(()=>catalog.value.groups.filter(g=>conversionTargets(g,sources.value).some(s=>s.bom_spec_id===sourceID.value)))
const group=computed(()=>groups.value.find(g=>g.id===groupID.value))
const targets=computed(()=>conversionTargets(group.value,catalog.value.specs).filter(s=>s.bom_spec_id!==sourceID.value))
const sourceLabels=computed(()=>sources.value.map(s=>s.spec_name || '规格'))
const tierLabels=computed(()=>tiers.value.map(t=>`${t.tier_label || '价格档'} · ¥${t.unit_price}`))
const groupLabels=computed(()=>groups.value.map(g=>`${g.name}（修订${g.revision}）`))
const targetLabels=computed(()=>targets.value.map(s=>`${s.spec_name}（${s.inventory_unit}）`))
function pickSource(event:{detail:{value:string|number}}) { sourceID.value=Number(sources.value[Number(event.detail.value)]?.bom_spec_id || 0);rowKey.value='';groupID.value=0;targetID.value=0 }
function pickTier(event:{detail:{value:string|number}}) { rowKey.value=tiers.value[Number(event.detail.value)]?.price_row_key || '' }
function pickGroup(event:{detail:{value:string|number}}) { groupID.value=groups.value[Number(event.detail.value)]?.id || 0;targetID.value=0 }
function pickTarget(event:{detail:{value:string|number}}) { targetID.value=targets.value[Number(event.detail.value)]?.bom_spec_id || 0 }
async function load() { const current=++catalogSequence;sequence++;clearTimeout(timer);quote.value=undefined;busy.value=false; try { const result=await fetchEmployeeSpecConversions(props.token,props.customerId,props.item.product_id,Number(props.item.bean_list_publication_id));if(current!==catalogSequence)return;catalog.value=result;error.value='';schedule() } catch(e) {if(current===catalogSequence)error.value=e instanceof Error?e.message:'换算组加载失败'} }
function schedule() {
 clearTimeout(timer);const current=++sequence;quote.value=undefined;error.value='';busy.value=false
 const target=targets.value.find(s=>s.bom_spec_id===targetID.value)
 if(!source.value || !group.value || !target || !tiers.value.some(t=>t.price_row_key===rowKey.value)) return
 const body={customer_id:props.customerId,product_id:props.item.product_id,publication_id:Number(props.item.bean_list_publication_id),group_id:group.value.id,revision:group.value.revision,source_bom_spec_id:sourceID.value,source_bom_variant_id:Number(tiers.value.find(t=>t.price_row_key===rowKey.value)?.bom_variant_id),price_row_key:rowKey.value,target_bom_spec_id:target.bom_spec_id,target_bom_variant_id:target.bom_variant_id,quantity:props.item.qty}
 busy.value=true;timer=setTimeout(async()=>{try{const result=await quoteEmployeeSpecConversion(props.token,body);if(current===sequence)quote.value=result}catch(e){if(current===sequence)error.value=e instanceof Error?e.message:'报价失败'}finally{if(current===sequence)busy.value=false}},250)
}
watch(()=>[sourceID.value,rowKey.value,groupID.value,targetID.value,props.item.qty,props.customerId,props.item.product_id,props.item.bean_list_publication_id],schedule)
watch(()=>[props.customerId,props.item.product_id,props.item.bean_list_publication_id],()=>{sourceID.value=0;rowKey.value='';groupID.value=0;targetID.value=0;catalog.value={groups:[],specs:[]};load()})
onMounted(load);onBeforeUnmount(()=>{catalogSequence++;sequence++;clearTimeout(timer)})
</script>
<template>
 <view class="conversion-mask" @tap.self="emit('close')"><scroll-view scroll-y class="conversion-sheet">
  <view class="heading"><text>临时换规格</text><button size="mini" @tap="emit('close')">关闭</button></view>
  <text class="hint">{{ item.product_name }} · {{ item.bean_list_version_no }}。数量变化保持所选档位。</text>
  <text class="label">来源规格</text><picker :range="sourceLabels" @change="pickSource"><view class="choice">{{ source?.spec_name || '请选择来源规格' }} ›</view></picker>
  <text class="label">来源档位</text><picker :range="tierLabels" @change="pickTier"><view class="choice">{{ tierLabels[tiers.findIndex(t=>t.price_row_key===rowKey)] || '请选择档位' }} ›</view></picker>
  <text class="label">换算组</text><picker :range="groupLabels" @change="pickGroup"><view class="choice">{{ group?.name || '请选择换算组' }} ›</view></picker>
  <button size="mini" @tap="load">刷新换算组</button><text class="hint">新增和维护换算组请到 ERP 商品管理的「规格换算」。</text>
  <text class="label">实际下单规格</text><picker :range="targetLabels" @change="pickTarget"><view class="choice">{{ targets.find(s=>s.bom_spec_id===targetID)?.spec_name || '请选择目标规格' }} ›</view></picker>
  <text v-if="error" class="error">{{ error }}</text>
  <view v-if="quote" class="result"><text>{{ conversionDescription(quote) }}</text><text class="amount">¥{{ quote.unit_price }} × {{ item.qty }} = ¥{{ quote.line_total }}</text><text>单价四舍五入到分。生产、库存和发货使用 {{ quote.target_spec_name }}。</text></view>
  <button class="apply" :disabled="!quote || busy" @tap="quote && emit('apply',quote)">{{ busy ? '正在报价…' : '使用此规格和价格' }}</button>
 </scroll-view></view>
</template>
<style scoped>
.conversion-mask{position:fixed;inset:0;background:rgba(0,0,0,.45);z-index:200;display:flex;align-items:flex-end}.conversion-sheet{background:#fff;border-radius:28rpx 28rpx 0 0;max-height:90vh;box-sizing:border-box;padding:32rpx 28rpx 60rpx}.heading{display:flex;justify-content:space-between;font-size:34rpx;font-weight:600}.heading button{margin:0}.hint,.label,.error{display:block;margin:20rpx 0;font-size:25rpx;line-height:1.6}.hint{color:#607569}.label{font-weight:600;margin-bottom:10rpx}.choice{padding:22rpx;border:1rpx solid #c5d1ca;border-radius:12rpx;margin-bottom:20rpx}.result{background:#edf7f0;padding:24rpx;border-radius:16rpx;margin:24rpx 0;font-size:26rpx;line-height:1.8}.result text{display:block}.amount{font-size:32rpx;font-weight:600;margin:20rpx 0}.error{color:#b42318}.apply{background:#176542;color:#fff}.apply[disabled]{opacity:.5}
</style>
