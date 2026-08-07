<script setup lang="ts">
import { computed, ref } from 'vue'
import type { EmployeeOrderProductFamily, EmployeeOrderProductSpec } from '../api/customerPortal'
import {
  employeeOrderProductCategories,
  productSpecLabel,
  scopedFulfillmentProductFamilies,
} from '../utils/customerFulfillmentSelector'

const props = defineProps<{
  families: EmployeeOrderProductFamily[]
  customerId: number
}>()

const emit = defineEmits<{
  select: [value: { family: EmployeeOrderProductFamily; spec: EmployeeOrderProductSpec }]
}>()

const query = ref('')
const category = ref<'all' | 'roasted' | 'drip_bag' | 'green_bean' | 'instant_coffee'>('all')
const visibleFamilies = computed(() => scopedFulfillmentProductFamilies(
  props.families,
  props.customerId,
  query.value,
  category.value,
))
</script>

<template>
  <view class="selector">
    <input v-model="query" class="search" placeholder="搜索商品名、拼音、编码或 SKU" />
    <scroll-view class="categories" scroll-x>
      <view class="category-row">
        <button
          v-for="item in employeeOrderProductCategories"
          :key="item.key"
          class="category"
          :class="{ active: category === item.key }"
          @tap="category = item.key">
          {{ item.label }}
        </button>
      </view>
    </scroll-view>
    <view v-if="visibleFamilies.length" class="families">
      <view v-for="family in visibleFamilies" :key="`${family.customer_id || 0}:${family.parent_product_id}:${family.customer_product_alias_id || 0}`" class="family">
        <text class="family-name">{{ family.customer_product_display_name || family.alias_name || family.name }}</text>
        <text class="family-meta">{{ family.customer_item_code || family.code || family.product_code || '' }}</text>
        <view class="specs">
          <button v-for="spec in family.specs" :key="spec.sku_id || spec.product_id" class="spec" @tap="emit('select', { family, spec })">
            {{ productSpecLabel(spec) }}{{ spec.sku_code ? ` · ${spec.sku_code}` : '' }}
          </button>
        </view>
      </view>
    </view>
    <text v-else class="empty">没有符合条件的商品规格</text>
  </view>
</template>

<style scoped>
.selector,.families,.family{display:flex;flex-direction:column;gap:14rpx}.search{min-height:76rpx;padding:0 20rpx;border:1rpx solid #ddd;border-radius:8rpx;background:#fafafa;box-sizing:border-box}.categories{width:100%;white-space:nowrap}.category-row,.specs{display:flex;gap:10rpx}.category,.spec{min-height:62rpx;margin:0;padding:0 18rpx;border:1rpx solid #d8d8d8;border-radius:8rpx;background:#fff;color:#333;font-size:23rpx}.category.active{background:#2b2118;color:#fff}.family{padding:18rpx;border:1rpx solid #e6e0d8;border-radius:8rpx;background:#fff}.family-name{font-size:28rpx;font-weight:800}.family-meta,.empty{color:#777;font-size:23rpx}.specs{flex-wrap:wrap}.spec{background:#fffaf2;color:#5b3b1f}
</style>
