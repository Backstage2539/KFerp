<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { fetchDirectShipCatalog, fetchRecipientAddresses, type CustomerRecipientAddress, type DirectShipCatalog, type FulfillmentMode } from '../api/customerPortal'
import { productSpecLabel } from '../utils/employeeOrder'

const props = defineProps<{ token: string; customerId: number; entryMode?: FulfillmentMode }>()
const catalog = ref<DirectShipCatalog | null>(null)
const addresses = ref<CustomerRecipientAddress[]>([])
const loading = ref(false)
const error = ref('')
let version = 0

async function load() {
  const run = ++version
  const token = props.token, customerId = props.customerId, mode = props.entryMode
  catalog.value = null
  addresses.value = []
  error.value = ''
  if (!token || !customerId) { loading.value = false; return }
  loading.value = true
  const current = () => run === version && token === props.token && customerId === props.customerId && mode === props.entryMode
  try {
    const [items, recipientData] = await Promise.all([fetchDirectShipCatalog(token, mode), fetchRecipientAddresses(token)])
    if (!current()) return
    if (Number(items.current_customer_id) !== customerId || (recipientData.rows || []).some(row => Number(row.customer_id) !== customerId)) throw new Error('客户数据不匹配，请返回后重试')
    catalog.value = items
    addresses.value = recipientData.rows || []
  } catch (reason) {
    if (current()) error.value = reason instanceof Error ? reason.message : '代发信息加载失败'
  } finally {
    if (current()) loading.value = false
  }
}
watch(() => [props.token, props.customerId, props.entryMode], () => { void load() }, { immediate: true })
onBeforeUnmount(() => { version++ })
</script>

<template>
  <view class="preview-panel">
    <text class="title">{{ entryMode === 'on_demand' ? '无库存代发' : '一件代发' }}</text>
    <text class="hint">客户只读预览：可查看商品和收件地址，不能编辑或提交订单。</text>
    <text v-if="loading" class="hint">加载中…</text>
    <view v-else-if="error"><text class="error">{{ error }}</text><button @tap="load">重新加载</button></view>
    <template v-else-if="catalog">
      <text class="section">可用商品</text>
      <text v-if="!catalog.product_families?.length" class="hint">暂无可用商品</text>
      <view v-for="family in catalog.product_families" :key="family.parent_product_id" class="row">
        <text class="name">{{ family.customer_product_display_name || family.name }}</text>
        <text v-for="spec in family.specs" :key="`${spec.product_id}:${spec.bom_spec_id || 0}:${spec.bom_variant_id || 0}`" class="hint">{{ productSpecLabel(spec) }}</text>
      </view>
      <text class="section">收件地址</text>
      <text v-if="!addresses.length" class="hint">暂无收件地址</text>
      <view v-for="address in addresses" :key="address.id" class="row">
        <text class="name">{{ address.recipient_name }} {{ address.phone }}</text>
        <text class="hint">{{ address.company || '' }} {{ address.province || '' }}{{ address.city || '' }}{{ address.district || '' }}{{ address.detail_address }}</text>
      </view>
    </template>
  </view>
</template>

<style scoped>
.preview-panel { padding: 28rpx; background: #fff; border-radius: 16rpx; display: flex; flex-direction: column; gap: 18rpx; }
.title, .section, .name { font-weight: 600; }
.title { font-size: 34rpx; }
.section { margin-top: 18rpx; font-size: 30rpx; }
.hint { color: #6d7b72; font-size: 26rpx; }
.error { color: #ad3333; }
.row { display: flex; flex-direction: column; gap: 8rpx; padding: 18rpx 0; border-bottom: 1px solid #eee; }
</style>
