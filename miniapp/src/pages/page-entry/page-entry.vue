<script setup lang="ts">
import { ref } from 'vue'
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
  needsLogin = ref(false), needsCustomerLogin=ref(false)
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
let sequence = 0
const registration=useBeanRegistration(()=>{void load()})
function cancelRegistration(){registration.close();backToBeans()}
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
  uni.navigateTo({ url: loginRouteFor(destination())+(needsCustomerLogin.value?'&mode=password':'') })
}
async function load() {
  const request = ++sequence
  publication.value = null
  document.value = null
  images.value = {}
  error.value = ''
  needsLogin.value = false;needsCustomerLogin.value=false
  if (!/^[a-f0-9]{32}$/.test(entry.value)) {
    error.value = '页面入口无效'
    return
  }
  loading.value = true
  try {
    await restoreRegistrationSession(session)
    const data = await fetchPageEntry(entry.value, session.token)
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
      publication.value = data.publication || null; document.value=data.document; images.value=local
      if(data.document.kind==='function' && data.target_path) uni.redirectTo({url:'/'+data.target_path})
    }
  } catch (e) {
    if (request !== sequence) return
    if(e instanceof MiniRequestError && e.statusCode===401 && e.code==='mini_registration_required') {
      registration.openRequired(entry.value)
      return
    }
    error.value = e instanceof Error ? e.message : '暂时无法加载页面'
    needsCustomerLogin.value=e instanceof MiniRequestError && e.statusCode===403
    needsLogin.value = e instanceof MiniRequestError && (e.statusCode === 401 || e.statusCode === 403)
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
  publication.value = null
  document.value = null
  images.value = {}
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
 <button v-if="needsCustomerLogin && session.bindings.length>1" @tap="chooseCustomer">选择客户</button>
 <button @tap="backToBeans">返回豆单中心</button>
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
    <PriceListContent v-if="publication" :publication="publication" :search-scope="priceListSearchScope(configuredMiniappEnvironment().environment, (session.miniUserID || -session.employeeID), session.currentCustomerID, entry)"/>
    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter :revealed="pullUpBrandRevealed" />
    </view>
    <BeanRegistrationSheet :visible="registration.visible.value" :title="registration.title.value" :nickname="registration.nickname.value" :busy="registration.busy.value" :error="registration.error.value" @nickname="registration.nickname.value=$event" @authorize="registration.submit" @close="cancelRegistration"/>
  </view>
</template>
<style scoped>
.page{min-height:100vh;background:#f5f2ec;padding:24rpx;box-sizing:border-box}.state{display:flex;flex-direction:column;gap:24rpx;text-align:center;padding:80rpx 30rpx}.sheet{padding:40rpx 28rpx;border-radius:24rpx;background:#fffdf8}.title{display:block;font-size:42rpx;font-weight:800;text-align:center}.category{font-size:32rpx;font-weight:700}.article-block{margin-top:30rpx}.paragraph{white-space:pre-wrap;line-height:1.8}.article-image{width:100%;display:block}.muted{display:block;font-size:24rpx;line-height:1.7;margin-top:12rpx;color:#7c877e}
</style>
