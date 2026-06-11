<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>生产计划</h2>
        <button class="secondary" type="button" @click="load(false)" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="stockTip" class="ok direct-ship-tip">
        <div>
          <strong>{{ stockTip }}</strong>
          <span>库存充足订单在录单保存时确认是否使用成品批次；确认使用后进入“库存待发货”，可直接发货。</span>
        </div>
        <button class="secondary" type="button" @click="openShipReadyOrders">去订单列表直接发货</button>
      </div>
      <div class="filters">
        <label>
          <span>开始日期</span>
          <input v-model.trim="filters.from" placeholder="YYYY-MM-DD" />
        </label>
        <label>
          <span>结束日期</span>
          <input v-model.trim="filters.to" placeholder="YYYY-MM-DD" />
        </label>
        <label>
          <span>客户ID</span>
          <input v-model.trim="filters.customer_id" placeholder="例如 123" />
        </label>
      </div>
      <div class="actions">
        <button class="primary" type="button" @click="createProductionPlan" :disabled="saving || !hasSelectedRows">创建生产计划</button>
      </div>
      <div v-if="currentPlan" class="ok plan-result">
        <strong>{{ currentPlan.plan_no }}</strong>
        <span :class="['status', `status-${productionPlanStatusTone(currentPlan.status)}`]">{{ productionPlanStatusLabel(currentPlan.status) }}</span>
        <span>计划行 {{ currentPlan.items?.length || 0 }} 条</span>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h2>生产计划单据</h2>
        <button class="secondary" type="button" @click="loadProductionPlans" :disabled="loading">刷新单据</button>
      </div>
      <div class="filters production-plan-filters">
        <label>
          <span>状态</span>
          <select v-model="productionPlanFilters.status" @change="loadProductionPlans">
            <option value="">全部</option>
            <option value="draft">草稿</option>
            <option value="submitted">已提交工单</option>
            <option value="in_progress">生产中</option>
            <option value="completed">已完成</option>
            <option value="cancelled">已取消</option>
          </select>
        </label>
        <label>
          <span>时间类型</span>
          <select v-model="productionPlanFilters.time_field" @change="loadProductionPlans">
            <option value="created_at">创建时间</option>
            <option value="submitted_at">提交时间</option>
            <option value="completed_at">完成时间</option>
          </select>
        </label>
        <label>
          <span>开始日期</span>
          <input v-model.trim="productionPlanFilters.from" type="date" />
        </label>
        <label>
          <span>结束日期</span>
          <input v-model.trim="productionPlanFilters.to" type="date" />
        </label>
        <button class="secondary filter-action" type="button" @click="loadProductionPlans" :disabled="loading">过滤</button>
      </div>
      <div class="actions plan-list-actions">
        <button class="primary" type="button" @click="submitSelectedProductionPlans" :disabled="saving || !hasSelectedProductionPlans">提交生成工单</button>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>
                <input
                  ref="productionPlanHeaderCheckbox"
                  class="bulk-checkbox"
                  type="checkbox"
                  :checked="allProductionPlansSelected"
                  :disabled="!productionPlanSelection.total"
                  :aria-checked="productionPlanSelection.indeterminate ? 'mixed' : String(productionPlanSelection.checked)"
                  aria-label="全选草稿生产计划"
                  @change="toggleAllProductionPlans($event.target.checked)"
                />
              </th>
              <th>计划号</th>
              <th>来源</th>
              <th>状态</th>
              <th>行数</th>
              <th>创建人</th>
              <th>提交人</th>
              <th>时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="plan in productionPlans" :key="plan.id">
              <td>
                <input
                  class="bulk-checkbox"
                  type="checkbox"
                  :checked="!!selectedProductionPlans[String(plan.id)]"
                  :disabled="!productionPlanSelectable(plan)"
                  :aria-label="`选择生产计划 ${plan.plan_no}`"
                  @change="toggleProductionPlan(plan, $event.target.checked)"
                />
              </td>
              <td><strong>{{ plan.plan_no }}</strong></td>
              <td>{{ plan.source_type || '-' }}</td>
              <td><span :class="['status', `status-${productionPlanStatusTone(plan.status)}`]">{{ productionPlanStatusLabel(plan.status) }}</span></td>
              <td>{{ plan.item_count || 0 }}</td>
              <td>{{ plan.created_by || '-' }}</td>
              <td>{{ plan.submitted_by || '-' }}</td>
              <td><small>建 {{ plan.created_at || '-' }}</small><small>交 {{ plan.submitted_at || '-' }}</small><small>完 {{ plan.completed_at || '-' }}</small></td>
            </tr>
            <tr v-if="!productionPlans.length">
              <td colspan="8" class="muted">暂无生产计划单据</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="section-title section-title-with-checkbox">
        <input
          ref="insufficientHeaderCheckbox"
          class="bulk-checkbox"
          type="checkbox"
          :checked="allInsufficientSelected"
          :disabled="!stockInsufficientRows.length"
          :aria-checked="insufficientSelection.indeterminate ? 'mixed' : String(insufficientSelection.checked)"
          aria-label="全选库存不足商品"
          @change="toggleAllInsufficient($event.target.checked)"
        />
        <span>库存不足（需生产）</span>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>选择</th>
              <th>商品</th>
              <th>订单号</th>
              <th>规格(g)</th>
              <th>需求(件)</th>
              <th>需求(g)</th>
              <th>库存(件)</th>
              <th>库存散装(g)</th>
              <th>库存合计(g)</th>
              <th>缺口(g)</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in stockInsufficientRows" :key="rowKey(row)">
              <td><input type="checkbox" :checked="!!selected[rowKey(row)]" @change="toggleInsufficientRow(row, $event.target.checked)" /></td>
              <td>{{ row.product }}</td>
              <td class="muted">{{ row.order_nos }}</td>
              <td>{{ row.spec_g }}</td>
              <td>{{ row.need_units }}</td>
              <td>{{ row.need_g }}</td>
              <td>{{ row.inv_units }}</td>
              <td>{{ row.inv_loose_g }}</td>
              <td>{{ row.inv_g }}</td>
              <td><strong>{{ row.gap_g }}</strong></td>
            </tr>
            <tr v-if="!stockInsufficientRows.length">
              <td colspan="10" class="muted">暂无库存不足商品</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">库存充足（只提示）</div>
      <div class="muted section-hint">以下订单已有成品库存覆盖，不进入生产计划；录单时确认使用批次后会进入库存待发货。</div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>商品</th>
              <th>订单号</th>
              <th>规格(g)</th>
              <th>需求(件)</th>
              <th>需求(g)</th>
              <th>库存(件)</th>
              <th>库存散装(g)</th>
              <th>库存合计(g)</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in stockSufficientRows" :key="rowKey(row)">
              <td>{{ row.product }}</td>
              <td class="muted">{{ row.order_nos }}</td>
              <td>{{ row.spec_g }}</td>
              <td>{{ row.need_units }}</td>
              <td>{{ row.need_g }}</td>
              <td>{{ row.inv_units }}</td>
              <td>{{ row.inv_loose_g }}</td>
              <td>{{ row.inv_g }}</td>
            </tr>
            <tr v-if="!stockSufficientRows.length">
              <td colspan="8" class="muted">暂无库存充足商品</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">生产计划（缺口 &gt; 0）</div>
      <div v-if="!planReady" class="muted">选择库存不足商品后点击“创建生产计划”。</div>
      <div v-else class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>商品</th>
              <th>规格(g)</th>
              <th>需求(g)</th>
              <th>库存(g)</th>
              <th>缺口(g)</th>
              <th>BOM摘要</th>
              <th>计划投料(g)</th>
              <th>工艺路线摘要</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in computedPlanRows" :key="rowKey(row)">
              <td>{{ row.product }}</td>
              <td>{{ row.spec_g }}</td>
              <td>{{ row.need_g }}</td>
              <td>{{ row.inv_g }}</td>
              <td><strong>{{ row.gap_g }}</strong></td>
              <td>默认 BOM / 预期产出率 {{ percent(row.bom_yield_rate) }}</td>
              <td>{{ row.input_g }}</td>
              <td>{{ productionRouteSummary(row) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">物料需求汇总（预计消耗）</div>
      <div v-if="!planReady" class="muted">选择库存不足商品后点击“创建生产计划”。</div>
      <div v-else class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>物料</th>
              <th>预计消耗数量</th>
              <th>单位</th>
              <th>WIP可用(g)</th>
              <th>建议领到WIP(g)</th>
              <th>原料仓(g)</th>
              <th>采购建议(g)</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in computedMaterials" :key="`${item.name}-${item.unit}`">
              <td>{{ item.name }}</td>
              <td>{{ item.qty }}</td>
              <td>{{ item.unit }}</td>
              <td>{{ item.available_g || 0 }}</td>
              <td><strong>{{ item.wip_transfer_suggestion_g || 0 }}</strong></td>
              <td>{{ item.raw_g || 0 }}</td>
              <td>{{ item.purchase_suggestion_g || 0 }}</td>
            </tr>
            <tr v-if="!computedMaterials.length">
              <td colspan="7" class="muted">暂无物料汇总</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch, watchEffect } from 'vue'
import { apiGet, apiSend } from '../api/client'
import {
  buildInsufficientSelection,
  buildProductionPlanBatchSubmitPayload,
  buildProductionPlanCreatePayload,
  buildProductionPlanListQuery,
  buildProductionPlanSelection,
  insufficientSelectionState,
  productionPlanBatchSubmitEndpoint,
  productionPlanSelectable,
  productionPlanSelectionState,
  productionPlanStatusLabel,
  productionPlanStatusTone,
  producePlanKey,
} from '../lib/produce-plan'
import { replaceHistoryURL } from '../lib/url-state'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const stockTip = ref('')
const rows = ref([])
const planRows = ref([])
const initialMaterials = ref([])
const productionPlans = ref([])
const currentPlan = ref(null)
const insufficientHeaderCheckbox = ref(null)
const productionPlanHeaderCheckbox = ref(null)
const selected = reactive({})
const selectedProductionPlans = reactive({})

const filters = reactive({
  from: '',
  to: '',
  customer_id: '',
})

const productionPlanFilters = reactive({
  status: '',
  time_field: 'created_at',
  from: '',
  to: '',
  limit: 50,
})

function rowKey(row) {
  return producePlanKey(row.product_id, row.spec_g)
}

function percent(v) {
  return `${(Number(v || 0) * 100).toFixed(2)}%`
}

const planReady = computed(() => planRows.value.length > 0)
const computedPlanRows = computed(() => planRows.value || [])
const computedMaterials = computed(() => initialMaterials.value || [])
const hasSelectedRows = computed(() => selectedKeys().length > 0)
const stockInsufficientRows = computed(() => rows.value.filter((row) => Number(row.gap_g || 0) > 0))
const stockSufficientRows = computed(() => rows.value.filter((row) => Number(row.gap_g || 0) <= 0))
const insufficientSelection = computed(() => insufficientSelectionState(stockInsufficientRows.value, selected))
const allInsufficientSelected = computed(() => insufficientSelection.value.checked)
const productionPlanSelection = computed(() => productionPlanSelectionState(productionPlans.value, selectedProductionPlans))
const allProductionPlansSelected = computed(() => productionPlanSelection.value.checked)
const hasSelectedProductionPlans = computed(() => productionPlanSelection.value.selectedCount > 0)

watchEffect(() => {
  if (insufficientHeaderCheckbox.value) {
    insufficientHeaderCheckbox.value.indeterminate = insufficientSelection.value.indeterminate
  }
  if (productionPlanHeaderCheckbox.value) {
    productionPlanHeaderCheckbox.value.indeterminate = productionPlanSelection.value.indeterminate
  }
})

function selectedKeys() {
  return Object.keys(selected).filter((key) => selected[key])
}

function replaceSelected(nextSelected) {
  Object.keys(selected).forEach((key) => delete selected[key])
  Object.assign(selected, nextSelected)
}

function replaceSelectedProductionPlans(nextSelected) {
  Object.keys(selectedProductionPlans).forEach((key) => delete selectedProductionPlans[key])
  Object.assign(selectedProductionPlans, nextSelected)
}

function updateUrl(plan) {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'producePlan')
  if (filters.from) url.searchParams.set('from', filters.from)
  else url.searchParams.delete('from')
  if (filters.to) url.searchParams.set('to', filters.to)
  else url.searchParams.delete('to')
  if (filters.customer_id) url.searchParams.set('customer_id', filters.customer_id)
  else url.searchParams.delete('customer_id')
  const keys = selectedKeys()
  if (plan && keys.length) {
    url.searchParams.set('plan', '1')
    url.searchParams.set('selected', keys.join(','))
  } else {
    url.searchParams.delete('plan')
    url.searchParams.delete('selected')
  }
  replaceHistoryURL(url)
}

async function load(plan) {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/produce/unproduced', window.location.origin)
    if (filters.from) url.searchParams.set('from', filters.from)
    if (filters.to) url.searchParams.set('to', filters.to)
    if (filters.customer_id) url.searchParams.set('customer_id', filters.customer_id)
    const keys = selectedKeys()
    if (plan && keys.length) {
      url.searchParams.set('plan', '1')
      url.searchParams.set('selected', keys.join(','))
    }
    const data = await apiGet(url)

    rows.value = data.rows || []
    stockTip.value = data.stock_tip || ''
    planRows.value = data.plan_rows || []
    initialMaterials.value = data.materials || []
    if (data.selected) {
      Object.keys(selected).forEach((key) => delete selected[key])
      for (const key of Object.keys(data.selected)) {
        if (data.selected[key]) selected[key] = true
      }
    }
    pruneSufficientSelections()
    updateUrl(plan)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadProductionPlans() {
  try {
    const data = await apiGet(buildProductionPlanListQuery(productionPlanFilters))
    productionPlans.value = data.rows || []
    pruneProductionPlanSelections()
  } catch (err) {
    error.value = err.message || '加载生产计划失败'
  }
}

function toggleAllInsufficient(checked) {
  replaceSelected(buildInsufficientSelection(stockInsufficientRows.value, checked))
}

function toggleInsufficientRow(row, checked) {
  const key = rowKey(row)
  if (checked) selected[key] = true
  else delete selected[key]
}

function toggleAllProductionPlans(checked) {
  replaceSelectedProductionPlans(buildProductionPlanSelection(productionPlans.value, checked))
}

function toggleProductionPlan(plan, checked) {
  const key = String(Number(plan?.id || 0))
  if (!productionPlanSelectable(plan)) {
    delete selectedProductionPlans[key]
    return
  }
  if (checked) selectedProductionPlans[key] = true
  else delete selectedProductionPlans[key]
}

function pruneSufficientSelections() {
  const allowed = new Set(stockInsufficientRows.value.map((row) => rowKey(row)))
  for (const key of Object.keys(selected)) {
    if (!allowed.has(key)) delete selected[key]
  }
}

function pruneProductionPlanSelections() {
  const allowed = new Set(productionPlans.value.filter(productionPlanSelectable).map((plan) => String(Number(plan.id))))
  for (const key of Object.keys(selectedProductionPlans)) {
    if (!allowed.has(key)) delete selectedProductionPlans[key]
  }
}

function productionRouteSummary(row) {
  const templateID = Number(row?.operation_template_id || 0)
  if (templateID > 0) return `工艺模板 #${templateID}`
  return '按商品默认工艺路线'
}

async function createProductionPlan() {
  let keys = selectedKeys()
  if (!keys.length) {
    window.alert('请先选择产品后再创建生产计划')
    return
  }
  saving.value = true
  error.value = ''
  try {
    if (!planReady.value) {
      await load(true)
      keys = selectedKeys()
      if (!keys.length) {
        window.alert('请先选择产品后再创建生产计划')
        return
      }
      if (!planReady.value) {
        if (!error.value) error.value = '没有可创建的生产计划，请检查库存缺口或订单商品绑定'
        return
      }
    }
    const payload = buildProductionPlanCreatePayload(filters, keys)
    currentPlan.value = await apiSend('/api/production-plans', { body: payload })
    await loadProductionPlans()
  } catch (err) {
    error.value = err.message || '创建生产计划失败'
  } finally {
    saving.value = false
  }
}

async function submitSelectedProductionPlans() {
  const payload = buildProductionPlanBatchSubmitPayload(selectedProductionPlans)
  if (!payload.ids.length) return
  saving.value = true
  error.value = ''
  try {
    const result = await apiSend(productionPlanBatchSubmitEndpoint(), { body: payload })
    const firstSuccess = Array.isArray(result.success) ? result.success[0] : null
    currentPlan.value = firstSuccess?.plan || currentPlan.value
    replaceSelectedProductionPlans({})
    await loadProductionPlans()
    if (Array.isArray(result.failed) && result.failed.length) {
      error.value = `部分生产计划提交失败：${result.failed.map((item) => `${item.id}: ${item.error}`).join('；')}`
    }
  } catch (err) {
    error.value = err.message || '提交生成工单失败'
  } finally {
    saving.value = false
  }
}

function openShipReadyOrders() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: { key: 'orders', params: { ship_ready: '1' } },
  }))
}

onMounted(async () => {
  const url = new URL(window.location.href)
  filters.from = url.searchParams.get('from') || ''
  filters.to = url.searchParams.get('to') || ''
  filters.customer_id = String(props.viewParams?.customer_id || props.customerContextId || url.searchParams.get('customer_id') || '')
  const selectedCsv = url.searchParams.get('selected') || ''
  if (selectedCsv) {
    for (const key of selectedCsv.split(',')) {
      if (key) selected[key] = true
    }
  }
  await load(url.searchParams.get('plan') === '1')
  await loadProductionPlans()
})

watch(() => [props.viewParams?.customer_id, props.customerContextId], async () => {
  const nextCustomerID = String(props.viewParams?.customer_id || props.customerContextId || '')
  if (String(filters.customer_id || '') === nextCustomerID) return
  filters.customer_id = nextCustomerID
  await load(false)
})
</script>

<style scoped>
.page { padding: 16px; display: grid; gap: 16px; }
.panel { border: 1px solid #eee; border-radius: 10px; padding: 12px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.panel-head h2, .section-title { margin: 0; font-size: 18px; font-weight: 700; }
.filters { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.production-plan-filters { grid-template-columns: repeat(4, minmax(0, 1fr)) auto; align-items: end; }
.filters label, .actions { display: flex; gap: 8px; }
.filters label { flex-direction: column; }
.filters span { font-size: 12px; color: #666; }
.actions { margin-top: 12px; flex-wrap: wrap; }
.plan-list-actions { margin: 12px 0; }
.filter-action { min-height: 42px; }
.section-title-with-checkbox { display: inline-flex; align-items: center; gap: 8px; }
.section-hint { margin: 6px 0 10px; }
input, select, button { font: inherit; }
input { width: 100%; padding: 10px; border: 1px solid #ddd; border-radius: 8px; }
select { width: 100%; padding: 10px; border: 1px solid #ddd; border-radius: 8px; background: #fff; }
button { border-radius: 8px; padding: 10px 12px; cursor: pointer; }
input.bulk-checkbox { width: 18px; min-width: 18px; height: 18px; padding: 0; cursor: pointer; }
input.bulk-checkbox:disabled { cursor: not-allowed; opacity: 0.45; }
.primary { border: 1px solid #111; background: #111; color: #fff; }
.secondary { border: 1px solid #999; background: #fff; color: #111; }
.compact { min-height: 30px; padding: 5px 10px; }
.status { display: inline-flex; border: 1px solid #d1d5db; border-radius: 999px; padding: 2px 8px; background: #f9fafb; color: #374151; white-space: nowrap; }
.status-draft { border-color: #d1d5db; background: #f3f4f6; color: #374151; }
.status-submitted { border-color: #93c5fd; background: #eff6ff; color: #1d4ed8; }
.status-in-progress { border-color: #fdba74; background: #fff7ed; color: #c2410c; }
.status-completed { border-color: #86efac; background: #f0fdf4; color: #15803d; }
.status-cancelled { border-color: #fca5a5; background: #fef2f2; color: #b91c1c; }
.status-unknown { border-color: #e5e7eb; background: #f9fafb; color: #4b5563; }
.plan-result { margin-top: 12px; display: flex; gap: 12px; align-items: center; }
.table-wrap { overflow: auto; }
table { width: 100%; border-collapse: collapse; min-width: 980px; }
th, td { border-bottom: 1px solid #f1f1f1; padding: 10px 8px; text-align: left; vertical-align: top; }
td small { display: block; color: #666; line-height: 1.6; }
.muted { color: #666; }
.error, .ok { border-radius: 8px; padding: 10px; margin-bottom: 12px; }
.error { background: #ffecec; border: 1px solid #ffb9b9; }
.ok { background: #e9ffe9; border: 1px solid #b8f5b8; }
.direct-ship-tip { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.direct-ship-tip div { display: grid; gap: 4px; }
.direct-ship-tip span { color: #28633b; font-size: 13px; }

@media (max-width: 900px) {
  .page { padding: 12px; }
  .filters, .production-plan-filters { grid-template-columns: 1fr; }
  .direct-ship-tip { align-items: stretch; flex-direction: column; }
}
</style>
