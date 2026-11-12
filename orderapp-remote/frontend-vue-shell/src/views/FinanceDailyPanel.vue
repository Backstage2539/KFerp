<template>
  <section class="daily panel">
    <div class="heading"><div><h3>每日经营走势</h3><p>按订单日期统计销售与对应成本；税费估算按月计算。</p></div><span v-if="daily?.available === false" class="notice">历史月结未保存每日明细</span></div>
    <div v-if="!daily" class="empty">正在加载每日数据…</div>
    <template v-else-if="daily.available !== false">
      <div v-if="daily.has_monthly_adjustments" class="notice">结账后月度调整未计入每日流水，日合计可能与月报不同。</div>
      <div class="legend"><span class="income">● 不含税收入</span><span class="cost">● 成本费用</span><span class="profit">● 税前经营利润</span><span>成本待核算的日期，成本和利润曲线会断开</span></div>
      <div v-if="days.some(day => day.cost_unavailable)" class="notice">所示日期已知销售成本小计：{{ money(knownCost) }}；未核算完整的成本和利润不计为零。</div>
      <div v-if="days.length" class="chart-scroll">
        <svg class="chart" viewBox="0 0 900 240" role="group" aria-label="每日收入、成本费用及税前经营利润曲线">
          <line x1="56" x2="876" :y1="chart.zeroY" :y2="chart.zeroY" stroke="#d6ddd8" />
          <text x="2" y="20">{{ money(chart.max) }}</text><text x="2" y="203">{{ money(chart.min) }}</text>
          <g v-for="field in fields" :key="field.key" :stroke="field.color" fill="none" stroke-width="2.5">
            <polyline v-for="(segment, index) in chart.series[field.key]" :key="index" :points="segment.map(p => `${p.x},${p.y}`).join(' ')" />
            <template v-for="(segment, index) in chart.series[field.key]" :key="`points-${index}`"><circle v-for="point in segment" :key="point.date" :cx="point.x" :cy="point.y" r="3" :fill="field.color" /></template>
          </g>
          <g v-for="(day, index) in days" :key="day.date" class="day-point" tabindex="0" role="button" :aria-pressed="date === day.date" :aria-label="day.date + ' 查看流水'" @click="selectDate(day.date)" @keydown.enter.prevent="selectDate(day.date)" @keydown.space.prevent="selectDate(day.date)">
            <rect :x="chart.positions[index].x - 11" y="8" width="22" height="202" fill="transparent" />
            <text v-if="index === 0 || index === days.length - 1 || index % 5 === 0" :x="chart.positions[index].x" y="225" text-anchor="middle">{{ day.date.slice(5) }}</text>
            <title>{{ day.date }} 收入 {{ display(day.revenue) }}；成本费用 {{ display(day.cost_expenses) }}；利润 {{ display(day.profit) }}</title>
          </g>
        </svg>
      </div>
      <div v-else class="empty">暂无可展示的日期</div>
      <div v-if="selectedDay" class="day-summary" aria-live="polite" :aria-label="`${selectedDay.date} 经营摘要`">
        <strong>{{ selectedDay.date }}</strong>
        <span>不含税收入 <b>{{ display(selectedDay.revenue) }}</b></span>
        <span>已知销售成本 <b>{{ display(selectedDay.known_cost) }}</b></span>
        <span>期间费用 <b>{{ display(selectedDay.period_expenses) }}</b></span>
        <span>毛利 <b>{{ selectedDay.cost_unavailable ? '待核算' : money((Math.round(Number(selectedDay.revenue || 0) * 100) - Math.round(Number(selectedDay.known_cost || 0) * 100)) / 100) }}</b></span>
        <span>税前经营利润 <b>{{ selectedDay.cost_unavailable ? '待核算' : display(selectedDay.profit) }}</b></span>
      </div>
      <h3 class="ledger-title">每日经营流水</h3>
      <div class="filters">
        <label>日期 <input v-model="date" type="date" :min="`${month}-01`" :max="dateMax" @change="filterChanged" /></label>
        <label>类别 <select v-model="section" @change="filterChanged"><option value="">全部</option><option value="revenue">收入</option><option value="main_cost">成本</option><option value="period_expense">费用</option></select></label>
        <label class="check"><input v-model="unavailable" type="checkbox" @change="filterChanged" /> 只看待核算</label>
        <button type="button" class="secondary" @click="clearFilters">清除筛选</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-else-if="loading" class="empty">正在加载流水…</div>
      <div v-else-if="!rows.length" class="empty">暂无符合条件的流水</div>
      <div v-else class="table-scroll"><table><thead><tr><th>日期</th><th>类别</th><th>来源单据</th><th>客户／商品</th><th>数量</th><th>金额</th><th>成本来源／待核算原因</th></tr></thead><tbody>
        <tr v-for="(row, index) in rows" :key="`${row.source_type}-${row.source_id}-${index}`"><td>{{ row.date || '—' }}</td><td>{{ sectionLabel(row.section) }}</td><td><button v-if="ledgerRoute(row.link)" class="link" type="button" @click="openSource(row)">{{ row.name || '查看来源' }}</button><span v-else>{{ row.name || '—' }}</span></td><td>{{ [row.counterparty, row.product_name].filter(Boolean).join(' / ') || '—' }}</td><td>{{ row.quantity == null ? '—' : `${row.quantity} ${row.unit || ''}` }}</td><td>{{ row.cost_unavailable ? '待核算' : display(row.amount) }}</td><td>{{ [row.cost_source, row.pending_reason].filter(Boolean).join('；') || '—' }}</td></tr>
      </tbody></table></div>
      <div class="pagination"><span>共 {{ total }} 条，第 {{ page }} / {{ totalPages }} 页</span><button class="secondary" type="button" :disabled="loading || page <= 1" @click="changePage(page - 1)">上一页</button><button class="secondary" type="button" :disabled="loading || page >= totalPages" @click="changePage(page + 1)">下一页</button></div>
    </template>
  </section>
</template>
<script setup>
import { computed, ref, watch } from 'vue'
import { fetchFinanceDailyLedger } from '../api/finance.js'
import { money } from '../lib/finance.js'
import { buildDailyChart, ledgerRoute, shanghaiToday } from '../lib/finance-daily.js'
const props = defineProps({ month: { type: String, required: true }, daily: { type: Object, default: null }, initialDate: { type: String, default: '' } })
const emit = defineEmits(['return-date-consumed'])
const fields = [{ key: 'revenue', color: '#228357' }, { key: 'cost_expenses', color: '#c87b23' }, { key: 'profit', color: '#466ac8' }]
const days = computed(() => (props.daily?.days || []).filter(day => day.date?.startsWith(`${props.month}-`) && day.date <= shanghaiToday()))
const chart = computed(() => buildDailyChart(days.value))
const selectedDay = computed(() => days.value.find(day => day.date === date.value))
const knownCost = computed(() => days.value.reduce((sum, day) => sum + Math.round(Number(day.known_cost || 0) * 100), 0) / 100)
const dateMax = computed(() => { if (!/^\d{4}-(0[1-9]|1[0-2])$/.test(props.month)) return ''; const [year, month] = props.month.split('-').map(Number); const end = new Date(Date.UTC(year, month, 0)).toISOString().slice(0, 10); return end < shanghaiToday() ? end : shanghaiToday() })
const date = ref(''), section = ref(''), unavailable = ref(false), page = ref(1), total = ref(0), rows = ref([]), loading = ref(false), error = ref('')
const pageSize = 20
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
let requestVersion = 0
function display(value) { return value === null || value === undefined ? '待核算' : money(value) }
function sectionLabel(value) { return ({ revenue: '收入', main_cost: '成本', period_expense: '费用' })[value] || value || '—' }
async function loadLedger() {
  const version = ++requestVersion
  if (!props.month || !props.daily || props.daily.available === false) { rows.value = []; total.value = 0; loading.value = false; return }
  loading.value = true; error.value = ''
  try {
    const data = await fetchFinanceDailyLedger({ month: props.month, date: date.value, section: section.value, unavailable: unavailable.value, page: page.value, pageSize })
    if (version !== requestVersion) return
    rows.value = data.rows || []; total.value = Number(data.total || 0)
    if (data.available === false) { rows.value = []; error.value = '历史月结未保存每日明细' }
  } catch (err) { if (version === requestVersion) { rows.value = []; total.value = 0; error.value = err.message || '加载流水失败' } }
  finally { if (version === requestVersion) loading.value = false }
}
function filterChanged() { page.value = 1; loadLedger() }
function selectDate(value) { date.value = date.value === value ? '' : value; filterChanged() }
function clearFilters() { date.value = ''; section.value = ''; unavailable.value = false; filterChanged() }
function changePage(value) { page.value = value; loadLedger() }
function openSource(row) { const detail = ledgerRoute(row.link, { month: props.month, date: date.value }, row); if (detail) window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail })) }
watch(() => [props.month, props.daily], () => {
  const restored = props.initialDate && props.initialDate.startsWith(`${props.month}-`) && props.initialDate <= dateMax.value && days.value.some(day => day.date === props.initialDate)
  date.value = restored ? props.initialDate : ''
  if (props.initialDate && props.daily) emit('return-date-consumed')
  page.value = 1
  loadLedger()
}, { immediate: true })
</script>
<style scoped>
.daily{border:1px solid #e3e3e3;border-radius:8px;background:#fff;padding:14px;min-width:0}.heading{display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap}h3{margin:0;font-size:16px}p{margin:5px 0;color:#66716a;font-size:13px}.notice{color:#986813}.legend,.filters,.pagination{display:flex;align-items:center;gap:16px;flex-wrap:wrap}.legend{margin:16px 0 8px;color:#68746d;font-size:12px}.income{color:#228357}.cost{color:#c87b23}.profit{color:#466ac8}.chart-scroll,.table-scroll{overflow-x:auto}.chart{display:block;width:100%;min-width:620px;height:auto;max-height:280px}.chart text{fill:#66716a;font-size:11px}.day-point{cursor:pointer}.day-point:focus{outline:2px solid #228357}.day-summary{display:flex;gap:14px;align-items:center;flex-wrap:wrap;background:#f6f9f7;border-radius:6px;padding:12px;font-size:13px}.day-summary span{display:flex;gap:5px;color:#58665d}.day-summary b{color:#17291e}.ledger-title{margin:18px 0 12px}.filters{gap:12px;margin-bottom:12px;font-size:13px}.filters label{display:flex;align-items:center;gap:6px}.filters input:not([type=checkbox]),select{height:34px;border:1px solid #cfcfcf;border-radius:6px;padding:4px 7px;background:white}.check input{width:16px;height:16px}.secondary{border:1px solid #ccc;border-radius:6px;background:#fff;padding:7px 10px;cursor:pointer}.secondary:disabled{opacity:.5;cursor:default}table{width:100%;border-collapse:collapse;min-width:800px;font-size:13px}th,td{text-align:left;padding:10px 8px;border-bottom:1px solid #eee;vertical-align:top}th{color:#66716a;font-weight:600}.link{border:0;padding:0;background:none;color:#23714b;text-decoration:underline;cursor:pointer;font:inherit}.pagination{justify-content:flex-end;margin-top:12px;font-size:13px}.empty{padding:20px 0;color:#777}.error{color:#8a1f1f;background:#fff0f0;padding:10px;border-radius:6px}
</style>
