<template>
  <div class="page">
    <section class="panel head">
      <div>
        <h2>月度结账</h2>
        <p>{{ month }} · {{ financeStatusLabel(report.status) }}</p>
      </div>
      <div class="toolbar">
        <input v-model="month" type="month" @change="load" />
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
        <button type="button" @click="closeMonth" :disabled="saving || report.status === 'closed' || report.status === 'adjusted'">结账</button>
      </div>
    </section>

    <div v-if="error" class="error">{{ error }}</div>
    <div v-if="ok" class="ok">{{ ok }}</div>

    <section class="metrics">
      <div v-for="card in cards" :key="card.label" class="metric">
        <span>{{ card.label }}</span>
        <strong>{{ card.display }}</strong>
      </div>
    </section>

    <section class="grid">
      <div class="panel">
        <div class="section-title">结账快照</div>
        <div class="rows">
          <div><span>含税收入</span><strong>{{ money(report.revenue_tax_inclusive) }}</strong></div>
          <div><span>不含税收入</span><strong>{{ money(report.tax_exclusive_revenue) }}</strong></div>
          <div><span>主营成本</span><strong>{{ money(report.main_business_cost) }}</strong></div>
          <div><span>期间费用</span><strong>{{ money(report.period_expenses) }}</strong></div>
          <div><span>附加税</span><strong>{{ money(report.tax?.surtax) }}</strong></div>
          <div><span>企业所得税</span><strong>{{ money(report.tax?.cit_payable) }}</strong></div>
        </div>
      </div>

      <div class="panel">
        <div class="section-title">结账后调整</div>
        <div class="adjust-form">
          <label>
            <span>类型</span>
            <select v-model="adjustment.type">
              <option value="revenue">收入</option>
              <option value="main_cost">主营成本</option>
              <option value="expense">期间费用</option>
              <option value="tax">税费</option>
              <option value="other">其他</option>
            </select>
          </label>
          <label>
            <span>金额</span>
            <input v-model="adjustment.amount" type="number" step="0.01" />
          </label>
          <label class="wide">
            <span>原因</span>
            <input v-model.trim="adjustment.reason" placeholder="补录/更正/税费差异" />
          </label>
          <label class="wide">
            <span>备注</span>
            <input v-model.trim="adjustment.note" placeholder="可选" />
          </label>
          <button type="button" @click="createAdjustment" :disabled="saving || report.status === 'draft'">新增调整</button>
        </div>
        <div v-if="!adjustments.length" class="empty">暂无调整</div>
        <div v-for="item in adjustments" :key="item.type" class="adjust-row">
          <span>{{ adjustmentTypeLabel(item.type) }}</span>
          <strong>{{ money(item.amount) }}</strong>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { closeFinanceMonth, createFinanceAdjustment, fetchFinanceReport } from '../api/finance.js'
import { currentMonth, financeMetricCards, financeStatusLabel, money } from '../lib/finance.js'

const month = ref(currentMonth())
const report = ref({})
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref('')
const adjustment = reactive({ type: 'expense', amount: '', reason: '', note: '' })
const cards = computed(() => financeMetricCards(report.value))
const adjustments = computed(() => Object.entries(report.value.applied_adjustments || {}).map(([type, amount]) => ({ type, amount })))

function adjustmentTypeLabel(type) {
  const labels = {
    revenue: '收入',
    main_cost: '主营成本',
    expense: '期间费用',
    tax: '税费',
    other: '其他',
  }
  return labels[type] || type
}

function resetAdjustment() {
  adjustment.type = 'expense'
  adjustment.amount = ''
  adjustment.reason = ''
  adjustment.note = ''
}

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

async function closeMonth() {
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    report.value = await closeFinanceMonth(month.value)
    ok.value = '已结账'
  } catch (err) {
    error.value = err.message || '结账失败'
  } finally {
    saving.value = false
  }
}

async function createAdjustmentRow() {
  await createFinanceAdjustment({
    month: month.value,
    type: adjustment.type,
    amount: Number(adjustment.amount || 0),
    reason: adjustment.reason,
    note: adjustment.note,
  })
}

async function createAdjustment() {
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    await createAdjustmentRow()
    resetAdjustment()
    ok.value = '已新增调整'
    await load()
  } catch (err) {
    error.value = err.message || '调整失败'
  } finally {
    saving.value = false
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
input, select { width: 100%; height: 38px; border: 1px solid #cfcfcf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; background: #1f1f1f; color: #fff; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { opacity: .55; cursor: not-allowed; }
.secondary { background: #fff; color: #1f1f1f; }
.metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.metric { border: 1px solid #e3e3e3; border-radius: 8px; padding: 14px; min-height: 96px; background: #fbfbfb; }
.metric span { color: #666; font-size: 13px; }
.metric strong { display: block; font-size: 24px; margin-top: 10px; }
.grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(360px, .8fr); gap: 14px; }
.section-title { font-weight: 800; margin-bottom: 10px; }
.rows { display: grid; gap: 8px; }
.rows div, .adjust-row { display: flex; justify-content: space-between; gap: 14px; border-bottom: 1px solid #f0f0f0; padding-bottom: 8px; }
.rows span, label span { color: #666; }
label span { display: block; font-size: 12px; margin-bottom: 5px; }
.adjust-form { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; align-items: end; margin-bottom: 12px; }
.wide { grid-column: span 2; }
.empty { color: #666; padding: 10px 0; }
.error, .ok { border-radius: 6px; padding: 10px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .head, .toolbar, .grid, .adjust-form { display: grid; grid-template-columns: 1fr; }
  .metrics { grid-template-columns: 1fr 1fr; }
  .wide { grid-column: auto; }
}
</style>
