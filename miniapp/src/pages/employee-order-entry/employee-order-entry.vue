<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import {
  createEmployeeOrder,
  fetchEmployeeOrderForm,
  type EmployeeOrderForm,
  type EmployeeOrderProductFamily,
  type EmployeeOrderProductSpec,
} from '../../api/customerPortal'
import {
  customerProductFamilies,
  customerShippingDefaults,
  defaultProductSpec,
  firstSpecUnitPrice,
  productSpecLabel,
  productSpecWeightG,
} from '../../utils/employeeOrder'
import { useSessionStore } from '../../stores/session'

const session = useSessionStore()
const formData = ref<EmployeeOrderForm>()
const saving = ref(false)
const form = ref({
  order_date: '',
  customer_id: 0,
  source_id: 0,
  order_type_id: 0,
  pay_status_id: 0,
  ship_status_id: 0,
  receiver_name: '',
  receiver_phone: '',
  receiver_address: '',
  receiver_company: '',
  notes: '',
  product_family_id: 0,
  product_id: 0,
  product_name: '',
  product_kind: 'roasted_bean',
  spec_label: '',
  spec_g: 0,
  sales_unit: '袋',
  unit_bag_count: 0,
  unit_bean_g: 0,
  qty: 1,
  unit_price: 0,
})

const customerLabels = computed(() => formData.value?.customers.map((row) => row.name) || [])
const productFamilies = computed(() => customerProductFamilies(
  formData.value?.product_families || [],
  form.value.customer_id,
))
const productLabels = computed(() => productFamilies.value.map((row) => row.name))
const selectedFamily = computed(() => productFamilies.value.find(
  (row) => Number(row.parent_product_id) === Number(form.value.product_family_id),
))
const specLabels = computed(() => selectedFamily.value?.specs.map(productSpecLabel) || [])
const selectedSpec = computed(() => selectedFamily.value?.specs.find(
  (row) => Number(row.product_id || row.sku_id) === Number(form.value.product_id),
))

function clearProduct() {
  Object.assign(form.value, {
    product_family_id: 0,
    product_id: 0,
    product_name: '',
    spec_label: '',
    spec_g: 0,
    unit_price: 0,
  })
}

function chooseCustomer(event: any) {
  const customer = formData.value?.customers[Number(event.detail.value)]
  if (!customer) return
  form.value.customer_id = customer.id
  Object.assign(form.value, customerShippingDefaults(customer))
  if (Number(customer.default_source_id || 0) > 0) form.value.source_id = Number(customer.default_source_id)
  if (Number(customer.default_order_type_id || 0) > 0) form.value.order_type_id = Number(customer.default_order_type_id)
  if (form.value.product_family_id && !selectedFamily.value) clearProduct()
}

function applySpec(family: EmployeeOrderProductFamily, spec?: EmployeeOrderProductSpec) {
  if (!spec) {
    clearProduct()
    return
  }
  const tier = spec.tiers?.[0]
  form.value.product_family_id = Number(family.parent_product_id)
  form.value.product_id = Number(spec.product_id || spec.sku_id || 0)
  form.value.product_name = family.name
  form.value.product_kind = spec.product_kind || family.product_kind || 'roasted_bean'
  form.value.spec_label = productSpecLabel(spec)
  form.value.spec_g = productSpecWeightG(spec)
  form.value.sales_unit = spec.sales_unit || tier?.sales_unit || '袋'
  form.value.unit_bag_count = Number(spec.unit_bag_count || tier?.unit_bag_count || 0)
  form.value.unit_bean_g = Number(spec.unit_bean_g || 0)
  form.value.unit_price = firstSpecUnitPrice(spec)
}

function chooseProduct(event: any) {
  const family = productFamilies.value[Number(event.detail.value)]
  if (family) applySpec(family, defaultProductSpec(family))
}

function chooseSpec(event: any) {
  const family = selectedFamily.value
  const spec = family?.specs[Number(event.detail.value)]
  if (family && spec) applySpec(family, spec)
}

async function submit() {
  if (!form.value.customer_id) {
    uni.showToast({ title: '请选择客户', icon: 'none' })
    return
  }
  if (!form.value.product_id || !form.value.spec_label) {
    uni.showToast({ title: '请选择商品和规格', icon: 'none' })
    return
  }
  if (Number(form.value.qty) <= 0) {
    uni.showToast({ title: '请填写正确数量', icon: 'none' })
    return
  }
  saving.value = true
  try {
    const result = await createEmployeeOrder(session.token, {
      order_date: form.value.order_date,
      customer_id: form.value.customer_id,
      source_id: form.value.source_id,
      order_type_id: form.value.order_type_id,
      pay_status_id: form.value.pay_status_id,
      ship_status_id: form.value.ship_status_id,
      receiver_name: form.value.receiver_name,
      receiver_phone: form.value.receiver_phone,
      receiver_address: form.value.receiver_address,
      receiver_company: form.value.receiver_company,
      notes: form.value.notes,
      items: [{
        product_id: form.value.product_id,
        name: form.value.product_name,
        product_kind: form.value.product_kind,
        qty: Number(form.value.qty),
        spec_g: Number(form.value.spec_g),
        unit: form.value.sales_unit,
        sales_unit: form.value.sales_unit,
        unit_bag_count: form.value.unit_bag_count,
        unit_bean_g: form.value.unit_bean_g,
        unit_price: Number(form.value.unit_price),
      }],
    })
    uni.showModal({
      title: '录单成功',
      content: result.order_no,
      showCancel: false,
      success: () => uni.navigateTo({ url: '/pages/employee-orders/employee-orders' }),
    })
  } catch (cause) {
    uni.showToast({ title: cause instanceof Error ? cause.message : '录单失败', icon: 'none' })
  } finally {
    saving.value = false
  }
}

onLoad(async () => {
  formData.value = await fetchEmployeeOrderForm(session.token)
  form.value.order_date = formData.value.today
  form.value.source_id = formData.value.sources[0]?.id || 0
  form.value.order_type_id = formData.value.order_types[0]?.id || 0
  form.value.pay_status_id = formData.value.pay_statuses[0]?.id || 0
  form.value.ship_status_id = formData.value.ship_statuses[0]?.id || 0
})
</script>

<template>
  <view class="page">
    <view class="panel">
      <text class="title">新建销售订单</text>

      <text class="label">订单日期</text>
      <picker mode="date" :value="form.order_date" @change="form.order_date = ($event.detail as any).value">
        <view class="field">{{ form.order_date || '选择订单日期' }}</view>
      </picker>

      <text class="label">客户</text>
      <picker :range="customerLabels" @change="chooseCustomer">
        <view class="field">{{ formData?.customers.find((row) => row.id === form.customer_id)?.name || '选择客户 *' }}</view>
      </picker>

      <view class="section-title">商品明细</view>
      <text class="label">商品</text>
      <picker :range="productLabels" @change="chooseProduct">
        <view class="field">{{ form.product_name || '选择商品 *' }}</view>
      </picker>

      <text class="label">规格</text>
      <picker :range="specLabels" :disabled="!selectedFamily" @change="chooseSpec">
        <view class="field" :class="{ muted: !selectedFamily }">
          {{ form.spec_label || (selectedFamily ? '选择该商品的规格 *' : '请先选择商品') }}
        </view>
      </picker>

      <text class="label">数量</text>
      <input v-model="form.qty" type="number" class="field" placeholder="填写数量 *" />

      <view class="item-summary">
        <view><text>商品</text><strong>{{ form.product_name || '未选择' }}</strong></view>
        <view><text>规格</text><strong>{{ form.spec_label || '未选择' }}</strong></view>
        <view><text>数量</text><strong>{{ Number(form.qty) > 0 ? form.qty : '未填写' }}</strong></view>
      </view>

      <text class="label">销售单价</text>
      <input v-model="form.unit_price" type="digit" class="field" placeholder="填写销售单价" />

      <view class="section-title">收货信息</view>
      <text class="hint">选择客户后自动带入，可按本次订单修改</text>
      <text class="label">收货人</text>
      <input v-model="form.receiver_name" class="field" placeholder="收货人" />
      <text class="label">联系电话</text>
      <input v-model="form.receiver_phone" type="number" class="field" placeholder="联系电话" />
      <text class="label">收货单位</text>
      <input v-model="form.receiver_company" class="field" placeholder="收货单位" />
      <text class="label">收货地址</text>
      <textarea v-model="form.receiver_address" class="field area" placeholder="收货地址" />

      <text class="label">备注</text>
      <textarea v-model="form.notes" class="field area" placeholder="备注" />
      <button class="submit" :loading="saving" :disabled="saving" @tap="submit">提交订单</button>
    </view>
  </view>
</template>

<style scoped>
.page { min-height: 100vh; padding: 28rpx; background: #f5f7f6; box-sizing: border-box; }
.panel { padding: 28rpx; background: #fff; border-radius: 18rpx; }
.title { display: block; margin-bottom: 28rpx; font-size: 36rpx; font-weight: 800; }
.section-title { margin: 30rpx 0 18rpx; padding-top: 24rpx; border-top: 1rpx solid #edf1ee; font-size: 30rpx; font-weight: 750; }
.label { display: block; margin: 0 0 8rpx 4rpx; color: #42524a; font-size: 25rpx; font-weight: 650; }
.hint { display: block; margin: -8rpx 0 20rpx; color: #718078; font-size: 23rpx; }
.field { width: 100%; min-height: 82rpx; margin-bottom: 18rpx; padding: 20rpx; border: 1rpx solid #dfe7e2; border-radius: 12rpx; box-sizing: border-box; background: #fff; }
.muted { color: #9aa59f; background: #f7f9f8; }
.area { height: 130rpx; }
.item-summary { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10rpx; margin: 4rpx 0 22rpx; }
.item-summary view { min-width: 0; padding: 16rpx 12rpx; border-radius: 10rpx; background: #f1f6f3; }
.item-summary text, .item-summary strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.item-summary text { margin-bottom: 6rpx; color: #6e7c74; font-size: 22rpx; }
.item-summary strong { color: #203b2e; font-size: 25rpx; }
.submit { margin-top: 12rpx; background: #28624a; color: #fff; }
</style>
