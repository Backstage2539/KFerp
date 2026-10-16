<script setup lang="ts">
import {computed,onBeforeUnmount} from 'vue'
import {useSampleSubmission} from '../composables/useSampleSubmission'
import type {BeanListProductSummary} from '../api/customerPortal'
import {submitSample,type SampleSubmit} from '../api/samples'
import {useSessionStore} from '../stores/session'
import {sampleDraftID} from '../utils/samples'
import {miniappStorageKey,configuredMiniappEnvironment} from '../config/environment'
const props=defineProps<{entryKey:string;publicationId:number;items:BeanListProductSummary[]}>()
const emit=defineEmits<{(e:'close'):void;(e:'submitted',id:number):void}>()
const session=useSessionStore()
const key=miniappStorageKey(`sample-draft:${session.miniUserID}:${props.entryKey}:${props.publicationId}`,configuredMiniappEnvironment())
let saved:{payload?:SampleSubmit;pending?:boolean}|undefined
try{saved=uni.getStorageSync(key)}catch{/* Submission reports a storage error before sending. */}
const sameProducts=JSON.stringify(saved?.payload?.product_keys)===JSON.stringify(props.items.map(i=>i.sample_key))
const payload:SampleSubmit=sameProducts&&saved?.payload?saved.payload:{request_id:sampleDraftID(),entry_key:props.entryKey,publication_id:props.publicationId,product_keys:props.items.map(i=>i.sample_key||''),receiver:{name:session.registration?.nickname||'',phone:session.registration?.verified_phone||'',address:''},note:''}
const {draft,busy,error,pending,successID,submit,dispose}=useSampleSubmission({payload,pending:!!(sameProducts&&saved?.pending),storage:{get:()=>uni.getStorageSync(key),set:value=>uni.setStorageSync(key,value),remove:()=>uni.removeStorageSync(key)},request:value=>submitSample(session.token,value),onSuccess:id=>emit('submitted',id)})
onBeforeUnmount(dispose)
const canSubmit=computed(()=>!!draft.value.receiver.name.trim()&&!!draft.value.receiver.phone.trim()&&!!draft.value.receiver.address.trim())
function close(){if(!busy.value)emit('close')}
function progress(){uni.navigateTo({url:`/pages/sample-requests/sample-requests?id=${successID.value}`})}
</script>
<template><view class="overlay" @tap="close"><view class="sheet" @tap.stop>
<view class="heading"><text class="title">申请样品</text><button :disabled="busy" class="close" aria-label="关闭样品申请" @tap="close">×</button></view>
<template v-if="successID"><text class="success">申请已提交，工作人员将联系您，对接样品事项。</text><button class="primary" @tap="progress">查看申请进度</button><button @tap="close">继续查看价格表</button></template>
<scroll-view v-else scroll-y class="scroll"><text class="hint">已选 {{items.length}} 项商品。具体规格、数量和费用由工作人员联系确认。</text><text v-for="item in items" :key="item.sample_key" class="product">{{item.name}}</text>
<label class="label">收件人<input v-model="draft.receiver.name" :disabled="busy||pending" maxlength="100" placeholder="填写收件人"/></label>
<label class="label">收件电话<input v-model="draft.receiver.phone" :disabled="busy||pending" type="text" maxlength="32" placeholder="填写收件电话"/></label><text class="hint">修改收件电话不会更改账号已验证手机号。</text>
<label class="label">收件地址<textarea v-model="draft.receiver.address" :disabled="busy||pending" maxlength="500" placeholder="省市区、街道及门牌号"/></label>
<label class="label">留言（可选）<textarea v-model="draft.note" :disabled="busy||pending" maxlength="2000" placeholder="用途、期望规格或其他事项"/></label>
<text v-if="error" class="error">{{error}}</text><text v-if="pending&&!busy" class="hint">尚未确认提交结果，请重试确认。本次申请不会重复生成。</text>
<button class="primary" :disabled="busy||!canSubmit" :loading="busy" @tap="submit">{{pending?'重试确认申请':'提交申请'}}</button></scroll-view>
</view></view></template>
<style scoped>
.overlay{position:fixed;inset:0;background:#162e2580;z-index:1200;display:flex;align-items:flex-end}.sheet{background:#fffdf8;border-radius:28rpx 28rpx 0 0;padding:28rpx 28rpx calc(30rpx + env(safe-area-inset-bottom));width:100%;box-sizing:border-box;color:#253d31}.heading{display:flex;align-items:center;justify-content:space-between}.title{font-size:34rpx;font-weight:700}.close{margin:0;background:transparent;font-size:40rpx;line-height:1.5}.close::after{border:0}.scroll{max-height:74vh}.hint{display:block;font-size:24rpx;color:#758076;line-height:1.6;margin:16rpx 0}.product{display:block;font-size:26rpx;padding:8rpx 0}.label{display:block;font-size:27rpx;margin-top:22rpx}input,textarea{box-sizing:border-box;width:100%;background:#f2f5ee;padding:18rpx;margin-top:12rpx;border-radius:10rpx;font-size:28rpx}input{height:80rpx}textarea{height:145rpx}.primary{background:#285d3e;color:white;margin-top:24rpx;font-size:28rpx}.error{color:#b42318;display:block;white-space:pre-wrap;margin:20rpx 0}.success{display:block;line-height:1.8;margin:40rpx 0}
</style>
