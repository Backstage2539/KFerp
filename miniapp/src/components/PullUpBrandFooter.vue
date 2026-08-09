<script setup lang="ts">
withDefaults(defineProps<{
  withFixedTabbar?: boolean
}>(), {
  withFixedTabbar: false,
})
</script>

<template>
  <view :class="['pull-up-brand-footer', { 'with-fixed-tabbar': withFixedTabbar }]">
    <view class="pull-up-brand-reveal-spacer" aria-hidden="true" />
    <view class="pull-up-brand-signature">
      <text class="pull-up-brand-kicker">Drived By</text>
      <image
        class="pull-up-brand-wordmark"
        src="/static/branding/kefan-wordmark-silver.png"
        mode="widthFix"
        alt="棵凡咖啡"
      />
    </view>
    <view class="pull-up-brand-bottom-clearance" aria-hidden="true" />
  </view>
</template>

<style scoped>
.pull-up-brand-footer {
  width: 100%;
  flex: 0 0 auto;
  box-sizing: border-box;
}

.pull-up-brand-reveal-spacer {
  min-height: 104rpx;
}

.pull-up-brand-signature {
  min-height: 58rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16rpx;
  padding: 0 16rpx;
  box-sizing: border-box;
  opacity: .86;
}

.pull-up-brand-kicker {
  flex: 0 0 auto;
  color: #9ea4aa;
  font-size: 19rpx;
  font-weight: 500;
  letter-spacing: 2rpx;
  line-height: 1;
  white-space: nowrap;
}

.pull-up-brand-wordmark {
  width: 144rpx;
  height: auto;
  flex: 0 0 auto;
  opacity: .9;
}

.pull-up-brand-bottom-clearance {
  min-height: env(safe-area-inset-bottom);
  min-height: constant(safe-area-inset-bottom);
}

.with-fixed-tabbar .pull-up-brand-bottom-clearance {
  min-height: calc(166rpx + env(safe-area-inset-bottom));
  min-height: calc(166rpx + constant(safe-area-inset-bottom));
}
</style>
