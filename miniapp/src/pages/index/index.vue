<script setup lang="ts">
import { onShow } from '@dcloudio/uni-app'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import { useSessionStore } from '../../stores/session'

const session = useSessionStore()

function routeBySession() {
  if (!session.token) {
    uni.reLaunch({ url: '/pages/login/login' })
    return
  }
  uni.reLaunch({ url: '/pages/home/home' })
}

onShow(() => {
  routeBySession()
})
</script>

<template>
  <view class="page">
    <EnvironmentBadge />
    <text>加载中...</text>
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f7f2ea;
  color: #666666;
  font-size: 28rpx;
}
</style>
