<script setup lang="ts">
import { onShow, onShareAppMessage, onShareTimeline } from '@dcloudio/uni-app'
import {
  defaultMiniappShare,
  defaultMiniappTimelineShare,
  refreshMiniappShareMenu,
} from '../../utils/miniappShare'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import MainTabBar from '../../components/MainTabBar.vue'
import { restoreRegistrationSession } from '../../utils/registrationSession'
import GuestHome from '../../components/GuestHome.vue'
import { useSessionStore } from '../../stores/session'
const session = useSessionStore()
onShow(async () => {
 try{await restoreRegistrationSession(session)}catch{/* Directory remains accessible if restoration fails. */}
 if(session.accountType==='employee'||session.currentCustomerID>0)uni.reLaunch({url:'/pages/home/home'})
})

onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
onShow(() => { void refreshMiniappShareMenu() })
</script>
<template>
  <view class="page">
    <EnvironmentBadge />
    <GuestHome />
 <MainTabBar v-if="session.accountType!=='employee'" current="home" />
  </view>
</template>
<style scoped>
.page{min-height:100vh;background:#f7f2ea;padding-bottom:140rpx}.loading{display:block;padding:80rpx;text-align:center;color:#666}
</style>
