<template>
  <div class="page">
    <section class="panel head">
      <div>
        <h2>票税台账</h2>
        <p>{{ month }} · 发票、税款和未取票事项</p>
      </div>
      <div class="toolbar">
        <input v-model="month" type="month" @change="load" />
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
    </section>

    <div v-if="error" class="error">{{ error }}</div>
    <div v-if="ok" class="ok">已保存</div>

    <section class="panel">
      <div class="section-title">新增台账</div>
      <div class="form-grid">
        <label>
          <span>类型</span>
          <select v-model="form.kind">
            <option value="sales_invoice">销售发票</option>
            <option value="purchase_invoice">采购发票</option>
            <option value="tax_payment">税款缴纳</option>
            <option value="other">其他</option>
          </select>
        </label>
        <label>
          <span>发票号</span>
          <input v-model.trim="form.invoice_no" placeholder="可空" />
        </label>
        <label>
          <span>往来方</span>
          <input v-model.trim="form.counterparty" placeholder="客户/供应商/税局" />
        </label>
        <label>
          <span>价税合计</span>
          <input v-model="form.total_amount" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>税额</span>
          <input v-model="form.tax_amount" type="number" min="0" step="0.01" />
        </label>
        <label>
          <span>状态</span>
          <select v-model="form.status">
            <option value="pending">待确认</option>
            <option value="confirmed">已确认</option>
            <option value="matched">已匹配</option>
          </select>
        </label>
        <label class="note">
          <span>备注</span>
          <input v-model.trim="form.note" placeholder="未取票说明/认证说明/申报差异" />
        </label>
        <button type="button" @click="save" :disabled="saving">保存</button>
      </div>
    </section>

    <section class="panel table-wrap">
      <table>
        <thead>
          <tr>
            <th>类型</th>
            <th>发票号</th>
            <th>往来方</th>
            <th>价税合计</th>
            <th>税额</th>
            <th>状态</th>
            <th>备注</th>
            <th>录入人</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td>{{ kindLabel(row.kind) }}</td>
            <td>{{ row.invoice_no }}</td>
            <td>{{ row.counterparty }}</td>
            <td>{{ money(row.total_amount) }}</td>
            <td>{{ money(row.tax_amount) }}</td>
            <td>{{ statusLabel(row.status) }}</td>
            <td>{{ row.note }}</td>
            <td>{{ row.actor }}</td>
          </tr>
          <tr v-if="!rows.length">
            <td colspan="8" class="empty">暂无票税记录</td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { fetchFinanceTaxLedger, saveFinanceTaxLedgerEntry } from '../api/finance.js'
import { currentMonth, money } from '../lib/finance.js'

const month = ref(currentMonth())
const rows = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const form = reactive({
  kind: 'sales_invoice',
  invoice_no: '',
  counterparty: '',
  total_amount: '',
  tax_amount: '',
  status: 'pending',
  note: '',
})

function kindLabel(value) {
  const labels = {
    sales_invoice: '销售发票',
    purchase_invoice: '采购发票',
    tax_payment: '税款缴纳',
    other: '其他',
  }
  return labels[value] || value || ''
}

function statusLabel(value) {
  const labels = {
    pending: '待确认',
    confirmed: '已确认',
    matched: '已匹配',
  }
  return labels[value] || value || ''
}

function resetForm() {
  form.kind = 'sales_invoice'
  form.invoice_no = ''
  form.counterparty = ''
  form.total_amount = ''
  form.tax_amount = ''
  form.status = 'pending'
  form.note = ''
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await fetchFinanceTaxLedger(month.value)
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
    await saveFinanceTaxLedgerEntry({
      month: month.value,
      kind: form.kind,
      invoice_no: form.invoice_no,
      counterparty: form.counterparty,
      total_amount: Number(form.total_amount || 0),
      tax_amount: Number(form.tax_amount || 0),
      status: form.status,
      note: form.note,
    })
    resetForm()
    ok.value = true
    await load()
  } catch (err) {
    error.value = err.message || '保存失败'
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
.head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
h2 { margin: 0 0 4px; font-size: 20px; }
p { margin: 0; color: #666; }
.toolbar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.section-title { font-weight: 800; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 10px; align-items: end; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select { width: 100%; height: 38px; border: 1px solid #cfcfcf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; background: #1f1f1f; color: #fff; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { opacity: .55; cursor: not-allowed; }
.secondary { background: #fff; color: #1f1f1f; }
.note { min-width: min(100%, 260px); }
.table-wrap { overflow: auto; }
table { width: 100%; border-collapse: collapse; min-width: 880px; }
th, td { border-bottom: 1px solid #eee; padding: 9px 8px; text-align: left; font-size: 14px; white-space: nowrap; }
th { background: #fbfbfb; }
.empty { color: #666; text-align: center; padding: 18px; }
.error, .ok { border-radius: 6px; padding: 10px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .head, .toolbar { display: grid; grid-template-columns: 1fr; }
}
</style>
