<script setup lang="ts">
import { reactive, ref } from 'vue'
import { onShareAppMessage, onShareTimeline, onShow } from '@dcloudio/uni-app'
import {
  createRecipientAddress,
  deleteRecipientAddress,
  fetchRecipientAddresses,
  parseEmployeeCustomerRecipient,
  updateRecipientAddress,
  type CustomerRecipientAddress,
} from '../../api/customerPortal'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { useSessionStore } from '../../stores/session'
import { defaultMiniappShare, defaultMiniappTimelineShare, refreshMiniappShareMenu } from '../../utils/miniappShare'

const session = useSessionStore()
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
const loading = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const rows = ref<CustomerRecipientAddress[]>([])
const pastedRecipient = ref('')
const editing = ref(false)
const form = reactive(emptyForm())

function emptyForm() {
  return { id: 0, recipient_name: '', phone: '', company: '', province: '', city: '', district: '', detail_address: '', is_default: false, revision: 0 }
}

function resetForm() {
  Object.assign(form, emptyForm())
  pastedRecipient.value = ''
  editing.value = false
}

async function load() {
  if (!session.token) {
    uni.reLaunch({ url: '/pages/login/login' })
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    const data = await fetchRecipientAddresses(session.token)
    rows.value = data.rows || []
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '收件地址加载失败'
  } finally {
    loading.value = false
  }
}

function startCreate() {
  resetForm()
  form.is_default = rows.value.length === 0
  editing.value = true
}

function startEdit(row: CustomerRecipientAddress) {
  Object.assign(form, row)
  pastedRecipient.value = ''
  editing.value = true
}

async function parseRecipient() {
  if (!pastedRecipient.value.trim()) return
  try {
    const parsed = await parseEmployeeCustomerRecipient(session.token, pastedRecipient.value)
    form.recipient_name = parsed.recipient_name || form.recipient_name
    form.phone = parsed.phone || form.phone
    form.province = parsed.province || ''
    form.city = parsed.city || ''
    form.district = parsed.district || ''
    form.detail_address = parsed.detail_address || parsed.address || ''
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '收件信息解析失败'
  }
}

async function save() {
  if (saving.value) return
  if (!form.recipient_name.trim() || !form.phone.trim() || !form.detail_address.trim()) {
    errorMessage.value = '请填写收件人、联系电话和详细地址'
    return
  }
  saving.value = true
  errorMessage.value = ''
  const payload = {
    recipient_name: form.recipient_name.trim(), phone: form.phone.trim(), company: form.company.trim(),
    province: form.province.trim(), city: form.city.trim(), district: form.district.trim(), detail_address: form.detail_address.trim(),
    is_default: form.is_default, expected_revision: Number(form.revision || 0),
  }
  try {
    if (form.id) await updateRecipientAddress(session.token, form.id, payload)
    else await createRecipientAddress(session.token, payload)
    uni.showToast({ title: '收件地址已保存', icon: 'success' })
    resetForm()
    await load()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '收件地址保存失败'
  } finally {
    saving.value = false
  }
}

async function remove(row: CustomerRecipientAddress) {
  const answer = await uni.showModal({ title: '删除收件地址', content: `确认删除 ${row.recipient_name} 的地址？` })
  if (!answer.confirm) return
  try {
    await deleteRecipientAddress(session.token, row)
    if (form.id === row.id) resetForm()
    await load()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '收件地址删除失败'
  }
}

function setDefault(event: Event) {
  form.is_default = (event as unknown as { detail?: { value?: boolean } }).detail?.value === true
}

onShow(() => { void load(); void refreshMiniappShareMenu() })
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
</script>

<template>
  <view
    class="page pull-up-brand-page"
    @touchstart="handlePullUpBrandTouchStart"
    @touchmove="handlePullUpBrandTouchMove"
    @touchend="handlePullUpBrandTouchEnd"
    @touchcancel="handlePullUpBrandTouchCancel"
  >
    <EnvironmentBadge />
    <view class="header"><text class="title">收件地址</text><text class="muted">当前客户的所有账号共用</text></view>
    <button v-if="!editing" class="primary" @tap="startCreate">新增收件地址</button>
    <view v-if="editing" class="panel form">
      <textarea v-model="pastedRecipient" class="textarea" placeholder="粘贴收件人、电话和地址" />
      <button class="secondary" @tap="parseRecipient">解析收件信息</button>
      <input v-model="form.recipient_name" class="input" placeholder="收件人" />
      <input v-model="form.phone" class="input" placeholder="联系电话" />
      <input v-model="form.company" class="input" placeholder="公司/门店（可选）" />
      <view class="region"><input v-model="form.province" class="input" placeholder="省" /><input v-model="form.city" class="input" placeholder="市" /><input v-model="form.district" class="input" placeholder="区/县" /></view>
      <input v-model="form.detail_address" class="input" placeholder="详细地址" />
      <label class="switch-row"><text>设为默认地址</text><switch :checked="form.is_default" color="#28624a" @change="setDefault" /></label>
      <view class="actions"><button class="secondary" @tap="resetForm">取消</button><button class="primary" :disabled="saving" @tap="save">保存</button></view>
    </view>
    <text v-if="loading" class="muted">加载中...</text>
    <view v-for="row in rows" :key="row.id" class="panel address-card">
      <view class="address-head"><text class="name">{{ row.recipient_name }} · {{ row.phone }}</text><text v-if="row.is_default" class="badge">默认</text></view>
      <text v-if="row.company" class="muted">{{ row.company }}</text>
      <text>{{ row.province }}{{ row.city }}{{ row.district }}{{ row.detail_address }}</text>
      <view class="actions"><button class="secondary compact" @tap="startEdit(row)">编辑</button><button class="danger compact" @tap="remove(row)">删除</button></view>
    </view>
    <text v-if="!loading && !rows.length" class="muted empty">暂无收件地址</text>
    <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter :revealed="pullUpBrandRevealed" />
    </view>
  </view>
</template>

<style scoped>
.page,.form,.address-card{display:flex;flex-direction:column;gap:16rpx}.page{padding:24rpx;background:#f6f4ef;min-height:100vh;box-sizing:border-box}.header{display:flex;justify-content:space-between;align-items:center}.title{font-size:34rpx;font-weight:900}.panel{padding:22rpx;border:1rpx solid #e4ded4;border-radius:12rpx;background:#fff}.input,.textarea{min-height:76rpx;padding:0 18rpx;border:1rpx solid #d8d2ca;border-radius:8rpx;background:#fafafa;box-sizing:border-box}.textarea{min-height:120rpx;padding-top:16rpx}.region,.actions,.address-head,.switch-row{display:flex;gap:12rpx;align-items:center}.region .input,.actions button{flex:1}.address-head,.switch-row{justify-content:space-between}.name{font-weight:800}.badge{padding:4rpx 12rpx;border-radius:999rpx;background:#e8f5ed;color:#28624a;font-size:22rpx}.primary,.secondary,.danger{min-height:72rpx;margin:0;border-radius:8rpx;font-size:25rpx}.primary{background:#2b2118;color:#fff}.secondary{background:#fff;border:1rpx solid #d8d2ca}.danger{background:#fff4f2;color:#a12b21;border:1rpx solid #e8c6c0}.compact{min-height:60rpx}.muted{color:#707070;font-size:24rpx}.empty{text-align:center;padding:40rpx}.error{padding:18rpx;color:#b42318}
</style>
