<script setup lang="ts">
import { ref } from 'vue'
import { loginWithCode } from '../../api/customerPortal'
import { useSessionStore } from '../../stores/session'

const session = useSessionStore()
const loading = ref(false)
const errorMessage = ref('')

function requestLoginCode(): Promise<string> {
  return new Promise((resolve, reject) => {
    uni.login({
      provider: 'weixin',
      success: (res) => {
        if (res.code) {
          resolve(res.code)
          return
        }
        reject(new Error('微信登录未返回 code'))
      },
      fail: (err) => reject(new Error(err.errMsg || '微信登录失败')),
    })
  })
}

async function handleLogin() {
  if (loading.value) return

  loading.value = true
  errorMessage.value = ''

  try {
    const code = await requestLoginCode()
    const response = await loginWithCode(code)
    session.setToken(response.token)
    session.applyContext(response)
    uni.redirectTo({ url: '/pages/home/home' })
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <view class="page">
    <view class="hero">
      <text class="eyebrow">KFerp</text>
      <text class="title">客户中心</text>
      <text class="subtitle">登录后查看豆单、订单、库存、物流和结算服务。</text>
    </view>

    <view class="panel">
      <button class="login-button" :loading="loading" :disabled="loading" @tap="handleLogin">
        微信一键登录
      </button>
      <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
    </view>
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 56rpx 32rpx;
  background: #f6f6f6;
  box-sizing: border-box;
}

.hero {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
  padding: 72rpx 0 48rpx;
}

.eyebrow {
  color: #6f5d2e;
  font-size: 26rpx;
  font-weight: 600;
}

.title {
  color: #171717;
  font-size: 48rpx;
  font-weight: 700;
}

.subtitle {
  color: #666666;
  font-size: 28rpx;
  line-height: 1.6;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 24rpx;
}

.login-button {
  width: 100%;
  background: #171717;
  border-radius: 8rpx;
  color: #ffffff;
  font-size: 30rpx;
}

.error {
  color: #b42318;
  font-size: 26rpx;
  line-height: 1.5;
}
</style>
