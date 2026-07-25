<template>
  <div class="stock-entry-page" :class="{ embedded: props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>库存单据</h2>
          <p>原料入库、发料、转仓和生产库存动作统一形成 SE 单据；草稿不会改变库存。</p>
        </div>
        <div class="head-actions">
          <button class="secondary" type="button" @click="openWipInventory">查看 WIP 库存</button>
          <button class="primary" type="button" @click="openNewDrawer()">新建库存单据</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label><span>搜索</span><input v-model.trim="filters.q" placeholder="SE 单号 / 备注" @keyup.enter="load" /></label>
        <label>
          <span>单据目的</span>
          <select v-model="filters.purpose">
            <option value="">全部</option>
            <option v-for="option in entryTypeOptions" :key="option.value" :value="normalizedPurpose(option.value)">{{ option.label }}</option>
          </select>
        </label>
        <label>
          <span>状态</span>
          <select v-model="filters.status">
            <option value="">全部</option>
            <option value="draft">草稿</option>
            <option value="submitted">已提交</option>
            <option value="cancelled">已取消</option>
          </select>
        </label>
        <label><span>工单ID</span><input v-model.number="filters.work_order_id" type="number" min="0" /></label>
        <button class="secondary" type="button" @click="load" :disabled="loading">查询</button>
      </div>
    </section>

    <section class="panel table-wrap">
      <table>
        <thead><tr><th>单号</th><th>目的</th><th>工单</th><th>数量</th><th>状态</th><th>操作人</th><th>时间</th><th>备注</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id">
            <td><strong>{{ row.entry_no }}</strong><small v-if="row.legacy">历史单据</small></td>
            <td>{{ documentPurposeLabel(row) }}</td>
            <td>{{ row.work_order_id ? `#${row.work_order_id}` : '-' }}</td>
            <td>{{ quantityLabel(row) }}<small>{{ row.item_count || 0 }} 行</small></td>
            <td><span class="status" :class="row.status">{{ statusLabel(row.status) }}</span></td>
            <td>{{ row.operator || '-' }}</td>
            <td>{{ row.created_at || '-' }}</td>
            <td>{{ row.note || '-' }}</td>
            <td class="row-actions">
              <button v-if="!row.legacy" class="link" type="button" @click="openExisting(row)">查看</button>
              <span v-else class="legacy-readonly">只读</span>
              <button v-if="row.status === 'cancelled'" class="link disabled" type="button" disabled>已取消</button>
              <button v-else-if="row.status === 'submitted' && !row.legacy" class="danger-link" type="button" @click="cancelDocument(row)">取消</button>
            </td>
          </tr>
          <tr v-if="!rows.length"><td colspan="9" class="muted">暂无库存单据</td></tr>
        </tbody>
      </table>
    </section>

    <div v-if="drawerOpen" class="drawer-mask" @click.self="closeDrawer">
      <aside class="drawer" aria-label="库存单据抽屉">
        <div class="drawer-head">
          <div>
            <h3>{{ form.id ? `${form.entry_no} · ${statusLabel(form.status)}` : '新建库存单据' }}</h3>
            <p v-if="form.return_source">返回来源：{{ form.return_source }}</p>
          </div>
          <button class="secondary" type="button" @click="closeDrawer">关闭</button>
        </div>
        <div v-if="drawerError" class="error">{{ drawerError }}</div>
        <div class="document-form">
          <label>
            <span>单据目的</span>
            <select v-model="form.purpose_key" :disabled="!isDraft" @change="applyEntryDefaults">
              <option v-for="option in entryTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </label>
          <label><span>工单</span><input v-model.number="form.work_order_id" type="number" min="0" :disabled="!isDraft" /></label>
          <label><span>工序卡</span><input v-model.number="form.job_card_id" type="number" min="0" :disabled="!isDraft" /></label>
          <label><span>生产中</span><input v-model.number="form.running_item_id" type="number" min="0" :disabled="!isDraft" /></label>
          <label class="wide"><span>备注</span><input v-model.trim="form.note" :disabled="!isDraft" /></label>
        </div>

        <div class="line-head">
          <h4>单据明细</h4>
          <button v-if="isDraft" class="secondary" type="button" @click="addItem">新增明细</button>
        </div>
        <div v-for="(item, index) in form.items" :key="item.local_key || item.id || index" class="item-card">
          <div class="item-grid">
            <label>
              <span>类型</span>
              <select v-model="item.item_type" :disabled="!isDraft" @change="resetItemObject(item)">
                <option value="material">物料</option>
                <option value="finished_product">商品 / SKU</option>
              </select>
            </label>
            <label class="wide">
              <span>{{ item.item_type === 'material' ? '物料' : '商品 / SKU' }}</span>
              <SearchableSelect
                v-if="isDraft"
                :model-value="selectedObjectID(item)"
                :options="item.item_type === 'material' ? materials : products"
                :option-label="itemOptionLabel"
                placeholder="输入名称 / 编号"
                @update:model-value="selectItemObject(item, $event)"
              />
              <input v-else :value="item.item_name || '-'" disabled />
            </label>
            <label v-if="item.item_type === 'finished_product'"><span>规格(g)</span><input v-model.number="item.spec_g" type="number" min="1" :disabled="!isDraft" /></label>
            <label><span>出库仓</span><select v-model="item.from_warehouse" :disabled="!isDraft"><option value="">无</option><option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}</option></select></label>
            <label><span>入库仓</span><select v-model="item.to_warehouse" :disabled="!isDraft"><option value="">无</option><option v-for="warehouse in warehouses" :key="warehouse.code" :value="warehouse.code">{{ warehouse.name }}</option></select></label>
            <label><span>数量(g)</span><input v-model.number="item.qty_g" type="number" min="0" :disabled="!isDraft" /></label>
            <label><span>数量(件)</span><input v-model.number="item.qty_units" type="number" min="0" :disabled="!isDraft" /></label>
            <label><span>指定批次（可选）</span><input v-model.trim="item.batch_code" :disabled="!isDraft" placeholder="不填按 FIFO" /></label>
            <label><span>单位成本</span><input v-model.number="item.unit_cost" type="number" min="0" step="0.0001" :disabled="!isDraft || !isReceipt" /></label>
            <template v-if="isReceipt && item.item_type === 'material'">
              <label><span>供应商</span><input v-model.trim="item.supplier" :disabled="!isDraft" /></label>
              <label><span>产季</span><input v-model.trim="item.crop_season" :disabled="!isDraft" /></label>
              <label><span>产地</span><input v-model.trim="item.origin" :disabled="!isDraft" /></label>
              <label class="wide"><span>产家风味描述</span><input v-model.trim="item.producer_flavor_description" :disabled="!isDraft" /></label>
            </template>
          </div>
          <div v-if="item.allocations?.length" class="allocations">
            <span v-for="allocation in item.allocations" :key="`${allocation.material_batch_id}-${allocation.batch_code}`">
              {{ allocation.batch_code }}：{{ allocation.qty_g ? `${allocation.qty_g}g` : `${allocation.qty_units}件` }}
            </span>
          </div>
          <button v-if="isDraft && form.items.length > 1" class="danger-link" type="button" @click="form.items.splice(index, 1)">删除明细</button>
        </div>
        <div class="drawer-actions">
          <button v-if="isDraft" class="secondary" type="button" @click="saveDraft" :disabled="saving">保存草稿</button>
          <button v-if="isDraft" class="primary" type="button" @click="submitDocument" :disabled="saving">提交并过账</button>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import SearchableSelect from '../components/SearchableSelect.vue'
import { stockEntryEndpoint, stockEntryTypeLabel, stockEntryTypeOptions } from '../lib/manufacturing-execution'

const props = defineProps({
  embedded: { type: Boolean, default: false },
  viewParams: { type: Object, default: () => ({}) },
})

const entryTypeOptions = stockEntryTypeOptions()
const rows = ref([])
const materials = ref([])
const products = ref([])
const warehouses = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const drawerError = ref('')
const drawerOpen = ref(false)
const filters = reactive({ q: '', purpose: '', status: '', work_order_id: 0 })
let localKey = 0
const form = reactive(emptyDocument())

const isDraft = computed(() => !form.id || form.status === 'draft')
const isReceipt = computed(() => form.purpose_key === 'material_receipt')

function emptyItem() {
  return {
    local_key: ++localKey, material_id: 0, product_id: 0, item_type: 'material', item_name: '',
    spec_g: 0, inventory_unit: '', from_warehouse: 'raw_materials', to_warehouse: 'wip',
    qty_g: 0, qty_units: 0, batch_code: '', unit_cost: 0, supplier: '', crop_season: '',
    origin: '', producer_flavor_description: '', allocations: [],
  }
}

function emptyDocument() {
  return {
    id: 0, entry_no: '', status: 'draft', purpose_key: 'material_transfer',
    work_order_id: 0, job_card_id: 0, running_item_id: 0, source_type: '', source_id: 0,
    return_source: '', note: '', items: [emptyItem()],
  }
}

function resetForm(next = emptyDocument()) {
  Object.keys(form).forEach((key) => delete form[key])
  Object.assign(form, next)
}

function normalizedPurpose(value) {
  if (value === 'material_return_from_manufacture') return 'material_transfer_for_manufacture'
  return value
}

function documentPurposeLabel(row) {
  if (row.is_return) return '退回未用原料'
  return stockEntryTypeLabel(row.purpose || row.entry_type)
}

function statusLabel(value) {
  return ({ draft: '草稿', submitted: '已提交', cancelled: '已取消' })[String(value || '')] || value || '-'
}

function quantityLabel(row) {
  return Number(row.total_qty_g || 0) > 0 ? `${Number(row.total_qty_g).toLocaleString('zh-CN')}g` : '-'
}

function itemOptionLabel(row) {
  const name = String(row?.name || row?.Name || '').trim()
  const code = String(row?.code || row?.Code || '').trim()
  return code ? `${name} (${code})` : name || `#${row?.id || ''}`
}

function selectedObjectID(item) {
  return Number(item.item_type === 'material' ? item.material_id : item.product_id) || 0
}

function selectItemObject(item, id) {
  const value = Number(id || 0)
  const options = item.item_type === 'material' ? materials.value : products.value
  const selected = options.find((row) => Number(row.id || 0) === value)
  item.material_id = item.item_type === 'material' ? value : 0
  item.product_id = item.item_type === 'finished_product' ? value : 0
  item.item_name = selected?.name || selected?.Name || ''
  item.inventory_unit = selected?.unit || selected?.inventory_unit || ''
  if (item.item_type === 'finished_product') item.spec_g = Number(selected?.spec_g || item.spec_g || 0)
}

function resetItemObject(item) {
  item.material_id = 0
  item.product_id = 0
  item.item_name = ''
  item.spec_g = item.item_type === 'finished_product' ? 454 : 0
}

function addItem() {
  const item = emptyItem()
  form.items.push(item)
  applyItemDefaults(item)
}

function applyItemDefaults(item) {
  const defaults = {
    material_receipt: ['', 'raw_materials', 'material'],
    material_issue: ['raw_materials', '', 'material'],
    material_transfer: ['raw_materials', 'wip', item.item_type || 'material'],
    material_transfer_for_manufacture: ['raw_materials', 'wip', 'material'],
    material_return_from_manufacture: ['wip', 'raw_materials', 'material'],
    material_consumption_for_manufacture: ['wip', '', 'material'],
    manufacture: ['', 'finished_goods', 'finished_product'],
  }[form.purpose_key] || ['raw_materials', 'wip', 'material']
  item.from_warehouse = defaults[0]
  item.to_warehouse = defaults[1]
  item.item_type = defaults[2]
}

function applyEntryDefaults() {
  form.items.forEach(applyItemDefaults)
}

function openNewDrawer(purpose = '') {
  resetForm(emptyDocument())
  if (purpose) form.purpose_key = purpose
  applyEntryDefaults()
  drawerError.value = ''
  drawerOpen.value = true
}

function closeDrawer() {
  drawerOpen.value = false
  drawerError.value = ''
}

function requestBody() {
  return {
    purpose: normalizedPurpose(form.purpose_key),
    is_return: form.purpose_key === 'material_return_from_manufacture',
    work_order_id: Number(form.work_order_id || 0),
    job_card_id: Number(form.job_card_id || 0),
    running_item_id: Number(form.running_item_id || 0),
    source_type: form.source_type || '',
    source_id: Number(form.source_id || 0),
    return_source: form.return_source || '',
    note: form.note || '',
    items: form.items.map((item) => ({
      material_id: Number(item.material_id || 0), product_id: Number(item.product_id || 0),
      item_type: item.item_type, item_name: item.item_name || '', spec_g: Number(item.spec_g || 0),
      inventory_unit: item.inventory_unit || '', from_warehouse: item.from_warehouse || '',
      to_warehouse: item.to_warehouse || '', qty_g: Number(item.qty_g || 0), qty_units: Number(item.qty_units || 0),
      batch_code: item.batch_code || '', unit_cost: Number(item.unit_cost || 0), supplier: item.supplier || '',
      crop_season: item.crop_season || '', origin: item.origin || '',
      producer_flavor_description: item.producer_flavor_description || '',
    })),
  }
}

async function saveDraft() {
  saving.value = true
  drawerError.value = ''
  try {
    const data = await apiSend(form.id ? `${stockEntryEndpoint()}/${form.id}` : stockEntryEndpoint(), {
      method: form.id ? 'PUT' : 'POST',
      body: requestBody(),
    })
    applyDocument(data)
    await load()
    return data
  } catch (err) {
    drawerError.value = err.message || '保存草稿失败'
    return null
  } finally {
    saving.value = false
  }
}

async function submitDocument() {
  const draft = await saveDraft()
  if (!draft?.id) return
  saving.value = true
  drawerError.value = ''
  try {
    const data = await apiSend(`${stockEntryEndpoint()}/${draft.id}/submit`, { body: {} })
    applyDocument(data)
    await load()
  } catch (err) {
    drawerError.value = err.message || '提交过账失败'
  } finally {
    saving.value = false
  }
}

async function cancelDocument(row) {
  error.value = ''
  try {
    await apiSend(`${stockEntryEndpoint()}/${row.id}/cancel`, { body: {} })
    await load()
  } catch (err) {
    error.value = err.message || '取消失败'
  }
}

function applyDocument(data = {}) {
  resetForm({
    ...emptyDocument(), ...data,
    purpose_key: data.is_return ? 'material_return_from_manufacture' : (data.purpose || data.entry_type || 'material_transfer'),
    items: (data.items || []).map((item) => ({ ...emptyItem(), ...item })),
  })
  drawerOpen.value = true
}

async function openExisting(row) {
  drawerError.value = ''
  try {
    applyDocument(await apiGet(`${stockEntryEndpoint()}/${row.id}`))
  } catch (err) {
    error.value = err.message || '加载单据失败'
  }
}

function queryURL() {
  const url = new URL(stockEntryEndpoint(), window.location.origin)
  if (filters.q) url.searchParams.set('q', filters.q)
  if (filters.purpose) url.searchParams.set('purpose', filters.purpose)
  if (filters.status) url.searchParams.set('status', filters.status)
  if (Number(filters.work_order_id || 0) > 0) url.searchParams.set('work_order_id', String(filters.work_order_id))
  return url
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

async function loadOptions() {
  try {
    const [materialData, productData, warehouseData] = await Promise.all([
      apiGet('/api/materials?limit=500'), apiGet('/api/products'), apiGet('/api/stock/warehouses'),
    ])
    materials.value = materialData.rows || []
    products.value = productData.rows || productData.products || []
    warehouses.value = warehouseData.rows || []
  } catch (err) {
    error.value = err.message || '基础资料加载失败'
  }
}

async function applyViewParams(params = {}) {
  const workOrderID = Number(params.work_order_id || 0)
  filters.work_order_id = workOrderID
  let action = String(params.action || '').trim()
  if (!action && (params.tab === 'wip' || Number(params.shortage_g || 0) > 0)) action = 'issue'
  if (action === 'receipt') {
    openNewDrawer('material_receipt')
    return
  }
  if (!workOrderID || !['issue', 'supplement', 'return', 'consume', 'finish'].includes(action)) return
  drawerError.value = ''
  try {
    const preview = await apiSend(`/api/produce/work-orders/${workOrderID}/stock-document-preview`, {
      body: {
        action, material_id: Number(params.material_id || 0), job_card_id: Number(params.job_card_id || 0),
        running_item_id: Number(params.running_item_id || 0), return_source: 'work_order',
      },
    })
    applyDocument({ ...preview.document, status: 'draft' })
    if (Number(params.shortage_g || 0) > 0 && form.items.length === 1) form.items[0].qty_g = Number(params.shortage_g)
  } catch (err) {
    drawerError.value = err.message || '工单库存预填失败'
    openNewDrawer(action === 'return' ? 'material_return_from_manufacture' : 'material_transfer_for_manufacture')
    form.work_order_id = workOrderID
  }
}

function openWipInventory() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: { key: 'warehouseInventory', params: { warehouse: 'wip', item_type: 'material' } },
  }))
}

watch(() => props.viewParams, (params) => applyViewParams(params || {}), { deep: true })
onMounted(async () => {
  await Promise.all([load(), loadOptions()])
  await applyViewParams(props.viewParams || {})
})
</script>

<style scoped>
.stock-entry-page{padding:16px;display:grid;gap:16px}.stock-entry-page.embedded{padding:0}.panel{border:1px solid #e5e7eb;border-radius:8px;background:#fff;padding:12px}.panel-head,.drawer-head,.line-head{display:flex;justify-content:space-between;align-items:flex-start;gap:12px}.panel-head{margin-bottom:12px}.panel-head h2,.drawer-head h3,.line-head h4{margin:0 0 4px}.panel-head p,.drawer-head p{margin:0;color:#6b7280;font-size:13px}.head-actions,.row-actions,.drawer-actions{display:flex;gap:8px;flex-wrap:wrap}.filters{display:grid;grid-template-columns:minmax(200px,1.5fr) repeat(3,minmax(130px,1fr)) auto;gap:10px;align-items:end}label{min-width:0}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}select,input,button{font:inherit;min-height:36px;border-radius:6px}select,input{width:100%;border:1px solid #d1d5db;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #9ca3af;background:#fff;color:#111}.link,.danger-link{border:0;background:transparent;color:#1d4ed8;padding:0;min-height:0}.danger-link{color:#b91c1c}.disabled{color:#9ca3af}.legacy-readonly{color:#6b7280}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}.table-wrap{overflow:auto}table{width:100%;min-width:1050px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px;vertical-align:top}th{background:#fbfbfb}td small{display:block;color:#6b7280;margin-top:3px}.status{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px}.status.submitted{border-color:#86efac;background:#f0fdf4;color:#15803d}.status.draft{border-color:#fcd34d;background:#fffbeb;color:#a16207}.muted{text-align:center;color:#666}.drawer-mask{position:fixed;inset:0;background:rgba(17,24,39,.35);z-index:80;display:flex;justify-content:flex-end}.drawer{width:min(980px,96vw);height:100%;overflow:auto;background:#fff;padding:18px;box-shadow:-12px 0 32px rgba(15,23,42,.2);display:grid;align-content:start;gap:16px}.drawer-head{border-bottom:1px solid #e5e7eb;padding-bottom:12px}.document-form,.item-grid{display:grid;grid-template-columns:repeat(4,minmax(130px,1fr));gap:10px}.wide{grid-column:span 2}.line-head{align-items:center}.item-card{border:1px solid #e5e7eb;border-radius:8px;padding:12px;display:grid;gap:10px}.allocations{display:flex;flex-wrap:wrap;gap:6px}.allocations span{background:#eff6ff;border:1px solid #bfdbfe;border-radius:999px;padding:3px 8px;font-size:12px}.drawer-actions{justify-content:flex-end;border-top:1px solid #e5e7eb;padding-top:12px}
@media(max-width:900px){.stock-entry-page{padding:12px}.panel-head,.drawer-head{display:grid}.filters,.document-form,.item-grid{grid-template-columns:1fr}.wide{grid-column:auto}.drawer{width:100%}}
</style>
