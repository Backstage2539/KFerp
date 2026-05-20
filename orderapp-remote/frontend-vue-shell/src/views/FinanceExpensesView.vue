<template>
  <div class="page">
    <section class="panel head">
      <div>
        <h2>费用管理</h2>
        <p>{{ month }} · {{ customerContextText }}期间费用与主营成本补录</p>
      </div>
      <div class="toolbar">
        <input v-model="month" type="month" @change="load" />
        <select v-model.number="employeeFilter" @change="load">
          <option :value="0">全部员工</option>
          <option v-for="employee in activeEmployees" :key="employee.id" :value="employee.id">
            {{ employee.name }}
          </option>
        </select>
        <button v-if="employeeFilter" class="secondary" type="button" @click="selectEmployeeFilter(0)">清除员工</button>
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
          <input
            v-model.trim="form.category"
            list="finance-expense-category-options"
            placeholder="输入筛选或自定义类别"
          />
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
          <span>关联员工</span>
          <select v-model.number="form.employee_id">
            <option :value="0">不关联</option>
            <option v-for="employee in activeEmployees" :key="employee.id" :value="employee.id">
              {{ employee.name }}
            </option>
          </select>
        </label>
        <label>
          <span>订单ID</span>
          <input v-model="form.order_id" type="number" min="0" placeholder="可选" />
        </label>
        <label>
          <span>客户ID</span>
          <input v-model="form.customer_id" type="number" min="0" placeholder="可选" />
        </label>
        <label>
          <span>商品ID</span>
          <input v-model="form.product_id" type="number" min="0" placeholder="可选" />
        </label>
        <label>
          <span>批次号</span>
          <input v-model.trim="form.batch_no" placeholder="可选" />
        </label>
        <label>
          <span>付款方式</span>
          <input
            v-model.trim="form.payment"
            list="finance-expense-payment-options"
            placeholder="输入筛选或自定义付款方式"
          />
        </label>
        <label class="note">
          <span>维度说明</span>
          <input v-model.trim="form.dimension_note" placeholder="客户样品/批次损耗/订单补贴" />
        </label>
        <label class="note">
          <span>备注</span>
          <input v-model.trim="form.note" placeholder="可选" />
        </label>
        <button type="button" @click="save" :disabled="saving">保存</button>
      </div>
      <datalist id="finance-expense-category-options">
        <option v-for="item in filteredExpenseCategoryOptions" :key="item" :value="item" />
      </datalist>
      <datalist id="finance-expense-payment-options">
        <option v-for="item in filteredExpensePaymentOptions" :key="item" :value="item" />
      </datalist>
    </section>

    <section class="panel table-wrap">
      <table>
        <thead>
          <tr>
            <th>日期</th>
            <th>类别</th>
            <th>金额</th>
            <th>归集</th>
            <th>关联员工</th>
            <th>业务维度</th>
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
            <td>
              <button
                v-if="row.employee_id"
                class="link-button"
                type="button"
                @click="selectEmployeeFilter(row.employee_id)"
              >
                {{ row.employee_name || employeeName(row.employee_id) }}
              </button>
              <span v-else class="muted">未关联</span>
            </td>
            <td>
              <span v-if="row.order_id">订单#{{ row.order_id }}</span>
              <span v-if="row.customer_id"> 客户#{{ row.customer_id }}</span>
              <span v-if="row.product_id"> 商品#{{ row.product_id }}</span>
              <span v-if="row.batch_no"> {{ row.batch_no }}</span>
              <span v-if="row.dimension_note" class="muted">{{ row.dimension_note }}</span>
            </td>
            <td>{{ row.payment }}</td>
            <td>{{ row.note }}</td>
            <td>{{ row.actor }}</td>
          </tr>
          <tr v-if="!rows.length">
            <td colspan="9" class="empty">暂无费用记录</td>
          </tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { apiGet } from '../api/client.js'
import { createFinanceExpense, fetchFinanceExpenses } from '../api/finance.js'
import { expenseCategoryOptions, expensePaymentOptions, filterExpenseOptions } from '../lib/finance-expense-options.js'
import { currentMonth, money, monthFromDate } from '../lib/finance.js'

const props = defineProps({
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})

const month = ref(currentMonth())
const rows = ref([])
const employees = ref([])
const employeeFilter = ref(0)
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const form = reactive({
  date: `${month.value}-01`,
  category: '',
  amount: '',
  allocation: 'period_expense',
  employee_id: 0,
  order_id: '',
  customer_id: '',
  product_id: '',
  batch_no: '',
  dimension_note: '',
  payment: '',
  note: '',
})

const activeEmployees = computed(() => employees.value.filter((employee) => employee.active !== false))
const filteredExpenseCategoryOptions = computed(() => filterExpenseOptions(expenseCategoryOptions, form.category))
const filteredExpensePaymentOptions = computed(() => filterExpenseOptions(expensePaymentOptions, form.payment))
const contextCustomerID = computed(() => Number(props.customerContextId || 0))
const customerContextText = computed(() => {
  if (!contextCustomerID.value) return ''
  return `${props.customerContextLabel || `客户#${contextCustomerID.value}`} · `
})

function allocationLabel(value) {
  return value === 'main_cost' ? '主营成本' : '期间费用'
}

function employeeName(id) {
  const employee = employees.value.find((row) => Number(row.id) === Number(id))
  return employee?.name || `员工#${id}`
}

function resetForm() {
  form.date = `${month.value}-01`
  form.category = ''
  form.amount = ''
  form.allocation = 'period_expense'
  form.employee_id = 0
  form.order_id = ''
  form.customer_id = contextCustomerID.value ? String(contextCustomerID.value) : ''
  form.product_id = ''
  form.batch_no = ''
  form.dimension_note = ''
  form.payment = ''
  form.note = ''
}

async function loadEmployees() {
  const data = await apiGet('/api/finance/employees')
  employees.value = Array.isArray(data) ? data : []
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await fetchFinanceExpenses(month.value, employeeFilter.value, contextCustomerID.value)
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
    const created = await createFinanceExpense({
      date: form.date,
      month: month.value,
      category: form.category,
      amount: Number(form.amount || 0),
      allocation: form.allocation,
      employee_id: form.employee_id,
      order_id: Number(form.order_id || 0),
      customer_id: Number(form.customer_id || 0),
      product_id: Number(form.product_id || 0),
      batch_no: form.batch_no,
      dimension_note: form.dimension_note,
      payment: form.payment,
      note: form.note,
    })
    if (created?.month && created.month !== month.value) {
      month.value = created.month
    }
    ok.value = true
    resetForm()
    await load()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

async function selectEmployeeFilter(employeeId) {
  employeeFilter.value = Number(employeeId || 0)
  await load()
}

watch(month, () => {
  if (monthFromDate(form.date) !== month.value) {
    form.date = `${month.value}-01`
  }
})

function syncMonthFromDate() {
  const postingMonth = monthFromDate(form.date)
  if (postingMonth && postingMonth !== month.value) {
    month.value = postingMonth
  }
}

watch(() => form.date, syncMonthFromDate)

watch(() => props.customerContextId, async () => {
  if (contextCustomerID.value > 0) form.customer_id = String(contextCustomerID.value)
  await load()
})

onMounted(async () => {
  if (contextCustomerID.value > 0) form.customer_id = String(contextCustomerID.value)
  await loadEmployees()
  await load()
})
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
.form-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(130px, 1fr)); gap: 10px; align-items: end; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select { width: 100%; height: 38px; border: 1px solid #cfcfcf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; background: #1f1f1f; color: #fff; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { opacity: .55; cursor: not-allowed; }
.secondary { background: #fff; color: #1f1f1f; }
.link-button { min-height: 0; border: 0; background: transparent; color: #1d4ed8; padding: 0; text-align: left; font: inherit; text-decoration: underline; text-underline-offset: 2px; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 1040px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee; padding: 9px 8px; text-align: left; font-size: 14px; }
th { background: #fbfbfb; }
.empty { text-align: center; color: #666; }
.muted { color: #777; }
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
