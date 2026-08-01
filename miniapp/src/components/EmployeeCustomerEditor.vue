<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  createEmployeeCustomer,
  fetchEmployeeCustomer,
  updateEmployeeCustomer,
  type EmployeeCustomer,
  type EmployeeCustomerPayload,
  type EmployeeCustomersResponse,
} from '../api/customerPortal'

const props = defineProps<{
  visible: boolean
  token: string
  customerId?: number
  context?: EmployeeCustomersResponse
}>()

const emit = defineEmits<{
  close: []
  saved: [customer: EmployeeCustomer]
}>()

const loading = ref(false)
const saving = ref(false)
const errorMessage = ref('')
let customerLoadSequence = 0
const isAdmin = computed(() => Boolean(props.context?.is_admin))
const title = computed(() => Number(props.customerId || 0) > 0 ? '修改客户信息' : '新增客户')

const form = reactive<EmployeeCustomerPayload>({
  name: '',
  customer_type: '',
  company_name: '',
  company_address: '',
  company_phone: '',
  contact: '',
  phone: '',
  address: '',
  default_source_id: 0,
  default_order_type_id: 0,
  responsible_employee_id: 0,
  active: true,
  portal_enabled: false,
})

function resetForm(customer?: EmployeeCustomer) {
  Object.assign(form, {
    name: customer?.name || '',
    customer_type: customer?.customer_type || props.context?.customer_type_options?.[0]?.value || '',
    company_name: customer?.company_name || '',
    company_address: customer?.company_address || '',
    company_phone: customer?.company_phone || '',
    contact: customer?.contact || '',
    phone: customer?.phone || '',
    address: customer?.address || '',
    default_source_id: Number(customer?.default_source_id || props.context?.sources?.[0]?.id || 0),
    default_order_type_id: Number(customer?.default_order_type_id || props.context?.order_types?.[0]?.id || 0),
    responsible_employee_id: Number(customer?.responsible_employee_id || 0),
    active: customer?.active !== false,
    portal_enabled: Boolean(customer?.portal_enabled),
  })
}

function selectedIndex(rows: Array<{ id: number }>, id?: number): number {
  return Math.max(0, rows.findIndex((row) => Number(row.id) === Number(id || 0)))
}

function selectedTypeIndex(): number {
  return Math.max(0, (props.context?.customer_type_options || [])
    .findIndex((row) => row.value === form.customer_type))
}

function selectedEmployeeIndex(): number {
  return (props.context?.employees || [])
    .findIndex((row) => Number(row.id) === Number(form.responsible_employee_id || 0))
}

function selectedEmployeeName(): string {
  const index = selectedEmployeeIndex()
  return index >= 0 ? String(props.context?.employees?.[index]?.name || '') : ''
}

function pickerValue(event: { detail?: { value?: number | string } }): number {
  return Number(event.detail?.value || 0)
}

function switchValue(event: unknown): boolean {
  return Boolean((event as { detail?: { value?: boolean } })?.detail?.value)
}

function chooseCustomerType(event: { detail?: { value?: number | string } }) {
  form.customer_type = props.context?.customer_type_options?.[pickerValue(event)]?.value || ''
}

function chooseSource(event: { detail?: { value?: number | string } }) {
  form.default_source_id = Number(props.context?.sources?.[pickerValue(event)]?.id || 0)
}

function chooseOrderType(event: { detail?: { value?: number | string } }) {
  form.default_order_type_id = Number(props.context?.order_types?.[pickerValue(event)]?.id || 0)
}

function chooseEmployee(event: { detail?: { value?: number | string } }) {
  form.responsible_employee_id = Number(props.context?.employees?.[pickerValue(event)]?.id || 0)
}

async function loadCustomer() {
  const sequence = ++customerLoadSequence
  const targetCustomerID = Number(props.customerId || 0)
  if (!props.visible) {
    loading.value = false
    errorMessage.value = ''
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    if (targetCustomerID > 0) {
      const response = await fetchEmployeeCustomer(props.token, targetCustomerID)
      if (sequence !== customerLoadSequence || !props.visible || Number(props.customerId || 0) !== targetCustomerID) return
      resetForm(response.customer)
    } else {
      if (sequence !== customerLoadSequence || !props.visible || Number(props.customerId || 0) !== targetCustomerID) return
      resetForm()
    }
  } catch (cause) {
    if (sequence !== customerLoadSequence || !props.visible || Number(props.customerId || 0) !== targetCustomerID) return
    errorMessage.value = cause instanceof Error ? cause.message : '客户资料加载失败'
  } finally {
    if (sequence === customerLoadSequence) loading.value = false
  }
}

function requestClose() {
  if (saving.value) return
  customerLoadSequence += 1
  emit('close')
}

async function save() {
  if (!form.name.trim()) {
    uni.showToast({ title: '请填写客户名称', icon: 'none' })
    return
  }
  if (!form.customer_type || !form.default_source_id || !form.default_order_type_id) {
    uni.showToast({ title: '请选择客户类型、来源和订单类型', icon: 'none' })
    return
  }
  if (isAdmin.value && !Number(form.responsible_employee_id || 0)) {
    uni.showToast({ title: '请选择负责人', icon: 'none' })
    return
  }

  saving.value = true
  const targetCustomerID = Number(props.customerId || 0)
  const targetSequence = customerLoadSequence
  try {
    const base: EmployeeCustomerPayload = {
      name: form.name.trim(),
      customer_type: form.customer_type,
      company_name: form.company_name?.trim(),
      company_address: form.company_address?.trim(),
      company_phone: form.company_phone?.trim(),
      contact: form.contact?.trim(),
      phone: form.phone?.trim(),
      address: form.address?.trim(),
      default_source_id: Number(form.default_source_id),
      default_order_type_id: Number(form.default_order_type_id),
    }
    const payload = isAdmin.value
      ? {
          ...base,
          responsible_employee_id: Number(form.responsible_employee_id || 0),
          active: Boolean(form.active),
          portal_enabled: Boolean(form.portal_enabled),
        }
      : base
    const response = targetCustomerID > 0
      ? await updateEmployeeCustomer(props.token, targetCustomerID, payload)
      : await createEmployeeCustomer(props.token, payload)
    if (!props.visible || Number(props.customerId || 0) !== targetCustomerID || customerLoadSequence !== targetSequence) return
    emit('saved', response.customer)
    emit('close')
  } catch (cause) {
    uni.showToast({ title: cause instanceof Error ? cause.message : '客户保存失败', icon: 'none' })
  } finally {
    saving.value = false
  }
}

watch(
  () => [props.visible, props.customerId, props.token] as const,
  () => void loadCustomer(),
  { immediate: true },
)
</script>

<template>
  <view v-if="visible" class="overlay" @tap.self="requestClose">
    <view class="editor" @tap.stop>
      <view class="editor-head">
        <text class="editor-title">{{ title }}</text>
        <text class="close" :class="{ disabled: saving }" @tap="requestClose">关闭</text>
      </view>

      <view v-if="loading" class="state">正在加载客户资料...</view>
      <view v-else-if="errorMessage" class="state error">{{ errorMessage }}</view>
      <scroll-view v-else scroll-y class="form-scroll">
        <text class="label">客户名称 *</text>
        <input v-model="form.name" class="field" placeholder="客户名称" />

        <text class="label">客户类型 *</text>
        <picker
          mode="selector"
          :range="context?.customer_type_options || []"
          range-key="label"
          :value="selectedTypeIndex()"
          @change="chooseCustomerType"
        >
          <view class="field selector">{{ context?.customer_type_options?.[selectedTypeIndex()]?.label || '请选择客户类型' }}</view>
        </picker>

        <text class="label">联系人</text>
        <input v-model="form.contact" class="field" placeholder="联系人" />
        <text class="label">联系电话</text>
        <input v-model="form.phone" class="field" placeholder="联系电话（可含区号或分机）" />
        <text class="label">联系地址</text>
        <textarea v-model="form.address" class="field area" placeholder="联系地址" />

        <text class="label">公司名称</text>
        <input v-model="form.company_name" class="field" placeholder="公司名称" />
        <text class="label">公司电话</text>
        <input v-model="form.company_phone" class="field" placeholder="公司电话" />
        <text class="label">公司地址</text>
        <textarea v-model="form.company_address" class="field area" placeholder="公司地址" />

        <text class="label">默认来源 *</text>
        <picker
          mode="selector"
          :range="context?.sources || []"
          range-key="name"
          :value="selectedIndex(context?.sources || [], form.default_source_id)"
          @change="chooseSource"
        >
          <view class="field selector">{{ context?.sources?.[selectedIndex(context?.sources || [], form.default_source_id)]?.name || '请选择来源' }}</view>
        </picker>

        <text class="label">默认订单类型 *</text>
        <picker
          mode="selector"
          :range="context?.order_types || []"
          range-key="name"
          :value="selectedIndex(context?.order_types || [], form.default_order_type_id)"
          @change="chooseOrderType"
        >
          <view class="field selector">{{ context?.order_types?.[selectedIndex(context?.order_types || [], form.default_order_type_id)]?.name || '请选择订单类型' }}</view>
        </picker>

        <template v-if="isAdmin">
          <text class="label">负责人 *</text>
          <picker
            mode="selector"
            :range="context?.employees || []"
            range-key="name"
            :value="Math.max(0, selectedEmployeeIndex())"
            @change="chooseEmployee"
          >
            <view class="field selector">{{ selectedEmployeeName() || '请选择负责人' }}</view>
          </picker>
          <view class="switch-row">
            <text>客户启用</text>
            <switch :checked="form.active" color="#28624a" @change="form.active = switchValue($event)" />
          </view>
          <view class="switch-row">
            <text>允许登录客户门户</text>
            <switch :checked="form.portal_enabled" color="#28624a" @change="form.portal_enabled = switchValue($event)" />
          </view>
        </template>

        <button class="save" :loading="saving" :disabled="saving" @tap="save">保存客户</button>
      </scroll-view>
    </view>
  </view>
</template>

<style scoped>
.overlay { position: fixed; inset: 0; z-index: 300; display: flex; align-items: flex-end; background: rgba(16, 28, 22, .48); }
.editor { width: 100%; max-height: 92vh; padding: 28rpx 28rpx calc(24rpx + env(safe-area-inset-bottom)); border-radius: 24rpx 24rpx 0 0; box-sizing: border-box; background: #fff; }
.editor-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 18rpx; }
.editor-title { color: #1e362a; font-size: 32rpx; font-weight: 800; }
.close { padding: 12rpx; color: #28624a; font-size: 26rpx; }
.close.disabled { color: #a8b1ac; }
.form-scroll { height: 78vh; }
.label { display: block; margin: 0 0 8rpx 4rpx; color: #42524a; font-size: 25rpx; font-weight: 650; }
.field { width: 100%; min-height: 78rpx; margin-bottom: 16rpx; padding: 18rpx 20rpx; border: 1rpx solid #dfe7e2; border-radius: 12rpx; box-sizing: border-box; background: #fff; }
.area { height: 112rpx; }
.selector { color: #263c31; }
.switch-row { display: flex; align-items: center; justify-content: space-between; min-height: 78rpx; margin-bottom: 14rpx; padding: 0 8rpx; border-bottom: 1rpx solid #edf1ee; color: #42524a; font-size: 26rpx; }
.save { margin: 24rpx 0 12rpx; background: #28624a; color: #fff; }
.state { padding: 70rpx 20rpx; color: #66756d; text-align: center; }
.error { color: #a7352a; }
</style>
