<script setup lang="ts">
import {ref,computed,nextTick} from 'vue'
import {canUseEmployeePriceCenter,hasAuthenticatedCustomerSession} from '../../utils/priceCenterAccess'
import {miniappStorageKey,configuredMiniappEnvironment} from '../../config/environment'
const browseKey=miniappStorageKey('bean-center-browse',configuredMiniappEnvironment())
import {onShow,onPageScroll,onShareAppMessage,onShareTimeline,onHide} from '@dcloudio/uni-app'
import {fetchBeanCenter,type BeanCard} from '../../api/registration'
import {beanCards,beanCenterPath} from '../../utils/beanCenter'
import {restoreRegistrationSession} from '../../utils/registrationSession'
import {useSessionStore} from '../../stores/session'
import MainTabBar from '../../components/MainTabBar.vue'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import CustomerPricePreview from '../../components/CustomerPricePreview.vue'
import { fetchDirectShipCatalog } from '../../api/customerPortal'
import { loginRouteFor } from '../../utils/loginReturn'
const saved=uni.getStorageSync(browseKey)||{}
const session=useSessionStore(), rows=ref<BeanCard[]>([]),search=ref(String(saved.search||'')),busy=ref(false),error=ref('')
const staff=computed(()=>canUseEmployeePriceCenter(session))
const authenticatedCustomer=computed(()=>hasAuthenticatedCustomerSession(session))
const priceUsage=ref<'product_order'|'direct_ship'>('product_order')
const enabledPrices=ref<Array<'product_order'|'direct_ship'>>([])
const priceError=ref('')
async function loadCustomerPrices(){ priceError.value=''; enabledPrices.value=[]; try { const caps=session.capabilities.filter(c=>c.enabled).map(c=>c.code); let modes:string[]=[]; if(caps.includes('direct_ship')) { const catalog=await fetchDirectShipCatalog(session.token); modes=(catalog.fulfillment_settings?.modes||[]).filter(m=>m.enabled).map(m=>m.mode) } if(caps.includes('product_order') || modes.includes('stock')) enabledPrices.value.push('product_order'); if(modes.includes('on_demand')) enabledPrices.value.push('direct_ship'); priceUsage.value=enabledPrices.value[0] || 'product_order' } catch(e){ priceError.value=e instanceof Error?e.message:'价格表配置读取失败，请重试' } }
function samples(){uni.navigateTo({url:'/pages/sample-requests/sample-requests'})}
function employeeHome(){uni.navigateTo({url:'/pages/home/home'})}
const filtered=computed(()=>beanCards(rows.value,search.value))
let scroll=Number(saved.scroll)||0
onHide(()=>uni.setStorageSync(browseKey,{search:search.value,scroll}))
onPageScroll(e=>{scroll=e.scrollTop})
async function load(){const position=scroll;busy.value=true;error.value='';try{await restoreRegistrationSession(session);if(session.accountType==='employee'&&!staff.value)throw new Error('只有销售或管理员可以查看价格表中心');if(authenticatedCustomer.value && session.currentCustomerID) await loadCustomerPrices(); else rows.value=(await fetchBeanCenter()).rows;await nextTick();uni.pageScrollTo({scrollTop:position,duration:0})}catch(e){rows.value=[];error.value=e instanceof Error?e.message:'价格表加载失败'}finally{busy.value=false}}
async function open(card:BeanCard){if(card.visibility !== 'public' && !authenticatedCustomer.value && !staff.value){uni.navigateTo({url:loginRouteFor(`/pages/page-entry/page-entry?entry=${card.key}`)});return}uni.navigateTo({url:`/pages/page-entry/page-entry?entry=${card.key}`})}
onShow(()=>{void load()})
const defaultMiniappShare=()=>({title:'棵凡咖啡 · 价格表中心',path:beanCenterPath})
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
import {defaultMiniappTimelineShare,refreshMiniappShareMenu} from '../../utils/miniappShare'
onShow(()=>{void refreshMiniappShareMenu()})
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
const {pullUpBrandRevealed,handlePullUpBrandTouchStart,handlePullUpBrandTouchMove,handlePullUpBrandTouchEnd,handlePullUpBrandTouchCancel}=usePullUpBrandGesture()
</script>
<template><view class="center pull-up-brand-page" @touchstart="handlePullUpBrandTouchStart" @touchmove="handlePullUpBrandTouchMove" @touchend="handlePullUpBrandTouchEnd" @touchcancel="handlePullUpBrandTouchCancel"><EnvironmentBadge/><view class="hero"><text class="eyebrow">QACOOHEE COFFEE</text><text class="title">价格表中心</text><text class="hint">选择一张价格表，查看商品、规格和价格。</text></view>
<view class="actions"><button v-if="staff||session.registrationComplete" @tap="samples">{{staff?'样品工单':'我的样品申请'}}</button><button v-if="staff" @tap="employeeHome">员工首页</button></view>
<template v-if="authenticatedCustomer && session.currentCustomerID"><view class="price-tabs"><button v-for="usage in enabledPrices" :key="usage" :class="{selected:priceUsage===usage}" @tap="priceUsage=usage">{{usage==='product_order'?'下单价格表':'一件代发价格表'}}</button></view><text v-if="priceError" class="state">{{priceError}}</text><text v-else-if="!busy && !enabledPrices.length" class="state">当前客户未开通价格表，请联系管理员。</text><CustomerPricePreview v-if="enabledPrices.length" :usage-code="priceUsage"/></template><template v-else>
<input v-model="search" class="search" placeholder="搜索价格表名称或版本"/>
<text v-if="session.accountType!=='employee'&&!session.registrationComplete&&!authenticatedCustomer" class="notice">查看受限价格表请先登录客户账号。</text>
<text v-if="busy" class="state">正在更新价格表…</text>
<view v-if="error" class="state"><text>{{error}}</text><button @tap="load">重新加载</button></view>
<view v-else-if="!busy&&!filtered.length" class="state">{{search?'没有找到相应价格表':'暂未发布价格表，请稍后查看'}}</view>
<button v-for="card in filtered" :key="card.key" class="card" @tap="open(card)"><view class="card-top"><text class="name">{{card.name}}</text><text class="arrow">›</text></view><text class="meta">版本 {{card.version}}</text><text class="meta">更新于 {{String(card.updated_at||'').slice(0,10)}}</text><text class="tag">{{card.visibility==='registered'?(authenticatedCustomer||session.registrationComplete||staff?'可直接查看':'登录后查看'):card.visibility==='public'?'公开价格表':'认证客户可见'}}</text></button>
</template><MainTabBar v-if="session.accountType!=='employee'" current="beans"/><view class="pull-up-brand-footer-anchor"><PullUpBrandFooter :revealed="pullUpBrandRevealed" :with-fixed-tabbar="session.accountType!=='employee'"/></view>

</view></template>
<style scoped>
.actions{display:flex;gap:16rpx;margin-bottom:20rpx}.actions button{flex:1;margin:0;font-size:25rpx;background:#e8efe3;color:#285d3e}
.center{min-height:100vh;box-sizing:border-box;padding:36rpx 28rpx calc(160rpx + env(safe-area-inset-bottom));background:#f6f4ee;color:#253d31}.hero{display:flex;flex-direction:column;gap:12rpx;margin:22rpx 0 34rpx}.eyebrow{font-size:22rpx;letter-spacing:3rpx;color:#718277}.title{font-size:48rpx;font-weight:800}.hint,.notice,.meta{font-size:25rpx;color:#69766c;line-height:1.6}.search{height:84rpx;background:white;border:1rpx solid #d9e3da;border-radius:12rpx;padding:0 24rpx;font-size:28rpx}.notice{display:block;padding:20rpx 0}.card{margin:20rpx 0;padding:30rpx;text-align:left;background:#fff;border-radius:18rpx;line-height:1.5}.card::after{border:1rpx solid #dfe5dc;border-radius:18rpx}.card-top{display:flex;align-items:center;justify-content:space-between;gap:20rpx}.name{font-size:32rpx;font-weight:700;overflow-wrap:anywhere}.arrow{font-size:44rpx;color:#5a7b62}.meta{display:block}.tag{display:inline-block;font-size:22rpx;color:#356144;background:#edf4eb;padding:4rpx 14rpx;border-radius:8rpx;margin-top:16rpx}.state{display:block;text-align:center;padding:48rpx 20rpx;color:#72776c;font-size:28rpx}
.price-tabs{display:flex;gap:12rpx;margin:20rpx 0}.price-tabs button{flex:1;margin:0;font-size:25rpx;line-height:1.4;padding:18rpx 8rpx;background:#eef3ec;color:#285d3e}.price-tabs button.selected{background:#285d3e;color:white}
</style>
