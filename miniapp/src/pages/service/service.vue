<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onShow } from '@dcloudio/uni-app'
import {
  acknowledgeBeanListVersion,
  assignCustomerProductCategory,
  type BeanListProductSummary,
  type BeanListSummary,
  buildResaleBeanListPDFPath,
  buildResaleBeanListPNGPath,
  createCustomerProductCategory,
  createDirectShipBatch,
  createFulfillmentOrder,
  createProcessingRequest,
  deleteCustomerProductCategory,
  fetchCustomerProducts,
  fetchResaleBeanListEditor,
  fetchResaleBeanLists,
  fetchServicePage,
  moveCustomerProductCategory,
  type CustomerPriceTableGroup,
  type CustomerProductCategory,
  type CustomerProductsPage,
  type CustomerProductSummary,
  type InventoryItem,
  type ProductSummary,
  publishResaleBeanList,
  type ResaleBeanListCommand,
  type ResaleBeanListEditor,
  type ResaleBeanListPage,
  type ResaleGradientTemplate,
  saveResaleBeanListDraft,
  type ServicePageResponse,
  updateCustomerProductCategory,
} from '../../api/customerPortal'
import { buildAPIURL } from '../../api/client'
import MainTabBar from '../../components/MainTabBar.vue'
import { useSessionStore } from '../../stores/session'
import { beanListCardRows, beanListDisplayStyle, beanListQualityLines, splitBeanListHighlight } from '../../utils/beanListDisplay'
import {
  beanListPageCacheChanged,
  beanListPageCacheStorageKey,
  nextBeanListPageCacheRecord,
  type BeanListPageCacheRecord,
} from '../../utils/beanListPageCache'
import { buildOrderServiceFilters, datePresetRange, normalizeDateRange, type OrderDatePreset } from '../../utils/orderFilters'
import { priceTableGroupLabel } from '../../utils/customerProducts'
import { buildResaleBeanListPublishPayload, defaultResaleBeanListDraft, resaleBeanListItemKey, resaleCardsPerRowOptions, resaleStyleColorPresets } from '../../utils/resaleBeanList'
import {
  buildFulfillmentOrderPayload,
  fulfillmentSalesUnitOptions,
  fulfillmentUnitOption,
  normalizeServiceKey,
  orderSectionTitle,
  serviceTitle,
  visibleServiceSections,
  type ServiceKey,
} from '../../utils/servicePage'
import { miniappThemeClass, miniappThemeMeta } from '../../utils/themes'

type OrderSearchForm = {
  keyword: string
  date_from: string
  date_to: string
  process_status: string
  pay_status: string
  ship_status: string
}

type OrderStatusField = 'process_status' | 'pay_status' | 'ship_status'

type PickerOption<T = unknown> = {
  label: string
  value: number
  data: T
}

const session = useSessionStore()
const serviceKey = ref<ServiceKey>('beanList')
const page = ref<ServicePageResponse | null>(null)
const loading = ref(false)
const submitting = ref(false)
const errorMessage = ref('')
const cachedBeanList = ref<BeanListSummary | null>(null)
const beanListCacheStatus = ref('')
const resalePage = ref<ResaleBeanListPage | null>(null)
const customerProductsPage = ref<CustomerProductsPage | null>(null)
const resaleEditor = ref<ResaleBeanListEditor | null>(null)
const resaleDraft = ref<ResaleBeanListCommand | null>(null)
const resaleLoading = ref(false)
const customerProductLoading = ref(false)
const customerCategoryName = ref('')
const selectedCustomerCategoryID = ref(0)
const categoryNameEdits = ref<Record<number, string>>({})
const expandedCustomerPriceTableTypes = ref<string[]>([])
const orderSearch = ref<OrderSearchForm>(emptyOrderSearch())

const defaultProcessStatusOptions = ['待处理', '生产中', '生产完成', '库存待发货', '无需生产']
const defaultPayStatusOptions = ['未付款', '已付款', '未收款', '已收款']
const defaultShipStatusOptions = ['未发货', '待发货', '已发货']

const directShipForm = ref({ source_name: '', total_rows: 0, note: '' })
const processingForm = ref({
  input_material_id: 0,
  input_qty_g: 0,
  target_product_id: 0,
  target_spec_g: 454,
  target_qty: 1,
  note: '',
})
const fulfillmentForm = ref({
  recipient_name: '',
  recipient_phone: '',
  recipient_address: '',
  recipient_company: '',
  product_id: 0,
  product_name: '',
  spec_g: 454,
  qty: 1,
  sales_unit: '',
  unit_bag_count: 0,
  unit_bean_g: 0,
  note: '',
})

const title = computed(() => page.value?.title || serviceTitle(serviceKey.value))
const mainTab = computed(() => {
  if (serviceKey.value === 'orders') return 'orders'
  if (serviceKey.value === 'settlement') return 'billing'
  if (serviceKey.value === 'beanList') return 'mine'
  return 'home'
})
const activeThemeKey = computed(() => page.value?.theme_key || session.themeKey)
const themeClass = computed(() => miniappThemeClass(activeThemeKey.value))
const themeMeta = computed(() => miniappThemeMeta(activeThemeKey.value))
const summary = computed(() => page.value?.summary || [])
const sections = computed(() => (page.value ? visibleServiceSections(page.value) : []))
const orderPanelTitle = computed(() => orderSectionTitle(serviceKey.value))
const beanListsForDisplay = computed(() => {
  if (page.value?.bean_lists?.length) return page.value.bean_lists
  return cachedBeanList.value ? [cachedBeanList.value] : []
})
const resaleFactorySources = computed(() => resalePage.value?.factory_supply_bean_lists || [])
const resaleCustomerVersions = computed(() => resalePage.value?.customer_resale_bean_lists || [])
const customerCategories = computed(() => customerProductsPage.value?.categories || [])
const customerProducts = computed(() => customerProductsPage.value?.products || [])
const customerCategoryPickerLabels = computed(() => ['未分类', ...customerCategories.value.map((item) => item.name)])
const selectedCustomerCategoryLabel = computed(() => {
  if (!selectedCustomerCategoryID.value) return '选择归类'
  return customerCategories.value.find((item) => item.id === selectedCustomerCategoryID.value)?.name || '选择归类'
})
const factoryPriceTableGroups = computed(() => customerProductsPage.value?.factory_price_table_groups || resalePage.value?.factory_price_table_groups || [])
const customerPriceTableGroups = computed(() => customerProductsPage.value?.customer_price_table_groups || resalePage.value?.customer_price_table_groups || [])
const resaleGradientTemplates = computed(() => {
  if (resaleEditor.value?.gradient_templates?.length) return resaleEditor.value.gradient_templates
  return resalePage.value?.gradient_templates || []
})
const resaleSourceLabels = computed(() => resaleFactorySources.value.length ? resaleFactorySources.value.map(resaleSourceLabel) : ['暂无工厂供货价格表'])
const resaleTemplateLabels = computed(() => resaleGradientTemplates.value.length ? resaleGradientTemplates.value.map(resaleTemplateLabel) : ['暂无授权阶梯价模板'])
const resaleSourcePickerValue = computed(() => Math.max(0, resaleFactorySources.value.findIndex((item) => item.id === resaleDraft.value?.source_publication_id)))
const resaleTemplatePickerValue = computed(() => Math.max(0, resaleGradientTemplates.value.findIndex((item) => item.id === resaleDraft.value?.gradient_template_id)))
const selectedResaleSourceLabel = computed(() => {
  const source = resaleFactorySources.value.find((item) => item.id === resaleDraft.value?.source_publication_id)
  return source ? resaleSourceLabel(source) : '选择工厂供货价格表'
})
const selectedResaleTemplateLabel = computed(() => {
  const template = resaleGradientTemplates.value.find((item) => item.id === resaleDraft.value?.gradient_template_id)
  return template ? resaleTemplateLabel(template) : '选择授权阶梯价模板'
})
const resaleDraftItems = computed(() => flattenBeanListItems(resaleEditor.value?.source))
const resaleSelectedCount = computed(() => resaleDraft.value?.selected_item_codes?.length || 0)
const resaleSelectedAll = computed(() => resaleDraftItems.value.length > 0 && resaleSelectedCount.value >= resaleDraftItems.value.length)
const hasDisplayData = computed(() => sections.value.length > 0 || (serviceKey.value === 'beanList' && (customerProducts.value.length > 0 || factoryPriceTableGroups.value.length > 0 || customerPriceTableGroups.value.length > 0 || beanListsForDisplay.value.length > 0 || resaleFactorySources.value.length > 0 || resaleCustomerVersions.value.length > 0)))
const processStatusPickerOptions = computed(() => orderStatusOptions('process_status', defaultProcessStatusOptions, '全部生产状态'))
const payStatusPickerOptions = computed(() => orderStatusOptions('pay_status', defaultPayStatusOptions, '全部收款状态'))
const shipStatusPickerOptions = computed(() => orderStatusOptions('ship_status', defaultShipStatusOptions, '全部发货状态'))
const processingInputOptions = computed(() => inventoryInputOptions(page.value?.inventory || []))
const processingInputLabels = computed(() => pickerLabels(processingInputOptions.value, '暂无可用投入物料'))
const processingTargetProductOptions = computed(() => productPickerOptions(page.value?.products || []))
const processingTargetProductLabels = computed(() => pickerLabels(processingTargetProductOptions.value, '暂无可选目标产品'))
const fulfillmentProductOptions = computed(() => productPickerOptions(page.value?.products || []))
const fulfillmentProductLabels = computed(() => pickerLabels(fulfillmentProductOptions.value, '暂无可选商品'))
const selectedProcessingInputLabel = computed(() => selectedPickerLabel(processingInputOptions.value, processingForm.value.input_material_id, '选择投入物料'))
const selectedProcessingTargetProductLabel = computed(() => selectedPickerLabel(processingTargetProductOptions.value, processingForm.value.target_product_id, '选择目标产品'))
const selectedFulfillmentProductLabel = computed(() => selectedPickerLabel(fulfillmentProductOptions.value, fulfillmentForm.value.product_id, '选择发货商品'))
const selectedFulfillmentProduct = computed(() => fulfillmentProductOptions.value.find((item) => item.value === fulfillmentForm.value.product_id)?.data || null)
const fulfillmentSalesUnitPickerOptions = computed(() => fulfillmentSalesUnitOptions(selectedFulfillmentProduct.value))
const fulfillmentSalesUnitLabels = computed(() => pickerLabels(fulfillmentSalesUnitPickerOptions.value, '暂无可选单位'))
const selectedFulfillmentSalesUnitLabel = computed(() => fulfillmentUnitOption(selectedFulfillmentProduct.value, fulfillmentForm.value.sales_unit)?.label || '选择销售单位')
const fulfillmentQuantityPlaceholder = computed(() => fulfillmentUnitOption(selectedFulfillmentProduct.value, fulfillmentForm.value.sales_unit)?.quantity_label || '件数')

async function loadPage() {
  if (!session.token) {
    uni.reLaunch({ url: '/pages/login/login' })
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    if (serviceKey.value === 'beanList') {
      primeCachedBeanListPage()
    }
    const filters = serviceKey.value === 'orders' || serviceKey.value === 'settlement' ? buildOrderServiceFilters(orderSearch.value) : {}
    page.value = await fetchServicePage(session.token, serviceKey.value, filters)
    if (page.value.theme_key) {
      session.applyContext({
        mini_user_id: session.miniUserID,
        current_customer_id: page.value.current_customer_id || session.currentCustomerID,
        current_customer_name: page.value.current_customer_name || session.currentCustomerName,
        theme_key: page.value.theme_key,
        miniapp_entry_mode: page.value.miniapp_entry_mode || session.entryMode,
        bindings: session.bindings,
        capabilities: session.capabilities,
      })
    }
    if (serviceKey.value === 'beanList') {
      await loadCustomerProductsWorkspace()
      cacheBeanListPages(page.value.bean_lists || [])
      await loadResaleBeanListWorkspace()
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '服务数据加载失败'
  } finally {
    loading.value = false
  }
}

function primeCachedBeanListPage() {
  const customerID = page.value?.current_customer_id || session.currentCustomerID
  for (const listType of ['commercial', 'retail']) {
    const cached = cachedBeanListPage({ id: 0, list_type: listType, version_no: '', status: '', published_at: '', changelog: '', cache_key: '' })
    if (cached?.page) {
      cachedBeanList.value = cached.page
      beanListCacheStatus.value = `${cached.version_no || '本地版本'} 已缓存，本次打开先展示本地内容`
      return
    }
  }
  if (!customerID) beanListCacheStatus.value = ''
}

function cachedBeanListPage(item: BeanListSummary): BeanListPageCacheRecord | null {
  const value = uni.getStorageSync(beanListPageCacheStorageKey(page.value?.current_customer_id || session.currentCustomerID, item))
  if (!value || typeof value !== 'object') return null
  return value as BeanListPageCacheRecord
}

function cacheBeanListPages(items: BeanListSummary[]) {
  if (!items.length) {
    if (!cachedBeanList.value) beanListCacheStatus.value = '暂无已发布商品价格表'
    return
  }
  let updated = false
  for (const item of items) {
    updated = cacheBeanListPage(item) || updated
  }
  cachedBeanList.value = items[0]
  beanListCacheStatus.value = updated ? '检测到新版商品价格表，已更新本地缓存' : '已使用本地缓存，发布新版后自动更新'
}

function cacheBeanListPage(item: BeanListSummary): boolean {
  const cached = cachedBeanListPage(item)
  const changed = !cached || beanListPageCacheChanged(cached, item)
  if (changed) {
    uni.setStorageSync(beanListPageCacheStorageKey(page.value?.current_customer_id || session.currentCustomerID, item), nextBeanListPageCacheRecord(item))
  }
  return changed
}

function showBeanListCategory(item: BeanListSummary, group: { show_category?: boolean; category?: string }): boolean {
  return item.show_category_numbers !== false && group.show_category !== false && Boolean(group.category)
}

function resetLocalForms() {
  directShipForm.value = { source_name: '', total_rows: 0, note: '' }
  processingForm.value = {
    input_material_id: 0,
    input_qty_g: 0,
    target_product_id: 0,
    target_spec_g: 454,
    target_qty: 1,
    note: '',
  }
  fulfillmentForm.value = {
    recipient_name: '',
    recipient_phone: '',
    recipient_address: '',
    recipient_company: '',
    product_id: 0,
    product_name: '',
    spec_g: 454,
    qty: 1,
    sales_unit: '',
    unit_bag_count: 0,
    unit_bean_g: 0,
    note: '',
  }
  orderSearch.value = emptyOrderSearch()
  page.value = null
  resalePage.value = null
  customerProductsPage.value = null
  resaleEditor.value = null
  resaleDraft.value = null
  customerCategoryName.value = ''
  selectedCustomerCategoryID.value = 0
  categoryNameEdits.value = {}
}

async function applyOrderFilters() {
  const normalized = normalizeDateRange(orderSearch.value.date_from, orderSearch.value.date_to)
  orderSearch.value.date_from = normalized.date_from || ''
  orderSearch.value.date_to = normalized.date_to || ''
  await loadPage()
}

async function applyDatePreset(preset: OrderDatePreset) {
  const range = datePresetRange(preset)
  orderSearch.value.date_from = range.date_from
  orderSearch.value.date_to = range.date_to
  await loadPage()
}

async function clearOrderFilters() {
  orderSearch.value = emptyOrderSearch()
  await loadPage()
}

function openOrderFromBill(orderNo?: string) {
  const keyword = String(orderNo || '').trim()
  if (!keyword) return
  uni.navigateTo({ url: `/pages/service/service?key=orders&q=${encodeURIComponent(keyword)}` })
}

function paymentMethodText(payStatus?: string, paymentMethod?: string): string {
  const method = String(paymentMethod || '').trim()
  if (method) return method
  const status = normalizeStatusText(payStatus)
  if (status.includes('已付') || status.includes('已收')) return '未填写'
  return '未付款'
}

function openOrderDocument(path?: string) {
  if (!path) {
    uni.showToast({ title: '单据暂不可用', icon: 'none' })
    return
  }
  if (!session.token) {
    uni.reLaunch({ url: '/pages/login/login' })
    return
  }
  uni.showLoading({ title: '打开中' })
  uni.downloadFile({
    url: buildAPIURL(path),
    header: {
      Authorization: `Bearer ${session.token}`,
    },
    success: (res) => {
      if (res.statusCode !== 200 || !res.tempFilePath) {
        uni.showToast({ title: '单据暂不可用', icon: 'none' })
        return
      }
      uni.openDocument({
        filePath: res.tempFilePath,
        fileType: 'pdf',
        showMenu: true,
        fail: () => {
          uni.showToast({ title: '单据打开失败', icon: 'none' })
        },
      })
    },
    fail: () => {
      uni.showToast({ title: '单据下载失败', icon: 'none' })
    },
    complete: () => {
      uni.hideLoading()
    },
  })
}

async function loadCustomerProductsWorkspace() {
  if (!session.token || serviceKey.value !== 'beanList') return
  customerProductLoading.value = true
  try {
    customerProductsPage.value = await fetchCustomerProducts(session.token)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '客户商品加载失败'
  } finally {
    customerProductLoading.value = false
  }
}

function setSelectedCustomerCategory(event: { detail?: { value?: number | string } }) {
  const index = Number(event.detail?.value ?? 0)
  selectedCustomerCategoryID.value = index <= 0 ? 0 : customerCategories.value[index - 1]?.id || 0
}

async function createCustomerCategory(parentID = 0) {
  if (!session.token) return
  const name = customerCategoryName.value.trim()
  if (!name) {
    errorMessage.value = '请填写分类名称'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    await createCustomerProductCategory(session.token, { name, parent_id: parentID })
    customerCategoryName.value = ''
    await loadCustomerProductsWorkspace()
    uni.showToast({ title: '分类已保存', icon: 'success' })
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '分类保存失败'
  } finally {
    submitting.value = false
  }
}

function categoryEditName(item: CustomerProductCategory): string {
  return categoryNameEdits.value[item.id] ?? item.name
}

function setCategoryEditName(item: CustomerProductCategory, event: unknown) {
  categoryNameEdits.value = { ...categoryNameEdits.value, [item.id]: inputEventValue(event) }
}

async function saveCategoryName(item: CustomerProductCategory) {
  if (!session.token) return
  const name = categoryEditName(item).trim()
  if (!name) {
    errorMessage.value = '请填写分类名称'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    await updateCustomerProductCategory(session.token, item.id, { name, parent_id: item.parent_id, sort_order: item.sort_order })
    await loadCustomerProductsWorkspace()
    uni.showToast({ title: '分类已更新', icon: 'success' })
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '分类更新失败'
  } finally {
    submitting.value = false
  }
}

async function removeCustomerCategory(item: CustomerProductCategory) {
  if (!session.token) return
  uni.showModal({
    title: '删除分类',
    content: `删除 ${item.name} 后，该分类下商品会回到未分类。`,
    confirmText: '删除',
    success: async (res) => {
      if (!res.confirm) return
      submitting.value = true
      errorMessage.value = ''
      try {
        await deleteCustomerProductCategory(session.token, item.id)
        await loadCustomerProductsWorkspace()
        uni.showToast({ title: '分类已删除', icon: 'success' })
      } catch (error) {
        errorMessage.value = error instanceof Error ? error.message : '分类删除失败'
      } finally {
        submitting.value = false
      }
    },
  })
}

async function moveCustomerCategory(item: CustomerProductCategory, direction: 'up' | 'down') {
  if (!session.token) return
  submitting.value = true
  errorMessage.value = ''
  try {
    await moveCustomerProductCategory(session.token, item.id, { direction, parent_id: item.parent_id })
    await loadCustomerProductsWorkspace()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '分类排序失败'
  } finally {
    submitting.value = false
  }
}

async function assignProductToSelectedCategory(item: CustomerProductSummary) {
  if (!session.token) return
  submitting.value = true
  errorMessage.value = ''
  try {
    await assignCustomerProductCategory(session.token, item.id, { category_id: selectedCustomerCategoryID.value })
    await loadCustomerProductsWorkspace()
    uni.showToast({ title: '已归类', icon: 'success' })
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '商品归类失败'
  } finally {
    submitting.value = false
  }
}

function priceTableGroupTitle(group: CustomerPriceTableGroup): string {
  return priceTableGroupLabel(group)
}

function customerPriceTableExpanded(group: CustomerPriceTableGroup): boolean {
  return expandedCustomerPriceTableTypes.value.includes(group.list_type)
}

function toggleCustomerPriceTableGroup(group: CustomerPriceTableGroup) {
  const set = new Set(expandedCustomerPriceTableTypes.value)
  if (set.has(group.list_type)) set.delete(group.list_type)
  else set.add(group.list_type)
  expandedCustomerPriceTableTypes.value = Array.from(set)
}

async function loadResaleBeanListWorkspace() {
  if (!session.token || serviceKey.value !== 'beanList') return
  resaleLoading.value = true
  try {
    const result = await fetchResaleBeanLists(session.token)
    resalePage.value = result
    const sourceID = resaleDraft.value?.source_publication_id || result.factory_supply_bean_lists?.[0]?.id || 0
    if (sourceID > 0 && (!resaleEditor.value || resaleEditor.value.source.id !== sourceID)) {
      await openResaleEditor(sourceID, false)
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '客户商品价格表加载失败'
  } finally {
    resaleLoading.value = false
  }
}

async function openResaleEditor(sourceID: number, showLoading = true) {
  if (!session.token || sourceID <= 0) return
  if (showLoading) resaleLoading.value = true
  try {
    const editor = await fetchResaleBeanListEditor(session.token, sourceID)
    resaleEditor.value = editor
    const draft = defaultResaleBeanListDraft(editor.source, editor.next_version_no || 'V1')
    const template = (editor.gradient_templates || resalePage.value?.gradient_templates || [])[0]
    if (template) draft.gradient_template_id = template.id
    resaleDraft.value = draft
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '编辑器加载失败'
  } finally {
    if (showLoading) resaleLoading.value = false
  }
}

function setResaleSource(event: { detail?: { value?: number | string } }) {
  const source = resaleFactorySources.value[Number(event.detail?.value ?? -1)]
  if (source) {
    void openResaleEditor(source.id)
  }
}

function setResaleTemplate(event: { detail?: { value?: number | string } }) {
  const template = resaleGradientTemplates.value[Number(event.detail?.value ?? -1)]
  if (template && resaleDraft.value) {
    resaleDraft.value.gradient_template_id = template.id
  }
}

function setResaleLayoutStyle(style: 'card' | 'table') {
  if (resaleDraft.value) resaleDraft.value.config.layoutStyle = style
}

function setResaleStylePreset(preset: { backgroundColor: string; fontColor: string }) {
  if (!resaleDraft.value) return
  resaleDraft.value.config.backgroundColor = preset.backgroundColor
  resaleDraft.value.config.fontColor = preset.fontColor
}

function resaleStylePresetActive(preset: { backgroundColor: string; fontColor: string }): boolean {
  return resaleConfigText('backgroundColor') === preset.backgroundColor && resaleConfigText('fontColor') === preset.fontColor
}

function setResaleCardsPerRow(value: number) {
  if (resaleDraft.value) resaleDraft.value.config.cardsPerRow = value
}

function resaleCardsPerRowActive(value: number): boolean {
  return Number(resaleDraft.value?.config.cardsPerRow || 0) === value
}

function resaleConfigText(key: string): string {
  const value = resaleDraft.value?.config?.[key]
  return value == null ? '' : String(value)
}

function setResaleConfigText(key: string, event: unknown) {
  if (!resaleDraft.value) return
  resaleDraft.value.config[key] = inputEventValue(event)
}

function flattenBeanListItems(item?: BeanListSummary | null): BeanListProductSummary[] {
  const out: BeanListProductSummary[] = []
  for (const group of item?.groups || []) {
    out.push(...(group.items || []))
  }
  return out
}

function resaleSourceLabel(item: BeanListSummary): string {
  return `${item.title || item.list_type_label || item.list_type} / ${item.version_no || '未标版本'}`
}

function resaleTemplateLabel(item: ResaleGradientTemplate): string {
  const unit = item.display_unit ? ` / ${item.display_unit}` : ''
  return `${item.name}${unit}`
}

function resaleItemSelected(item: BeanListProductSummary): boolean {
  const key = resaleBeanListItemKey(item)
  return Boolean(key && resaleDraft.value?.selected_item_codes?.includes(key))
}

function toggleResaleItem(item: BeanListProductSummary) {
  if (!resaleDraft.value) return
  const key = resaleBeanListItemKey(item)
  if (!key) return
  const selected = new Set(resaleDraft.value.selected_item_codes || [])
  if (selected.has(key)) selected.delete(key)
  else selected.add(key)
  resaleDraft.value.selected_item_codes = Array.from(selected)
}

function setAllResaleItems(selected: boolean) {
  if (!resaleDraft.value) return
  resaleDraft.value.selected_item_codes = selected ? resaleDraftItems.value.map(resaleBeanListItemKey).filter(Boolean) : []
}

function resaleItemOverride(item: BeanListProductSummary) {
  if (!resaleDraft.value) return null
  const key = resaleBeanListItemKey(item)
  if (!key) return null
  if (!resaleDraft.value.item_overrides) resaleDraft.value.item_overrides = []
  let row = resaleDraft.value.item_overrides.find((override) => override.code === key)
  if (!row) {
    row = { code: key }
    resaleDraft.value.item_overrides.push(row)
  }
  return row
}

function setResaleItemBadge(item: BeanListProductSummary, label: string) {
  const row = resaleItemOverride(item)
  if (!row) return
  row.badge_label = label
  row.highlight_terms = label ? [label] : []
}

function inputEventValue(event: unknown): string {
  const candidate = event as { detail?: unknown; target?: { value?: unknown } }
  const detail = candidate?.detail
  if (detail && typeof detail === 'object' && 'value' in detail) {
    return String((detail as { value?: unknown }).value ?? '').trim()
  }
  return String(candidate?.target?.value ?? '').trim()
}

async function submitResaleBeanListDraft() {
  await submitResaleBeanList('draft')
}

async function submitResaleBeanListPublication() {
  await submitResaleBeanList('published')
}

async function submitResaleBeanList(status: 'draft' | 'published') {
  if (!resaleDraft.value) return
  if (!resaleDraft.value.source_publication_id || !resaleSelectedCount.value) {
    errorMessage.value = '请选择来源价格表和商品'
    return
  }
  if (resaleGradientTemplates.value.length > 0 && !resaleDraft.value.gradient_template_id) {
    errorMessage.value = '请选择授权阶梯价模板'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    const payload = buildResaleBeanListPublishPayload(resaleDraft.value)
    const sourceID = payload.source_publication_id
    if (status === 'published') {
      await publishResaleBeanList(session.token, payload)
    } else {
      await saveResaleBeanListDraft(session.token, payload)
    }
    uni.showToast({ title: status === 'published' ? '已发布' : '草稿已保存', icon: 'success' })
    await loadResaleBeanListWorkspace()
    if (sourceID > 0) {
      await openResaleEditor(sourceID, false)
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '保存失败'
  } finally {
    submitting.value = false
  }
}

function openResaleOutput(item: BeanListSummary, kind: 'pdf' | 'png') {
  if (!session.token) {
    uni.reLaunch({ url: '/pages/login/login' })
    return
  }
  const path = kind === 'pdf' ? buildResaleBeanListPDFPath(item.id) : buildResaleBeanListPNGPath(item.id)
  uni.showLoading({ title: '生成中' })
  uni.downloadFile({
    url: buildAPIURL(path),
    header: { Authorization: `Bearer ${session.token}` },
    success: (res) => {
      if (res.statusCode !== 200 || !res.tempFilePath) {
        uni.showToast({ title: '文件暂不可用', icon: 'none' })
        return
      }
      if (kind === 'pdf') {
        uni.openDocument({
          filePath: res.tempFilePath,
          fileType: 'pdf',
          showMenu: true,
          fail: () => uni.showToast({ title: 'PDF 打开失败', icon: 'none' }),
        })
      } else {
        uni.previewImage({
          urls: [res.tempFilePath],
          fail: () => uni.showToast({ title: '图片预览失败', icon: 'none' }),
        })
      }
    },
    fail: () => {
      uni.showToast({ title: '文件下载失败', icon: 'none' })
    },
    complete: () => {
      uni.hideLoading()
    },
  })
}

function setOrderDateFrom(event: { detail?: { value?: string } }) {
  orderSearch.value.date_from = event.detail?.value || ''
}

function setOrderDateTo(event: { detail?: { value?: string } }) {
  orderSearch.value.date_to = event.detail?.value || ''
}

function setOrderProcessStatus(event: { detail?: { value?: number | string } }) {
  setOrderStatus('process_status', processStatusPickerOptions.value, event)
}

function setOrderPayStatus(event: { detail?: { value?: number | string } }) {
  setOrderStatus('pay_status', payStatusPickerOptions.value, event)
}

function setOrderShipStatus(event: { detail?: { value?: number | string } }) {
  setOrderStatus('ship_status', shipStatusPickerOptions.value, event)
}

function setOrderStatus(field: OrderStatusField, options: string[], event: { detail?: { value?: number | string } }) {
  const index = Number(event.detail?.value ?? 0)
  orderSearch.value[field] = index > 0 ? options[index] || '' : ''
}

function statusPickerValue(options: string[], value: string): number {
  const index = options.indexOf(normalizeStatusText(value))
  return index > 0 ? index : 0
}

function orderStatusOptions(field: OrderStatusField, defaults: string[], emptyLabel: string): string[] {
  const seen = new Set<string>()
  const out = [emptyLabel]
  for (const value of [...defaults, orderSearch.value[field], ...(page.value?.orders || []).map((item) => item[field])]) {
    const normalized = normalizeStatusText(value)
    if (!normalized || seen.has(normalized)) continue
    seen.add(normalized)
    out.push(normalized)
  }
  return out
}

function normalizeStatusText(value?: string): string {
  return (value || '').trim().replace(/\s+/g, ' ')
}

function emptyOrderSearch(): OrderSearchForm {
  return { keyword: '', date_from: '', date_to: '', process_status: '', pay_status: '', ship_status: '' }
}

function pickerLabels(options: Array<{ label: string }>, emptyLabel: string): string[] {
  return options.length ? options.map((item) => item.label) : [emptyLabel]
}

function selectedPickerLabel(options: PickerOption[], value: number, emptyLabel: string): string {
  return options.find((item) => item.value === value)?.label || emptyLabel
}

function productKindLabel(item: Pick<ProductSummary, 'product_kind'>): string {
  return item.product_kind === 'green_bean' ? '生豆' : '熟豆'
}

function pickerOptionAt<T>(options: PickerOption<T>[], event: { detail?: { value?: number | string } }): PickerOption<T> | null {
  return options[Number(event.detail?.value ?? -1)] || null
}

function inventoryInputOptions(items: InventoryItem[]): PickerOption<InventoryItem>[] {
  const inputTypes = new Set(['raw_bean', 'material', 'green_bean'])
  return items
    .filter((item) => inputTypes.has(item.item_type) && item.status === 'available' && Number(item.qty_g) > 0)
    .map((item) => ({
      label: `${item.item_name} / ${item.qty_g}g 可用`,
      value: item.item_id,
      data: item,
    }))
}

function productPickerOptions(products: ProductSummary[]): PickerOption<ProductSummary>[] {
  return products.map((item) => ({
    label: `${productKindLabel(item)} / ${item.name} / 默认 ¥${item.default_price || '0.00'}`,
    value: item.id,
    data: item,
  }))
}

function setProcessingInputMaterial(event: { detail?: { value?: number | string } }) {
  const option = pickerOptionAt(processingInputOptions.value, event)
  if (!option) return
  processingForm.value.input_material_id = option.value
}

function setProcessingTargetProduct(event: { detail?: { value?: number | string } }) {
  const option = pickerOptionAt(processingTargetProductOptions.value, event)
  if (!option) return
  processingForm.value.target_product_id = option.value
}

function setFulfillmentProduct(event: { detail?: { value?: number | string } }) {
  const option = pickerOptionAt(fulfillmentProductOptions.value, event)
  if (!option) return
  fulfillmentForm.value.product_id = option.value
  fulfillmentForm.value.product_name = option.data.name
  const unit = fulfillmentUnitOption(option.data)
  if (unit) {
    applyFulfillmentSalesUnit(unit.sales_unit)
  } else {
    fulfillmentForm.value.sales_unit = ''
    fulfillmentForm.value.unit_bag_count = 0
    fulfillmentForm.value.unit_bean_g = 0
    fulfillmentForm.value.spec_g = 454
  }
}

function setFulfillmentSalesUnit(event: { detail?: { value?: number | string } }) {
  const option = fulfillmentSalesUnitPickerOptions.value[Number(event.detail?.value ?? -1)]
  if (!option) return
  applyFulfillmentSalesUnit(option.sales_unit)
}

function applyFulfillmentSalesUnit(salesUnit: string) {
  const unit = fulfillmentUnitOption(selectedFulfillmentProduct.value, salesUnit)
  if (!unit) return
  fulfillmentForm.value.sales_unit = unit.sales_unit
  fulfillmentForm.value.unit_bag_count = unit.unit_bag_count
  fulfillmentForm.value.unit_bean_g = unit.unit_bean_g
  fulfillmentForm.value.spec_g = unit.spec_g
}

async function submitDirectShipBatch() {
  if (!directShipForm.value.source_name.trim()) {
    errorMessage.value = '请填写批次名称'
    return
  }
  const totalRows = Number(directShipForm.value.total_rows) || 0
  if (totalRows <= 0) {
    errorMessage.value = '订单行数必须大于 0'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    await createDirectShipBatch(session.token, {
      source_name: directShipForm.value.source_name,
      total_rows: totalRows,
      note: directShipForm.value.note,
    })
    directShipForm.value = { source_name: '', total_rows: 0, note: '' }
    uni.showToast({ title: '已提交', icon: 'success' })
    await loadPage()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '提交失败'
  } finally {
    submitting.value = false
  }
}

async function submitProcessingRequest() {
  const payload = {
    input_material_id: Number(processingForm.value.input_material_id) || 0,
    input_qty_g: Number(processingForm.value.input_qty_g) || 0,
    target_product_id: Number(processingForm.value.target_product_id) || 0,
    target_spec_g: Number(processingForm.value.target_spec_g) || 0,
    target_qty: Number(processingForm.value.target_qty) || 0,
    note: processingForm.value.note,
  }
  if (!payload.input_material_id || !payload.input_qty_g || !payload.target_product_id || !payload.target_spec_g || !payload.target_qty) {
    errorMessage.value = '请填写完整加工信息'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    await createProcessingRequest(session.token, payload)
    processingForm.value = {
      input_material_id: 0,
      input_qty_g: 0,
      target_product_id: 0,
      target_spec_g: 454,
      target_qty: 1,
      note: '',
    }
    uni.showToast({ title: '已提交', icon: 'success' })
    await loadPage()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '提交失败'
  } finally {
    submitting.value = false
  }
}

function fulfillmentServiceCode(): 'direct_ship' | 'processing_ship' | 'product_order' {
  if (serviceKey.value === 'processing') return 'processing_ship'
  if (serviceKey.value === 'productOrder') return 'product_order'
  return 'direct_ship'
}

async function submitFulfillmentOrder() {
  const payload = buildFulfillmentOrderPayload(fulfillmentServiceCode(), fulfillmentForm.value)
  if (!payload.recipient_name || !payload.recipient_phone || !payload.recipient_address || !payload.product_id || !payload.spec_g || !payload.qty) {
    errorMessage.value = '请填写完整发货订单'
    return
  }
  if (!(await confirmBeanListUpdateIfNeeded())) return
  submitting.value = true
  errorMessage.value = ''
  try {
    await createFulfillmentOrder(session.token, payload)
    fulfillmentForm.value = {
      recipient_name: '',
      recipient_phone: '',
      recipient_address: '',
      recipient_company: '',
      product_id: 0,
      product_name: '',
      spec_g: 454,
      qty: 1,
      sales_unit: '',
      unit_bag_count: 0,
      unit_bean_g: 0,
      note: '',
    }
    uni.showToast({ title: '订单已提交', icon: 'success' })
    await loadPage()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '提交失败'
  } finally {
    submitting.value = false
  }
}

function beanListPromptTarget(): BeanListSummary | null {
  return beanListsForDisplay.value.find((item) => item.requires_acknowledgement) || null
}

function beanListDiffText(item: BeanListSummary | null): string {
  if (!item) return ''
  const diff = item.diff || {}
  const lines: string[] = []
  if (item.changelog) lines.push(item.changelog)
  const previous = diff.previous_version_no ? `从 ${diff.previous_version_no} 更新到 ${item.version_no || '新版'}` : `当前版本 ${item.version_no || ''}`.trim()
  if (previous) lines.push(previous)
  if (diff.added?.length) lines.push(`新增：${diff.added.map((bean) => bean.name).slice(0, 4).join('、')}`)
  if (diff.removed?.length) lines.push(`下架：${diff.removed.map((bean) => bean.name).slice(0, 4).join('、')}`)
  if (diff.changed?.length) lines.push(`调整：${diff.changed.map((bean) => bean.name).slice(0, 4).join('、')}`)
  return lines.filter(Boolean).join('\n') || '商品价格表已更新，请确认后继续下单。'
}

function showConfirmModal(title: string, content: string): Promise<boolean> {
  return new Promise((resolve) => {
    uni.showModal({
      title,
      content,
      confirmText: '查看并确认',
      cancelText: '稍后下单',
      success: (res) => resolve(!!res.confirm),
      fail: () => resolve(false),
    })
  })
}

async function confirmBeanListUpdateIfNeeded(): Promise<boolean> {
  const target = beanListPromptTarget()
  if (!target || !session.token) return true
  const confirmed = await showConfirmModal('商品价格表已更新', beanListDiffText(target))
  if (!confirmed) return false
  try {
    await acknowledgeBeanListVersion(session.token, target.id)
    target.requires_acknowledgement = false
    if (cachedBeanList.value?.id === target.id) cachedBeanList.value.requires_acknowledgement = false
    return true
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '商品价格表确认失败'
    return false
  }
}

onLoad((query) => {
  serviceKey.value = normalizeServiceKey(String(query?.key || 'beanList'))
  orderSearch.value = emptyOrderSearch()
  const keyword = String(query?.q || '').trim()
  if (keyword) {
    orderSearch.value.keyword = keyword
  }
})

onShow(() => {
  void loadPage()
})
</script>

<template>
  <view class="page" :class="themeClass">
    <view class="header">
      <text class="eyebrow">{{ themeMeta.eyebrow }}</text>
      <text class="title">{{ title }}</text>
      <text class="subtitle">{{ page?.current_customer_name || session.currentCustomerName || '客户中心' }}</text>
    </view>

    <view v-if="loading" class="state">
      <text>加载中...</text>
    </view>

    <view v-else>
      <view v-if="errorMessage" class="state error">
        <text>{{ errorMessage }}</text>
      </view>

      <view v-if="summary.length" class="metrics">
        <view v-for="item in summary" :key="item.label" class="metric">
          <text class="metric-value">{{ item.value }}</text>
          <text class="metric-label">{{ item.label }}</text>
        </view>
      </view>

      <view v-if="serviceKey === 'directShip'" class="panel">
        <text class="panel-title">新建代发批次</text>
        <input v-model="directShipForm.source_name" class="input" placeholder="批次名称，例如 5月直播订单" />
        <input v-model.number="directShipForm.total_rows" class="input" type="number" placeholder="订单行数" />
        <textarea v-model="directShipForm.note" class="textarea" placeholder="备注" />
        <button class="primary" :disabled="submitting" @tap="submitDirectShipBatch">提交批次</button>
      </view>

      <view v-if="serviceKey === 'processing'" class="panel">
        <text class="panel-title">提交加工申请</text>
        <picker mode="selector" :range="processingInputLabels" @change="setProcessingInputMaterial">
          <view class="picker-field">{{ selectedProcessingInputLabel }}</view>
        </picker>
        <input v-model.number="processingForm.input_qty_g" class="input" type="number" placeholder="投入生豆克重" />
        <picker mode="selector" :range="processingTargetProductLabels" @change="setProcessingTargetProduct">
          <view class="picker-field">{{ selectedProcessingTargetProductLabel }}</view>
        </picker>
        <input v-model.number="processingForm.target_spec_g" class="input" type="number" placeholder="规格克重" />
        <input v-model.number="processingForm.target_qty" class="input" type="number" placeholder="目标件数" />
        <textarea v-model="processingForm.note" class="textarea" placeholder="加工要求" />
        <button class="primary" :disabled="submitting" @tap="submitProcessingRequest">提交申请</button>
      </view>

      <view v-if="serviceKey === 'directShip' || serviceKey === 'processing' || serviceKey === 'productOrder'" class="panel">
        <text class="panel-title">新建发货订单</text>
        <input v-model="fulfillmentForm.recipient_name" class="input" placeholder="收件人" />
        <input v-model="fulfillmentForm.recipient_phone" class="input" placeholder="手机号" />
        <input v-model="fulfillmentForm.recipient_address" class="input" placeholder="收件地址" />
        <input v-model="fulfillmentForm.recipient_company" class="input" placeholder="公司/门店，可选" />
        <picker mode="selector" :range="fulfillmentProductLabels" @change="setFulfillmentProduct">
          <view class="picker-field">{{ selectedFulfillmentProductLabel }}</view>
        </picker>
        <picker v-if="fulfillmentSalesUnitPickerOptions.length" mode="selector" :range="fulfillmentSalesUnitLabels" @change="setFulfillmentSalesUnit">
          <view class="picker-field">{{ selectedFulfillmentSalesUnitLabel }}</view>
        </picker>
        <input v-model.number="fulfillmentForm.spec_g" class="input" type="number" placeholder="规格克重" />
        <input v-model.number="fulfillmentForm.qty" class="input" type="number" :placeholder="fulfillmentQuantityPlaceholder" />
        <textarea v-model="fulfillmentForm.note" class="textarea" placeholder="订单备注" />
        <button class="primary" :disabled="submitting" @tap="submitFulfillmentOrder">提交订单</button>
      </view>

      <view v-if="sections.length" class="section-list">
        <view v-for="section in sections" :key="section.title" class="section-row">
          <text class="section-title">{{ section.title }}</text>
          <text class="section-count">{{ section.count }}</text>
        </view>
      </view>

      <view v-if="serviceKey === 'beanList'" class="panel">
        <view class="panel-heading">
          <text class="panel-title">商品分类</text>
          <button class="secondary compact refresh-button" :disabled="customerProductLoading" @tap="loadCustomerProductsWorkspace">刷新</button>
        </view>
        <view class="category-editor-row">
          <input v-model="customerCategoryName" class="input" placeholder="分类名称" />
          <button class="secondary compact" :disabled="submitting" @tap="createCustomerCategory(0)">新增一级</button>
        </view>
        <picker mode="selector" :range="customerCategoryPickerLabels" @change="setSelectedCustomerCategory">
          <view class="picker-field">{{ selectedCustomerCategoryLabel }}</view>
        </picker>
        <button class="secondary" :disabled="submitting || !selectedCustomerCategoryID" @tap="createCustomerCategory(selectedCustomerCategoryID)">新增二级分类</button>
        <view v-if="customerCategories.length" class="category-list">
          <view v-for="item in customerCategories" :key="`category-${item.id}`" class="category-row" :class="{ child: item.parent_id > 0 }">
            <input class="input category-name-input" :value="categoryEditName(item)" @input="setCategoryEditName(item, $event)" />
            <text class="row-sub">{{ item.level === 2 ? '二级' : '一级' }} / {{ item.product_count || 0 }} 个商品</text>
            <view class="row-actions">
              <button class="secondary compact" :disabled="submitting" @tap="saveCategoryName(item)">保存</button>
              <button class="secondary compact" :disabled="submitting" @tap="moveCustomerCategory(item, 'up')">上移</button>
              <button class="secondary compact" :disabled="submitting" @tap="moveCustomerCategory(item, 'down')">下移</button>
              <button class="secondary compact" :disabled="submitting" @tap="removeCustomerCategory(item)">删除</button>
            </view>
          </view>
        </view>
        <text v-else class="row-sub">暂无分类，可先新增一级分类。</text>
      </view>

      <view v-if="serviceKey === 'beanList'" class="panel">
        <text class="panel-title">商品价格表</text>
        <view v-if="factoryPriceTableGroups.length" class="price-table-groups">
          <view v-for="group in factoryPriceTableGroups" :key="`factory-${group.list_type}`" class="list-row">
            <view class="row-head">
              <text class="row-main">{{ group.list_type_label || group.list_type }}</text>
              <text class="status-pill">{{ group.latest_version?.version_no || '暂无版本' }}</text>
            </view>
            <text class="row-sub">{{ priceTableGroupTitle(group) }}</text>
          </view>
        </view>
        <text v-else class="row-sub">暂无工厂供货商品价格表。</text>
      </view>

      <view v-if="serviceKey === 'beanList'" class="panel">
        <text class="panel-title">已发布商品价格表</text>
        <view v-if="customerPriceTableGroups.length" class="price-table-groups">
          <view v-for="group in customerPriceTableGroups" :key="`customer-${group.list_type}`" class="list-row">
            <view class="row-head" @tap="toggleCustomerPriceTableGroup(group)">
              <view class="resale-version-main">
                <text class="row-main">{{ group.list_type_label || group.list_type }}</text>
                <text class="row-sub">最新 {{ group.latest_version?.version_no || '暂无版本' }} / 共 {{ group.price_table_count }} 个版本</text>
              </view>
              <button class="secondary compact">{{ customerPriceTableExpanded(group) ? '收起' : '展开' }}</button>
            </view>
            <view v-if="customerPriceTableExpanded(group)" class="resale-version-list">
              <view v-for="item in group.versions || []" :key="`resale-${item.id}`" class="resale-version-row">
                <view class="resale-version-main">
                  <text class="row-main">{{ item.title || '商品价格表' }}</text>
                  <text class="row-sub">{{ item.version_no || '未标版本' }} / {{ item.published_at || '已发布' }}</text>
                </view>
                <view class="resale-output-actions">
                  <button class="secondary compact" @tap="openResaleOutput(item, 'pdf')">预览 PDF</button>
                  <button class="secondary compact" @tap="openResaleOutput(item, 'png')">预览长图</button>
                </view>
              </view>
            </view>
          </view>
        </view>
        <text v-else class="row-sub">还没有发布自己的商品价格表，可以在下方设置里发布。</text>
      </view>

      <view v-if="serviceKey === 'beanList' && customerProducts.length" class="panel">
        <text class="panel-title">我的商品</text>
        <view v-for="item in customerProducts" :key="`customer-product-${item.id}`" class="list-row">
          <view class="row-head">
            <text class="row-main">{{ item.name }}</text>
            <text class="status-pill">{{ item.list_type_label || item.list_type }}</text>
          </view>
          <text class="row-sub">{{ item.code || '无编号' }} / {{ item.category_name || '未分类' }}</text>
          <button class="secondary compact" :disabled="submitting" @tap="assignProductToSelectedCategory(item)">归类到：{{ selectedCustomerCategoryLabel }}</button>
        </view>
      </view>

      <view v-if="serviceKey === 'beanList'" class="panel resale-editor">
        <view class="panel-heading">
          <text class="panel-title">我的价格表设置</text>
          <button class="secondary compact refresh-button" :disabled="resaleLoading" @tap="loadResaleBeanListWorkspace">刷新</button>
        </view>
        <text class="row-sub">从工厂给客户的价格表复制，选择客户自己的商品和授权阶梯价模板后发布。</text>

        <view class="resale-editor-form">
          <picker mode="selector" :range="resaleSourceLabels" :value="resaleSourcePickerValue" @change="setResaleSource">
            <view class="picker-field">{{ selectedResaleSourceLabel }}</view>
          </picker>
          <picker mode="selector" :range="resaleTemplateLabels" :value="resaleTemplatePickerValue" @change="setResaleTemplate">
            <view class="picker-field">{{ selectedResaleTemplateLabel }}</view>
          </picker>

          <view v-if="resaleDraft" class="resale-form-grid">
            <input v-model="resaleDraft.version_no" class="input" placeholder="版本号，例如 V1" />
            <input class="input" :value="resaleConfigText('brandName')" placeholder="品牌名" @input="setResaleConfigText('brandName', $event)" />
            <textarea class="textarea full" :value="resaleConfigText('brandIntro')" placeholder="价格表说明/品牌介绍" @input="setResaleConfigText('brandIntro', $event)" />
            <textarea v-model="resaleDraft.changelog" class="textarea full" placeholder="版本说明" />
            <input v-model.number="resaleDraft.price_rule.add_amount" class="input" type="number" placeholder="统一加价" />
            <input v-model.number="resaleDraft.price_rule.multiplier" class="input" type="number" placeholder="倍率加价，例如 1.1" />
            <view class="color-presets full">
              <button v-for="preset in resaleStyleColorPresets" :key="preset.key" :class="['color-chip', { active: resaleStylePresetActive(preset) }]" @tap="setResaleStylePreset(preset)">
                <text class="swatch" :style="{ backgroundColor: preset.backgroundColor, color: preset.fontColor }">A</text>
                <text>{{ preset.label }}</text>
              </button>
            </view>
            <input class="input full" :value="resaleConfigText('backgroundImage')" placeholder="背景图 URL，可选" @input="setResaleConfigText('backgroundImage', $event)" />
            <input class="input full" :value="resaleConfigText('logoImage')" placeholder="Logo URL，可选" @input="setResaleConfigText('logoImage', $event)" />
            <view class="segmented full">
              <button :class="['chip', { active: resaleDraft.config.layoutStyle !== 'table' }]" @tap="setResaleLayoutStyle('card')">卡片</button>
              <button :class="['chip', { active: resaleDraft.config.layoutStyle === 'table' }]" @tap="setResaleLayoutStyle('table')">表格</button>
            </view>
            <view class="segmented full">
              <button v-for="count in resaleCardsPerRowOptions" :key="`cards-${count}`" :class="['chip', { active: resaleCardsPerRowActive(count) }]" @tap="setResaleCardsPerRow(count)">{{ count }} 列</button>
            </view>
          </view>

          <view v-if="resaleDraft" class="resale-item-panel">
            <view class="resale-item-head">
              <text class="panel-title">选择商品</text>
              <button class="secondary compact" @tap="setAllResaleItems(!resaleSelectedAll)">{{ resaleSelectedAll ? '取消全选' : '全选' }}</button>
            </view>
            <text class="row-sub">已选 {{ resaleSelectedCount }} / {{ resaleDraftItems.length }}</text>
            <view v-for="bean in resaleDraftItems" :key="resaleBeanListItemKey(bean)" class="resale-item-row">
              <label class="resale-item-check" @tap="toggleResaleItem(bean)">
                <checkbox :checked="resaleItemSelected(bean)" color="#171717" />
                <text class="row-main">{{ bean.name }}</text>
              </label>
              <text class="row-sub">{{ bean.code || '无编号' }} / {{ bean.prices?.[0]?.label || '原档位' }} {{ bean.prices?.[0]?.value || '' }}</text>
              <view class="resale-tag-actions">
                <button class="chip" @tap="setResaleItemBadge(bean, '上新')">上新</button>
                <button class="chip" @tap="setResaleItemBadge(bean, '推荐')">推荐</button>
                <button class="chip" @tap="setResaleItemBadge(bean, '')">清除</button>
              </view>
            </view>
          </view>

          <view v-if="resaleDraft" class="resale-actions">
            <button class="secondary" :disabled="submitting" @tap="submitResaleBeanListDraft">保存草稿</button>
            <button class="primary compact" :disabled="submitting" @tap="submitResaleBeanListPublication">发布商品价格表</button>
          </view>
        </view>
      </view>

      <view v-if="serviceKey === 'beanList' && beanListsForDisplay.length" class="bean-list-native">
        <view v-for="item in beanListsForDisplay" :key="`${item.id}-${item.cache_key || item.version_no}`" class="bean-list-surface" :style="beanListDisplayStyle(item)">
          <view class="bean-list-cover">
            <view class="bean-list-cover-main">
              <image v-if="item.logo_image" class="bean-list-logo" :src="item.logo_image" mode="aspectFit" />
              <text v-if="item.show_version !== false" class="bean-list-version">{{ item.version_no || '当前版本' }}</text>
              <text class="bean-list-title">{{ item.title || '商品价格表' }}</text>
              <text v-if="item.subtitle" class="bean-list-subtitle">{{ item.subtitle }}</text>
              <text v-if="item.brand_intro" class="bean-list-brand-intro">{{ item.brand_intro }}</text>
            </view>
            <text class="bean-list-type">{{ item.list_type_label || item.list_type }}</text>
          </view>
          <text v-if="beanListCacheStatus" class="bean-list-cache-hint">{{ beanListCacheStatus }}</text>
          <view v-if="item.requires_acknowledgement" class="bean-list-update">
            <text class="bean-list-section-label">新版提示</text>
            <text>{{ beanListDiffText(item) }}</text>
          </view>

          <view v-for="group in item.groups || []" :key="`${item.id}-${group.category}`" class="bean-list-group">
            <text v-if="showBeanListCategory(item, group)" class="bean-list-category">{{ group.category }}</text>

            <view v-if="item.layout_style === 'table'" class="bean-list-table">
              <view v-for="bean in group.items" :key="`${item.id}-${group.category}-${bean.code || bean.name}`" class="bean-list-table-row">
                <text class="bean-list-code-cell">{{ bean.code || '-' }}</text>
                <view class="bean-list-table-main">
                  <view class="bean-list-product-head">
                    <text class="bean-list-name">
                      <text v-for="(part, index) in splitBeanListHighlight(bean.name, bean.highlight_terms || [])" :key="`${bean.name}-${index}`" :class="{ red: part.red }">{{ part.text }}</text>
                    </text>
                    <text v-if="bean.badge_label" :class="['bean-list-badge', bean.badge ? `badge-${bean.badge}` : '']">{{ bean.badge_label }}</text>
                  </view>
                  <text v-if="bean.recommended_use" class="bean-list-table-line">
                    出品建议 <text v-for="(part, index) in splitBeanListHighlight(bean.recommended_use, bean.highlight_terms || [])" :key="`${bean.name}-use-${index}`" :class="{ red: part.red }">{{ part.text }}</text>
                  </text>
                  <text v-if="bean.flavor" class="bean-list-table-line">
                    风味 <text v-for="(part, index) in splitBeanListHighlight(bean.flavor, bean.highlight_terms || [])" :key="`${bean.name}-flavor-${index}`" :class="{ red: part.red }">{{ part.text }}</text>
                  </text>
                  <text v-if="bean.description" class="bean-list-table-line">
                    特点 <text v-for="(part, index) in splitBeanListHighlight(bean.description, bean.highlight_terms || [])" :key="`${bean.name}-desc-${index}`" :class="{ red: part.red }">{{ part.text }}</text>
                  </text>
                  <text v-for="quality in beanListQualityLines(bean)" :key="`${bean.name}-quality-${quality.label}`" class="bean-list-table-line">
                    {{ quality.label }} {{ quality.value }}
                  </text>
                </view>
                <view class="bean-list-table-prices">
                  <text v-for="price in bean.prices || []" :key="`${price.label}-${price.value}`" :class="['bean-list-table-price', { red: price.red }]">{{ price.label }} {{ price.value }}</text>
                </view>
              </view>
            </view>

            <view v-else class="bean-list-card-rows">
              <view v-for="(row, rowIndex) in beanListCardRows(group.items, item.cards_per_row || 1)" :key="`${item.id}-${group.category}-row-${rowIndex}`" class="bean-list-card-row">
                <view v-for="bean in row" :key="`${item.id}-${group.category}-${bean.code || bean.name}`" class="bean-list-product">
                  <view class="bean-list-product-head">
                    <text v-if="bean.code" class="bean-list-code">{{ bean.code }}</text>
                    <text class="bean-list-name">
                      <text v-for="(part, index) in splitBeanListHighlight(bean.name, bean.highlight_terms || [])" :key="`${bean.name}-${index}`" :class="{ red: part.red }">{{ part.text }}</text>
                    </text>
                    <text v-if="bean.badge_label" :class="['bean-list-badge', bean.badge ? `badge-${bean.badge}` : '']">{{ bean.badge_label }}</text>
                  </view>
                  <view v-if="bean.recommended_use" class="bean-list-detail">
                    <text class="bean-list-detail-label">出品建议</text>
                    <text class="bean-list-detail-value">
                      <text v-for="(part, index) in splitBeanListHighlight(bean.recommended_use, bean.highlight_terms || [])" :key="`${bean.name}-use-${index}`" :class="{ red: part.red }">{{ part.text }}</text>
                    </text>
                  </view>
                  <view v-if="bean.flavor" class="bean-list-detail">
                    <text class="bean-list-detail-label">风味</text>
                    <text class="bean-list-detail-value">
                      <text v-for="(part, index) in splitBeanListHighlight(bean.flavor, bean.highlight_terms || [])" :key="`${bean.name}-flavor-${index}`" :class="{ red: part.red }">{{ part.text }}</text>
                    </text>
                  </view>
                  <view v-if="bean.description" class="bean-list-detail">
                    <text class="bean-list-detail-label">特点</text>
                    <text class="bean-list-detail-value">
                      <text v-for="(part, index) in splitBeanListHighlight(bean.description, bean.highlight_terms || [])" :key="`${bean.name}-desc-${index}`" :class="{ red: part.red }">{{ part.text }}</text>
                    </text>
                  </view>
                  <view v-for="quality in beanListQualityLines(bean)" :key="`${bean.name}-quality-${quality.label}`" class="bean-list-detail">
                    <text class="bean-list-detail-label">{{ quality.label }}</text>
                    <text class="bean-list-detail-value">{{ quality.value }}</text>
                  </view>
                  <view v-if="bean.prices?.length" class="bean-list-price-block">
                    <text class="bean-list-section-label">报价</text>
                    <view class="bean-list-prices">
                      <view v-for="price in bean.prices" :key="`${price.label}-${price.value}`" class="bean-list-price">
                        <text :class="{ red: price.red }">{{ price.label }}</text>
                        <text :class="['bean-list-price-value', { red: price.red }]">{{ price.value }}</text>
                      </view>
                    </view>
                  </view>
                </view>
              </view>
            </view>
          </view>

          <view v-if="item.show_changelog !== false && item.changelog" class="bean-list-changelog">
            <text class="bean-list-section-label">更新</text>
            <text>{{ item.changelog }}</text>
          </view>
          <view class="bean-list-footer">
            <text>{{ item.brand_name || '棵凡咖啡' }}</text>
            <text>{{ item.version_no }}</text>
          </view>
        </view>
      </view>

      <view v-if="serviceKey === 'orders'" class="panel filter-panel">
        <text class="panel-title">订单查询</text>
        <input
          v-model="orderSearch.keyword"
          class="input"
          confirm-type="search"
          placeholder="收件人/地址/产品"
          @confirm="applyOrderFilters"
        />
        <view class="date-presets">
          <button class="chip" @tap="applyDatePreset('today')">今天</button>
          <button class="chip" @tap="applyDatePreset('last3')">最近三天</button>
          <button class="chip" @tap="applyDatePreset('last7')">最近7天</button>
          <button class="chip" @tap="applyDatePreset('month')">本月</button>
        </view>
        <view class="date-range">
          <picker mode="date" :value="orderSearch.date_from" @change="setOrderDateFrom">
            <view class="picker-field">{{ orderSearch.date_from || '开始日期' }}</view>
          </picker>
          <picker mode="date" :value="orderSearch.date_to" @change="setOrderDateTo">
            <view class="picker-field">{{ orderSearch.date_to || '结束日期' }}</view>
          </picker>
        </view>
        <view class="status-filters">
          <picker mode="selector" :range="processStatusPickerOptions" :value="statusPickerValue(processStatusPickerOptions, orderSearch.process_status)" @change="setOrderProcessStatus">
            <view class="picker-field status-picker">{{ orderSearch.process_status || '生产状态' }}</view>
          </picker>
          <picker mode="selector" :range="payStatusPickerOptions" :value="statusPickerValue(payStatusPickerOptions, orderSearch.pay_status)" @change="setOrderPayStatus">
            <view class="picker-field status-picker">{{ orderSearch.pay_status || '收款状态' }}</view>
          </picker>
          <picker mode="selector" :range="shipStatusPickerOptions" :value="statusPickerValue(shipStatusPickerOptions, orderSearch.ship_status)" @change="setOrderShipStatus">
            <view class="picker-field status-picker">{{ orderSearch.ship_status || '发货状态' }}</view>
          </picker>
        </view>
        <view class="filter-actions">
          <button class="secondary" @tap="clearOrderFilters">清除</button>
          <button class="primary compact" @tap="applyOrderFilters">查询</button>
        </view>
      </view>

      <view v-if="serviceKey === 'settlement'" class="panel filter-panel">
        <text class="panel-title">账期筛选</text>
        <view class="date-presets bill-presets">
          <button class="chip" @tap="applyDatePreset('week')">本周</button>
          <button class="chip" @tap="applyDatePreset('month')">本月</button>
          <button class="chip" @tap="applyDatePreset('year')">本年</button>
        </view>
        <view class="date-range">
          <picker mode="date" :value="orderSearch.date_from" @change="setOrderDateFrom">
            <view class="picker-field">{{ orderSearch.date_from || '开始日期' }}</view>
          </picker>
          <picker mode="date" :value="orderSearch.date_to" @change="setOrderDateTo">
            <view class="picker-field">{{ orderSearch.date_to || '结束日期' }}</view>
          </picker>
        </view>
        <view class="billing-status-row">
          <picker mode="selector" :range="payStatusPickerOptions" :value="statusPickerValue(payStatusPickerOptions, orderSearch.pay_status)" @change="setOrderPayStatus">
            <view class="picker-field status-picker">{{ orderSearch.pay_status || '收款状态' }}</view>
          </picker>
        </view>
        <view class="filter-actions">
          <button class="secondary" @tap="clearOrderFilters">重置</button>
          <button class="primary compact" @tap="applyOrderFilters">查询</button>
        </view>
      </view>

      <view v-if="page?.products?.length" class="panel">
        <text class="panel-title">现货商品</text>
        <view v-for="item in page.products" :key="item.id" class="list-row">
          <text class="row-main">{{ item.name }}</text>
          <text class="row-sub">{{ productKindLabel(item) }} / {{ item.roast_level || '未烘焙' }} / 默认 ¥{{ item.default_price }}</text>
        </view>
      </view>

      <view v-if="serviceKey === 'settlement' && page?.orders?.length" class="panel bill-panel">
        <text class="panel-title">订单账单</text>
        <view v-for="item in page.orders" :key="item.id" class="list-row bill-row">
          <view class="row-head">
            <text class="row-main order-link" @tap="openOrderFromBill(item.order_no)">{{ item.order_no || '未编号订单' }}</text>
            <text class="price">¥{{ item.grand_total || '0.00' }}</text>
          </view>
          <view class="bill-meta">
            <text class="status-pill" :class="{ unpaid: !item.pay_status || item.pay_status.includes('未') }">{{ item.pay_status || '未付款' }}</text>
            <text class="row-sub">{{ item.order_date || '未填写日期' }}</text>
          </view>
          <text class="row-sub">收款方式：{{ paymentMethodText(item.pay_status, item.payment_method) }}</text>
        </view>
      </view>

      <view v-if="serviceKey !== 'settlement' && page?.orders?.length" class="panel">
        <text class="panel-title">{{ orderPanelTitle }}</text>
        <view v-for="item in page.orders" :key="item.id" class="list-row">
          <view class="row-head">
            <text class="row-main">{{ item.order_no || '未编号订单' }}</text>
            <text class="price">¥{{ item.grand_total || '0.00' }}</text>
          </view>
          <text v-if="item.receiver_name || item.receiver_phone || item.receiver_address" class="row-sub">
            收件人：{{ item.receiver_name || '未填写' }} {{ item.receiver_phone || '' }} {{ item.receiver_address || '' }}
          </text>
          <text class="row-sub">{{ item.order_date || '未填写日期' }} / {{ item.process_status || '生产待处理' }} / {{ item.pay_status || '未收款' }} / {{ item.ship_status || '待发货' }}</text>
          <view v-if="item.items?.length" class="order-items">
            <view v-for="line in item.items" :key="line.id" class="item-line">
              <text>{{ line.item_name }} {{ line.spec }}</text>
              <text>{{ line.qty }}{{ line.unit }} x ¥{{ line.unit_price }} = ¥{{ line.line_total }}</text>
              <text v-if="line.bean_list_version_no" class="line-meta">价格表版本：{{ line.bean_list_version_no }}</text>
            </view>
          </view>
          <text class="row-sub">运费：¥{{ item.shipping_amount || '0.00' }}</text>
          <text class="row-sub">物流：{{ item.ship_tracking_no || '暂无单号' }}</text>
          <view class="document-actions">
            <button class="secondary compact" @tap="openOrderDocument(item.sales_order_url)">销售单</button>
            <button class="secondary compact" @tap="openOrderDocument(item.delivery_note_url)">出库单</button>
          </view>
        </view>
      </view>

      <view v-if="page?.direct_ship_batches?.length" class="panel">
        <text class="panel-title">一件代发批次</text>
        <view v-for="item in page.direct_ship_batches" :key="item.id" class="list-row">
          <text class="row-main">{{ item.batch_no }}</text>
          <text class="row-sub">{{ item.source_name }} / {{ item.status }} / {{ item.total_rows }} 单</text>
        </view>
      </view>

      <view v-if="page?.inventory?.length" class="panel">
        <text class="panel-title">库存</text>
        <view v-for="item in page.inventory" :key="item.id" class="list-row">
          <text class="row-main">{{ item.item_name }}</text>
          <text class="row-sub">{{ item.warehouse }} / {{ item.qty_g }}g / {{ item.qty_units }} 件</text>
        </view>
      </view>

      <view v-if="page?.processing_requests?.length" class="panel">
        <text class="panel-title">加工申请</text>
        <view v-for="item in page.processing_requests" :key="item.id" class="list-row">
          <text class="row-main">{{ item.request_no }}</text>
          <text class="row-sub">{{ item.status }} / {{ item.input_qty_g }}g -> {{ item.target_qty }} 件</text>
        </view>
      </view>

      <view v-if="page?.fee_items?.length" class="panel">
        <text class="panel-title">费用明细</text>
        <view v-for="item in page.fee_items" :key="item.id" class="list-row">
          <text class="row-main">{{ item.fee_type }} ¥{{ item.amount }}</text>
          <text class="row-sub">{{ item.status }} / {{ item.occurred_at }}</text>
        </view>
      </view>

      <view v-if="page?.settlement_batches?.length" class="panel">
        <text class="panel-title">结算单</text>
        <view v-for="item in page.settlement_batches" :key="item.id" class="list-row">
          <text class="row-main">{{ item.settlement_no }} ¥{{ item.total_amount }}</text>
          <text class="row-sub">{{ item.status }} / {{ item.period_from }} 至 {{ item.period_to }}</text>
        </view>
      </view>

      <view v-if="page && !hasDisplayData" class="state">
        <text>暂无数据</text>
      </view>
    </view>

    <MainTabBar :current="mainTab" />
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 32rpx 32rpx 160rpx;
  background: #f7f2ea;
  box-sizing: border-box;
}

.page.theme-coffee-factory {
  background: #f7f2ea;
}

.page.theme-clean-ops {
  background: #f5f7f6;
}

.page.theme-premium-partner {
  background: #fbf7ef;
}

.header {
  display: flex;
  flex-direction: column;
  gap: 14rpx;
  padding: 30rpx 28rpx 34rpx;
  margin-bottom: 24rpx;
  border-radius: 28rpx;
  background: linear-gradient(135deg, #2b2118 0%, #6b4b2b 100%);
}

.theme-clean-ops .header {
  background: #ffffff;
  border: 1rpx solid #dfe7e2;
}

.theme-premium-partner .header {
  background: linear-gradient(135deg, #111111 0%, #513018 55%, #b88a46 100%);
}

.eyebrow {
  color: rgba(255, 248, 235, 0.78);
  font-size: 24rpx;
  font-weight: 900;
}

.theme-clean-ops .eyebrow {
  color: #28624a;
}

.title {
  color: #fff8eb;
  font-size: 42rpx;
  font-weight: 900;
  line-height: 1.18;
}

.theme-clean-ops .title {
  color: #14201a;
}

.subtitle {
  color: rgba(255, 248, 235, 0.82);
  font-size: 26rpx;
  line-height: 1.55;
}

.theme-clean-ops .subtitle {
  color: #66756c;
}

.metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18rpx;
  margin-bottom: 20rpx;
}

.metric,
.panel,
.section-row {
  background: #ffffff;
  border: 1rpx solid #e8e8e8;
  border-radius: 8rpx;
}

.theme-clean-ops .metric,
.theme-clean-ops .panel,
.theme-clean-ops .section-row {
  border-color: #dde7e1;
}

.theme-premium-partner .metric,
.theme-premium-partner .panel,
.theme-premium-partner .section-row {
  border-color: #eadab7;
  background: #fffdf8;
}

.metric {
  min-height: 110rpx;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8rpx;
  padding: 20rpx;
  box-sizing: border-box;
}

.metric-value {
  color: #171717;
  font-size: 36rpx;
  font-weight: 700;
}

.metric-label,
.row-sub {
  color: #666666;
  font-size: 24rpx;
  line-height: 1.5;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
  padding: 24rpx;
  margin-bottom: 20rpx;
  box-sizing: border-box;
}

.panel-title {
  color: #171717;
  font-size: 30rpx;
  font-weight: 700;
}

.input,
.textarea {
  width: 100%;
  min-height: 76rpx;
  padding: 0 20rpx;
  background: #f8f8f8;
  border: 1rpx solid #e2e2e2;
  border-radius: 8rpx;
  color: #171717;
  font-size: 26rpx;
  box-sizing: border-box;
}

.textarea {
  min-height: 132rpx;
  padding-top: 18rpx;
}

.primary {
  width: 100%;
  min-height: 82rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #171717;
  color: #ffffff;
  border-radius: 8rpx;
  font-size: 28rpx;
}

.theme-clean-ops .primary {
  background: #173b2e;
}

.theme-premium-partner .primary {
  background: #17120d;
  color: #f8ddb0;
}

.primary.compact,
.secondary {
  min-height: 72rpx;
  font-size: 26rpx;
}

.secondary {
  display: flex;
  align-items: center;
  justify-content: center;
  background: #ffffff;
  color: #171717;
  border: 1rpx solid #d8d8d8;
  border-radius: 8rpx;
}

.theme-clean-ops .secondary {
  border-color: #c9d8d0;
  color: #173b2e;
}

.theme-premium-partner .secondary {
  border-color: #eadab7;
  color: #6b431a;
}

.filter-panel {
  gap: 16rpx;
}

.date-presets {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10rpx;
}

.bill-presets {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.chip {
  min-height: 64rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0;
  padding: 0 6rpx;
  background: #f8f8f8;
  color: #171717;
  border: 1rpx solid #e2e2e2;
  border-radius: 8rpx;
  font-size: 22rpx;
  line-height: 1.1;
}

.theme-clean-ops .chip {
  background: #f5f7f6;
  border-color: #d7e4dd;
  color: #173b2e;
}

.theme-premium-partner .chip {
  background: #fff8eb;
  border-color: #eadab7;
  color: #6b431a;
}

.date-range,
.status-filters,
.filter-actions {
  display: grid;
  gap: 12rpx;
}

.date-range,
.filter-actions {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.status-filters {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.billing-status-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
}

.picker-field {
  min-height: 72rpx;
  display: flex;
  align-items: center;
  padding: 0 18rpx;
  background: #f8f8f8;
  border: 1rpx solid #e2e2e2;
  border-radius: 8rpx;
  color: #171717;
  font-size: 25rpx;
  box-sizing: border-box;
}

.theme-clean-ops .input,
.theme-clean-ops .textarea,
.theme-clean-ops .picker-field {
  background: #f7faf8;
  border-color: #d7e4dd;
}

.theme-premium-partner .input,
.theme-premium-partner .textarea,
.theme-premium-partner .picker-field {
  background: #fffaf2;
  border-color: #eadab7;
}

.status-picker {
  justify-content: center;
  padding: 0 10rpx;
  font-size: 23rpx;
}

.section-list {
  display: flex;
  flex-direction: column;
  gap: 14rpx;
  margin-bottom: 20rpx;
}

.section-row {
  min-height: 86rpx;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24rpx;
}

.section-title,
.row-main {
  color: #171717;
  font-size: 28rpx;
  font-weight: 600;
}

.row-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16rpx;
}

.price {
  color: #171717;
  font-size: 28rpx;
  font-weight: 700;
  white-space: nowrap;
}

.bill-panel {
  gap: 14rpx;
}

.bill-row {
  gap: 12rpx;
}

.order-link {
  color: #7a4b12;
  text-decoration: underline;
  text-decoration-thickness: 1rpx;
  text-underline-offset: 5rpx;
}

.bill-meta {
  display: flex;
  align-items: center;
  gap: 12rpx;
  flex-wrap: wrap;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  min-height: 42rpx;
  padding: 0 14rpx;
  border-radius: 8rpx;
  background: #eef7ef;
  color: #28624a;
  font-size: 22rpx;
  font-weight: 700;
}

.status-pill.unpaid {
  background: #fff0e3;
  color: #9a4b10;
}

.order-items {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  padding: 12rpx;
  background: #f8f8f8;
  border-radius: 8rpx;
}

.document-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12rpx;
}

.panel-heading,
.resale-item-head,
.category-editor-row,
.row-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16rpx;
}

.category-editor-row .input {
  flex: 1;
}

.category-list,
.price-table-groups {
  display: flex;
  flex-direction: column;
  gap: 12rpx;
}

.category-row {
  display: flex;
  flex-direction: column;
  gap: 10rpx;
  padding: 14rpx 0;
  border-top: 1rpx solid #eeeeee;
}

.category-row.child {
  padding-left: 24rpx;
}

.category-name-input {
  min-height: 68rpx;
}

.row-actions {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8rpx;
}

.refresh-button {
  width: 144rpx;
  flex: 0 0 auto;
}

.resale-editor {
  gap: 20rpx;
}

.resale-version-list,
.resale-editor-form,
.resale-item-panel {
  display: flex;
  flex-direction: column;
  gap: 14rpx;
}

.resale-version-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 190rpx;
  align-items: center;
  gap: 14rpx;
  padding: 16rpx 0;
  border-top: 1rpx solid #eeeeee;
}

.resale-version-row:first-child {
  border-top: 0;
}

.resale-version-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6rpx;
}

.resale-output-actions,
.resale-actions,
.segmented,
.resale-tag-actions,
.color-presets {
  display: grid;
  gap: 10rpx;
}

.resale-output-actions,
.resale-actions {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.resale-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14rpx;
}

.resale-form-grid .full,
.segmented {
  grid-column: 1 / -1;
}

.segmented {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.color-presets {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.color-chip {
  min-height: 72rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10rpx;
  margin: 0;
  padding: 0 10rpx;
  background: #ffffff;
  border: 1rpx solid #e2e2e2;
  border-radius: 8rpx;
  color: #171717;
  font-size: 24rpx;
}

.color-chip.active {
  border-color: #171717;
  box-shadow: inset 0 0 0 2rpx #171717;
}

.swatch {
  width: 42rpx;
  height: 42rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  border: 1rpx solid rgba(0, 0, 0, 0.14);
  font-size: 22rpx;
  font-weight: 800;
}

.chip.active {
  background: #171717;
  color: #ffffff;
  border-color: #171717;
}

.resale-item-row {
  display: flex;
  flex-direction: column;
  gap: 10rpx;
  padding: 16rpx 0;
  border-top: 1rpx solid #eeeeee;
}

.resale-item-row:first-of-type {
  border-top: 0;
}

.resale-item-check {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 12rpx;
}

.resale-tag-actions {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.bean-list-native {
  margin-bottom: 20rpx;
}

.theme-clean-ops .bean-list-native,
.theme-premium-partner .bean-list-native {
  border-radius: 16rpx;
}

.bean-list-surface {
  display: flex;
  min-height: 100vh;
  flex-direction: column;
  gap: 26rpx;
  padding: 34rpx 28rpx 40rpx;
  border: 1rpx solid rgba(0, 0, 0, 0.12);
  border-radius: 8rpx;
  background-color: #f8f1e5;
  background-position: center;
  background-size: cover;
  box-sizing: border-box;
}

.bean-list-cover {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18rpx;
  padding-bottom: 22rpx;
  border-bottom: 6rpx solid currentColor;
}

.bean-list-cover-main {
  min-width: 0;
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  gap: 10rpx;
}

.bean-list-logo {
  width: 96rpx;
  height: 96rpx;
  margin-bottom: 4rpx;
}

.bean-list-version,
.bean-list-cache-hint,
.bean-list-footer {
  color: #666666;
  font-size: 23rpx;
  line-height: 1.45;
}

.bean-list-title {
  font-size: 46rpx;
  font-weight: 900;
  line-height: 1.15;
}

.bean-list-subtitle,
.bean-list-brand-intro {
  font-size: 26rpx;
  line-height: 1.5;
}

.bean-list-type {
  flex: 0 0 auto;
  border: 3rpx solid currentColor;
  border-radius: 999rpx;
  padding: 8rpx 18rpx;
  font-size: 24rpx;
  font-weight: 900;
}

.bean-list-group {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
}

.bean-list-category {
  padding: 10rpx 0 10rpx 16rpx;
  border-left: 8rpx solid currentColor;
  font-size: 32rpx;
  font-weight: 900;
  line-height: 1.25;
}

.bean-list-card-rows {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
}

.bean-list-card-row {
  display: flex;
  gap: 18rpx;
  align-items: stretch;
}

.bean-list-product {
  min-width: 0;
  display: flex;
  flex: 1 1 0;
  flex-direction: column;
  gap: 16rpx;
  padding: 18rpx;
  border: 1rpx solid rgba(0, 0, 0, 0.18);
  border-radius: 8rpx;
  background: rgba(255, 255, 255, 0.86);
  box-sizing: border-box;
}

.bean-list-product-head {
  display: flex;
  align-items: center;
  gap: 12rpx;
}

.bean-list-code,
.bean-list-code-cell {
  flex: 0 0 auto;
  border: 1rpx solid currentColor;
  border-radius: 8rpx;
  padding: 6rpx 10rpx;
  font-size: 26rpx;
  font-weight: 900;
  line-height: 1.1;
}

.bean-list-name {
  min-width: 0;
  flex: 1 1 auto;
  font-size: 32rpx;
  font-weight: 900;
  line-height: 1.2;
  overflow-wrap: anywhere;
}

.bean-list-badge {
  flex: 0 0 auto;
  border: 1rpx solid currentColor;
  border-radius: 6rpx;
  padding: 2rpx 8rpx;
  font-size: 20rpx;
  font-weight: 900;
}

.badge-new,
.red {
  color: #d81717;
}

.badge-thumb,
.badge-medal {
  color: #7a4d00;
}

.bean-list-detail {
  display: grid;
  grid-template-columns: 112rpx minmax(0, 1fr);
  gap: 10rpx;
  font-size: 24rpx;
  font-weight: 700;
  line-height: 1.55;
}

.bean-list-detail-label,
.bean-list-section-label {
  color: #777777;
  font-weight: 900;
}

.bean-list-detail-value {
  min-width: 0;
  overflow-wrap: anywhere;
}

.bean-list-price-block {
  margin-top: auto;
  padding-top: 8rpx;
}

.bean-list-prices {
  display: flex;
  flex-direction: column;
  gap: 10rpx;
  margin-top: 8rpx;
}

.bean-list-price {
  min-height: 70rpx;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12rpx;
  padding: 12rpx;
  border: 1rpx solid rgba(55, 128, 55, 0.18);
  border-radius: 8rpx;
  background: #dff5d8;
  font-size: 24rpx;
  font-weight: 800;
  box-sizing: border-box;
}

.bean-list-price:nth-child(even) {
  background: #d9ebf8;
  border-color: rgba(46, 93, 125, 0.18);
}

.bean-list-price-value {
  flex: 0 0 auto;
  font-size: 30rpx;
  font-weight: 950;
}

.bean-list-table {
  overflow: hidden;
  border: 1rpx solid rgba(0, 0, 0, 0.22);
  background: rgba(255, 255, 255, 0.84);
}

.bean-list-table-row {
  display: grid;
  grid-template-columns: 82rpx minmax(0, 1fr) 164rpx;
  border-top: 1rpx solid rgba(0, 0, 0, 0.22);
}

.bean-list-table-row:first-child {
  border-top: 0;
}

.bean-list-code-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-right: 1rpx solid rgba(0, 0, 0, 0.22);
  border-radius: 0;
}

.bean-list-table-main,
.bean-list-table-prices {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  padding: 12rpx;
  border-right: 1rpx solid rgba(0, 0, 0, 0.22);
  box-sizing: border-box;
}

.bean-list-table-prices {
  border-right: 0;
}

.bean-list-table-line,
.bean-list-table-price {
  color: #444444;
  font-size: 22rpx;
  line-height: 1.45;
}

.bean-list-table-price {
  font-weight: 900;
}

.bean-list-update,
.bean-list-changelog {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  margin-top: 8rpx;
  padding-top: 18rpx;
  border-top: 2rpx solid rgba(0, 0, 0, 0.18);
  font-size: 24rpx;
  line-height: 1.6;
}

.bean-list-update {
  padding: 18rpx;
  border: 2rpx solid rgba(216, 23, 23, 0.26);
  border-radius: 8rpx;
  background: rgba(255, 245, 245, 0.9);
}

.bean-list-footer {
  display: flex;
  justify-content: space-between;
  gap: 18rpx;
  margin-top: 4rpx;
}

.item-line {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 16rpx;
  color: #333333;
  font-size: 24rpx;
  line-height: 1.45;
}

.line-meta {
  width: 100%;
  color: #7a6a55;
  font-size: 22rpx;
}

.section-count {
  color: #6f5d2e;
  font-size: 30rpx;
  font-weight: 700;
}

.theme-clean-ops .section-count {
  color: #28624a;
}

.theme-premium-partner .section-count {
  color: #8a5c20;
}

.list-row {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  padding: 18rpx 0;
  border-top: 1rpx solid #eeeeee;
}

.list-row:first-of-type {
  border-top: 0;
}

.state {
  min-height: 160rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #666666;
  font-size: 28rpx;
}

.error {
  color: #b42318;
}
</style>
