<template>
  <div class="page">
    <section class="panel head">
      <div>
        <h2>费用管理</h2>
        <p>{{ month }} · 期间费用与主营成本补录</p>
      </div>
      <div class="toolbar">
        <input v-model="month" type="month" @change="load" />
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
    </section>

    <div v-if="error" class="error">{{ error }}</div>
    <div v-if="ok" class="ok">已保存</div>

    <section class="panel">
      <div class="section-title">新增费用</div>
      <div class="form-grid">
        <label>
          <span>日期</span>
          <input v-model="form.date" type="date" />
        </label>
        <label>
          <span>类别</span>
          <input v-model.trim="form.category" placeholder="房租/物流/人工" />
        </label>
        <label>
          <span>金额</span>
          <input v-model="form.amount" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>归集</span>
          <select v-model="form.allocation">
            <option value="period_expense">期间费用</option>
            <option value="main_cost">主营成本</option>
          </select>
        </label>
        <label>
          <span>付款方式</span>
          <input v-model.trim="form.payment" placeholder="微信/银行/现金" />
        </label>
        <label class="note">
          <span>备注</span>
          <input v-model.trim="form.note" placeholder="可选" />
        </label>
        <button type="button" @click="save" :disabled="saving">保存</button>
      </div>
    </section>

    <section class="panel table-wrap">
      <table>
        <thead>
          <tr>
            <th>日期</th>
            <th>类别</th>
            <th>金额</th>
            <th>归集</th>
            <th>付款方式</th>
            <th>备注</th>
            <th>录入人</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td>{{ row.date }}</td>
            <td>{{ row.category }}</td>
            <td>{{ money(row.amount) }}</td>
            <td>{{ allocationLabel(row.allocation) }}</td>
            <td>{{ row.payment }}</td>
            <td>{{ row.note }}</td>
            <td>{{ row.actor }}</td>
          </tr>
          <tr v-if="!rows.length">
            <td colspan="7" class="empty">暂无费用记录</td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { createFinanceExpense, fetchFinanceExpenses } from '../api/finance.js'
import { currentMonth, money } from '../lib/finance.js'

const month = ref(currentMonth())
const rows = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const form = reactive({
  date: `${month.value}-01`,
  category: '',
  amount: '',
  allocation: 'period_expense',
  payment: '',
  note: '',
})

function allocationLabel(value) {
  return value === 'main_cost' ? '主营成本' : '期间费用'
}

function resetForm() {
  form.date = `${month.value}-01`
  form.category = ''
  form.amount = ''
  form.allocation = 'period_expense'
  form.payment = ''
  form.note = ''
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await fetchFinanceExpenses(month.value)
    rows.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    await createFinanceExpense({
      date: form.date,
      month: month.value,
      category: form.category,
      amount: Number(form.amount || 0),
      allocation: form.allocation,
      payment: form.payment,
      note: form.note,
    })
    ok.value = true
    resetForm()
    await load()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

watch(month, () => {
  form.date = `${month.value}-01`
})

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; display: grid; gap: 14px; }
.panel { border: 1px solid #e3e3e3; border-radius: 8px; background: #fff; padding: 14px; }
.head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
h2 { margin: 0 0 4px; font-size: 20px; }
p { margin: 0; color: #666; }
.toolbar { display: flex; align-items: center; gap: 10px; }
.section-title { font-weight: 800; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: 150px 1fr 140px 140px 150px 1fr 86px; gap: 10px; align-items: end; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select { width: 100%; height: 38px; border: 1px solid #cfcfcf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; background: #1f1f1f; color: #fff; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { opacity: .55; cursor: not-allowed; }
.secondary { background: #fff; color: #1f1f1f; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 920px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee; padding: 9px 8px; text-align: left; font-size: 14px; }
th { background: #fbfbfb; }
.empty { text-align: center; color: #666; }
.error, .ok { border-radius: 6px; padding: 10px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 1100px) {
  .form-grid { grid-template-columns: 1fr 1fr; }
  .note { grid-column: span 2; }
}
@media (max-width: 900px) {
  .page { padding: 12px; }
  .head, .toolbar, .form-grid { display: grid; grid-template-columns: 1fr; }
  .note { grid-column: auto; }
}
</style>
