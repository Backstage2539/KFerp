<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onShow } from '@dcloudio/uni-app'
import EmployeeCustomerEditor from '../../components/EmployeeCustomerEditor.vue'
import ProductFamilyPickerSheet from '../../components/ProductFamilyPickerSheet.vue'
import {
  createEmployeeOrder,
  deleteEmployeeOrderDraft,
  fetchEmployeeCustomers,
  fetchEmployeeOrderDraft,
  fetchEmployeeOrderDetail,
  fetchEmployeeOrderForm,
  saveEmployeeOrderDraft,
  updateEmployeeOrder,
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
import { isAuthenticationExpiredRequestError, MiniRequestError } from '../../api/client'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import {
  buildEmployeeOrderItemsPayload,
  applyEmployeeOrderQuantityChange,
  createEmployeeOrderItem,
  customerProductFamilies,
  customerShippingDefaults,
  defaultProductSpec,
  employeeOrderItemForSpecSelection,
  employeeOrderItemsTotal,
  employeeOrderGrandTotal,
  employeeOrderItemDiscountAmount,
  employeeOrderEditableOrderDiscount,
  employeeOrderOutsourceTotal,
  employeeOrderProductFamilyKey,
  employeeOrderShippingChanged,
  filterEmployeeOrderCustomers,
  hydrateEmployeeOrderEditItems,
  isEmployeeOrderNonNegativeMoney,
  productSpecLabel,
  preserveEmployeeOrderDraftItemsForMissingCustomer,
  revalidateEmployeeOrderItems,
  salesUnitLabel,
  shanghaiToday,
  type EmployeeOrderShippingSnapshot,
} from '../../utils/employeeOrder'
import { useSessionStore } from '../../stores/session'

const session = useSessionStore()
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
const formData = ref<EmployeeOrderForm>()
const customerContext = ref<EmployeeCustomersResponse>()
const loading = ref(false)
const productCatalogLoading = ref(false)
const saving = ref(false)
const savingDraft = ref(false)
const clearingDraft = ref(false)
const loadError = ref('')
const authExpired = ref(false)
const customerSelectorOpen = ref(false)
const customerQuery = ref('')
const productSelectorOpen = ref(false)
const editingItemKey = ref('')
const customerEditorOpen = ref(false)
const editingCustomerID = ref(0)
const editingCustomerMode = ref<'create' | 'edit'>('create')
const quantityInputs = ref<Record<string, string>>({})
const draftRecord = ref<EmployeeOrderDraft | null>(null)
const editOrderID = ref(0)
const preservedOutsourceTotal = ref(0)
const preservedRoundToInt = ref(false)
const isEditMode = computed(() => editOrderID.value > 0)
const canCreateCustomer = computed(() => session.permissions.includes('customers.read')
  && session.permissions.includes('customers.write'))
let customerEditorIntentSequence = 0
let customerContextLoadPromise: Promise<boolean> | undefined
let productCatalogLoadSequence = 0
let employeeOrderPageHasShown = false

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
    shipping_amount: 0,
    discount_amount: 0,
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
const editingItem = computed(() => form.value.items.find((item) => item.key === editingItemKey.value))
const orderItemsTotal = computed(() => employeeOrderItemsTotal(form.value.items))
const orderGrandTotal = computed(() => employeeOrderGrandTotal(
  form.value.items,
  Number(form.value.shipping_amount || 0),
  Number(form.value.discount_amount || 0),
  preservedOutsourceTotal.value,
  preservedRoundToInt.value,
))

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
    (spec) => item.migration_state === 'cutover'
      ? Number(spec.bom_spec_id || 0) === Number(item.bom_spec_id || 0)
      : Number(spec.product_id || spec.sku_id) === Number(item.product_id),
  ) ?? 0)
}

function displayedSalesUnit(item: EmployeeOrderDraftItem): string {
  return salesUnitLabel(item.sales_unit)
}

function hasPendingQuantity(item: EmployeeOrderDraftItem): boolean {
  return Object.prototype.hasOwnProperty.call(quantityInputs.value, item.key)
}

function quantityInputValue(item: EmployeeOrderDraftItem): number | string {
  if (hasPendingQuantity(item)) return quantityInputs.value[item.key]
  return Number(item.qty) > 0 ? item.qty : ''
}

function unitPriceInputValue(item: EmployeeOrderDraftItem): number | string {
  if (hasPendingQuantity(item)) return ''
  return Number(item.unit_price) > 0 ? item.unit_price : ''
}

function clearPendingQuantity(item: EmployeeOrderDraftItem) {
  const next = { ...quantityInputs.value }
  delete next[item.key]
  quantityInputs.value = next
}

function employeeOrderInputValue(event: unknown): number | string {
  const detail = (event as { detail?: unknown } | undefined)?.detail
  if (detail && typeof detail === 'object' && 'value' in detail) {
    const value = (detail as { value?: unknown }).value
    return typeof value === 'number' || typeof value === 'string' ? value : ''
  }
  return ''
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
  if (isEditMode.value || loading.value || !formData.value) return
  customerEditorIntentSequence += 1
  customerQuery.value = ''
  customerSelectorOpen.value = true
}

function closeCustomerSelector() {
  customerEditorIntentSequence += 1
  customerSelectorOpen.value = false
}

async function loadCustomerProductCatalog(targetCustomerID: number): Promise<boolean> {
  const sequence = ++productCatalogLoadSequence
  productCatalogLoading.value = true
  try {
    const data = await fetchEmployeeOrderForm(session.token, targetCustomerID)
    if (sequence !== productCatalogLoadSequence || Number(form.value.customer_id) !== targetCustomerID) return false
    if (!formData.value) return false
    formData.value = {
      ...formData.value,
      ...data,
      customers: data.customers || formData.value.customers || [],
      sources: data.sources || formData.value.sources || [],
      order_types: data.order_types || formData.value.order_types || [],
      pay_statuses: data.pay_statuses || formData.value.pay_statuses || [],
      ship_statuses: data.ship_statuses || formData.value.ship_statuses || [],
      product_families: data.product_families || [],
    }
    return true
  } catch (cause) {
    if (sequence !== productCatalogLoadSequence || Number(form.value.customer_id) !== targetCustomerID) return false
    if (formData.value) formData.value.product_families = []
    uni.showToast({ title: cause instanceof Error ? cause.message : '客户可售商品加载失败', icon: 'none' })
    return false
  } finally {
    if (sequence === productCatalogLoadSequence) productCatalogLoading.value = false
  }
}

async function chooseCustomer(customer: EmployeeOrderCustomer) {
  if (isEditMode.value) return
  const selectedBefore = form.value.items.filter((item) => item.product_id > 0).length
  const customerChanged = Number(form.value.customer_id || 0) !== Number(customer.id)
  form.value.customer_id = Number(customer.id)
  if (customerChanged) {
    form.value.items = [createEmployeeOrderItem()]
    quantityInputs.value = {}
  }
  applyCustomerShipping(customer)
  if (Number(customer.default_source_id || 0) > 0) form.value.source_id = Number(customer.default_source_id)
  if (Number(customer.default_order_type_id || 0) > 0) form.value.order_type_id = Number(customer.default_order_type_id)
  closeCustomerSelector()
  const catalogLoaded = await loadCustomerProductCatalog(Number(customer.id))
  if (Number(form.value.customer_id) !== Number(customer.id)) return
  form.value.items = revalidateEmployeeOrderItems(
    form.value.items,
    formData.value?.product_families || [],
    form.value.customer_id,
    catalogLoaded ? {} : { preserveUnavailable: true, preserveUnitPrice: true },
  )
  const selectedAfter = form.value.items.filter((item) => item.product_id > 0).length
  if (selectedAfter < selectedBefore) {
    uni.showToast({ title: '部分商品不适用于该客户，已清空', icon: 'none' })
  }
}

async function refreshCurrentProductCatalog(): Promise<boolean> {
  const customerID = Number(form.value.customer_id || 0)
  if (customerID <= 0) return false
  const loaded = await loadCustomerProductCatalog(customerID)
  if (Number(form.value.customer_id || 0) !== customerID) return false
  form.value.items = revalidateEmployeeOrderItems(
    form.value.items,
    formData.value?.product_families || [],
    customerID,
    { preserveUnavailable: true, preserveManualPrice: true },
  )
  return loaded
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
    void chooseCustomer(customer)
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
  if (form.value.items[index]) clearPendingQuantity(form.value.items[index])
  if (form.value.items.length === 1) {
    form.value.items.splice(0, 1, createEmployeeOrderItem(form.value.items[0]?.key))
    return
  }
  form.value.items.splice(index, 1)
}

async function openProductSelector(itemKey: string) {
  if (!form.value.customer_id) {
    uni.showToast({ title: '请先选择客户', icon: 'none' })
    return
  }
  if (productCatalogLoading.value || loading.value || !formData.value) return
  if (!await refreshCurrentProductCatalog()) return
  if (!form.value.items.some((item) => item.key === itemKey)) return
  editingItemKey.value = itemKey
  productSelectorOpen.value = true
}

function closeProductSelector() {
  productSelectorOpen.value = false
  editingItemKey.value = ''
}

function applySpec(family: EmployeeOrderProductFamily, spec: EmployeeOrderProductSpec, item?: EmployeeOrderDraftItem) {
  const target = item || editingItem.value
  if (!target) return
  clearPendingQuantity(target)
  Object.assign(target, employeeOrderItemForSpecSelection(target, family, spec))
}

function markPriceOverride(item: EmployeeOrderDraftItem, event: unknown) {
  item.unit_price = Number(employeeOrderInputValue(event) || 0)
  item.price_override = true
  item.discount_amount = employeeOrderItemDiscountAmount(item)
}

function quantityChanged(item: EmployeeOrderDraftItem, event: unknown) {
  const rawQuantity = String(employeeOrderInputValue(event) ?? '')
  quantityInputs.value = { ...quantityInputs.value, [item.key]: rawQuantity }
  const family = familyForItem(item)
  const result = applyEmployeeOrderQuantityChange(item, family, rawQuantity)
  if (!result.accepted) return
  Object.assign(item, result.item)
  clearPendingQuantity(item)
}

function quantityBlurred(item: EmployeeOrderDraftItem) {
  if (!hasPendingQuantity(item)) return
  const family = familyForItem(item)
  const result = applyEmployeeOrderQuantityChange(item, family, quantityInputs.value[item.key])
  clearPendingQuantity(item)
  if (!result.accepted) {
    uni.showToast({ title: result.error, icon: 'none' })
    return
  }
  Object.assign(item, result.item)
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

async function restoreDraft(draft: EmployeeOrderDraft) {
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
    await loadCustomerProductCatalog(Number(customer.id))
    form.value.items = revalidateEmployeeOrderItems(
      form.value.items,
      formData.value?.product_families || [],
      form.value.customer_id,
      { preserveManualPrice: true, preserveUnavailable: true },
    )
  } else {
    shippingBaseline.value = emptyShippingSnapshot()
    form.value.items = preserveEmployeeOrderDraftItemsForMissingCustomer(form.value.items)
  }
  draftRecord.value = draft
}

async function loadDraft() {
  if (isEditMode.value) return
  const response = await fetchEmployeeOrderDraft(session.token)
  if (response.draft) await restoreDraft(response.draft)
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
  preservedOutsourceTotal.value = 0
  preservedRoundToInt.value = false
  try {
    if (!session.token) throw new Error('登录已失效')
    const detailResponse = isEditMode.value
      ? await fetchEmployeeOrderDetail(session.token, editOrderID.value)
      : undefined
    if (detailResponse) {
      const allowed = detailResponse.can_edit ?? detailResponse.order.can_edit
      if (allowed === false) {
        throw new Error(detailResponse.edit_block_reason || detailResponse.order.edit_block_reason || '订单当前不能编辑')
      }
    }
    const targetCustomerID = Number(detailResponse?.order.customer_id || 0)
    const data = await fetchEmployeeOrderForm(session.token, targetCustomerID)
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
    if (detailResponse) {
      const detail = detailResponse.order
      const customer = formData.value.customers.find((row) => Number(row.id) === targetCustomerID)
      const editOrderTypeName = String(formData.value.order_types.find(
        (row) => Number(row.id) === Number(detail.order_type_id || 0),
      )?.name || '')
      const editRetailOrder = /零售|retail/i.test(editOrderTypeName)
      preservedOutsourceTotal.value = employeeOrderOutsourceTotal(detail)
      preservedRoundToInt.value = Boolean(detail.round_to_int)
      form.value = {
        ...createOrderForm(),
        edit_revision: String(detail.edit_revision || ''),
        order_date: String(detail.order_date || detail.document_date || shanghaiToday()),
        customer_id: targetCustomerID,
        source_id: Number(detail.source_id || 0),
        order_type_id: Number(detail.order_type_id || 0),
        pay_status_id: Number(detail.pay_status_id || 0),
        ship_status_id: Number(detail.ship_status_id || 0),
        receiver_name: String(detail.receiver_name || ''),
        receiver_phone: String(detail.receiver_phone || ''),
        receiver_address: String(detail.receiver_address || ''),
        receiver_company: String(detail.receiver_company || ''),
        shipping_amount: Number(detail.shipping_amount || detail.payment_shipping_amount || 0),
        discount_amount: employeeOrderEditableOrderDiscount(
          detail.discount_amount,
          detail.items || [],
          detail.order_discount_amount,
        ),
        notes: String(detail.notes || ''),
        items: hydrateEmployeeOrderEditItems(detail.items || [], formData.value.product_families, targetCustomerID, editRetailOrder),
      }
      shippingBaseline.value = customer ? customerShippingDefaults(customer) : currentShippingSnapshot()
      applyDefaultOptions()
    }
    if (!isEditMode.value) await loadDraft()
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
  if (isEditMode.value) return
  if (productCatalogLoading.value) return
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
  if (isEditMode.value) return
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
    if (item.migration_state === 'cutover' && Number(item.bom_spec_id || 0) <= 0) {
      uni.showToast({ title: `第${index + 1}行请选择 BOM 规格`, icon: 'none' })
      return false
    }
    if (item.validation_error) {
      uni.showToast({ title: `第${index + 1}行${item.validation_error}`, icon: 'none' })
      return false
    }
    if (!Number.isFinite(Number(item.qty)) || Number(item.qty) <= 0) {
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
  if (!isEmployeeOrderNonNegativeMoney(form.value.shipping_amount)
    || !isEmployeeOrderNonNegativeMoney(form.value.discount_amount)) {
    uni.showToast({ title: '运费和优惠必须是大于或等于 0 的有效数字', icon: 'none' })
    return false
  }
  return true
}

function resetAfterSubmit() {
  form.value = createOrderForm()
  quantityInputs.value = {}
  shippingBaseline.value = emptyShippingSnapshot()
  draftRecord.value = null
  applyDefaultOptions()
}

function returnToOrderDetail() {
  uni.navigateBack({
    delta: 1,
    fail: () => uni.redirectTo({
      url: `/pages/employee-order-detail/employee-order-detail?id=${editOrderID.value}`,
    }),
  })
}

function isPriceCatalogInvalidationError(cause: unknown): boolean {
  return cause instanceof MiniRequestError
    && (cause.statusCode === 400 || cause.statusCode === 409)
    && /当前价格表|价格表已更新|价格目录/.test(cause.message)
}

async function submit() {
  if (clearingDraft.value) return
  if (productCatalogLoading.value) return
  if (!validateOrder()) return
  saving.value = true
  try {
    const payload = {
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
      shipping_amount: Number(form.value.shipping_amount || 0),
      discount_amount: Number(form.value.discount_amount || 0),
      notes: form.value.notes,
      items: buildEmployeeOrderItemsPayload(form.value.items),
    }
    if (isEditMode.value) {
      await updateEmployeeOrder(session.token, editOrderID.value, {
        ...payload,
        edit_revision: String(form.value.edit_revision || ''),
      })
      uni.showToast({ title: '订单已更新', icon: 'success' })
      returnToOrderDetail()
      return
    }
    const result = await createEmployeeOrder(session.token, payload)
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
    if (isPriceCatalogInvalidationError(cause)) {
      if (await refreshCurrentProductCatalog()) {
        uni.showToast({ title: '价格目录已更新，请检查商品和价格后重试', icon: 'none' })
      }
      return
    }
    if (isEditMode.value && cause instanceof MiniRequestError && cause.statusCode === 409) {
      uni.showModal({
        title: '订单已不能编辑',
        content: cause.message || '订单状态已经变化，请返回详情查看最新状态',
        showCancel: false,
        success: returnToOrderDetail,
      })
      return
    }
    uni.showToast({ title: cause instanceof Error ? cause.message : (isEditMode.value ? '订单更新失败' : '录单失败'), icon: 'none' })
  } finally {
    saving.value = false
  }
}

onLoad((options) => {
  editOrderID.value = Number(options?.edit_id || 0)
  if (isEditMode.value) uni.setNavigationBarTitle({ title: '编辑销售订单' })
  void loadForm()
})

onShow(() => {
  if (!employeeOrderPageHasShown) {
    employeeOrderPageHasShown = true
    return
  }
  if (loading.value || productCatalogLoading.value || Number(form.value.customer_id || 0) <= 0) return
  void refreshCurrentProductCatalog()
})
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
    <view class="panel">
      <view class="title-row">
        <text class="title">{{ isEditMode ? '编辑销售订单' : '新建销售订单' }}</text>
        <text v-if="!isEditMode && draftRecord" class="draft-meta">草稿 {{ draftRecord.updated_at }}</text>
      </view>

      <view v-if="loading" class="status-card">
        <text>{{ isEditMode ? '正在加载订单和当前可售商品...' : '正在加载客户和商品...' }}</text>
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
          :class="{ muted: isEditMode || loading || !formData }"
          @tap="openCustomerSelector"
        >
          <text>{{ selectedCustomer?.name || '搜索并选择客户 *' }}</text>
          <text v-if="!isEditMode" class="chevron">›</text>
        </view>
        <button
          v-if="!isEditMode && selectedCustomer?.can_maintain"
          class="compact-action"
          @tap="openSelectedCustomerEdit"
        >
          维护
        </button>
      </view>

      <view class="section-head">
        <text class="section-title">商品明细</text>
      </view>

      <view v-for="(item, index) in form.items" :key="item.key" class="item-card">
        <view class="item-head">
          <text class="item-title">商品 {{ index + 1 }}</text>
          <button class="remove-item" @tap="removeItem(index)">删除本行</button>
        </view>

        <text class="label">商品</text>
        <view
          class="field selector-field"
          :class="{ muted: !form.customer_id || productCatalogLoading || loading || !formData }"
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
          :disabled="productCatalogLoading || !familyForItem(item)"
          @change="chooseSpec(item, $event)"
        >
          <view class="field selector-field" :class="{ muted: !familyForItem(item) }">
            <text>{{ item.spec_label || (familyForItem(item) ? '选择该商品的规格 *' : '请先选择商品') }}</text>
            <text class="chevron">›</text>
          </view>
        </picker>

        <text class="label">数量（{{ displayedSalesUnit(item) }}）</text>
        <view class="input-with-unit">
          <input :value="quantityInputValue(item)" type="number" class="field" :placeholder="`填写数量（${displayedSalesUnit(item)}） *`" @input="quantityChanged(item, $event)" @blur="quantityBlurred(item)" />
          <text class="unit-suffix">{{ displayedSalesUnit(item) }}</text>
        </view>

        <text class="label">销售单价（元/{{ displayedSalesUnit(item) }}）*</text>
        <view class="input-with-unit">
          <input :value="unitPriceInputValue(item)" type="digit" class="field" :disabled="hasPendingQuantity(item) || Number(item.qty) <= 0" :placeholder="Number(item.qty) > 0 && !hasPendingQuantity(item) ? `填写每${displayedSalesUnit(item)}单价` : '先填写数量后自动匹配'" @input="markPriceOverride(item, $event)" />
          <text class="unit-suffix">元/{{ displayedSalesUnit(item) }}</text>
        </view>
      </view>

      <button class="add-item add-item-after-list" @tap="addItem">新增商品</button>

      <view class="order-total">
        <text>商品估算合计</text>
        <text class="order-total-value">¥{{ orderItemsTotal.toFixed(2) }}</text>
      </view>

      <view class="fee-editor-grid">
        <view>
          <text class="label">运费（元）</text>
          <input v-model="form.shipping_amount" type="digit" class="field" placeholder="0.00" />
        </view>
        <view>
          <text class="label">优惠（元）</text>
          <input v-model="form.discount_amount" type="digit" class="field" placeholder="0.00" />
        </view>
      </view>
      <view v-if="isEditMode && (preservedOutsourceTotal > 0 || preservedRoundToInt)" class="preserved-pricing-hint">
        <text v-if="preservedOutsourceTotal > 0">原订单代加工费用（保留）：¥{{ preservedOutsourceTotal.toFixed(2) }}</text>
        <text v-if="preservedRoundToInt">原订单应收按原设置向下取整</text>
      </view>
      <view class="order-total grand-total">
        <text>订单应收合计</text>
        <text class="order-total-value">¥{{ orderGrandTotal.toFixed(2) }}</text>
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
        <view v-if="!isEditMode" class="draft-action-group">
          <button
            v-if="draftRecord"
            class="clear-draft-button"
            :loading="clearingDraft"
            :disabled="savingDraft || saving || clearingDraft || productCatalogLoading"
            @tap="clearDraft"
          >
            清除草稿
          </button>
          <button
            class="draft-button"
            :loading="savingDraft"
            :disabled="savingDraft || saving || clearingDraft || loading || productCatalogLoading || Boolean(loadError)"
            @tap="saveDraft"
          >
            保存草稿
          </button>
        </view>
        <button
          class="submit"
          :loading="saving"
          :disabled="saving || savingDraft || clearingDraft || loading || productCatalogLoading || Boolean(loadError)"
          @tap="submit"
        >
          {{ isEditMode ? '保存修改' : '提交订单' }}
        </button>
      </view>
      </template>
    </view>

    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter :revealed="pullUpBrandRevealed" />
    </view>

    <view v-if="customerSelectorOpen && !isEditMode" class="overlay" @tap.self="closeCustomerSelector">
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

    <ProductFamilyPickerSheet
      :visible="productSelectorOpen"
      :families="formData?.product_families || []"
      :customer-id="form.customer_id"
      :loading="productCatalogLoading"
      @close="closeProductSelector"
      @select="chooseProduct"
    />

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
.add-item-after-list { width: 100%; margin: 2rpx 0 20rpx; border-style: dashed; }
.remove-item { border-color: #d7aaa3; color: #9a3e34; }
.item-error { display: block; margin: -8rpx 0 16rpx; color: #a7352a; font-size: 23rpx; }
.order-total { display: flex; align-items: center; justify-content: space-between; margin: 10rpx 0 24rpx; padding: 22rpx; border-radius: 12rpx; background: #edf5f0; color: #355345; font-size: 27rpx; font-weight: 700; }
.order-total-value { color: #1d5b3f; font-size: 32rpx; font-weight: 850; }
.fee-editor-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16rpx; }
.grand-total { margin-top: 0; background: #e4f1e9; }
.preserved-pricing-hint { display: flex; flex-direction: column; gap: 6rpx; margin: -4rpx 0 18rpx; color: #718078; font-size: 22rpx; }
.form-actions { display: flex; flex-wrap: wrap; gap: 14rpx; margin-top: 12rpx; }
.form-actions button { margin: 0; }
.draft-action-group { display: flex; flex: 1 1 100%; flex-wrap: wrap; gap: 14rpx; }
.form-actions .clear-draft-button { flex: 1 1 100%; border: 1rpx solid #d7aaa3; background: #fff; color: #9a3e34; }
.form-actions .draft-button, .form-actions .submit { flex: 1 1 0; }
.draft-button { border: 1rpx solid #28624a; background: #fff; color: #28624a; }
.submit { background: #28624a; color: #fff; }
.status-card { display: flex; align-items: center; justify-content: space-between; gap: 16rpx; margin-bottom: 22rpx; padding: 18rpx 20rpx; border-radius: 12rpx; background: #eef5f1; color: #355345; font-size: 24rpx; }
.error-card { background: #fff2f0; color: #a7352a; }
.status-action { flex: 0 0 auto; margin: 0; padding: 0 22rpx; min-height: 58rpx; line-height: 58rpx; background: #28624a; color: #fff; font-size: 24rpx; }
.overlay { position: fixed; inset: 0; z-index: 100; display: flex; align-items: flex-end; background: rgba(16, 28, 22, .48); }
.select-sheet { width: 100%; max-height: 78vh; padding: 28rpx 28rpx calc(24rpx + env(safe-area-inset-bottom)); border-radius: 24rpx 24rpx 0 0; box-sizing: border-box; background: #fff; }
.sheet-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 20rpx; }
.sheet-actions { display: flex; align-items: center; gap: 12rpx; }
.sheet-title { color: #1e362a; font-size: 32rpx; font-weight: 800; }
.sheet-create, .sheet-close { padding: 12rpx; color: #28624a; font-size: 26rpx; }
.sheet-close { color: #718078; }
.search-input { width: 100%; min-height: 78rpx; padding: 0 22rpx; border: 2rpx solid #b8ccc0; border-radius: 12rpx; box-sizing: border-box; background: #f9fbfa; }
.option-list { height: 52vh; margin-top: 14rpx; }
.option-row { display: flex; flex-direction: column; gap: 8rpx; padding: 22rpx 10rpx; border-bottom: 1rpx solid #edf1ee; }
.option-name { color: #172c22; font-size: 29rpx; font-weight: 700; }
.empty-state { display: block; padding: 80rpx 20rpx; color: #7d8982; text-align: center; }
.result-hint { display: block; padding-top: 14rpx; color: #89958e; font-size: 21rpx; text-align: center; }
@media (max-width: 380px) {
  .fee-editor-grid { grid-template-columns: 1fr; gap: 0; }
}
</style>
