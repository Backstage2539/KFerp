<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onShow } from '@dcloudio/uni-app'
import {
  type BeanListSummary,
  createDirectShipBatch,
  createProcessingRequest,
  fetchServicePage,
  type ServicePageResponse,
} from '../../api/customerPortal'
import { buildAPIURL } from '../../api/client'
import { useSessionStore } from '../../stores/session'
import {
  beanListCacheStorageKey,
  beanListVersionChanged,
  nextBeanListCacheRecord,
  type BeanListPDFCacheRecord,
} from '../../utils/beanListPdfCache'
import { buildOrderServiceFilters, datePresetRange, normalizeDateRange, type OrderDatePreset } from '../../utils/orderFilters'
import { normalizeServiceKey, serviceTitle, visibleServiceSections, type ServiceKey } from '../../utils/servicePage'

const session = useSessionStore()
const serviceKey = ref<ServiceKey>('beanList')
const page = ref<ServicePageResponse | null>(null)
const loading = ref(false)
const submitting = ref(false)
const openingBeanListPDF = ref(false)
const errorMessage = ref('')
const autoOpenedBeanListCacheKey = ref('')
const orderSearch = ref({ keyword: '', date_from: '', date_to: '' })

const directShipForm = ref({ source_name: '', total_rows: 0, note: '' })
const processingForm = ref({
  input_material_id: 0,
  input_qty_g: 0,
  target_product_id: 0,
  target_spec_g: 454,
  target_qty: 1,
  note: '',
})

const title = computed(() => page.value?.title || serviceTitle(serviceKey.value))
const summary = computed(() => page.value?.summary || [])
const sections = computed(() => (page.value ? visibleServiceSections(page.value) : []))
const orderPanelTitle = computed(() => (serviceKey.value === 'orders' ? '我的订单' : '订单 / 物流'))

async function loadPage() {
  if (!session.token) {
    uni.redirectTo({ url: '/pages/login/login' })
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    const filters = serviceKey.value === 'orders' ? buildOrderServiceFilters(orderSearch.value) : {}
    page.value = await fetchServicePage(session.token, serviceKey.value, filters)
    maybeAutoOpenLatestBeanListPDF()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '服务数据加载失败'
  } finally {
    loading.value = false
  }
}

function maybeAutoOpenLatestBeanListPDF() {
  if (serviceKey.value !== 'beanList') return
  const item = page.value?.bean_lists?.find((row) => row.pdf_url)
  if (!item) return
  const cacheKey = item.cache_key || `${item.id}-${item.version_no}`
  if (autoOpenedBeanListCacheKey.value === cacheKey) return
  autoOpenedBeanListCacheKey.value = cacheKey
  void openBeanListPDF(item, true)
}

async function openBeanListPDF(item: BeanListSummary, auto = false) {
  if (!session.token || !item.pdf_url) {
    if (!auto) uni.showToast({ title: '暂无 PDF', icon: 'none' })
    return
  }
  openingBeanListPDF.value = true
  errorMessage.value = ''
  try {
    const cached = cachedBeanListPDF(item)
    if (cached && !beanListVersionChanged(cached, item) && (await savedFileExists(cached.saved_file_path))) {
      await openPDFFile(cached.saved_file_path)
      return
    }
    if (cached && beanListVersionChanged(cached, item) && (await savedFileExists(cached.saved_file_path))) {
      const shouldUpdate = await confirmBeanListPDFUpdate(item)
      if (!shouldUpdate) {
        await openPDFFile(cached.saved_file_path)
        return
      }
    }
    await downloadSaveAndOpenBeanListPDF(item)
  } catch (error) {
    const message = error instanceof Error ? error.message : '豆单 PDF 打开失败'
    if (!auto) errorMessage.value = message
  } finally {
    openingBeanListPDF.value = false
  }
}

function cachedBeanListPDF(item: BeanListSummary): BeanListPDFCacheRecord | null {
  const value = uni.getStorageSync(beanListCacheStorageKey(page.value?.current_customer_id || session.currentCustomerID, item))
  if (!value || typeof value !== 'object') return null
  return value as BeanListPDFCacheRecord
}

function confirmBeanListPDFUpdate(item: BeanListSummary): Promise<boolean> {
  return new Promise((resolve) => {
    uni.showModal({
      title: '豆单有更新',
      content: `检测到 ${item.version_no || '新版本'}，是否更新本地 PDF？`,
      confirmText: '更新',
      cancelText: '先看旧版',
      success: (res) => resolve(Boolean(res.confirm)),
      fail: () => resolve(true),
    })
  })
}

function savedFileExists(filePath: string): Promise<boolean> {
  return new Promise((resolve) => {
    if (!filePath) {
      resolve(false)
      return
    }
    uni.getSavedFileInfo({
      filePath,
      success: () => resolve(true),
      fail: () => resolve(false),
    })
  })
}

function openPDFFile(filePath: string): Promise<void> {
  return new Promise((resolve, reject) => {
    uni.openDocument({
      filePath,
      fileType: 'pdf',
      showMenu: true,
      success: () => resolve(),
      fail: (err) => reject(new Error(err.errMsg || 'PDF 打开失败')),
    })
  })
}

function downloadSaveAndOpenBeanListPDF(item: BeanListSummary): Promise<void> {
  return new Promise((resolve, reject) => {
    uni.downloadFile({
      url: buildAPIURL(item.pdf_url),
      header: { Authorization: `Bearer ${session.token}` },
      success: (downloadRes) => {
        if (downloadRes.statusCode && downloadRes.statusCode >= 400) {
          reject(new Error(`PDF 下载失败：${downloadRes.statusCode}`))
          return
        }
        uni.saveFile({
          tempFilePath: downloadRes.tempFilePath,
          success: async (saveRes) => {
            try {
              uni.setStorageSync(
                beanListCacheStorageKey(page.value?.current_customer_id || session.currentCustomerID, item),
                nextBeanListCacheRecord(item, saveRes.savedFilePath),
              )
              await openPDFFile(saveRes.savedFilePath)
              resolve()
            } catch (error) {
              reject(error)
            }
          },
          fail: (err) => reject(new Error(err.errMsg || 'PDF 保存失败')),
        })
      },
      fail: (err) => reject(new Error(err.errMsg || 'PDF 下载失败')),
    })
  })
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
  orderSearch.value = { keyword: '', date_from: '', date_to: '' }
  await loadPage()
}

function setOrderDateFrom(event: { detail?: { value?: string } }) {
  orderSearch.value.date_from = event.detail?.value || ''
}

function setOrderDateTo(event: { detail?: { value?: string } }) {
  orderSearch.value.date_to = event.detail?.value || ''
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

      <view v-if="sections.length" class="section-list">
        <view v-for="section in sections" :key="section.title" class="section-row">
          <text class="section-title">{{ section.title }}</text>
          <text class="section-count">{{ section.count }}</text>
        </view>
      </view>

      <view v-if="page?.bean_lists?.length" class="panel">
        <text class="panel-title">豆单</text>
        <view v-for="item in page.bean_lists" :key="item.id" class="list-row">
          <view class="row-head">
            <text class="row-main">{{ item.list_type }} {{ item.version_no }}</text>
            <button class="small-action" :disabled="openingBeanListPDF || !item.pdf_url" @tap="openBeanListPDF(item)">
              打开 PDF
            </button>
          </view>
          <text class="row-sub">{{ item.published_at }} / 本地缓存随版本更新</text>
          <view v-if="!item.pdf_url && item.groups?.length" class="bean-list-items">
            <view v-for="group in item.groups" :key="`${item.id}-${group.category}`" class="bean-list-group">
              <text v-if="group.category" class="bean-list-category">{{ group.category }}</text>
              <view v-for="bean in group.items" :key="`${item.id}-${group.category}-${bean.code || bean.name}`" class="bean-list-product">
                <view class="bean-list-product-head">
                  <text class="bean-list-name">{{ bean.code ? `${bean.code} ${bean.name}` : bean.name }}</text>
                  <text v-if="bean.badge_label" class="bean-list-badge">{{ bean.badge_label }}</text>
                </view>
                <text v-if="bean.recommended_use" class="row-sub">{{ bean.recommended_use }}</text>
                <text v-if="bean.flavor" class="row-sub">{{ bean.flavor }}</text>
                <text v-if="bean.description" class="bean-list-description">{{ bean.description }}</text>
                <view v-if="bean.prices?.length" class="bean-list-prices">
                  <text v-for="price in bean.prices" :key="`${price.label}-${price.value}`" :class="['bean-list-price', { red: price.red }]">
                    {{ price.label }} {{ price.value }}
                  </text>
                </view>
              </view>
            </view>
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

      <view v-if="page && !sections.length" class="state">
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

.small-action {
  min-width: 152rpx;
  min-height: 60rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0;
  padding: 0 18rpx;
  background: #171717;
  color: #ffffff;
  border-radius: 8rpx;
  font-size: 24rpx;
  line-height: 1;
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
.filter-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12rpx;
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

.bean-list-items {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
  padding-top: 10rpx;
}

.bean-list-group {
  display: flex;
  flex-direction: column;
  gap: 12rpx;
}

.bean-list-category {
  color: #6f5d2e;
  font-size: 25rpx;
  font-weight: 700;
}

.bean-list-product {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  padding: 16rpx;
  background: #f8f8f8;
  border: 1rpx solid #ececec;
  border-radius: 8rpx;
}

.bean-list-product-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12rpx;
}

.bean-list-name {
  color: #171717;
  font-size: 27rpx;
  font-weight: 700;
  line-height: 1.35;
}

.bean-list-badge {
  flex: 0 0 auto;
  color: #6f5d2e;
  font-size: 22rpx;
  font-weight: 700;
}

.bean-list-description {
  color: #333333;
  font-size: 24rpx;
  line-height: 1.5;
}

.bean-list-prices {
  display: flex;
  flex-wrap: wrap;
  gap: 10rpx;
}

.bean-list-price {
  color: #171717;
  font-size: 24rpx;
  font-weight: 700;
}

.bean-list-price.red {
  color: #b42318;
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
