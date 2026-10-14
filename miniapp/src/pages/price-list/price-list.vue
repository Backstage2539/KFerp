<script setup lang="ts">
import { ref } from 'vue'
import {
  onLoad,
  onShow,
  onHide,
  onShareAppMessage,
  onShareTimeline,
} from '@dcloudio/uni-app'
import { fetchFixedPriceEntry } from '../../api/officialAccount'
import { fetchMe, type BeanListSummary } from '../../api/customerPortal'
import {
  MiniRequestError,
  isAuthenticationExpiredRequestError,
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
const session = useSessionStore(),
  entry = ref(''),
  publication = ref<BeanListSummary | null>(null),
  loading = ref(false),
  error = ref(''),
  needsLogin = ref(false)
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
let sequence = 0
const destination = () => `/pages/price-list/price-list?entry=${entry.value}`
const defaultMiniappShare = () => ({
  ...baseShare(),
  title: publication.value?.title || '棵凡咖啡价格表',
  path: destination(),
})
const defaultMiniappTimelineShare = () => ({
  ...baseTimeline(),
  title: publication.value?.title || '棵凡咖啡价格表',
  query: `entry=${encodeURIComponent(entry.value)}`,
})
function login() {
  uni.navigateTo({ url: loginRouteFor(destination()) })
}
async function load() {
  const request = ++sequence
  publication.value = null
  error.value = ''
  needsLogin.value = false
  if (!/^[a-f0-9]{32}$/.test(entry.value)) {
    error.value = '豆单入口无效'
    return
  }
  loading.value = true
  try {
    if (session.token) {
      try {
        session.applyContext(await fetchMe(session.token))
      } catch (e) {
        if (isAuthenticationExpiredRequestError(e)) session.clearSession()
        else throw e
      }
    }
    const data = await fetchFixedPriceEntry(entry.value, session.token)
    if (request === sequence) publication.value = data.publication
  } catch (e) {
    if (request !== sequence) return
    error.value = e instanceof Error ? e.message : '暂时无法加载豆单'
    needsLogin.value = e instanceof MiniRequestError && e.statusCode === 401
  } finally {
    if (request === sequence) loading.value = false
  }
}
onLoad((query) => {
  entry.value = String(query?.entry || '')
})
onShow(() => {
  void load()
  void refreshMiniappShareMenu()
})
onHide(() => {
  sequence++
  publication.value = null
})
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
    <text v-if="loading" class="state">正在加载价格表…</text>
    <view v-if="error" class="state">
      <text>{{ error }}</text>
      <button v-if="needsLogin" @tap="login">登录后查看</button>
      <button v-else @tap="load">重新加载</button>
    </view>
    <PriceListContent v-if="publication" :publication="publication" :search-scope="priceListSearchScope(configuredMiniappEnvironment().environment, (session.miniUserID || -session.employeeID), session.currentCustomerID, entry)"/>
    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter :revealed="pullUpBrandRevealed" />
    </view>
  </view>
</template>
<style scoped>
.page{min-height:100vh;background:#f5f2ec;padding:24rpx;box-sizing:border-box}.state{display:flex;flex-direction:column;gap:24rpx;text-align:center;padding:80rpx 30rpx}
</style>
