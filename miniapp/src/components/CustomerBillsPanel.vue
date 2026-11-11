<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import { fetchCustomerIssuedBill, fetchCustomerIssuedBills, customerIssuedBillDocumentPath, type CustomerIssuedBill, type CustomerIssuedBills } from '../api/customerPortal'
import { openMiniappFileOutput } from '../utils/fileOutput'

const props = defineProps<{ token: string; initialStatus?: string }>()
type Period = 'all' | 'month' | 'week' | 'custom'
const now = new Date()
function dateText(date: Date): string { return `${date.getFullYear()}-${String(date.getMonth()+1).padStart(2,'0')}-${String(date.getDate()).padStart(2,'0')}` }
const today = dateText(now)
const period = ref<Period>('all')
const anchor = ref(today)
const from = ref(today), to = ref(today)
const status = ref(props.initialStatus === 'unpaid' ? 'unpaid' : '')
const page = ref(1), loading = ref(false), error = ref(''), expanded = ref<number | null>(null)
const details = ref<Record<number, CustomerIssuedBill>>({})
const data = ref<CustomerIssuedBills>({ rows: [], total: 0, page: 1, limit: 20, summary: { count: 0, unpaid_count: 0, total_amount_cents: 0, received_cents: 0, due_amount_cents: 0, adjustment_cents: 0 } })
let generation = 0
const range = computed(() => {
 if (period.value === 'all') return { date_from: '', date_to: '' }
 if (period.value === 'custom') return { date_from: from.value, date_to: to.value }
 const [year, month, day] = anchor.value.split('-').map(Number)
 if (period.value === 'month') return { date_from: dateText(new Date(year, month-1, 1)), date_to: dateText(new Date(year, month, 0)) }
 const date = new Date(year,month-1,day), offset = (date.getDay()+6)%7
 const start = new Date(year,month-1,day-offset), end = new Date(year,month-1,day-offset+6)
 return { date_from: dateText(start), date_to: dateText(end) }
})
const pages = computed(() => Math.ceil(data.value.total / (data.value.limit || 20)))
function money(cents?: number) { return (Number(cents || 0)/100).toFixed(2) }
async function load() {
 const version = ++generation
 if (range.value.date_from && range.value.date_to && range.value.date_from > range.value.date_to) { error.value = '开始日期不能晚于结束日期'; return }
 loading.value = true; error.value = ''
 try { const result = await fetchCustomerIssuedBills(props.token, { ...range.value, status: status.value, page: page.value, limit: 20 }); if (version === generation) { data.value = result; expanded.value = null; details.value = {} } }
 catch (e) { if (version === generation) { data.value.rows = []; error.value = e instanceof Error ? e.message : '账单加载失败' } }
 finally { if (version === generation) loading.value = false }
}
function setPeriod(value: string) { period.value=value as Period; page.value=1; void load() }
function setStatus(value: string) { status.value=value; page.value=1; void load() }
function setDate(field: 'anchor' | 'from' | 'to', event: {detail?: {value?: string}}) { let value=String(event.detail?.value || ''); if(field==='anchor' && /^\d{4}-\d{2}$/.test(value)) value += '-01'; if(!/^\d{4}-\d{2}-\d{2}$/.test(value)) return; if(field==='anchor') anchor.value=value; else if(field==='from') from.value=value; else to.value=value; page.value=1; void load() }
async function toggle(bill: CustomerIssuedBill) {
 if (expanded.value === bill.batch_id) { expanded.value = null; return }
 expanded.value = bill.batch_id
 if (details.value[bill.batch_id]) return
 try { const detail=await fetchCustomerIssuedBill(props.token,bill.batch_id); details.value = {...details.value,[bill.batch_id]:detail} }
 catch(e) { error.value=e instanceof Error ? e.message : '账单详情加载失败'; expanded.value=null }
}
async function download(bill: CustomerIssuedBill, kind: 'pdf' | 'xlsx') {
 try { await openMiniappFileOutput({path:customerIssuedBillDocumentPath(bill.batch_id,kind),token:props.token,kind,loadingTitle:'生成账单'}) }
 catch(e) { error.value=e instanceof Error ? e.message : '账单下载失败' }
}
function changePage(next: number) { if(next>=1 && next<=pages.value) {page.value=next; void load()} }
onMounted(() => { void load() })
onBeforeUnmount(() => { generation++ })
</script>

<template>
 <view class="workspace">
  <view class="panel">
   <view class="head"><text class="title">费用中心</text><button class="small" @tap="load">刷新</button></view>
   <view class="chips"><button v-for="item in [{key:'all',label:'所有'},{key:'month',label:'按月'},{key:'week',label:'按周'},{key:'custom',label:'起止日期'}]" :key="item.key" :class="{active:period===item.key}" @tap="setPeriod(item.key)">{{item.label}}</button></view>
   <view v-if="period==='month' || period==='week'" class="dates"><picker mode="date" :fields="period==='month' ? 'month' : 'day'" :value="anchor" @change="setDate('anchor',$event)"><view class="date">{{period==='month' ? anchor.slice(0,7) : anchor}}</view></picker><text>{{range.date_from}} 至 {{range.date_to}}</text></view>
   <view v-if="period==='custom'" class="dates"><picker mode="date" :value="from" @change="setDate('from',$event)"><view class="date">{{from}}</view></picker><text>至</text><picker mode="date" :value="to" @change="setDate('to',$event)"><view class="date">{{to}}</view></picker></view>
   <text class="hint">按出账日期筛选；账单金额和付款状态以 ERP 为准。</text>
   <view class="chips"><button :class="{active:status===''}" @tap="setStatus('')">全部</button><button :class="{active:status==='unpaid'}" @tap="setStatus('unpaid')">未付</button><button :class="{active:status==='paid'}" @tap="setStatus('paid')">已付</button></view>
   <view class="summary"><view><text>出账金额</text><strong>¥{{money(data.summary.total_amount_cents)}}</strong></view><view><text>账后已付</text><strong>¥{{money(data.summary.received_cents)}}</strong></view><view><text>当前未付</text><strong>¥{{money(data.summary.due_amount_cents)}}</strong></view><view><text>调整</text><strong>¥{{money(data.summary.adjustment_cents)}}</strong></view></view>
  </view>
  <view class="panel"><text class="title">已出账账单（{{data.total}}）</text>
   <view v-for="bill in data.rows" :key="bill.batch_id" class="bill">
    <view class="head" @tap="toggle(bill)"><text class="name">{{bill.settlement_no}}</text><text class="amount">¥{{money(bill.total_amount_cents)}}</text></view>
    <text class="hint" @tap="toggle(bill)">出账 {{bill.issued_at || '未记录'}} · {{bill.current_due_amount_cents > 0 ? '未付 ¥'+money(bill.current_due_amount_cents) : '已付'}} · {{expanded===bill.batch_id ? '收起' : '展开'}}</text>
    <view v-if="expanded===bill.batch_id" class="detail">
     <text>账期 {{bill.period_from}} 至 {{bill.period_to}}</text><text>账后已付 ¥{{money(bill.current_received_cents)}} · 当前未付 ¥{{money(bill.current_due_amount_cents)}} · 调整 ¥{{money(bill.adjustment_cents)}}</text>
     <view v-for="(line,index) in details[bill.batch_id]?.lines || []" :key="line.id || index" class="line"><text>{{line.description || line.charge_type}} · ¥{{money(line.amount_cents)}}</text><text v-if="line.detail?.order_no">订单：{{line.detail.order_no}}</text><text v-if="line.detail?.receiver_address">收件地址：{{line.detail.receiver_address}}</text><text v-for="(item,i) in line.detail?.items || []" :key="i">{{item.item_name}} {{item.spec}} · {{item.qty}}{{item.unit}} · 单价 ¥{{item.unit_price}} · 金额 ¥{{item.line_total}}</text><text v-for="(shipment,i) in line.detail?.shipments || []" :key="i">运单：{{shipment.tracking_no || '未记录'}} · 发货：{{shipment.shipped_at || '未记录'}}</text></view>
     <view class="actions"><button @tap="download(bill,'pdf')">下载 PDF</button><button @tap="download(bill,'xlsx')">下载 Excel</button></view>
    </view>
   </view>
   <view v-if="pages>1" class="actions"><button :disabled="page<=1 || loading" @tap="changePage(page-1)">上一页</button><text>第 {{page}} / {{pages}} 页</text><button :disabled="page>=pages || loading" @tap="changePage(page+1)">下一页</button></view>
   <text v-if="loading" class="hint">加载中…</text><text v-else-if="!error && !data.rows.length" class="hint">当前范围暂无账单</text><text v-if="error" class="error">{{error}}</text>
  </view>
 </view>
</template>
<style scoped>
.workspace,.panel,.bill,.detail,.line{display:flex;flex-direction:column;gap:14rpx}.panel{padding:24rpx;margin-bottom:20rpx;border:1rpx solid #e6e0d8;border-radius:12rpx;background:white}.head,.dates,.actions{display:flex;align-items:center;justify-content:space-between;gap:12rpx}.title{font-size:30rpx;font-weight:900}.name,.amount{font-size:27rpx;font-weight:800}.amount{color:#5d3b17}.hint{font-size:23rpx;color:#707070;line-height:1.5}.chips{display:flex;gap:8rpx;flex-wrap:wrap}.chips button,.actions button,.small{margin:0;padding:8rpx 14rpx;font-size:23rpx;background:#fff;border:1rpx solid #ddd}.chips .active{background:#2b2118;color:white}.date{padding:12rpx;border:1rpx solid #ddd}.summary{display:grid;grid-template-columns:repeat(2,1fr);gap:12rpx}.summary view{display:flex;flex-direction:column;gap:8rpx;padding:16rpx;background:#fafafa}.summary strong{font-size:28rpx}.bill{padding:18rpx;border:1rpx solid #eee;border-radius:10rpx}.detail,.line{font-size:24rpx}.line{padding:12rpx 0;border-top:1rpx dashed #ddd}.actions button{flex:1}.error{color:#b42318;font-size:24rpx}
</style>
