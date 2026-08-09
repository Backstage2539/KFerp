<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { EmployeeOrderProductFamily } from '../api/customerPortal'
import {
  employeeOrderProductCategories,
  employeeOrderProductCategory,
  employeeOrderProductFamilyKey,
  filterEmployeeOrderProductFamilies,
  productSpecLabel,
  type EmployeeOrderProductCategory,
} from '../utils/employeeOrder'

const props = withDefaults(defineProps<{
  visible: boolean
  families: EmployeeOrderProductFamily[]
  customerId: number
  loading?: boolean
  customerFacingNames?: boolean
}>(), { loading: false, customerFacingNames: false })

const emit = defineEmits<{
  close: []
  select: [family: EmployeeOrderProductFamily]
}>()

const query = ref('')
const category = ref<EmployeeOrderProductCategory>('all')
const filteredFamilies = computed(() => filterEmployeeOrderProductFamilies(
  props.families,
  props.customerId,
  query.value,
  category.value,
))

watch(() => props.visible, (visible) => {
  if (!visible) return
  query.value = ''
  category.value = 'all'
})

function categoryLabel(family: EmployeeOrderProductFamily): string {
  const key = employeeOrderProductCategory(family)
  return employeeOrderProductCategories.find((row) => row.key === key)?.label || '未分类'
}

function displayName(family: EmployeeOrderProductFamily): string {
  if (!props.customerFacingNames) return String(family.name || '').trim()
  return String(family.customer_product_display_name || family.alias_name || family.name || '').trim()
}
</script>

<template>
  <view v-if="visible" class="overlay" @tap.self="emit('close')">
    <view class="select-sheet product-sheet" @tap.stop>
      <view class="sheet-head">
        <text class="sheet-title">选择商品</text>
        <text class="sheet-close" @tap="emit('close')">关闭</text>
      </view>
      <input
        v-model="query"
        class="search-input"
        focus
        confirm-type="search"
        placeholder="商品 / 别名 / 拼音 / 编码 / 规格"
      />
      <scroll-view scroll-x class="category-scroll" :show-scrollbar="false">
        <view class="category-row">
          <button
            v-for="item in employeeOrderProductCategories"
            :key="item.key"
            class="category-button"
            :class="{ active: category === item.key }"
            @tap="category = item.key"
          >
            {{ item.label }}
          </button>
        </view>
      </scroll-view>
      <scroll-view scroll-y class="option-list product-list">
        <view
          v-for="family in filteredFamilies"
          :key="employeeOrderProductFamilyKey(family)"
          class="option-row product-row"
          @tap="emit('select', family)"
        >
          <text class="option-name">{{ displayName(family) }}</text>
          <text class="option-meta">{{ categoryLabel(family) }} · {{ family.specs.length }} 个规格</text>
          <text class="option-specs">{{ family.specs.map(productSpecLabel).join('、') || '暂无规格' }}</text>
        </view>
        <text v-if="loading" class="empty-state">商品目录加载中...</text>
        <text v-else-if="!filteredFamilies.length" class="empty-state">没有找到符合条件的商品</text>
      </scroll-view>
      <text class="result-hint">每个商品只显示一条，最多显示 30 条</text>
    </view>
  </view>
</template>

<style scoped>
.overlay{position:fixed;inset:0;z-index:1100;display:flex;align-items:flex-end;background:rgba(16,28,22,.48)}
.select-sheet{width:100%;max-height:86vh;padding:28rpx 28rpx calc(24rpx + env(safe-area-inset-bottom));border-radius:24rpx 24rpx 0 0;box-sizing:border-box;background:#fff}
.sheet-head{display:flex;align-items:center;justify-content:space-between;margin-bottom:20rpx}
.sheet-title{color:#1e362a;font-size:32rpx;font-weight:800}
.sheet-close{padding:12rpx;color:#718078;font-size:26rpx}
.search-input{width:100%;min-height:78rpx;padding:0 22rpx;border:2rpx solid #b8ccc0;border-radius:12rpx;box-sizing:border-box;background:#f9fbfa}
.category-scroll{width:100%;margin:18rpx 0 8rpx;white-space:nowrap}
.category-row{display:inline-flex;gap:12rpx;padding-right:20rpx}
.category-button{display:inline-flex;align-items:center;margin:0;padding:0 22rpx;min-height:62rpx;line-height:62rpx;border:1rpx solid #cad6cf;border-radius:32rpx;background:#fff;color:#516159;font-size:24rpx}
.category-button::after{border:0}
.category-button.active{border-color:#28624a;background:#eaf3ee;color:#28624a;font-weight:750}
.option-list{height:48vh;margin-top:14rpx}
.option-row{display:flex;flex-direction:column;gap:8rpx;padding:22rpx 10rpx;border-bottom:1rpx solid #edf1ee}
.option-name{color:#172c22;font-size:29rpx;font-weight:700}
.option-meta{color:#7b5a25;font-size:23rpx}
.option-specs{color:#6d7b73;font-size:23rpx;line-height:1.5}
.empty-state{display:block;padding:80rpx 20rpx;color:#7d8982;text-align:center}
.result-hint{display:block;padding-top:14rpx;color:#89958e;font-size:21rpx;text-align:center}
</style>
