<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import {
  buildCustomerAccountDocumentPath,
  fetchCustomerAccount,
  type CustomerAccountData,
  type CustomerAccountFeeLine,
  type CustomerAccountFilters,
  type CustomerOrderFeeGroup,
} from '../api/customerPortal'
import { openMiniappFileOutput } from '../utils/fileOutput'
import { customerFeeRange, validCustomerFeeDates, type CustomerFeePeriod } from '../utils/customerBillDate'
import { customerDestinationURL } from '../utils/customerNavigation'

const props = defineProps<{ token: string }>()
type PickerChangeEvent = { detail?: { value?: string | number } }
type ChargeType = 'payable' | 'product' | 'processing' | 'direct_ship_service' | 'shipping'

const emptyAccount = (): CustomerAccountData => ({
  customer_name: '', date_from: '', date_to: '', as_of: '', rows: [], fees: [], settlements: [], order_fee_groups: [],
  summary: {
    count: 0, goods_cents: 0, shipping_cents: 0, discount_cents: 0, total_cents: 0,
    paid_cents: 0, due_cents: 0, processing_cents: 0, direct_ship_service_cents: 0,
    fee_shipping_cents: 0, adjustment_cents: 0, refund_cents: 0, additional_fee_cents: 0, payable_cents: 0,
  },
  fee_summary: {
    goods_cents: 0, processing_cents: 0, direct_ship_service_cents: 0, shipping_cents: 0,
    other_cents: 0, payable_cents: 0, paid_cents: 0, due_cents: 0, refund_cents: 0,
  },
  total: 0, page: 1, limit: 20, total_pages: 0,
})

const loading = ref(false)
const downloading = ref(false)
const errorMessage = ref('')
const data = ref<CustomerAccountData>(emptyAccount())
const period = ref<CustomerFeePeriod>('current')
const initialRange = customerFeeRange('current')
const customFrom = ref(initialRange.date_from || '')
const customTo = ref(initialRange.date_to || '')
const page = ref(1)
const activeCategory = ref<ChargeType>('payable')
let loadVersion = 0

const filters = computed<CustomerAccountFilters>(() => ({
  view: 'order_fees', ...(period.value === 'custom' ? { date_from: customFrom.value, date_to: customTo.value } : customerFeeRange(period.value)), page: page.value, limit: 20,
  charge_type: activeCategory.value === 'payable' ? '' : activeCategory.value,
}))
const summary = computed(() => data.value.fee_summary || {
  goods_cents: data.value.summary.goods_cents,
  processing_cents: data.value.summary.processing_cents,
  direct_ship_service_cents: data.value.summary.direct_ship_service_cents,
  shipping_cents: data.value.summary.shipping_cents + data.value.summary.fee_shipping_cents,
  other_cents: 0,
  payable_cents: data.value.summary.payable_cents,
  paid_cents: data.value.summary.paid_cents,
  due_cents: data.value.summary.due_cents,
  refund_cents: data.value.summary.refund_cents,
})
const categories = computed<Array<{ key: ChargeType; label: string; amount: () => number }>>(() => [
  { key: 'payable', label: period.value === 'unpaid' ? '未付合计' : '应付', amount: () => summary.value.payable_cents },
  { key: 'product', label: '商品货款', amount: () => summary.value.goods_cents },
  { key: 'processing', label: '加工费', amount: () => summary.value.processing_cents },
  { key: 'direct_ship_service', label: '代发服务费', amount: () => summary.value.direct_ship_service_cents },
  { key: 'shipping', label: '运费', amount: () => summary.value.shipping_cents },
])
const feeTypeLabels: Record<string, string> = {
  product: '商品货款', order_goods: '商品货款', processing: '加工费', roasting: '加工费', labor: '加工费',
  material: '加工费', packaging: '加工费', direct_ship_service: '代发服务费', shipping: '运费',
  storage: '仓储费', adjustment: '调整',
}
const paymentStatusLabels: Record<string, string> = { unpaid: '未付', partial: '部分已付', paid: '已付', reversed: '已退款/冲销', unknown: '以 ERP 为准' }

function money(cents?: number): string { return (Number(cents || 0) / 100).toFixed(2) }
function feeName(fee: CustomerAccountFeeLine): string { return fee.fee_name || feeTypeLabels[fee.fee_type] || fee.fee_type || '费用' }
function groupTitle(group: CustomerOrderFeeGroup): string { return group.unlinked ? '未关联订单费用' : group.order.order_no || '订单费用' }
function openOrder(orderNo: string) {
  if (!orderNo) return
  uni.navigateTo({ url: customerDestinationURL('orders', { q: orderNo }) })
}

async function load() {
  const version = ++loadVersion
  if (period.value === 'custom' && !validCustomerFeeDates(customFrom.value, customTo.value)) {
    loading.value = false; data.value = emptyAccount(); errorMessage.value = '请选择有效的起止日期，开始日期不能晚于结束日期'; return
  }
  loading.value = true
  errorMessage.value = ''
  try { const result = await fetchCustomerAccount(props.token, filters.value); if (version === loadVersion) data.value = result }
  catch (error) { if (version === loadVersion) { data.value = emptyAccount(); errorMessage.value = error instanceof Error ? error.message : '费用明细加载失败' } }
  finally { if (version === loadVersion) loading.value = false }
}

async function setPeriod(next: CustomerFeePeriod) { period.value = next; page.value = 1; data.value = emptyAccount(); await load() }
async function setCustomDate(field: 'from' | 'to', event: PickerChangeEvent) { const value = String(event.detail?.value || ''); if (field === 'from') customFrom.value = value; else customTo.value = value; page.value = 1; data.value = emptyAccount(); await load() }
async function selectCategory(next: ChargeType) { if (activeCategory.value === next) return; activeCategory.value = next; page.value = 1; data.value = emptyAccount(); await load() }

async function download(kind: 'pdf' | 'xlsx') {
  if (downloading.value) return
  if (period.value === 'custom' && !validCustomerFeeDates(customFrom.value, customTo.value)) { errorMessage.value = '请选择有效的起止日期，开始日期不能晚于结束日期'; return }
  downloading.value = true
  errorMessage.value = ''
  try {
    await openMiniappFileOutput({ path: buildCustomerAccountDocumentPath(kind, filters.value), token: props.token, kind, loadingTitle: '生成费用明细' })
  } catch (error) { errorMessage.value = error instanceof Error ? error.message : '费用明细下载失败' }
  finally { downloading.value = false }
}

async function changePage(next: number) {
  if (next < 1 || next > data.value.total_pages || next === page.value) return
  page.value = next
  await load()
}

onMounted(() => { void load() })
onBeforeUnmount(() => { loadVersion++ })
</script>

<template>
  <view class="workspace">
    <view class="panel">
      <view class="head">
        <view class="heading"><text class="title">费用中心</text><text class="hint">{{ data.customer_name || '当前客户' }} · 数据截至 {{ data.as_of || '加载中' }}</text></view>
        <button class="secondary compact" :disabled="loading" @tap="load">刷新</button>
      </view>
      <view class="period-row">
        <button class="chip" :class="{ active: period === 'current' }" @tap="setPeriod('current')">本月</button>
        <button class="chip" :class="{ active: period === 'previous' }" @tap="setPeriod('previous')">上月</button>
        <button class="chip" :class="{ active: period === 'custom' }" @tap="setPeriod('custom')">自定义</button>
        <button class="chip" :class="{ active: period === 'unpaid' }" @tap="setPeriod('unpaid')">所有未付</button>
      </view>
      <view v-if="period === 'custom'" class="period-row"><picker mode="date" :value="customFrom" @change="setCustomDate('from', $event)"><view class="date-field">{{ customFrom || '开始日期' }}</view></picker><text>至</text><picker mode="date" :value="customTo" @change="setCustomDate('to', $event)"><view class="date-field">{{ customTo || '结束日期' }}</view></picker></view>
      <text class="hint">{{ period === 'unpaid' ? `所有未付，截至 ${data.as_of || '加载中'}` : `账期 ${data.date_from || '-'} 至 ${data.date_to || '-'}。商品货款按订单日期，运费及服务费按费用入账日期。` }}</text>
      <view class="summary-grid">
        <button v-for="category in categories" :key="category.key" class="summary category-card" :class="{ selected: activeCategory === category.key }" @tap="selectCategory(category.key)">
          <text>{{ category.label }}</text><strong>¥{{ money(category.amount()) }}</strong>
        </button>
        <template v-if="period !== 'unpaid'"><view class="summary"><text>已付</text><strong>¥{{ money(summary.paid_cents) }}</strong></view><view class="summary"><text>未付</text><strong>¥{{ money(summary.due_cents) }}</strong></view><view class="summary"><text>退款/冲销</text><strong>¥{{ money(summary.refund_cents) }}</strong></view></template>
      </view>
      <view class="actions">
        <button class="secondary" :disabled="downloading" @tap="download('pdf')">{{ downloading ? '正在生成…' : '下载 PDF' }}</button>
        <button class="secondary" :disabled="downloading" @tap="download('xlsx')">{{ downloading ? '正在生成…' : '下载 Excel' }}</button>
      </view>
    </view>

    <view class="panel">
      <view class="head"><text class="title">费用明细</text><text class="hint">{{ period === 'unpaid' ? '按订单展示未付余额' : activeCategory === 'payable' ? '按订单展示本账期全部费用' : `${categories.find(item => item.key === activeCategory)?.label || '费用'}明细` }}</text></view>
      <view v-for="group in data.order_fee_groups || []" :key="group.unlinked ? 'unlinked' : group.order.id" class="order-group">
        <view class="order-heading">
          <text class="name" :class="{ tappable: !group.unlinked }" @tap="openOrder(group.order.order_no)">{{ groupTitle(group) }}</text>
          <text class="amount">{{ period === 'unpaid' ? '未付' : '应付' }} ¥{{ money(group.payable_cents) }}</text>
        </view>
        <text v-if="!group.unlinked" class="hint" @tap="openOrder(group.order.order_no)">{{ group.order.order_date || '跨期费用' }} · {{ group.order.receiver_name || '' }}</text>
        <view v-if="period !== 'unpaid'" class="balance"><text>已付 ¥{{ money(group.paid_cents) }}</text><text>未付 ¥{{ money(group.due_cents) }}</text></view>

        <view v-if="group.fee_lines.some(line => line.category === 'product')" class="goods-details">
          <text class="subhead">商品货款 · {{ period === 'unpaid' ? '未付' : '折后' }} ¥{{ money(group.fee_lines.filter(line => line.category === 'product').reduce((sum, line) => sum + line.amount_cents, 0)) }}</text>
          <text v-for="line in group.fee_lines.filter(item => item.category === 'product' && period === 'unpaid' && item.received_cents)" :key="`paid-${line.source_id}`" class="hint">原应付 ¥{{ money(line.receivable_cents) }} · 已付 ¥{{ money(line.received_cents) }}</text>
          <text v-if="period === 'unpaid'" class="hint">以下为原订单商品明细</text>
          <view v-for="(item, itemIndex) in group.order.items || []" :key="`${item.item_name}-${itemIndex}`" class="item-row">
            <text>{{ item.item_name }} {{ item.spec }} · {{ item.qty }}{{ item.unit }}</text>
            <text class="hint">单价 ¥{{ item.unit_price }} · 折后金额 ¥{{ item.line_total }}</text>
          </view>
          <text v-if="group.order.discount_cents" class="hint">订单优惠 ¥{{ money(group.order.discount_cents) }}</text>
          <text v-if="!(group.order.items || []).length" class="hint">商品明细未记录</text>
          <view v-for="shipment in group.shipments || []" :key="`goods-${shipment.tracking_no}`" class="shipment-details">
            <text>发货时间：{{ shipment.shipped_at || '未记录' }}<text v-if="shipment.source">（{{ shipment.source }}）</text></text>
            <text>运单号：{{ shipment.tracking_no || '未记录' }} · 寄件重量：{{ shipment.settlement_weight_kg ? `${shipment.settlement_weight_kg} kg` : '未记录' }}</text>
          </view>
        </view>

        <view v-for="(line, lineIndex) in group.fee_lines.filter(item => item.category !== 'product')" :key="`${line.id || line.source_type}-${line.source_id}-${lineIndex}`" class="fee-line">
          <view class="line-head"><text>{{ feeName(line) }}</text><text class="amount">¥{{ money(line.amount_cents) }}</text></view>
          <text v-if="period === 'unpaid' && line.received_cents" class="hint">原应付 ¥{{ money(line.receivable_cents) }} · 已付 ¥{{ money(line.received_cents) }}</text>
          <text class="hint">{{ line.occurred_at || '未记录' }} · {{ line.source_type || '来源' }} {{ line.source_id || '' }}</text>
          <view v-if="line.category === 'shipping' || line.category === 'direct_ship_service'" v-for="shipment in (group.shipments || []).filter(item => !line.tracking_no || item.tracking_no === line.tracking_no)" :key="`${shipment.tracking_no}-${lineIndex}`" class="shipment-details">
            <text>订单号：{{ shipment.order_no || group.order.order_no || '未关联' }}</text>
            <text>发货时间：{{ shipment.shipped_at || '未记录' }}<text v-if="shipment.source">（{{ shipment.source }}）</text></text>
            <text>运单号：{{ shipment.tracking_no || '未记录' }}</text>
            <text>寄件重量：{{ shipment.settlement_weight_kg ? `${shipment.settlement_weight_kg} kg` : '未记录' }}</text>
          </view>
          <text v-if="(line.category === 'shipping' || line.category === 'direct_ship_service') && !(group.shipments || []).length" class="hint">发货时间、运单号和寄件重量：未记录</text>
        </view>
      </view>
      <view v-if="data.total_pages > 1" class="pagination">
        <button class="secondary compact" :disabled="page <= 1 || loading" @tap="changePage(page - 1)">上一页</button><text>第 {{ page }} / {{ data.total_pages }} 页 · {{ data.total }} 笔订单</text><button class="secondary compact" :disabled="page >= data.total_pages || loading" @tap="changePage(page + 1)">下一页</button>
      </view>
      <view v-if="!loading && !errorMessage && !(data.order_fee_groups || []).length" class="empty"><text>{{ period === 'unpaid' ? '暂无未付费用' : '当前账期暂无费用' }}</text><text class="hint">可切换范围查看其他费用。</text></view>
      <text v-if="loading" class="hint loading">费用明细加载中...</text>
      <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
    </view>
  </view>
</template>

<style scoped>
.workspace,.panel,.heading,.order-group,.goods-details,.item-row,.fee-line,.shipment-details,.empty{display:flex;flex-direction:column;gap:12rpx}.panel{padding:24rpx;margin-bottom:20rpx;border:1rpx solid #e6e0d8;border-radius:12rpx;background:#fff}.head,.period-row,.actions,.pagination,.order-heading,.balance,.line-head{display:flex;align-items:center;justify-content:space-between;gap:12rpx}.period-row{flex-wrap:wrap}.heading{flex:1;gap:4rpx}.title{font-size:30rpx;font-weight:900}.name{font-size:27rpx;font-weight:800}.amount{font-weight:900;color:#5d3b17}.hint{color:#707070;font-size:23rpx;line-height:1.5}.summary-grid{display:grid;grid-template-columns:repeat(2,1fr);gap:12rpx}.summary{display:flex;flex-direction:column;align-items:flex-start;gap:8rpx;min-height:100rpx;padding:16rpx;border:1rpx solid #eee;border-radius:10rpx;background:#fafafa;color:#222;text-align:left}.summary strong{font-size:30rpx}.category-card{margin:0}.category-card.selected{background:#2b2118;color:#fff;border-color:#2b2118}.chip,.primary,.secondary{min-height:68rpx;margin:0;padding:0 18rpx;border-radius:8rpx;font-size:23rpx}.chip,.secondary{background:#fff;border:1rpx solid #ddd}.chip.active,.primary{background:#2b2118;color:#fff}.date-field{min-height:68rpx;padding:0 18rpx;border:1rpx solid #ddd;border-radius:8rpx;line-height:68rpx}.actions button{flex:1}.order-group{padding:18rpx;border:1rpx solid #eee;border-radius:10rpx}.tappable{color:#28624a;text-decoration:underline}.balance{justify-content:flex-start;color:#555;font-size:23rpx}.subhead{font-weight:800}.item-row,.fee-line{padding:12rpx 0;border-top:1rpx dashed #ddd}.shipment-details{margin-top:8rpx;padding:12rpx;background:#faf8f5;border-radius:8rpx;color:#555;font-size:23rpx}.pagination{justify-content:center}.compact{min-height:58rpx}.empty{align-items:center;padding:28rpx;color:#777}.loading{display:block;padding:12rpx}.error{display:block;padding:18rpx;color:#b42318}
</style>
