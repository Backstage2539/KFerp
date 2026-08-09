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
  min-height: calc(100vh + 64rpx + constant(safe-area-inset-bottom)) !important;
  min-height: calc(100vh + 64rpx + env(safe-area-inset-bottom)) !important;
  display: flex;
  flex-direction: column;
}

.pull-up-brand-page-with-tabbar {
  min-height: calc(100vh + 230rpx + constant(safe-area-inset-bottom)) !important;
  min-height: calc(100vh + 230rpx + env(safe-area-inset-bottom)) !important;
}

.pull-up-brand-page > .pull-up-brand-footer {
  margin-top: auto;
}
</style>
