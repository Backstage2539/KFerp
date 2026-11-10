<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow, onShareAppMessage, onShareTimeline } from '@dcloudio/uni-app'
import { beginPreview, exitPreview, fetchPreviewCustomers, type PreviewCustomer } from '../../api/customerPreview'
import { fetchMe } from '../../api/customerPortal'
import { isAuthenticationExpiredRequestError } from '../../api/client'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { defaultMiniappShare, defaultMiniappTimelineShare, refreshMiniappShareMenu } from '../../utils/miniappShare'
import { useSessionStore } from '../../stores/session'

const session = useSessionStore()
const { pullUpBrandRevealed, handlePullUpBrandTouchStart, handlePullUpBrandTouchMove, handlePullUpBrandTouchEnd, handlePullUpBrandTouchCancel } = usePullUpBrandGesture()
const query = ref('')
const rows = ref<PreviewCustomer[]>([])
const page = ref(1)
const hasNext = ref(false)
const loading = ref(false)
const opening = ref(false)
const error = ref('')
let requestVersion = 0
const isAdmin = computed(() => session.accountType === 'employee' && session.roles.includes('admin'))

async function load(append = false) {
  if (!isAdmin.value || !session.token) return
  const version = ++requestVersion
  const token = session.token
  const nextPage = append ? page.value + 1 : 1
  loading.value = true
  error.value = ''
  try {
    const result = await fetchPreviewCustomers(token, query.value, nextPage)
    if (version !== requestVersion || session.token !== token) return
    const existing = append ? rows.value : []
    const known = new Set(existing.map(item => item.id))
    rows.value = [...existing, ...(result.rows || []).filter(item => !known.has(item.id))]
    page.value = nextPage
    hasNext.value = !!result.has_next
  } catch (cause) {
    if (version !== requestVersion || session.token !== token) return
    if (isAuthenticationExpiredRequestError(cause)) {
      session.clearSession()
      uni.reLaunch({ url: '/pages/login/login' })
      return
    }
    error.value = cause instanceof Error ? cause.message : '客户加载失败'
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

async function openCustomer(customer: PreviewCustomer) {
  if (!isAdmin.value || opening.value || !customer.id) return
  const employeeToken = session.token
  opening.value = true
  error.value = ''
  let previewToken = ''
  try {
    const result = await beginPreview(employeeToken, customer.id)
    previewToken = result.token
    const context = await fetchMe(previewToken)
    if (session.token !== employeeToken || !isAdmin.value) {
      void exitPreview(previewToken).catch(() => {})
      return
    }
    if (context.current_customer_id !== customer.id) throw new Error('客户预览身份不匹配，请重试')
    session.beginCustomerPreview(previewToken, context)
    requestVersion++
    uni.reLaunch({ url: '/pages/home/home' })
  } catch (cause) {
    if (previewToken) void exitPreview(previewToken).catch(() => {})
    error.value = cause instanceof Error ? cause.message : '无法打开客户预览'
  } finally {
    opening.value = false
  }
}

async function restoreAndLoad() {
  if (!session.token) { uni.reLaunch({ url: '/pages/login/login' }); return }
  if (!session.accountType) {
    const token = session.token
    loading.value = true
    try {
      const context = await fetchMe(token)
      if (session.token !== token) return
      session.applyContext(context)
    } catch (cause) {
      if (session.token !== token) return
      if (isAuthenticationExpiredRequestError(cause)) { session.clearSession(); uni.reLaunch({ url: '/pages/login/login' }); return }
      error.value = cause instanceof Error ? cause.message : '账号信息加载失败'
      return
    } finally { loading.value = false }
  }
  await load()
}
onShow(() => { void restoreAndLoad() })
onShow(() => { void refreshMiniappShareMenu() })
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
</script>

<template>
  <view class="page pull-up-brand-page" @touchstart="handlePullUpBrandTouchStart" @touchmove="handlePullUpBrandTouchMove" @touchend="handlePullUpBrandTouchEnd" @touchcancel="handlePullUpBrandTouchCancel">
    <EnvironmentBadge />
    <text class="title">客户只读预览</text>
    <text class="subtitle">选择客户后，可查看该客户的小程序。预览期间不能修改业务数据。</text>
    <view v-if="error && !isAdmin" class="state error">{{ error }} <button @tap="restoreAndLoad">重试</button></view>
    <view v-else-if="loading && !isAdmin" class="state">正在加载账号...</view>
    <view v-else-if="!isAdmin" class="state">仅管理员可以使用此功能</view>
    <template v-else>
      <view class="search-row"><input v-model="query" placeholder="搜索客户名称" confirm-type="search" @confirm="load(false)" /><button @tap="load(false)">搜索</button></view>
      <view v-if="error" class="state error">{{ error }}</view>
      <view v-if="loading && !rows.length" class="state">正在加载客户...</view>
      <view v-else-if="!rows.length" class="state">暂无客户</view>
      <view v-for="customer in rows" :key="customer.id" class="customer-row">
        <text>{{ customer.name }}</text>
        <button :disabled="opening" @tap="openCustomer(customer)">查看</button>
      </view>
      <button v-if="hasNext" :loading="loading" :disabled="loading || opening" @tap="load(true)">加载更多</button>
    </template>
    <view class="pull-up-brand-footer-anchor"><PullUpBrandFooter :revealed="pullUpBrandRevealed" /></view>
  </view>
</template>

<style scoped>
.page { min-height: 100vh; padding: 32rpx; background: #f5f7f6; box-sizing: border-box; }
.title { display: block; font-size: 40rpx; font-weight: 700; color: #173b2d; }
.subtitle { display: block; margin: 16rpx 0 28rpx; color: #66756c; line-height: 1.5; }
.search-row, .customer-row { display: flex; align-items: center; gap: 20rpx; padding: 20rpx; margin-bottom: 16rpx; background: white; border-radius: 16rpx; }
.search-row input, .customer-row text { flex: 1; min-width: 0; }
.search-row button, .customer-row button { margin: 0; font-size: 26rpx; color: #fff; background: #225941; }
.state { padding: 24rpx 0; color: #66756c; }
.error { color: #a23526; }
</style>
