<script setup lang="ts">
import { ref } from 'vue'
import { loginWithCode, loginWithPassword, loginWithPhoneVerify, type LoginResponse } from '../../api/customerPortal'
import { useSessionStore } from '../../stores/session'
import { miniappThemeClass, miniappThemeMeta } from '../../utils/themes'

const session = useSessionStore()
const loading = ref(false)
const errorMessage = ref('')
const loginMode = ref<'quick' | 'password'>('quick')
const phone = ref('')
const password = ref('')
const themeClass = miniappThemeClass()
const themeMeta = miniappThemeMeta()
const phoneRe = /^1\d{10}$/

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

function completeLogin(response: LoginResponse) {
  session.setToken(response.token)
  session.applyContext(response)
  uni.redirectTo({ url: '/pages/home/home' })
}

async function handlePhoneLogin(event: { detail?: { code?: string; errMsg?: string } }) {
  if (loading.value) return

  const phoneCode = String(event?.detail?.code || '').trim()
  if (!phoneCode) {
    errorMessage.value = event?.detail?.errMsg || '未获得手机号授权'
    return
  }

  loading.value = true
  errorMessage.value = ''

  try {
    const code = await requestLoginCode()
    const response = await loginWithPhoneVerify(code, phoneCode)
    completeLogin(response)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '登录失败'
  } finally {
    loading.value = false
  }
}

async function handlePasswordLogin() {
  if (loading.value) return
  const normalizedPhone = phone.value.trim()
  const normalizedPassword = password.value.trim()
  if (!phoneRe.test(normalizedPhone)) {
    errorMessage.value = '请输入11位手机号'
    return
  }
  if (!normalizedPassword) {
    errorMessage.value = '请输入密码'
    return
  }

  loading.value = true
  errorMessage.value = ''

  try {
    const code = await requestLoginCode()
    const response = await loginWithPassword(code, normalizedPhone, normalizedPassword)
    completeLogin(response)
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
      <view class="mode-tabs">
        <button class="mode-tab" :class="{ active: loginMode === 'quick' }" @tap="loginMode = 'quick'">手机号快捷登录</button>
        <button class="mode-tab" :class="{ active: loginMode === 'password' }" @tap="loginMode = 'password'">密码登录</button>
      </view>

      <view v-if="loginMode === 'quick'" class="login-block">
        <button
          class="login-button"
          open-type="getPhoneNumber"
          :loading="loading"
          :disabled="loading"
          @getphonenumber="handlePhoneLogin"
        >
          手机号快捷登录
        </button>
        <button class="plain-button" :loading="loading" :disabled="loading" @tap="handleLogin">
          微信身份登录
        </button>
      </view>

      <view v-else class="login-block">
        <input v-model="phone" class="input" type="number" maxlength="11" placeholder="手机号" />
        <input v-model="password" class="input" password placeholder="密码" />
        <button class="login-button" :loading="loading" :disabled="loading" @tap="handlePasswordLogin">
          登录
        </button>
      </view>
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

.mode-tabs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12rpx;
  padding: 8rpx;
  background: rgba(43, 33, 24, 0.08);
  border-radius: 12rpx;
}

.mode-tab {
  min-height: 64rpx;
  background: transparent;
  border-radius: 8rpx;
  color: #5f5a52;
  font-size: 26rpx;
  font-weight: 800;
}

.mode-tab.active {
  background: #ffffff;
  color: #171717;
}

.login-block {
  display: flex;
  flex-direction: column;
  gap: 20rpx;
}

.input {
  height: 88rpx;
  padding: 0 24rpx;
  background: #ffffff;
  border: 2rpx solid rgba(43, 33, 24, 0.16);
  border-radius: 10rpx;
  color: #171717;
  font-size: 28rpx;
  box-sizing: border-box;
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

.plain-button {
  width: 100%;
  min-height: 76rpx;
  background: transparent;
  border: 2rpx solid rgba(43, 33, 24, 0.22);
  border-radius: 10rpx;
  color: #3b3128;
  font-size: 26rpx;
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
