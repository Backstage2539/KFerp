<script setup lang="ts">
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import {ref} from 'vue'
import {onLoad,onShow,onShareAppMessage,onShareTimeline} from '@dcloudio/uni-app'
import {fetchMe,switchCurrentCustomer} from '../../api/customerPortal'
import {safeLoginReturn,loginRouteFor} from '../../utils/loginReturn'
import {useSessionStore} from '../../stores/session'
const session=useSessionStore(),destination=ref(''),error=ref(''),busy=ref(false)
onLoad(q=>{try{destination.value=safeLoginReturn(decodeURIComponent(String(q?.return_to||'')))}catch{}})
onShow(async()=>{try{session.applyContext(await fetchMe(session.token))}catch{uni.redirectTo({url:loginRouteFor(destination.value)})}})
async function select(id:number){if(busy.value)return;busy.value=true;try{session.applyContext(await switchCurrentCustomer(session.token,id));uni.reLaunch({url:destination.value||'/pages/home/home'})}catch(e){error.value=e instanceof Error?e.message:'选择失败'}finally{busy.value=false}}
function back(){uni.reLaunch({url:'/pages/bean-list-center/bean-list-center'})}
import {defaultMiniappShare,defaultMiniappTimelineShare,refreshMiniappShareMenu} from '../../utils/miniappShare'
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
onShow(()=>{void refreshMiniappShareMenu()})
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
const {pullUpBrandRevealed,handlePullUpBrandTouchStart,handlePullUpBrandTouchMove,handlePullUpBrandTouchEnd,handlePullUpBrandTouchCancel}=usePullUpBrandGesture()
</script>
<template><view class="page pull-up-brand-page" @touchstart="handlePullUpBrandTouchStart" @touchmove="handlePullUpBrandTouchMove" @touchend="handlePullUpBrandTouchEnd" @touchcancel="handlePullUpBrandTouchCancel"><EnvironmentBadge/><text class="title">请选择本次使用的客户</text><text>仅查看所选客户有权限的业务。</text><button v-for="binding in session.bindings" :key="binding.customer_id" :disabled="busy" @tap="select(binding.customer_id)">{{binding.customer_name}}</button><text v-if="!session.bindings.length">暂无有效客户权限，请使用已有客户账号登录或联系工作人员。</text><text>{{error}}</text><button @tap="back">返回豆单中心</button><view class="pull-up-brand-footer-anchor"><PullUpBrandFooter :revealed="pullUpBrandRevealed" /></view></view></template>
<style scoped>.page{padding:48rpx 30rpx;display:flex;flex-direction:column;gap:24rpx;background:#f7f4ed;min-height:100vh}.title{font-size:36rpx;font-weight:700}.page button{width:100%;background:white;color:#28553a;padding:14rpx;white-space:normal}</style>
