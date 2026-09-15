<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import {
  cancelDirectShipRequest,
  createDirectShipRequest,
  createProductOrder,
  fetchDirectShipCatalog,
  fetchDirectShipRequests,
  fetchProductOrderCatalog,
  fetchRecipientAddresses,
  previewDirectShipRequest,
  previewProductOrder,
  type CustomerRecipientAddress,
  type DirectShipCatalog,
  type DirectShipPreview,
  type DirectShipRequest,
  type EmployeeOrderProductFamily,
  type EmployeeOrderProductSpec,
} from '../api/customerPortal'
import { useCustomerOrderDraftStore, type CustomerOrderMode, type CustomerOrderRecipient } from '../stores/customerOrderDraft'
import { employeeOrderProductFamilyKey, productSpecLabel, shanghaiToday } from '../utils/employeeOrder'
import { directShipStatusLabel } from '../utils/customerFulfillment'
import {
  buildDirectShipDraftItems,
  createDirectShipDraftLine,
  directShipDraftValidation,
  selectDirectShipDraftProduct,
  selectDirectShipDraftSpec,
  type DirectShipDraftLine,
} from '../utils/directShipDraft'
import {
  directShipDatePresetRange,
  directShipRequestTitle,
  normalizeDirectShipDateRange,
  type DirectShipDatePreset,
} from '../utils/directShipFilters'
import { filterRecipientAddresses, recipientAddressSummary, tierQuantityLabel } from '../utils/customerOrderEntry'
import ProductFamilyPickerSheet from './ProductFamilyPickerSheet.vue'
import ProductSpecPickerSheet from './ProductSpecPickerSheet.vue'

const props = withDefaults(defineProps<{ token: string; customerId: number; showCreate?: boolean; orderMode?: CustomerOrderMode }>(), { showCreate: true, orderMode: 'direct_ship' })
type PickerChangeEvent = { detail?: { value?: string | number } }
type RecipientEvent = { customerID?: number; orderMode?: CustomerOrderMode }

const orderDraft = useCustomerOrderDraftStore()
const loading = ref(false)
const submitting = ref(false)
const previewing = ref(false)
const errorMessage = ref('')
const catalog = ref<DirectShipCatalog>({ current_customer_id: 0, product_families: [] })
const requests = ref<DirectShipRequest[]>([])
const recipientAddresses = ref<CustomerRecipientAddress[]>([])
const selectedRecipient = ref<CustomerOrderRecipient | null>(null)
const recipientSelectorOpen = ref(false)
const recipientQuery = ref('')
const orderDate = ref(shanghaiToday())
const note = ref('')
const lines = ref<DirectShipDraftLine[]>([createDirectShipDraftLine()])
const preview = ref<DirectShipPreview | null>(null)
const productSelectorOpen = ref(false)
const specSelectorOpen = ref(false)
const editingLineKey = ref('')
const idempotencyKey = ref(newIdempotencyKey())
const shipmentQueryInput = ref('')
const shipmentQuery = ref('')
const shippedFrom = ref('')
const shippedTo = ref('')
const activeDatePreset = ref<DirectShipDatePreset | ''>('')
const currentPage = ref(1)
const pageLimit = ref(10)
const totalRows = ref(0)
const totalPages = ref(1)
const jumpPage = ref('1')
const pageLimitOptions = [10, 20, 50]
const pageLimitLabels = pageLimitOptions.map((value) => `每页 ${value} 条`)
let loadVersion = 0
let previewVersion = 0
let previewTimer: ReturnType<typeof setTimeout> | undefined

const isProductOrder = computed(() => props.orderMode === 'product_order')
const createTitle = computed(() => isProductOrder.value ? '新建现货订单' : '新建代发订单')
const submitTitle = computed(() => isProductOrder.value ? '提交现货订单' : '提交代发订单')
const selectedSummary = computed(() => lines.value.filter((line) => Number(line.product_id || 0) > 0).map((line) => `${line.product_name} ${line.spec_label} × ${line.qty}`).join('；'))
const filteredRecipients = computed(() => filterRecipientAddresses(recipientAddresses.value, recipientQuery.value))
const selectedRecipientSummary = computed(() => recipientAddressSummary(selectedRecipient.value))
const totalAmount = computed(() => preview.value?.total_amount ?? 0)
const activeSpecLine = computed(() => lines.value.find((line) => line.key === editingLineKey.value))
const activeSpecFamily = computed(() => activeSpecLine.value ? familyForLine(activeSpecLine.value) : undefined)

function newIdempotencyKey(): string {
  return `mini-${props.orderMode === 'product_order' ? 'po' : 'ds'}-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

function currentOrderMode(): CustomerOrderMode { return isProductOrder.value ? 'product_order' : 'direct_ship' }

function saveDraft() {
  orderDraft.saveDraft(props.customerId, currentOrderMode(), {
    order_date: orderDate.value,
    recipient: selectedRecipient.value,
    note: note.value,
    lines: lines.value,
    idempotency_key: idempotencyKey.value,
  })
}

function restoreDraft() {
  const draft = orderDraft.restoreDraft(props.customerId, currentOrderMode())
  if (!draft) return
  orderDate.value = draft.order_date || shanghaiToday()
  selectedRecipient.value = draft.recipient
  note.value = draft.note || ''
  lines.value = draft.lines?.length ? draft.lines : [createDirectShipDraftLine()]
  idempotencyKey.value = draft.idempotency_key || newIdempotencyKey()
}

function consumeReturnedRecipient(event?: RecipientEvent) {
  if (event && (Number(event.customerID || 0) !== props.customerId || event.orderMode !== currentOrderMode())) return
  const recipient = orderDraft.consumeRecipient(props.customerId, currentOrderMode())
  if (!recipient) return
  applyRecipientAddress(recipient)
  saveDraft()
}

async function load() {
  const version = ++loadVersion
  loading.value = true
  errorMessage.value = ''
  try {
    if (props.showCreate) {
      const [value, addressData] = await Promise.all([
        isProductOrder.value ? fetchProductOrderCatalog(props.token) : fetchDirectShipCatalog(props.token),
        fetchRecipientAddresses(props.token),
      ])
      if (version !== loadVersion) return
      catalog.value = value
      recipientAddresses.value = addressData.rows || []
      const defaultAddress = recipientAddresses.value.find((row) => row.is_default)
      if (defaultAddress && !selectedRecipient.value) applyRecipientAddress(defaultAddress, false)
    } else {
      const value = await fetchDirectShipRequests(props.token, { q: shipmentQuery.value, shipped_from: shippedFrom.value, shipped_to: shippedTo.value, page: currentPage.value, limit: pageLimit.value })
      if (version !== loadVersion) return
      requests.value = value.rows || []
      totalRows.value = Math.max(0, Number(value.total ?? requests.value.length) || 0)
      pageLimit.value = Math.max(1, Number(value.limit ?? pageLimit.value) || pageLimit.value)
      totalPages.value = Math.max(1, Number(value.total_pages) || Math.ceil(totalRows.value / pageLimit.value) || 1)
      currentPage.value = Math.min(totalPages.value, Math.max(1, Number(value.page ?? currentPage.value) || 1))
      jumpPage.value = String(currentPage.value)
    }
  } catch (error) {
    if (version !== loadVersion) return
    errorMessage.value = error instanceof Error ? error.message : '信息加载失败'
  } finally {
    if (version === loadVersion) loading.value = false
  }
}

function applyRecipientAddress(address: CustomerOrderRecipient, schedule = true) {
  selectedRecipient.value = { ...address }
  recipientSelectorOpen.value = false
  invalidatePreview(schedule)
}

function openRecipientSelector() {
  if (submitting.value) return
  recipientQuery.value = ''
  recipientSelectorOpen.value = true
}

function closeRecipientSelector() { recipientSelectorOpen.value = false }

function openRecipientEditor(addressID = 0) {
  saveDraft()
  recipientSelectorOpen.value = false
  const id = Math.max(0, Number(addressID || 0))
  uni.navigateTo({ url: `/pages/customer-addresses/customer-addresses?order_mode=${currentOrderMode()}${id ? `&address_id=${id}` : ''}` })
}

function openPriceTablePreview() {
  saveDraft()
  uni.navigateTo({ url: `/pages/order-price-table-preview/order-price-table-preview?usage_code=${currentOrderMode()}` })
}

function validateRecipient(): string {
  const row = selectedRecipient.value
  return !row?.recipient_name?.trim() || !row.phone?.trim() || !row.detail_address?.trim() ? '请选择完整的收件客户信息' : ''
}

function invalidatePreview(schedule = true) {
  previewVersion += 1
  preview.value = null
  if (previewTimer) clearTimeout(previewTimer)
  if (schedule) schedulePreview()
}

function schedulePreview() {
  if (!props.showCreate || validate()) return
  previewTimer = setTimeout(() => { void previewRequest(true) }, 350)
}

function addLine() {
  if (submitting.value) return
  lines.value.push(createDirectShipDraftLine())
  invalidatePreview()
}

function removeLine(index: number) {
  if (submitting.value) return
  if (lines.value.length === 1) lines.value.splice(0, 1, createDirectShipDraftLine(lines.value[0]?.key))
  else lines.value.splice(index, 1)
  invalidatePreview()
}

function familyForLine(line: DirectShipDraftLine): EmployeeOrderProductFamily | undefined {
  return catalog.value.product_families.find((family) => employeeOrderProductFamilyKey(family) === line.product_family_key)
}

function selectedSpecForLine(line: DirectShipDraftLine): EmployeeOrderProductSpec | undefined {
  return familyForLine(line)?.specs.find((spec) => Number(line.bom_spec_id || 0) > 0
    ? Number(spec.bom_spec_id || 0) === Number(line.bom_spec_id) && Number(spec.bom_variant_id || 0) === Number(line.bom_variant_id || 0)
    : Number(spec.sku_id || spec.product_id || 0) === Number(line.product_id || 0))
}

function allSpecLabels(line: DirectShipDraftLine): string {
  return familyForLine(line)?.specs.map(productSpecLabel).join('、') || ''
}

function previewItemForLine(line: DirectShipDraftLine) {
  return (preview.value?.items || []).find((item) => Number(item.product_id || 0) === Number(line.product_id || 0)
    && (Number(line.bom_spec_id || 0) > 0
      ? Number(item.bom_spec_id || 0) === Number(line.bom_spec_id || 0)
        && (Number(line.bom_variant_id || 0) <= 0 || Number(item.bom_variant_id || 0) === Number(line.bom_variant_id || 0))
      : Number(item.spec_g || 0) === Number(line.spec_g || 0)))
}

function lineUnitPrice(line: DirectShipDraftLine): number { return Number(previewItemForLine(line)?.unit_price || 0) }
function lineAmount(line: DirectShipDraftLine): number { return lineUnitPrice(line) * Number(line.qty || 0) }
function money(value?: number): string { return Number(value || 0).toFixed(2) }

function priceTableLabel(item: { table_name?: string; version_no?: string; list_type?: string }): string {
  return [item.table_name || item.list_type || '指定价格表', item.version_no ? `版本 ${item.version_no}` : ''].filter(Boolean).join(' · ')
}

function openProductSelector(lineKey: string) {
  if (loading.value || submitting.value) return
  editingLineKey.value = lineKey
  productSelectorOpen.value = true
}

function closeProductSelector() { productSelectorOpen.value = false; editingLineKey.value = '' }

function chooseProduct(family: EmployeeOrderProductFamily) {
  const line = lines.value.find((item) => item.key === editingLineKey.value)
  if (!line) return
  const selected = selectDirectShipDraftProduct(line, family)
  if (!selected) { uni.showToast({ title: '该商品暂无可选规格', icon: 'none' }); return }
  Object.assign(line, selected)
  closeProductSelector()
  invalidatePreview()
}

function openSpecSelector(lineKey: string) {
  const line = lines.value.find((item) => item.key === lineKey)
  if (!line || !familyForLine(line) || submitting.value) return
  editingLineKey.value = lineKey
  specSelectorOpen.value = true
}

function closeSpecSelector() { specSelectorOpen.value = false; editingLineKey.value = '' }

function chooseSpec(spec: EmployeeOrderProductSpec) {
  const line = lines.value.find((item) => item.key === editingLineKey.value)
  if (!line) return
  Object.assign(line, selectDirectShipDraftSpec(line, spec))
  closeSpecSelector()
  invalidatePreview()
}

function setOrderDate(event: PickerChangeEvent) { orderDate.value = String(event.detail?.value || shanghaiToday()); invalidatePreview() }

function payload() {
  const recipient = selectedRecipient.value
  return {
    idempotency_key: idempotencyKey.value,
    order_date: orderDate.value,
    recipient_name: recipient?.recipient_name?.trim() || '',
    recipient_phone: recipient?.phone?.trim() || '',
    province: recipient?.province?.trim() || '',
    city: recipient?.city?.trim() || '',
    district: recipient?.district?.trim() || '',
    detail_address: recipient?.detail_address?.trim() || '',
    recipient_company: recipient?.company?.trim() || '',
    items: buildDirectShipDraftItems(lines.value),
    note: note.value.trim(),
  }
}

function validate(): string { return validateRecipient() || directShipDraftValidation(lines.value) }

async function previewRequest(silent = false): Promise<DirectShipPreview | null> {
  const validation = validate()
  if (validation) { if (!silent) errorMessage.value = validation; return null }
  const version = ++previewVersion
  const command = payload()
  previewing.value = true
  try {
    const checked = isProductOrder.value ? await previewProductOrder(props.token, command) : await previewDirectShipRequest(props.token, command)
    if (version !== previewVersion) return null
    preview.value = checked
    errorMessage.value = checked.can_submit ? '' : '当前商品暂不可下单，请检查价格表与商品配置'
    return checked
  } catch (error) {
    if (version !== previewVersion) return null
    errorMessage.value = error instanceof Error ? error.message : '自动核价失败'
    return null
  } finally {
    if (version === previewVersion) previewing.value = false
  }
}

async function submitRequest() {
  if (submitting.value) return
  const validation = validate()
  if (validation) { errorMessage.value = validation; return }
  const version = ++previewVersion
  const command = payload()
  submitting.value = true
  errorMessage.value = ''
  try {
    const checked = isProductOrder.value ? await previewProductOrder(props.token, command) : await previewDirectShipRequest(props.token, command)
    if (version !== previewVersion) throw new Error('订单内容已变更，请重新提交')
    preview.value = checked
    if (!checked.can_submit) throw new Error('当前商品暂不可下单，请检查价格表与商品配置')
    if (!checked.price_quote_token) throw new Error('报价版本缺失，请重新提交')
    if (isProductOrder.value) await createProductOrder(props.token, { ...command, price_quote_token: checked.price_quote_token })
    else await createDirectShipRequest(props.token, { ...command, price_quote_token: checked.price_quote_token })
    lines.value = [createDirectShipDraftLine()]
    preview.value = null
    note.value = ''
    idempotencyKey.value = newIdempotencyKey()
    orderDraft.clearDraft(props.customerId, currentOrderMode())
    uni.showToast({ title: isProductOrder.value ? '现货订单已创建' : '代发订单已创建', icon: 'success' })
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '订单提交失败'
  } finally {
    submitting.value = false
  }
}

async function cancelRequest(item: DirectShipRequest) {
  try { await cancelDirectShipRequest(props.token, item.id); uni.showToast({ title: '代发申请已取消', icon: 'success' }); await load() }
  catch (error) { errorMessage.value = error instanceof Error ? error.message : '取消失败' }
}

function directShipItemSpecLabel(item: { bom_spec_id?: number; spec_name?: string; bom_spec_name?: string; spec_label?: string; spec_g?: number }): string {
  if (Number(item.bom_spec_id || 0) > 0) return String(item.spec_name || item.bom_spec_name || item.spec_label || '').trim() || '当前 BOM 规格'
  return String(item.spec_label || '').trim() || `${Number(item.spec_g || 0)}g`
}
function directShipItemQtyLabel(item: { qty?: number; bom_spec_id?: number; inventory_unit?: string }): string { return `${Number(item.qty || 0)} ${Number(item.bom_spec_id || 0) > 0 ? String(item.inventory_unit || '').trim() || '件' : '件'}` }
async function applyShipmentFilters() { shipmentQuery.value = shipmentQueryInput.value.trim(); const range = normalizeDirectShipDateRange(shippedFrom.value, shippedTo.value); shippedFrom.value = range.shipped_from || ''; shippedTo.value = range.shipped_to || ''; activeDatePreset.value = ''; currentPage.value = 1; await load() }
async function applyDatePreset(preset: DirectShipDatePreset) { shipmentQuery.value = shipmentQueryInput.value.trim(); const range = directShipDatePresetRange(preset); shippedFrom.value = range.shipped_from; shippedTo.value = range.shipped_to; activeDatePreset.value = preset; currentPage.value = 1; await load() }
function setShippedFrom(event: PickerChangeEvent) { shippedFrom.value = String(event.detail?.value || ''); activeDatePreset.value = '' }
function setShippedTo(event: PickerChangeEvent) { shippedTo.value = String(event.detail?.value || ''); activeDatePreset.value = '' }
async function clearShipmentFilters() { shipmentQueryInput.value = ''; shipmentQuery.value = ''; shippedFrom.value = ''; shippedTo.value = ''; activeDatePreset.value = ''; currentPage.value = 1; await load() }
async function goToPage(value: number | string) { const target = Math.min(totalPages.value, Math.max(1, Math.trunc(Number(value) || 1))); jumpPage.value = String(target); if (target === currentPage.value) return; currentPage.value = target; await load() }
async function changePageLimit(event: PickerChangeEvent) { const index = Math.max(0, Number(event.detail?.value || 0)); pageLimit.value = pageLimitOptions[index] || pageLimitOptions[0]; currentPage.value = 1; await load() }

function recipientEventHandler(event: RecipientEvent) { consumeReturnedRecipient(event) }
onMounted(() => { restoreDraft(); uni.$on('customer-order-recipient-selected', recipientEventHandler); void load() })
onShow(() => { consumeReturnedRecipient() })
onBeforeUnmount(() => { if (previewTimer) clearTimeout(previewTimer); uni.$off('customer-order-recipient-selected', recipientEventHandler) })
</script>

<template>
  <view class="workspace">
    <view v-if="showCreate" class="order-panel">
      <text class="title">{{ createTitle }}</text>
      <view class="field-block">
        <text class="field-label">订单日期</text>
        <picker mode="date" :value="orderDate" @change="setOrderDate"><view class="selector-field"><text>{{ orderDate }}</text><text class="chevron">›</text></view></picker>
      </view>
      <view class="field-block">
        <view class="field-head"><text class="field-label">收件客户</text><text class="manage-link" @tap="openRecipientEditor(Number(selectedRecipient?.id || 0))">维护</text></view>
        <view class="selector-field recipient-field" @tap="openRecipientSelector"><text :class="{ muted: !selectedRecipientSummary }">{{ selectedRecipientSummary || '搜索并选择收件客户 *' }}</text><text class="chevron">›</text></view>
      </view>
      <view class="price-table-card">
        <view class="price-table-copy"><text class="field-label">当前价格表</text><text v-for="table in catalog.price_tables || []" :key="table.id" class="muted">{{ priceTableLabel(table) }}</text><text v-if="!catalog.price_tables?.length" class="muted">ERP 尚未绑定可用价格表</text></view>
        <button class="text-button" @tap="openPriceTablePreview">查看价格表</button>
      </view>

      <view class="section-head"><text class="subtitle">商品明细</text><text class="muted">价格由 ERP 自动计算</text></view>
      <view v-for="(line, index) in lines" :key="line.key" class="line-card">
        <view class="line-head"><text class="line-name">商品 {{ index + 1 }}</text><button class="remove" @tap="removeLine(index)">删除本行</button></view>
        <text class="field-label">商品</text>
        <view class="selector-field" @tap="openProductSelector(line.key)"><text :class="{ muted: !line.product_name }">{{ line.product_name || '搜索并选择商品' }}</text><text class="chevron">›</text></view>
        <text class="field-label">规格</text>
        <view class="selector-field" :class="{ disabled: !familyForLine(line) }" @tap="openSpecSelector(line.key)"><text :class="{ muted: !line.spec_label }">{{ line.spec_label || (familyForLine(line) ? '选择该商品的规格' : '请先选择商品') }}</text><text class="chevron">›</text></view>
        <text v-if="allSpecLabels(line)" class="spec-options">全部可选规格：{{ allSpecLabels(line) }}</text>
        <text class="field-label">数量（{{ line.sales_unit || line.inventory_unit || '件' }}）</text>
        <view class="quantity-field"><input v-model.number="line.qty" type="number" :disabled="submitting" placeholder="填写数量" @input="invalidatePreview()" /><text>{{ line.sales_unit || line.inventory_unit || '件' }}</text></view>
        <text v-if="selectedSpecForLine(line)?.price_tiers?.length" class="tier-hint">数量报价：{{ selectedSpecForLine(line)?.price_tiers?.map(tier => `${tierQuantityLabel(tier)} ¥${money(tier.unit_price)}`).join('；') }}</text>
        <view class="line-price"><text>销售单价（只读）</text><text>{{ lineUnitPrice(line) > 0 ? `¥${money(lineUnitPrice(line))}` : '自动核价' }}</text><text>小计 {{ lineAmount(line) > 0 ? `¥${money(lineAmount(line))}` : '--' }}</text></view>
      </view>
      <button class="add-line" :disabled="submitting" @tap="addLine">新增商品</button>
      <view class="total-card"><text>商品估算合计</text><text class="amount">{{ previewing ? '核价中…' : `¥${money(totalAmount)}` }}</text></view>
      <textarea v-model="note" class="textarea" :disabled="submitting" placeholder="订单备注（可选）" @input="saveDraft" />
      <text v-if="selectedSummary" class="selected-summary">已选：{{ selectedSummary }}</text>
      <text v-if="preview && !preview.stock_ready" class="shortage">当前库存不足，订单仍可提交；ERP 将按现有订单生产流程补货。</text>
      <button class="primary submit" :disabled="submitting" @tap="submitRequest">{{ submitting ? '提交中...' : submitTitle }}</button>
    </view>

    <view v-if="!showCreate" class="list-panel">
      <text class="title">发货中心</text>
      <view class="filters">
        <input v-model="shipmentQueryInput" class="input" confirm-type="search" placeholder="搜索收件客户/公司、收件人、电话、目的地" @confirm="applyShipmentFilters" />
        <text class="filter-label">按实际发货时间</text>
        <view class="date-presets"><button class="chip" :class="{ active: activeDatePreset === 'today' }" @tap="applyDatePreset('today')">当天</button><button class="chip" :class="{ active: activeDatePreset === 'last3' }" @tap="applyDatePreset('last3')">三天内</button><button class="chip" :class="{ active: activeDatePreset === 'last7' }" @tap="applyDatePreset('last7')">一周内</button><button class="chip" :class="{ active: activeDatePreset === 'month' }" @tap="applyDatePreset('month')">当月</button></view>
        <view class="date-range"><picker mode="date" :value="shippedFrom" @change="setShippedFrom"><view class="picker-field">{{ shippedFrom || '发货开始日期' }}</view></picker><picker mode="date" :value="shippedTo" @change="setShippedTo"><view class="picker-field">{{ shippedTo || '发货结束日期' }}</view></picker></view>
        <view class="filter-actions"><button class="secondary compact" @tap="clearShipmentFilters">清除</button><button class="primary compact" @tap="applyShipmentFilters">查询</button></view>
      </view>
      <text v-if="loading" class="muted">加载中...</text>
      <view v-for="item in requests" :key="item.id" class="request">
        <view class="request-head"><text class="line-name">{{ directShipRequestTitle(item) }}</text><text class="status">{{ directShipStatusLabel(item.status) }}</text></view>
        <text v-if="item.order_no" class="muted">系统订单：{{ item.order_no }} · 金额 ¥{{ money(item.total_amount) }}</text>
        <text v-for="table in item.price_tables || []" :key="`${item.id}:table:${table.id}`" class="muted">价格快照：{{ priceTableLabel(table) }}</text>
        <text v-if="item.recipient_company" class="muted">收件客户/公司：{{ item.recipient_company }}</text>
        <text class="muted">{{ item.recipient_phone }} · {{ item.province }}{{ item.city }}{{ item.district }}{{ item.detail_address }}</text>
        <text v-for="line in item.items || []" :key="`${line.product_id}:${line.bom_spec_id || 0}:${line.bom_variant_id || 0}:${line.spec_g}`" class="muted">{{ line.product_name || `商品 ${line.product_id}` }} · {{ directShipItemSpecLabel(line) }} · {{ directShipItemQtyLabel(line) }}</text>
        <view v-for="pkg in item.packages || []" :key="pkg.id" class="package"><text>{{ pkg.order_no }} · {{ pkg.warehouse }} · {{ directShipStatusLabel(pkg.status) }}</text><text class="muted">生产：{{ pkg.process_status || '待进入生产流程' }} · 发货：{{ pkg.ship_status || '待发货' }}</text><text v-for="line in pkg.items || []" :key="`${pkg.id}:item:${line.product_id}:${line.bom_spec_id || 0}:${line.spec_g}`" class="muted">{{ line.product_name }} · {{ directShipItemSpecLabel(line) }} · {{ directShipItemQtyLabel(line) }}</text><text v-if="pkg.shipped_at" class="muted">发货时间：{{ pkg.shipped_at }}</text><text class="muted">{{ pkg.tracking_no ? `${pkg.carrier_name || '承运商待录入'} ${pkg.tracking_no}` : '运单号尚未录入' }}</text><view v-for="(event, index) in pkg.events || []" :key="`${pkg.id}:event:${index}`" class="event"><text class="muted">{{ event.time || '' }} {{ event.description || directShipStatusLabel(event.status) }}</text><text v-if="event.location" class="muted">{{ event.location }}</text></view><text v-if="pkg.tracking_no && !(pkg.events || []).length" class="muted">暂未获取物流轨迹</text></view>
        <button v-if="['pending','reserved','待处理','待发货'].includes(item.status)" class="secondary compact" @tap="cancelRequest(item)">取消发货</button>
      </view>
      <text v-if="!loading && !requests.length" class="muted">暂无发货记录</text>
      <view v-if="totalPages > 1" class="pagination"><text class="muted">共 {{ totalRows }} 条 · 共 {{ totalPages }} 页</text><view class="page-actions"><button class="secondary compact" :disabled="currentPage <= 1 || loading" @tap="goToPage(currentPage - 1)">上一页</button><text class="page-current">第 {{ currentPage }} / {{ totalPages }} 页</text><button class="secondary compact" :disabled="currentPage >= totalPages || loading" @tap="goToPage(currentPage + 1)">下一页</button></view><view class="page-jump"><picker mode="selector" :range="pageLimitLabels" :value="Math.max(0, pageLimitOptions.indexOf(pageLimit))" @change="changePageLimit"><view class="picker-field">每页 {{ pageLimit }} 条</view></picker><input v-model="jumpPage" class="jump-input" type="number" @confirm="goToPage(jumpPage)" /><button class="secondary compact" @tap="goToPage(jumpPage)">跳页</button></view></view>
    </view>

    <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
    <ProductFamilyPickerSheet :visible="productSelectorOpen" :families="catalog.product_families" :customer-id="customerId" :loading="loading" customer-facing-names @close="closeProductSelector" @select="chooseProduct" />
    <ProductSpecPickerSheet :visible="specSelectorOpen" :family="activeSpecFamily" :selected-bom-spec-id="activeSpecLine?.bom_spec_id" :selected-product-id="activeSpecLine?.product_id" @close="closeSpecSelector" @select="chooseSpec" />
    <view v-if="recipientSelectorOpen" class="overlay recipient-overlay" @tap.self="closeRecipientSelector">
      <view class="recipient-sheet" @tap.stop>
        <view class="sheet-head"><view><text class="sheet-title">选择收件客户</text><text class="sheet-subtitle">选择后返回录单，已录商品会保留</text></view><text class="sheet-close" @tap="closeRecipientSelector">关闭</text></view>
        <input v-model="recipientQuery" class="search-input" focus confirm-type="search" placeholder="搜索姓名、电话、公司或地址" />
        <scroll-view scroll-y class="recipient-list">
          <view v-for="row in filteredRecipients" :key="row.id" class="recipient-row" @tap="applyRecipientAddress(row)"><view class="recipient-copy"><view class="recipient-head"><text class="recipient-name">{{ row.recipient_name }} · {{ row.phone }}</text><text v-if="row.is_default" class="badge">默认</text></view><text v-if="row.company" class="muted">{{ row.company }}</text><text class="recipient-address">{{ row.province }}{{ row.city }}{{ row.district }}{{ row.detail_address }}</text></view><text class="edit-link" @tap.stop="openRecipientEditor(row.id)">编辑</text></view>
          <text v-if="!filteredRecipients.length" class="empty">暂无匹配的收件客户</text>
        </scroll-view>
        <button class="primary" @tap="openRecipientEditor(0)">新增收件客户</button>
      </view>
    </view>
  </view>
</template>

<style scoped>
.workspace{display:flex;flex-direction:column;gap:18rpx}.order-panel,.list-panel{display:flex;flex-direction:column;gap:22rpx;padding:28rpx;border:1rpx solid #dce5df;border-radius:20rpx;background:#fff}.title{font-size:38rpx;font-weight:900;color:#173126}.subtitle{font-size:31rpx;font-weight:850;color:#173126}.field-block,.price-table-copy,.line-card,.filters,.request,.package,.event{display:flex;flex-direction:column;gap:12rpx}.field-head,.section-head,.line-head,.line-price,.total-card,.request-head,.sheet-head,.recipient-head,.recipient-row{display:flex;align-items:center;justify-content:space-between;gap:14rpx}.field-label,.line-name{font-weight:800;color:#29483a}.manage-link,.edit-link,.text-button{color:#28624a;font-weight:750}.selector-field,.quantity-field,.input,.textarea,.picker-field,.search-input{width:100%;min-height:86rpx;padding:0 22rpx;border:1rpx solid #d6e0da;border-radius:13rpx;box-sizing:border-box;background:#fafcfb}.selector-field,.quantity-field{display:flex;align-items:center;justify-content:space-between;gap:16rpx}.selector-field text:first-child{flex:1}.selector-field.disabled{background:#f1f4f2;color:#9aa49e}.chevron{color:#718078;font-size:38rpx}.price-table-card{display:flex;align-items:center;justify-content:space-between;gap:18rpx;padding:22rpx;border:1rpx solid #dbe5df;border-radius:16rpx;background:#f7faf8}.text-button{min-width:150rpx;min-height:62rpx;margin:0;padding:0 12rpx;border:1rpx solid #b8cec1;border-radius:10rpx;background:#fff;font-size:23rpx}.text-button::after,.remove::after{border:0}.line-card{padding:24rpx;border:1rpx solid #dbe5df;border-radius:18rpx;background:#fbfdfc}.remove{min-height:58rpx;margin:0;padding:0 18rpx;border:1rpx solid #e2c5c0;border-radius:10rpx;background:#fff;color:#9e3e35;font-size:23rpx}.spec-options,.tier-hint,.selected-summary,.muted{color:#74827a;font-size:23rpx;line-height:1.55}.quantity-field input{flex:1}.line-price{padding-top:10rpx;color:#53675d;font-size:23rpx}.line-price text:nth-child(2),.amount{color:#17603f;font-weight:900}.add-line{min-height:76rpx;margin:0;border:1rpx dashed #7ea28e;border-radius:12rpx;background:#fff;color:#28624a;font-size:26rpx}.total-card{padding:24rpx;border-radius:16rpx;background:#eaf4ee;color:#244839;font-size:29rpx;font-weight:850}.amount{font-size:34rpx}.textarea{min-height:120rpx;padding-top:18rpx}.primary,.secondary{min-height:76rpx;margin:0;border-radius:12rpx;font-size:26rpx}.primary{background:#28624a;color:#fff}.secondary{border:1rpx solid #cbd8d1;background:#fff;color:#315844}.submit{margin-top:2rpx}.error,.shortage{padding:18rpx;border-radius:10rpx;color:#b42318;background:#fff4f1;font-size:24rpx}.ready{color:#28624a}.filters{padding:18rpx;background:#fafcfb;border-radius:12rpx}.date-presets,.date-range,.filter-actions,.page-actions,.page-jump{display:flex;gap:12rpx}.date-range picker,.filter-actions button,.page-actions button{flex:1}.chip{min-height:56rpx;margin:0;padding:0 14rpx;border:1rpx solid #d4ded8;border-radius:30rpx;background:#fff;font-size:22rpx}.chip.active{background:#e6f2eb;color:#28624a}.compact{min-height:60rpx}.request,.package{padding:20rpx;border:1rpx solid #e2e8e4;border-radius:12rpx}.package{background:#f8faf9}.status{color:#28624a;font-weight:800}.pagination{display:flex;flex-direction:column;gap:14rpx}.page-current{display:flex;align-items:center}.page-jump{align-items:center}.page-jump picker{flex:1}.jump-input{width:120rpx;min-height:60rpx;border:1rpx solid #d5ddd8;border-radius:8rpx;text-align:center}.overlay{position:fixed;inset:0;z-index:1110;display:flex;align-items:flex-end;background:rgba(16,28,22,.48)}.recipient-sheet{width:100%;max-height:82vh;padding:28rpx 28rpx calc(24rpx + env(safe-area-inset-bottom));border-radius:24rpx 24rpx 0 0;box-sizing:border-box;background:#fff}.sheet-title,.sheet-subtitle{display:block}.sheet-title{font-size:32rpx;font-weight:850;color:#173126}.sheet-subtitle{margin-top:6rpx;color:#7a8880;font-size:22rpx}.sheet-close{padding:12rpx;color:#607268}.search-input{margin:18rpx 0}.recipient-list{height:46vh}.recipient-row{padding:20rpx 8rpx;border-bottom:1rpx solid #edf1ee}.recipient-copy{display:flex;flex:1;flex-direction:column;gap:8rpx}.recipient-name{font-size:28rpx;font-weight:800;color:#213b2f}.recipient-address{color:#53655b;font-size:24rpx}.badge{padding:3rpx 10rpx;border-radius:999rpx;background:#e6f3eb;color:#28624a;font-size:20rpx}.empty{display:block;padding:70rpx 20rpx;color:#7d8982;text-align:center}
</style>
