<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onShow, onShareAppMessage, onShareTimeline } from '@dcloudio/uni-app'
import { fetchProcessingRequestDetail, type ProcessingRequest } from '../../api/customerPortal'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { useSessionStore } from '../../stores/session'
import { useCustomerOrderDraftStore } from '../../stores/customerOrderDraft'
import { createDirectShipDraftLine } from '../../utils/directShipDraft'
import { shanghaiToday } from '../../utils/employeeOrder'
import { processingRequestProgress, processingRequestTimeline, productionStatusLabel } from '../../utils/customerFulfillment'
import { defaultMiniappShare, defaultMiniappTimelineShare, refreshMiniappShareMenu } from '../../utils/miniappShare'

const session = useSessionStore()
const orderDraft = useCustomerOrderDraftStore()
const requestID = ref(0)
const loading = ref(false)
const errorMessage = ref('')
const request = ref<ProcessingRequest | null>(null)
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
const progress = computed(() => processingRequestProgress(request.value || {}))
const timeline = computed(() => processingRequestTimeline(request.value || {}))
const currentOperation = computed(() => {
  const rows = request.value?.items || []
  if (rows.some((item) => item.status === 'paused')) return '生产暂停，请联系工厂确认'
  if (rows.some((item) => Number(item.output_shortfall_qty || 0) > 0)) return '产出存在缺口，关联订单可能受影响'
  const active = rows.find((item) => ['running', 'paused', 'partially_completed'].includes(item.status))
  if (active?.current_operation) return `当前工序：${active.current_operation}`
  if (active) return '生产工序执行中'
  return productionStatusLabel(request.value?.status)
})

async function load() {
  if (!session.token || requestID.value <= 0) return
  loading.value = true
  errorMessage.value = ''
  try { request.value = (await fetchProcessingRequestDetail(session.token, requestID.value)).request }
  catch (error) { errorMessage.value = error instanceof Error ? error.message : '生产工单详情加载失败' }
  finally { loading.value = false }
}
function openOrder(orderNo: string) { uni.navigateTo({ url: `/pages/service/service?key=orders&q=${encodeURIComponent(orderNo)}` }) }
function continueOrder() {
  const item = request.value?.items?.[0]
  if (!item) return
  orderDraft.saveDraft(session.currentCustomerID, 'direct_ship', {
    order_date: shanghaiToday(), recipient: null, note: `关联生产申请 ${request.value?.request_no || ''}`,
    lines: [{
      ...createDirectShipDraftLine(),
      product_family_key: `processing:${item.product_id}`,
      product_id: Number(item.product_id || 0),
      bom_spec_id: Number(item.bom_spec_id || 0) || undefined,
      bom_variant_id: Number(item.bom_variant_id || 0) || undefined,
      product_name: item.product_name,
      spec_g: Number(item.bom_spec_id || 0) > 0 ? 0 : Number(item.spec_g || 0),
      spec_label: item.spec_name || `${item.spec_g || 0}g`,
      inventory_unit: item.inventory_unit,
      qty: 1,
    }],
    idempotency_key: `processing-direct-ship-${request.value?.id || 0}-${Date.now()}`,
  })
  uni.navigateTo({ url: '/pages/service/service?key=directShip' })
}
onLoad((query) => { requestID.value = Number(query?.id || 0) })
onShow(() => { void load(); void refreshMiniappShareMenu() })
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
</script>

<template>
  <view class="page pull-up-brand-page" @touchstart="handlePullUpBrandTouchStart" @touchmove="handlePullUpBrandTouchMove" @touchend="handlePullUpBrandTouchEnd" @touchcancel="handlePullUpBrandTouchCancel">
    <EnvironmentBadge />
    <view v-if="loading" class="state-card">生产工单加载中...</view>
    <view v-else-if="errorMessage" class="state-card error"><text>{{ errorMessage }}</text><button @tap="load">重试</button></view>
    <template v-else-if="request">
      <view class="hero">
        <view><text class="eyebrow">生产工单</text><text class="title">{{ request.request_no }}</text><text class="hint">{{ request.created_at }} · {{ session.currentCustomerName }}</text></view>
        <text class="status">{{ productionStatusLabel(request.status) }}</text>
      </view>
      <view class="timeline">
        <view v-for="step in timeline" :key="step.key" class="timeline-step" :class="step.state"><view class="dot" /><view><text class="step-label">{{ step.label }}</text><text class="hint">{{ step.time }}</text></view></view>
      </view>
      <view class="metrics">
        <view><text>申请数量</text><strong>{{ progress.requestedQty }}</strong></view><view><text>累计入库</text><strong>{{ progress.inboundQty }}</strong></view><view><text>有效预订</text><strong>{{ progress.activeReservedQty }}</strong></view><view><text>剩余可预订</text><strong>{{ progress.remainingReservableQty }}</strong></view>
      </view>
      <view class="notice" :class="{ warning: request.status === 'paused' || Number(request.output_shortfall_qty || 0) > 0 }">{{ currentOperation }}</view>
      <button class="continue-order" @tap="continueOrder">使用现货或在制产出继续下单</button>
      <view v-for="item in request.items || []" :key="item.id || item.line_no" class="card">
        <view class="card-head"><text class="product">{{ item.product_name }}</text><text>{{ productionStatusLabel(item.status) }}</text></view>
        <text class="hint">{{ item.spec_name || `${item.spec_g}g` }} · BOM {{ item.bom_version_no || '暂无记录' }}</text>
        <view class="line-grid"><text>申请 {{ item.qty }} {{ item.inventory_unit || '件' }}</text><text>入库 {{ item.actual_inbound_qty || 0 }}</text><text>目标仓库 {{ item.target_warehouse || '暂无记录' }}</text><text>物料预订 {{ item.material_reserved_g || item.material_reserved_units || 0 }}</text></view>
        <view v-for="order in item.related_orders || []" :key="order.order_id" class="order" @tap="openOrder(order.order_no)"><view><text class="product">{{ order.order_no }}</text><text class="hint">预订 {{ order.reserved_qty }} · 已转库存 {{ order.converted_qty }} · 缺口 {{ order.shortfall_qty }}</text></view><text>查看 ›</text></view>
      </view>
    </template>
    <view class="pull-up-brand-footer-anchor"><PullUpBrandFooter :revealed="pullUpBrandRevealed" /></view>
  </view>
</template>

<style scoped>
.page{min-height:100vh;padding:26rpx;box-sizing:border-box;background:#f5f1eb;color:#2b2118}.hero,.timeline,.metrics,.card,.state-card,.notice{padding:26rpx;margin-bottom:20rpx;border:1rpx solid #e6ded3;border-radius:20rpx;background:#fff}.hero,.card-head,.order{display:flex;justify-content:space-between;gap:18rpx}.hero>view{display:flex;flex-direction:column;gap:8rpx}.eyebrow,.hint,.metrics text{color:#82776d;font-size:22rpx}.title{font-size:36rpx;font-weight:900}.status{align-self:flex-start;padding:8rpx 16rpx;border-radius:999rpx;background:#e8f2eb;color:#245c40;font-weight:850}.timeline{display:grid;grid-template-columns:repeat(4,1fr);gap:4rpx}.timeline-step{display:flex;flex-direction:column;align-items:center;text-align:center;gap:10rpx}.timeline-step>view:last-child{display:flex;flex-direction:column;gap:5rpx}.dot{width:22rpx;height:22rpx;border-radius:50%;background:#d5cec5}.timeline-step.done .dot,.timeline-step.current .dot{background:#9a6938}.timeline-step.current .step-label{color:#9a6938}.step-label{font-size:22rpx;font-weight:850}.metrics{display:grid;grid-template-columns:repeat(4,1fr);gap:10rpx}.metrics view{display:flex;flex-direction:column;gap:8rpx;text-align:center}.metrics strong{font-size:32rpx}.notice{color:#315c44;background:#eef7f1}.notice.warning,.error{color:#a34432;background:#fff2ee}.continue-order{min-height:82rpx;margin:0 0 20rpx;border-radius:14rpx;background:#2b2118;color:#fff;font-size:26rpx;font-weight:850}.card{display:flex;flex-direction:column;gap:14rpx}.product{font-size:28rpx;font-weight:900}.line-grid{display:grid;grid-template-columns:1fr 1fr;gap:14rpx;font-size:23rpx}.order{align-items:center;padding:18rpx;border-radius:14rpx;background:#f8f5f0}.order>view{display:flex;flex-direction:column;gap:6rpx}.state-card{display:flex;flex-direction:column;gap:16rpx}
</style>
