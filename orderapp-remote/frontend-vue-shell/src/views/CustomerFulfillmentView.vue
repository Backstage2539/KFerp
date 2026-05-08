<template>
  <div class="page customer-fulfillment">
    <section class="panel control-panel">
      <div class="panel-head">
        <div>
          <h2>客户履约账户</h2>
          <p>{{ overview.customer_name || selectedCustomerLabel || '未选择客户' }}</p>
        </div>
        <div class="head-actions">
          <button class="secondary" type="button" @click="openManual">客户履约手册</button>
          <button class="secondary" type="button" @click="loadAll" :disabled="loading || !normalizedCustomerId">刷新</button>
        </div>
      </div>

      <div class="toolbar">
        <label class="customer-picker-field">
          <span>选择客户</span>
          <SearchableSelect
            v-model="customerId"
            :options="customerOptions"
            :option-label="customerFulfillmentCustomerOptionLabel"
            :option-meta="customerFulfillmentCustomerOptionMeta"
            :option-value="optionNumericValue"
            placeholder="搜索客户名/公司/联系人/电话"
            empty-text="没有匹配客户"
            :disabled="loading"
            @select="selectCustomer" />
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
        <label class="file-picker">
          <span>Excel 文件</span>
          <input type="file" accept=".xlsx,.xls" @change="onFileChange" />
          <strong>{{ selectedFileName || '未选择文件' }}</strong>
        </label>
        <button class="primary" type="button" @click="parseImport" :disabled="loading || !normalizedCustomerId || !selectedFile">解析导入</button>
        <button class="secondary" type="button" @click="applyLatest" :disabled="loading || !selectedParsedBatchId">应用当前类型最新批次</button>
        <span v-if="!normalizedCustomerId" class="muted import-hint">先选择客户再上传 Excel</span>
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

      <div class="ops-grid">
        <div class="ops-panel">
          <h3>库存手动调整</h3>
          <div class="ops-form">
            <label>
              <span>类型</span>
              <select v-model="adjustment.item_type">
                <option value="raw_bean">生豆</option>
                <option value="packaging">包材</option>
                <option value="product">成品</option>
              </select>
            </label>
            <label>
              <span>名称</span>
              <input v-model.trim="adjustment.item_name" placeholder="库存名称" />
            </label>
            <label>
              <span>规格</span>
              <input v-model.trim="adjustment.spec" placeholder="可选" />
            </label>
            <label>
              <span>克重增减</span>
              <input v-model.number="adjustment.quantity_g_delta" type="number" />
            </label>
            <label>
              <span>件数增减</span>
              <input v-model.number="adjustment.quantity_units_delta" type="number" />
            </label>
            <label class="wide-field">
              <span>备注</span>
              <input v-model.trim="adjustment.note" placeholder="手工调整原因" />
            </label>
            <button class="secondary" type="button" @click="adjustCustody" :disabled="loading || !normalizedCustomerId">保存调整</button>
          </div>
        </div>

        <div class="ops-panel">
          <h3>客户 ERP 账号绑定</h3>
          <div class="ops-form binding-form">
            <label>
              <span>员工ID</span>
              <input v-model.number="binding.employee_id" type="number" min="1" placeholder="客户登录账号的员工ID" />
            </label>
            <label>
              <span>状态</span>
              <select v-model="binding.status">
                <option value="active">启用</option>
                <option value="inactive">停用</option>
              </select>
            </label>
            <button class="secondary" type="button" @click="saveERPBinding" :disabled="loading || !normalizedCustomerId">绑定账号</button>
          </div>
          <div v-if="erpBindings.length" class="binding-list">
            <span v-for="row in erpBindings" :key="`${row.customer_id}-${row.employee_id}`">
              #{{ row.employee_id }} {{ row.employee_name || '未命名员工' }} / {{ bindingStatusLabel(row.status) }}
            </span>
          </div>
          <div v-else class="muted">暂无绑定账号</div>
        </div>
      </div>

      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <section v-if="selectedParsedBatch" class="panel apply-panel">
      <div class="panel-head">
        <div>
          <h3>当前可应用批次</h3>
          <p>{{ selectedParsedBatchLabel }}</p>
        </div>
        <button class="secondary" type="button" @click="loadApplyPreview" :disabled="loading || !selectedParsedBatchId">刷新预览</button>
      </div>
      <div class="batch-line">
        <span>ID {{ selectedParsedBatch.id }}</span>
        <span>{{ importTypeLabel(selectedParsedBatch.import_type) }}</span>
        <span>{{ rowStatusLabel(selectedParsedBatch.status) }}</span>
        <span>有效 {{ selectedParsedBatch.summary?.valid_rows || 0 }}</span>
        <span>错误 {{ selectedParsedBatch.summary?.invalid_rows || 0 }}</span>
      </div>
      <div v-if="applyPreviewEffects.length" class="preview-grid">
        <div v-for="effect in applyPreviewEffects" :key="effect.label" class="preview-item">
          <span>{{ effect.label }}</span>
          <strong>{{ effect.value }}</strong>
        </div>
      </div>
      <div v-if="applyPreview?.warning" class="muted">{{ applyPreview.warning }}</div>
    </section>

    <section ref="resultAnchor" v-if="summaryCards.length" class="metric-grid">
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
      <div v-if="invalidRowGroups.length" class="error-groups">
        <div v-for="group in invalidRowGroups" :key="group.key">
          <strong>{{ group.count }}</strong>
          <span>{{ group.sheet_name }} / {{ group.row_type }} / {{ group.error }}</span>
        </div>
      </div>
      <table v-if="invalidRows.length">
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
            <tr><th>ID</th><th>类型</th><th>文件</th><th>状态</th><th>有效/错误</th><th>操作</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in imports" :key="row.id">
              <td>{{ row.id }}</td>
              <td>{{ importTypeLabel(row.import_type) }}</td>
              <td>{{ row.source_filename }}</td>
              <td>{{ rowStatusLabel(row.status) }}</td>
              <td>{{ row.summary?.valid_rows || 0 }} / {{ row.summary?.invalid_rows || 0 }}</td>
              <td>
                <button class="link-button" type="button" @click="loadInvalidRows(row)" :disabled="loading || !(row.summary?.invalid_rows > 0)">查看错误行</button>
              </td>
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
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import DataPanel from '../components/DataPanel.vue'
import SearchableSelect from '../components/SearchableSelect.vue'
import {
  applyCustomerFulfillmentImport,
  adjustCustomerFulfillmentCustodyInventory,
  createCustomerFulfillmentSettlement,
  fetchCustomerFulfillmentCustomers,
  fetchCustomerFulfillmentERPBindings,
  fetchCustomerFulfillmentImportPreview,
  fetchCustomerFulfillmentImportRows,
  fetchCustomerFulfillmentImports,
  fetchCustomerFulfillmentOverview,
  parseCustomerFulfillmentImport,
  upsertCustomerFulfillmentERPBinding,
} from '../api/customer-fulfillment'
import {
  activeCustomerFulfillmentCustomers,
  buildImportPreviewEffects,
  customerFulfillmentCustomerOptionLabel,
  customerFulfillmentCustomerOptionMeta,
  groupInvalidImportRows,
  importSummaryCards,
  importTypeOptions,
  latestParsedBatchForType,
  rowStatusLabel,
} from '../lib/customer-fulfillment'

const customerId = ref(0)
const customerOptions = ref([])
const selectedImportType = ref('processing_workbook')
const selectedFile = ref(null)
const latestSummary = ref(null)
const latestBatch = ref(null)
const applyPreview = ref(null)
const imports = ref([])
const overview = ref({})
const invalidRows = ref([])
const erpBindings = ref([])
const resultAnchor = ref(null)
const loading = ref(false)
const error = ref('')
const ok = ref('')
const settlement = reactive({
  period_from: '',
  period_to: '',
})
const adjustment = reactive({
  item_type: 'raw_bean',
  item_name: '',
  spec: '',
  quantity_g_delta: 0,
  quantity_units_delta: 0,
  note: '',
})
const binding = reactive({
  employee_id: '',
  status: 'active',
})

const importTypes = importTypeOptions()
const normalizedCustomerId = computed(() => Number(customerId.value || 0))
const selectedCustomer = computed(() => customerOptions.value.find((row) => Number(row.id) === normalizedCustomerId.value) || null)
const selectedCustomerLabel = computed(() => selectedCustomer.value ? customerFulfillmentCustomerOptionLabel(selectedCustomer.value) : '')
const selectedFileName = computed(() => selectedFile.value?.name || '')
const summaryCards = computed(() => importSummaryCards(latestSummary.value || latestBatch.value?.summary || {}))
const latestInvalidCount = computed(() => Number((latestSummary.value || latestBatch.value?.summary || {}).invalid_rows || 0))
const invalidRowGroups = computed(() => groupInvalidImportRows(invalidRows.value))
const selectedParsedBatch = computed(() => latestParsedBatchForType(imports.value, latestBatch.value, selectedImportType.value))
const selectedParsedBatchId = computed(() => Number(selectedParsedBatch.value?.id || selectedParsedBatch.value?.batch_id || 0))
const selectedParsedBatchLabel = computed(() => {
  const batch = selectedParsedBatch.value
  if (!batch) return ''
  return `${batch.source_filename || '未命名文件'} / ${importTypeLabel(batch.import_type)}`
})
const applyPreviewEffects = computed(() => {
  if (Array.isArray(applyPreview.value?.effects) && applyPreview.value.effects.length) return applyPreview.value.effects
  return buildImportPreviewEffects(selectedParsedBatch.value?.summary || {})
})

watch(selectedImportType, async () => {
  invalidRows.value = []
  await loadApplyPreview()
})

watch(selectedParsedBatchId, async () => {
  await loadApplyPreview()
})

onMounted(async () => {
  await loadCustomerOptions()
  const params = new URL(window.location.href).searchParams
  customerId.value = Number(params.get('customer_id') || 0)
  if (customerId.value) await loadAll()
})

function onFileChange(event) {
  selectedFile.value = event.target.files?.[0] || null
}

async function loadCustomerOptions(query = '') {
  try {
    const data = await fetchCustomerFulfillmentCustomers(query, 200)
    customerOptions.value = activeCustomerFulfillmentCustomers(data)
  } catch (err) {
    if (!customerOptions.value.length) error.value = err.message || '加载客户列表失败'
  }
}

async function selectCustomer(customer) {
  customerId.value = Number(customer?.id || 0)
  if (customerId.value) await loadAll()
}

async function loadAll() {
  if (!normalizedCustomerId.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const [overviewData, importData, bindingData] = await Promise.all([
      fetchCustomerFulfillmentOverview(normalizedCustomerId.value),
      fetchCustomerFulfillmentImports(normalizedCustomerId.value),
      fetchCustomerFulfillmentERPBindings(normalizedCustomerId.value),
    ])
    overview.value = overviewData || {}
    rememberOverviewCustomer(overviewData)
    imports.value = importData?.imports || overviewData?.imports || []
    erpBindings.value = bindingData?.bindings || []
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
    await loadApplyPreview()
    await nextTick()
    resultAnchor.value?.scrollIntoView?.({ behavior: 'smooth', block: 'start' })
  } catch (err) {
    error.value = err.message || '解析失败'
  } finally {
    loading.value = false
  }
}

async function applyLatest() {
  const batch = selectedParsedBatch.value
  const batchId = selectedParsedBatchId.value
  if (!batch || !batchId) return
  const confirmed = window.confirm(`确认应用 ${importTypeLabel(batch.import_type)} 批次 ${batchId}？\n文件：${batch.source_filename || '-'}\n有效行：${batch.summary?.valid_rows || 0}\n错误行：${batch.summary?.invalid_rows || 0}`)
  if (!confirmed) return
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

async function adjustCustody() {
  if (!normalizedCustomerId.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const row = await adjustCustomerFulfillmentCustodyInventory(normalizedCustomerId.value, {
      item_type: adjustment.item_type,
      item_name: adjustment.item_name,
      spec: adjustment.spec,
      quantity_g_delta: Number(adjustment.quantity_g_delta || 0),
      quantity_units_delta: Number(adjustment.quantity_units_delta || 0),
      note: adjustment.note,
    })
    ok.value = `已调整库存：${row.item_name || adjustment.item_name}`
    adjustment.quantity_g_delta = 0
    adjustment.quantity_units_delta = 0
    adjustment.note = ''
    await loadAll()
  } catch (err) {
    error.value = err.message || '库存调整失败'
  } finally {
    loading.value = false
  }
}

async function saveERPBinding() {
  if (!normalizedCustomerId.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const row = await upsertCustomerFulfillmentERPBinding(normalizedCustomerId.value, {
      employee_id: Number(binding.employee_id || 0),
      status: binding.status,
    })
    ok.value = `已绑定账号 #${row.employee_id}`
    binding.employee_id = ''
    await loadERPBindings()
  } catch (err) {
    error.value = err.message || '绑定账号失败'
  } finally {
    loading.value = false
  }
}

async function loadERPBindings() {
  if (!normalizedCustomerId.value) return
  const data = await fetchCustomerFulfillmentERPBindings(normalizedCustomerId.value)
  erpBindings.value = data?.bindings || []
}

async function loadInvalidRows(batch) {
  const batchId = Number(batch?.id || 0)
  if (!batchId) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await fetchCustomerFulfillmentImportRows(batchId, { status: 'invalid', limit: 200 })
    invalidRows.value = data?.rows || []
    latestBatch.value = batch
    latestSummary.value = batch?.summary || {}
    ok.value = `已载入批次 ${batchId} 的错误行`
    await nextTick()
    resultAnchor.value?.scrollIntoView?.({ behavior: 'smooth', block: 'start' })
  } catch (err) {
    error.value = err.message || '加载错误行失败'
  } finally {
    loading.value = false
  }
}

async function loadApplyPreview() {
  const batchId = selectedParsedBatchId.value
  if (!batchId) {
    applyPreview.value = null
    return
  }
  try {
    applyPreview.value = await fetchCustomerFulfillmentImportPreview(batchId)
  } catch {
    applyPreview.value = {
      batch: selectedParsedBatch.value,
      effects: buildImportPreviewEffects(selectedParsedBatch.value?.summary || {}),
      warning: '应用预览暂时不可用，请按批次摘要核对后再应用。',
    }
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

function bindingStatusLabel(value) {
  return value === 'active' ? '启用' : '停用'
}

function moneyFromCents(value) {
  return (Number(value || 0) / 100).toFixed(2)
}

function optionNumericValue(option) {
  return Number(option?.id || 0)
}

function rememberOverviewCustomer(data) {
  const id = Number(data?.customer_id || 0)
  if (!id || !data?.customer_name) return
  if (customerOptions.value.some((row) => Number(row.id) === id)) return
  customerOptions.value = [
    { id, name: data.customer_name, active: true },
    ...customerOptions.value,
  ]
}

function openManual() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: { key: 'customerFulfillmentManual' },
  }))
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

.head-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
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

.customer-picker-field {
  flex: 1 1 340px;
  max-width: 520px;
}

.file-picker {
  min-width: 260px;
}

.file-picker input {
  max-width: 260px;
}

.file-picker strong {
  color: #0f172a;
  font-weight: 600;
  max-width: 260px;
  overflow-wrap: anywhere;
}

.import-hint {
  align-self: center;
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

select {
  min-height: 34px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  padding: 6px 8px;
  background: #fff;
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

.link-button {
  min-height: 28px;
  border: 0;
  padding: 0;
  color: #0f766e;
  background: transparent;
  font-weight: 600;
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

.ops-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 12px;
  margin-top: 12px;
}

.ops-panel {
  border: 1px solid #d8dee4;
  border-radius: 8px;
  padding: 12px;
  background: #f8fafc;
}

.ops-panel h3 {
  margin: 0 0 10px;
  font-size: 16px;
}

.ops-form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
  gap: 8px;
  align-items: end;
}

.binding-form {
  grid-template-columns: minmax(160px, 1fr) 120px 100px;
}

.wide-field {
  grid-column: span 2;
}

.binding-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.binding-list span {
  border: 1px solid #cbd5e1;
  border-radius: 999px;
  padding: 5px 8px;
  background: #fff;
  color: #334155;
  font-size: 12px;
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

.apply-panel {
  display: grid;
  gap: 10px;
}

.batch-line {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.batch-line span {
  border: 1px solid #d8dee4;
  border-radius: 999px;
  padding: 4px 8px;
  background: #f8fafc;
  color: #334155;
  font-size: 12px;
}

.preview-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 8px;
}

.preview-item {
  border: 1px solid #d8dee4;
  border-radius: 8px;
  padding: 10px;
  background: #fff;
}

.preview-item span {
  display: block;
  color: #64748b;
  font-size: 12px;
}

.preview-item strong {
  display: block;
  margin-top: 4px;
  color: #0f172a;
}

.error-groups {
  display: grid;
  gap: 8px;
  margin-bottom: 10px;
}

.error-groups div {
  display: flex;
  align-items: start;
  gap: 8px;
  border: 1px solid #fecaca;
  border-radius: 8px;
  padding: 8px;
  background: #fef2f2;
  color: #991b1b;
}

.error-groups strong {
  min-width: 28px;
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
