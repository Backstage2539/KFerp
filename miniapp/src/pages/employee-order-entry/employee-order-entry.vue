<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import EmployeeCustomerEditor from '../../components/EmployeeCustomerEditor.vue'
import {
  createEmployeeOrder,
  deleteEmployeeOrderDraft,
  fetchEmployeeCustomers,
  fetchEmployeeOrderDraft,
  fetchEmployeeOrderForm,
  saveEmployeeOrderDraft,
  type EmployeeCustomer,
  type EmployeeCustomersResponse,
  type EmployeeOrderCustomer,
  type EmployeeOrderDraft,
  type EmployeeOrderDraftItem,
  type EmployeeOrderDraftPayload,
  type EmployeeOrderForm,
  type EmployeeOrderProductFamily,
  type EmployeeOrderProductSpec,
} from '../../api/customerPortal'
import { isAuthenticationExpiredRequestError } from '../../api/client'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import {
  buildEmployeeOrderItemsPayload,
  createEmployeeOrderItem,
  customerProductFamilies,
  customerShippingDefaults,
  defaultProductSpec,
  employeeOrderItemFromSpec,
  employeeOrderItemsTotal,
  employeeOrderProductCategories,
  employeeOrderProductCategory,
  employeeOrderProductFamilyKey,
  employeeOrderShippingChanged,
  filterEmployeeOrderCustomers,
  filterEmployeeOrderProductFamilies,
  productSpecLabel,
  preserveEmployeeOrderDraftItemsForMissingCustomer,
  revalidateEmployeeOrderItems,
  salesUnitLabel,
  shanghaiToday,
  type EmployeeOrderProductCategory,
  type EmployeeOrderShippingSnapshot,
} from '../../utils/employeeOrder'
import { useSessionStore } from '../../stores/session'

const session = useSessionStore()
const formData = ref<EmployeeOrderForm>()
const customerContext = ref<EmployeeCustomersResponse>()
const loading = ref(false)
const saving = ref(false)
const savingDraft = ref(false)
const clearingDraft = ref(false)
const loadError = ref('')
const authExpired = ref(false)
const customerSelectorOpen = ref(false)
const customerQuery = ref('')
const productSelectorOpen = ref(false)
const productQuery = ref('')
const productCategory = ref<EmployeeOrderProductCategory>('all')
const editingItemKey = ref('')
const customerEditorOpen = ref(false)
const editingCustomerID = ref(0)
const editingCustomerMode = ref<'create' | 'edit'>('create')
const draftRecord = ref<EmployeeOrderDraft | null>(null)
const canCreateCustomer = computed(() => session.permissions.includes('customers.read')
  && session.permissions.includes('customers.write'))
let customerEditorIntentSequence = 0
let customerContextLoadPromise: Promise<boolean> | undefined

function emptyShippingSnapshot(): EmployeeOrderShippingSnapshot {
  return {
    receiver_name: '',
    receiver_phone: '',
    receiver_address: '',
    receiver_company: '',
  }
}

const shippingBaseline = ref<EmployeeOrderShippingSnapshot>(emptyShippingSnapshot())

function createOrderForm(): EmployeeOrderDraftPayload {
  return {
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
    items: [createEmployeeOrderItem()],
  }
}

const form = ref<EmployeeOrderDraftPayload>(createOrderForm())

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
const editingItem = computed(() => form.value.items.find((item) => item.key === editingItemKey.value))
const orderItemsTotal = computed(() => employeeOrderItemsTotal(form.value.items))

function categoryLabel(family: EmployeeOrderProductFamily): string {
  const category = employeeOrderProductCategory(family)
  return employeeOrderProductCategories.find((row) => row.key === category)?.label || '未分类'
}

function currentShippingSnapshot(): EmployeeOrderShippingSnapshot {
  return {
    receiver_name: form.value.receiver_name,
    receiver_phone: form.value.receiver_phone,
    receiver_address: form.value.receiver_address,
    receiver_company: form.value.receiver_company,
  }
}

function applyCustomerShipping(customer: EmployeeOrderCustomer) {
  const defaults = customerShippingDefaults(customer)
  Object.assign(form.value, defaults)
  shippingBaseline.value = { ...defaults }
}

function familyForItem(item: EmployeeOrderDraftItem): EmployeeOrderProductFamily | undefined {
  return productFamilies.value.find((family) => employeeOrderProductFamilyKey(family) === item.product_family_key)
}

function specLabelsForItem(item: EmployeeOrderDraftItem): string[] {
  return familyForItem(item)?.specs.map(productSpecLabel) || []
}

function selectedSpecIndexForItem(item: EmployeeOrderDraftItem): number {
  const family = familyForItem(item)
  return Math.max(0, family?.specs.findIndex(
    (spec) => Number(spec.product_id || spec.sku_id) === Number(item.product_id),
  ) ?? 0)
}

function displayedSalesUnit(item: EmployeeOrderDraftItem): string {
  return salesUnitLabel(item.sales_unit)
}

function upsertOrderCustomer(customer: EmployeeCustomer) {
  if (!formData.value) return
  const index = formData.value.customers.findIndex((row) => Number(row.id) === Number(customer.id))
  if (index >= 0) {
    const current = formData.value.customers[index]
    formData.value.customers.splice(index, 1, {
      ...current,
      ...customer,
      can_maintain: customer.can_maintain ?? current.can_maintain,
    })
  } else {
    formData.value.customers.unshift({ ...customer, can_maintain: customer.can_maintain ?? true })
  }
}

function openCustomerSelector() {
  if (loading.value || !formData.value) return
  customerEditorIntentSequence += 1
  customerQuery.value = ''
  customerSelectorOpen.value = true
}

function closeCustomerSelector() {
  customerEditorIntentSequence += 1
  customerSelectorOpen.value = false
}

function chooseCustomer(customer: EmployeeOrderCustomer) {
  const selectedBefore = form.value.items.filter((item) => item.product_id > 0).length
  form.value.customer_id = Number(customer.id)
  applyCustomerShipping(customer)
  if (Number(customer.default_source_id || 0) > 0) form.value.source_id = Number(customer.default_source_id)
  if (Number(customer.default_order_type_id || 0) > 0) form.value.order_type_id = Number(customer.default_order_type_id)
  form.value.items = revalidateEmployeeOrderItems(
    form.value.items,
    formData.value?.product_families || [],
    form.value.customer_id,
  )
  const selectedAfter = form.value.items.filter((item) => item.product_id > 0).length
  if (selectedAfter < selectedBefore) {
    uni.showToast({ title: '部分商品不适用于该客户，已清空', icon: 'none' })
  }
  closeCustomerSelector()
}

async function ensureCustomerContext(): Promise<boolean> {
  if (customerContext.value) return true
  if (!customerContextLoadPromise) {
    customerContextLoadPromise = (async () => {
      try {
        customerContext.value = await fetchEmployeeCustomers(session.token)
        return true
      } catch (cause) {
        uni.showToast({ title: cause instanceof Error ? cause.message : '客户维护数据加载失败', icon: 'none' })
        return false
      }
    })()
  }
  const pending = customerContextLoadPromise
  try {
    return await pending
  } finally {
    if (customerContextLoadPromise === pending) customerContextLoadPromise = undefined
  }
}

async function openCustomerCreate() {
  const intentSequence = ++customerEditorIntentSequence
  if (!await ensureCustomerContext()) return
  if (intentSequence !== customerEditorIntentSequence) return
  editingCustomerMode.value = 'create'
  editingCustomerID.value = 0
  customerSelectorOpen.value = false
  customerEditorOpen.value = true
}

async function openSelectedCustomerEdit() {
  const intentSequence = ++customerEditorIntentSequence
  const targetCustomerID = Number(selectedCustomer.value?.id || 0)
  if (targetCustomerID <= 0 || !selectedCustomer.value?.can_maintain) return
  if (!await ensureCustomerContext()) return
  if (intentSequence !== customerEditorIntentSequence) return
  if (Number(selectedCustomer.value?.id || 0) !== targetCustomerID || !selectedCustomer.value?.can_maintain) return
  editingCustomerMode.value = 'edit'
  editingCustomerID.value = targetCustomerID
  customerEditorOpen.value = true
}

function closeCustomerEditor() {
  customerEditorIntentSequence += 1
  customerEditorOpen.value = false
}

function customerSaved(customer: EmployeeCustomer) {
  if (customer.active === false) {
    const customerIndex = formData.value?.customers.findIndex((row) => Number(row.id) === Number(customer.id)) ?? -1
    if (customerIndex >= 0) formData.value?.customers.splice(customerIndex, 1)
    if (Number(form.value.customer_id) === Number(customer.id)) {
      form.value.customer_id = 0
      form.value.source_id = 0
      form.value.order_type_id = 0
      shippingBaseline.value = emptyShippingSnapshot()
      form.value.items = preserveEmployeeOrderDraftItemsForMissingCustomer(form.value.items)
    }
    uni.showToast({ title: '客户已停用，请重新选择启用客户', icon: 'none' })
    return
  }
  upsertOrderCustomer(customer)
  if (editingCustomerMode.value === 'create') {
    chooseCustomer(customer)
    uni.showToast({ title: '客户已新增并选中', icon: 'success' })
    return
  }
  if (Number(form.value.customer_id) !== Number(customer.id)) return
  if (Number(customer.default_source_id || 0) > 0) form.value.source_id = Number(customer.default_source_id)
  if (Number(customer.default_order_type_id || 0) > 0) form.value.order_type_id = Number(customer.default_order_type_id)

  if (!employeeOrderShippingChanged(currentShippingSnapshot(), shippingBaseline.value)) {
    applyCustomerShipping(customer)
    return
  }
  uni.showModal({
    title: '客户资料已更新',
    content: '本单收货资料已手动修改，是否同步客户最新收货资料？',
    confirmText: '同步',
    cancelText: '保留本单',
    success: (result) => {
      if (result.confirm) applyCustomerShipping(customer)
    },
  })
}

function addItem() {
  form.value.items.push(createEmployeeOrderItem())
}

function removeItem(index: number) {
  if (form.value.items.length === 1) {
    form.value.items.splice(0, 1, createEmployeeOrderItem(form.value.items[0]?.key))
    return
  }
  form.value.items.splice(index, 1)
}

function openProductSelector(itemKey: string) {
  if (!form.value.customer_id) {
    uni.showToast({ title: '请先选择客户', icon: 'none' })
    return
  }
  if (loading.value || !formData.value) return
  editingItemKey.value = itemKey
  productQuery.value = ''
  productCategory.value = 'all'
  productSelectorOpen.value = true
}

function closeProductSelector() {
  productSelectorOpen.value = false
  editingItemKey.value = ''
}

function applySpec(family: EmployeeOrderProductFamily, spec: EmployeeOrderProductSpec, item?: EmployeeOrderDraftItem) {
  const target = item || editingItem.value
  if (!target) return
  Object.assign(target, employeeOrderItemFromSpec(target, family, spec))
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

function chooseSpec(item: EmployeeOrderDraftItem, event: { detail?: { value?: number | string } }) {
  const family = familyForItem(item)
  const spec = family?.specs[Number(event.detail?.value || 0)]
  if (family && spec) applySpec(family, spec, item)
}

function setProductCategory(category: EmployeeOrderProductCategory) {
  productCategory.value = category
}

function goToLogin() {
  session.clearSession()
  uni.reLaunch({ url: '/pages/login/login' })
}

function normalizeDraftItems(items: EmployeeOrderDraftItem[] | undefined): EmployeeOrderDraftItem[] {
  if (!Array.isArray(items) || !items.length) return [createEmployeeOrderItem()]
  return items.map((item, index) => ({
    ...createEmployeeOrderItem(String(item?.key || `draft-${index + 1}`)),
    ...item,
    key: String(item?.key || `draft-${index + 1}`),
    qty: item?.qty == null ? 1 : Number(item.qty),
    unit_price: Number(item?.unit_price || 0),
  }))
}

function restoreDraft(draft: EmployeeOrderDraft) {
  const payload = draft.payload || ({} as EmployeeOrderDraftPayload)
  const customer = formData.value?.customers.find((row) => Number(row.id) === Number(payload.customer_id || 0))
  form.value = {
    ...createOrderForm(),
    ...payload,
    customer_id: customer ? Number(payload.customer_id) : 0,
    items: normalizeDraftItems(payload.items),
  }
  if (customer) {
    shippingBaseline.value = customerShippingDefaults(customer)
    form.value.items = revalidateEmployeeOrderItems(
      form.value.items,
      formData.value?.product_families || [],
      form.value.customer_id,
      { preserveUnitPrice: true, preserveUnavailable: true },
    )
  } else {
    shippingBaseline.value = emptyShippingSnapshot()
    form.value.items = preserveEmployeeOrderDraftItemsForMissingCustomer(form.value.items)
  }
  draftRecord.value = draft
}

async function loadDraft() {
  const response = await fetchEmployeeOrderDraft(session.token)
  if (response.draft) restoreDraft(response.draft)
}

function applyDefaultOptions() {
  form.value.source_id ||= formData.value?.sources[0]?.id || 0
  form.value.order_type_id ||= formData.value?.order_types[0]?.id || 0
  form.value.pay_status_id ||= formData.value?.pay_statuses[0]?.id || 0
  form.value.ship_status_id ||= formData.value?.ship_statuses[0]?.id || 0
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
    applyDefaultOptions()
    await loadDraft()
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

async function saveDraft() {
  if (clearingDraft.value) return
  savingDraft.value = true
  try {
    const response = await saveEmployeeOrderDraft(session.token, {
      ...form.value,
      items: form.value.items.map((item) => ({ ...item })),
    })
    draftRecord.value = response.draft
    uni.showToast({ title: '草稿已保存', icon: 'success' })
  } catch (cause) {
    uni.showToast({ title: cause instanceof Error ? cause.message : '草稿保存失败', icon: 'none' })
  } finally {
    savingDraft.value = false
  }
}

function clearDraft() {
  if (saving.value || savingDraft.value || clearingDraft.value) return
  uni.showModal({
    title: '清除草稿',
    content: '确定清除服务器上的当前录单草稿吗？',
    confirmText: '清除',
    confirmColor: '#a7352a',
    success: async (result) => {
      if (!result.confirm) return
      clearingDraft.value = true
      try {
        await deleteEmployeeOrderDraft(session.token)
        resetAfterSubmit()
        uni.showToast({ title: '草稿已清除', icon: 'success' })
      } catch (cause) {
        uni.showToast({ title: cause instanceof Error ? cause.message : '草稿清除失败', icon: 'none' })
      } finally {
        clearingDraft.value = false
      }
    },
  })
}

function validateOrder(): boolean {
  if (!form.value.customer_id) {
    uni.showToast({ title: '请选择客户', icon: 'none' })
    return false
  }
  let completeCount = 0
  for (let index = 0; index < form.value.items.length; index += 1) {
    const item = form.value.items[index]
    const blank = !item.product_id && !item.product_name && !item.spec_label
    if (blank) continue
    if (!item.product_id || !item.spec_label) {
      uni.showToast({ title: `第${index + 1}行请选择商品和规格`, icon: 'none' })
      return false
    }
    if (item.validation_error) {
      uni.showToast({ title: `第${index + 1}行${item.validation_error}`, icon: 'none' })
      return false
    }
    if (Number(item.qty) <= 0) {
      uni.showToast({ title: `第${index + 1}行数量不正确`, icon: 'none' })
      return false
    }
    if (!Number.isFinite(Number(item.unit_price)) || Number(item.unit_price) <= 0) {
      uni.showToast({ title: `第${index + 1}行销售单价必须大于 0`, icon: 'none' })
      return false
    }
    completeCount += 1
  }
  if (!completeCount) {
    uni.showToast({ title: '请至少添加一个商品', icon: 'none' })
    return false
  }
  return true
}

function resetAfterSubmit() {
  form.value = createOrderForm()
  shippingBaseline.value = emptyShippingSnapshot()
  draftRecord.value = null
  applyDefaultOptions()
}

async function submit() {
  if (clearingDraft.value) return
  if (!validateOrder()) return
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
      items: buildEmployeeOrderItemsPayload(form.value.items),
    })
    // The backend creates the order and clears the employee draft atomically.
    resetAfterSubmit()
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
    <EnvironmentBadge />
    <view class="panel">
      <view class="title-row">
        <text class="title">新建销售订单</text>
        <text v-if="draftRecord" class="draft-meta">草稿 {{ draftRecord.updated_at }}</text>
      </view>

      <view v-if="loading" class="status-card">
        <text>正在加载客户和商品...</text>
      </view>
      <view v-else-if="loadError" class="status-card error-card">
        <text>{{ loadError }}</text>
        <button v-if="authExpired" class="status-action" @tap="goToLogin">重新登录</button>
        <button v-else class="status-action" @tap="loadForm">重试</button>
      </view>

      <template v-else-if="formData">
      <text class="label">订单日期</text>
      <picker mode="date" :value="form.order_date" @change="form.order_date = ($event.detail as any).value">
        <view class="field selector-field">{{ form.order_date }}</view>
      </picker>

      <text class="label">客户</text>
      <view class="customer-field-row">
        <view
          class="field selector-field customer-field"
          :class="{ muted: loading || !formData }"
          @tap="openCustomerSelector"
        >
          <text>{{ selectedCustomer?.name || '搜索并选择客户 *' }}</text>
          <text class="chevron">›</text>
        </view>
        <button
          v-if="selectedCustomer?.can_maintain"
          class="compact-action"
          @tap="openSelectedCustomerEdit"
        >
          维护
        </button>
      </view>

      <view class="section-head">
        <text class="section-title">商品明细</text>
        <button class="add-item" @tap="addItem">新增商品</button>
      </view>

      <view v-for="(item, index) in form.items" :key="item.key" class="item-card">
        <view class="item-head">
          <text class="item-title">商品 {{ index + 1 }}</text>
          <button class="remove-item" @tap="removeItem(index)">删除本行</button>
        </view>

        <text class="label">商品</text>
        <view
          class="field selector-field"
          :class="{ muted: !form.customer_id || loading || !formData }"
          @tap="openProductSelector(item.key)"
        >
          <text>{{ item.product_name || (form.customer_id ? '搜索并选择商品 *' : '请先选择客户') }}</text>
          <text class="chevron">›</text>
        </view>
        <text v-if="item.validation_error" class="item-error">{{ item.validation_error }}</text>

        <text class="label">规格</text>
        <picker
          mode="selector"
          :range="specLabelsForItem(item)"
          :value="selectedSpecIndexForItem(item)"
          :disabled="!familyForItem(item)"
          @change="chooseSpec(item, $event)"
        >
          <view class="field selector-field" :class="{ muted: !familyForItem(item) }">
            <text>{{ item.spec_label || (familyForItem(item) ? '选择该商品的规格 *' : '请先选择商品') }}</text>
            <text class="chevron">›</text>
          </view>
        </picker>

        <text class="label">数量（{{ displayedSalesUnit(item) }}）</text>
        <view class="input-with-unit">
          <input v-model="item.qty" type="number" class="field" :placeholder="`填写数量（${displayedSalesUnit(item)}） *`" />
          <text class="unit-suffix">{{ displayedSalesUnit(item) }}</text>
        </view>

        <text class="label">销售单价（元/{{ displayedSalesUnit(item) }}）*</text>
        <view class="input-with-unit">
          <input v-model="item.unit_price" type="digit" class="field" :placeholder="`填写每${displayedSalesUnit(item)}单价`" />
          <text class="unit-suffix">元/{{ displayedSalesUnit(item) }}</text>
        </view>
      </view>

      <view class="order-total">
        <text>商品估算合计</text>
        <text class="order-total-value">¥{{ orderItemsTotal.toFixed(2) }}</text>
      </view>

      <view class="section-title standalone">收货信息</view>
      <text class="hint">选择客户后自动带入，可按本次订单修改</text>
      <text class="label">收货人</text>
      <input v-model="form.receiver_name" class="field" placeholder="收货人" />
      <text class="label">联系电话</text>
      <input v-model="form.receiver_phone" class="field" placeholder="联系电话（可含区号或分机）" />
      <text class="label">收货单位</text>
      <input v-model="form.receiver_company" class="field" placeholder="收货单位" />
      <text class="label">收货地址</text>
      <textarea v-model="form.receiver_address" class="field area" placeholder="收货地址" />

      <text class="label">备注</text>
      <textarea v-model="form.notes" class="field area" placeholder="备注" />
      <view class="form-actions">
        <button
          v-if="draftRecord"
          class="clear-draft-button"
          :loading="clearingDraft"
          :disabled="savingDraft || saving || clearingDraft"
          @tap="clearDraft"
        >
          清除草稿
        </button>
        <button
          class="draft-button"
          :loading="savingDraft"
          :disabled="savingDraft || saving || clearingDraft || loading || Boolean(loadError)"
          @tap="saveDraft"
        >
          保存草稿
        </button>
        <button
          class="submit"
          :loading="saving"
          :disabled="saving || savingDraft || clearingDraft || loading || Boolean(loadError)"
          @tap="submit"
        >
          提交订单
        </button>
      </view>
      </template>
    </view>

    <view v-if="customerSelectorOpen" class="overlay" @tap.self="closeCustomerSelector">
      <view class="select-sheet" @tap.stop>
        <view class="sheet-head">
          <text class="sheet-title">选择客户</text>
          <view class="sheet-actions">
            <text v-if="canCreateCustomer" class="sheet-create" @tap="openCustomerCreate">新增客户</text>
            <text class="sheet-close" @tap="closeCustomerSelector">关闭</text>
          </view>
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

    <EmployeeCustomerEditor
      :visible="customerEditorOpen"
      :token="session.token"
      :customer-id="editingCustomerID"
      :context="customerContext"
      @close="closeCustomerEditor"
      @saved="customerSaved"
    />
  </view>
</template>

<style scoped>
.page { min-height: 100vh; padding: 28rpx; background: #f5f7f6; box-sizing: border-box; }
.panel { padding: 28rpx; background: #fff; border-radius: 18rpx; }
.title-row { display: flex; align-items: center; justify-content: space-between; gap: 16rpx; margin-bottom: 28rpx; }
.title { font-size: 36rpx; font-weight: 800; }
.draft-meta { color: #718078; font-size: 21rpx; }
.section-head { display: flex; align-items: center; justify-content: space-between; margin: 30rpx 0 18rpx; padding-top: 24rpx; border-top: 1rpx solid #edf1ee; }
.section-title { font-size: 30rpx; font-weight: 750; }
.section-title.standalone { margin: 30rpx 0 18rpx; padding-top: 24rpx; border-top: 1rpx solid #edf1ee; }
.label { display: block; margin: 0 0 8rpx 4rpx; color: #42524a; font-size: 25rpx; font-weight: 650; }
.hint { display: block; margin: -8rpx 0 20rpx; color: #718078; font-size: 23rpx; }
.field { width: 100%; min-height: 82rpx; margin-bottom: 18rpx; padding: 20rpx; border: 1rpx solid #dfe7e2; border-radius: 12rpx; box-sizing: border-box; background: #fff; }
.selector-field { display: flex; align-items: center; justify-content: space-between; gap: 16rpx; }
.customer-field-row { display: flex; align-items: stretch; gap: 12rpx; }
.customer-field { flex: 1; min-width: 0; }
.compact-action { flex: 0 0 auto; width: 110rpx; margin: 0 0 18rpx; padding: 0; min-height: 82rpx; line-height: 82rpx; border: 1rpx solid #28624a; background: #fff; color: #28624a; font-size: 24rpx; }
.chevron { color: #7b8780; font-size: 38rpx; line-height: 1; }
.muted { color: #9aa59f; background: #f7f9f8; }
.area { height: 130rpx; }
.input-with-unit { position: relative; }
.input-with-unit .field { padding-right: 150rpx; }
.unit-suffix { position: absolute; top: 25rpx; right: 22rpx; color: #65736b; font-size: 24rpx; }
.item-card { margin-bottom: 18rpx; padding: 22rpx; border: 1rpx solid #dce7e0; border-radius: 16rpx; background: #fbfdfc; }
.item-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 18rpx; }
.item-title { color: #203b2e; font-size: 27rpx; font-weight: 750; }
.add-item, .remove-item { margin: 0; padding: 0 20rpx; min-height: 58rpx; line-height: 58rpx; border: 1rpx solid #28624a; background: #fff; color: #28624a; font-size: 23rpx; }
.remove-item { border-color: #d7aaa3; color: #9a3e34; }
.item-error { display: block; margin: -8rpx 0 16rpx; color: #a7352a; font-size: 23rpx; }
.order-total { display: flex; align-items: center; justify-content: space-between; margin: 10rpx 0 24rpx; padding: 22rpx; border-radius: 12rpx; background: #edf5f0; color: #355345; font-size: 27rpx; font-weight: 700; }
.order-total-value { color: #1d5b3f; font-size: 32rpx; font-weight: 850; }
.form-actions { display: flex; flex-wrap: wrap; gap: 14rpx; margin-top: 12rpx; }
.form-actions button { margin: 0; }
.form-actions .clear-draft-button { flex: 1 1 100%; border: 1rpx solid #d7aaa3; background: #fff; color: #9a3e34; }
.form-actions .draft-button, .form-actions .submit { flex: 1 1 0; }
.draft-button { border: 1rpx solid #28624a; background: #fff; color: #28624a; }
.submit { background: #28624a; color: #fff; }
.status-card { display: flex; align-items: center; justify-content: space-between; gap: 16rpx; margin-bottom: 22rpx; padding: 18rpx 20rpx; border-radius: 12rpx; background: #eef5f1; color: #355345; font-size: 24rpx; }
.error-card { background: #fff2f0; color: #a7352a; }
.status-action { flex: 0 0 auto; margin: 0; padding: 0 22rpx; min-height: 58rpx; line-height: 58rpx; background: #28624a; color: #fff; font-size: 24rpx; }
.overlay { position: fixed; inset: 0; z-index: 100; display: flex; align-items: flex-end; background: rgba(16, 28, 22, .48); }
.select-sheet { width: 100%; max-height: 78vh; padding: 28rpx 28rpx calc(24rpx + env(safe-area-inset-bottom)); border-radius: 24rpx 24rpx 0 0; box-sizing: border-box; background: #fff; }
.product-sheet { max-height: 86vh; }
.sheet-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 20rpx; }
.sheet-actions { display: flex; align-items: center; gap: 12rpx; }
.sheet-title { color: #1e362a; font-size: 32rpx; font-weight: 800; }
.sheet-create, .sheet-close { padding: 12rpx; color: #28624a; font-size: 26rpx; }
.sheet-close { color: #718078; }
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
