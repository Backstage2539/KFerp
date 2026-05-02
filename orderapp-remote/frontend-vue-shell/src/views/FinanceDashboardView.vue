<template>
  <div class="page">
    <section class="panel head">
      <div>
        <h2>财务首页</h2>
        <p>{{ month }} · {{ companyTypeLabel(settings.company_type) }} · {{ taxpayerTypeLabel(settings.taxpayer_type) }}</p>
      </div>
      <div class="toolbar">
        <input v-model="month" type="month" @change="load" />
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
    </section>

    <div v-if="error" class="error">{{ error }}</div>

    <section class="metrics">
      <div v-for="card in cards" :key="card.label" class="metric" :class="card.tone">
        <span>{{ card.label }}</span>
        <strong>{{ card.display }}</strong>
        <em v-if="card.sub">{{ card.sub }}</em>
      </div>
    </section>

    <section class="grid">
      <div class="panel">
        <div class="section-title">月度状态</div>
        <div class="rows">
          <div><span>结账状态</span><strong>{{ financeStatusLabel(report.status) }}</strong></div>
          <div><span>锁账模式</span><strong>{{ closingModeLabel(settings.closing_mode) }}</strong></div>
          <div><span>含税收入</span><strong>{{ money(report.revenue_tax_inclusive) }}</strong></div>
          <div><span>主营成本</span><strong>{{ money(report.main_business_cost) }}</strong></div>
          <div><span>期间费用</span><strong>{{ money(report.period_expenses) }}</strong></div>
        </div>
      </div>
      <div class="panel">
        <div class="section-title">异常提醒</div>
        <div v-if="!exceptions.length" class="empty">暂无异常</div>
        <div v-for="item in exceptions" :key="item.code" class="exception">
          <strong>{{ item.message }}</strong>
          <span v-if="item.count">{{ item.count }} 条</span>
        </div>
      </div>
    </section>

    <section class="panel actions">
      <button type="button" @click="go('financeExpenses')">费用管理</button>
      <button type="button" @click="go('financeClosing')">月度结账</button>
      <button type="button" @click="go('financeReport')">经营报告</button>
      <button type="button" @click="go('financeSettings')">财务设置</button>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { fetchFinanceDashboard } from '../api/finance.js'
import {
  closingModeLabel,
  companyTypeLabel,
  currentMonth,
  financeMetricCards,
  financeStatusLabel,
  money,
  taxpayerTypeLabel,
} from '../lib/finance.js'

const month = ref(currentMonth())
const loading = ref(false)
const error = ref('')
const settings = ref({})
const report = ref({})
const exceptions = ref([])
const cards = computed(() => financeMetricCards(report.value))

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await fetchFinanceDashboard(month.value)
    settings.value = data.settings || {}
    report.value = data.report || {}
    exceptions.value = data.exceptions || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function go(key) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key } }))
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
.toolbar, .actions { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; }
input { height: 38px; border: 1px solid #cfcfcf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; background: #1f1f1f; color: #fff; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { opacity: .55; cursor: not-allowed; }
.secondary { background: #fff; color: #1f1f1f; }
.metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.metric { border: 1px solid #e3e3e3; border-radius: 8px; padding: 14px; min-height: 104px; display: grid; align-content: space-between; background: #fbfbfb; }
.metric span { color: #666; font-size: 13px; }
.metric strong { display: block; font-size: 26px; line-height: 1.2; margin-top: 10px; }
.metric em { color: #666; font-style: normal; font-size: 13px; }
.grid { display: grid; grid-template-columns: minmax(0, 1.25fr) minmax(0, .75fr); gap: 14px; }
.section-title { font-weight: 800; margin-bottom: 10px; }
.rows { display: grid; gap: 8px; }
.rows div { display: flex; justify-content: space-between; gap: 14px; border-bottom: 1px solid #f0f0f0; padding-bottom: 8px; }
.rows span { color: #666; }
.exception { border: 1px solid #ead8a8; border-radius: 8px; background: #fffaf0; padding: 10px; display: flex; justify-content: space-between; gap: 10px; margin-bottom: 8px; }
.empty { color: #666; padding: 10px 0; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; border-radius: 6px; padding: 10px; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .head, .grid { grid-template-columns: 1fr; display: grid; }
  .metrics { grid-template-columns: 1fr 1fr; }
  .metric strong { font-size: 20px; }
}
</style>
