<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import {
  createEmployeeOrder,
  fetchEmployeeOrderForm,
  type EmployeeOrderCustomer,
  type EmployeeOrderForm,
  type EmployeeOrderProductFamily,
  type EmployeeOrderProductSpec,
} from '../../api/customerPortal'
import { isAuthenticationExpiredRequestError } from '../../api/client'
import {
  customerProductFamilies,
  customerShippingDefaults,
  defaultProductSpec,
  employeeOrderProductCategories,
  employeeOrderProductCategory,
  employeeOrderProductFamilyKey,
  filterEmployeeOrderCustomers,
  filterEmployeeOrderProductFamilies,
  firstSpecUnitPrice,
  productSpecLabel,
  productSpecWeightG,
  salesUnitLabel,
  shanghaiToday,
  type EmployeeOrderProductCategory,
} from '../../utils/employeeOrder'
import { useSessionStore } from '../../stores/session'

const session = useSessionStore()
const formData = ref<EmployeeOrderForm>()
const loading = ref(false)
const saving = ref(false)
const loadError = ref('')
const authExpired = ref(false)
const customerSelectorOpen = ref(false)
const customerQuery = ref('')
const productSelectorOpen = ref(false)
const productQuery = ref('')
const productCategory = ref<EmployeeOrderProductCategory>('all')

const form = ref({
  order_date: shanghaiToday(),
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
  product_family_key: '',
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

const selectedCustomer = computed(() => formData.value?.customers.find(
  (row) => Number(row.id) === Number(form.value.customer_id),
))
const filteredCustomers = computed(() => filterEmployeeOrderCustomers(
  formData.value?.customers || [],
  customerQuery.value,
))
const productFamilies = computed(() => customerProductFamilies(
  formData.value?.product_families || [],
  form.value.customer_id,
))
const filteredProductFamilies = computed(() => filterEmployeeOrderProductFamilies(
  formData.value?.product_families || [],
  form.value.customer_id,
  productQuery.value,
  productCategory.value,
))
const selectedFamily = computed(() => productFamilies.value.find(
  (row) => employeeOrderProductFamilyKey(row) === form.value.product_family_key,
))
const specLabels = computed(() => selectedFamily.value?.specs.map(productSpecLabel) || [])
const selectedSpecIndex = computed(() => Math.max(0, selectedFamily.value?.specs.findIndex(
  (row) => Number(row.product_id || row.sku_id) === Number(form.value.product_id),
) ?? 0))
const displayedSalesUnit = computed(() => salesUnitLabel(form.value.sales_unit))

function categoryLabel(family: EmployeeOrderProductFamily): string {
  const category = employeeOrderProductCategory(family)
  return employeeOrderProductCategories.find((row) => row.key === category)?.label || '未分类'
}

function clearProduct() {
  Object.assign(form.value, {
    product_family_key: '',
    product_family_id: 0,
    product_id: 0,
    product_name: '',
    spec_label: '',
    spec_g: 0,
    sales_unit: '袋',
    unit_bag_count: 0,
    unit_bean_g: 0,
    unit_price: 0,
  })
}

function openCustomerSelector() {
  if (loading.value || !formData.value) return
  customerQuery.value = ''
  customerSelectorOpen.value = true
}

function closeCustomerSelector() {
  customerSelectorOpen.value = false
}

function chooseCustomer(customer: EmployeeOrderCustomer) {
  form.value.customer_id = Number(customer.id)
  Object.assign(form.value, customerShippingDefaults(customer))
  if (Number(customer.default_source_id || 0) > 0) form.value.source_id = Number(customer.default_source_id)
  if (Number(customer.default_order_type_id || 0) > 0) form.value.order_type_id = Number(customer.default_order_type_id)
  if (form.value.product_family_key && !selectedFamily.value) clearProduct()
  closeCustomerSelector()
}

function openProductSelector() {
  if (!form.value.customer_id) {
    uni.showToast({ title: '请先选择客户', icon: 'none' })
    return
  }
  if (loading.value || !formData.value) return
  productQuery.value = ''
  productCategory.value = 'all'
  productSelectorOpen.value = true
}

function closeProductSelector() {
  productSelectorOpen.value = false
}

function applySpec(family: EmployeeOrderProductFamily, spec: EmployeeOrderProductSpec) {
  const tier = spec.tiers?.[0]
  form.value.product_family_key = employeeOrderProductFamilyKey(family)
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

function chooseProduct(family: EmployeeOrderProductFamily) {
  const spec = defaultProductSpec(family)
  if (!spec) {
    uni.showToast({ title: '该商品暂无可选规格', icon: 'none' })
    return
  }
  applySpec(family, spec)
  closeProductSelector()
}

function chooseSpec(event: { detail?: { value?: number | string } }) {
  const family = selectedFamily.value
  const spec = family?.specs[Number(event.detail?.value || 0)]
  if (family && spec) applySpec(family, spec)
}

function setProductCategory(category: EmployeeOrderProductCategory) {
  productCategory.value = category
}

function goToLogin() {
  session.clearSession()
  uni.reLaunch({ url: '/pages/login/login' })
}

async function loadForm() {
  loading.value = true
  loadError.value = ''
  authExpired.value = false
  try {
    if (!session.token) throw new Error('登录已失效')
    const data = await fetchEmployeeOrderForm(session.token)
    formData.value = {
      ...data,
      customers: data.customers || [],
      sources: data.sources || [],
      order_types: data.order_types || [],
      pay_statuses: data.pay_statuses || [],
      ship_statuses: data.ship_statuses || [],
      product_families: data.product_families || [],
    }
    form.value.source_id ||= formData.value.sources[0]?.id || 0
    form.value.order_type_id ||= formData.value.order_types[0]?.id || 0
    form.value.pay_status_id ||= formData.value.pay_statuses[0]?.id || 0
    form.value.ship_status_id ||= formData.value.ship_statuses[0]?.id || 0
  } catch (cause) {
    authExpired.value = !session.token || isAuthenticationExpiredRequestError(cause)
    if (authExpired.value) session.clearSession()
    formData.value = undefined
    loadError.value = authExpired.value
      ? '登录已失效，请重新登录'
      : (cause instanceof Error ? cause.message : '录单数据加载失败')
  } finally {
    loading.value = false
  }
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
    if (isAuthenticationExpiredRequestError(cause)) {
      authExpired.value = true
      loadError.value = '登录已失效，请重新登录'
      session.clearSession()
      return
    }
    uni.showToast({ title: cause instanceof Error ? cause.message : '录单失败', icon: 'none' })
  } finally {
    saving.value = false
  }
}

onLoad(() => void loadForm())
</script>

<template>
  <view class="page">
    <view class="panel">
      <text class="title">新建销售订单</text>

      <view v-if="loading" class="status-card">
        <text>正在加载客户和商品...</text>
      </view>
      <view v-else-if="loadError" class="status-card error-card">
        <text>{{ loadError }}</text>
        <button v-if="authExpired" class="status-action" @tap="goToLogin">重新登录</button>
        <button v-else class="status-action" @tap="loadForm">重试</button>
      </view>

      <text class="label">订单日期</text>
      <picker mode="date" :value="form.order_date" @change="form.order_date = ($event.detail as any).value">
        <view class="field selector-field">{{ form.order_date }}</view>
      </picker>

      <text class="label">客户</text>
      <view
        class="field selector-field"
        :class="{ muted: loading || !formData }"
        @tap="openCustomerSelector"
      >
        <text>{{ selectedCustomer?.name || '搜索并选择客户 *' }}</text>
        <text class="chevron">›</text>
      </view>

      <view class="section-title">商品明细</view>
      <text class="label">商品</text>
      <view
        class="field selector-field"
        :class="{ muted: !form.customer_id || loading || !formData }"
        @tap="openProductSelector"
      >
        <text>{{ form.product_name || (form.customer_id ? '搜索并选择商品 *' : '请先选择客户') }}</text>
        <text class="chevron">›</text>
      </view>

      <text class="label">规格</text>
      <picker mode="selector" :range="specLabels" :value="selectedSpecIndex" :disabled="!selectedFamily" @change="chooseSpec">
        <view class="field selector-field" :class="{ muted: !selectedFamily }">
          <text>{{ form.spec_label || (selectedFamily ? '选择该商品的规格 *' : '请先选择商品') }}</text>
          <text class="chevron">›</text>
        </view>
      </picker>

      <text class="label">数量（{{ displayedSalesUnit }}）</text>
      <view class="input-with-unit">
        <input v-model="form.qty" type="number" class="field" :placeholder="`填写数量（${displayedSalesUnit}） *`" />
        <text class="unit-suffix">{{ displayedSalesUnit }}</text>
      </view>

      <view class="item-summary">
        <view><text>商品</text><text class="summary-value">{{ form.product_name || '未选择' }}</text></view>
        <view><text>规格</text><text class="summary-value">{{ form.spec_label || '未选择' }}</text></view>
        <view><text>数量</text><text class="summary-value">{{ Number(form.qty) > 0 ? `${form.qty} ${displayedSalesUnit}` : '未填写' }}</text></view>
      </view>

      <text class="label">销售单价（元/{{ displayedSalesUnit }}）</text>
      <view class="input-with-unit">
        <input v-model="form.unit_price" type="digit" class="field" :placeholder="`填写每${displayedSalesUnit}单价`" />
        <text class="unit-suffix">元/{{ displayedSalesUnit }}</text>
      </view>

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
      <button class="submit" :loading="saving" :disabled="saving || loading || Boolean(loadError)" @tap="submit">提交订单</button>
    </view>

    <view v-if="customerSelectorOpen" class="overlay" @tap.self="closeCustomerSelector">
      <view class="select-sheet" @tap.stop>
        <view class="sheet-head">
          <text class="sheet-title">选择客户</text>
          <text class="sheet-close" @tap="closeCustomerSelector">关闭</text>
        </view>
        <input
          v-model="customerQuery"
          class="search-input"
          focus
          confirm-type="search"
          placeholder="搜索客户名称 / 拼音 / 首字母"
        />
        <scroll-view scroll-y class="option-list">
          <view v-for="customer in filteredCustomers" :key="customer.id" class="option-row" @tap="chooseCustomer(customer)">
            <text class="option-name">{{ customer.name }}</text>
          </view>
          <text v-if="!filteredCustomers.length" class="empty-state">没有找到客户</text>
        </scroll-view>
        <text class="result-hint">最多显示 20 条，请输入更多关键词缩小范围</text>
      </view>
    </view>

    <view v-if="productSelectorOpen" class="overlay" @tap.self="closeProductSelector">
      <view class="select-sheet product-sheet" @tap.stop>
        <view class="sheet-head">
          <text class="sheet-title">选择商品</text>
          <text class="sheet-close" @tap="closeProductSelector">关闭</text>
        </view>
        <input
          v-model="productQuery"
          class="search-input"
          focus
          confirm-type="search"
          placeholder="商品 / 别名 / 拼音 / 编码 / 规格"
        />
        <scroll-view scroll-x class="category-scroll" :show-scrollbar="false">
          <view class="category-row">
            <button
              v-for="category in employeeOrderProductCategories"
              :key="category.key"
              class="category-button"
              :class="{ active: productCategory === category.key }"
              @tap="setProductCategory(category.key)"
            >
              {{ category.label }}
            </button>
          </view>
        </scroll-view>
        <scroll-view scroll-y class="option-list product-list">
          <view
            v-for="family in filteredProductFamilies"
            :key="employeeOrderProductFamilyKey(family)"
            class="option-row product-row"
            @tap="chooseProduct(family)"
          >
            <text class="option-name">{{ family.name }}</text>
            <text class="option-meta">{{ categoryLabel(family) }} · {{ family.specs.length }} 个规格</text>
            <text class="option-specs">{{ family.specs.map(productSpecLabel).join('、') || '暂无规格' }}</text>
          </view>
          <text v-if="!filteredProductFamilies.length" class="empty-state">没有找到符合条件的商品</text>
        </scroll-view>
        <text class="result-hint">每个商品只显示一条，最多显示 30 条</text>
      </view>
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
.selector-field { display: flex; align-items: center; justify-content: space-between; gap: 16rpx; }
.chevron { color: #7b8780; font-size: 38rpx; line-height: 1; }
.muted { color: #9aa59f; background: #f7f9f8; }
.area { height: 130rpx; }
.input-with-unit { position: relative; }
.input-with-unit .field { padding-right: 150rpx; }
.unit-suffix { position: absolute; top: 25rpx; right: 22rpx; color: #65736b; font-size: 24rpx; }
.item-summary { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10rpx; margin: 4rpx 0 22rpx; }
.item-summary view { min-width: 0; padding: 16rpx 12rpx; border-radius: 10rpx; background: #f1f6f3; }
.item-summary text { display: block; overflow: hidden; margin-bottom: 6rpx; color: #6e7c74; font-size: 22rpx; text-overflow: ellipsis; white-space: nowrap; }
.item-summary .summary-value { margin-bottom: 0; color: #203b2e; font-size: 25rpx; font-weight: 700; }
.submit { margin-top: 12rpx; background: #28624a; color: #fff; }
.status-card { display: flex; align-items: center; justify-content: space-between; gap: 16rpx; margin-bottom: 22rpx; padding: 18rpx 20rpx; border-radius: 12rpx; background: #eef5f1; color: #355345; font-size: 24rpx; }
.error-card { background: #fff2f0; color: #a7352a; }
.status-action { flex: 0 0 auto; margin: 0; padding: 0 22rpx; min-height: 58rpx; line-height: 58rpx; background: #28624a; color: #fff; font-size: 24rpx; }
.overlay { position: fixed; inset: 0; z-index: 100; display: flex; align-items: flex-end; background: rgba(16, 28, 22, 0.48); }
.select-sheet { width: 100%; max-height: 78vh; padding: 28rpx 28rpx calc(24rpx + env(safe-area-inset-bottom)); border-radius: 24rpx 24rpx 0 0; box-sizing: border-box; background: #fff; }
.product-sheet { max-height: 86vh; }
.sheet-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 20rpx; }
.sheet-title { color: #1e362a; font-size: 32rpx; font-weight: 800; }
.sheet-close { padding: 12rpx; color: #28624a; font-size: 26rpx; }
.search-input { width: 100%; min-height: 78rpx; padding: 0 22rpx; border: 2rpx solid #b8ccc0; border-radius: 12rpx; box-sizing: border-box; background: #f9fbfa; }
.category-scroll { width: 100%; margin: 18rpx 0 8rpx; white-space: nowrap; }
.category-row { display: inline-flex; gap: 12rpx; padding-right: 20rpx; }
.category-button { display: inline-flex; align-items: center; margin: 0; padding: 0 22rpx; min-height: 62rpx; line-height: 62rpx; border: 1rpx solid #cad6cf; border-radius: 32rpx; background: #fff; color: #516159; font-size: 24rpx; }
.category-button::after { border: 0; }
.category-button.active { border-color: #28624a; background: #eaf3ee; color: #28624a; font-weight: 750; }
.option-list { height: 52vh; margin-top: 14rpx; }
.product-list { height: 48vh; }
.option-row { display: flex; flex-direction: column; gap: 8rpx; padding: 22rpx 10rpx; border-bottom: 1rpx solid #edf1ee; }
.option-name { color: #172c22; font-size: 29rpx; font-weight: 700; }
.option-meta { color: #7b5a25; font-size: 23rpx; }
.option-specs { color: #6d7b73; font-size: 23rpx; line-height: 1.5; }
.empty-state { display: block; padding: 80rpx 20rpx; color: #7d8982; text-align: center; }
.result-hint { display: block; padding-top: 14rpx; color: #89958e; font-size: 21rpx; text-align: center; }
</style>
