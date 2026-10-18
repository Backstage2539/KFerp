<script setup lang="ts">
import {computed,ref} from 'vue'
import {onShow,onHide,onShareAppMessage,onShareTimeline} from '@dcloudio/uni-app'
import {fetchStaffNotifications,readStaffNotification,type StaffNotification} from '../../api/samples'
import {MiniRequestError} from '../../api/client'
import {fetchMe} from '../../api/customerPortal'
import {useSessionStore} from '../../stores/session'
import {canUseEmployeePriceCenter} from '../../utils/priceCenterAccess'
import {sampleDate} from '../../utils/samples'
import {loginRouteFor} from '../../utils/loginReturn'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import {defaultMiniappShare,defaultMiniappTimelineShare,refreshMiniappShareMenu} from '../../utils/miniappShare'
const {pullUpBrandRevealed,handlePullUpBrandTouchStart,handlePullUpBrandTouchMove,handlePullUpBrandTouchEnd,handlePullUpBrandTouchCancel}=usePullUpBrandGesture()
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
onShow(()=>{void refreshMiniappShareMenu()})
const session=useSessionStore(), rows=ref<StaffNotification[]>([]),loading=ref(false),error=ref(''),unreadOnly=ref(false),busy=ref(0),unread=ref(0),hasMore=ref(false),needsLogin=ref(false)
const visible=computed(()=>unreadOnly.value?rows.value.filter(row=>!row.read):rows.value)
let sequence=0
onHide(()=>{sequence++;rows.value=[]})
async function load(more=false){const request=++sequence;loading.value=true;error.value='';needsLogin.value=false;try{const context=await fetchMe(session.token);if(request!==sequence)return;session.applyContext(context);if(!canUseEmployeePriceCenter(session))throw new Error('只有销售或管理员可以查看员工通知');const response=await fetchStaffNotifications(session.token,unreadOnly.value,more?rows.value[rows.value.length-1]?.id:0);if(request!==sequence)return;const next=response.notifications||[];rows.value=more?[...rows.value,...next]:next;unread.value=response.unread_count||0;hasMore.value=next.length===100}catch(e){if(request!==sequence)return;if(!more)rows.value=[];needsLogin.value=e instanceof MiniRequestError&&e.statusCode===401;error.value=e instanceof Error?e.message:'消息加载失败'}finally{if(request===sequence)loading.value=false}}
async function open(row:StaffNotification){if(busy.value)return;const request=sequence,token=session.token;busy.value=row.id;error.value='';try{await readStaffNotification(session.token,row.id);if(request!==sequence||token!==session.token)return;if(!row.read){row.read=true;unread.value=Math.max(0,unread.value-1)}const id=Number(row.payload?.request_id||(row.source_type==='sample_request'?row.source_id:0));if(id>0)uni.navigateTo({url:`/pages/sample-requests/sample-requests?id=${id}`})}catch(e){if(request!==sequence)return;needsLogin.value=e instanceof MiniRequestError&&e.statusCode===401;error.value=e instanceof Error?e.message:'操作失败，请重试'}finally{busy.value=0}}
function toggleUnread(){unreadOnly.value=!unreadOnly.value;void load()}
function login(){uni.navigateTo({url:loginRouteFor('/pages/employee-notifications/employee-notifications')})}
function home(){uni.navigateTo({url:'/pages/home/home'})}
onShow(()=>{void load()})
</script>
<template><view class="page pull-up-brand-page" @touchstart="handlePullUpBrandTouchStart" @touchmove="handlePullUpBrandTouchMove" @touchend="handlePullUpBrandTouchEnd" @touchcancel="handlePullUpBrandTouchCancel"><EnvironmentBadge/><view class="heading"><text class="title">员工通知</text><button size="mini" @tap="home">员工首页</button></view><view class="filters"><text>未读 {{unread}} 条</text><button size="mini" @tap="toggleUnread">{{unreadOnly?'显示全部':'仅看未读'}}</button></view><text class="hint">点击标记已读并查看相关工单。</text><view v-if="error" class="error"><text>{{error}}</text><button @tap="load(false)">重新加载</button><button v-if="needsLogin||!session.token" @tap="login">登录</button></view><text v-if="loading" class="state">正在加载…</text><text v-else-if="!visible.length&&!error" class="state">暂无{{unreadOnly?'未读':''}}通知</text><view v-for="row in visible" :key="row.id" :class="['card',{unread:!row.read}]" @tap="open(row)"><view class="heading"><text class="strong">{{row.title}}</text><text class="badge">{{row.read?'已读':'未读'}}</text></view><text>{{row.body}}</text><text class="hint">{{sampleDate(row.created_at)}} {{busy===row.id?'正在打开…':'→'}}</text></view><button v-if="hasMore" :disabled="loading" @tap="load(true)">加载更早通知</button><view class="pull-up-brand-footer-anchor"><PullUpBrandFooter :revealed="pullUpBrandRevealed" /></view></view></template>
<style scoped>
.page{min-height:100vh;background:#f5f4ef;padding:28rpx 24rpx 60rpx;box-sizing:border-box;color:#253d31;font-size:28rpx}.heading,.filters{display:flex;align-items:center;justify-content:space-between;gap:16rpx;margin:16rpx 0}.title{font-size:38rpx;font-weight:800}.heading button,.filters button{margin:0;flex-shrink:0}.hint{display:block;color:#728075;font-size:24rpx;line-height:1.7;margin:14rpx 0}.card{padding:22rpx 28rpx;background:#fff;border-radius:18rpx;margin:20rpx 0;border-left:6rpx solid transparent;line-height:1.7}.card.unread{border-color:#285d3e}.strong{font-weight:700;font-size:29rpx}.badge{font-size:22rpx;flex-shrink:0;color:#5d7e60}.state{display:block;padding:60rpx 20rpx;text-align:center}.error{color:#b42318;padding:24rpx;background:#fff1ef;border-radius:16rpx}
</style>
