<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { fetchMe } from '../../api/customerPortal'
import { useSessionStore } from '../../stores/session'
import { visibleHomeEntries } from '../../utils/capabilities'
import { miniappThemeClass, miniappThemeMeta } from '../../utils/themes'

const session = useSessionStore()
const loading = ref(false)
const errorMessage = ref('')

const entries = computed(() => visibleHomeEntries(session.capabilities))
const customerName = computed(() => session.currentCustomerName || '客户中心')
const themeClass = computed(() => miniappThemeClass(session.themeKey))
const themeMeta = computed(() => miniappThemeMeta(session.themeKey))

function openEntry(url: string) {
  uni.navigateTo({ url })
}

function openProfile() {
  uni.navigateTo({ url: '/pages/profile/profile' })
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
        <button class="profile-link" size="mini" @tap="openProfile">个人中心</button>
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

.profile-link {
  min-height: 56rpx;
  padding: 0 22rpx;
  border: 1rpx solid rgba(255, 248, 235, .56);
  border-radius: 8rpx;
  background: rgba(255, 255, 255, .12);
  color: #fff8eb;
  font-size: 24rpx;
  font-weight: 900;
  line-height: 56rpx;
  margin: 0;
}

.theme-clean-ops .profile-link {
  border-color: #cddbd4;
  background: #eef6f2;
  color: #28624a;
}

.theme-premium-partner .profile-link {
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
