<script setup lang="ts">
import { computed, ref } from 'vue'
import { guestServiceGuides, packagingEstimate } from '../utils/guest-entry'
const selectedService = ref('')
const selected = computed(() => guestServiceGuides.find(service => service.id === selectedService.value))
const kilograms = ref('10')
const grams = ref('227')
const estimate = computed(() => packagingEstimate(Number(kilograms.value), Number(grams.value)))
function openLogin() { uni.navigateTo({ url: '/pages/login/login' }) }
</script>
<template>
  <view class="guest-home">
    <view class="hero">
      <text class="eyebrow">QACOOHEE SERVICE</text>
      <text class="title">棵凡小程序</text>
      <text class="subtitle">咖啡选品、加工与仓储服务</text>
      <text class="browse-hint">先浏览服务，需要查询个人业务时再登录。</text>
    </view>
    <view class="section-heading">了解我们的服务</view>
    <view class="service-list">
      <button v-for="service in guestServiceGuides" :key="service.id" class="service-card" @tap="selectedService = selectedService === service.id ? '' : service.id">
        <view><text class="service-title">{{ service.title }}</text><text class="service-summary">{{ service.summary }}</text></view>
        <text class="arrow">{{ selectedService === service.id ? '−' : '›' }}</text>
      </button>
    </view>
    <view v-if="selected" class="guide-card">
      <text class="service-title">{{ selected.title }} · 服务流程</text>
      <text v-for="(step, index) in selected.steps" :key="step" class="guide-step">{{ index + 1 }}. {{ step }}</text>
      <button class="close-guide" @tap="selectedService = ''">收起</button>
    </view>
    <view class="calculator">
      <text class="service-title">包装数量换算</text>
      <view class="inputs">
        <view><text class="input-label">咖啡净重（kg）</text><input v-model="kilograms" type="digit" /></view>
        <view><text class="input-label">每袋净重（g）</text><input v-model="grams" type="digit" /></view>
      </view>
      <view class="result"><text>预计可装</text><text class="pack-count">{{ estimate.packs }} 袋</text><text>余 {{ estimate.remainderGrams }} g</text></view>
      <text class="calculator-note">按净重换算，未计生产损耗。</text>
    </view>
    <view class="login-footer">
      <text>已有员工或客户账号？</text>
      <button class="login-link" @tap="openLogin">登录，查看我的业务</button>
      <text class="privacy-note">浏览以上内容无需授权手机号、头像或昵称。</text>
    </view>
  </view>
</template>
<style scoped>
.guest-home{padding:36rpx 32rpx 44rpx;color:#25251f;background:#f7f2ea;min-height:100vh;box-sizing:border-box}.hero{display:flex;flex-direction:column;gap:10rpx;margin-bottom:32rpx}.eyebrow{font-size:23rpx;color:#76652e;font-weight:700;letter-spacing:1rpx}.title{font-size:44rpx;font-weight:800}.subtitle{font-size:27rpx;color:#66695e}.browse-hint{font-size:23rpx;color:#77796f;margin-top:8rpx}.section-heading{font-size:28rpx;font-weight:700;margin-bottom:16rpx}.service-list{display:flex;flex-direction:column;gap:12rpx}.service-card{display:flex;align-items:center;justify-content:space-between;text-align:left;padding:20rpx 24rpx;background:#fff;width:100%;margin:0;line-height:1.5;border-radius:14rpx}.service-card::after,.close-guide::after,.login-link::after{border:0}.service-title{display:block;font-size:28rpx;font-weight:700}.service-summary{display:block;font-size:23rpx;color:#777b6f;margin-top:4rpx}.arrow{font-size:36rpx;color:#65744e}.guide-card{background:#fff;padding:24rpx;border-radius:14rpx;margin-top:16rpx}.guide-step{display:block;font-size:25rpx;line-height:1.8;margin-top:12rpx;color:#535b4a}.close-guide{font-size:24rpx;line-height:2;background:transparent;color:#40623c;margin-top:12rpx}.calculator{padding:24rpx;background:#eef1e7;border:1rpx solid #dce3d1;border-radius:14rpx;margin-top:24rpx}.inputs{display:flex;gap:20rpx;margin-top:18rpx}.inputs>view{flex:1;min-width:0}.input-label{display:block;color:#616b55;font-size:23rpx;margin-bottom:8rpx}.inputs input{height:66rpx;padding:0 16rpx;background:#fff;border:1rpx solid #d5ddca;border-radius:8rpx;font-size:30rpx;box-sizing:border-box}.result{display:flex;align-items:baseline;gap:16rpx;margin-top:18rpx;font-size:25rpx;color:#4f5f43}.pack-count{font-size:36rpx;font-weight:800;color:#34512d}.calculator-note{display:block;font-size:22rpx;color:#7a826f;margin-top:10rpx}.login-footer{text-align:center;margin-top:28rpx;font-size:23rpx;color:#777b70}.login-link{margin:10rpx 0 0;background:transparent;color:#3d6739;font-size:27rpx;font-weight:700;line-height:2}.privacy-note{display:block;font-size:21rpx;margin-top:6rpx}
</style>
