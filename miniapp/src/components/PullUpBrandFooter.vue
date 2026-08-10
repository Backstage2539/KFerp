<script setup lang="ts">
withDefaults(defineProps<{
  withFixedTabbar?: boolean
  revealed?: boolean
}>(), {
  withFixedTabbar: false,
  revealed: false,
})
</script>

<template>
  <view
    :class="[
      'pull-up-brand-footer',
      {
        'with-fixed-tabbar': withFixedTabbar,
        'is-revealed': revealed,
      },
    ]"
    :aria-hidden="!revealed"
  >
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
  max-height: 0;
  overflow: hidden;
  pointer-events: none;
  flex: 0 0 auto;
  box-sizing: border-box;
  opacity: 0;
  transition: max-height 220ms cubic-bezier(.22, .72, .2, 1), opacity 140ms ease-out;
}

.pull-up-brand-footer.is-revealed {
  max-height: calc(162rpx + env(safe-area-inset-bottom));
  max-height: calc(162rpx + constant(safe-area-inset-bottom));
  opacity: 1;
}

.pull-up-brand-footer.with-fixed-tabbar.is-revealed {
  max-height: calc(328rpx + env(safe-area-inset-bottom));
  max-height: calc(328rpx + constant(safe-area-inset-bottom));
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
