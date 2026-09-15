<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onShareAppMessage, onShareTimeline, onShow } from '@dcloudio/uni-app'
import { fetchOrderPriceTablePreview, type OrderPriceTablePreview } from '../../api/customerPortal'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { useSessionStore } from '../../stores/session'
import { tierQuantityLabel } from '../../utils/customerOrderEntry'
import { defaultMiniappShare, defaultMiniappTimelineShare, refreshMiniappShareMenu } from '../../utils/miniappShare'

const session = useSessionStore()
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
const usageCode = ref<'direct_ship' | 'product_order'>('direct_ship')
const loading = ref(false)
const errorMessage = ref('')
const preview = ref<OrderPriceTablePreview | null>(null)
const title = computed(() => usageCode.value === 'product_order' ? '现货下单价格表' : '一件代发价格表')

onLoad((options) => {
  usageCode.value = options?.usage_code === 'product_order' ? 'product_order' : 'direct_ship'
})

async function load() {
  if (!session.token) {
    uni.reLaunch({ url: '/pages/login/login' })
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    preview.value = await fetchOrderPriceTablePreview(session.token, usageCode.value)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '价格表加载失败'
  } finally {
    loading.value = false
  }
}

function money(value?: number): string { return Number(value || 0).toFixed(2) }
function priceTableSpecLabel(row: { spec_name?: string }): string {
  const name = String(row.spec_name || '').trim()
  return name && name !== '默认规格' ? name : '规格待维护'
}

onShow(() => { void load(); void refreshMiniappShareMenu() })
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
</script>

<template>
  <view class="page pull-up-brand-page" @touchstart="handlePullUpBrandTouchStart" @touchmove="handlePullUpBrandTouchMove" @touchend="handlePullUpBrandTouchEnd" @touchcancel="handlePullUpBrandTouchCancel">
    <EnvironmentBadge />
    <view class="head">
      <text class="title">{{ title }}</text>
      <text class="hint">当前登录客户正在使用的已发布价格，只读预览</text>
    </view>
    <view v-for="table in preview?.price_tables || []" :key="table.id" class="table-card">
      <text class="table-name">{{ table.table_name || table.list_type || '当前价格表' }}</text>
      <text class="table-version">{{ table.version_no ? `版本 ${table.version_no}` : '' }}</text>
    </view>
    <text v-if="loading" class="state">价格表加载中...</text>
    <scroll-view v-else-if="preview?.rows?.length" scroll-x class="table-scroll">
      <view class="price-grid">
        <view class="table-row table-header"><text>商品</text><text>规格</text><text>数量档位</text><text>单价</text></view>
        <view v-for="(row, index) in preview.rows" :key="`${row.publication_id}:${row.product_id}:${row.bom_spec_id || 0}:${index}`" class="table-row">
          <text>{{ row.product_name }}</text>
          <text>{{ priceTableSpecLabel(row) }}</text>
          <text>{{ tierQuantityLabel(row) }}</text>
          <text class="price">¥{{ money(row.unit_price) }}/{{ row.sales_unit || '件' }}</text>
        </view>
      </view>
    </scroll-view>
    <text v-else-if="!loading && !errorMessage" class="state">当前价格表暂无可预览的商品价格</text>
    <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
    <view class="pull-up-brand-footer-anchor"><PullUpBrandFooter :revealed="pullUpBrandRevealed" /></view>
  </view>
</template>

<style scoped>
.page{min-height:100vh;padding:28rpx;box-sizing:border-box;background:#f4f7f5;color:#173126}.head{display:flex;flex-direction:column;gap:10rpx;margin:16rpx 0 24rpx}.title{font-size:38rpx;font-weight:900}.hint,.table-version,.state{color:#728078;font-size:24rpx}.table-card{display:flex;justify-content:space-between;gap:16rpx;margin-bottom:18rpx;padding:22rpx;border:1rpx solid #dbe5df;border-radius:16rpx;background:#fff}.table-name{font-weight:800}.table-scroll{width:100%;border:1rpx solid #dbe5df;border-radius:16rpx;background:#fff}.price-grid{min-width:940rpx}.table-row{display:grid;grid-template-columns:260rpx 220rpx 220rpx 240rpx;min-height:84rpx;border-bottom:1rpx solid #e8eeea}.table-row text{display:flex;align-items:center;padding:14rpx 18rpx;border-right:1rpx solid #eef2ef;font-size:24rpx}.table-header{min-height:72rpx;background:#eaf3ee;font-weight:800}.price{color:#17603f;font-weight:800}.state{display:block;padding:80rpx 20rpx;text-align:center}.error{display:block;padding:24rpx;color:#b42318}
</style>
