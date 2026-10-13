<script setup lang="ts">
import { computed } from 'vue'
import { useSessionStore } from '../stores/session'

import { customerMainTabs, type CustomerTabKey as MainTabKey } from '../utils/beanCenter'
const props = defineProps<{current:MainTabKey}>()
const session = useSessionStore()
const tabs = computed(()=>customerMainTabs(session.currentCustomerID>0))

function openTab(tab: { key: MainTabKey; url: string }) {
  if (tab.key === props.current) return
  uni.reLaunch({ url: tab.url })
}
</script>

<template>
  <view class="main-tabbar">
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
