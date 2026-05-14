<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { fetchMe, switchCurrentCustomer } from '../../api/customerPortal'
import { useSessionStore } from '../../stores/session'
import { visibleHomeEntries } from '../../utils/capabilities'
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

const entries = computed(() => visibleHomeEntries(session.capabilities))
const customerName = computed(() => session.currentCustomerName || '客户中心')
const themeClass = computed(() => miniappThemeClass(session.themeKey))
const themeMeta = computed(() => miniappThemeMeta(session.themeKey))
const canSwitchCustomer = computed(() => shouldShowCustomerSwitcher(session.bindings))
const customerPickerLabels = computed(() => buildCustomerPickerLabels(session.bindings, session.currentCustomerID))
const customerPickerIndex = computed(() => selectedCustomerPickerIndex(session.bindings, session.currentCustomerID))

function openEntry(url: string) {
  uni.navigateTo({ url })
}

function logout() {
  session.clearSession()
  uni.redirectTo({ url: '/pages/login/login' })
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
    uni.redirectTo({ url: customerEntryRoute(response) })
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '切换客户失败'
  } finally {
    switching.value = false
  }
}

async function loadContext() {
  if (!session.token) {
    uni.redirectTo({ url: '/pages/login/login' })
    return
  }

  loading.value = true
  errorMessage.value = ''

  try {
    const response = await fetchMe(session.token)
    session.applyContext(response)
    if (session.entryMode === 'mall' && session.capabilities.some((item) => item.code === 'mall' && item.enabled)) {
      uni.redirectTo({ url: '/pages/mall/mall' })
      return
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '客户信息加载失败'
    session.clearSession()
    uni.redirectTo({ url: '/pages/login/login' })
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
    <view class="header">
      <text class="eyebrow">{{ themeMeta.eyebrow }}</text>
      <text class="title">{{ customerName }}</text>
      <text class="subtitle">{{ themeMeta.subtitle }}</text>
      <view class="account-actions">
        <picker v-if="canSwitchCustomer" mode="selector" :range="customerPickerLabels" :value="customerPickerIndex" @change="handleCustomerSwitch">
          <view class="customer-switch">{{ switching ? '切换中...' : customerPickerLabels[customerPickerIndex] || '切换客户' }}</view>
        </picker>
        <button class="logout-link" size="mini" @tap="logout">退出登录</button>
      </view>
    </view>

    <view v-if="loading" class="state">
      <text>加载中...</text>
    </view>

    <view v-else-if="errorMessage" class="state error">
      <text>{{ errorMessage }}</text>
    </view>

    <view v-else-if="entries.length" class="grid">
      <view v-for="entry in entries" :key="entry.key" class="entry" hover-class="entry-pressed" @tap="openEntry(entry.url)">
        <text class="entry-label">{{ entry.label }}</text>
      </view>
    </view>

    <view v-else class="state">
      <text>暂无可用服务</text>
    </view>
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 32rpx;
  background: #f7f2ea;
  box-sizing: border-box;
}

.page.theme-coffee-factory {
  background: #f7f2ea;
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

.account-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 14rpx;
  margin-top: 8rpx;
}

.customer-switch,
.logout-link {
  min-height: 56rpx;
  padding: 0 22rpx;
  border: 1rpx solid rgba(255, 248, 235, .56);
  border-radius: 8rpx;
  background: rgba(255, 255, 255, .12);
  color: #fff8eb;
  font-size: 24rpx;
  font-weight: 900;
  line-height: 56rpx;
}

.logout-link {
  margin: 0;
}

.theme-clean-ops .customer-switch,
.theme-clean-ops .logout-link {
  border-color: #cddbd4;
  background: #eef6f2;
  color: #28624a;
}

.theme-premium-partner .customer-switch,
.theme-premium-partner .logout-link {
  border-color: rgba(255, 248, 235, .5);
  background: rgba(255, 248, 235, .14);
}

.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20rpx;
}

.entry {
  min-height: 168rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24rpx;
  background: #fffaf2;
  border: 1rpx solid #ead9bd;
  border-radius: 16rpx;
  box-sizing: border-box;
}

.theme-clean-ops .entry {
  background: #ffffff;
  border-color: #dde7e1;
}

.theme-premium-partner .entry {
  background: #fffdf8;
  border-color: #eadab7;
}

.entry-pressed {
  transform: scale(.99);
  opacity: .86;
}

.entry-label {
  color: #171717;
  font-size: 30rpx;
  font-weight: 800;
  line-height: 1.35;
  text-align: center;
}

.state {
  min-height: 180rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #666666;
  font-size: 28rpx;
}

.error {
  color: #b42318;
}
</style>
