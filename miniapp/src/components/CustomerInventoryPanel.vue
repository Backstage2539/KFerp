<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  fetchCustomerAssetInventory,
  type CustomerAssetInventory,
} from '../api/customerPortal'
import { useProcessingPrefillStore } from '../stores/processingPrefill'
import { customerInventoryDetailPath } from '../utils/customerInventory'

const props = defineProps<{ token: string; customerId: number }>()
const processingPrefill = useProcessingPrefillStore()
const loading = ref(false)
const errorMessage = ref('')
const rows = ref<CustomerAssetInventory[]>([])
const queryInput = ref('')
const query = ref('')
const activeType = ref('')
const selectedByKey = ref<Record<string, CustomerAssetInventory>>({})
const navigating = ref(false)
let navigationUnlockTimer: ReturnType<typeof setTimeout> | null = null
let loadVersion = 0

const typeOptions = [
  { key: '', label: '全部' },
  { key: 'finished_product', label: '成品' },
  { key: 'green_bean', label: '生豆' },
  { key: 'packaging', label: '包材' },
  { key: 'semi_finished', label: '半成品' },
]

const selectedItems = computed(() => Object.values(selectedByKey.value))

function itemKey(item: CustomerAssetInventory): string {
  return [item.inventory_type, item.item_id, item.bom_spec_id || 0, item.spec_g || 0].join(':')
}

function typeLabel(value: string): string {
  return ({ finished_product: '成品', green_bean: '生豆', packaging: '包材', semi_finished: '半成品' } as Record<string, string>)[value] || value
}

function quantity(value: number, unit: string): string {
  const amount = Number(value || 0)
  const digits = Number.isInteger(amount) ? 0 : 3
  return `${amount.toLocaleString('zh-CN', { maximumFractionDigits: digits })} ${unit || ''}`.trim()
}

function warehouseNames(item: CustomerAssetInventory): string {
  return (item.warehouses || []).map((row) => row.warehouse_name || row.warehouse_code).filter(Boolean).join('、') || '暂无仓库记录'
}

async function load() {
  const version = ++loadVersion
  loading.value = true
  errorMessage.value = ''
  try {
    const result = await fetchCustomerAssetInventory(props.token, activeType.value, query.value)
    if (version !== loadVersion) return
    rows.value = result.rows || []
    const visible = new Set(rows.value.map(itemKey))
    selectedByKey.value = Object.fromEntries(Object.entries(selectedByKey.value).filter(([key]) => visible.has(key)))
  } catch (error) {
    if (version !== loadVersion) return
    errorMessage.value = error instanceof Error ? error.message : '客户库存加载失败'
  } finally {
    if (version === loadVersion) loading.value = false
  }
}

async function selectType(key: string) {
  if (loading.value || activeType.value === key) return
  activeType.value = key
  await load()
}

async function applySearch() {
  query.value = queryInput.value.trim()
  await load()
}

async function clearSearch() {
  queryInput.value = ''
  query.value = ''
  await load()
}

function isSelected(item: CustomerAssetInventory): boolean {
  return Boolean(selectedByKey.value[itemKey(item)])
}

function toggleSelection(item: CustomerAssetInventory) {
  if (!item.can_create_processing_request) return
  const key = itemKey(item)
  const next = { ...selectedByKey.value }
  if (next[key]) delete next[key]
  else next[key] = item
  selectedByKey.value = next
}

function openInventoryDetail(item: CustomerAssetInventory) {
  if (navigating.value) return
  const productID = Number(item.product_id || item.item_id || 0)
  if (productID <= 0) {
    uni.showToast({ title: '商品信息不完整，暂时无法查看库存详情', icon: 'none' })
    return
  }
  navigating.value = true
  uni.navigateTo({
    url: customerInventoryDetailPath({
      product_id: productID,
      bom_spec_id: Number(item.bom_spec_id || 0),
      bom_variant_id: Number(item.bom_variant_id || 0),
      spec_g: Number(item.spec_g || 0),
      inventory_unit: item.unit,
    }),
    fail: () => { navigating.value = false },
    success: () => {
      navigationUnlockTimer = setTimeout(() => { navigating.value = false }, 800)
    },
  })
}

function generateProductionOrder() {
  if (navigating.value || !selectedItems.value.length) return
  if (props.customerId <= 0) {
    uni.showToast({ title: '客户信息尚未加载，请稍后重试', icon: 'none' })
    return
  }
  navigating.value = true
  processingPrefill.stage(props.customerId, selectedItems.value.map((item) => ({
    product_id: Number(item.product_id || item.item_id || 0),
    bom_spec_id: Number(item.bom_spec_id || 0) || undefined,
    bom_variant_id: Number(item.bom_variant_id || 0) || undefined,
    spec_g: Number(item.spec_g || 0),
    spec_name: item.spec,
    inventory_unit: item.unit,
    product_name: item.item_name,
    sku_code: item.item_code,
  })))
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
      <text class="hint">库存按当前客户货权展示。成品、生豆、包材和半成品分别使用各自单位，生产中数量不计入可用库存。</text>

      <scroll-view class="type-tabs" scroll-x>
        <view class="type-tab-row">
          <button
            v-for="option in typeOptions"
            :key="option.key || 'all'"
            class="type-tab"
            :class="{ active: activeType === option.key }"
            :disabled="loading"
            @tap="selectType(option.key)"
          >{{ option.label }}</button>
        </view>
      </scroll-view>

      <view class="search-row">
        <input v-model="queryInput" class="search" placeholder="搜索名称、编码或规格" confirm-type="search" @confirm="applySearch" />
        <button class="secondary search-button" :disabled="loading" @tap="clearSearch">清除</button>
        <button class="primary search-button" :disabled="loading" @tap="applySearch">查询</button>
      </view>

      <view v-if="selectedItems.length" class="batch-action">
        <text class="selection-count">已选 {{ selectedItems.length }} 个代加工成品</text>
        <button class="primary compact-action" :disabled="navigating" @tap="generateProductionOrder">
          {{ navigating ? '打开中...' : '带入生产工单' }}
        </button>
      </view>

      <text v-if="loading" class="hint loading">库存加载中...</text>
      <view v-for="item in rows" :key="itemKey(item)" class="inventory">
        <view class="inventory-main" @tap="openInventoryDetail(item)">
          <checkbox
            v-if="item.can_create_processing_request"
            class="selection"
            :checked="isSelected(item)"
            color="#2b2118"
            @tap.stop="toggleSelection(item)"
          />
          <view class="inventory-copy">
            <view class="head">
              <view class="name-row"><text class="type-badge">{{ typeLabel(item.inventory_type) }}</text><text class="name">{{ item.item_name }}</text></view>
              <text class="available">可用 {{ quantity(item.available_qty, item.unit) }}</text>
            </view>
            <text class="hint">{{ item.item_code || '暂无编码' }}<template v-if="item.spec"> · {{ item.spec }}</template></text>
            <view class="quantity-grid">
              <text>总库存 {{ quantity(item.total_qty, item.unit) }}</text>
              <text>占用 {{ quantity(item.occupied_qty, item.unit) }}</text>
              <text>生产中 {{ quantity(item.in_production_qty, item.unit) }}</text>
            </view>
            <text class="hint">{{ warehouseNames(item) }}</text>
            <text v-if="item.legacy" class="legacy">历史托管库存</text>
            <text class="detail-hint">查看库存详情 ›</text>
          </view>
        </view>
      </view>

      <view v-if="!loading && !rows.length" class="empty-state">
        <text class="empty-title">{{ query ? '没有找到匹配库存' : '当前分类暂无客户库存' }}</text>
        <text class="hint">可切换库存分类，或联系业务人员核对客户货权和仓库。</text>
      </view>
    </view>
    <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
  </view>
</template>

<style scoped>
.workspace,.panel,.inventory,.inventory-copy{display:flex;flex-direction:column;gap:14rpx}.panel{padding:24rpx;margin-bottom:20rpx;border:1rpx solid #e6e0d8;border-radius:8rpx;background:#fff}.title{font-size:30rpx;font-weight:900}.name{font-size:27rpx;font-weight:800}.hint{color:#707070;font-size:23rpx;line-height:1.5}.type-tabs{width:100%;white-space:nowrap}.type-tab-row{display:inline-flex;gap:12rpx}.type-tab{width:auto;min-height:62rpx;padding:0 22rpx;border:1rpx solid #ddd;border-radius:999rpx;background:#fff;font-size:23rpx}.type-tab.active{border-color:#2b2118;background:#2b2118;color:#fff}.search,.search-button{min-height:76rpx}.search{min-width:0;flex:1;padding:0 20rpx;border:1rpx solid #ddd;border-radius:8rpx;background:#fafafa;box-sizing:border-box}.batch-action,.head,.inventory-main,.search-row,.name-row{display:flex;align-items:center;gap:14rpx}.batch-action,.head{justify-content:space-between}.search-button{width:auto;padding:0 20rpx;font-size:23rpx}.selection-count{color:#5f5f5f;font-size:23rpx}.inventory{padding:18rpx;border:1rpx solid #eee;border-radius:8rpx}.inventory-main{align-items:flex-start}.selection{flex:0 0 auto;margin-top:2rpx}.inventory-copy{min-width:0;flex:1;gap:7rpx}.name-row{min-width:0;align-items:center}.type-badge,.legacy{flex:0 0 auto;padding:3rpx 9rpx;border-radius:6rpx;background:#eef5f1;color:#28624a;font-size:20rpx}.available{flex:0 0 auto;color:#28624a;font-weight:800}.quantity-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:8rpx;color:#4d5b54;font-size:21rpx}.detail-hint{color:#28624a;font-size:22rpx;text-align:right}.primary,.secondary{min-height:72rpx;margin:0;border-radius:8rpx}.primary{background:#2b2118;color:#fff}.primary[disabled]{background:#aaa;color:#fff}.secondary{background:#fff;border:1rpx solid #ddd}.compact-action{width:auto;min-height:64rpx;padding:0 18rpx;font-size:23rpx}.loading{padding:24rpx 0;text-align:center}.empty-state{padding:36rpx 18rpx;text-align:center}.empty-title{display:block;margin-bottom:10rpx;font-weight:800}.error{display:block;padding:18rpx;color:#b42318}
</style>
