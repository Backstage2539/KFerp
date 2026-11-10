<script setup lang="ts">
import { computed, ref } from 'vue'
import { useSessionStore } from '../stores/session'
import { exitPreview } from '../api/customerPreview'

import { customerMainTabs, type CustomerTabKey as MainTabKey } from '../utils/beanCenter'
defineProps<{current:MainTabKey}>()
const session = useSessionStore()
const tabs = computed(()=>customerMainTabs(session.currentCustomerID>0, session.capabilities, session.accountType))
const leaving = ref(false)

async function returnToEmployee() {
  if (!session.isCustomerPreview || leaving.value) return
  const token = session.token
  leaving.value = true
  try { await exitPreview(token) } catch { /* Expired previews can still return safely. */ }
  if (session.token === token && session.isCustomerPreview) session.endCustomerPreview()
  leaving.value = false
  uni.reLaunch({ url: '/pages/home/home' })
}

function openTab(tab: { key: MainTabKey; url: string }) {
  uni.reLaunch({ url: tab.url })
}
</script>

<template>
  <view v-if="session.isCustomerPreview" class="preview-banner"><text>客户只读预览 · {{ session.currentCustomerName }}</text><button :disabled="leaving" @tap="returnToEmployee">返回员工工作台</button></view>
  <view v-if="tabs.length" class="main-tabbar">
    <view
      v-for="tab in tabs"
      :key="tab.key"
      :class="['tab-item', { active: tab.key === current }]"
      hover-class="tab-pressed"
      @tap="openTab(tab)">
      <text class="tab-label">{{ tab.label }}</text>
    </view>
  </view>
</template>

<style scoped>
.preview-banner { position: fixed; z-index: 1000; bottom: 124rpx; left: 0; right: 0; display: flex; align-items: center; justify-content: space-between; gap: 12rpx; padding: 12rpx 20rpx; background: #e8f3ec; color: #194f37; font-size: 23rpx; }
.preview-banner text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.preview-banner button { flex-shrink: 0; margin: 0; font-size: 22rpx; color: #fff; background: #225941; }
.main-tabbar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 999;
  display: flex;
  gap: 10rpx;
  padding: 14rpx 18rpx 22rpx;
  background: rgba(255, 255, 255, .96);
  border-top: 1rpx solid #e6ddd0;
  box-shadow: 0 -8rpx 22rpx rgba(23, 23, 23, .08);
  box-sizing: border-box;
}

.tab-item {
  min-height: 86rpx;
  flex: 1 1 0;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 8rpx;
  border: 1rpx solid #e7dac7;
  border-radius: 8rpx;
  background: #fffaf2;
  box-sizing: border-box;
}

.tab-item.active {
  border-color: #2b2118;
  background: #2b2118;
}

.tab-label {
  color: #44382f;
  font-size: 24rpx;
  font-weight: 900;
  line-height: 1.2;
  text-align: center;
  white-space: nowrap;
}

.tab-item.active .tab-label {
  color: #fff8eb;
}

.tab-pressed {
  opacity: .82;
}
</style>
