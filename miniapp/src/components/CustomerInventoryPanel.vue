<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  fetchCustomerInventory,
  type CustomerInventorySummary,
} from '../api/customerPortal'
import { useProcessingPrefillStore } from '../stores/processingPrefill'
import {
  customerInventorySelectionItems,
  customerInventoryDetailPath,
  customerInventoryItemKey,
  toggleCustomerInventorySelection,
  type CustomerInventorySelection,
} from '../utils/customerInventory'

const props = defineProps<{ token: string; customerId: number }>()
const processingPrefill = useProcessingPrefillStore()
const loading = ref(false)
const errorMessage = ref('')
const rows = ref<CustomerInventorySummary[]>([])
const queryInput = ref('')
const query = ref('')
const selectedByKey = ref<CustomerInventorySelection>({})
const page = ref(1)
const pageSize = ref(10)
const jumpPage = ref('1')
const totalRows = ref(0)
const totalPages = ref(1)
const navigating = ref(false)
const pageSizeOptions = [10, 20, 50]
const pageSizeLabels = pageSizeOptions.map((value) => `${value} 条`)
let navigationUnlockTimer: ReturnType<typeof setTimeout> | null = null
let loadVersion = 0

const selectedItems = computed(() => customerInventorySelectionItems(selectedByKey.value))

async function load() {
  const version = ++loadVersion
  loading.value = true
  errorMessage.value = ''
  try {
    const result = await fetchCustomerInventory(props.token, {
      q: query.value,
      page: page.value,
      limit: pageSize.value,
    })
    if (version !== loadVersion) return
    rows.value = result.rows || []
    totalRows.value = Math.max(0, Number(result.total ?? rows.value.length) || 0)
    pageSize.value = Math.max(1, Number(result.limit ?? pageSize.value) || pageSize.value)
    totalPages.value = Math.max(1, Number(result.total_pages) || Math.ceil(totalRows.value / pageSize.value) || 1)
    page.value = Math.min(totalPages.value, Math.max(1, Number(result.page ?? page.value) || 1))
    jumpPage.value = String(page.value)
  } catch (error) {
    if (version !== loadVersion) return
    errorMessage.value = error instanceof Error ? error.message : '客户库存加载失败'
  } finally {
    if (version === loadVersion) loading.value = false
  }
}

function openItem(item: CustomerInventorySummary) {
  uni.navigateTo({ url: customerInventoryDetailPath(item) })
}

function isSelected(item: CustomerInventorySummary): boolean {
  return Boolean(selectedByKey.value[customerInventoryItemKey(item)])
}

function toggleSelection(item: CustomerInventorySummary) {
  selectedByKey.value = toggleCustomerInventorySelection(selectedByKey.value, item)
}

function generateProductionOrder() {
  if (navigating.value) return
  if (!selectedItems.value.length) {
    uni.showToast({ title: '请先选择库存商品', icon: 'none' })
    return
  }
  if (props.customerId <= 0) {
    uni.showToast({ title: '客户信息尚未加载，请稍后重试', icon: 'none' })
    return
  }
  navigating.value = true
  processingPrefill.stage(props.customerId, selectedItems.value)
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

async function previousPage() {
  if (page.value <= 1 || loading.value) return
  page.value -= 1
  await load()
}

async function nextPage() {
  if (page.value >= totalPages.value || loading.value) return
  page.value += 1
  await load()
}

async function goToPage() {
  const target = Math.floor(Number(jumpPage.value) || 1)
  const next = Math.min(totalPages.value, Math.max(1, target))
  jumpPage.value = String(next)
  if (next === page.value || loading.value) return
  page.value = next
  await load()
}

async function setPageSize(event: { detail?: { value?: number | string } }) {
  const index = Number(event.detail?.value ?? 0)
  pageSize.value = pageSizeOptions[index] || 10
  page.value = 1
  jumpPage.value = '1'
  await load()
}

async function applyInventorySearch() {
  query.value = queryInput.value.trim()
  page.value = 1
  jumpPage.value = '1'
  await load()
}

async function clearInventorySearch() {
  queryInput.value = ''
  query.value = ''
  page.value = 1
  jumpPage.value = '1'
  await load()
}

onMounted(() => { void load() })
onBeforeUnmount(() => {
  loadVersion += 1
  if (navigationUnlockTimer) clearTimeout(navigationUnlockTimer)
})
</script>

<template>
  <view class="workspace">
    <view class="panel">
      <text class="title">我的库存</text>
      <text class="hint">仅展示绑定到当前客户的成品仓库存，不显示成本、供应商或操作人。</text>
      <view class="search-row">
        <input v-model="queryInput" class="search" placeholder="搜索商品名称" confirm-type="search" @confirm="applyInventorySearch" />
        <button class="secondary search-button" :disabled="loading" @tap="clearInventorySearch">清除</button>
        <button class="primary search-button" :disabled="loading" @tap="applyInventorySearch">查询</button>
      </view>

      <view class="batch-action">
        <text class="selection-count">已选 {{ selectedItems.length }} 项</text>
        <button class="primary compact-action" :disabled="!selectedItems.length || navigating" @tap="generateProductionOrder">
          {{ navigating ? '打开中...' : `生成生产工单（${selectedItems.length}）` }}
        </button>
      </view>

      <text v-if="loading" class="hint">加载中...</text>
      <view
        v-for="item in rows"
        :key="customerInventoryItemKey(item)"
        class="inventory"
        hover-class="inventory-active"
        @tap="openItem(item)">
        <view class="inventory-main">
          <checkbox class="selection" :checked="isSelected(item)" color="#2b2118" @tap.stop="toggleSelection(item)" />
          <view class="inventory-copy">
            <view class="head"><text class="name">{{ item.product_name }}</text><text class="available">可用 {{ item.available_qty }}</text></view>
            <text class="hint">{{ item.sku_code || `SKU ${item.product_id}` }} · 规格 {{ item.spec_g }}g · 预留 {{ item.reserved_qty }}</text>
            <text class="hint">{{ item.warehouses.join('、') }}</text>
            <text class="detail-hint">查看库存详情 ›</text>
          </view>
        </view>
      </view>

      <text v-if="!loading && !totalRows && query.trim()" class="hint empty">没有找到匹配的商品</text>
      <text v-if="!loading && !totalRows && !query.trim()" class="hint empty">当前客户绑定成品仓暂无库存</text>

      <view class="pagination">
        <text class="page-summary">共 {{ totalRows }} 条 · 共 {{ totalPages }} 页</text>
        <view class="page-controls">
          <button class="secondary page-button" :disabled="page <= 1 || loading" @tap="previousPage">上一页</button>
          <text class="current-page">第 {{ page }} / {{ totalPages }} 页</text>
          <button class="secondary page-button" :disabled="page >= totalPages || loading" @tap="nextPage">下一页</button>
        </view>
        <view class="page-settings">
          <picker mode="selector" :range="pageSizeLabels" :value="Math.max(0, pageSizeOptions.indexOf(pageSize))" @change="setPageSize">
            <view class="picker-field">每页 {{ pageSize }} 条</view>
          </picker>
          <input v-model="jumpPage" class="jump-input" type="number" placeholder="页码" />
          <button class="secondary jump-button" :disabled="loading" @tap="goToPage">跳转</button>
        </view>
      </view>
    </view>
    <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
  </view>
</template>

<style scoped>
.workspace,.panel,.inventory,.inventory-copy,.pagination{display:flex;flex-direction:column;gap:14rpx}.panel{padding:24rpx;margin-bottom:20rpx;border:1rpx solid #e6e0d8;border-radius:8rpx;background:#fff}.title{font-size:30rpx;font-weight:900}.name{font-size:27rpx;font-weight:800}.hint{color:#707070;font-size:23rpx;line-height:1.5}.search,.jump-input{min-height:76rpx;padding:0 20rpx;border:1rpx solid #ddd;border-radius:8rpx;background:#fafafa;box-sizing:border-box}.batch-action,.head,.inventory-main,.page-controls,.page-settings,.search-row{display:flex;align-items:center;gap:14rpx}.batch-action,.head{justify-content:space-between}.search{min-width:0;flex:1}.search-button{width:auto;min-height:76rpx;padding:0 20rpx;font-size:23rpx}.selection-count{color:#5f5f5f;font-size:23rpx}.inventory{padding:18rpx;border:1rpx solid #eee;border-radius:8rpx}.inventory-active{background:#fffaf2}.inventory-main{align-items:flex-start}.selection{flex:0 0 auto;margin-top:2rpx}.inventory-copy{min-width:0;flex:1;gap:7rpx}.available{flex:0 0 auto;color:#28624a;font-weight:800}.detail-hint{color:#28624a;font-size:22rpx;text-align:right}.primary,.secondary{min-height:72rpx;margin:0;border-radius:8rpx}.primary{background:#2b2118;color:#fff}.primary[disabled]{background:#aaa;color:#fff}.secondary{background:#fff;border:1rpx solid #ddd}.compact-action{width:auto;min-height:64rpx;padding:0 18rpx;font-size:23rpx}.pagination{padding-top:18rpx;border-top:1rpx solid #eee}.page-summary,.current-page{color:#666;font-size:22rpx;text-align:center}.page-controls{justify-content:space-between}.page-button{flex:0 0 150rpx;font-size:22rpx}.current-page{flex:1}.page-settings{justify-content:center}.picker-field,.jump-input,.jump-button{min-height:64rpx;font-size:22rpx}.picker-field{display:flex;align-items:center;padding:0 16rpx;border:1rpx solid #ddd;border-radius:8rpx;background:#fff}.jump-input{width:120rpx}.jump-button{padding:0 20rpx}.empty{padding:24rpx 0;text-align:center}.error{display:block;padding:18rpx;color:#b42318}
</style>
