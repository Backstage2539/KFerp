<template>
  <div class="page customer-fulfillment">
    <section class="panel control-panel">
      <div class="panel-head">
        <div>
          <h2>客户履约账户</h2>
          <p>{{ overview.customer_name || (customerId ? `客户 ${customerId}` : '未选择客户') }}</p>
        </div>
        <button class="secondary" type="button" @click="loadAll" :disabled="loading || !normalizedCustomerId">刷新</button>
      </div>

      <div class="toolbar">
        <label>
          <span>客户 ID</span>
          <input v-model.trim="customerId" inputmode="numeric" placeholder="例如 147" @keyup.enter="loadAll" />
        </label>
        <button class="primary" type="button" @click="loadAll" :disabled="loading || !normalizedCustomerId">载入账户</button>
      </div>

      <div class="import-row">
        <div class="segmented">
          <button
            v-for="option in importTypes"
            :key="option.value"
            type="button"
            :class="{ active: selectedImportType === option.value }"
            @click="selectedImportType = option.value">
            {{ option.label }}
          </button>
        </div>
        <input type="file" accept=".xlsx,.xls" @change="onFileChange" />
        <button class="primary" type="button" @click="parseImport" :disabled="loading || !normalizedCustomerId || !selectedFile">解析导入</button>
        <button class="secondary" type="button" @click="applyLatest" :disabled="loading || !latestParsedBatchId">应用最新批次</button>
      </div>

      <div class="settlement-row">
        <label>
          <span>结算开始</span>
          <input v-model="settlement.period_from" type="date" />
        </label>
        <label>
          <span>结算结束</span>
          <input v-model="settlement.period_to" type="date" />
        </label>
        <button class="secondary" type="button" @click="createSettlement" :disabled="loading || !normalizedCustomerId">生成月结</button>
      </div>

      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <section v-if="summaryCards.length" class="metric-grid">
      <div v-for="card in summaryCards" :key="card.label" class="metric">
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </div>
    </section>

    <section v-if="invalidRows.length || latestInvalidCount" class="panel">
      <div class="panel-head">
        <h3>错误行</h3>
      </div>
      <div v-if="latestInvalidCount && !invalidRows.length" class="muted">最近批次有 {{ latestInvalidCount }} 行错误，请在导入批次中查看源文件。</div>
      <table v-else>
        <thead>
          <tr><th>表</th><th>行号</th><th>类型</th><th>错误</th></tr>
        </thead>
        <tbody>
          <tr v-for="row in invalidRows" :key="`${row.sheet_name}-${row.row_no}-${row.row_type}`">
            <td>{{ row.sheet_name }}</td>
            <td>{{ row.row_no }}</td>
            <td>{{ row.row_type }}</td>
            <td>{{ row.error }}</td>
          </tr>
        </tbody>
      </table>
    </section>

    <section class="grid-2">
      <DataPanel title="导入批次" :rows="imports" empty="暂无导入批次">
        <table>
          <thead>
            <tr><th>ID</th><th>类型</th><th>文件</th><th>状态</th><th>有效/错误</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in imports" :key="row.id">
              <td>{{ row.id }}</td>
              <td>{{ importTypeLabel(row.import_type) }}</td>
              <td>{{ row.source_filename }}</td>
              <td>{{ rowStatusLabel(row.status) }}</td>
              <td>{{ row.summary?.valid_rows || 0 }} / {{ row.summary?.invalid_rows || 0 }}</td>
            </tr>
          </tbody>
        </table>
      </DataPanel>

      <DataPanel title="托管库存" :rows="overview.custody_balances" empty="暂无托管库存">
        <table>
          <thead>
            <tr><th>类型</th><th>名称</th><th>规格</th><th>克重</th><th>件数</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in overview.custody_balances || []" :key="`${row.item_type}-${row.item_name}-${row.spec}`">
              <td>{{ custodyTypeLabel(row.item_type) }}</td>
              <td>{{ row.item_name }}</td>
              <td>{{ row.spec || '-' }}</td>
              <td>{{ row.quantity_g || 0 }}</td>
              <td>{{ row.quantity_units || 0 }}</td>
            </tr>
          </tbody>
        </table>
      </DataPanel>

      <DataPanel title="加工工单" :rows="overview.processing_orders" empty="暂无加工工单">
        <table>
          <thead>
            <tr><th>工单号</th><th>产品</th><th>状态</th><th>投豆</th><th>产量</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in overview.processing_orders || []" :key="row.work_order_no">
              <td>{{ row.work_order_no }}</td>
              <td>{{ row.product_name }}</td>
              <td>{{ row.status || '-' }}</td>
              <td>{{ row.quantity_g || 0 }}</td>
              <td>{{ row.units || 0 }}</td>
            </tr>
          </tbody>
        </table>
      </DataPanel>

      <DataPanel title="代发订单" :rows="overview.direct_ship_orders" empty="暂无代发订单">
        <table>
          <thead>
            <tr><th>订单号</th><th>日期</th><th>收件地址</th><th>状态</th><th>明细</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in overview.direct_ship_orders || []" :key="row.order_no">
              <td>{{ row.order_no }}</td>
              <td>{{ row.order_date || '-' }}</td>
              <td>{{ row.receiver_address }}</td>
              <td>{{ row.status || '-' }}</td>
              <td>{{ row.item_count || 0 }}</td>
            </tr>
          </tbody>
        </table>
      </DataPanel>

      <DataPanel title="费用明细" :rows="overview.fees" empty="暂无费用">
        <table>
          <thead>
            <tr><th>类型</th><th>名称</th><th>金额</th><th>来源</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in overview.fees || []" :key="`${row.fee_type}-${row.fee_name}-${row.amount_cents}`">
              <td>{{ row.fee_type }}</td>
              <td>{{ row.fee_name }}</td>
              <td>{{ moneyFromCents(row.amount_cents) }}</td>
              <td>{{ row.source || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </DataPanel>

      <DataPanel title="结算批次" :rows="overview.settlements" empty="暂无结算">
        <table>
          <thead>
            <tr><th>ID</th><th>期间</th><th>状态</th><th>金额</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in overview.settlements || []" :key="row.batch_id">
              <td>{{ row.batch_id }}</td>
              <td>{{ row.period_from }} 至 {{ row.period_to }}</td>
              <td>{{ row.status }}</td>
              <td>{{ moneyFromCents(row.total_amount_cents) }}</td>
            </tr>
          </tbody>
        </table>
      </DataPanel>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import {
  applyCustomerFulfillmentImport,
  createCustomerFulfillmentSettlement,
  fetchCustomerFulfillmentImports,
  fetchCustomerFulfillmentOverview,
  parseCustomerFulfillmentImport,
} from '../api/customer-fulfillment'
import { importSummaryCards, importTypeOptions, rowStatusLabel } from '../lib/customer-fulfillment'

const DataPanel = {
  props: {
    title: { type: String, required: true },
    rows: { type: Array, default: () => [] },
    empty: { type: String, default: '暂无数据' },
  },
  template: `
    <section class="panel data-panel">
      <div class="panel-head"><h3>{{ title }}</h3></div>
      <div v-if="!rows.length" class="muted empty">{{ empty }}</div>
      <slot v-else />
    </section>
  `,
}

const customerId = ref('')
const selectedImportType = ref('processing_workbook')
const selectedFile = ref(null)
const latestSummary = ref(null)
const latestBatch = ref(null)
const imports = ref([])
const overview = ref({})
const invalidRows = ref([])
const loading = ref(false)
const error = ref('')
const ok = ref('')
const settlement = reactive({
  period_from: '',
  period_to: '',
})

const importTypes = importTypeOptions()
const normalizedCustomerId = computed(() => Number(customerId.value || 0))
const summaryCards = computed(() => importSummaryCards(latestSummary.value || latestBatch.value?.summary || {}))
const latestInvalidCount = computed(() => Number((latestSummary.value || latestBatch.value?.summary || {}).invalid_rows || 0))
const latestParsedBatchId = computed(() => {
  if (latestBatch.value?.batch_id) return latestBatch.value.batch_id
  if (latestBatch.value?.id) return latestBatch.value.id
  const parsed = imports.value.find((row) => row.status === 'parsed')
  return parsed?.id || 0
})

onMounted(() => {
  const params = new URL(window.location.href).searchParams
  customerId.value = params.get('customer_id') || ''
  if (customerId.value) loadAll()
})

function onFileChange(event) {
  selectedFile.value = event.target.files?.[0] || null
}

async function loadAll() {
  if (!normalizedCustomerId.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const [overviewData, importData] = await Promise.all([
      fetchCustomerFulfillmentOverview(normalizedCustomerId.value),
      fetchCustomerFulfillmentImports(normalizedCustomerId.value),
    ])
    overview.value = overviewData || {}
    imports.value = importData?.imports || overviewData?.imports || []
  } catch (err) {
    error.value = err.message || '加载客户履约账户失败'
  } finally {
    loading.value = false
  }
}

async function parseImport() {
  if (!normalizedCustomerId.value || !selectedFile.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await parseCustomerFulfillmentImport(normalizedCustomerId.value, selectedImportType.value, selectedFile.value)
    latestBatch.value = data.batch || data
    latestSummary.value = data.summary || data.batch?.summary || {}
    invalidRows.value = data.invalid_rows || []
    ok.value = `已解析批次 ${data.batch_id || latestBatch.value?.id || ''}`
    await loadAll()
  } catch (err) {
    error.value = err.message || '解析失败'
  } finally {
    loading.value = false
  }
}

async function applyLatest() {
  const batchId = latestParsedBatchId.value
  if (!batchId) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await applyCustomerFulfillmentImport(batchId)
    ok.value = `已应用 ${result.applied_rows || 0} 行`
    await loadAll()
  } catch (err) {
    error.value = err.message || '应用失败'
  } finally {
    loading.value = false
  }
}

async function createSettlement() {
  if (!normalizedCustomerId.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await createCustomerFulfillmentSettlement(normalizedCustomerId.value, settlement)
    ok.value = `已生成结算 ${result.batch_id || ''}`
    await loadAll()
  } catch (err) {
    error.value = err.message || '生成结算失败'
  } finally {
    loading.value = false
  }
}

function importTypeLabel(value) {
  return importTypes.find((option) => option.value === value)?.label || value
}

function custodyTypeLabel(value) {
  return { raw_bean: '生豆', packaging: '包材', product: '产品' }[value] || value
}

function moneyFromCents(value) {
  return (Number(value || 0) / 100).toFixed(2)
}
</script>

<style scoped>
.customer-fulfillment {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.panel {
  background: #fff;
  border: 1px solid #d8dee4;
  border-radius: 8px;
  padding: 14px;
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.panel-head h2,
.panel-head h3 {
  margin: 0;
}

.panel-head p {
  margin: 4px 0 0;
  color: #64748b;
}

.toolbar,
.import-row,
.settlement-row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 10px;
  margin-top: 10px;
}

label {
  display: grid;
  gap: 4px;
  color: #475569;
  font-size: 13px;
}

input {
  min-height: 34px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  padding: 6px 8px;
  font: inherit;
}

button {
  min-height: 34px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  padding: 6px 12px;
  background: #fff;
  cursor: pointer;
}

button:disabled {
  cursor: not-allowed;
  opacity: .6;
}

.primary {
  border-color: #0f766e;
  background: #0f766e;
  color: #fff;
}

.secondary {
  background: #f8fafc;
}

.segmented {
  display: inline-flex;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  overflow: hidden;
}

.segmented button {
  border: 0;
  border-radius: 0;
}

.segmented button + button {
  border-left: 1px solid #cbd5e1;
}

.segmented .active {
  background: #0f766e;
  color: #fff;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 10px;
}

.metric {
  border: 1px solid #d8dee4;
  border-radius: 8px;
  padding: 12px;
  background: #f8fafc;
}

.metric span {
  display: block;
  color: #64748b;
  font-size: 13px;
}

.metric strong {
  display: block;
  margin-top: 6px;
  font-size: 22px;
}

.grid-2 {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
  gap: 14px;
}

table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

th,
td {
  border-bottom: 1px solid #e2e8f0;
  padding: 8px;
  text-align: left;
  vertical-align: top;
}

th {
  color: #475569;
  background: #f8fafc;
}

.muted {
  color: #64748b;
}

.empty {
  padding: 12px 0;
}

.error,
.ok {
  margin-top: 10px;
  padding: 8px 10px;
  border-radius: 6px;
}

.error {
  background: #fef2f2;
  color: #b91c1c;
}

.ok {
  background: #ecfdf5;
  color: #047857;
}
</style>
