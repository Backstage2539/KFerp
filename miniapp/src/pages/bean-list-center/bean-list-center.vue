<script setup lang="ts">
import {ref,computed,nextTick} from 'vue'
import {miniappStorageKey,configuredMiniappEnvironment} from '../../config/environment'
const browseKey=miniappStorageKey('bean-center-browse',configuredMiniappEnvironment())
import {onShow,onPageScroll,onShareAppMessage,onShareTimeline,onHide} from '@dcloudio/uni-app'
import {fetchBeanCenter,type BeanCard} from '../../api/registration'
import {beanCards,beanCenterPath} from '../../utils/beanCenter'
import {restoreRegistrationSession} from '../../utils/registrationSession'
import {useSessionStore} from '../../stores/session'
import MainTabBar from '../../components/MainTabBar.vue'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
const saved=uni.getStorageSync(browseKey)||{}
const session=useSessionStore(), rows=ref<BeanCard[]>([]),search=ref(String(saved.search||'')),busy=ref(false),error=ref('')
const filtered=computed(()=>beanCards(rows.value,search.value))
let scroll=Number(saved.scroll)||0
onHide(()=>uni.setStorageSync(browseKey,{search:search.value,scroll}))
onPageScroll(e=>{scroll=e.scrollTop})
async function load(){const position=scroll;busy.value=true;error.value='';try{rows.value=(await fetchBeanCenter()).rows;await nextTick();uni.pageScrollTo({scrollTop:position,duration:0})}catch(e){rows.value=[];error.value=e instanceof Error?e.message:'豆单加载失败'}finally{busy.value=false}}
function open(card:BeanCard){uni.navigateTo({url:`/pages/page-entry/page-entry?entry=${card.key}`})}
onShow(()=>{void load();void restoreRegistrationSession(session).catch(()=>{})})
const defaultMiniappShare=()=>({title:'棵凡咖啡 · 豆单中心',path:beanCenterPath})
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
import {defaultMiniappTimelineShare,refreshMiniappShareMenu} from '../../utils/miniappShare'
onShow(()=>{void refreshMiniappShareMenu()})
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
const {pullUpBrandRevealed,handlePullUpBrandTouchStart,handlePullUpBrandTouchMove,handlePullUpBrandTouchEnd,handlePullUpBrandTouchCancel}=usePullUpBrandGesture()
</script>
<template><view class="center pull-up-brand-page" @touchstart="handlePullUpBrandTouchStart" @touchmove="handlePullUpBrandTouchMove" @touchend="handlePullUpBrandTouchEnd" @touchcancel="handlePullUpBrandTouchCancel"><EnvironmentBadge/><view class="hero"><text class="eyebrow">QACOOHEE COFFEE</text><text class="title">豆单中心</text><text class="hint">选择一张价格表，查看商品、规格和价格。</text></view>
<input v-model="search" class="search" placeholder="搜索价格表名称或版本"/>
<text v-if="!session.registrationComplete" class="notice">首次查看登记豆单时，授权手机号并填写昵称即可。</text>
<text v-if="busy" class="state">正在更新豆单…</text>
<view v-if="error" class="state"><text>{{error}}</text><button @tap="load">重新加载</button></view>
<view v-else-if="!busy&&!filtered.length" class="state">{{search?'没有找到相应豆单':'暂未发布豆单，请稍后查看'}}</view>
<button v-for="card in filtered" :key="card.key" class="card" @tap="open(card)"><view class="card-top"><text class="name">{{card.name}}</text><text class="arrow">›</text></view><text class="meta">版本 {{card.version}}</text><text class="meta">更新于 {{String(card.updated_at||'').slice(0,10)}}</text><text class="tag">{{card.visibility==='registered'?'登记后查看':card.visibility==='public'?'公开豆单':'认证客户可见'}}</text></button>
<MainTabBar current="beans"/><view class="pull-up-brand-footer-anchor"><PullUpBrandFooter :revealed="pullUpBrandRevealed" :with-fixed-tabbar="true"/></view></view></template>
<style scoped>
.center{min-height:100vh;box-sizing:border-box;padding:36rpx 28rpx calc(160rpx + env(safe-area-inset-bottom));background:#f6f4ee;color:#253d31}.hero{display:flex;flex-direction:column;gap:12rpx;margin:22rpx 0 34rpx}.eyebrow{font-size:22rpx;letter-spacing:3rpx;color:#718277}.title{font-size:48rpx;font-weight:800}.hint,.notice,.meta{font-size:25rpx;color:#69766c;line-height:1.6}.search{height:84rpx;background:white;border:1rpx solid #d9e3da;border-radius:12rpx;padding:0 24rpx;font-size:28rpx}.notice{display:block;padding:20rpx 0}.card{margin:20rpx 0;padding:30rpx;text-align:left;background:#fff;border-radius:18rpx;line-height:1.5}.card::after{border:1rpx solid #dfe5dc;border-radius:18rpx}.card-top{display:flex;align-items:center;justify-content:space-between;gap:20rpx}.name{font-size:32rpx;font-weight:700;overflow-wrap:anywhere}.arrow{font-size:44rpx;color:#5a7b62}.meta{display:block}.tag{display:inline-block;font-size:22rpx;color:#356144;background:#edf4eb;padding:4rpx 14rpx;border-radius:8rpx;margin-top:16rpx}.state{display:block;text-align:center;padding:48rpx 20rpx;color:#72776c;font-size:28rpx}
</style>
