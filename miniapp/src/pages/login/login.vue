<script setup lang="ts">
import { onHide, onLoad, onShow, onShareAppMessage, onShareTimeline } from '@dcloudio/uni-app'
import {
  defaultMiniappShare,
  defaultMiniappTimelineShare,
  refreshMiniappShareMenu,
} from '../../utils/miniappShare'
import { ref } from 'vue'
import { restoreRegistrationSession } from '../../utils/registrationSession'
import { safeLoginReturn } from '../../utils/loginReturn'
import { loginWithPhoneIdentify, type LoginResponse } from '../../api/customerPortal'
import { MiniRequestError } from '../../api/client'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { useSessionStore } from '../../stores/session'
import { phoneVerifiedDestination } from '../../utils/customerSwitch'
import { miniappThemeClass, miniappThemeMeta } from '../../utils/themes'

const session = useSessionStore()
const returnTo=ref('')
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
const loading = ref(false)
const errorMessage = ref('')
const themeClass = miniappThemeClass()
const themeMeta = miniappThemeMeta()
onLoad(query=>{try{returnTo.value=safeLoginReturn(decodeURIComponent(String(query?.return_to||'')))}catch{returnTo.value=''}})

function browseServices() { uni.reLaunch({ url: '/pages/bean-list-center/bean-list-center' }) }

function requestLoginCode(): Promise<string> {
  return new Promise((resolve, reject) => {
    uni.login({
      provider: 'weixin',
      success(result) {
        const code = String(result.code || '').trim()
        if (!code) {
          reject(new Error('未获得微信登录凭证'))
          return
        }
        resolve(code)
      },
      fail() {
        reject(new Error('微信登录失败，请联系管理员确认客户账号'))
      },
    })
  })
}

function completeLogin(response: LoginResponse) {
  session.setToken(response.token)
  session.applyContext(response)
  uni.reLaunch({ url: phoneVerifiedDestination(response, returnTo.value) })
}

async function handlePhoneLogin(event: { detail?: { code?: string; errMsg?: string } }) {
  if (loading.value) return

  const phoneCode = String(event?.detail?.code || '').trim()
  if (!phoneCode) {
    errorMessage.value = '未完成手机号授权，请重试或继续浏览'
    return
  }

  loading.value = true
  errorMessage.value = ''

  try {
    const code = await requestLoginCode()
    const response = await loginWithPhoneIdentify(code, phoneCode)
    completeLogin(response)
  } catch (error) {
    errorMessage.value = error instanceof MiniRequestError && (error.statusCode === 401 || error.statusCode === 403) ? '当前手机号无法登录，请联系管理员' : error instanceof Error ? error.message : '登录失败，请重试'
  } finally {
    loading.value = false
  }
}

onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
let pageVisible = false
const canRestoreRegistration = () => pageVisible && !session.autoRestoreBlocked
onHide(() => { pageVisible = false })
onShow(async () => {
  pageVisible = true
  void refreshMiniappShareMenu()
  if (!canRestoreRegistration()) return
  const hadToken = !!session.token
  try {
    await restoreRegistrationSession(session)
    if (canRestoreRegistration() && session.token && (session.accountType === 'employee' || session.bindings.some(binding => binding.status === 'approved'))) {
      completeLogin({ token: session.token, mini_user_id: session.miniUserID, current_customer_id: session.currentCustomerID, bindings: session.bindings, capabilities: session.capabilities, registration_complete: session.registrationComplete, registration: session.registration || undefined, account_type: session.accountType })
    }
  } catch (error) {
    if (hadToken && canRestoreRegistration()) errorMessage.value = error instanceof Error ? error.message : '登录状态恢复失败'
  }
})
</script>

<template>
  <view
    class="page pull-up-brand-page"
    :class="themeClass"
    @touchstart="handlePullUpBrandTouchStart"
    @touchmove="handlePullUpBrandTouchMove"
    @touchend="handlePullUpBrandTouchEnd"
    @touchcancel="handlePullUpBrandTouchCancel"
  >
    <EnvironmentBadge />
    <view class="hero">
      <text class="eyebrow">{{ themeMeta.eyebrow }}</text>
      <text class="title">棵凡小程序</text>
      <text class="subtitle">{{ themeMeta.subtitle }}</text>
    </view>

    <view class="browse-option">
      <button class="browse-button" @tap="browseServices">返回价格表中心</button>
      <text>授权手机号后可直接登录已绑定的客户账号。</text>
    </view>
    <view class="panel">
      <view class="login-block">
        <text>授权手机号后，自动识别员工、客户或访客身份。</text>
        <button class="login-button" open-type="getPhoneNumber" :loading="loading" :disabled="loading" @getphonenumber="handlePhoneLogin">
          授权手机号登录
        </button>
      </view>

      <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
    </view>

    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter :revealed="pullUpBrandRevealed" />
    </view>
  </view>
</template>

<style scoped>
.browse-option{text-align:center;margin-bottom:32rpx;color:#777b70;font-size:24rpx}.browse-button{background:#e6ecdd;color:#395331;font-weight:700;font-size:30rpx;margin-bottom:16rpx}.browse-button::after{border:0}
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
