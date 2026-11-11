<script setup lang="ts">
import {ref,computed} from 'vue'
import {onShow,onShareAppMessage,onShareTimeline} from '@dcloudio/uni-app'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import {defaultMiniappShare,defaultMiniappTimelineShare,refreshMiniappShareMenu} from '../../utils/miniappShare'
const {pullUpBrandRevealed,handlePullUpBrandTouchStart,handlePullUpBrandTouchMove,handlePullUpBrandTouchEnd,handlePullUpBrandTouchCancel}=usePullUpBrandGesture()
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
onShow(()=>{void refreshMiniappShareMenu()})
import {MiniRequestError,miniRequest} from '../../api/client'
import {fetchMe} from '../../api/customerPortal'
import {useSessionStore} from '../../stores/session'
import {loginRouteFor} from '../../utils/loginReturn'
import {orderNotificationChoices} from '../../utils/orderNotificationChoices'
const session=useSessionStore(),busy=ref(false),error=ref(''),message=ref(''),ready=ref(false),qr=ref(''),challenge=ref(''),enabled=ref(false),subscribed=ref(false),bound=ref(false),events=ref<string[]>([])
const promptVersion=ref<number|null>(null)
const choices=orderNotificationChoices
const base='/api/mini/order-notifications',destination='/pages/order-notifications/order-notifications'
const account=computed(()=>session.accountType==='employee'?'员工账号':session.currentCustomerName||'客户账号')
async function request<T>(path='',method:'GET'|'POST'|'PUT'='GET',data?:Record<string,unknown>){return miniRequest<T>(base+path,{token:session.token,method,data})}
async function run(fn:()=>Promise<void>){if(busy.value)return;busy.value=true;error.value='';message.value='';try{await fn()}catch(e){error.value=e instanceof Error?e.message:'暂时无法操作'}finally{busy.value=false}}
async function load(){if(!session.token){uni.redirectTo({url:loginRouteFor(destination)});return}session.applyContext(await fetchMe(session.token));const v=await request<{enabled:boolean,prompt_version?:number,mini_user_id?:number,customer_id?:number,employee_id?:number,rows:Array<{employee_id:number,customer_id:number,active?:boolean,enabled:boolean,subscribed:boolean,events:string[]}>}>();if(v.mini_user_id!==undefined&&(v.mini_user_id!==session.miniUserID||v.customer_id!==session.currentCustomerID||v.employee_id!==session.employeeID))throw Error('当前账号已变化，请重新打开通知设置');ready.value=v.enabled;promptVersion.value=typeof v.prompt_version==='number'?v.prompt_version:null;const p=v.rows.find(p=>session.accountType==='employee'?p.employee_id===session.employeeID:p.customer_id===session.currentCustomerID&&p.employee_id===0);bound.value=!!p&&p.active!==false;enabled.value=!!p?.enabled;subscribed.value=!!p?.subscribed;events.value=p?.events||[]}
async function bind(){const v=await request<{qr_url:string,token:string}>('/binding','POST');qr.value=v.qr_url;challenge.value=v.token}
async function confirm(){const v=await request<{scanned:boolean}>('/binding/'+challenge.value);if(!v.scanned)throw Error('请先识别二维码并关注服务号，再返回确认');await request('/binding/'+challenge.value+'/confirm','POST');qr.value='';challenge.value='';await load();message.value=enabled.value?'已绑定服务号，订单通知已开启':'已绑定服务号，订单通知尚未开启'}
async function save(){try{await request('','PUT',{enabled:enabled.value,events:events.value,...(promptVersion.value===null?{}:{prompt_version:promptVersion.value,mini_user_id:session.miniUserID,customer_id:session.currentCustomerID,employee_id:session.employeeID})})}catch(e){if(e instanceof MiniRequestError&&e.statusCode===409){await load();throw Error('账号或通知选项已更新，请重新选择后保存')}throw e}await load();message.value='通知偏好已保存'}
function previewQR(){uni.previewImage({urls:[qr.value]})}
function toggleEnabled(e:Event){enabled.value=Boolean((e as unknown as {detail:{value:boolean}}).detail.value)}
function toggle(key:string){events.value=events.value.includes(key)?events.value.filter(v=>v!==key):[...events.value,key]}
onShow(()=>run(load))
</script>
<template>
<view class="page pull-up-brand-page" @touchstart="handlePullUpBrandTouchStart" @touchmove="handlePullUpBrandTouchMove" @touchend="handlePullUpBrandTouchEnd" @touchcancel="handlePullUpBrandTouchCancel"><EnvironmentBadge/><text class="title">订单通知</text><text>{{account}}</text><text class="hint">账号编号：{{session.miniUserID}}。服务号仅发送你有权查看的订单；你可随时关闭或取消关注。部分通知尚未开通，选中后仍以服务号模板和系统启用情况为准。</text><text v-if="error" class="error">{{error}}</text><text v-if="message">{{message}}</text><text v-if="!ready">服务号尚未配置，请联系工作人员。</text><template v-else>
 <view class="card"><text>{{bound?(subscribed?'已关注并绑定':'关注状态待确认'):'尚未绑定服务号'}}</text><button :disabled="busy" @tap="run(bind)">{{bound?'重新确认关注账号':'生成关注二维码'}}</button><template v-if="qr"><text>识别下面的二维码，关注服务号后返回本页。二维码 5 分钟有效。确认后按你的选择接收通知。</text><image :src="qr" mode="widthFix" show-menu-by-longpress @tap="previewQR"/><button :disabled="busy" @tap="run(confirm)">已关注，确认接收通知</button></template></view>
 <view v-if="bound" class="card"><label><switch :checked="enabled" :disabled="busy" @change="toggleEnabled"/>接收订单通知</label><label v-for="[key,label] in choices" :key="key" @tap="toggle(key)"><checkbox :checked="events.includes(key)"/>{{label}}</label><button :disabled="busy" @tap="run(save)">保存通知偏好</button></view>
 </template><view class="pull-up-brand-footer-anchor"><PullUpBrandFooter :revealed="pullUpBrandRevealed" /></view></view>
</template>
<style scoped>.page{display:flex;flex-direction:column;gap:24rpx;padding:32rpx;background:#f6f7f8;min-height:100vh;box-sizing:border-box}.title{font-size:38rpx;font-weight:600}.hint{color:#687381;font-size:25rpx;line-height:1.6}.card{display:flex;flex-direction:column;gap:24rpx;background:#fff;padding:28rpx;border-radius:18rpx}.card image{width:440rpx;align-self:center}.card button{width:100%;background:#28553a;color:white}.card label{display:flex;gap:16rpx;align-items:center}.error{color:#b42323}</style>
