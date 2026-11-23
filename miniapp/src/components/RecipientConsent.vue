<script setup lang="ts">
import { ref } from 'vue'
import { useSessionStore } from '../stores/session'
import { acceptRecipientConsent, consentScope } from '../utils/recipientConsent'

defineProps<{ employeeEntry?: boolean }>()
const emit = defineEmits<{ accepted: []; declined: [] }>()
const session = useSessionStore()
const error = ref('')

function accept() {
  try {
    acceptRecipientConsent(session)
    error.value = ''
    emit('accepted')
  } catch {
    error.value = '未能保存授权，请稍后重试。'
  }
}
</script>

<template>
  <view class="consent-card">
    <text class="consent-title">使用收件信息前，请先阅读</text>
    <text>棵凡咖啡供应链使用收件人姓名、联系电话和地址，安排订单配送或样品寄送。点击「解析收件信息」时，你粘贴的文字会发送到服务器进行识别。</text>
    <text v-if="employeeEntry">代客户维护档案或录单前，请确认收件人已授权。保存到客户档案的资料可由有权限的工作人员查看和复用。</text>
    <text v-else>「仅本单使用」的地址用于当前订单。保存到地址簿后，当前客户名下的其他账号也可查看和复用。</text>
    <view class="consent-links">
      <navigator url="/pages/service-agreements/service-agreements?type=terms">《用户服务协议》</navigator>
      <navigator url="/pages/service-agreements/service-agreements?type=privacy">《隐私政策》</navigator>
    </view>
    <text v-if="error" class="consent-error">{{ error }}</text>
    <view class="consent-actions">
      <button @tap="emit('declined')">暂不同意</button>
      <button class="consent-accept" :disabled="!consentScope(session)" @tap="accept">同意并继续</button>
    </view>
  </view>
</template>

<style scoped>
.consent-card { display: flex; flex-direction: column; gap: 18rpx; padding: 28rpx; margin: 16rpx 0; color: #263c34; background: #fff; border: 1rpx solid #cbdad2; border-radius: 18rpx; font-size: 27rpx; line-height: 1.6; }
.consent-title { font-size: 32rpx; font-weight: bold; }
.consent-links, .consent-actions { display: flex; flex-wrap: wrap; gap: 12rpx; }
.consent-links navigator { padding: 6rpx 0; color: #20583e; text-decoration: underline; }
.consent-actions button { flex: 1; margin: 0; font-size: 28rpx; }
.consent-accept { background: #285c45; color: #fff; }
.consent-error { color: #a02d20; }
</style>
