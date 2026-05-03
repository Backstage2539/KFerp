<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { fetchMe } from '../../api/customerPortal'
import { useSessionStore } from '../../stores/session'
import { visibleHomeEntries } from '../../utils/capabilities'

const session = useSessionStore()
const loading = ref(false)
const errorMessage = ref('')

const entries = computed(() => visibleHomeEntries(session.capabilities))
const customerName = computed(() => session.currentCustomerName || '客户中心')

async function loadContext() {
  if (!session.token) {
    uni.redirectTo({ url: '/pages/login/login' })
    return
  }

  loading.value = true
  errorMessage.value = ''

  try {
    const response = await fetchMe(session.token)
    session.applyContext(response)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '客户信息加载失败'
    uni.redirectTo({ url: '/pages/login/login' })
  } finally {
    loading.value = false
  }
}

onShow(() => {
  void loadContext()
})
</script>

<template>
  <view class="page">
    <view class="header">
      <text class="eyebrow">当前客户</text>
      <text class="title">{{ customerName }}</text>
      <text class="subtitle">选择一个服务入口继续办理。</text>
    </view>

    <view v-if="loading" class="state">
      <text>加载中...</text>
    </view>

    <view v-else-if="errorMessage" class="state error">
      <text>{{ errorMessage }}</text>
    </view>

    <view v-else-if="entries.length" class="grid">
      <view v-for="entry in entries" :key="entry.key" class="entry">
        <text class="entry-label">{{ entry.label }}</text>
      </view>
    </view>

    <view v-else class="state">
      <text>暂无可用服务</text>
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
  font-size: 40rpx;
  font-weight: 700;
}

.subtitle {
  color: #666666;
  font-size: 26rpx;
}

.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20rpx;
}

.entry {
  min-height: 156rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24rpx;
  background: #ffffff;
  border: 1rpx solid #e8e8e8;
  border-radius: 8rpx;
  box-sizing: border-box;
}

.entry-label {
  color: #171717;
  font-size: 30rpx;
  font-weight: 600;
}

.state {
  min-height: 180rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #666666;
  font-size: 28rpx;
}

.error {
  color: #b42318;
}
</style>
