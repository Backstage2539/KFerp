<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import {
  buildResaleBeanListPDFPath,
  buildResaleBeanListPNGPath,
  fetchCustomerProducts,
  type BeanListSummary,
  type CustomerPriceTableGroup,
  type CustomerProductSummary,
} from '../../api/customerPortal'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import MainTabBar from '../../components/MainTabBar.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { useSessionStore } from '../../stores/session'
import { openMiniappFileOutput } from '../../utils/fileOutput'
import { miniappThemeClass } from '../../utils/themes'

const session = useSessionStore()
const loading = ref(false)
const errorMessage = ref('')
const outputLoading = ref(false)
const customerPriceTableGroups = ref<CustomerPriceTableGroup[]>([])
const customerProducts = ref<CustomerProductSummary[]>([])
const expandedGroups = ref<Record<string, boolean>>({})

const themeClass = computed(() => miniappThemeClass(session.themeKey))

function groupKey(group: CustomerPriceTableGroup): string {
  return String(group.list_type || group.list_type_label || '').trim()
}

function customerPriceTableExpanded(group: CustomerPriceTableGroup): boolean {
  return expandedGroups.value[groupKey(group)] === true
}

function toggleCustomerPriceTableGroup(group: CustomerPriceTableGroup) {
  const key = groupKey(group)
  expandedGroups.value = { ...expandedGroups.value, [key]: !customerPriceTableExpanded(group) }
}

function openSettings() {
  uni.navigateTo({ url: '/pages/price-table-settings/price-table-settings' })
}

async function openResaleOutput(item: BeanListSummary, kind: 'pdf' | 'png') {
  if (!session.token || !item.id || outputLoading.value) return
  outputLoading.value = true
  const path = kind === 'pdf' ? buildResaleBeanListPDFPath(item.id) : buildResaleBeanListPNGPath(item.id)
  await openMiniappFileOutput({ path, token: session.token, kind })
  outputLoading.value = false
}

async function loadCustomerProducts() {
  if (!session.token) {
    uni.reLaunch({ url: '/pages/login/login' })
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    const page = await fetchCustomerProducts(session.token)
    customerPriceTableGroups.value = page.customer_price_table_groups || []
    customerProducts.value = page.products || []
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '我的商品加载失败'
  } finally {
    loading.value = false
  }
}

onShow(() => {
  void loadCustomerProducts()
})
</script>

<template>
  <view class="page pull-up-brand-page pull-up-brand-page-with-tabbar" :class="themeClass">
    <EnvironmentBadge />
    <view class="header">
      <text class="eyebrow">客户商品</text>
      <text class="title">我的商品</text>
      <text class="subtitle">管理客户自己发布的商品价格表</text>
    </view>

    <view class="action-bar">
      <button class="primary-button" @tap="openSettings">价格表设置</button>
    </view>

    <view v-if="loading" class="state">加载中...</view>
    <text v-else-if="errorMessage" class="error">{{ errorMessage }}</text>

    <view v-else class="panel-list">
      <view class="summary-panel">
        <view class="summary-item">
          <text class="summary-value">{{ customerProducts.length }}</text>
          <text class="summary-label">我的商品</text>
        </view>
        <view class="summary-item">
          <text class="summary-value">{{ customerPriceTableGroups.length }}</text>
          <text class="summary-label">价格表类型</text>
        </view>
      </view>

      <view class="panel">
        <view class="panel-head">
          <text class="panel-title">已发布商品价格表</text>
          <button class="link-button" @tap="openSettings">设置</button>
        </view>
        <view v-if="customerPriceTableGroups.length" class="version-groups">
          <view v-for="group in customerPriceTableGroups" :key="`customer-${group.list_type}`" class="version-group">
            <view class="group-head" @tap="toggleCustomerPriceTableGroup(group)">
              <view class="group-main">
                <text class="group-title">{{ group.list_type_label || group.list_type }}</text>
                <text class="group-meta">最新 {{ group.latest_version?.version_no || '暂无版本' }} / 共 {{ group.price_table_count || 0 }} 个版本</text>
              </view>
              <text class="fold-text">{{ customerPriceTableExpanded(group) ? '收起' : '展开' }}</text>
            </view>
            <view v-if="customerPriceTableExpanded(group)" class="version-list">
              <view v-for="item in group.versions || []" :key="`version-${item.id}`" class="version-row">
                <view class="version-main">
                  <text class="version-title">{{ item.title || '商品价格表' }}</text>
                  <text class="version-meta">{{ item.version_no || '未标版本' }} / {{ item.published_at || '已发布' }}</text>
                </view>
                <view class="output-actions">
                  <button class="output-button" :disabled="outputLoading" @tap.stop="openResaleOutput(item, 'pdf')">PDF</button>
                  <button class="output-button" :disabled="outputLoading" @tap.stop="openResaleOutput(item, 'png')">长图</button>
                </view>
              </view>
            </view>
          </view>
        </view>
        <text v-else class="empty">还没有发布自己的商品价格表。</text>
      </view>
    </view>

    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter with-fixed-tabbar />
    </view>
    <MainTabBar current="mine" />
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 28rpx 28rpx 160rpx;
  background: #f5f7f6;
  box-sizing: border-box;
}

.header {
  display: flex;
  flex-direction: column;
  gap: 12rpx;
  padding: 30rpx 28rpx;
  border-radius: 10rpx;
  background: #173b2e;
}

.eyebrow,
.subtitle {
  color: rgba(255, 255, 255, 0.76);
  font-size: 24rpx;
}

.title {
  color: #ffffff;
  font-size: 40rpx;
  font-weight: 900;
}

.action-bar,
.panel-list,
.version-groups,
.version-list {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
}

.action-bar,
.panel-list {
  margin-top: 22rpx;
}

.primary-button {
  width: 100%;
  height: 78rpx;
  border-radius: 10rpx;
  background: #171717;
  color: #ffffff;
  font-size: 28rpx;
  font-weight: 900;
  line-height: 78rpx;
}

.summary-panel,
.panel,
.version-group {
  border: 1rpx solid #dce7e1;
  border-radius: 10rpx;
  background: #ffffff;
}

.summary-panel {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rpx;
  overflow: hidden;
}

.summary-item {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  padding: 24rpx;
}

.summary-value {
  color: #14201a;
  font-size: 38rpx;
  font-weight: 900;
}

.summary-label,
.group-meta,
.version-meta,
.fold-text {
  color: #66756c;
  font-size: 24rpx;
}

.panel {
  padding: 22rpx;
}

.panel-head,
.group-head,
.version-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18rpx;
}

.panel-head {
  margin-bottom: 18rpx;
}

.panel-title,
.group-title,
.version-title {
  color: #14201a;
  font-size: 28rpx;
  font-weight: 900;
}

.link-button {
  min-width: 90rpx;
  height: 52rpx;
  border: 1rpx solid #bdd3c8;
  border-radius: 8rpx;
  background: #eef6f2;
  color: #173b2e;
  font-size: 24rpx;
  line-height: 52rpx;
}

.group-head,
.version-row {
  padding: 20rpx;
}

.group-main,
.version-main {
  min-width: 0;
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 8rpx;
}

.version-list {
  padding: 0 16rpx 16rpx;
}

.version-row {
  border-top: 1rpx solid #edf2ef;
}

.output-actions {
  display: flex;
  gap: 10rpx;
}

.output-button {
  min-width: 84rpx;
  height: 54rpx;
  border: 1rpx solid #bdd3c8;
  border-radius: 8rpx;
  background: #eef6f2;
  color: #173b2e;
  font-size: 24rpx;
  font-weight: 900;
  line-height: 54rpx;
}

.state,
.empty,
.error {
  padding: 22rpx;
  border-radius: 10rpx;
  background: #ffffff;
  color: #66756c;
  font-size: 26rpx;
}

.error {
  display: block;
  margin-top: 22rpx;
  color: #b42318;
}
</style>
