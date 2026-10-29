<script setup lang="ts">
import { computed, ref } from 'vue'
import SampleApplication from '../../components/SampleApplication.vue'
import type { BeanListProductSummary } from '../../api/customerPortal'
import {canOpenPageEntry,pageEntryAccessAction} from '../../utils/priceCenterAccess'
import {restoreRegistrationSession} from '../../utils/registrationSession'
import {
  onLoad,
  onShow,
  onHide,
  onUnload,
  onShareAppMessage,
  onShareTimeline,
} from '@dcloudio/uni-app'
import { fetchPageEntry, type PageDocument } from '../../api/pageEntry'
import { buildAPIURL } from '../../api/client'
import { type BeanListSummary } from '../../api/customerPortal'
import {
  MiniRequestError,
} from '../../api/client'
import { useSessionStore } from '../../stores/session'
import { loginRouteFor } from '../../utils/loginReturn'
import PriceListContent from '../../components/PriceListContent.vue'
import { priceListSearchScope } from '../../utils/priceListSearch'
import { configuredMiniappEnvironment } from '../../config/environment'
import {
  defaultMiniappShare as baseShare,
  defaultMiniappTimelineShare as baseTimeline,
  refreshMiniappShareMenu,
} from '../../utils/miniappShare'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import BeanRegistrationSheet from '../../components/BeanRegistrationSheet.vue'
import {useBeanRegistration} from '../../composables/useBeanRegistration'
const document = ref<PageDocument | null>(null), images = ref<Record<string,string>>({})
const session = useSessionStore(),
  entry = ref(''),
  publication = ref<BeanListSummary | null>(null),
  loading = ref(false),
  error = ref(''),
  needsLogin = ref(false), needsCustomerLogin=ref(false), needsCustomerSelection=ref(false)
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
let sequence = 0
const sampleEnabled=ref(false), sampleOpen=ref(false), selectedKeys=ref<string[]>([]), sampleVersion=ref(0)
let openSampleAfterRegistration=false
const sampleItems=computed(()=>(publication.value?.groups||[]).flatMap(g=>g.items).filter(item=>!!item.sample_key&&selectedKeys.value.includes(item.sample_key)))
function toggleSample(item:BeanListProductSummary){if(!item.sample_key)return;selectedKeys.value=selectedKeys.value.includes(item.sample_key)?selectedKeys.value.filter(k=>k!==item.sample_key):[...selectedKeys.value,item.sample_key]}
async function applySamples(){if(!sampleItems.value.length)return;const request=sequence;try{await restoreRegistrationSession(session);if(request!==sequence)return;if(!session.registrationComplete){openSampleAfterRegistration=true;registration.openRequired(entry.value,'申请样品');return}sampleOpen.value=true}catch(e){if(request!==sequence)return;error.value=e instanceof Error?e.message:'身份恢复失败，请重试'}}
const registration=useBeanRegistration(()=>{const resumeSamples=openSampleAfterRegistration;openSampleAfterRegistration=false;const request=sequence+1;void load().then(()=>{if(request===sequence&&resumeSamples&&publication.value&&sampleItems.value.length)sampleOpen.value=true})})
function cancelRegistration(){registration.close();if(openSampleAfterRegistration){openSampleAfterRegistration=false;return}backToBeans()}
const destination = () => `/pages/page-entry/page-entry?entry=${entry.value}`
const defaultMiniappShare = () => ({
  ...baseShare(),
  title: document.value?.name || '棵凡咖啡',
  path: destination(),
})
const defaultMiniappTimelineShare = () => ({
  ...baseTimeline(),
  title: document.value?.name || '棵凡咖啡',
  query: `entry=${encodeURIComponent(entry.value)}`,
})
function backToBeans(){uni.reLaunch({url:'/pages/bean-list-center/bean-list-center'})}
function chooseCustomer(){uni.navigateTo({url:'/pages/customer-select/customer-select?return_to='+encodeURIComponent(destination())})}
function login() {
  uni.navigateTo({ url: loginRouteFor(destination()) })
}
async function load() {
  const request = ++sequence
  publication.value = null
  document.value = null
  images.value = {}
  error.value = ''
  needsLogin.value = false;needsCustomerLogin.value=false;needsCustomerSelection.value=false
  if (!/^[a-f0-9]{32}$/.test(entry.value)) {
    error.value = '页面入口无效'
    return
  }
  loading.value = true
  try {
    await restoreRegistrationSession(session)
    const data = await fetchPageEntry(entry.value, session.token)
    if(request!==sequence)return
    if(!canOpenPageEntry(session,data))throw new Error('只有销售或管理员可以查看价格表中心')
    const local: Record<string,string> = {}
    for (const block of data.document.blocks || []) {
      if (block.kind !== 'image' || !block.asset_id) continue
      const asset = block.asset_id
      local[asset] = await new Promise<string>((resolve,reject)=>uni.downloadFile({
        url: buildAPIURL(`/api/pages/${entry.value}/images/${asset}`), header: session.token ? {Authorization:`Bearer ${session.token}`} : {},
        success:r=>r.statusCode===200?resolve(r.tempFilePath):reject(new MiniRequestError('图片不可用，请重新加载',r.statusCode)), fail:()=>reject(new Error('图片加载失败'))
      }))
    }
    if (request === sequence) {
      publication.value = data.publication || null; document.value=data.document; images.value=local; sampleEnabled.value=!!data.sample_enabled
      if(data.publication && sampleVersion.value!==data.publication.id){selectedKeys.value=[];sampleVersion.value=data.publication.id}
      if(data.document.kind==='function' && data.target_path) uni.redirectTo({url:'/'+data.target_path})
    }
  } catch (e) {
    if (request !== sequence) return
    if(e instanceof MiniRequestError && e.statusCode===401 && e.code==='mini_registration_required') {
      needsLogin.value = true
      needsCustomerLogin.value = false
      error.value = '请先登录客户账号后查看'
      return
    }
    error.value = e instanceof Error ? e.message : '暂时无法加载页面'
    const action = pageEntryAccessAction(e)
    needsCustomerLogin.value = action === 'login' && session.accountType === 'customer'
    needsLogin.value = action === 'login'
    needsCustomerSelection.value = action === 'choose_customer'
    if (action === 'denied') error.value = '当前账号无权查看'
  } finally {
    if (request === sequence) loading.value = false
  }
}
onLoad((query) => {
  entry.value = String(query?.entry || '')
})
onShow(() => {
  if(!registration.visible.value) void load()
  void refreshMiniappShareMenu()
})
onHide(() => {
  sequence++
  sampleOpen.value=false
})
onUnload(()=>registration.close())
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
</script>
<template>
  <view
    class="page pull-up-brand-page"
    @touchstart="handlePullUpBrandTouchStart"
    @touchmove="handlePullUpBrandTouchMove"
    @touchend="handlePullUpBrandTouchEnd"
    @touchcancel="handlePullUpBrandTouchCancel"
  >
    <EnvironmentBadge />
    <text v-if="loading" class="state">正在加载页面…</text>
    <view v-if="error" class="state">
      <text>{{ error }}</text>
      <button v-if="needsLogin" @tap="login">{{needsCustomerLogin?'使用客户账号登录':'登记或登录后查看'}}</button>
 <button v-if="needsCustomerSelection" @tap="chooseCustomer">选择客户</button>
 <button @tap="backToBeans">返回价格表中心</button>
      <button v-if="!needsLogin" @tap="load">重新加载</button>
    </view>
    <view v-if="document?.kind === 'article'" class="sheet">
      <text class="title">{{ document.name }}</text>
      <view v-for="(block,index) in document.blocks || []" :key="index" class="article-block">
        <text v-if="block.kind === 'heading'" class="category">{{ block.text }}</text>
        <text v-else-if="block.kind === 'text'" class="paragraph">{{ block.text }}</text>
        <view v-else-if="block.kind === 'image'"><image v-if="images[block.asset_id || '']" :src="images[block.asset_id || '']" mode="widthFix" class="article-image"/><text class="muted">{{ block.caption }}</text></view>
      </view>
    </view>
    <PriceListContent v-if="publication" :publication="publication" :search-scope="priceListSearchScope(configuredMiniappEnvironment().environment, (session.miniUserID || -session.employeeID), session.currentCustomerID, entry)">
      <template #notice><text v-if="sampleEnabled&&session.accountType!=='employee'" class="sample-hint">勾选感兴趣的商品，即可申请样品。工作人员会联系确认寄样事项。</text></template>
      <template #product-action="{item}"><button v-if="sampleEnabled&&session.accountType!=='employee'&&item.sample_key" size="mini" :class="['sample-select',{selected:selectedKeys.includes(item.sample_key)}]" @tap="toggleSample(item)">{{selectedKeys.includes(item.sample_key)?'✓ 已选样品':'选择样品'}}</button></template>
    </PriceListContent>
    <view v-if="sampleEnabled&&session.accountType!=='employee'&&publication" class="sample-bar"><text>已选 {{sampleItems.length}} 项</text><button class="sample-submit" :disabled="!sampleItems.length||loading" @tap="applySamples">申请样品</button></view>
    <SampleApplication v-if="sampleOpen&&publication" :entry-key="entry" :publication-id="publication.id" :items="sampleItems" @close="sampleOpen=false" @submitted="selectedKeys=[]"/>
    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter :revealed="pullUpBrandRevealed" />
    </view>
    <BeanRegistrationSheet :visible="registration.visible.value" :title="registration.title.value" :nickname="registration.nickname.value" :busy="registration.busy.value" :error="registration.error.value" @nickname="registration.nickname.value=$event" @authorize="registration.submit" @close="cancelRegistration"/>
  </view>
</template>
<style scoped>
.sample-hint{display:block;font-size:24rpx;color:#58725e;line-height:1.7;padding:14rpx 0 24rpx}.sample-select{margin:12rpx 0;font-size:23rpx;line-height:2;background:#eef4e9;color:#356144}.sample-select.selected{background:#dcebd6}.sample-bar{position:sticky;bottom:20rpx;display:flex;align-items:center;justify-content:space-between;background:#fffdf8;padding:18rpx 24rpx;margin-top:24rpx;border:1rpx solid #d9e3da;border-radius:16rpx;color:#285d3e;font-size:28rpx;box-shadow:0 4rpx 22rpx #253d3115}.sample-submit{margin:0;font-size:27rpx;background:#285d3e;color:#fff}
.page{min-height:100vh;background:#f5f2ec;padding:24rpx;box-sizing:border-box}.state{display:flex;flex-direction:column;gap:24rpx;text-align:center;padding:80rpx 30rpx}.sheet{padding:40rpx 28rpx;border-radius:24rpx;background:#fffdf8}.title{display:block;font-size:42rpx;font-weight:800;text-align:center}.category{font-size:32rpx;font-weight:700}.article-block{margin-top:30rpx}.paragraph{white-space:pre-wrap;line-height:1.8}.article-image{width:100%;display:block}.muted{display:block;font-size:24rpx;line-height:1.7;margin-top:12rpx;color:#7c877e}
</style>
