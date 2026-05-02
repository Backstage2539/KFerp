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
          <span>库存充足的订单生产状态设为“无需生产”，发货状态保持“未发货”；回填快递单号后再变为“已发货”。</span>
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
        <button class="secondary" type="button" @click="pickInsufficient">勾选库存不足商品</button>
        <button class="primary" type="button" @click="buildPlan" :disabled="loading">生成计划</button>
        <button class="primary" type="button" @click="startProduction" :disabled="saving || !planReady">开始生产</button>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">待发货产品</div>
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
            <tr v-for="row in rows" :key="rowKey(row)">
              <td><input type="checkbox" :checked="!!selected[rowKey(row)]" @change="selected[rowKey(row)] = !selected[rowKey(row)]" /></td>
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
            <tr v-if="!rows.length">
              <td colspan="10" class="muted">暂无待发货产品</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">生产计划（缺口 &gt; 0）</div>
      <div v-if="!planReady" class="muted">请先选择产品并点击“生成计划”。</div>
      <div v-else class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>商品</th>
              <th>规格(g)</th>
              <th>需求(g)</th>
              <th>库存(g)</th>
              <th>缺口(g)</th>
              <th>BOM出品率</th>
              <th>投料数(g)</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in computedPlanRows" :key="rowKey(row)">
              <td>{{ row.product }}</td>
              <td>{{ row.spec_g }}</td>
              <td>{{ row.need_g }}</td>
              <td>{{ row.inv_g }}</td>
              <td><strong>{{ row.gap_g }}</strong></td>
              <td>{{ percent(row.bom_yield_rate) }}</td>
              <td>{{ row.input_g }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">物料需求汇总（预计消耗）</div>
      <div v-if="!planReady" class="muted">请先选择产品并点击“生成计划”。</div>
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

    <section class="panel">
      <div class="section-title">烘焙建议</div>
      <div v-if="!planReady" class="muted">请先选择产品并点击“生成计划”。</div>
      <div v-else class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>商品</th>
              <th>推荐机器</th>
              <th>每锅数量(g)</th>
              <th>锅数</th>
              <th>最终投料数(g)</th>
              <th>熟豆总需求(kg)</th>
              <th>损耗比</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in roastPlans" :key="row.key">
              <td>{{ row.product_name }}</td>
              <td>{{ row.machine || '未匹配设备' }}</td>
              <td><input type="number" min="1" step="1" v-model.number="row.batch_g" @input="syncRoastPlan(row)" /></td>
              <td>{{ row.batch_count }}</td>
              <td><strong>{{ row.final_input_g }}</strong></td>
              <td>{{ row.finished_kg_str }}</td>
              <td>{{ row.yield_pct_str }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { buildMaterialSummary, buildStartPayload, rebuildPlanRows, producePlanKey } from '../lib/produce-plan'
import { replaceHistoryURL } from '../lib/url-state'

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const stockTip = ref('')
const rows = ref([])
const planRows = ref([])
const roastPlans = ref([])
const materialRatios = ref([])
const initialMaterials = ref([])
const selected = reactive({})

const filters = reactive({
  from: '',
  to: '',
  customer_id: '',
})

function rowKey(row) {
  return producePlanKey(row.product_id, row.spec_g)
}

function percent(v) {
  return `${(Number(v || 0) * 100).toFixed(2)}%`
}

const planReady = computed(() => roastPlans.value.length > 0 && planRows.value.length > 0)
const computedPlanRows = computed(() => rebuildPlanRows(planRows.value, roastPlans.value))
const computedMaterials = computed(() =>
  buildMaterialSummary(planRows.value, roastPlans.value, materialRatios.value, initialMaterials.value),
)

function selectedKeys() {
  return Object.keys(selected).filter((key) => selected[key])
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
    roastPlans.value = (data.roast_plans || []).map((row) => ({
      ...row,
      batch_g: Number(row.batch_g || 0),
      batch_count: Number(row.batch_count || 0),
      final_input_g: Number(row.final_input_g || 0),
    }))
    materialRatios.value = data.material_ratios || []
    initialMaterials.value = data.materials || []
    if (data.selected) {
      Object.keys(selected).forEach((key) => delete selected[key])
      for (const key of Object.keys(data.selected)) {
        if (data.selected[key]) selected[key] = true
      }
    }
    updateUrl(plan)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function pickInsufficient() {
  Object.keys(selected).forEach((key) => delete selected[key])
  for (const row of rows.value) {
    if (Number(row.gap_g || 0) > 0) {
      selected[rowKey(row)] = true
    }
  }
}

async function buildPlan() {
  if (!selectedKeys().length) {
    window.alert('请先选择产品后再生成计划')
    return
  }
  await load(true)
}

function syncRoastPlan(row) {
  row.batch_g = Math.max(1, Number(row.batch_g || 0))
  row.final_input_g = row.batch_g * Number(row.batch_count || 0)
}

async function startProduction() {
  const keys = selectedKeys()
  if (!keys.length) {
    window.alert('请先选择产品后再开始生产')
    return
  }
  if (!planReady.value) {
    window.alert('请先生成计划')
    return
  }
  saving.value = true
  error.value = ''
  try {
    const payload = buildStartPayload(filters, keys, roastPlans.value, computedPlanRows.value)
    await apiSend('/api/produce/start', { body: payload })
    window.location.href = '/produce/running?ok=1'
  } catch (err) {
    error.value = err.message || '开始生产失败'
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
  filters.customer_id = url.searchParams.get('customer_id') || ''
  const selectedCsv = url.searchParams.get('selected') || ''
  if (selectedCsv) {
    for (const key of selectedCsv.split(',')) {
      if (key) selected[key] = true
    }
  }
  await load(url.searchParams.get('plan') === '1')
})
</script>

<style scoped>
.page { padding: 16px; display: grid; gap: 16px; }
.panel { border: 1px solid #eee; border-radius: 10px; padding: 12px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.panel-head h2, .section-title { margin: 0; font-size: 18px; font-weight: 700; }
.filters { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.filters label, .actions { display: flex; gap: 8px; }
.filters label { flex-direction: column; }
.filters span { font-size: 12px; color: #666; }
.actions { margin-top: 12px; flex-wrap: wrap; }
input, button { font: inherit; }
input { width: 100%; padding: 10px; border: 1px solid #ddd; border-radius: 8px; }
button { border-radius: 8px; padding: 10px 12px; cursor: pointer; }
.primary { border: 1px solid #111; background: #111; color: #fff; }
.secondary { border: 1px solid #999; background: #fff; color: #111; }
.table-wrap { overflow: auto; }
table { width: 100%; border-collapse: collapse; min-width: 980px; }
th, td { border-bottom: 1px solid #f1f1f1; padding: 10px 8px; text-align: left; vertical-align: top; }
.muted { color: #666; }
.error, .ok { border-radius: 8px; padding: 10px; margin-bottom: 12px; }
.error { background: #ffecec; border: 1px solid #ffb9b9; }
.ok { background: #e9ffe9; border: 1px solid #b8f5b8; }
.direct-ship-tip { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.direct-ship-tip div { display: grid; gap: 4px; }
.direct-ship-tip span { color: #28633b; font-size: 13px; }

@media (max-width: 900px) {
  .page { padding: 12px; }
  .filters { grid-template-columns: 1fr; }
  .direct-ship-tip { align-items: stretch; flex-direction: column; }
}
</style>
