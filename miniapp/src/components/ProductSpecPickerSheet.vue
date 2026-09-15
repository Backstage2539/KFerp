<script setup lang="ts">
import { computed } from 'vue'
import type { EmployeeOrderProductFamily, EmployeeOrderProductSpec } from '../api/customerPortal'
import { productSpecLabel } from '../utils/employeeOrder'

const props = defineProps<{
  visible: boolean
  family?: EmployeeOrderProductFamily
  selectedBomSpecId?: number
  selectedProductId?: number
}>()

const emit = defineEmits<{
  close: []
  select: [spec: EmployeeOrderProductSpec]
}>()

const specs = computed(() => props.family?.specs || [])

function selected(spec: EmployeeOrderProductSpec): boolean {
  if (Number(spec.bom_spec_id || 0) > 0) return Number(spec.bom_spec_id) === Number(props.selectedBomSpecId || 0)
  return Number(spec.sku_id || spec.product_id || 0) === Number(props.selectedProductId || 0)
}
</script>

<template>
  <view v-if="visible" class="overlay" @tap.self="emit('close')">
    <view class="sheet" @tap.stop>
      <view class="sheet-head">
        <view><text class="sheet-title">选择规格</text><text class="sheet-subtitle">全部可选规格 {{ specs.length }} 项</text></view>
        <text class="sheet-close" @tap="emit('close')">关闭</text>
      </view>
      <scroll-view scroll-y class="spec-list">
        <view v-for="spec in specs" :key="`${spec.bom_spec_id || 0}:${spec.bom_variant_id || 0}:${spec.sku_id || spec.product_id || 0}`" class="spec-row" :class="{ selected: selected(spec) }" @tap="emit('select', spec)">
          <view class="spec-copy">
            <view class="spec-name-row"><text class="spec-name">{{ productSpecLabel(spec) }}</text><text v-if="spec.is_default || spec.is_default_sku" class="default-badge">默认</text></view>
            <text class="spec-meta">{{ spec.inventory_unit || spec.sales_unit || '单位未配置' }}</text>
          </view>
          <text class="check">{{ selected(spec) ? '✓' : '›' }}</text>
        </view>
        <text v-if="!specs.length" class="empty">该商品暂无可选规格</text>
      </scroll-view>
    </view>
  </view>
</template>

<style scoped>
.overlay{position:fixed;inset:0;z-index:1120;display:flex;align-items:flex-end;background:rgba(16,28,22,.48)}
.sheet{width:100%;max-height:76vh;padding:28rpx 28rpx calc(24rpx + env(safe-area-inset-bottom));border-radius:24rpx 24rpx 0 0;box-sizing:border-box;background:#fff}
.sheet-head,.spec-row,.spec-name-row{display:flex;align-items:center}.sheet-head,.spec-row{justify-content:space-between}.sheet-head{margin-bottom:16rpx}.sheet-title,.sheet-subtitle{display:block}.sheet-title{color:#173126;font-size:32rpx;font-weight:850}.sheet-subtitle{margin-top:6rpx;color:#7a8880;font-size:22rpx}.sheet-close{padding:12rpx;color:#607268;font-size:26rpx}.spec-list{max-height:58vh}.spec-row{min-height:104rpx;padding:18rpx 8rpx;border-bottom:1rpx solid #edf1ee}.spec-row.selected{background:#f0f7f3}.spec-copy{display:flex;flex-direction:column;gap:8rpx}.spec-name-row{gap:12rpx}.spec-name{color:#193027;font-size:29rpx;font-weight:750}.spec-meta{color:#7c8a82;font-size:23rpx}.default-badge{padding:4rpx 12rpx;border-radius:999rpx;background:#dceee4;color:#28624a;font-size:20rpx}.check{color:#28624a;font-size:34rpx}.empty{display:block;padding:80rpx 20rpx;color:#7d8982;text-align:center}
</style>
