<script setup lang="ts">
type MainTabKey = 'home' | 'orders' | 'billing' | 'mine'

const props = defineProps<{
  current: MainTabKey
}>()

const tabs: Array<{ key: MainTabKey; label: string; url: string }> = [
  { key: 'home', label: '首页', url: '/pages/home/home' },
  { key: 'orders', label: '订单', url: '/pages/service/service?key=orders' },
  { key: 'billing', label: '账单', url: '/pages/service/service?key=settlement' },
  { key: 'mine', label: '我的', url: '/pages/profile/profile' },
]

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
  z-index: 20;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10rpx;
  padding: 14rpx 18rpx calc(14rpx + env(safe-area-inset-bottom));
  background: rgba(255, 255, 255, .96);
  border-top: 1rpx solid #e6ddd0;
  box-shadow: 0 -8rpx 22rpx rgba(23, 23, 23, .08);
  box-sizing: border-box;
}

.tab-item {
  min-height: 86rpx;
  display: flex;
  align-items: center;
  justify-content: center;
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
  font-size: 28rpx;
  font-weight: 900;
  line-height: 1.2;
}

.tab-item.active .tab-label {
  color: #fff8eb;
}

.tab-pressed {
  opacity: .82;
}
</style>
