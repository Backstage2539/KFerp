<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'

type ServiceKey = 'beanList' | 'productOrder' | 'directShip' | 'processing' | 'inventory' | 'shipping' | 'settlement'

const serviceLabels: Record<ServiceKey, string> = {
  beanList: '我的豆单',
  productOrder: '现货下单',
  directShip: '一件代发',
  processing: '代加工',
  inventory: '我的库存',
  shipping: '物流查询',
  settlement: '结算中心',
}

const serviceKey = ref<ServiceKey>('beanList')
const title = computed(() => serviceLabels[serviceKey.value])

onLoad((query) => {
  const key = String(query?.key || '')
  if (key in serviceLabels) {
    serviceKey.value = key as ServiceKey
  }
})
</script>

<template>
  <view class="page">
    <view class="header">
      <text class="eyebrow">服务入口</text>
      <text class="title">{{ title }}</text>
      <text class="subtitle">这个入口已经打通，业务页面会按优先级逐步接入。</text>
    </view>

    <view class="panel">
      <text class="panel-title">当前状态</text>
      <text class="panel-body">已完成客户身份识别、客户绑定和能力包控制。下一步接入对应业务数据。</text>
    </view>
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 32rpx;
  background: #f6f6f6;
  box-sizing: border-box;
}

.header {
  display: flex;
  flex-direction: column;
  gap: 14rpx;
  padding: 24rpx 0 36rpx;
}

.eyebrow {
  color: #6f5d2e;
  font-size: 24rpx;
  font-weight: 600;
}

.title {
  color: #171717;
  font-size: 44rpx;
  font-weight: 700;
}

.subtitle {
  color: #666666;
  font-size: 26rpx;
  line-height: 1.6;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
  padding: 28rpx;
  background: #ffffff;
  border: 1rpx solid #e8e8e8;
  border-radius: 8rpx;
}

.panel-title {
  color: #171717;
  font-size: 30rpx;
  font-weight: 700;
}

.panel-body {
  color: #666666;
  font-size: 26rpx;
  line-height: 1.6;
}
</style>
