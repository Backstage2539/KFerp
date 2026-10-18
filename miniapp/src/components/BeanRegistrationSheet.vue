<script setup lang="ts">
import { ref, watch } from 'vue'
const props = defineProps<{visible:boolean; title:string; nickname:string; busy:boolean; error:string}>()
const emit = defineEmits<{
  close: []
  nickname: [value:string]
  authorize: [detail:{code?:string;errMsg?:string}]
}>()
const keyboardHeight = ref(0)
watch(() => props.visible, () => { keyboardHeight.value = 0 })
function changeNickname(event: unknown) {
  const value = (event as {detail?:{value?:string}}).detail?.value
  if (typeof value === 'string') emit('nickname', value)
}
function close() { uni.hideKeyboard(); emit('close') }
function authorize(event: {detail:{code?:string;errMsg?:string}}) {
  uni.hideKeyboard()
  keyboardHeight.value = 0
  emit('authorize', event.detail)
}
function keyboardChange(event: {detail:{height:number}}) { keyboardHeight.value = Math.max(0, event.detail.height || 0) }
</script>

<template>
  <view v-if="visible" class="registration-overlay" @tap="close" @touchmove.stop.prevent>
    <view class="registration-sheet" :class="{typing: keyboardHeight > 0}" :style="{bottom: keyboardHeight + 'px', maxHeight: `calc(100vh - ${keyboardHeight}px - 24px)`}" @tap.stop>
      <view class="heading"><text class="title">登记后查看价格表</text><button class="close" aria-label="关闭登记" @tap="close">×</button></view>
      <text class="target">{{ title }}</text>
      <text class="label">昵称</text>
      <input
        :value="nickname" type="nickname" class="nickname" placeholder="选择微信昵称或填写昵称"
        maxlength="32" :adjust-position="false" confirm-type="done"
        @input="changeNickname" @change="changeNickname" @blur="changeNickname"
        @keyboardheightchange="keyboardChange"
      />
      <text class="hint">仅首次需要登记，之后可直接查看。</text>
      <text v-if="error" class="error">{{ error }}</text>
      <button class="authorize" :class="{inactive: busy || !nickname.trim()}" open-type="getPhoneNumber" :disabled="busy || !nickname.trim()" :loading="busy" @getphonenumber="authorize">{{ busy ? '正在打开价格表…' : '授权手机号并查看' }}</button>
      <text class="privacy">手机号用于身份验证与业务联系。</text>
      <button class="later" :disabled="busy" @tap="close">暂不查看</button>
    </view>
  </view>
</template>

<style scoped>
.registration-overlay{position:fixed;inset:0;z-index:1200;background:rgba(20,35,26,.42)}
.registration-sheet{position:absolute;left:0;right:0;box-sizing:border-box;max-height:90vh;overflow-y:auto;padding:32rpx 36rpx calc(20rpx + env(safe-area-inset-bottom));background:#fffdf8;border-radius:32rpx 32rpx 0 0;color:#253d31;animation:sheet-in .2s ease-out}
.typing .target,.typing .hint,.typing .privacy,.typing .later{display:none}.typing .heading{margin-bottom:12rpx}.typing .authorize{margin-top:20rpx}
@keyframes sheet-in{from{transform:translateY(100%)}to{transform:translateY(0)}}
.heading{display:flex;align-items:center;justify-content:space-between;gap:16rpx}.title{font-size:38rpx;font-weight:700}.close{margin:0;padding:0;width:64rpx;height:64rpx;line-height:58rpx;background:transparent;color:#758076;font-size:48rpx}.close::after,.later::after{border:0}
.target{display:block;line-height:1.5;margin:8rpx 0 24rpx;font-size:26rpx;color:#6c786d;overflow-wrap:anywhere}.label{display:block;margin-bottom:14rpx;font-size:27rpx;font-weight:600}
.nickname{box-sizing:border-box;height:88rpx;padding:0 24rpx;border:1rpx solid #d6dfd4;border-radius:14rpx;background:#fff;font-size:30rpx}
.hint,.privacy{display:block;color:#758076;font-size:24rpx;line-height:1.6}.hint{margin:16rpx 0 24rpx}.error{display:block;color:#a7352c;font-size:25rpx;line-height:1.5;margin:0 0 18rpx}
.authorize{background:#285d3e;color:#fff;font-size:30rpx;font-weight:600;line-height:92rpx;border-radius:14rpx}.authorize.inactive{background:#dfe7dc;color:#7c8c7c}.privacy{text-align:center;margin-top:18rpx}.later{background:transparent;color:#687568;font-size:27rpx;line-height:72rpx;margin-top:4rpx}
</style>
