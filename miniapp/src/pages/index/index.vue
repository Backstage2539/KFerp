<script setup lang="ts">
import { onShow } from '@dcloudio/uni-app'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import GuestHome from '../../components/GuestHome.vue'
import { useSessionStore } from '../../stores/session'
const session = useSessionStore()
onShow(() => {
  if (session.token) uni.reLaunch({ url: '/pages/home/home' })
})
</script>
<template>
  <view class="page">
    <EnvironmentBadge />
    <GuestHome v-if="!session.token" />
    <text v-else class="loading">加载中...</text>
  </view>
</template>
<style scoped>
.page{min-height:100vh;background:#f7f2ea}.loading{display:block;padding:80rpx;text-align:center;color:#666}
</style>
