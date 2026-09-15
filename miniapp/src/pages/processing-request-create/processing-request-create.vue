<script setup lang="ts">
import { onLoad, onShareAppMessage, onShareTimeline, onShow } from '@dcloudio/uni-app'
import { ref } from 'vue'
import CustomerProcessingPanel from '../../components/CustomerProcessingPanel.vue'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { useProcessingPrefillStore } from '../../stores/processingPrefill'
import { useSessionStore } from '../../stores/session'
import type { ProcessingPrefillItem } from '../../utils/customerInventory'
import { defaultMiniappShare, defaultMiniappTimelineShare, refreshMiniappShareMenu } from '../../utils/miniappShare'

const session = useSessionStore()
const prefill = useProcessingPrefillStore()
const items = ref<ProcessingPrefillItem[]>([])
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()

onLoad(() => { items.value = prefill.consume(session.currentCustomerID) })
onShow(() => { void refreshMiniappShareMenu() })
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
</script>

<template>
  <view class="page pull-up-brand-page" @touchstart="handlePullUpBrandTouchStart" @touchmove="handlePullUpBrandTouchMove" @touchend="handlePullUpBrandTouchEnd" @touchcancel="handlePullUpBrandTouchCancel">
    <EnvironmentBadge />
    <view class="customer-card">
      <text class="eyebrow">客户生产申请</text>
      <text class="customer-name">{{ session.currentCustomerName || '当前客户' }}</text>
      <text class="hint">提交时重新校验物料，并立即预订可用原料和包材</text>
    </view>
    <CustomerProcessingPanel :token="session.token" :customer-id="session.currentCustomerID" :prefill-items="items" standalone />
    <view class="pull-up-brand-footer-anchor"><PullUpBrandFooter :revealed="pullUpBrandRevealed" /></view>
  </view>
</template>

<style scoped>
.page{min-height:100vh;padding:26rpx;box-sizing:border-box;background:#f5f1eb;color:#2b2118}.customer-card{display:flex;flex-direction:column;gap:8rpx;padding:28rpx;margin-bottom:20rpx;border-radius:22rpx;background:linear-gradient(135deg,#2b2118,#75502f);color:#fff8eb}.eyebrow{font-size:22rpx;font-weight:800;opacity:.8}.customer-name{font-size:38rpx;font-weight:900}.hint{font-size:23rpx;opacity:.8}
</style>
