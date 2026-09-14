<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow, onShareAppMessage, onShareTimeline } from '@dcloudio/uni-app'
import {
  defaultMiniappShare,
  defaultMiniappTimelineShare,
  refreshMiniappShareMenu,
} from '../../utils/miniappShare'
import {
  fetchCustomerAccount,
  fetchCustomerAssetInventory,
  fetchDirectShipRequests,
  fetchMe,
  fetchProcessingRequests,
} from '../../api/customerPortal'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import MainTabBar from '../../components/MainTabBar.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { useSessionStore } from '../../stores/session'
import { visibleHomeEntries } from '../../utils/capabilities'
import { miniappThemeClass, miniappThemeMeta } from '../../utils/themes'

const session = useSessionStore()
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
const loading = ref(false)
const errorMessage = ref('')
const overviewLoading = ref(false)
const overviewMetrics = ref<Array<{ key: string; label: string; value: string; hint: string; url: string }>>([])

const entries = computed(() => visibleHomeEntries(session.capabilities))
const employeeEntries = [
  { key: 'employeeOrderEntry', label: '录单', url: '/pages/employee-order-entry/employee-order-entry' },
  { key: 'employeeOrders', label: '查看订单', url: '/pages/employee-orders/employee-orders' },
  { key: 'employeeCustomers', label: '客户维护', url: '/pages/employee-customers/employee-customers' },
  { key: 'employeeProfile', label: '个人中心', url: '/pages/profile/profile' },
]
const visibleEntries = computed(() => {
  if (session.accountType !== 'employee') return entries.value
  const canMaintainCustomers = session.permissions.includes('customers.read')
    && session.permissions.includes('customers.write')
  return employeeEntries.filter((entry) => entry.key !== 'employeeCustomers' || canMaintainCustomers)
})
const customerName = computed(() => session.accountType === 'employee' ? `${session.employeeName || '员工'} · 简易 ERP` : (session.currentCustomerName || '客户中心'))
const themeClass = computed(() => miniappThemeClass(session.themeKey))
const themeMeta = computed(() => miniappThemeMeta(session.themeKey))

function openEntry(url: string) {
  uni.navigateTo({ url })
}

function capabilityEnabled(code: string): boolean {
  return session.capabilities.some((item) => item.code === code && item.enabled)
}

async function loadCustomerOverview() {
  if (session.accountType === 'employee' || !session.token) { overviewMetrics.value = []; return }
  overviewLoading.value = true
  const jobs: Array<Promise<{ key: string; label: string; value: string; hint: string; url: string }>> = []
  if (capabilityEnabled('direct_ship') || capabilityEnabled('processing')) jobs.push(
    fetchDirectShipRequests(session.token, { page: 1, limit: 50 }).then((result) => {
      const pending = (result.rows || []).filter((item) => !['shipped', 'delivered', 'cancelled'].includes(item.status)).length
      return { key: 'orders', label: '待处理订单', value: String(pending), hint: '查看生产与发货', url: '/pages/service/service?key=orders' }
    }),
  )
  if (capabilityEnabled('processing')) jobs.push(
    fetchProcessingRequests(session.token).then((result) => ({
      key: 'processing', label: '生产进度',
      value: String((result.rows || []).filter((item) => !['completed', 'cancelled'].includes(item.status)).length),
      hint: '待排产及生产中', url: '/pages/service/service?key=processing',
    })),
  )
  if (capabilityEnabled('inventory_custody') || capabilityEnabled('processing')) jobs.push(
    fetchCustomerAssetInventory(session.token).then((result) => ({
      key: 'inventory', label: '客户库存', value: String((result.rows || []).length), hint: '项客户货权库存', url: '/pages/service/service?key=inventory',
    })),
  )
  if (capabilityEnabled('settlement')) jobs.push(
    fetchCustomerAccount(session.token, { period: 'month', page: 1, limit: 1 }).then((result) => ({
      key: 'settlement', label: '待对账',
      value: String((result.settlements || []).filter((item) => item.reconciliation_status !== 'confirmed').length),
      hint: `未付 ¥${(Number(result.summary?.due_cents || 0) / 100).toFixed(2)}`, url: '/pages/service/service?key=settlement',
    })),
  )
  const results = await Promise.allSettled(jobs)
  overviewMetrics.value = results.flatMap((result) => result.status === 'fulfilled' ? [result.value] : [])
  overviewLoading.value = false
}

async function loadContext() {
  if (!session.token) {
    uni.reLaunch({ url: '/pages/index/index' })
    return
  }

  loading.value = true
  errorMessage.value = ''

  try {
    const response = await fetchMe(session.token)
    session.applyContext(response)
    await loadCustomerOverview()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '客户信息加载失败'
    session.clearSession()
    uni.reLaunch({ url: '/pages/index/index' })
  } finally {
    loading.value = false
  }
}

onShow(() => {
  void loadContext()
})

onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
onShow(() => { void refreshMiniappShareMenu() })
</script>

<template>
  <view
    class="page pull-up-brand-page"
    :class="[themeClass, { 'pull-up-brand-page-with-tabbar': session.accountType !== 'employee' }]"
    @touchstart="handlePullUpBrandTouchStart"
    @touchmove="handlePullUpBrandTouchMove"
    @touchend="handlePullUpBrandTouchEnd"
    @touchcancel="handlePullUpBrandTouchCancel"
  >
    <EnvironmentBadge />
    <view class="header">
      <text class="eyebrow">{{ themeMeta.eyebrow }}</text>
      <text class="title">{{ customerName }}</text>
      <text class="subtitle">{{ themeMeta.subtitle }}</text>
    </view>

    <view v-if="loading" class="state">
      <text>加载中...</text>
    </view>

    <view v-else-if="errorMessage" class="state error">
      <text>{{ errorMessage }}</text>
    </view>

    <view v-else-if="visibleEntries.length">
      <view v-if="overviewMetrics.length" class="overview-grid">
        <view v-for="metric in overviewMetrics" :key="metric.key" class="metric" @tap="openEntry(metric.url)">
          <text class="metric-label">{{ metric.label }}</text><text class="metric-value">{{ metric.value }}</text><text class="metric-hint">{{ metric.hint }}</text>
        </view>
      </view>
      <text v-else-if="overviewLoading" class="overview-loading">业务摘要加载中...</text>
      <view class="grid">
      <view v-for="entry in visibleEntries" :key="entry.key" class="entry" hover-class="entry-pressed" @tap="openEntry(entry.url)">
        <text class="entry-label">{{ entry.label }}</text>
      </view>
      </view>
    </view>

    <view v-else class="state">
      <text>暂无可用服务</text>
    </view>

    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter
        :with-fixed-tabbar="session.accountType !== 'employee'"
        :revealed="pullUpBrandRevealed"
      />
    </view>
    <MainTabBar v-if="session.accountType !== 'employee'" current="home" />
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

.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20rpx;
}
.overview-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14rpx; margin-bottom: 24rpx; }
.metric { display: flex; flex-direction: column; gap: 8rpx; padding: 20rpx; border: 1rpx solid #e5ddd2; border-radius: 14rpx; background: #fff; }
.metric-label,.metric-hint { color: #666; font-size: 22rpx; }.metric-value { color: #2b2118; font-size: 38rpx; font-weight: 900; }.overview-loading { display:block; margin-bottom:20rpx; color:#666; font-size:24rpx; }

.entry {
  min-height: 168rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24rpx;
  background: #fffaf2;
  border: 1rpx solid #ead9bd;
  border-radius: 16rpx;
  box-sizing: border-box;
}

.theme-clean-ops .entry {
  background: #ffffff;
  border-color: #dde7e1;
}

.theme-premium-partner .entry {
  background: #fffdf8;
  border-color: #eadab7;
}

.entry-pressed {
  transform: scale(.99);
  opacity: .86;
}

.entry-label {
  color: #171717;
  font-size: 30rpx;
  font-weight: 800;
  line-height: 1.35;
  text-align: center;
}

.state {
  min-height: 180rpx;
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
