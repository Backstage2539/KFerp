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
import {miniRequest} from '../../api/client'
import {fetchMe} from '../../api/customerPortal'
import {useSessionStore} from '../../stores/session'
import {loginRouteFor} from '../../utils/loginReturn'
const session=useSessionStore(),busy=ref(false),error=ref(''),message=ref(''),ready=ref(false),qr=ref(''),challenge=ref(''),enabled=ref(false),subscribed=ref(false),bound=ref(false),events=ref<string[]>([])
const choices=[['created','下单成功'],['paid','付款成功'],['production_started','开始生产'],['production_finished','生产完成'],['shipped','发货进度'],['cancelled','订单取消']]
const base='/api/mini/order-notifications',destination='/pages/order-notifications/order-notifications'
const account=computed(()=>session.accountType==='employee'?'员工账号':session.currentCustomerName||'客户账号')
async function request<T>(path='',method:'GET'|'POST'|'PUT'='GET',data?:Record<string,unknown>){return miniRequest<T>(base+path,{token:session.token,method,data})}
async function run(fn:()=>Promise<void>){if(busy.value)return;busy.value=true;error.value='';message.value='';try{await fn()}catch(e){error.value=e instanceof Error?e.message:'暂时无法操作'}finally{busy.value=false}}
async function load(){if(!session.token){uni.redirectTo({url:loginRouteFor(destination)});return}session.applyContext(await fetchMe(session.token));const v=await request<{enabled:boolean,rows:Array<{employee_id:number,customer_id:number,enabled:boolean,subscribed:boolean,events:string[]}>}>();ready.value=v.enabled;const p=v.rows.find(p=>session.accountType==='employee'?p.employee_id===session.employeeID:p.customer_id===session.currentCustomerID&&p.employee_id===0);bound.value=!!p;enabled.value=!!p?.enabled;subscribed.value=!!p?.subscribed;events.value=p?.events||choices.map(v=>v[0])}
async function bind(){const v=await request<{qr_url:string,token:string}>('/binding','POST');qr.value=v.qr_url;challenge.value=v.token}
async function confirm(){const v=await request<{scanned:boolean}>('/binding/'+challenge.value);if(!v.scanned)throw Error('请先识别二维码并关注服务号，再返回确认');await request('/binding/'+challenge.value+'/confirm','POST');qr.value='';challenge.value='';await load();message.value='已确认接收订单通知'}
async function save(){await request('','PUT',{enabled:enabled.value,events:events.value});await load();message.value='通知偏好已保存'}
function previewQR(){uni.previewImage({urls:[qr.value]})}
function toggleEnabled(e:Event){enabled.value=Boolean((e as unknown as {detail:{value:boolean}}).detail.value)}
function toggle(key:string){events.value=events.value.includes(key)?events.value.filter(v=>v!==key):[...events.value,key]}
onShow(()=>run(load))
</script>
<template>
<view class="page pull-up-brand-page" @touchstart="handlePullUpBrandTouchStart" @touchmove="handlePullUpBrandTouchMove" @touchend="handlePullUpBrandTouchEnd" @touchcancel="handlePullUpBrandTouchCancel"><EnvironmentBadge/><text class="title">订单通知</text><text>{{account}}</text><text class="hint">账号编号：{{session.miniUserID}}。服务号仅发送你有权查看的订单；你可随时关闭或取消关注。</text><text v-if="error" class="error">{{error}}</text><text v-if="message">{{message}}</text><text v-if="!ready">服务号尚未配置，请联系工作人员。</text><template v-else>
 <view class="card"><text>{{bound?(subscribed?'已关注并绑定':'关注状态待确认'):'尚未绑定服务号'}}</text><button :disabled="busy" @tap="run(bind)">{{bound?'重新确认关注账号':'生成关注二维码'}}</button><template v-if="qr"><text>识别下面的二维码，关注服务号后返回本页。二维码 5 分钟有效。确认后开启当前账号的订单通知。</text><image :src="qr" mode="widthFix" show-menu-by-longpress @tap="previewQR"/><button :disabled="busy" @tap="run(confirm)">已关注，确认接收通知</button></template></view>
 <view v-if="bound" class="card"><label><switch :checked="enabled" :disabled="busy" @change="toggleEnabled"/>接收订单通知</label><label v-for="[key,label] in choices" :key="key" @tap="toggle(key)"><checkbox :checked="events.includes(key)"/>{{label}}</label><button :disabled="busy" @tap="run(save)">保存通知偏好</button></view>
 </template><view class="pull-up-brand-footer-anchor"><PullUpBrandFooter :revealed="pullUpBrandRevealed" /></view></view>
</template>
<style scoped>.page{display:flex;flex-direction:column;gap:24rpx;padding:32rpx;background:#f6f7f8;min-height:100vh;box-sizing:border-box}.title{font-size:38rpx;font-weight:600}.hint{color:#687381;font-size:25rpx;line-height:1.6}.card{display:flex;flex-direction:column;gap:24rpx;background:#fff;padding:28rpx;border-radius:18rpx}.card image{width:440rpx;align-self:center}.card button{width:100%;background:#28553a;color:white}.card label{display:flex;gap:16rpx;align-items:center}.error{color:#b42323}</style>
