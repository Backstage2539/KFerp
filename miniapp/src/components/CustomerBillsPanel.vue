<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  buildCustomerAccountDocumentPath,
  confirmCustomerStatement,
  createCustomerStatementDispute,
  fetchCustomerAccount,
  type CustomerAccountData,
  type CustomerAccountFee,
  type CustomerAccountFilters,
  type CustomerAccountSettlement,
} from '../api/customerPortal'
import { openMiniappFileOutput } from '../utils/fileOutput'

const props = defineProps<{ token: string }>()
type PickerChangeEvent = { detail?: { value?: string | number } }

const emptyAccount = (): CustomerAccountData => ({
  customer_name: '', date_from: '', date_to: '', as_of: '', rows: [], fees: [], settlements: [],
  summary: {
    count: 0, goods_cents: 0, shipping_cents: 0, discount_cents: 0, total_cents: 0,
    paid_cents: 0, due_cents: 0, processing_cents: 0, direct_ship_service_cents: 0,
    fee_shipping_cents: 0, adjustment_cents: 0, refund_cents: 0, additional_fee_cents: 0, payable_cents: 0,
  },
  total: 0, page: 1, limit: 20, total_pages: 0,
})

const loading = ref(false)
const writing = ref(false)
const errorMessage = ref('')
const data = ref<CustomerAccountData>(emptyAccount())
const period = ref<'week' | 'month'>('month')
const anchor = ref(new Date().toISOString().slice(0, 10))
const page = ref(1)
const disputeSettlementID = ref(0)
const disputeFeeItemID = ref(0)
const disputeReason = ref('')

const filters = computed<CustomerAccountFilters>(() => ({ period: period.value, anchor: anchor.value, page: page.value, limit: 20 }))
const feeTypeLabels: Record<string, string> = {
  product: '商品货款', processing: '加工费', roasting: '加工费', labor: '加工费', material: '加工费',
  packaging: '加工费', direct_ship_service: '代发服务费', shipping: '运费', storage: '仓储费', adjustment: '调整',
}
const paymentStatusLabels: Record<string, string> = {
  unpaid: '未付', partial: '部分已付', paid: '已付', reversed: '已退款/冲销', unknown: '以 ERP 为准',
}
const reconciliationLabels: Record<string, string> = {
  pending: '待对账', confirmed: '已确认对账', changed: '账单已变更，需重新确认', disputed: '异议处理中',
}

function money(cents?: number): string { return (Number(cents || 0) / 100).toFixed(2) }
function feeName(fee: CustomerAccountFee): string { return fee.fee_name || feeTypeLabels[fee.fee_type] || fee.fee_type || '费用' }
function settlementStatus(item: CustomerAccountSettlement): string {
  if (item.status === 'paid') return '已付款'
  if (item.status === 'reversed') return '已退款/冲销'
  return reconciliationLabels[item.reconciliation_status] || item.reconciliation_status || '待对账'
}

async function load() {
  loading.value = true
  errorMessage.value = ''
  try { data.value = await fetchCustomerAccount(props.token, filters.value) }
  catch (error) { errorMessage.value = error instanceof Error ? error.message : '客户账单加载失败' }
  finally { loading.value = false }
}

async function setPeriod(next: 'week' | 'month') { period.value = next; page.value = 1; await load() }
async function setAnchor(event: PickerChangeEvent) { anchor.value = String(event.detail?.value || anchor.value); page.value = 1; await load() }

async function download(kind: 'pdf' | 'xlsx') {
  errorMessage.value = ''
  try {
    await openMiniappFileOutput({ path: buildCustomerAccountDocumentPath(kind, filters.value), token: props.token, kind, loadingTitle: '生成账单' })
  } catch (error) { errorMessage.value = error instanceof Error ? error.message : '账单下载失败' }
}

async function confirmStatement(item: CustomerAccountSettlement) {
  if (writing.value || item.reconciliation_status === 'confirmed') return
  const confirmed = await new Promise<boolean>((resolve) => uni.showModal({
    title: '确认对账', content: `确认账单 ${item.settlement_no} 的费用明细无误？此操作不会改变付款状态。`,
    success: (res) => resolve(Boolean(res.confirm)), fail: () => resolve(false),
  }))
  if (!confirmed) return
  writing.value = true
  errorMessage.value = ''
  try {
    await confirmCustomerStatement(props.token, item.id, item.statement_revision)
    uni.showToast({ title: '对账已确认', icon: 'success' })
    await load()
  } catch (error) { errorMessage.value = error instanceof Error ? error.message : '确认失败' }
  finally { writing.value = false }
}

function openDispute(item: CustomerAccountSettlement, feeItemID = 0) {
  disputeSettlementID.value = item.id
  disputeFeeItemID.value = feeItemID
  disputeReason.value = ''
}
function closeDispute() { disputeSettlementID.value = 0; disputeFeeItemID.value = 0; disputeReason.value = '' }

async function submitDispute(item: CustomerAccountSettlement) {
  const reason = disputeReason.value.trim()
  if (!reason) { errorMessage.value = '请填写异议原因'; return }
  writing.value = true
  errorMessage.value = ''
  try {
    await createCustomerStatementDispute(props.token, item.id, {
      statement_revision: item.statement_revision, fee_item_id: disputeFeeItemID.value || undefined, reason,
    })
    closeDispute()
    uni.showToast({ title: '异议已提交', icon: 'success' })
    await load()
  } catch (error) { errorMessage.value = error instanceof Error ? error.message : '异议提交失败' }
  finally { writing.value = false }
}

async function changePage(next: number) {
  if (next < 1 || next > data.value.total_pages || next === page.value) return
  page.value = next
  await load()
}

onMounted(() => { void load() })
</script>

<template>
  <view class="workspace">
    <view class="panel">
      <view class="head">
        <view class="heading"><text class="title">费用中心</text><text class="hint">{{ data.customer_name || '当前客户' }} · 数据截至 {{ data.as_of || '加载中' }}</text></view>
        <button class="secondary compact" :disabled="loading" @tap="load">刷新</button>
      </view>
      <view class="period-row">
        <button class="chip" :class="{ active: period === 'month' }" @tap="setPeriod('month')">按月</button>
        <button class="chip" :class="{ active: period === 'week' }" @tap="setPeriod('week')">按周</button>
        <picker mode="date" :value="anchor" @change="setAnchor"><view class="date-field">{{ anchor }}</view></picker>
      </view>
      <text class="hint">账期 {{ data.date_from || '-' }} 至 {{ data.date_to || '-' }}。付款、退款和调整状态均来自 ERP。</text>
      <view class="summary-grid">
        <view class="summary primary-summary"><text>应付</text><strong>¥{{ money(data.summary.payable_cents) }}</strong></view>
        <view class="summary"><text>已付</text><strong>¥{{ money(data.summary.paid_cents) }}</strong></view>
        <view class="summary"><text>未付</text><strong>¥{{ money(data.summary.due_cents) }}</strong></view>
        <view class="summary"><text>退款/冲销</text><strong>¥{{ money(data.summary.refund_cents) }}</strong></view>
      </view>
      <view class="category-grid">
        <text>商品货款 ¥{{ money(data.summary.goods_cents) }}</text>
        <text>加工费 ¥{{ money(data.summary.processing_cents) }}</text>
        <text>代发服务费 ¥{{ money(data.summary.direct_ship_service_cents) }}</text>
        <text>运费 ¥{{ money(data.summary.shipping_cents + data.summary.fee_shipping_cents) }}</text>
      </view>
      <view class="actions"><button class="secondary" @tap="download('pdf')">下载 PDF</button><button class="secondary" @tap="download('xlsx')">下载 Excel</button></view>
    </view>

    <view class="panel">
      <text class="title">对账单</text>
      <view v-for="item in data.settlements || []" :key="item.id" class="statement">
        <view class="head"><text class="name">{{ item.settlement_no }}</text><text class="amount">¥{{ money(item.total_cents) }}</text></view>
        <text class="hint">{{ item.period_from }} 至 {{ item.period_to }} · {{ settlementStatus(item) }}</text>
        <view v-for="fee in item.fees || []" :key="fee.id" class="fee-row">
          <view><text>{{ feeName(fee) }}</text><text class="hint">来源：{{ fee.order_no || `${fee.source_type || '单据'} ${fee.source_id || ''}` }}</text></view>
          <view class="fee-side"><text>¥{{ money(fee.amount_cents) }}</text><button class="link" @tap="openDispute(item, fee.id)">对此项异议</button></view>
        </view>
        <view v-for="dispute in item.disputes || []" :key="dispute.id" class="dispute-history">
          <text>异议：{{ dispute.reason }}</text><text class="hint">{{ dispute.created_at }} · {{ ['resolved','closed'].includes(dispute.status) ? '已处理' : '处理中' }}</text>
          <text v-if="dispute.reply" class="reply">ERP 回复：{{ dispute.reply }}</text>
        </view>
        <view v-if="disputeSettlementID === item.id" class="dispute-form">
          <text class="hint">{{ disputeFeeItemID ? `针对费用项 #${disputeFeeItemID}` : '针对整张账单' }}</text>
          <textarea v-model="disputeReason" class="textarea" placeholder="请说明有疑问的费用、金额或单据" />
          <view class="actions"><button class="secondary" @tap="closeDispute">取消</button><button class="primary" :disabled="writing" @tap="submitDispute(item)">提交异议</button></view>
        </view>
        <view v-else class="actions">
          <button class="secondary" @tap="openDispute(item)">提出异议</button>
          <button class="primary" :disabled="writing || item.reconciliation_status === 'confirmed' || item.reconciliation_status === 'disputed'" @tap="confirmStatement(item)">{{ item.reconciliation_status === 'confirmed' ? '已确认对账' : '确认对账' }}</button>
        </view>
      </view>
      <view v-if="!loading && !(data.settlements || []).length" class="empty"><text>当前账期暂无已生成的对账单</text><text class="hint">ERP 生成结算单后会显示在这里。</text></view>
    </view>

    <view class="panel">
      <text class="title">费用明细</text>
      <view v-for="fee in data.fees || []" :key="fee.id" class="detail">
        <view class="head"><text class="name">{{ feeName(fee) }}</text><text class="amount">¥{{ money(fee.amount_cents) }}</text></view>
        <text class="hint">{{ fee.occurred_at }} · {{ paymentStatusLabels[fee.payment_status] || fee.payment_status }}</text>
        <text class="hint">关联单据：{{ fee.order_no || fee.settlement_no || `${fee.source_type || '来源'} ${fee.source_id || ''}` }}</text>
        <text v-if="fee.included_in_order" class="included">已计入订单金额，不重复汇总</text>
      </view>
      <text v-if="!loading && !(data.fees || []).length" class="empty">当前账期暂无独立费用</text>
    </view>

    <view class="panel">
      <text class="title">订单货款</text>
      <view v-for="order in data.rows || []" :key="order.id" class="detail">
        <view class="head"><text class="name">{{ order.order_no }}</text><text class="amount">¥{{ money(order.total_cents) }}</text></view>
        <text class="hint">{{ order.order_date }} · {{ order.receiver_name }} · {{ paymentStatusLabels[order.payment_status] || order.payment_status }}</text>
        <text class="hint">商品 ¥{{ money(order.goods_cents) }} · 运费 ¥{{ money(order.shipping_cents) }} · 未付 ¥{{ money(order.due_cents) }}</text>
      </view>
      <view v-if="data.total_pages > 1" class="pagination">
        <button class="secondary compact" :disabled="page <= 1 || loading" @tap="changePage(page - 1)">上一页</button><text>第 {{ page }} / {{ data.total_pages }} 页</text><button class="secondary compact" :disabled="page >= data.total_pages || loading" @tap="changePage(page + 1)">下一页</button>
      </view>
      <view v-if="!loading && !(data.rows || []).length" class="empty"><text>当前账期暂无订单</text><text class="hint">可切换账期查看历史货款。</text></view>
    </view>
    <text v-if="loading" class="hint loading">账单加载中...</text><text v-if="errorMessage" class="error">{{ errorMessage }}</text>
  </view>
</template>

<style scoped>
.workspace,.panel,.heading,.statement,.detail,.fee-row>view,.dispute-form,.dispute-history,.empty{display:flex;flex-direction:column;gap:12rpx}.panel{padding:24rpx;margin-bottom:20rpx;border:1rpx solid #e6e0d8;border-radius:12rpx;background:#fff}.head,.period-row,.actions,.fee-row,.pagination{display:flex;align-items:center;justify-content:space-between;gap:12rpx}.heading{flex:1;gap:4rpx}.title{font-size:30rpx;font-weight:900}.name{font-size:26rpx;font-weight:800}.amount{font-weight:900;color:#5d3b17}.hint{color:#707070;font-size:23rpx;line-height:1.5}.summary-grid,.category-grid{display:grid;grid-template-columns:repeat(2,1fr);gap:12rpx}.summary,.category-grid text{padding:16rpx;border:1rpx solid #eee;border-radius:10rpx;background:#fafafa}.summary{display:flex;flex-direction:column;gap:8rpx}.summary strong{font-size:30rpx}.primary-summary{background:#2b2118;color:#fff}.chip,.primary,.secondary{min-height:68rpx;margin:0;padding:0 18rpx;border-radius:8rpx;font-size:23rpx}.chip,.secondary{background:#fff;border:1rpx solid #ddd}.chip.active,.primary{background:#2b2118;color:#fff}.date-field{min-height:68rpx;padding:0 18rpx;border:1rpx solid #ddd;border-radius:8rpx;line-height:68rpx}.actions button{flex:1}.statement,.detail{padding:18rpx;border:1rpx solid #eee;border-radius:10rpx}.fee-row{padding:12rpx 0;border-top:1rpx dashed #ddd}.fee-side{align-items:flex-end}.link{min-height:auto;margin:0;padding:4rpx 0;color:#7a4d21;background:transparent;border:0;font-size:22rpx}.textarea{min-height:120rpx;padding:14rpx;border:1rpx solid #ddd;border-radius:8rpx;background:#fafafa}.dispute-history{padding:14rpx;background:#fff8ec;border-radius:8rpx}.reply,.included{color:#28624a;font-size:23rpx}.pagination{justify-content:center}.compact{min-height:58rpx}.empty{align-items:center;padding:28rpx;color:#777}.loading{display:block;padding:12rpx}.error{display:block;padding:18rpx;color:#b42318}
</style>
