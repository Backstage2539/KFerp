<script setup lang="ts">
</script>

<template>
  <view />
</template>

<style>
page {
  background: #f7f2ea;
  color: #171717;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}

button {
  border-radius: 6px;
}

.pull-up-brand-page {
  box-sizing: content-box !important;
  min-height: calc(100vh + 162rpx + constant(safe-area-inset-bottom)) !important;
  min-height: calc(100vh + 162rpx + env(safe-area-inset-bottom)) !important;
  display: flex;
  flex-direction: column;
}

.pull-up-brand-page-with-tabbar {
  min-height: calc(100vh + 328rpx + constant(safe-area-inset-bottom)) !important;
  min-height: calc(100vh + 328rpx + env(safe-area-inset-bottom)) !important;
}

.pull-up-brand-page > .pull-up-brand-footer-anchor {
  width: 100%;
  flex: 0 0 auto;
  order: 999;
  margin-top: auto;
  box-sizing: border-box;
}
</style>
