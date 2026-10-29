<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onShareAppMessage, onShareTimeline, onShow } from '@dcloudio/uni-app'
import CustomerPricePreview from '../../components/CustomerPricePreview.vue'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { defaultMiniappShare, defaultMiniappTimelineShare, refreshMiniappShareMenu } from '../../utils/miniappShare'

const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
const usageCode = ref<'direct_ship' | 'product_order'>('direct_ship')
const title = computed(() => usageCode.value === 'product_order' ? '下单价格表' : '一件代发价格表')

onLoad((options) => {
  usageCode.value = options?.usage_code === 'product_order' ? 'product_order' : 'direct_ship'
})

onShow(() => { void refreshMiniappShareMenu() })
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
</script>

<template>
  <view class="page pull-up-brand-page" @touchstart="handlePullUpBrandTouchStart" @touchmove="handlePullUpBrandTouchMove" @touchend="handlePullUpBrandTouchEnd" @touchcancel="handlePullUpBrandTouchCancel">
    <EnvironmentBadge />
    <view class="head">
      <text class="title">{{ title }}</text>
      <text class="hint">当前登录客户正在使用的已发布价格，只读预览</text>
    </view>
    <CustomerPricePreview :usage-code="usageCode" />
    <view class="pull-up-brand-footer-anchor"><PullUpBrandFooter :revealed="pullUpBrandRevealed" /></view>
  </view>
</template>

<style scoped>
.page{min-height:100vh;padding:28rpx;box-sizing:border-box;background:#f4f7f5;color:#173126}.head{display:flex;flex-direction:column;gap:10rpx;margin:16rpx 0 24rpx}.title{font-size:38rpx;font-weight:900}.hint,.table-version,.state{color:#728078;font-size:24rpx}.table-card{display:flex;justify-content:space-between;gap:16rpx;margin-bottom:18rpx;padding:22rpx;border:1rpx solid #dbe5df;border-radius:16rpx;background:#fff}.table-name{font-weight:800}.table-scroll{width:100%;border:1rpx solid #dbe5df;border-radius:16rpx;background:#fff}.price-grid{min-width:940rpx}.table-row{display:grid;grid-template-columns:260rpx 220rpx 220rpx 240rpx;min-height:84rpx;border-bottom:1rpx solid #e8eeea}.table-row text{display:flex;align-items:center;padding:14rpx 18rpx;border-right:1rpx solid #eef2ef;font-size:24rpx}.table-header{min-height:72rpx;background:#eaf3ee;font-weight:800}.price{color:#17603f;font-weight:800}.state{display:block;padding:80rpx 20rpx;text-align:center}.error{display:block;padding:24rpx;color:#b42318}
</style>
