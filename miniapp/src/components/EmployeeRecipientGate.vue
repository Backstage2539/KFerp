<script setup lang="ts">
import { onMounted, ref } from 'vue'
import RecipientConsent from './RecipientConsent.vue'
import { useSessionStore } from '../stores/session'
import { consentScope } from '../utils/recipientConsent'
import { restoreRegistrationSession } from '../utils/registrationSession'

const session = useSessionStore()
const error = ref('')
async function identify() {
  if (!session.token || consentScope(session)) return
  error.value = ''
  try { await restoreRegistrationSession(session) }
  catch { error.value = '无法确认当前账号，请重试' }
}
onMounted(identify)
function back() {
  uni.navigateBack({ fail: () => uni.reLaunch({ url: '/pages/index/index' }) })
}
</script>

<template>
  <view v-if="!session.token">
    <navigator url="/pages/login/login">登录后继续</navigator>
    <button @tap="back">返回</button>
  </view>
  <view v-else-if="error"><text>{{ error }}</text><button @tap="identify">重试</button></view>
  <RecipientConsent v-else employee-entry @declined="back" />
</template>
