<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { onLoad, onShow } from '@dcloudio/uni-app'
import {
  fetchCustomerInventory,
  fetchCustomerInventoryBatches,
  type CustomerInventoryBatch,
  type CustomerInventorySummary,
} from '../../api/customerPortal'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { useProcessingPrefillStore } from '../../stores/processingPrefill'
import { useSessionStore } from '../../stores/session'

const session = useSessionStore()
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
const processingPrefill = useProcessingPrefillStore()
const productID = ref(0)
const specG = ref(0)
const loading = ref(false)
const errorMessage = ref('')
const navigating = ref(false)
const summary = ref<CustomerInventorySummary | null>(null)
const batches = ref<CustomerInventoryBatch[]>([])
let navigationUnlockTimer: ReturnType<typeof setTimeout> | null = null
let loadVersion = 0

const firstBatch = computed(() => batches.value[0])
const productName = computed(() => summary.value?.product_name || firstBatch.value?.product_name || `商品 ${productID.value}`)
const skuCode = computed(() => summary.value?.sku_code || firstBatch.value?.sku_code || `SKU ${productID.value}`)
const warehouses = computed(() => summary.value?.warehouses || Array.from(new Set(batches.value.map((item) => item.warehouse))))

async function load() {
  const version = ++loadVersion
  const token = session.token
  const customerID = session.currentCustomerID
  const requestedProductID = productID.value
  const requestedSpecG = specG.value
  if (!token) {
    loading.value = false
    summary.value = null
    batches.value = []
    errorMessage.value = '登录已失效，请返回后重新登录'
    return
  }
  if (requestedProductID <= 0 || requestedSpecG <= 0) {
    loading.value = false
    summary.value = null
    batches.value = []
    errorMessage.value = '库存商品参数不正确'
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    const [inventoryResponse, batchResponse] = await Promise.all([
      fetchCustomerInventory(token),
      fetchCustomerInventoryBatches(token, requestedProductID, requestedSpecG),
    ])
    if (
      version !== loadVersion
      || token !== session.token
      || customerID !== session.currentCustomerID
      || requestedProductID !== productID.value
      || requestedSpecG !== specG.value
    ) return
    const currentSummary = (inventoryResponse.rows || []).find((item) => (
      Number(item.product_id) === requestedProductID && Number(item.spec_g) === requestedSpecG
    )) || null
    if (!currentSummary) {
      summary.value = null
      batches.value = []
      errorMessage.value = '当前库存已变化，请返回库存列表刷新'
      return
    }
    summary.value = currentSummary
    batches.value = batchResponse.rows || []
    uni.setNavigationBarTitle({ title: `${productName.value}库存` })
  } catch (error) {
    if (version !== loadVersion) return
    errorMessage.value = error instanceof Error ? error.message : '库存详情加载失败'
  } finally {
    if (version === loadVersion) loading.value = false
  }
}

function productionDate(item: CustomerInventoryBatch): string {
  if (item.historical_without_production_date || !item.production_date) return '历史库存，暂无生产日期'
  return item.production_date
}

function addProductionOrder() {
  if (navigating.value) return
  if (session.currentCustomerID <= 0) {
    uni.showToast({ title: '客户信息尚未加载，请稍后重试', icon: 'none' })
    return
  }
  navigating.value = true
  processingPrefill.stage(session.currentCustomerID, [{
    product_id: productID.value,
    spec_g: specG.value,
    product_name: productName.value,
    sku_code: skuCode.value,
  }])
  uni.navigateTo({
    url: '/pages/service/service?key=processing',
    fail: () => {
      processingPrefill.clear()
      navigating.value = false
    },
    success: () => {
      navigationUnlockTimer = setTimeout(() => { navigating.value = false }, 800)
    },
  })
}

onLoad((query) => {
  productID.value = Number(query?.product_id || 0)
  specG.value = Number(query?.spec_g || 0)
})
onShow(() => { void load() })
onBeforeUnmount(() => {
  loadVersion += 1
  if (navigationUnlockTimer) clearTimeout(navigationUnlockTimer)
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
    <view v-if="loading" class="state-card"><text>库存详情加载中...</text></view>
    <view v-else-if="errorMessage" class="state-card error-card">
      <text>{{ errorMessage }}</text>
      <button class="secondary retry" @tap="load">重试</button>
    </view>

    <template v-else>
      <view class="summary-card">
        <view class="summary-head">
          <view class="summary-copy">
            <text class="title">{{ productName }}</text>
            <text class="hint">{{ skuCode }} · 规格 {{ specG }}g</text>
          </view>
          <text class="available">可用 {{ summary?.available_qty || 0 }}</text>
        </view>
        <view class="summary-grid">
          <view><text class="label">库存总数</text><text>{{ summary?.total_qty || 0 }}</text></view>
          <view><text class="label">已预留</text><text>{{ summary?.reserved_qty || 0 }}</text></view>
          <view class="wide"><text class="label">所在仓库</text><text>{{ warehouses.join('、') || '暂无记录' }}</text></view>
        </view>
        <button v-if="summary" class="primary" :disabled="navigating" @tap="addProductionOrder">{{ navigating ? '打开中...' : '添加生产工单' }}</button>
      </view>

      <view class="section">
        <text class="section-title">库存批次</text>
        <view v-for="batch in batches" :key="`${batch.batch_id}:${batch.batch_no}:${batch.warehouse}`" class="batch">
          <view class="batch-head"><text class="batch-no">批次 {{ batch.batch_no || batch.batch_id }}</text><text class="available">可用 {{ batch.available_qty }}</text></view>
          <text class="hint">生产日期：{{ productionDate(batch) }}</text>
          <text class="hint">所在仓库：{{ batch.warehouse }}</text>
          <text class="hint">入库时间：{{ batch.inbound_at || '暂无记录' }}</text>
          <text class="hint">已预留 {{ batch.reserved_qty }} · 质量状态 {{ batch.quality_status || '正常' }}</text>
        </view>
        <text v-if="!batches.length" class="empty">当前库存已变化，暂无可追溯批次</text>
      </view>
    </template>

    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter :revealed="pullUpBrandRevealed" />
    </view>
  </view>
</template>

<style scoped>
.page{min-height:100vh;padding:28rpx;background:#f5f7f6;box-sizing:border-box;color:#172c22}.summary-card,.section,.state-card{margin-bottom:20rpx;padding:26rpx;border:1rpx solid #dfe7e2;border-radius:18rpx;background:#fff}.summary-copy,.batch,.state-card{display:flex;flex-direction:column;gap:12rpx}.summary-head,.batch-head{display:flex;align-items:flex-start;justify-content:space-between;gap:18rpx}.summary-copy{min-width:0;flex:1}.title{font-size:34rpx;font-weight:850;overflow-wrap:anywhere}.hint,.empty{color:#68766f;font-size:23rpx;line-height:1.55}.available{flex:0 0 auto;color:#28624a;font-weight:850}.summary-grid{display:grid;grid-template-columns:1fr 1fr;gap:18rpx;margin:24rpx 0}.summary-grid .wide{grid-column:1/-1}.summary-grid text:not(.label){display:block;font-size:25rpx;line-height:1.5}.label{display:block;margin-bottom:5rpx;color:#748078;font-size:22rpx}.primary,.secondary{width:100%;min-height:76rpx;margin:0;border-radius:10rpx}.primary{background:#2b2118;color:#fff}.secondary{background:#fff;border:1rpx solid #d8d8d8}.section-title{display:block;margin-bottom:18rpx;font-size:30rpx;font-weight:850}.batch{padding:18rpx 0;border-top:1rpx solid #edf1ee}.batch:first-of-type{border-top:0}.batch-no{font-size:26rpx;font-weight:800}.empty{display:block;padding:28rpx 0;text-align:center}.state-card{align-items:stretch}.error-card{color:#b42318}.retry{margin-top:10rpx}
</style>
