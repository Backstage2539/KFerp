<template>
  <div class="stock-entry-page" :class="{ embedded: props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>Stock Entry单据</h2>
          <p>领料到WIP、WIP退料、工单消耗、完工入库、报废/损耗统一在这里形成库存业务单据。</p>
        </div>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label>
          <span>单据类型</span>
          <select v-model="filters.entry_type">
            <option value="">全部</option>
            <option v-for="option in entryTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
        </label>
        <label>
          <span>状态</span>
          <select v-model="filters.status">
            <option value="">全部</option>
            <option value="submitted">已提交</option>
            <option value="cancelled">已取消</option>
          </select>
        </label>
        <label>
          <span>工单ID</span>
          <input v-model.number="filters.work_order_id" type="number" min="0" />
        </label>
        <button class="primary" type="button" @click="load" :disabled="loading">查询</button>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head compact-head">
        <h3>新建 Stock Entry</h3>
        <button class="primary" type="button" @click="createEntry" :disabled="loading">提交单据</button>
      </div>
      <div class="entry-form">
        <label>
          <span>单据类型</span>
          <select v-model="form.entry_type" @change="applyEntryDefaults">
            <option v-for="option in entryTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
        </label>
        <label><span>工单ID</span><input v-model.number="form.work_order_id" type="number" min="0" /></label>
        <label><span>工序卡ID</span><input v-model.number="form.job_card_id" type="number" min="0" /></label>
        <label><span>生产中ID</span><input v-model.number="form.running_item_id" type="number" min="0" /></label>
        <label>
          <span>物料/商品</span>
          <select v-model="form.item_type">
            <option value="material">物料</option>
            <option value="finished_product">商品</option>
          </select>
        </label>
        <label><span>物料ID</span><input v-model.number="form.material_id" type="number" min="0" /></label>
        <label><span>商品ID</span><input v-model.number="form.product_id" type="number" min="0" /></label>
        <label><span>名称</span><input v-model.trim="form.item_name" placeholder="物料或商品名称" /></label>
        <label>
          <span>出库仓</span>
          <select v-model="form.from_warehouse">
            <option value="">无</option>
            <option v-for="warehouse in warehouseOptions" :key="warehouse.value" :value="warehouse.value">{{ warehouse.label }}</option>
          </select>
        </label>
        <label>
          <span>入库仓</span>
          <select v-model="form.to_warehouse">
            <option value="">无</option>
            <option v-for="warehouse in warehouseOptions" :key="warehouse.value" :value="warehouse.value">{{ warehouse.label }}</option>
          </select>
        </label>
        <label><span>数量(g)</span><input v-model.number="form.qty_g" type="number" min="0" /></label>
        <label><span>数量(件)</span><input v-model.number="form.qty_units" type="number" min="0" /></label>
        <label><span>单位成本</span><input v-model.number="form.unit_cost" type="number" min="0" step="0.0001" /></label>
        <label class="wide"><span>备注</span><input v-model.trim="form.note" placeholder="单据备注" /></label>
      </div>
    </section>

    <section class="panel table-wrap">
      <table>
        <thead>
          <tr>
            <th>单号</th>
            <th>类型</th>
            <th>工单/工序</th>
            <th>数量</th>
            <th>成本</th>
            <th>状态</th>
            <th>操作人</th>
            <th>时间</th>
            <th>备注</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td><strong>{{ row.entry_no }}</strong><small>#{{ row.id }}</small></td>
            <td>{{ stockEntryTypeLabel(row.entry_type) }}</td>
            <td><small>工单 #{{ row.work_order_id || '-' }}</small><small>工序 #{{ row.job_card_id || '-' }}</small></td>
            <td>{{ formatG(row.total_qty_g) }}<small>{{ row.item_count || 0 }} 行</small></td>
            <td>{{ money(row.total_cost) }}</td>
            <td><span class="status">{{ statusLabel(row.status) }}</span></td>
            <td>{{ row.operator || '-' }}</td>
            <td>{{ row.created_at || '-' }}</td>
            <td>{{ row.note || '-' }}</td>
          </tr>
          <tr v-if="!rows.length"><td colspan="9" class="muted">暂无 Stock Entry 单据</td></tr>
        </tbody>
      </table>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { stockEntryEndpoint, stockEntryTypeLabel, stockEntryTypeOptions } from '../lib/manufacturing-execution'

const props = defineProps({
  embedded: { type: Boolean, default: false },
})

const entryTypeOptions = stockEntryTypeOptions()
const warehouseOptions = [
  { value: 'raw_materials', label: '原料仓' },
  { value: 'wip', label: 'WIP在制仓' },
  { value: 'finished_goods', label: '成品仓' },
  { value: 'scrap', label: '报废仓' },
]
const rows = ref([])
const loading = ref(false)
const error = ref('')
const filters = reactive({ entry_type: '', status: '', work_order_id: 0 })
const form = reactive({
  entry_type: 'material_issue_to_wip',
  work_order_id: 0,
  job_card_id: 0,
  running_item_id: 0,
  item_type: 'material',
  material_id: 0,
  product_id: 0,
  item_name: '',
  from_warehouse: 'raw_materials',
  to_warehouse: 'wip',
  qty_g: 0,
  qty_units: 0,
  unit_cost: 0,
  note: '',
})

function applyEntryDefaults() {
  const defaults = {
    material_issue_to_wip: ['raw_materials', 'wip', 'material'],
    wip_return: ['wip', 'raw_materials', 'material'],
    material_consume: ['wip', '', 'material'],
    finished_receipt: ['', 'finished_goods', 'finished_product'],
    scrap_loss: ['wip', 'scrap', 'material'],
  }[form.entry_type] || ['raw_materials', 'wip', 'material']
  form.from_warehouse = defaults[0]
  form.to_warehouse = defaults[1]
  form.item_type = defaults[2]
}

function queryURL() {
  const url = new URL(stockEntryEndpoint(), window.location.origin)
  if (filters.entry_type) url.searchParams.set('entry_type', filters.entry_type)
  if (filters.status) url.searchParams.set('status', filters.status)
  if (Number(filters.work_order_id || 0) > 0) url.searchParams.set('work_order_id', String(Number(filters.work_order_id || 0)))
  return url
}

function formatG(value) {
  return `${Number(value || 0).toLocaleString('zh-CN')}g`
}

function money(value) {
  return Number(value || 0).toFixed(2)
}

function statusLabel(value) {
  return ({ submitted: '已提交', cancelled: '已取消' })[String(value || '').trim()] || value || '-'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet(queryURL())
    rows.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function createEntry() {
  loading.value = true
  error.value = ''
  try {
    await apiSend(stockEntryEndpoint(), {
      body: {
        entry_type: form.entry_type,
        work_order_id: Number(form.work_order_id || 0),
        job_card_id: Number(form.job_card_id || 0),
        running_item_id: Number(form.running_item_id || 0),
        note: form.note || '',
        items: [{
          material_id: Number(form.material_id || 0),
          product_id: Number(form.product_id || 0),
          item_type: form.item_type,
          item_name: form.item_name || '',
          from_warehouse: form.from_warehouse || '',
          to_warehouse: form.to_warehouse || '',
          qty_g: Number(form.qty_g || 0),
          qty_units: Number(form.qty_units || 0),
          unit_cost: Number(form.unit_cost || 0),
        }],
      },
    })
    await load()
  } catch (err) {
    error.value = err.message || '提交失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.stock-entry-page{padding:16px;display:grid;gap:16px}.stock-entry-page.embedded{padding:0}.panel{border:1px solid #e5e7eb;border-radius:8px;background:#fff;padding:12px}.panel-head{display:flex;justify-content:space-between;align-items:flex-start;gap:12px;margin-bottom:12px}.compact-head{align-items:center}.panel-head h2,.panel-head h3{margin:0 0 4px;font-size:18px}.panel-head h3{font-size:16px}.panel-head p{margin:0;color:#6b7280;font-size:13px}.filters,.entry-form{display:grid;grid-template-columns:repeat(4,minmax(120px,1fr));gap:10px;align-items:end}.entry-form{grid-template-columns:repeat(5,minmax(120px,1fr))}.wide{grid-column:span 2}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,input,button{font:inherit;min-height:36px;border-radius:6px}select,input{width:100%;border:1px solid #ddd;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #9ca3af;background:#fff;color:#111}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}.table-wrap{overflow:auto}table{width:100%;min-width:980px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px;vertical-align:top}th{background:#fbfbfb}td small{display:block;color:#6b7280;margin-top:3px}.status{display:inline-flex;border:1px solid #93c5fd;background:#eff6ff;color:#1d4ed8;border-radius:999px;padding:2px 8px}.muted{text-align:center;color:#666}
@media (max-width:900px){.stock-entry-page{padding:12px}.filters,.entry-form{grid-template-columns:1fr}.wide{grid-column:auto}.panel-head{display:grid}}
</style>
