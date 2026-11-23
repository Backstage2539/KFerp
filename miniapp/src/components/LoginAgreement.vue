<script setup lang="ts">
defineProps<{ checked: boolean; disabled?: boolean }>()
const emit = defineEmits<{ change: [checked: boolean] }>()
</script>

<template>
  <view class="agreement">
    <checkbox-group @change="emit('change', $event.detail.value.includes('agree'))">
      <label><checkbox value="agree" :checked="checked" :disabled="disabled" color="#285d3e" />我已阅读并同意</label>
    </checkbox-group>
    <navigator url="/pages/service-agreements/service-agreements?type=terms">《用户服务协议》</navigator>
    <text>和</text>
    <navigator url="/pages/service-agreements/service-agreements?type=privacy">《隐私政策》</navigator>
  </view>
</template>

<style scoped>
.agreement { display:flex; flex-wrap:wrap; align-items:center; color:#425047; font-size:26rpx; line-height:1.8; margin:12rpx 0; }
.agreement navigator { color:#20583e; text-decoration:underline; padding:6rpx 0; }
</style>
