<script setup lang="ts">
import {ref,watch} from 'vue'
import {useSessionStore} from '../stores/session'
import {updateRegistration,registerVisitor,wechatLoginCode} from '../api/registration'
const session=useSessionStore(),nickname=ref(''),busy=ref(false),message=ref('')
watch(()=>session.registration,p=>{nickname.value=p?.nickname||''},{immediate:true})
async function save(){busy.value=true;message.value='';try{session.applyContext(await updateRegistration(session.token,nickname.value));message.value='昵称已更新'}catch(e){message.value=e instanceof Error?e.message:'保存失败'}finally{busy.value=false}}
async function phone(e:{detail?:{code?:string}}){if(!e.detail?.code){message.value='已取消授权，原手机号保持不变';return}busy.value=true;message.value='';try{const result=await registerVisitor(await wechatLoginCode(),e.detail.code,nickname.value);session.setToken(result.token);session.applyContext(result);message.value='手机号已重新验证'}catch(e){message.value=e instanceof Error?e.message:'验证失败'}finally{busy.value=false}}
function register(){uni.navigateTo({url:'/pages/login/login?return_to='+encodeURIComponent('/pages/profile/profile')})}
</script>
<template><view class="registration"><text class="title">我的登记资料</text><template v-if="session.registrationComplete"><text>手机号：{{session.registration?.verified_phone}}（已验证）</text><input v-model="nickname" type="nickname" maxlength="32" placeholder="昵称"/><button :disabled="busy||!nickname.trim()" @tap="save">保存昵称</button><button open-type="getPhoneNumber" :disabled="busy||!nickname.trim()" @getphonenumber="phone">重新授权手机号</button></template><template v-else><text>登记手机号和昵称，即可查看普通豆单。</text><button @tap="register">完成登记</button></template><text v-if="message">{{message}}</text></view></template>
<style scoped>.registration{display:flex;flex-direction:column;gap:20rpx;padding:28rpx;background:white;border-radius:16rpx;margin:24rpx 0;color:#34523f;font-size:26rpx}.title{font-size:30rpx;font-weight:700}input{border:1rpx solid #d6dfd5;padding:20rpx;border-radius:8rpx}.registration button{width:100%;font-size:27rpx;background:#e8f0e5;color:#30523a}</style>
