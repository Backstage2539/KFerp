<template>
  <div class="page customer-processing-portal">
    <section class="portal-head">
      <div>
        <h2>{{ overview.customer_name || '代加工工作台' }}</h2>
        <p>原料库存、生产进度、成品库存、代发和结算</p>
      </div>
      <button class="secondary" type="button" @click="loadOverview" :disabled="loading">刷新</button>
    </section>

    <div v-if="error" class="error">{{ error }}</div>
    <div v-if="ok" class="ok">{{ ok }}</div>

    <section class="metric-grid">
      <div v-for="item in metrics" :key="item.label" class="metric">
        <span>{{ item.label }}</span>
        <strong>{{ item.value }}</strong>
      </div>
    </section>

    <section class="form-grid">
      <form class="panel" @submit.prevent="submitProcessing">
        <div class="panel-head">
          <h3>提交加工工单</h3>
        </div>
        <div class="fields">
          <label>
            <span>成品名称</span>
            <input v-model.trim="processingForm.product_name" required placeholder="例如 誉观山冷萃豆" />
          </label>
          <label>
            <span>原料名称</span>
            <input v-model.trim="processingForm.raw_bean_name" placeholder="例如 埃塞花魁" />
          </label>
          <label>
            <span>投豆克重</span>
            <input v-model.number="processingForm.input_quantity_g" type="number" min="1" required />
          </label>
          <label>
            <span>计划产量</span>
            <input v-model.number="processingForm.planned_output_units" type="number" min="1" required />
          </label>
          <label>
            <span>期望日期</span>
            <input v-model="processingForm.expected_date" type="date" />
          </label>
          <label class="wide">
            <span>备注</span>
            <input v-model.trim="processingForm.note" placeholder="加工要求" />
          </label>
        </div>
        <button class="primary" type="submit" :disabled="loading">提交工单</button>
      </form>

      <form class="panel" @submit.prevent="submitDirectShip">
        <div class="panel-head">
          <h3>提交代发信息</h3>
        </div>
        <div class="fields">
          <label>
            <span>收件人</span>
            <input v-model.trim="directShipForm.receiver_name" required />
          </label>
          <label>
            <span>电话</span>
            <input v-model.trim="directShipForm.receiver_phone" required />
          </label>
          <label class="wide">
            <span>地址</span>
            <input v-model.trim="directShipForm.receiver_address" required />
          </label>
          <label>
            <span>商品</span>
            <input v-model.trim="directShipForm.product_name" required />
          </label>
          <label>
            <span>规格</span>
            <input v-model.trim="directShipForm.spec" placeholder="100g" />
          </label>
          <label>
            <span>数量</span>
            <input v-model.number="directShipForm.quantity_units" type="number" min="1" required />
          </label>
          <label class="wide">
            <span>备注</span>
            <input v-model.trim="directShipForm.note" placeholder="发货要求" />
          </label>
        </div>
        <button class="primary" type="submit" :disabled="loading">提交代发</button>
      </form>
    </section>

    <section class="grid-2">
      <div class="panel">
        <div class="panel-head"><h3>原料托管库存</h3></div>
        <table>
          <thead><tr><th>类型</th><th>名称</th><th>规格</th><th>克重</th><th>件数</th></tr></thead>
          <tbody>
            <tr v-for="row in overview.custody_balances || []" :key="`${row.item_type}-${row.item_name}-${row.spec}`">
              <td>{{ custodyTypeLabel(row.item_type) }}</td>
              <td>{{ row.item_name }}</td>
              <td>{{ row.spec || '-' }}</td>
              <td>{{ formatG(row.quantity_g) }}</td>
              <td>{{ row.quantity_units || 0 }}</td>
            </tr>
            <tr v-if="!(overview.custody_balances || []).length"><td colspan="5" class="muted">暂无库存</td></tr>
          </tbody>
        </table>
      </div>

      <div class="panel">
        <div class="panel-head"><h3>成品库存</h3></div>
        <table>
          <thead><tr><th>产品</th><th>规格</th><th>仓库</th><th>克重</th><th>件数</th></tr></thead>
          <tbody>
            <tr v-for="row in overview.finished_goods || []" :key="`${row.product_id}-${row.product_name}-${row.spec_g}-${row.warehouse}`">
              <td>{{ row.product_name }}</td>
              <td>{{ row.spec_g ? `${row.spec_g}g` : '-' }}</td>
              <td>{{ row.warehouse || '-' }}</td>
              <td>{{ formatG(row.quantity_g) }}</td>
              <td>{{ row.quantity_units || 0 }}</td>
            </tr>
            <tr v-if="!(overview.finished_goods || []).length"><td colspan="5" class="muted">暂无成品库存</td></tr>
          </tbody>
        </table>
      </div>

      <div class="panel">
        <div class="panel-head"><h3>加工进度</h3></div>
        <table>
          <thead><tr><th>工单号</th><th>产品</th><th>状态</th><th>投豆</th><th>产量</th></tr></thead>
          <tbody>
            <tr v-for="row in overview.processing_orders || []" :key="row.work_order_no">
              <td>{{ row.work_order_no }}</td>
              <td>{{ row.product_name }}</td>
              <td>{{ statusLabel(row.status) }}</td>
              <td>{{ formatG(row.quantity_g) }}</td>
              <td>{{ row.units || 0 }}</td>
            </tr>
            <tr v-if="!(overview.processing_orders || []).length"><td colspan="5" class="muted">暂无工单</td></tr>
          </tbody>
        </table>
      </div>

      <div class="panel">
        <div class="panel-head"><h3>代发订单</h3></div>
        <table>
          <thead><tr><th>订单号</th><th>日期</th><th>地址</th><th>状态</th><th>明细</th></tr></thead>
          <tbody>
            <tr v-for="row in overview.direct_ship_orders || []" :key="row.order_no">
              <td>{{ row.order_no }}</td>
              <td>{{ row.order_date || '-' }}</td>
              <td>{{ row.receiver_address }}</td>
              <td>{{ statusLabel(row.status) }}</td>
              <td>{{ row.item_count || 0 }}</td>
            </tr>
            <tr v-if="!(overview.direct_ship_orders || []).length"><td colspan="5" class="muted">暂无代发</td></tr>
          </tbody>
        </table>
      </div>

      <div class="panel">
        <div class="panel-head"><h3>费用明细</h3></div>
        <table>
          <thead><tr><th>类型</th><th>名称</th><th>金额</th><th>来源</th></tr></thead>
          <tbody>
            <tr v-for="row in overview.fees || []" :key="`${row.fee_type}-${row.fee_name}-${row.amount_cents}`">
              <td>{{ row.fee_type }}</td>
              <td>{{ row.fee_name }}</td>
              <td>{{ moneyFromCents(row.amount_cents) }}</td>
              <td>{{ row.source || '-' }}</td>
            </tr>
            <tr v-if="!(overview.fees || []).length"><td colspan="4" class="muted">暂无费用</td></tr>
          </tbody>
        </table>
      </div>

      <div class="panel">
        <div class="panel-head"><h3>结算单</h3></div>
        <table>
          <thead><tr><th>ID</th><th>期间</th><th>状态</th><th>金额</th></tr></thead>
          <tbody>
            <tr v-for="row in overview.settlements || []" :key="row.batch_id">
              <td>{{ row.batch_id }}</td>
              <td>{{ row.period_from }} 至 {{ row.period_to }}</td>
              <td>{{ statusLabel(row.status) }}</td>
              <td>{{ moneyFromCents(row.total_amount_cents) }}</td>
            </tr>
            <tr v-if="!(overview.settlements || []).length"><td colspan="4" class="muted">暂无结算</td></tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import {
  fetchCustomerProcessingPortalOverview,
  submitCustomerDirectShipOrder,
  submitCustomerProcessingWorkOrder,
} from '../api/customer-fulfillment'

const loading = ref(false)
const error = ref('')
const ok = ref('')
const overview = ref({})
const processingForm = reactive({
  product_name: '',
  raw_bean_name: '',
  input_quantity_g: '',
  planned_output_units: '',
  expected_date: '',
  note: '',
})
const directShipForm = reactive({
  receiver_name: '',
  receiver_phone: '',
  receiver_address: '',
  product_name: '',
  spec: '',
  quantity_units: 1,
  note: '',
})

const metrics = computed(() => [
  { label: '原料库存', value: (overview.value.custody_balances || []).length },
  { label: '加工工单', value: (overview.value.processing_orders || []).length },
  { label: '成品库存', value: (overview.value.finished_goods || []).length },
  { label: '代发订单', value: (overview.value.direct_ship_orders || []).length },
  { label: '未结费用', value: (overview.value.fees || []).length },
  { label: '结算单', value: (overview.value.settlements || []).length },
])

onMounted(loadOverview)

async function loadOverview() {
  loading.value = true
  error.value = ''
  try {
    overview.value = await fetchCustomerProcessingPortalOverview()
  } catch (err) {
    error.value = err.message || '加载代加工数据失败'
  } finally {
    loading.value = false
  }
}

async function submitProcessing() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const row = await submitCustomerProcessingWorkOrder({
      product_name: processingForm.product_name,
      raw_bean_name: processingForm.raw_bean_name,
      input_quantity_g: Number(processingForm.input_quantity_g || 0),
      planned_output_units: Number(processingForm.planned_output_units || 0),
      expected_date: processingForm.expected_date,
      note: processingForm.note,
    })
    ok.value = `已提交工单 ${row.work_order_no || ''}`
    processingForm.product_name = ''
    processingForm.raw_bean_name = ''
    processingForm.input_quantity_g = ''
    processingForm.planned_output_units = ''
    processingForm.expected_date = ''
    processingForm.note = ''
    await loadOverview()
  } catch (err) {
    error.value = err.message || '提交工单失败'
  } finally {
    loading.value = false
  }
}

async function submitDirectShip() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const row = await submitCustomerDirectShipOrder({
      receiver_name: directShipForm.receiver_name,
      receiver_phone: directShipForm.receiver_phone,
      receiver_address: directShipForm.receiver_address,
      product_name: directShipForm.product_name,
      spec: directShipForm.spec,
      quantity_units: Number(directShipForm.quantity_units || 0),
      note: directShipForm.note,
    })
    ok.value = `已提交代发 ${row.order_no || ''}`
    directShipForm.receiver_name = ''
    directShipForm.receiver_phone = ''
    directShipForm.receiver_address = ''
    directShipForm.product_name = ''
    directShipForm.spec = ''
    directShipForm.quantity_units = 1
    directShipForm.note = ''
    await loadOverview()
  } catch (err) {
    error.value = err.message || '提交代发失败'
  } finally {
    loading.value = false
  }
}

function custodyTypeLabel(value) {
  return { raw_bean: '生豆', packaging: '包材', product: '成品' }[value] || value || '-'
}

function statusLabel(value) {
  return {
    submitted: '已提交',
    accepted: '已受理',
    planned: '已排产',
    running: '生产中',
    finished: '已完成',
    settled: '已结算',
    draft: '草稿',
  }[value] || value || '-'
}

function formatG(value) {
  const n = Number(value || 0)
  if (!n) return '0'
  if (Math.abs(n) >= 1000) return `${(n / 1000).toFixed(2)} kg`
  return `${n} g`
}

function moneyFromCents(value) {
  return (Number(value || 0) / 100).toFixed(2)
}
</script>

<style scoped>
.customer-processing-portal {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.portal-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  border: 1px solid #d8dee4;
  border-radius: 8px;
  padding: 16px;
  background: #fff;
}

.portal-head h2,
.portal-head p,
.panel h3 {
  margin: 0;
}

.portal-head p {
  margin-top: 4px;
  color: #64748b;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 10px;
}

.metric,
.panel {
  border: 1px solid #d8dee4;
  border-radius: 8px;
  background: #fff;
}

.metric {
  padding: 12px;
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

.form-grid,
.grid-2 {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
  gap: 14px;
}

.panel {
  padding: 14px;
  min-width: 0;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 10px;
  margin-bottom: 12px;
}

.wide {
  grid-column: span 2;
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

.error,
.ok {
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

@media (max-width: 720px) {
  .portal-head {
    align-items: stretch;
    flex-direction: column;
  }

  .form-grid,
  .grid-2 {
    grid-template-columns: 1fr;
  }

  .wide {
    grid-column: auto;
  }
}
</style>
