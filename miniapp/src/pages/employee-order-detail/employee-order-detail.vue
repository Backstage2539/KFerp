<script setup lang="ts">
import {orderProgressLabel} from '../../utils/orderProgress'
import OrderDetailsCopyButton from '../../components/OrderDetailsCopyButton.vue'
import OrderItemWeights from '../../components/OrderItemWeights.vue'
import PaymentSummary from '../../components/PaymentSummary.vue'
import { computed, ref } from 'vue'
import { onLoad, onShow, onShareAppMessage, onShareTimeline } from '@dcloudio/uni-app'
import {
  defaultMiniappShare,
  defaultMiniappTimelineShare,
  refreshMiniappShareMenu,
} from '../../utils/miniappShare'
import {
  buildEmployeeOrderDocumentPath,
  fetchEmployeeOrderDetail,
  fetchEmployeeShareSettings,
  generateEmployeeOrderDocument,
  updateEmployeeOrderStatus,
  uploadEmployeeOrderPaymentVoucher,
  type EmployeeOrderDetail,
  type EmployeeOrderDetailResponse,
  type EmployeeOrderStatusOptions,
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
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
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
import { employeeReceiptMethods, useEmployeeOrderStatus } from '../../composables/useEmployeeOrderStatus'

const session = useSessionStore()
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
const orderID = ref(0)
const order = ref<EmployeeOrderDetail>()
const documents = ref<EmployeeOrderDocuments>({})
const loading = ref(false)
const errorMessage = ref('')
const authExpired = ref(false)
const sharingKey = ref('')
const canEdit = ref(false)
const editBlockReason = ref('')
const canUpdateStatus = ref(false)
const statusBlockReason = ref('')
const canUpdateShipStatus = ref(false)
const shipStatusBlockReason = ref('')
const statusOptions = ref<EmployeeOrderStatusOptions>({ pay_statuses: [], ship_statuses: [] })
const voucherUploading = ref(false)
const voucherFilename = ref('')
const {
  editing: statusEditing, saving: statusSaving, error: statusError, draft: statusDraft,
  start: beginStatusEdit, cancel: cancelStatusEdit, save: submitStatus,
} = useEmployeeOrderStatus({
  order: () => order.value,
  canUpdate: () => canUpdateStatus.value,
  options: () => statusOptions.value,
  request: payload => updateEmployeeOrderStatus(session.token, orderID.value, payload),
  refresh: () => fetchEmployeeOrderDetail(session.token, orderID.value),
  onSaved: response => {
    applyDetail(response)
    uni.showToast({ title: '状态已保存', icon: 'success' })
  },
  onError: cause => {
    if (isAuthenticationExpiredRequestError(cause)) {
      session.clearSession()
      authExpired.value = true
      errorMessage.value = '登录已失效，请重新登录'
    }
  },
})
const selectedPaymentStatus = computed(() => statusOptions.value.pay_statuses.find(status => status.id === statusDraft.value.pay_status_id)?.name || '')
const noPaymentRequired = computed(() => order.value?.pay_status === '无需收款')
const statusNeedsMethod = computed(() => !noPaymentRequired.value && /预付款|已付|已收|已支付/.test(selectedPaymentStatus.value))
const statusNeedsReceipt = computed(() => !noPaymentRequired.value && selectedPaymentStatus.value.includes('已收款'))

function startStatusEdit() {
  voucherFilename.value = order.value?.payment_voucher?.filename || (order.value?.payment_voucher_asset_id ? '已保存收款凭证' : '')
  beginStatusEdit()
}

function saveStatus() {
  if (!voucherUploading.value) void submitStatus()
}

function choosePaymentVoucher() {
  if (voucherUploading.value || statusSaving.value) return
  uni.chooseImage({
    count: 1,
    sizeType: ['compressed'],
    success: async result => {
      const filePath = result.tempFilePaths[0]
      if (!filePath) return
      voucherUploading.value = true
      statusError.value = ''
      try {
        const uploaded = await uploadEmployeeOrderPaymentVoucher(session.token, orderID.value, filePath)
        if (!uploaded.asset?.id) throw new Error('凭证上传未完成，请重试')
        statusDraft.value.payment_voucher_asset_id = uploaded.asset.id
        voucherFilename.value = uploaded.asset.filename || '收款凭证已上传'
      } catch (cause) {
        statusError.value = cause instanceof Error ? cause.message : '凭证上传失败，请重试'
        if (isAuthenticationExpiredRequestError(cause)) {
          session.clearSession()
          authExpired.value = true
          errorMessage.value = '登录已失效，请重新登录'
        }
      } finally {
        voucherUploading.value = false
      }
    },
  })
}

function applyDetail(response: EmployeeOrderDetailResponse) {
  order.value = response.order
  documents.value = response.documents || {}
  canEdit.value = Boolean(response.can_edit ?? response.order.can_edit)
  editBlockReason.value = String(response.edit_block_reason || response.order.edit_block_reason || '')
  canUpdateStatus.value = Boolean(response.can_update_status) && !response.order.is_void
  statusBlockReason.value = response.status_update_block_reason || ''
  canUpdateShipStatus.value = Boolean(response.can_update_ship_status)
  shipStatusBlockReason.value = response.ship_status_update_block_reason || ''
  statusOptions.value = response.status_options || { pay_statuses: [], ship_statuses: [] }
}

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
    applyDetail(response)
    if (!statusEditing.value) statusError.value = ''
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

function openCopy() {
  if (orderID.value <= 0) return
  uni.navigateTo({
    url: `/pages/employee-order-entry/employee-order-entry?copy_id=${orderID.value}`,
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
        applyDetail({ ...refreshed, documents: refreshed.documents || documents.value })
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

onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
onShow(() => { void refreshMiniappShareMenu() })
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
    <view v-if="loading" class="state-card"><text>订单详情加载中...</text></view>
    <view v-else-if="errorMessage" class="state-card error-card">
      <text>{{ errorMessage }}</text>
      <button v-if="authExpired" class="state-action" @tap="goToLogin">重新登录</button>
      <button v-else class="state-action" @tap="loadDetail">重试</button>
    </view>

    <template v-else-if="order">
      <view class="section status-section">
        <view class="status-heading">
          <text class="section-title">订单状态</text>
          <button v-if="canUpdateStatus && !statusEditing" class="status-edit-button" @tap="startStatusEdit">{{ order.delivery_mode === 'customer_stock' ? '编辑付款状态' : '编辑付款与发货状态' }}</button>
        </view>
        <text v-if="order.is_void" class="void-status">已作废</text>
        <PaymentSummary :order="order" />
        <view v-if="!statusEditing" class="status-grid">
          <view><text>收款</text><text class="status-value">{{ order.pay_status || '-' }}{{ order.payment_method ? ` / ${order.payment_method}` : '' }}</text></view>
          <view v-if="order.delivery_mode !== 'customer_stock'"><text>发货</text><text class="status-value">{{ order.ship_status || '-' }}</text></view>
          <view><text>履约</text><text class="status-value">{{ orderProgressLabel(order) }}</text></view>
          <view><text>开票</text><text class="status-value">{{ employeeOrderInvoiceStatusLabel(order.invoice_status) }}</text></view>
        </view>
        <view v-else class="status-editor">
          <text class="label">付款状态</text>
          <view v-if="noPaymentRequired" class="status-field">无需收款 · 已结清</view>
          <picker v-else :disabled="statusSaving" :range="statusOptions.pay_statuses" range-key="name" :value="Math.max(0, statusOptions.pay_statuses.findIndex(status => status.id === statusDraft.pay_status_id))" @change="statusDraft.pay_status_id = statusOptions.pay_statuses[Number($event.detail.value)]?.id || 0">
            <view class="status-field">{{ selectedPaymentStatus || order.pay_status || '请选择付款状态' }}<text>⌄</text></view>
          </picker>
          <template v-if="!noPaymentRequired">
            <template v-if="selectedPaymentStatus.includes('预付款') || Number(statusDraft.prepayment_amount) > 0">
              <text class="label">已支付预付款（元）</text>
              <input v-model="statusDraft.prepayment_amount" :disabled="statusSaving" class="status-field" type="digit" placeholder="填写实际已收预付款" />
            </template>
            <template v-if="statusNeedsMethod">
              <text class="label">收款方式</text>
              <picker :disabled="statusSaving" :range="employeeReceiptMethods" :value="Math.max(0, employeeReceiptMethods.indexOf(statusDraft.payment_method))" @change="statusDraft.payment_method = employeeReceiptMethods[Number($event.detail.value)] || ''">
                <view class="status-field">{{ statusDraft.payment_method || '请选择收款方式' }}<text>⌄</text></view>
              </picker>
            </template>
            <template v-if="statusNeedsReceipt">
              <text class="label">实收货款（元）</text>
              <input v-model="statusDraft.payment_goods_amount" :disabled="statusSaving" class="status-field" type="digit" placeholder="填写实际收到的货款" />
              <text class="label">实收运费（元）</text>
              <input v-model="statusDraft.payment_shipping_amount" :disabled="statusSaving" class="status-field" type="digit" placeholder="无运费请填 0" />
              <text class="label">收款凭证</text>
              <text v-if="voucherFilename" class="voucher-filename">{{ voucherFilename }}</text>
              <button :loading="voucherUploading" :disabled="statusSaving || voucherUploading" class="voucher-upload-button" @tap="choosePaymentVoucher">{{ statusDraft.payment_voucher_asset_id ? '更换收款凭证' : '上传收款截图' }}</button>
            </template>
          </template>
          <text v-if="order.delivery_mode !== 'customer_stock'" class="label">发货状态</text>
          <picker v-if="order.delivery_mode !== 'customer_stock' && canUpdateShipStatus" :disabled="statusSaving" :range="statusOptions.ship_statuses" range-key="name" :value="Math.max(0, statusOptions.ship_statuses.findIndex(status => status.id === statusDraft.ship_status_id))" @change="statusDraft.ship_status_id = statusOptions.ship_statuses[Number($event.detail.value)]?.id || 0">
            <view class="status-field">{{ statusOptions.ship_statuses.find(status => status.id === statusDraft.ship_status_id)?.name || order.ship_status || '请选择发货状态' }}<text>⌄</text></view>
          </picker>
          <view v-else-if="order.delivery_mode !== 'customer_stock'" class="status-field status-field-readonly">{{ order.ship_status || '-' }}</view>
          <text v-if="!canUpdateShipStatus && shipStatusBlockReason" class="section-hint status-hint">{{ shipStatusBlockReason }}</text>
          <view class="status-actions">
            <button :disabled="statusSaving || voucherUploading" class="status-cancel-button" @tap="cancelStatusEdit">取消</button>
            <button :loading="statusSaving" :disabled="statusSaving || voucherUploading" class="status-save-button" @tap="saveStatus">保存状态</button>
          </view>
        </view>
        <text v-if="!canUpdateStatus && statusBlockReason" class="edit-block-reason">{{ statusBlockReason }}</text>
        <text v-if="statusError" class="status-error">{{ statusError }}</text>
        <button v-if="statusError && !statusEditing" class="status-refresh-button" @tap="loadDetail">刷新详情</button>
      </view>

      <view class="section document-section">
        <text class="section-title">导出并微信分享</text>
        <text class="section-hint">没有历史版本时会先按网页同一模板生成，再打开微信分享；低版本微信会回退到预览菜单。</text>
        <view class="document-grid">
          <button :loading="sharingKey === 'sales-order.pdf'" :disabled="Boolean(sharingKey)" @tap="shareDocument('sales-order', 'pdf')">销售单 PDF</button>
          <button :loading="sharingKey === 'sales-order.png'" :disabled="Boolean(sharingKey)" @tap="shareDocument('sales-order', 'png')">销售单图片</button>
          <button v-if="order.delivery_mode !== 'customer_stock'" :loading="sharingKey === 'delivery-note.pdf'" :disabled="Boolean(sharingKey)" @tap="shareDocument('delivery-note', 'pdf')">发货单 PDF</button>
          <button v-if="order.delivery_mode !== 'customer_stock'" :loading="sharingKey === 'delivery-note.png'" :disabled="Boolean(sharingKey)" @tap="shareDocument('delivery-note', 'png')">发货单图片</button>
        </view>
        <text class="document-status">销售单：PDF {{ hasDocument('sales-order', 'pdf') ? '已有版本' : '点击自动生成' }}，图片 {{ hasDocument('sales-order', 'png') ? '已有版本' : '点击自动生成' }}</text>
        <text v-if="order.delivery_mode !== 'customer_stock'" class="document-status">发货单：PDF {{ hasDocument('delivery-note', 'pdf') ? '已有版本' : '点击自动生成' }}，图片 {{ hasDocument('delivery-note', 'png') ? '已有版本' : '点击自动生成' }}</text>
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
        <view class="hero-actions">
          <OrderDetailsCopyButton :order-id="order.id" employee />
          <button class="copy-order-button" @tap="openCopy">复制订单</button>
          <button v-if="canEdit" class="edit-order-button" @tap="openEditor">编辑订单</button>
        </view>
        <text v-if="!canEdit && editBlockReason" class="edit-block-reason">{{ editBlockReason }}</text>
      </view>

      <view v-if="order.delivery_mode !== 'customer_stock'" class="section">
        <text class="section-title">收件信息</text>
        <view class="info-grid">
          <view><text class="label">收件人</text><text>{{ order.receiver_name || '-' }}</text></view>
          <view><text class="label">联系电话</text><text>{{ order.receiver_phone || '-' }}</text></view>
          <view><text class="label">收件公司</text><text>{{ order.receiver_company || '-' }}</text></view>
          <view class="wide"><text class="label">收件地址</text><text selectable>{{ order.receiver_address || '-' }}</text></view>
        </view>
      </view>

      <view v-if="order.delivery_mode !== 'customer_stock'" class="section">
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
            <OrderItemWeights :item="item" />
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

    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter :revealed="pullUpBrandRevealed" />
    </view>
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
.hero-actions { display: flex; flex-wrap: wrap; gap: 14rpx; margin-top: 20rpx; }
.hero-actions button { flex: 1; width: auto; margin: 0; font-size: 25rpx; }
.edit-order-button, .copy-order-button { border: 1rpx solid #28624a; background: #fff; color: #28624a; }
.edit-block-reason { display: block; margin-top: 18rpx; padding: 14rpx 16rpx; border-radius: 10rpx; background: #f5f1e8; color: #745b2d; font-size: 22rpx; line-height: 1.5; }
.section-title { display: block; margin-bottom: 20rpx; font-size: 30rpx; font-weight: 800; }
.section-hint { display: block; margin: -8rpx 0 20rpx; color: #6e7d75; font-size: 23rpx; line-height: 1.6; }
.info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 18rpx 22rpx; font-size: 25rpx; }
.info-grid view { min-width: 0; }
.info-grid text:not(.label) { display: block; line-height: 1.55; overflow-wrap: anywhere; }
.info-grid .wide { grid-column: 1 / -1; }
.label { display: block; margin-bottom: 5rpx; color: #748078; font-size: 22rpx; }
.status-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14rpx; }
.status-heading { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12rpx; margin-bottom: 8rpx; }
.status-heading .section-title { margin: 0; }
.status-edit-button { width: auto; margin: 0; padding: 0 16rpx; border: 1rpx solid #28624a; background: #fff; color: #28624a; font-size: 23rpx; }
.status-section .status-grid { margin-top: 18rpx; }
.status-editor { margin-top: 22rpx; }
.status-editor .label { margin-top: 20rpx; }
.status-field { display: flex; align-items: center; justify-content: space-between; min-height: 78rpx; width: 100%; padding: 18rpx 20rpx; box-sizing: border-box; border: 1rpx solid #ceddd3; border-radius: 12rpx; background: #fff; font-size: 26rpx; }
input.status-field { height: 82rpx; }
.status-field-readonly { background: #f1f5f2; color: #62766b; }
.status-hint { margin: 12rpx 0; }
.status-actions { display: flex; gap: 16rpx; margin-top: 24rpx; }
.status-actions button { flex: 1; margin: 0; font-size: 26rpx; }
.status-cancel-button { border: 1rpx solid #d4dfd8; background: #fff; color: #526158; }
.status-save-button { background: #28624a; color: #fff; }
.status-error, .void-status { display: block; margin-top: 16rpx; color: #a7352a; font-size: 24rpx; line-height: 1.6; }
.status-refresh-button { margin-top: 14rpx; background: #fff; color: #28624a; font-size: 24rpx; }
.voucher-filename { display: block; margin: 10rpx 0; color: #526158; font-size: 23rpx; overflow-wrap: anywhere; }
.voucher-upload-button { margin: 0; border: 1rpx solid #28624a; background: #fff; color: #28624a; font-size: 25rpx; }
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
