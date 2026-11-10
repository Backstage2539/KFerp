<script setup lang="ts">
import { computed, ref } from 'vue'
import { canUseEmployeePriceCenter } from '../../utils/priceCenterAccess'
import { fetchStaffNotifications } from '../../api/samples'
import { onShow, onShareAppMessage, onShareTimeline } from '@dcloudio/uni-app'
import {
  defaultMiniappShare,
  defaultMiniappTimelineShare,
  refreshMiniappShareMenu,
} from '../../utils/miniappShare'
import {
  fetchMe,
  fetchServicePage,
  fetchCustomerIssuedBills,
  type CustomerFulfillmentSettings,
} from '../../api/customerPortal'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import MainTabBar from '../../components/MainTabBar.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { useSessionStore } from '../../stores/session'
import { visibleHomeEntries } from '../../utils/capabilities'
import { previewSafeEntries } from '../../utils/customerPreview'
import { miniappThemeClass, miniappThemeMeta } from '../../utils/themes'

const session = useSessionStore()
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
const staffUnread = ref(0)
const loading = ref(false)
const errorMessage = ref('')
const fulfillmentSettings = ref<CustomerFulfillmentSettings>()
const orderCounts = ref<{ accepted: number; in_production: number; shipped: number } | null>(null)
const unpaidBills = ref<{ count: number; cents: number } | null>(null)
const overviewError = ref(false)
let contextVersion = 0
const progressCards = [{key: 'accepted', label: '已接单'}, {key: 'in_production', label: '生产中'}, {key: 'shipped', label: '已发货'}] as const
const hasFulfillmentOverview = computed(() => session.capabilities.some(item => item.enabled && (item.code === 'direct_ship' || item.code === 'settlement')))
const canSeeBills = computed(() => session.capabilities.some(item => item.enabled && item.code === 'settlement'))

const entries = computed(() => visibleHomeEntries(session.capabilities, fulfillmentSettings.value))
const employeeEntries = [
  { key: 'priceCenter', label: '价格表中心', url: '/pages/bean-list-center/bean-list-center' },
  { key: 'sampleRequests', label: '样品工单', url: '/pages/sample-requests/sample-requests' },
  { key: 'staffNotifications', label: '员工通知', url: '/pages/employee-notifications/employee-notifications' },
  { key: 'employeeOrderEntry', label: '录单', url: '/pages/employee-order-entry/employee-order-entry' },
  { key: 'employeeOrders', label: '查看订单', url: '/pages/employee-orders/employee-orders' },
  { key: 'employeeCustomers', label: '客户维护', url: '/pages/employee-customers/employee-customers' },
  { key: 'customerPreview', label: '客户只读预览', url: '/pages/admin-customer-preview/admin-customer-preview' },
  { key: 'employeeProfile', label: '个人中心', url: '/pages/profile/profile' },
]
const visibleEntries = computed(() => {
  if (session.accountType !== 'employee') return session.isCustomerPreview ? previewSafeEntries(entries.value) : entries.value
  const canMaintainCustomers = session.permissions.includes('customers.read')
    && session.permissions.includes('customers.write')
  return employeeEntries.filter((entry) => (entry.key !== 'employeeCustomers' || canMaintainCustomers) && (entry.key !== 'customerPreview' || session.roles.includes('admin')) && (!['priceCenter','sampleRequests','staffNotifications'].includes(entry.key) || canUseEmployeePriceCenter(session)))
})
const customerName = computed(() => session.accountType === 'employee' ? `${session.employeeName || '员工'} · 简易 ERP` : (session.currentCustomerName || '客户中心'))
const themeClass = computed(() => miniappThemeClass(session.themeKey))
const themeMeta = computed(() => miniappThemeMeta(session.themeKey))

function openEntry(url: string) {
  uni.navigateTo({ url })
}

async function loadContext() {
  const version = ++contextVersion
  if (!session.token) {
    uni.reLaunch({ url: '/pages/index/index' })
    return
  }

  loading.value = true
  errorMessage.value = ''
  const requestToken = session.token

  try {
    const response = await fetchMe(requestToken)
    if (version !== contextVersion || session.token !== requestToken) return
    session.applyContext(response)
 if(response.account_type!=='employee'&&!response.current_customer_id){uni.reLaunch({url:'/pages/index/index'});return}
    fulfillmentSettings.value = undefined
    if (response.account_type !== 'employee') {
      fulfillmentSettings.value = {customer_id:response.current_customer_id,revision:0,modes:(response.fulfillment_modes||[]).map(mode=>({mode,enabled:true,warehouse_code:'',fee_configured:false,fixed_fee_cents:0,per_package_fee_cents:0,shipping_template_id:0}))}
      orderCounts.value = null
      unpaidBills.value = null
      overviewError.value = false
      const customerID = response.current_customer_id
      if (hasFulfillmentOverview.value) {
      const results = await Promise.allSettled([
        fetchServicePage(requestToken, 'orders', { page: 1, page_size: 1 }),
        canSeeBills.value ? fetchCustomerIssuedBills(requestToken, { status: 'unpaid', page: 1, limit: 1 }) : Promise.resolve(null),
      ])
      if (version !== contextVersion || session.token !== requestToken || session.currentCustomerID !== customerID) return
      if (results[0].status === 'fulfilled') orderCounts.value = results[0].value.progress_counts || null
      else overviewError.value = true
      if (results[1].status === 'fulfilled' && results[1].value) {
        const summary = results[1].value.summary
        unpaidBills.value = { count: summary.unpaid_count, cents: summary.due_amount_cents }
      } else if (results[1].status === 'rejected') overviewError.value = true
      }
    }
    if(canUseEmployeePriceCenter(session)){try{staffUnread.value=(await fetchStaffNotifications(session.token,true)).unread_count||0}catch{staffUnread.value=0}}
  } catch (error) {
    if (version !== contextVersion || session.token !== requestToken) return
    errorMessage.value = error instanceof Error ? error.message : '客户信息加载失败'
    session.clearSession()
    uni.reLaunch({ url: '/pages/index/index' })
  } finally {
    if (version === contextVersion) loading.value = false
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

    <view v-if="session.accountType !== 'employee' && hasFulfillmentOverview && !loading" class="overview-grid">
      <view v-for="card in progressCards" :key="card.key" class="overview-card" @tap="openEntry(`/pages/service/service?key=orders&source=official&progress=${card.key}`)">
        <text>{{ card.label }}</text><strong>{{ orderCounts ? orderCounts[card.key] : '—' }}</strong>
      </view>
      <view v-if="canSeeBills" class="overview-card" @tap="openEntry('/pages/service/service?key=settlement&status=unpaid')">
        <text>待付款账单</text><strong>{{ unpaidBills ? unpaidBills.count : '—' }}</strong>
        <text v-if="unpaidBills" class="overview-hint">未付 ¥{{ (unpaidBills.cents / 100).toFixed(2) }}</text>
      </view>
    </view>
    <text v-if="overviewError && session.accountType !== 'employee'" class="overview-hint">部分数据加载失败，请刷新页面重试</text>

    <view v-if="loading" class="state">
      <text>加载中...</text>
    </view>

    <view v-else-if="errorMessage" class="state error">
      <text>{{ errorMessage }}</text>
    </view>

    <view v-else-if="visibleEntries.length">
      <view class="grid">
      <view v-for="entry in visibleEntries" :key="entry.key" class="entry" hover-class="entry-pressed" @tap="openEntry(entry.url)">
        <text class="entry-label">{{ entry.label }}{{entry.key==='staffNotifications'&&staffUnread?' ('+staffUnread+')':''}}</text>
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
.overview-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16rpx; margin-bottom: 24rpx; }
.overview-card { display: flex; flex-direction: column; gap: 8rpx; padding: 24rpx; border: 1rpx solid #e7e2d9; border-radius: 16rpx; background: #fff; color: #333; font-size: 25rpx; }
.overview-card strong { font-size: 38rpx; }
.overview-hint { color: #777; font-size: 23rpx; }
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
