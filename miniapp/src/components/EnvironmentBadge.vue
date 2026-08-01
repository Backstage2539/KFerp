<script setup lang="ts">
import { environmentBadgeText } from '../config/environment'

const label = environmentBadgeText()
</script>

<template>
  <view v-if="label" class="environment-badge">{{ label }}</view>
</template>

<style scoped>
.environment-badge {
  position: fixed;
  top: 12rpx;
  right: 16rpx;
  z-index: 1200;
  padding: 7rpx 14rpx;
  border: 1rpx solid rgba(151, 75, 0, 0.36);
  border-radius: 999rpx;
  background: rgba(255, 241, 213, 0.94);
  color: #8a4300;
  font-size: 20rpx;
  font-weight: 700;
  line-height: 1.3;
  pointer-events: none;
}
</style>
