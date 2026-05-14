<script setup lang="ts">
import { ref } from 'vue'
import { loginWithPassword } from '../../api/customerPortal'
import { useSessionStore } from '../../stores/session'
import { customerEntryRoute } from '../../utils/customerSwitch'
import { miniappThemeClass, miniappThemeMeta } from '../../utils/themes'

const session = useSessionStore()
const loading = ref(false)
const errorMessage = ref('')
const loginForm = ref({ login: '', password: '' })
const themeClass = miniappThemeClass()
const themeMeta = miniappThemeMeta()

async function handleLogin() {
  if (loading.value) return

  const login = loginForm.value.login.trim()
  const password = loginForm.value.password.trim()
  if (!login || !password) {
    errorMessage.value = '请输入用户名和密码'
    return
  }

  loading.value = true
  errorMessage.value = ''

  try {
    const response = await loginWithPassword(login, password)
    session.setToken(response.token)
    session.applyContext(response)
    uni.redirectTo({ url: customerEntryRoute(response) })
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '账号或密码不正确'
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
      <input v-model="loginForm.login" class="input" placeholder="用户名或手机号" />
      <input v-model="loginForm.password" class="input" type="password" placeholder="密码" />
      <button class="login-button" :loading="loading" :disabled="loading" @tap="handleLogin">
        登录
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

.input {
  min-height: 88rpx;
  padding: 0 28rpx;
  border: 1rpx solid #ead9bd;
  border-radius: 10rpx;
  background: #fffaf2;
  color: #171717;
  font-size: 30rpx;
  box-sizing: border-box;
}

.theme-clean-ops .input {
  border-color: #dfe7e2;
  background: #ffffff;
}

.theme-premium-partner .input {
  border-color: #eadab7;
  background: #fffdf8;
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
