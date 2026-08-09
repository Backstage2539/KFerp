<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onShow } from '@dcloudio/uni-app'
import {
  buildEmployeeOrderDocumentPath,
  fetchEmployeeOrderDetail,
  fetchEmployeeShareSettings,
  generateEmployeeOrderDocument,
  type EmployeeOrderDetail,
  type EmployeeOrderDetailItem,
  type EmployeeOrderDocumentAsset,
  type EmployeeOrderDocumentFormat,
  type EmployeeOrderDocumentGenerateResponse,
  type EmployeeOrderDocumentKind,
  type EmployeeOrderDocuments,
  type EmployeeOrderTrace,
} from '../../api/customerPortal'
import { isAuthenticationExpiredRequestError, MiniRequestError } from '../../api/client'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { useSessionStore } from '../../stores/session'
import {
  employeeOrderDocumentAsset,
  employeeOrderFeeLines,
  employeeOrderItemDisplayName,
  employeeOrderInvoiceStatusLabel,
  employeeOrderItemPriceSourceLabel,
  employeeOrderItemPriceTableVersion,
  employeeOrderItemSpecLabel,
  employeeOrderTraceLineLabel,
  employeeOrderTraceSourceLines,
} from '../../utils/employeeOrderDetail'
import { shareMiniappFileOutput } from '../../utils/fileOutput'

const session = useSessionStore()
const orderID = ref(0)
const order = ref<EmployeeOrderDetail>()
const documents = ref<EmployeeOrderDocuments>({})
const loading = ref(false)
const errorMessage = ref('')
const authExpired = ref(false)
const sharingKey = ref('')
const canEdit = ref(false)
const editBlockReason = ref('')

const fees = computed(() => employeeOrderFeeLines(
  (order.value || {}) as unknown as Record<string, unknown>,
))

function normalizedDocumentAsset(response: EmployeeOrderDocumentGenerateResponse): EmployeeOrderDocumentAsset {
  return response.document || response.asset || response
}

function setDocumentAsset(
  kind: EmployeeOrderDocumentKind,
  format: EmployeeOrderDocumentFormat,
  asset: EmployeeOrderDocumentAsset,
) {
  const groupKey = kind === 'sales-order' ? 'sales_order' : 'delivery_note'
  documents.value = {
    ...documents.value,
    [groupKey]: {
      ...(documents.value[groupKey] || {}),
      [format]: { ...asset, available: asset.available !== false },
    },
  }
}

function hasDocument(kind: EmployeeOrderDocumentKind, format: EmployeeOrderDocumentFormat): boolean {
  const asset = employeeOrderDocumentAsset(documents.value, kind, format)
  return Boolean(asset && asset.available !== false)
}

function documentLabel(kind: EmployeeOrderDocumentKind): string {
  return kind === 'sales-order' ? '销售单' : '发货单'
}

function documentFileName(kind: EmployeeOrderDocumentKind, format: EmployeeOrderDocumentFormat): string {
  const asset = employeeOrderDocumentAsset(documents.value, kind, format)
  const formalFilename = String(asset?.filename || '').trim()
  if (formalFilename) return formalFilename
  const no = String(order.value?.order_no || orderID.value || '订单').replace(/[\\/:*?"<>|]/g, '-')
  const version = String(asset?.version_no || '').trim()
  return `${documentLabel(kind)}-${no}${version ? `-V${version.replace(/^V/i, '')}` : ''}.${format}`
}

async function loadDetail() {
  if (!orderID.value) {
    errorMessage.value = '订单参数不正确'
    return
  }
  loading.value = true
  errorMessage.value = ''
  authExpired.value = false
  try {
    const response = await fetchEmployeeOrderDetail(session.token, orderID.value)
    order.value = response.order
    documents.value = response.documents || {}
    canEdit.value = Boolean(response.can_edit ?? response.order.can_edit)
    editBlockReason.value = String(response.edit_block_reason || response.order.edit_block_reason || '')
    if (order.value?.order_no) uni.setNavigationBarTitle({ title: order.value.order_no })
  } catch (cause) {
    authExpired.value = isAuthenticationExpiredRequestError(cause)
    if (authExpired.value) session.clearSession()
    errorMessage.value = authExpired.value
      ? '登录已失效，请重新登录'
      : (cause instanceof Error ? cause.message : '订单详情加载失败')
  } finally {
    loading.value = false
  }
}

function goToLogin() {
  session.clearSession()
  uni.reLaunch({ url: '/pages/login/login' })
}

function openEditor() {
  if (!canEdit.value || orderID.value <= 0) return
  uni.navigateTo({
    url: `/pages/employee-order-entry/employee-order-entry?edit_id=${orderID.value}`,
  })
}

function showShareSettingsFallbackNotice(): Promise<void> {
  return new Promise((resolve) => {
    uni.showModal({
      title: '分享设置读取失败',
      content: '本次将按安全方式继续，图片不会携带小程序入口。',
      showCancel: false,
      complete: () => resolve(),
    })
  })
}

async function shareDocument(kind: EmployeeOrderDocumentKind, format: EmployeeOrderDocumentFormat) {
  if (!order.value || sharingKey.value) return
  const key = `${kind}.${format}`
  sharingKey.value = key
  try {
    let imageNeedShowEntrance = false
    if (format === 'png') {
      try {
        const shareSettings = await fetchEmployeeShareSettings(session.token)
        imageNeedShowEntrance = shareSettings.settings?.image_need_show_entrance === true
      } catch (cause) {
        if (isAuthenticationExpiredRequestError(cause)) throw cause
        await showShareSettingsFallbackNotice()
      }
    }
    let generatedBeforeDownload = false
    const generateAndRefresh = async () => {
      generatedBeforeDownload = true
      const generated = await generateEmployeeOrderDocument(session.token, orderID.value, kind, format)
      setDocumentAsset(kind, format, normalizedDocumentAsset(generated))
      try {
        const refreshed = await fetchEmployeeOrderDetail(session.token, orderID.value)
        order.value = refreshed.order
        documents.value = refreshed.documents || documents.value
        canEdit.value = Boolean(refreshed.can_edit ?? refreshed.order.can_edit)
        editBlockReason.value = String(refreshed.edit_block_reason || refreshed.order.edit_block_reason || '')
      } catch {
        // The generated path remains usable even if the metadata refresh is interrupted.
      }
    }
    if (!hasDocument(kind, format)) {
      await generateAndRefresh()
    }
    const downloadAndShare = () => shareMiniappFileOutput({
        path: buildEmployeeOrderDocumentPath(orderID.value, kind, format),
        token: session.token,
        kind: format,
        fileName: documentFileName(kind, format),
        loadingTitle: '准备分享',
        needShowEntrance: imageNeedShowEntrance,
      })
    try {
      await downloadAndShare()
    } catch (cause) {
      if (!generatedBeforeDownload && cause instanceof MiniRequestError && cause.statusCode === 404) {
        await generateAndRefresh()
        await downloadAndShare()
      } else {
        throw cause
      }
    }
  } catch (cause) {
    if (isAuthenticationExpiredRequestError(cause)) {
      session.clearSession()
      authExpired.value = true
      errorMessage.value = '登录已失效，请重新登录'
      return
    }
    uni.showToast({ title: cause instanceof Error ? cause.message : '单据生成失败', icon: 'none' })
  } finally {
    sharingKey.value = ''
  }
}

function itemPriceVersion(item: EmployeeOrderDetailItem): string {
  return employeeOrderItemPriceTableVersion(item) || '-'
}

function itemSubtotal(item: EmployeeOrderDetailItem): string {
  const supplied = String(item.line_total || '').trim()
  if (supplied) return supplied
  const calculated = Number(item.qty || 0) * Number(item.unit_price || 0)
  return Number.isFinite(calculated) ? calculated.toFixed(2) : '0.00'
}

function traceLines(row: EmployeeOrderTrace, type: 'quote' | 'production'): string[] {
  return employeeOrderTraceSourceLines(row, type)
}

function logisticsLabel(): string {
  const values = [order.value?.logistics_company, order.value?.logistics_product, order.value?.ship_method]
    .map((value) => String(value || '').trim())
    .filter(Boolean)
  return Array.from(new Set(values)).join(' / ') || '-'
}

onLoad((options) => {
  orderID.value = Number(options?.id || 0)
})
onShow(() => void loadDetail())
</script>

<template>
  <view class="page pull-up-brand-page">
    <EnvironmentBadge />
    <view v-if="loading" class="state-card"><text>订单详情加载中...</text></view>
    <view v-else-if="errorMessage" class="state-card error-card">
      <text>{{ errorMessage }}</text>
      <button v-if="authExpired" class="state-action" @tap="goToLogin">重新登录</button>
      <button v-else class="state-action" @tap="loadDetail">重试</button>
    </view>

    <template v-else-if="order">
      <view class="section document-section">
        <text class="section-title">导出并微信分享</text>
        <text class="section-hint">没有历史版本时会先按网页同一模板生成，再打开微信分享；低版本微信会回退到预览菜单。</text>
        <view class="document-grid">
          <button :loading="sharingKey === 'sales-order.pdf'" :disabled="Boolean(sharingKey)" @tap="shareDocument('sales-order', 'pdf')">销售单 PDF</button>
          <button :loading="sharingKey === 'sales-order.png'" :disabled="Boolean(sharingKey)" @tap="shareDocument('sales-order', 'png')">销售单图片</button>
          <button :loading="sharingKey === 'delivery-note.pdf'" :disabled="Boolean(sharingKey)" @tap="shareDocument('delivery-note', 'pdf')">发货单 PDF</button>
          <button :loading="sharingKey === 'delivery-note.png'" :disabled="Boolean(sharingKey)" @tap="shareDocument('delivery-note', 'png')">发货单图片</button>
        </view>
        <text class="document-status">销售单：PDF {{ hasDocument('sales-order', 'pdf') ? '已有版本' : '点击自动生成' }}，图片 {{ hasDocument('sales-order', 'png') ? '已有版本' : '点击自动生成' }}</text>
        <text class="document-status">发货单：PDF {{ hasDocument('delivery-note', 'pdf') ? '已有版本' : '点击自动生成' }}，图片 {{ hasDocument('delivery-note', 'png') ? '已有版本' : '点击自动生成' }}</text>
      </view>

      <view class="hero-card">
        <view class="hero-line">
          <text class="order-no">{{ order.order_no }}</text>
          <text class="amount">¥{{ order.grand_total || '0.00' }}</text>
        </view>
        <text class="customer">{{ order.customer || '-' }}</text>
        <view class="date-grid">
          <text>单据日期：{{ order.document_date || order.order_date || '-' }}</text>
          <text>订单日期：{{ order.order_date || '-' }}</text>
        </view>
        <button v-if="canEdit" class="edit-order-button" @tap="openEditor">编辑订单</button>
        <text v-else-if="editBlockReason" class="edit-block-reason">{{ editBlockReason }}</text>
      </view>

      <view class="section">
        <text class="section-title">收件信息</text>
        <view class="info-grid">
          <view><text class="label">收件人</text><text>{{ order.receiver_name || '-' }}</text></view>
          <view><text class="label">联系电话</text><text>{{ order.receiver_phone || '-' }}</text></view>
          <view><text class="label">收件公司</text><text>{{ order.receiver_company || '-' }}</text></view>
          <view class="wide"><text class="label">收件地址</text><text selectable>{{ order.receiver_address || '-' }}</text></view>
        </view>
      </view>

      <view class="section">
        <text class="section-title">物流信息</text>
        <view class="info-grid">
          <view><text class="label">物流方式</text><text>{{ logisticsLabel() }}</text></view>
          <view><text class="label">寄件人</text><text>{{ order.sender_label || order.sender_name || '-' }}</text></view>
          <view><text class="label">来源仓库</text><text>{{ order.source_warehouse || '-' }}</text></view>
          <view class="wide"><text class="label">物流单号</text><text selectable>{{ order.ship_tracking_no || '-' }}</text></view>
          <view><text class="label">业务来源</text><text>{{ order.portal_service_code || '-' }}</text></view>
        </view>
      </view>

      <view class="section">
        <text class="section-title">订单状态</text>
        <view class="status-grid">
          <view><text>收款</text><text class="status-value">{{ order.pay_status || '-' }}{{ order.payment_method ? ` / ${order.payment_method}` : '' }}</text></view>
          <view><text>发货</text><text class="status-value">{{ order.ship_status || '-' }}</text></view>
          <view><text>生产</text><text class="status-value">{{ order.process_status || '-' }}</text></view>
          <view><text>开票</text><text class="status-value">{{ employeeOrderInvoiceStatusLabel(order.invoice_status) }}</text></view>
        </view>
      </view>

      <view class="section">
        <text class="section-title">费用明细</text>
        <view class="fee-list">
          <view v-for="line in fees" :key="line.key" class="fee-line" :class="{ emphasized: line.emphasized }">
            <text>{{ line.label }}</text><text>{{ line.value }}</text>
          </view>
        </view>
      </view>

      <view class="section">
        <text class="section-title">商品明细</text>
        <view v-if="order.items?.length" class="item-list">
          <view v-for="(item, index) in order.items" :key="item.item_id || item.id || `${item.product_id}-${index}`" class="item-card">
            <view class="item-title-line">
              <text class="item-index">{{ index + 1 }}</text>
              <text class="item-name">{{ employeeOrderItemDisplayName(item) }}</text>
              <text class="item-subtotal">小计 ¥{{ itemSubtotal(item) }}</text>
            </view>
            <text class="item-meta">{{ employeeOrderItemSpecLabel(item) }} × {{ item.qty || '0' }}{{ item.unit || '' }}</text>
            <text class="item-meta">单价：¥{{ item.unit_price || '0.00' }} / {{ item.unit || '-' }}</text>
            <text class="item-meta">价格表版本：{{ itemPriceVersion(item) }}</text>
            <text class="item-meta">价格来源：{{ employeeOrderItemPriceSourceLabel(item) }}</text>
            <text v-if="item.note" class="item-note">明细备注：{{ item.note }}</text>
          </view>
        </view>
        <text v-else class="empty">暂无商品明细</text>
      </view>

      <view class="section">
        <text class="section-title">报价来源</text>
        <view v-if="order.quote_source_trace?.length" class="trace-list">
          <view v-for="(row, index) in order.quote_source_trace" :key="`quote-${row.product_id || index}`" class="trace-card">
            <text class="trace-title">{{ employeeOrderTraceLineLabel(row) }}</text>
            <text v-for="line in traceLines(row, 'quote')" :key="line" class="trace-line">{{ line }}</text>
          </view>
        </view>
        <text v-else class="empty">暂无报价来源</text>
      </view>

      <view class="section">
        <text class="section-title">生产来源</text>
        <view v-if="order.production_source_trace?.length" class="trace-list">
          <view v-for="(row, index) in order.production_source_trace" :key="`production-${row.product_id || index}`" class="trace-card">
            <text class="trace-title">{{ employeeOrderTraceLineLabel(row) }}</text>
            <text v-for="line in traceLines(row, 'production')" :key="line" class="trace-line">{{ line }}</text>
          </view>
        </view>
        <text v-else class="empty">暂无生产来源</text>
      </view>

      <view class="section">
        <text class="section-title">订单信息</text>
        <view class="info-grid">
          <view><text class="label">订单类型</text><text>{{ order.order_type || '-' }}</text></view>
          <view><text class="label">订单来源</text><text>{{ order.source || order.portal_service_code || '-' }}</text></view>
          <view><text class="label">负责人</text><text>{{ order.responsible_name || '-' }}</text></view>
          <view><text class="label">录入人</text><text>{{ order.created_by_employee || '-' }}</text></view>
          <view class="wide"><text class="label">备注</text><text selectable>{{ order.notes || '-' }}</text></view>
        </view>
      </view>

    </template>

    <PullUpBrandFooter />
  </view>
</template>

<style scoped>
.page { min-height: 100vh; padding: 28rpx; box-sizing: border-box; background: #f5f7f6; color: #172c22; }
.hero-card, .section, .state-card { margin-bottom: 20rpx; padding: 26rpx; border: 1rpx solid #dfe7e2; border-radius: 18rpx; background: #fff; }
.hero-line { display: flex; align-items: baseline; justify-content: space-between; gap: 18rpx; }
.order-no { font-size: 34rpx; font-weight: 850; overflow-wrap: anywhere; }
.amount { color: #28624a; font-size: 34rpx; font-weight: 850; }
.customer { display: block; margin-top: 12rpx; font-size: 29rpx; font-weight: 700; }
.date-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10rpx; margin-top: 14rpx; color: #68766f; font-size: 23rpx; }
.edit-order-button { width: 100%; margin: 20rpx 0 0; border: 1rpx solid #28624a; background: #fff; color: #28624a; font-size: 25rpx; }
.edit-block-reason { display: block; margin-top: 18rpx; padding: 14rpx 16rpx; border-radius: 10rpx; background: #f5f1e8; color: #745b2d; font-size: 22rpx; line-height: 1.5; }
.section-title { display: block; margin-bottom: 20rpx; font-size: 30rpx; font-weight: 800; }
.section-hint { display: block; margin: -8rpx 0 20rpx; color: #6e7d75; font-size: 23rpx; line-height: 1.6; }
.info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 18rpx 22rpx; font-size: 25rpx; }
.info-grid view { min-width: 0; }
.info-grid text:not(.label) { display: block; line-height: 1.55; overflow-wrap: anywhere; }
.info-grid .wide { grid-column: 1 / -1; }
.label { display: block; margin-bottom: 5rpx; color: #748078; font-size: 22rpx; }
.status-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14rpx; }
.status-grid view { padding: 18rpx; border-radius: 12rpx; background: #eef5f1; }
.status-grid text { display: block; }
.status-grid text { color: #6b7971; font-size: 22rpx; }
.status-grid .status-value { margin-top: 7rpx; color: #214d38; font-size: 25rpx; font-weight: 750; }
.fee-list { display: flex; flex-direction: column; }
.fee-line { display: flex; justify-content: space-between; gap: 20rpx; padding: 15rpx 0; border-bottom: 1rpx solid #edf1ee; font-size: 25rpx; }
.fee-line:last-child { border-bottom: 0; }
.fee-line.emphasized { color: #1d5b3f; font-size: 29rpx; font-weight: 850; }
.item-list, .trace-list { display: flex; flex-direction: column; gap: 16rpx; }
.item-card, .trace-card { padding: 20rpx; border: 1rpx solid #dce7e0; border-radius: 14rpx; background: #fbfdfc; }
.item-title-line { display: grid; grid-template-columns: 42rpx minmax(0, 1fr) auto; align-items: start; gap: 10rpx; }
.item-index { display: flex; align-items: center; justify-content: center; width: 36rpx; height: 36rpx; border-radius: 50%; background: #28624a; color: #fff; font-size: 20rpx; }
.item-name { font-size: 27rpx; font-weight: 800; overflow-wrap: anywhere; }
.item-subtotal { color: #28624a; font-size: 27rpx; font-weight: 800; }
.item-meta, .item-note, .trace-line { display: block; margin-top: 9rpx; color: #647269; font-size: 23rpx; line-height: 1.5; overflow-wrap: anywhere; }
.item-note { padding-top: 8rpx; border-top: 1rpx dashed #dfe7e2; color: #55472f; }
.trace-title { display: block; font-size: 26rpx; font-weight: 760; }
.empty { display: block; padding: 20rpx 0; color: #849088; font-size: 24rpx; text-align: center; }
.document-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14rpx; }
.document-grid button { width: 100%; margin: 0; padding: 0 8rpx; background: #28624a; color: #fff; font-size: 24rpx; }
.document-grid button:nth-child(even) { border: 1rpx solid #28624a; background: #fff; color: #28624a; }
.document-status { display: block; margin-top: 13rpx; color: #718078; font-size: 21rpx; line-height: 1.5; }
.state-card { display: flex; align-items: center; justify-content: space-between; gap: 16rpx; color: #526158; }
.error-card { color: #a7352a; }
.state-action { flex: 0 0 auto; margin: 0; padding: 0 20rpx; background: #28624a; color: #fff; font-size: 23rpx; }
@media (max-width: 380px) {
  .date-grid, .info-grid { grid-template-columns: 1fr; }
  .info-grid .wide { grid-column: auto; }
}
</style>
