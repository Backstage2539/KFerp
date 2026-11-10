<script setup lang="ts">
defineProps<{visible:boolean; title:string; busy:boolean; error:string}>()
const emit = defineEmits<{ close: []; authorize: [detail:{code?:string;errMsg?:string}] }>()
function close() { emit('close') }
function authorize(event: {detail:{code?:string;errMsg?:string}}) { emit('authorize', event.detail) }
</script>

<template>
  <view v-if="visible" class="registration-overlay" @tap="close" @touchmove.stop.prevent>
    <view class="registration-sheet" @tap.stop>
      <view class="heading"><text class="title">登记后查看价格表</text><button class="close" aria-label="关闭登记" @tap="close">×</button></view>
      <text class="target">{{ title }}</text>
      <text class="hint">授权手机号后识别身份，访客可继续浏览。</text>
      <text v-if="error" class="error">{{ error }}</text>
      <button class="authorize" :class="{inactive: busy}" open-type="getPhoneNumber" :disabled="busy" :loading="busy" @getphonenumber="authorize">{{ busy ? '正在打开价格表…' : '授权手机号并查看' }}</button>
      <text class="privacy">手机号用于身份验证与业务联系。</text>
      <button class="later" :disabled="busy" @tap="close">暂不查看</button>
    </view>
  </view>
</template>

<style scoped>
.registration-overlay{position:fixed;inset:0;z-index:1200;background:rgba(20,35,26,.42)}
.registration-sheet{position:absolute;left:0;right:0;bottom:0;box-sizing:border-box;max-height:90vh;overflow-y:auto;padding:32rpx 36rpx calc(20rpx + env(safe-area-inset-bottom));background:#fffdf8;border-radius:32rpx 32rpx 0 0;color:#253d31;animation:sheet-in .2s ease-out}
@keyframes sheet-in{from{transform:translateY(100%)}to{transform:translateY(0)}}
.heading{display:flex;align-items:center;justify-content:space-between;gap:16rpx}.title{font-size:38rpx;font-weight:700}.close{margin:0;padding:0;width:64rpx;height:64rpx;line-height:58rpx;background:transparent;color:#758076;font-size:48rpx}.close::after,.later::after{border:0}
.target{display:block;line-height:1.5;margin:8rpx 0 24rpx;font-size:26rpx;color:#6c786d;overflow-wrap:anywhere}
.hint,.privacy{display:block;color:#758076;font-size:24rpx;line-height:1.6}.hint{margin:16rpx 0 24rpx}.error{display:block;color:#a7352c;font-size:25rpx;line-height:1.5;margin:0 0 18rpx}
.authorize{background:#285d3e;color:#fff;font-size:30rpx;font-weight:600;line-height:92rpx;border-radius:14rpx}.authorize.inactive{background:#dfe7dc;color:#7c8c7c}.privacy{text-align:center;margin-top:18rpx}.later{background:transparent;color:#687568;font-size:27rpx;line-height:72rpx;margin-top:4rpx}
</style>
