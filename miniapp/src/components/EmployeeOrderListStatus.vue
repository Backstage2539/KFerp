<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  fetchEmployeeOrderDetail, updateEmployeeOrderStatus,
  type EmployeeOrder, type EmployeeOrderDetailResponse,
} from '../api/customerPortal'
import { employeeReceiptMethods } from '../composables/useEmployeeOrderStatus'
import { chooseEmployeePaymentVoucher } from '../utils/employeePaymentVoucher'
import { useSessionStore } from '../stores/session'

const props = defineProps<{ order: EmployeeOrder }>()
const emit = defineEmits<{ changed: [id: number] }>()
const session = useSessionStore()
const detail = ref<EmployeeOrderDetailResponse>()
const field = ref<'pay' | 'ship' | ''>('')
const selected = ref(0)
const method = ref('')
const prepayment = ref('')
const goods = ref('')
const shipping = ref('')
const voucher = ref(0)
const voucherName = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const options = computed(() => field.value === 'pay' ? detail.value?.status_options?.pay_statuses || [] : detail.value?.status_options?.ship_statuses || [])
const selectedName = computed(() => options.value.find(item => item.id === selected.value)?.name || '')
const paid = computed(() => /已付款|已收款|已支付/.test(selectedName.value))
const received = computed(() => selectedName.value.includes('已收款'))
watch(selected, () => {
  if (!detail.value || field.value !== 'pay') return
  prepayment.value = detail.value.order.prepayment_amount || '0'
  voucher.value = Number(detail.value.order.payment_voucher_asset_id || 0)
  voucherName.value = detail.value.order.payment_voucher?.filename || (voucher.value ? '已上传凭证' : '')
})

function close() {
  if (!loading.value && !saving.value) field.value = ''
}

async function open(target: 'pay' | 'ship') {
  if (loading.value || saving.value) return
  loading.value = true
  error.value = ''
  try {
    const result = await fetchEmployeeOrderDetail(session.token, props.order.id)
    if (!result.can_update_status || (target === 'ship' && !result.can_update_ship_status)) {
      throw new Error(target === 'ship' ? result.ship_status_update_block_reason || '当前订单无法修改发货状态' : result.status_update_block_reason || '当前订单无法修改付款状态')
    }
    detail.value = result
    field.value = target
    selected.value = Number(target === 'pay' ? result.order.pay_status_id : result.order.ship_status_id) || 0
    method.value = result.order.payment_method || ''
    prepayment.value = result.order.prepayment_amount || '0'
    goods.value = result.order.payment_goods_amount || ''
    shipping.value = result.order.payment_shipping_amount || ''
    voucher.value = Number(result.order.payment_voucher_asset_id || 0)
    voucherName.value = result.order.payment_voucher?.filename || (voucher.value ? '已上传凭证' : '')
  } catch (cause) {
    uni.showToast({ title: cause instanceof Error ? cause.message : '加载失败，请重试', icon: 'none' })
  } finally {
    loading.value = false
  }
}

async function chooseVoucher() {
  if (loading.value || saving.value) return
  loading.value = true
  try {
    const asset = await chooseEmployeePaymentVoucher(session.token, props.order.id)
    if (asset) {
      voucher.value = asset.id
      voucherName.value = asset.filename || '已上传凭证'
      error.value = ''
    }
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '凭证上传失败，请重试'
  } finally {
    loading.value = false
  }
}

async function save() {
  const current = detail.value?.order
  if (!current || !field.value || saving.value || loading.value) return
  if (!options.value.some(item => item.id === selected.value)) { error.value = '请选择状态'; return }
  const previousStatus = field.value === 'pay' ? Number(current.pay_status_id || 0) : Number(current.ship_status_id || 0)
  if (field.value === 'ship' && selected.value === previousStatus) {
    field.value = ''
    return
  }
  const changed = field.value === 'pay' && selected.value !== previousStatus
  if (changed && paid.value && !voucher.value) { error.value = '请上传收款凭证'; return }
  if (field.value === 'pay' && /预付款|已付|已收|已支付/.test(selectedName.value) && !method.value) { error.value = '请选择收款方式'; return }
  const money = /^\d+(\.\d{1,2})?$/
  if (field.value === 'pay' && (!money.test(prepayment.value.trim()) || Number(prepayment.value) > Number(current.grand_total || 0))) {
    error.value = '预付款须为有效金额，且不超过订单应收'
    return
  }
  if (field.value === 'pay' && received.value && (!money.test(goods.value.trim()) || !money.test(shipping.value.trim()))) {
    error.value = '请填写有效的实收货款和运费'
    return
  }
  saving.value = true
  error.value = ''
  try {
    await updateEmployeeOrderStatus(session.token, props.order.id, {
      edit_revision: current.edit_revision || '',
      pay_status_id: field.value === 'pay' ? selected.value : Number(current.pay_status_id || 0),
      ship_status_id: field.value === 'ship' ? selected.value : Number(current.ship_status_id || 0),
      payment_method: field.value === 'pay' ? method.value : current.payment_method || '',
      ...(field.value === 'ship' ? { payment_unchanged: true } : {
        prepayment_amount: Number(prepayment.value || 0),
        ...(received.value ? { payment_goods_amount: Number(goods.value), payment_shipping_amount: Number(shipping.value) } : {}),
        ...(voucher.value ? { payment_voucher_asset_id: voucher.value } : {}),
      }),
    })
    field.value = ''
    emit('changed', props.order.id)
    uni.showToast({ title: '状态已保存', icon: 'success' })
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '保存失败，请重试'
  } finally { saving.value = false }
}
</script>

<template>
  <view class="statuses">
    <view v-if="order.pay_status !== '无需收款'" class="status" @tap.stop="open('pay')">付款 {{ order.pay_status || '-' }} <text>⌄</text></view>
    <view v-else class="status">无需收款</view>
    <view v-if="order.delivery_mode === 'customer_stock'" class="status">无需发货</view>
    <view v-else-if="order.confirmation_required" class="status">发货 {{ order.ship_status || '-' }}</view>
    <view v-else class="status" @tap.stop="open('ship')">发货 {{ order.ship_status || '-' }} <text>⌄</text></view>
  </view>
  <view v-if="field" class="overlay" @tap.stop="close">
    <view class="sheet" @tap.stop>
      <text class="title">{{ field === 'pay' ? '修改付款状态' : '修改发货状态' }}</text>
      <picker :range="options" range-key="name" :value="Math.max(0, options.findIndex(item => item.id === selected))" @change="selected = options[Number($event.detail.value)]?.id || 0">
        <view class="input">{{ selectedName || '请选择状态' }} ⌄</view>
      </picker>
      <template v-if="field === 'pay'">
        <template v-if="selectedName.includes('预付款') || Number(prepayment) > 0">
          <text>预付款金额</text><input v-model="prepayment" class="input" type="digit" />
        </template>
        <template v-if="/预付款|已付|已收|已支付/.test(selectedName)">
          <text>收款方式</text>
          <picker :range="employeeReceiptMethods" :value="Math.max(0, employeeReceiptMethods.indexOf(method))" @change="method = employeeReceiptMethods[Number($event.detail.value)] || ''">
            <view class="input">{{ method || '请选择收款方式' }} ⌄</view>
          </picker>
        </template>
        <template v-if="received">
          <text>实收货款及其他费用</text><input v-model="goods" class="input" type="digit" />
          <text>实收运费</text><input v-model="shipping" class="input" type="digit" placeholder="无运费请填 0" />
        </template>
        <template v-if="paid">
          <text>收款凭证</text>
          <button :loading="loading" :disabled="loading || saving" @tap="chooseVoucher">{{ voucherName || '从相册选择凭证' }}</button>
        </template>
      </template>
      <text v-if="error" class="error">{{ error }}</text>
      <view class="actions"><button :disabled="saving || loading" @tap="close">取消</button><button :loading="saving" :disabled="saving || loading" @tap="save">保存</button></view>
    </view>
  </view>
</template>

<style scoped>
.statuses{display:flex;gap:20rpx;margin:14rpx 0;color:#28624a;font-size:24rpx}.status{padding:10rpx 12rpx;background:#edf6f1;border-radius:8rpx}.overlay{position:fixed;inset:0;z-index:99;background:rgba(0,0,0,.4);display:flex;align-items:flex-end}.sheet{width:100%;background:white;border-radius:24rpx 24rpx 0 0;padding:32rpx;box-sizing:border-box;display:flex;flex-direction:column;gap:14rpx;max-height:85vh;overflow:auto}.title{font-size:32rpx;font-weight:700}.input{padding:18rpx;background:#f4f7f5;border-radius:10rpx}.error{color:#b42318}.actions{display:flex;gap:20rpx}.actions button{flex:1}
</style>
