<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow, onShareAppMessage, onShareTimeline } from '@dcloudio/uni-app'
import {
  defaultMiniappShare,
  defaultMiniappTimelineShare,
  refreshMiniappShareMenu,
} from '../../utils/miniappShare'
import {
  fetchEmployeeShareSettings,
  fetchDirectShipRequests,
  fetchMe,
  saveEmployeeShareSettings,
  saveEmployeeShareScope,
  switchCurrentCustomer,
} from '../../api/customerPortal'
import type { EmployeeShareScope, MeResponse } from '../../api/customerPortal'
import { isAuthenticationExpiredRequestError } from '../../api/client'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import MainTabBar from '../../components/MainTabBar.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { useSessionStore } from '../../stores/session'
import {
  customerEntryRoute,
  customerPickerIndex as selectedCustomerPickerIndex,
  customerPickerLabels as buildCustomerPickerLabels,
  selectedCustomerID,
  shouldShowCustomerSwitcher,
} from '../../utils/customerSwitch'
import { miniappThemeClass, miniappThemeMeta } from '../../utils/themes'

const session = useSessionStore()
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
const loading = ref(false)
const switching = ref(false)
const errorMessage = ref('')
const currentContext = ref<MeResponse | null>(null)
const recentRecipients = ref<Array<{ key: string; name: string; phone: string; address: string }>>([])
const recipientLoading = ref(false)
const recipientError = ref('')
const shareSettingLoading = ref(false)
const shareSettingSaving = ref(false)
const shareSettingLoaded = ref(false)
const shareSettingError = ref('')
const imageNeedShowEntrance = ref(false)
const savedImageNeedShowEntrance = ref(false)
const shareScope = ref<EmployeeShareScope>('employee')
const savedShareScope = ref<EmployeeShareScope>('employee')
const shareScopeOptions: Array<{ value: EmployeeShareScope; label: string; hint: string }> = [
  { value: 'employee', label: '员工可以分享', hint: '所有员工可分享，客户和访客不可分享' },
  { value: 'all', label: '所有人可以分享', hint: '员工、客户和未登录访客均可分享' },
  { value: 'admin', label: '管理员可以分享', hint: '仅管理员显示小程序转发入口' },
]

const isEmployee = computed(() => session.accountType === 'employee')
function hasCapability(code: string): boolean {
  return session.capabilities.some((item) => item.code === code && item.enabled)
}
const canOpenFactoryProducts = computed(() => (
  !isEmployee.value
  && hasCapability('product_order')
  && hasCapability('bean_list')
))
const canManageShareSettings = computed(() => (
  session.accountType === 'employee'
  && session.roles.includes('admin')
  && session.permissions.includes('settings.write')
))
const accountName = computed(() => isEmployee.value ? (session.employeeName || '员工') : (session.currentCustomerName || '客户中心'))
const themeClass = computed(() => miniappThemeClass(session.themeKey))
const themeMeta = computed(() => miniappThemeMeta(session.themeKey))
const canSwitchCustomer = computed(() => shouldShowCustomerSwitcher(session.bindings))
const customerPickerLabels = computed(() => buildCustomerPickerLabels(session.bindings, session.currentCustomerID))
const customerPickerIndex = computed(() => selectedCustomerPickerIndex(session.bindings, session.currentCustomerID))
const serviceDescriptions = computed(() => {
  const items: string[] = []
  if (hasCapability('direct_ship')) items.push('一件代发按 ERP 指定价格表下单，缺货订单自动进入现有生产流程。')
  if (hasCapability('processing')) items.push('生产工单可提交代加工需求并查看排产、完工与入库进度。')
  if (hasCapability('inventory_custody')) items.push('我的库存按客户货权查看成品、生豆、包材和半成品。')
  if (hasCapability('shipping_query')) items.push('发货中心可查看订单包裹、运单号和已获取的物流轨迹。')
  if (hasCapability('settlement')) items.push('费用中心可下载账单、确认对账或对具体费用提出异议。')
  return items
})

function clearAndLogin() {
  session.clearSession()
  uni.reLaunch({ url: '/pages/login/login' })
}

function redirectExpiredShareSettingsSession(error: unknown): boolean {
  if (!isAuthenticationExpiredRequestError(error)) return false
  clearAndLogin()
  return true
}

function normalizeShareScope(value: unknown): EmployeeShareScope {
  if (value === 'admin' || value === 'all') return value
  return 'employee'
}

function openCustomerProducts() {
  uni.navigateTo({ url: '/pages/customer-products/customer-products' })
}

function openFactoryProducts() {
  uni.navigateTo({ url: '/pages/factory-products/factory-products' })
}

async function loadRecentRecipients() {
  recentRecipients.value = []
  recipientError.value = ''
  if (isEmployee.value || !session.token || !hasCapability('direct_ship')) return
  recipientLoading.value = true
  try {
    const response = await fetchDirectShipRequests(session.token, { page: 1, limit: 20 })
    const seen = new Set<string>()
    recentRecipients.value = (response.rows || []).flatMap((row) => {
      const address = [row.province, row.city, row.district, row.detail_address].filter(Boolean).join('')
      const key = `${row.recipient_name}|${row.recipient_phone}|${address}`
      if (!row.recipient_name || seen.has(key)) return []
      seen.add(key)
      return [{ key, name: row.recipient_name, phone: row.recipient_phone, address }]
    }).slice(0, 5)
  } catch (error) {
    if (redirectExpiredShareSettingsSession(error)) return
    recipientError.value = error instanceof Error ? error.message : '常用收件人加载失败'
  } finally {
    recipientLoading.value = false
  }
}

async function handleCustomerSwitch(event: { detail?: { value?: number | string } }) {
  if (switching.value || !session.token) return
  const customerID = selectedCustomerID(session.bindings, Number(event.detail?.value ?? -1))
  if (!customerID || customerID === session.currentCustomerID) return

  switching.value = true
  errorMessage.value = ''
  try {
    const response = await switchCurrentCustomer(session.token, customerID)
    session.applyContext(response)
    uni.reLaunch({ url: customerEntryRoute(response) })
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '切换客户失败'
  } finally {
    switching.value = false
  }
}

async function loadShareSettings() {
  if (!canManageShareSettings.value || !session.token) return
  shareSettingLoading.value = true
  shareSettingLoaded.value = false
  shareSettingError.value = ''
  try {
    const response = await fetchEmployeeShareSettings(session.token)
    const value = response.settings?.image_need_show_entrance === true
    imageNeedShowEntrance.value = value
    savedImageNeedShowEntrance.value = value
    const loadedScope = normalizeShareScope(response.settings?.share_scope)
    shareScope.value = loadedScope
    savedShareScope.value = loadedScope
    shareSettingLoaded.value = true
  } catch (error) {
    if (redirectExpiredShareSettingsSession(error)) return
    shareSettingError.value = error instanceof Error ? error.message : '分享设置加载失败'
  } finally {
    shareSettingLoading.value = false
  }
}

async function handleShareScopeChange(event: Event) {
  if (!canManageShareSettings.value || !shareSettingLoaded.value || shareSettingSaving.value) return
  const detail = (event as unknown as { detail?: { value?: string } }).detail
  const nextScope = normalizeShareScope(detail?.value)
  if (nextScope === savedShareScope.value) return
  shareScope.value = nextScope
  shareSettingSaving.value = true
  shareSettingError.value = ''
  try {
    const response = await saveEmployeeShareScope(session.token, nextScope)
    const savedScope = normalizeShareScope(response.settings?.share_scope)
    shareScope.value = savedScope
    savedShareScope.value = savedScope
    await refreshMiniappShareMenu()
    uni.showToast({ title: '分享范围已保存', icon: 'success' })
  } catch (error) {
    shareScope.value = savedShareScope.value
    if (redirectExpiredShareSettingsSession(error)) return
    shareSettingError.value = error instanceof Error ? error.message : '分享范围保存失败'
    uni.showToast({ title: '保存失败，已恢复原设置', icon: 'none' })
  } finally {
    shareSettingSaving.value = false
  }
}

async function handleShareSettingChange(event: Event) {
  if (!canManageShareSettings.value || !shareSettingLoaded.value || shareSettingSaving.value) return
  const detail = (event as unknown as { detail?: { value?: boolean } }).detail
  const nextValue = detail?.value === true
  imageNeedShowEntrance.value = nextValue
  shareSettingSaving.value = true
  shareSettingError.value = ''
  try {
    const response = await saveEmployeeShareSettings(session.token, nextValue)
    const savedValue = response.settings?.image_need_show_entrance === true
    imageNeedShowEntrance.value = savedValue
    savedImageNeedShowEntrance.value = savedValue
    uni.showToast({ title: '分享设置已保存', icon: 'success' })
  } catch (error) {
    imageNeedShowEntrance.value = savedImageNeedShowEntrance.value
    if (redirectExpiredShareSettingsSession(error)) return
    shareSettingError.value = error instanceof Error ? error.message : '分享设置保存失败'
    uni.showToast({ title: '保存失败，已恢复原设置', icon: 'none' })
  } finally {
    shareSettingSaving.value = false
  }
}

async function loadContext() {
  if (!session.token) {
    uni.reLaunch({ url: '/pages/login/login' })
    return
  }

  loading.value = true
  errorMessage.value = ''
  try {
    const response = await fetchMe(session.token)
    currentContext.value = response
    session.applyContext(response)
    if (canManageShareSettings.value) await loadShareSettings()
    await loadRecentRecipients()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '账号信息加载失败'
    session.clearSession()
    uni.reLaunch({ url: '/pages/login/login' })
  } finally {
    loading.value = false
  }
}

onShow(() => {
  void loadContext()
})

onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
onShow(() => { void refreshMiniappShareMenu() })
</script>

<template>
  <view
    class="page pull-up-brand-page"
    :class="[themeClass, { 'pull-up-brand-page-with-tabbar': !isEmployee }]"
    @touchstart="handlePullUpBrandTouchStart"
    @touchmove="handlePullUpBrandTouchMove"
    @touchend="handlePullUpBrandTouchEnd"
    @touchcancel="handlePullUpBrandTouchCancel"
  >
    <EnvironmentBadge />
    <view class="header">
      <text class="eyebrow">{{ themeMeta.eyebrow }}</text>
      <text class="title">个人中心</text>
      <text class="subtitle">{{ accountName }}</text>
    </view>

    <view v-if="loading" class="state">
      <text>加载中...</text>
    </view>

    <view v-else class="panel">
      <view class="info-row">
        <text class="label">{{ isEmployee ? '当前员工' : '当前客户' }}</text>
        <text class="value">{{ accountName }}</text>
      </view>

      <view v-if="!isEmployee" class="profile-card">
        <text class="card-title">客户资料</text>
        <view class="detail-row">
          <text class="label">联系人</text>
          <text class="value">{{ currentContext?.current_customer_contact || '未配置' }}</text>
        </view>
        <view class="detail-row">
          <text class="label">联系电话</text>
          <text class="value">{{ currentContext?.current_customer_phone || '未配置' }}</text>
        </view>
        <view class="detail-row">
          <text class="label">联系地址</text>
          <text class="value value-wrap">{{ currentContext?.current_customer_address || '未配置' }}</text>
        </view>
      </view>

      <view v-if="!isEmployee" class="profile-card">
        <text class="card-title">业务联系人</text>
        <text class="card-copy">
          {{ currentContext?.business_contact_name || 'ERP 暂未配置业务联系人' }}
          <template v-if="currentContext?.business_contact_phone"> · {{ currentContext.business_contact_phone }}</template>
        </text>
      </view>

      <view v-if="!isEmployee && hasCapability('direct_ship')" class="profile-card">
        <text class="card-title">常用收件人</text>
        <text v-if="recipientLoading" class="card-copy">正在读取最近收件人...</text>
        <text v-else-if="recipientError" class="error">{{ recipientError }}</text>
        <view v-else-if="recentRecipients.length" class="recipient-list">
          <view v-for="recipient in recentRecipients" :key="recipient.key" class="recipient-item">
            <text class="recipient-title">{{ recipient.name }} · {{ recipient.phone }}</text>
            <text class="card-copy">{{ recipient.address }}</text>
          </view>
        </view>
        <text v-else class="card-copy">完成一件代发订单后，最近使用的收件人会显示在这里。</text>
      </view>

      <view v-if="!isEmployee && serviceDescriptions.length" class="profile-card">
        <text class="card-title">服务说明</text>
        <text v-for="item in serviceDescriptions" :key="item" class="service-copy">{{ item }}</text>
      </view>

      <view v-if="canManageShareSettings" class="settings-card">
        <view class="setting-copy">
          <text class="setting-title">小程序分享范围</text>
          <text class="setting-hint">控制微信右上角“转发”和“分享到朋友圈”的可见范围。</text>
        </view>
        <radio-group class="scope-options" @change="handleShareScopeChange">
          <label v-for="option in shareScopeOptions" :key="option.value" class="scope-option">
            <radio
              color="#28624a"
              :value="option.value"
              :checked="shareScope === option.value"
              :disabled="shareSettingLoading || shareSettingSaving || !shareSettingLoaded"
            />
            <view class="setting-copy">
              <text class="scope-label">{{ option.label }}</text>
              <text class="setting-hint">{{ option.hint }}</text>
            </view>
          </label>
        </radio-group>

        <view class="setting-divider" />
        <view class="setting-row">
          <view class="setting-copy">
            <text class="setting-title">分享图片时携带小程序入口</text>
            <text class="setting-hint">全系统开关，对所有员工之后分享的销售单、发货单图片生效。</text>
          </view>
          <switch
            color="#28624a"
            :checked="imageNeedShowEntrance"
            :disabled="shareSettingLoading || shareSettingSaving || !shareSettingLoaded"
            @change="handleShareSettingChange"
          />
        </view>
        <text v-if="shareSettingLoading" class="setting-status">正在读取系统设置...</text>
        <text v-else-if="shareSettingSaving" class="setting-status">正在保存...</text>
        <view v-else-if="shareSettingError" class="setting-error-row">
          <text class="error">{{ shareSettingError }}</text>
          <button class="retry-button" @tap="loadShareSettings">重试</button>
        </view>
        <text v-else class="setting-status">当前：{{ imageNeedShowEntrance ? '携带入口' : '不携带入口' }}</text>
      </view>

      <picker v-if="!isEmployee && canSwitchCustomer" mode="selector" :range="customerPickerLabels" :value="customerPickerIndex" @change="handleCustomerSwitch">
        <view class="customer-switch">{{ switching ? '切换中...' : customerPickerLabels[customerPickerIndex] || '切换客户' }}</view>
      </picker>

      <text v-if="errorMessage" class="error">{{ errorMessage }}</text>

      <button v-if="canOpenFactoryProducts" class="secondary-button" @tap="openFactoryProducts">工厂商品表</button>
      <button v-if="canOpenFactoryProducts" class="secondary-button" @tap="openCustomerProducts">我的商品</button>
      <button class="secondary-button" @tap="clearAndLogin">切换用户</button>
      <button class="danger-button" @tap="clearAndLogin">退出登录</button>
    </view>

    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter :with-fixed-tabbar="!isEmployee" :revealed="pullUpBrandRevealed" />
    </view>
    <MainTabBar v-if="!isEmployee" current="mine" />
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 32rpx 32rpx 160rpx;
  background: #f7f2ea;
  box-sizing: border-box;
}

.page.theme-clean-ops {
  background: #f5f7f6;
}

.page.theme-premium-partner {
  background: #fbf7ef;
}

.header {
  display: flex;
  flex-direction: column;
  gap: 14rpx;
  padding: 30rpx 28rpx 34rpx;
  margin-bottom: 24rpx;
  border-radius: 28rpx;
  background: linear-gradient(135deg, #2b2118 0%, #6b4b2b 100%);
}

.theme-clean-ops .header {
  background: #ffffff;
  border: 1rpx solid #dfe7e2;
}

.theme-premium-partner .header {
  background: linear-gradient(135deg, #111111 0%, #513018 55%, #b88a46 100%);
}

.eyebrow {
  color: rgba(255, 248, 235, 0.78);
  font-size: 24rpx;
  font-weight: 900;
}

.theme-clean-ops .eyebrow {
  color: #28624a;
}

.title {
  color: #fff8eb;
  font-size: 42rpx;
  font-weight: 900;
  line-height: 1.18;
}

.theme-clean-ops .title {
  color: #14201a;
}

.subtitle {
  color: rgba(255, 248, 235, 0.82);
  font-size: 26rpx;
  line-height: 1.55;
}

.theme-clean-ops .subtitle {
  color: #66756c;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 22rpx;
}

.info-row {
  display: flex;
  justify-content: space-between;
  gap: 20rpx;
  padding: 28rpx;
  border: 1rpx solid #ead9bd;
  border-radius: 16rpx;
  background: #fffaf2;
}

.settings-card {
  padding: 28rpx;
  border: 1rpx solid #d8e4dd;
  border-radius: 16rpx;
  background: #ffffff;
}

.profile-card {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
  padding: 28rpx;
  border: 1rpx solid #ead9bd;
  border-radius: 16rpx;
  background: #fffdf8;
}

.card-title {
  color: #2b2118;
  font-size: 29rpx;
  font-weight: 900;
}

.card-copy,
.service-copy {
  color: #6f665d;
  font-size: 24rpx;
  line-height: 1.6;
}

.service-copy {
  display: block;
  padding-left: 22rpx;
  position: relative;
}

.service-copy::before {
  position: absolute;
  left: 0;
  content: '•';
}

.detail-row {
  display: flex;
  justify-content: space-between;
  gap: 24rpx;
}

.value-wrap {
  max-width: 68%;
  line-height: 1.55;
}

.recipient-list {
  display: flex;
  flex-direction: column;
  gap: 16rpx;
}

.recipient-item {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  padding-top: 16rpx;
  border-top: 1rpx solid #eee2cf;
}

.recipient-title {
  color: #2b2118;
  font-size: 25rpx;
  font-weight: 800;
}

.scope-options {
  display: flex;
  flex-direction: column;
  gap: 16rpx;
  margin-top: 22rpx;
}

.scope-option {
  display: flex;
  align-items: center;
  gap: 18rpx;
  padding: 20rpx;
  border: 1rpx solid #e3ebe6;
  border-radius: 12rpx;
  background: #f8fbf9;
}

.scope-label {
  color: #172c22;
  font-size: 26rpx;
  font-weight: 800;
}

.setting-divider {
  height: 1rpx;
  margin: 28rpx 0;
  background: #e3ebe6;
}

.setting-row,
.setting-error-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24rpx;
}

.setting-copy {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 10rpx;
}

.setting-title {
  color: #172c22;
  font-size: 28rpx;
  font-weight: 900;
}

.setting-hint,
.setting-status {
  color: #66756c;
  font-size: 23rpx;
  line-height: 1.55;
}

.setting-status {
  display: block;
  margin-top: 14rpx;
}

.retry-button {
  flex: 0 0 auto;
  min-height: 58rpx;
  margin: 0;
  padding: 0 22rpx;
  border: 1rpx solid #cddbd4;
  background: #eef6f2;
  color: #28624a;
  font-size: 23rpx;
  line-height: 58rpx;
}

.theme-clean-ops .info-row {
  border-color: #dde7e1;
  background: #ffffff;
}

.theme-premium-partner .info-row {
  border-color: #eadab7;
  background: #fffdf8;
}

.label {
  color: #6f665d;
  font-size: 26rpx;
}

.value {
  color: #171717;
  font-size: 28rpx;
  font-weight: 900;
  text-align: right;
}

.customer-switch,
.primary-button,
.secondary-button,
.danger-button {
  width: 100%;
  min-height: 82rpx;
  border-radius: 10rpx;
  font-size: 28rpx;
  font-weight: 900;
  line-height: 82rpx;
  box-sizing: border-box;
}

.customer-switch {
  padding: 0 26rpx;
  border: 1rpx solid #ead9bd;
  background: #fffaf2;
  color: #2b2118;
}

.primary-button {
  background: #2b2118;
  color: #ffffff;
}

.secondary-button {
  border: 1rpx solid #ead9bd;
  background: #fffaf2;
  color: #2b2118;
}

.danger-button {
  border: 1rpx solid #f3b0a6;
  background: #fff4f2;
  color: #b42318;
}

.theme-clean-ops .primary-button {
  background: #173b2e;
}

.theme-clean-ops .secondary-button,
.theme-clean-ops .customer-switch {
  border-color: #cddbd4;
  background: #eef6f2;
  color: #28624a;
}

.theme-premium-partner .primary-button {
  background: #17120d;
  color: #f8ddb0;
}

.error {
  color: #b42318;
  font-size: 26rpx;
  line-height: 1.5;
}

.state {
  padding: 80rpx 0;
  color: #6f665d;
  font-size: 28rpx;
  text-align: center;
}
</style>
