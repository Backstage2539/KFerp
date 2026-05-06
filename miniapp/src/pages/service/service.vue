<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onShow } from '@dcloudio/uni-app'
import {
  type BeanListSummary,
  createDirectShipBatch,
  createFulfillmentOrder,
  createProcessingRequest,
  fetchServicePage,
  type ServicePageResponse,
} from '../../api/customerPortal'
import { useSessionStore } from '../../stores/session'
import { beanListCardRows, beanListDisplayStyle, splitBeanListHighlight } from '../../utils/beanListDisplay'
import {
  beanListPageCacheChanged,
  beanListPageCacheStorageKey,
  nextBeanListPageCacheRecord,
  type BeanListPageCacheRecord,
} from '../../utils/beanListPageCache'
import { buildOrderServiceFilters, datePresetRange, normalizeDateRange, type OrderDatePreset } from '../../utils/orderFilters'
import { normalizeServiceKey, serviceTitle, visibleServiceSections, type ServiceKey } from '../../utils/servicePage'

type OrderSearchForm = {
  keyword: string
  date_from: string
  date_to: string
  process_status: string
  pay_status: string
  ship_status: string
}

type OrderStatusField = 'process_status' | 'pay_status' | 'ship_status'

const session = useSessionStore()
const serviceKey = ref<ServiceKey>('beanList')
const page = ref<ServicePageResponse | null>(null)
const loading = ref(false)
const submitting = ref(false)
const errorMessage = ref('')
const cachedBeanList = ref<BeanListSummary | null>(null)
const beanListCacheStatus = ref('')
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
  unit_price: 0,
  shipping_amount: 0,
  note: '',
})

const title = computed(() => page.value?.title || serviceTitle(serviceKey.value))
const summary = computed(() => page.value?.summary || [])
const sections = computed(() => (page.value ? visibleServiceSections(page.value) : []))
const orderPanelTitle = computed(() => (serviceKey.value === 'orders' ? '我的订单' : '订单 / 物流'))
const beanListsForDisplay = computed(() => {
  if (page.value?.bean_lists?.length) return page.value.bean_lists
  return cachedBeanList.value ? [cachedBeanList.value] : []
})
const hasDisplayData = computed(() => sections.value.length > 0 || (serviceKey.value === 'beanList' && beanListsForDisplay.value.length > 0))
const processStatusPickerOptions = computed(() => orderStatusOptions('process_status', defaultProcessStatusOptions, '全部生产状态'))
const payStatusPickerOptions = computed(() => orderStatusOptions('pay_status', defaultPayStatusOptions, '全部收款状态'))
const shipStatusPickerOptions = computed(() => orderStatusOptions('ship_status', defaultShipStatusOptions, '全部发货状态'))

async function loadPage() {
  if (!session.token) {
    uni.redirectTo({ url: '/pages/login/login' })
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    if (serviceKey.value === 'beanList') {
      primeCachedBeanListPage()
    }
    const filters = serviceKey.value === 'orders' ? buildOrderServiceFilters(orderSearch.value) : {}
    page.value = await fetchServicePage(session.token, serviceKey.value, filters)
    if (serviceKey.value === 'beanList') {
      cacheBeanListPages(page.value.bean_lists || [])
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
    if (!cachedBeanList.value) beanListCacheStatus.value = '暂无已发布豆单'
    return
  }
  let updated = false
  for (const item of items) {
    updated = cacheBeanListPage(item) || updated
  }
  cachedBeanList.value = items[0]
  beanListCacheStatus.value = updated ? '检测到新版豆单，已更新本地缓存' : '已使用本地缓存，发布新版后自动更新'
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

async function submitDirectShipBatch() {
  if (!directShipForm.value.source_name.trim()) {
    errorMessage.value = '请填写批次名称'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    await createDirectShipBatch(session.token, {
      source_name: directShipForm.value.source_name,
      total_rows: Number(directShipForm.value.total_rows) || 0,
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
  const payload = {
    service_code: fulfillmentServiceCode(),
    recipient_name: fulfillmentForm.value.recipient_name.trim(),
    recipient_phone: fulfillmentForm.value.recipient_phone.trim(),
    recipient_address: fulfillmentForm.value.recipient_address.trim(),
    recipient_company: fulfillmentForm.value.recipient_company.trim(),
    product_id: Number(fulfillmentForm.value.product_id) || 0,
    product_name: fulfillmentForm.value.product_name.trim(),
    spec_g: Number(fulfillmentForm.value.spec_g) || 0,
    qty: Number(fulfillmentForm.value.qty) || 0,
    unit_price: Number(fulfillmentForm.value.unit_price) || 0,
    shipping_amount: Number(fulfillmentForm.value.shipping_amount) || 0,
    note: fulfillmentForm.value.note,
  }
  if (!payload.recipient_name || !payload.recipient_phone || !payload.recipient_address || !payload.product_id || !payload.spec_g || !payload.qty) {
    errorMessage.value = '请填写完整发货订单'
    return
  }
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
      unit_price: 0,
      shipping_amount: 0,
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

onLoad((query) => {
  serviceKey.value = normalizeServiceKey(String(query?.key || 'beanList'))
})

onShow(() => {
  void loadPage()
})
</script>

<template>
  <view class="page">
    <view class="header">
      <text class="eyebrow">服务入口</text>
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
        <input v-model.number="processingForm.input_material_id" class="input" type="number" placeholder="生豆物料ID" />
        <input v-model.number="processingForm.input_qty_g" class="input" type="number" placeholder="投入生豆克重" />
        <input v-model.number="processingForm.target_product_id" class="input" type="number" placeholder="目标产品ID" />
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
        <input v-model.number="fulfillmentForm.product_id" class="input" type="number" placeholder="产品ID" />
        <input v-model="fulfillmentForm.product_name" class="input" placeholder="产品名称，可选" />
        <input v-model.number="fulfillmentForm.spec_g" class="input" type="number" placeholder="规格克重" />
        <input v-model.number="fulfillmentForm.qty" class="input" type="number" placeholder="件数" />
        <input v-model.number="fulfillmentForm.unit_price" class="input" type="digit" placeholder="单价，可不填" />
        <input v-model.number="fulfillmentForm.shipping_amount" class="input" type="digit" placeholder="运费，可不填" />
        <textarea v-model="fulfillmentForm.note" class="textarea" placeholder="订单备注" />
        <button class="primary" :disabled="submitting" @tap="submitFulfillmentOrder">提交订单</button>
      </view>

      <view v-if="sections.length" class="section-list">
        <view v-for="section in sections" :key="section.title" class="section-row">
          <text class="section-title">{{ section.title }}</text>
          <text class="section-count">{{ section.count }}</text>
        </view>
      </view>

      <view v-if="serviceKey === 'beanList' && beanListsForDisplay.length" class="bean-list-native">
        <view v-for="item in beanListsForDisplay" :key="`${item.id}-${item.cache_key || item.version_no}`" class="bean-list-surface" :style="beanListDisplayStyle(item)">
          <view class="bean-list-cover">
            <view class="bean-list-cover-main">
              <image v-if="item.logo_image" class="bean-list-logo" :src="item.logo_image" mode="aspectFit" />
              <text v-if="item.show_version !== false" class="bean-list-version">{{ item.version_no || '当前版本' }}</text>
              <text class="bean-list-title">{{ item.title || '我的豆单' }}</text>
              <text v-if="item.subtitle" class="bean-list-subtitle">{{ item.subtitle }}</text>
              <text v-if="item.brand_intro" class="bean-list-brand-intro">{{ item.brand_intro }}</text>
            </view>
            <text class="bean-list-type">{{ item.list_type_label || item.list_type }}</text>
          </view>
          <text v-if="beanListCacheStatus" class="bean-list-cache-hint">{{ beanListCacheStatus }}</text>

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

      <view v-if="page?.products?.length" class="panel">
        <text class="panel-title">现货商品</text>
        <view v-for="item in page.products" :key="item.id" class="list-row">
          <text class="row-main">{{ item.name }}</text>
          <text class="row-sub">{{ item.roast_level }} / 默认 ¥{{ item.default_price }}</text>
        </view>
      </view>

      <view v-if="page?.orders?.length" class="panel">
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
            </view>
          </view>
          <text class="row-sub">运费：¥{{ item.shipping_amount || '0.00' }}</text>
          <text class="row-sub">物流：{{ item.ship_tracking_no || '暂无单号' }}</text>
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
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 32rpx;
  background: #f6f6f6;
  box-sizing: border-box;
}

.header {
  display: flex;
  flex-direction: column;
  gap: 14rpx;
  padding: 24rpx 0 32rpx;
}

.eyebrow {
  color: #6f5d2e;
  font-size: 24rpx;
  font-weight: 600;
}

.title {
  color: #171717;
  font-size: 42rpx;
  font-weight: 700;
}

.subtitle {
  color: #666666;
  font-size: 26rpx;
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

.filter-panel {
  gap: 16rpx;
}

.date-presets {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10rpx;
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

.order-items {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  padding: 12rpx;
  background: #f8f8f8;
  border-radius: 8rpx;
}

.bean-list-native {
  margin-bottom: 20rpx;
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

.bean-list-footer {
  display: flex;
  justify-content: space-between;
  gap: 18rpx;
  margin-top: 4rpx;
}

.item-line {
  display: flex;
  justify-content: space-between;
  gap: 16rpx;
  color: #333333;
  font-size: 24rpx;
  line-height: 1.45;
}

.section-count {
  color: #6f5d2e;
  font-size: 30rpx;
  font-weight: 700;
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
