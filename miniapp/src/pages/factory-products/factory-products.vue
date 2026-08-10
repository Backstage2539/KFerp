<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import {
  buildBeanListPDFPath,
  buildBeanListPNGPath,
  fetchCustomerProducts,
  type BeanListGroupSummary,
  type BeanListSummary,
  type CustomerPriceTableGroup,
} from '../../api/customerPortal'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import MainTabBar from '../../components/MainTabBar.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { useSessionStore } from '../../stores/session'
import { openMiniappFileOutput } from '../../utils/fileOutput'
import { miniappThemeClass } from '../../utils/themes'

const session = useSessionStore()
const loading = ref(false)
const outputLoading = ref(false)
const errorMessage = ref('')
const factory_price_table_groups = ref<CustomerPriceTableGroup[]>([])
const expandedTypes = ref<Record<string, boolean>>({})
const expandedCategories = ref<Record<string, boolean>>({})

const themeClass = computed(() => miniappThemeClass(session.themeKey))

function typeKey(group: CustomerPriceTableGroup): string {
  return String(group.list_type || group.list_type_label || '').trim()
}

function categoryKey(table: BeanListSummary, group: BeanListGroupSummary): string {
  return `${table.id}-${group.category || 'default'}`
}

function typeExpanded(group: CustomerPriceTableGroup): boolean {
  return expandedTypes.value[typeKey(group)] !== false
}

function categoryExpanded(table: BeanListSummary, group: BeanListGroupSummary): boolean {
  return expandedCategories.value[categoryKey(table, group)] !== false
}

function toggleType(group: CustomerPriceTableGroup) {
  const key = typeKey(group)
  expandedTypes.value = { ...expandedTypes.value, [key]: !typeExpanded(group) }
}

function toggleCategory(table: BeanListSummary, group: BeanListGroupSummary) {
  const key = categoryKey(table, group)
  expandedCategories.value = { ...expandedCategories.value, [key]: !categoryExpanded(table, group) }
}

async function openBeanListOutput(item: BeanListSummary, kind: 'pdf' | 'png') {
  if (!session.token || !item.id || outputLoading.value) return
  outputLoading.value = true
  errorMessage.value = ''
  const path = kind === 'pdf' ? buildBeanListPDFPath(item.id) : buildBeanListPNGPath(item.id)
  await openMiniappFileOutput({ path, token: session.token, kind })
  outputLoading.value = false
}

async function loadFactoryProducts() {
  if (!session.token) {
    uni.reLaunch({ url: '/pages/login/login' })
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    const page = await fetchCustomerProducts(session.token)
    factory_price_table_groups.value = page.factory_price_table_groups || []
    const nextExpanded: Record<string, boolean> = {}
    for (const group of factory_price_table_groups.value) {
      nextExpanded[typeKey(group)] = true
    }
    expandedTypes.value = nextExpanded
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '工厂商品表加载失败'
  } finally {
    loading.value = false
  }
}

onShow(() => {
  void loadFactoryProducts()
})
</script>

<template>
  <view class="page pull-up-brand-page pull-up-brand-page-with-tabbar" :class="themeClass">
    <EnvironmentBadge />
    <view class="header">
      <text class="eyebrow">商品价格表</text>
      <text class="title">工厂商品表</text>
      <text class="subtitle">工厂给当前客户看的最新商品价格表</text>
    </view>

    <view v-if="loading" class="state">加载中...</view>
    <text v-else-if="errorMessage" class="error">{{ errorMessage }}</text>

    <view v-else class="price-table-list">
      <view v-for="group in factory_price_table_groups" :key="`factory-${group.list_type}`" class="type-block">
        <view class="type-head" @tap="toggleType(group)">
          <view class="type-main">
            <text class="type-title">{{ group.list_type_label || group.list_type }}</text>
            <text class="type-meta">{{ group.product_count || 0 }} 个商品 / {{ group.price_table_count || 0 }} 个价格表</text>
          </view>
          <text class="fold-text">{{ typeExpanded(group) ? '收起' : '展开' }}</text>
        </view>

        <view v-if="typeExpanded(group) && group.latest_version" class="table-entry">
          <view class="table-head">
            <view class="table-main">
              <text class="table-title">{{ group.latest_version.title || '商品价格表' }}</text>
              <text class="table-meta">{{ group.latest_version.version_no || '最新版本' }} / {{ group.latest_version.published_at || '已发布' }}</text>
            </view>
            <view class="output-actions">
              <button class="output-button" :disabled="outputLoading" @tap.stop="openBeanListOutput(group.latest_version, 'pdf')">PDF</button>
              <button class="output-button" :disabled="outputLoading" @tap.stop="openBeanListOutput(group.latest_version, 'png')">长图</button>
            </view>
          </view>

          <view v-for="(section, sectionIndex) in group.latest_version.groups || []" :key="`${group.latest_version.id}-${section.category}-${sectionIndex}`" class="category-block">
            <view class="category-head" @tap="toggleCategory(group.latest_version, section)">
              <text class="category-title">{{ section.category || '未分类' }}</text>
              <text class="fold-text">{{ categoryExpanded(group.latest_version, section) ? '收起' : '展开' }}</text>
            </view>
            <view v-if="categoryExpanded(group.latest_version, section)" class="product-list">
              <view v-for="(item, itemIndex) in section.items || []" :key="`${group.latest_version.id}-${sectionIndex}-${item.code || item.name}-${itemIndex}`" class="product-row">
                <view class="product-main">
                  <text class="product-name">{{ item.name }}</text>
                  <text class="product-meta">{{ item.code || '无编号' }}</text>
                </view>
                <view class="price-list">
                  <text v-for="(price, priceIndex) in item.prices || []" :key="`${item.code || item.name}-${price.label}-${priceIndex}`" class="price-pill">{{ price.label }} {{ price.value }}</text>
                </view>
              </view>
            </view>
          </view>
        </view>
      </view>

      <view v-if="!factory_price_table_groups.length" class="empty">暂无工厂商品表。</view>
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

.price-table-list,
.table-entry,
.product-list {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
}

.price-table-list {
  margin-top: 22rpx;
}

.type-block,
.table-entry,
.category-block {
  border: 1rpx solid #dce7e1;
  border-radius: 10rpx;
  background: #ffffff;
}

.type-head,
.table-head,
.category-head,
.product-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18rpx;
  padding: 22rpx;
}

.type-main,
.table-main,
.product-main {
  min-width: 0;
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 8rpx;
}

.type-title,
.table-title,
.category-title,
.product-name {
  color: #14201a;
  font-size: 28rpx;
  font-weight: 900;
}

.type-meta,
.table-meta,
.product-meta,
.fold-text {
  color: #66756c;
  font-size: 24rpx;
}

.output-actions {
  display: flex;
  gap: 10rpx;
}

.output-button {
  min-width: 96rpx;
  height: 56rpx;
  border: 1rpx solid #bdd3c8;
  border-radius: 8rpx;
  background: #eef6f2;
  color: #173b2e;
  font-size: 24rpx;
  font-weight: 900;
  line-height: 56rpx;
}

.category-block {
  margin: 0 18rpx 18rpx;
}

.product-row {
  border-top: 1rpx solid #edf2ef;
}

.price-list {
  max-width: 330rpx;
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8rpx;
}

.price-pill {
  padding: 8rpx 12rpx;
  border-radius: 8rpx;
  background: #f4f8f6;
  color: #173b2e;
  font-size: 22rpx;
  font-weight: 800;
}

.state,
.empty,
.error {
  margin-top: 22rpx;
  padding: 24rpx;
  border-radius: 10rpx;
  background: #ffffff;
  color: #66756c;
  font-size: 26rpx;
}

.error {
  color: #b42318;
}
</style>
