<template>
  <div class="page">
    <section class="panel head">
      <div>
        <h2>月度经营报告</h2>
        <p>{{ month }} · {{ financeStatusLabel(report.status) }}</p>
      </div>
      <div class="toolbar">
        <input v-model="month" type="month" @change="load" />
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
        <a :href="exportUrls.pdf" target="_blank" rel="noreferrer">PDF</a>
        <a :href="exportUrls.excel" target="_blank" rel="noreferrer">Excel</a>
      </div>
    </section>

    <div v-if="error" class="error">{{ error }}</div>

    <section class="metrics">
      <div v-for="card in cards" :key="card.label" class="metric">
        <span>{{ card.label }}</span>
        <strong>{{ card.display }}</strong>
        <em v-if="card.sub">{{ card.sub }}</em>
      </div>
    </section>

    <section class="panel brief">
      <div class="section-title">老板简报</div>
      <div class="brief-grid">
        <div>
          <span>收入</span>
          <strong>{{ money(report.adjusted_tax_exclusive_revenue ?? report.tax_exclusive_revenue) }}</strong>
        </div>
        <div>
          <span>毛利</span>
          <strong>{{ money(report.adjusted_gross_profit ?? report.gross_profit) }}</strong>
        </div>
        <div>
          <span>净利</span>
          <strong>{{ money(report.adjusted_net_profit ?? report.operating_net_profit) }}</strong>
        </div>
        <div>
          <span>税费</span>
          <strong>{{ money(report.adjusted_tax_total ?? report.tax?.total_tax) }}</strong>
        </div>
      </div>
    </section>

    <section class="grid">
      <div class="panel table-wrap">
        <div class="section-title">利润表</div>
        <table>
          <tbody>
            <tr><th>含税收入</th><td>{{ money(report.revenue_tax_inclusive) }}</td></tr>
            <tr><th>不含税收入</th><td>{{ money(report.tax_exclusive_revenue) }}</td></tr>
            <tr><th>主营成本</th><td>{{ money(report.main_business_cost) }}</td></tr>
            <tr><th>毛利</th><td>{{ money(report.gross_profit) }}</td></tr>
            <tr><th>毛利率</th><td>{{ percent(report.gross_margin) }}</td></tr>
            <tr><th>期间费用</th><td>{{ money(report.period_expenses) }}</td></tr>
            <tr><th>经营净利</th><td>{{ money(report.operating_net_profit) }}</td></tr>
            <tr><th>调整后净利</th><td>{{ money(report.adjusted_net_profit) }}</td></tr>
          </tbody>
        </table>
      </div>
      <div class="panel table-wrap">
        <div class="section-title">税费估算</div>
        <table>
          <tbody>
            <tr v-for="row in taxRows" :key="row[0]">
              <th>{{ row[0] }}</th>
              <td>{{ money(row[1]) }}</td>
            </tr>
          </tbody>
        </table>
        <p class="note">{{ report.tax?.estimate_note }}</p>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { fetchFinanceReport } from '../api/finance.js'
import {
  currentMonth,
  financeMetricCards,
  financeReportExportUrls,
  financeStatusLabel,
  financeTaxRows,
  money,
  percent,
} from '../lib/finance.js'

const month = ref(currentMonth())
const report = ref({})
const loading = ref(false)
const error = ref('')
const cards = computed(() => financeMetricCards(report.value))
const taxRows = computed(() => financeTaxRows(report.value))
const exportUrls = computed(() => financeReportExportUrls(month.value))

async function load() {
  loading.value = true
  error.value = ''
  try {
    report.value = await fetchFinanceReport(month.value)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; display: grid; gap: 14px; }
.panel { border: 1px solid #e3e3e3; border-radius: 8px; background: #fff; padding: 14px; }
.head { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
h2 { margin: 0 0 4px; font-size: 20px; }
p { margin: 0; color: #666; }
.toolbar { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
input { height: 38px; border: 1px solid #cfcfcf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button, a { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; background: #1f1f1f; color: #fff; padding: 8px 12px; font: inherit; cursor: pointer; text-decoration: none; display: inline-flex; align-items: center; }
button:disabled { opacity: .55; cursor: not-allowed; }
.secondary { background: #fff; color: #1f1f1f; }
.metrics, .brief-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.metric, .brief-grid div { border: 1px solid #e3e3e3; border-radius: 8px; padding: 14px; min-height: 96px; background: #fbfbfb; }
.metric span, .brief-grid span { color: #666; font-size: 13px; }
.metric strong, .brief-grid strong { display: block; font-size: 24px; margin-top: 10px; }
.metric em { color: #666; font-style: normal; font-size: 13px; }
.section-title { font-weight: 800; margin-bottom: 10px; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.table-wrap { overflow: auto; }
table { width: 100%; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee; padding: 9px 8px; text-align: left; font-size: 14px; }
th { width: 52%; background: #fbfbfb; }
.note { margin-top: 10px; font-size: 13px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; border-radius: 6px; padding: 10px; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .head, .toolbar, .grid { display: grid; grid-template-columns: 1fr; }
  .metrics, .brief-grid { grid-template-columns: 1fr 1fr; }
}
</style>
