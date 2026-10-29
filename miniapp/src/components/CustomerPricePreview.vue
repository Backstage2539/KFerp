<script setup lang="ts">
import { ref, watch } from 'vue'
import { fetchOrderPriceTablePreview, type OrderPriceTablePreview } from '../api/customerPortal'
import { useSessionStore } from '../stores/session'
import { tierQuantityLabel } from '../utils/customerOrderEntry'
const props = defineProps<{ usageCode: 'product_order' | 'direct_ship' }>()
const session = useSessionStore()
const preview = ref<OrderPriceTablePreview | null>(null)
const loading = ref(false), error = ref('')
let request = 0
async function load() {
 const version = ++request; loading.value = true; error.value = ''; preview.value = null
 try { const result = await fetchOrderPriceTablePreview(session.token, props.usageCode); if(version === request) preview.value = result }
 catch(e) { if(version === request) error.value = e instanceof Error ? e.message : '价格表加载失败，请重试' }
 finally { if(version === request) loading.value = false }
}
watch(() => [props.usageCode, session.currentCustomerID, session.token], () => { if(session.token && session.currentCustomerID) void load() }, { immediate: true })
function money(value?: number) { return Number(value || 0).toFixed(2) }
function priceTableSpecLabel(row: { spec_name?: string }) { const name = String(row.spec_name || '').trim(); return name && name !== '默认规格' ? name : '规格待维护' }
</script>
<template>
 <view>
  <view v-for="table in preview?.price_tables || []" :key="table.id" class="table-card"><text>{{ table.table_name || table.list_type || '当前价格表' }}</text><text>{{ table.version_no ? `版本 ${table.version_no}` : '' }}</text></view>
  <text v-if="loading" class="state">价格表加载中…</text>
  <view v-else-if="error" class="state"><text>{{ error }}</text><button @tap="load">重新加载</button></view>
  <text v-else-if="preview && !preview.price_tables?.length" class="state">未配置可用的已发布价格表，请联系管理员。</text>
  <scroll-view v-else-if="preview?.rows?.length" scroll-x class="table-scroll"><view class="price-grid"><view class="table-row header"><text>商品</text><text>规格</text><text>数量档位</text><text>单价</text></view><view v-for="(row,index) in preview.rows" :key="`${row.publication_id}:${row.product_id}:${row.bom_spec_id || 0}:${index}`" class="table-row"><text>{{ row.product_name }}</text><text>{{ priceTableSpecLabel(row) }}</text><text>{{ tierQuantityLabel(row) }}</text><text class="price">¥{{ money(row.unit_price) }}/{{ row.sales_unit || '件' }}</text></view></view></scroll-view>
  <text v-else-if="preview" class="state">当前已发布价格表暂无商品价格。</text>
 </view>
</template>
<style scoped>
.table-card{display:flex;justify-content:space-between;gap:16rpx;margin-bottom:18rpx;padding:22rpx;border:1rpx solid #dbe5df;border-radius:16rpx;background:#fff}.table-scroll{width:100%;border:1rpx solid #dbe5df;border-radius:16rpx;background:#fff}.price-grid{min-width:940rpx}.table-row{display:grid;grid-template-columns:260rpx 220rpx 220rpx 240rpx;min-height:84rpx;border-bottom:1rpx solid #e8eeea}.table-row text{display:flex;align-items:center;padding:14rpx 18rpx;border-right:1rpx solid #eef2ef;font-size:24rpx}.header{min-height:72rpx;background:#eaf3ee;font-weight:800}.price{color:#17603f;font-weight:800}.state{display:block;padding:40rpx 20rpx;text-align:center;color:#728078;font-size:24rpx}
</style>
