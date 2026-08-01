<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { fetchMe, switchCurrentCustomer } from '../../api/customerPortal'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import MainTabBar from '../../components/MainTabBar.vue'
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
const loading = ref(false)
const switching = ref(false)
const errorMessage = ref('')

const customerName = computed(() => session.currentCustomerName || '客户中心')
const themeClass = computed(() => miniappThemeClass(session.themeKey))
const themeMeta = computed(() => miniappThemeMeta(session.themeKey))
const canSwitchCustomer = computed(() => shouldShowCustomerSwitcher(session.bindings))
const customerPickerLabels = computed(() => buildCustomerPickerLabels(session.bindings, session.currentCustomerID))
const customerPickerIndex = computed(() => selectedCustomerPickerIndex(session.bindings, session.currentCustomerID))

function clearAndLogin() {
  session.clearSession()
  uni.reLaunch({ url: '/pages/login/login' })
}

function openCustomerProducts() {
  uni.navigateTo({ url: '/pages/customer-products/customer-products' })
}

function openFactoryProducts() {
  uni.navigateTo({ url: '/pages/factory-products/factory-products' })
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

async function loadContext() {
  if (!session.token) {
    uni.reLaunch({ url: '/pages/login/login' })
    return
  }

  loading.value = true
  errorMessage.value = ''
  try {
    const response = await fetchMe(session.token)
    session.applyContext(response)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '客户信息加载失败'
    session.clearSession()
    uni.reLaunch({ url: '/pages/login/login' })
  } finally {
    loading.value = false
  }
}

onShow(() => {
  void loadContext()
})
</script>

<template>
  <view class="page" :class="themeClass">
    <EnvironmentBadge />
    <view class="header">
      <text class="eyebrow">{{ themeMeta.eyebrow }}</text>
      <text class="title">个人中心</text>
      <text class="subtitle">{{ customerName }}</text>
    </view>

    <view v-if="loading" class="state">
      <text>加载中...</text>
    </view>

    <view v-else class="panel">
      <view class="info-row">
        <text class="label">当前客户</text>
        <text class="value">{{ customerName }}</text>
      </view>

      <picker v-if="canSwitchCustomer" mode="selector" :range="customerPickerLabels" :value="customerPickerIndex" @change="handleCustomerSwitch">
        <view class="customer-switch">{{ switching ? '切换中...' : customerPickerLabels[customerPickerIndex] || '切换客户' }}</view>
      </picker>

      <text v-if="errorMessage" class="error">{{ errorMessage }}</text>

      <button class="secondary-button" @tap="openFactoryProducts">工厂商品表</button>
      <button class="secondary-button" @tap="openCustomerProducts">我的商品</button>
      <button class="secondary-button" @tap="clearAndLogin">切换用户</button>
      <button class="danger-button" @tap="clearAndLogin">退出登录</button>
    </view>

    <MainTabBar current="mine" />
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
