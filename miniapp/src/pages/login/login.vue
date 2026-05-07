<script setup lang="ts">
import { ref } from 'vue'
import { loginWithCode } from '../../api/customerPortal'
import { useSessionStore } from '../../stores/session'
import { miniappThemeClass, miniappThemeMeta } from '../../utils/themes'

const session = useSessionStore()
const loading = ref(false)
const errorMessage = ref('')
const themeClass = miniappThemeClass()
const themeMeta = miniappThemeMeta()

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
  <view class="page" :class="themeClass">
    <view class="hero">
      <text class="eyebrow">{{ themeMeta.eyebrow }}</text>
      <text class="title">客户中心</text>
      <text class="subtitle">{{ themeMeta.subtitle }}</text>
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
  background: #f7f2ea;
  box-sizing: border-box;
}

.page.theme-coffee-factory {
  background: #f7f2ea;
}

.page.theme-clean-ops {
  background: #f5f7f6;
}

.page.theme-premium-partner {
  background: #fbf7ef;
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
  font-weight: 800;
}

.theme-clean-ops .eyebrow {
  color: #28624a;
}

.theme-premium-partner .eyebrow {
  color: #8a5c20;
}

.title {
  color: #171717;
  font-size: 52rpx;
  font-weight: 900;
  line-height: 1.14;
}

.subtitle {
  color: #5f5a52;
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
  min-height: 88rpx;
  background: #2b2118;
  border-radius: 10rpx;
  color: #ffffff;
  font-size: 30rpx;
  font-weight: 800;
}

.theme-clean-ops .login-button {
  background: #173b2e;
}

.theme-premium-partner .login-button {
  background: #17120d;
  color: #f8ddb0;
}

.error {
  color: #b42318;
  font-size: 26rpx;
  line-height: 1.5;
}
</style>
