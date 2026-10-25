<template>
  <div class="stock-operation-page" :class="{ embedded: props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>WIP在制仓</h2>
          <p>把原料领到 WIP，或把未使用原料退回原料仓。</p>
        </div>
        <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已提交：{{ ok }}</div>
      <div v-if="contextSummary" class="context-summary">{{ contextSummary }}</div>
      <div class="operation-grid">
        <label>
          <span>操作</span>
          <select v-model="mode" @change="applyModeWarehouses">
            <option value="issue">领料到WIP</option>
            <option value="return">退回原料仓</option>
          </select>
        </label>
        <label class="span-2">
          <span>物料</span>
          <SearchableSelect
            v-model="form.material_id"
            :options="materials"
            :option-label="materialLabel"
            placeholder="输入物料名称 / 编号"
            empty-text="没有匹配物料"
          />
        </label>
        <label>
          <span>数量(g)</span>
          <input type="number" min="1" step="1" v-model.number="form.qty_g" />
        </label>
        <label>
          <span>来源仓</span>
          <select v-model="form.from_warehouse">
            <option v-for="w in warehouses" :key="w.code" :value="w.code">{{ w.name }}</option>
          </select>
        </label>
        <label>
          <span>目标仓</span>
          <select v-model="form.to_warehouse">
            <option v-for="w in warehouses" :key="w.code" :value="w.code">{{ w.name }}</option>
          </select>
        </label>
        <label class="span-2">
          <span>备注</span>
          <input v-model.trim="form.note" />
        </label>
        <button class="primary" type="button" @click="submit" :disabled="saving">提交</button>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>WIP批次库存</h2>
          <p>查看原料仓和 WIP 在制仓的物料批次位置。</p>
        </div>
        <button class="secondary" type="button" @click="loadLocations" :disabled="loading">刷新库存</button>
      </div>
      <div class="filters">
        <label><span>搜索</span><input v-model.trim="filters.q" placeholder="批次/物料" @keyup.enter="loadLocationPage(1)" /></label>
        <label>
          <span>仓库</span>
          <select v-model="filters.warehouse">
            <option value="wip">WIP在制仓</option>
            <option value="raw_materials">原料仓</option>
            <option value="">全部</option>
          </select>
        </label>
        <button class="primary" type="button" @click="loadLocationPage(1)" :disabled="loading">查询</button>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>仓库</th><th>批次号</th><th>物料</th><th>数量(g)</th><th>入库时间</th><th>更新时间</th><th>核对</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in locations" :key="`${row.material_batch_id}-${row.warehouse}`">
              <td>{{ row.warehouse_name || row.warehouse }}</td>
              <td>{{ row.batch_code }}</td>
              <td>{{ row.material_name }}</td>
              <td>{{ row.qty_g }}</td>
              <td>{{ row.received_at || '-' }}</td>
              <td>{{ row.updated_at || '-' }}</td>
              <td><button class="secondary compact" type="button" @click="openLocationRepair(row)" :disabled="!row.material_batch_id || repairLoading">核对库位</button></td>
            </tr>
            <tr v-if="!locations.length"><td colspan="7" class="muted">暂无库存</td></tr>
          </tbody>
        </table>
      </div>
      <PaginationControls
        :page="locationPage"
        :page-size="locationLimit"
        :total="locationTotal"
        :disabled="loading"
        @change="handleLocationPaginationChange"
      />
    </section>

    <div v-if="repairPreview" class="repair-backdrop" @click.self="closeLocationRepair">
      <section class="repair-dialog" role="dialog" aria-modal="true" aria-label="核对物料批次库位">
        <div class="panel-head">
          <div>
            <h2>核对批次库位</h2>
            <p>{{ repairPreview.batch_code }} · {{ repairPreview.material_name }} · 批次余量 {{ repairPreview.remaining_g }}g / {{ repairPreview.remaining_units }}件</p>
          </div>
          <button class="secondary" type="button" @click="closeLocationRepair" :disabled="repairSaving">关闭</button>
        </div>
        <div class="error repair-error" v-if="repairError">{{ repairError }}</div>
        <p class="repair-warning">请先核对现场实物所在仓。提交只调整系统中的批次库位和关联预留，不会搬动物料。修复后各仓合计必须等于批次余量。</p>
        <h3>修复后的批次库位</h3>
        <div class="repair-rows">
          <div v-for="(location, index) in repairLocations" :key="index" class="repair-row">
            <label><span>仓库</span><select v-model="location.warehouse"><option value="">请选择仓库</option><option v-for="w in repairWarehouses" :key="w.code" :value="w.code">{{ w.name }} · {{ w.code }}</option></select></label>
            <label><span>数量(g)</span><input v-model.number="location.qty_g" type="number" min="0" step="1" /></label>
            <label><span>数量(件)</span><input v-model.number="location.qty_units" type="number" min="0" step="1" /></label>
          </div>
        </div>
        <button class="secondary compact" type="button" @click="addRepairLocation">增加库位</button>
        <div class="repair-reservations" v-if="repairPreview.reservations?.length">
          <h3>关联工单预留</h3>
          <p>预留仍在被删除库位时必须明确选择实际来源仓，否则不能提交。</p>
          <label v-for="binding in repairPreview.reservations" :key="binding.reservation_batch_id" class="reservation-row">
            <span>{{ binding.work_order_no }} · {{ binding.material_name }} · 预留 {{ Math.max(0, binding.reserved_g - binding.consumed_g - binding.returned_g) }}g</span>
            <select v-model="repairReservationMoves[binding.reservation_batch_id]"><option value="">请选择来源仓</option><option v-for="w in repairWarehouses" :key="w.code" :value="w.code">{{ w.name }} · {{ w.code }}</option></select>
          </label>
        </div>
        <label class="repair-reason"><span>修复原因（必填）</span><textarea v-model.trim="repairReason" maxlength="500" placeholder="例如：现场核实批次仅在 WIP 在制仓，原料仓余额为初始化重复记录" /></label>
        <div class="repair-total">修复后合计：{{ repairTotalG }}g / {{ repairTotalUnits }}件</div>
        <div class="drawer-actions">
          <button class="primary" type="button" @click="submitLocationRepair" :disabled="repairSaving || !repairCanSubmit">确认修复库位</button>
          <button class="secondary" type="button" @click="closeLocationRepair" :disabled="repairSaving">取消</button>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import PaginationControls from '../components/PaginationControls.vue'
import SearchableSelect from '../components/SearchableSelect.vue'
import { normalizePageSize, paginationFromApi } from '../lib/pagination'

const props = defineProps({
  embedded: { type: Boolean, default: false },
  viewParams: { type: Object, default: () => ({}) },
})

const materials = ref([])
const warehouses = ref([])
const locations = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref('')
const mode = ref('issue')
const form = reactive({
  material_id: 0,
  from_warehouse: 'raw_materials',
  to_warehouse: 'wip',
  qty_g: 0,
  note: '',
})
const filters = reactive({ q: '', warehouse: 'wip' })
const locationPage = ref(1)
const locationLimit = ref(50)
const locationTotal = ref(0)
const repairPreview = ref(null)
const repairLocations = ref([])
const repairWarehouses = ref([])
const repairReservationMoves = ref({})
const repairReason = ref('')
const repairError = ref('')
const repairLoading = ref(false)
const repairSaving = ref(false)
const repairTotalG = computed(() => repairLocations.value.reduce((total, row) => total + Math.max(0, Number(row.qty_g || 0)), 0))
const repairTotalUnits = computed(() => repairLocations.value.reduce((total, row) => total + Math.max(0, Number(row.qty_units || 0)), 0))
const repairHasDuplicateWarehouse = computed(() => {
  const selected = repairLocations.value.map((row) => row.warehouse).filter(Boolean)
  return new Set(selected).size !== selected.length
})
const repairCanSubmit = computed(() => {
  const preview = repairPreview.value
  if (!preview || !repairReason.value || repairLocations.value.length === 0) return false
  if (repairTotalG.value !== Number(preview.remaining_g || 0) || repairTotalUnits.value !== Number(preview.remaining_units || 0)) return false
  if (repairHasDuplicateWarehouse.value) return false
  const allowedWarehouses = new Set(repairWarehouses.value.map((warehouse) => warehouse.code))
  if (repairLocations.value.some((row) => !allowedWarehouses.has(row.warehouse) || !Number.isInteger(Number(row.qty_g || 0)) || !Number.isInteger(Number(row.qty_units || 0)) || Number(row.qty_g || 0) < 0 || Number(row.qty_units || 0) < 0)) return false
  return (preview.reservations || []).every((row) => repairReservationMoves.value[row.reservation_batch_id] && repairLocations.value.some((location) => location.warehouse === repairReservationMoves.value[row.reservation_batch_id]))
})
const contextSummary = computed(() => {
  const params = props.viewParams || {}
  const parts = [
    params.work_order_id ? `工单 #${params.work_order_id}` : '',
    params.job_card_id ? `工序卡 #${params.job_card_id}` : '',
    params.running_item_id ? `生产中 #${params.running_item_id}` : '',
    params.shortage_g ? `缺口 ${params.shortage_g}g` : '',
  ].filter(Boolean)
  return parts.length ? `生产上下文：${parts.join(' / ')}` : ''
})

function materialLabel(row) {
  const name = String(row?.name || row?.Name || '').trim()
  const code = String(row?.code || row?.Code || '').trim()
  return code ? `${name} (${code})` : name
}

function materialKind(row) {
  return String(row?.kind || row?.Kind || '').trim().toLowerCase()
}

function applyModeWarehouses() {
  if (mode.value === 'return') {
    form.from_warehouse = 'wip'
    form.to_warehouse = 'raw_materials'
    return
  }
  form.from_warehouse = 'raw_materials'
  form.to_warehouse = 'wip'
}

function applyViewParams() {
  const params = props.viewParams || {}
  if (params.tab && params.tab !== 'wip') return
  mode.value = 'issue'
  form.from_warehouse = 'raw_materials'
  form.to_warehouse = 'wip'
  const materialID = Number(params.material_id || 0)
  const shortageG = Number(params.shortage_g || params.gap_g || 0)
  if (materialID > 0) form.material_id = materialID
  if (shortageG > 0) form.qty_g = shortageG
  const noteParts = [
    params.work_order_id ? `work_order_id=${params.work_order_id}` : '',
    params.job_card_id ? `job_card_id=${params.job_card_id}` : '',
    params.running_item_id ? `running_item_id=${params.running_item_id}` : '',
  ].filter(Boolean)
  if (noteParts.length) form.note = `生产补料：${noteParts.join('，')}`
  filters.warehouse = 'wip'
}

async function loadOptions() {
  const [mat, wh] = await Promise.all([
    apiGet('/api/materials?limit=500'),
    apiGet('/api/stock/warehouses'),
  ])
  materials.value = (mat.rows || []).filter((m) => materialKind(m) !== 'pack')
  warehouses.value = wh.rows || []
}

async function loadLocations() {
  const url = new URL('/api/stock/material-batch-locations', window.location.origin)
  if (filters.q) url.searchParams.set('q', filters.q)
  if (filters.warehouse) url.searchParams.set('warehouse', filters.warehouse)
  url.searchParams.set('active_only', '1')
  url.searchParams.set('page', String(locationPage.value))
  url.searchParams.set('limit', String(locationLimit.value))
  const data = await apiGet(url)
  const pagination = paginationFromApi(data)
  locations.value = data.rows || []
  locationPage.value = pagination.page
  locationLimit.value = pagination.pageSize
  locationTotal.value = pagination.total
}

async function loadLocationPage(nextPage) {
  locationPage.value = Math.max(1, Number(nextPage || 1))
  await loadLocations()
}

async function openLocationRepair(row) {
  const batchID = Number(row?.material_batch_id || 0)
  if (batchID <= 0) return
  repairLoading.value = true
  repairError.value = ''
  try {
    const preview = await apiSend('/api/stock/material-batch-locations/repair-preview', { body: { material_batch_id: batchID } })
    const warehouses = await apiGet(`/api/stock/warehouses?customer_id=${Number(preview.owner_customer_id || 0)}`)
    repairPreview.value = preview
    repairWarehouses.value = (warehouses.rows || []).filter((warehouse) => warehouse.active !== false)
    repairLocations.value = (preview.locations || []).map((location) => ({ ...location }))
    if (!repairLocations.value.length) repairLocations.value.push({ warehouse: '', qty_g: 0, qty_units: 0 })
    repairReservationMoves.value = Object.fromEntries((preview.reservations || []).map((binding) => [binding.reservation_batch_id, binding.warehouse]))
    repairReason.value = ''
  } catch (err) {
    repairError.value = err.message || '批次库位预览失败'
    repairPreview.value = { batch_code: row.batch_code || '', material_name: row.material_name || '', remaining_g: 0, remaining_units: 0, locations: [], reservations: [] }
  } finally {
    repairLoading.value = false
  }
}

function addRepairLocation() {
  repairLocations.value.push({ warehouse: '', qty_g: 0, qty_units: 0 })
}

function closeLocationRepair() {
  if (repairSaving.value) return
  repairPreview.value = null
  repairError.value = ''
}

async function submitLocationRepair() {
  if (!repairCanSubmit.value) return
  const preview = repairPreview.value
  const requestID = globalThis.crypto?.randomUUID?.() || `location-repair-${Date.now()}-${Math.random().toString(16).slice(2)}`
  const body = {
    material_batch_id: preview.material_batch_id,
    expected_version: preview.version,
    request_id: requestID,
    reason: repairReason.value,
    locations: repairLocations.value.map((row) => ({ warehouse: row.warehouse, qty_g: Number(row.qty_g || 0), qty_units: Number(row.qty_units || 0) })),
    reservation_moves: (preview.reservations || []).map((row) => ({ reservation_batch_id: row.reservation_batch_id, warehouse: repairReservationMoves.value[row.reservation_batch_id] })),
  }
  if (!window.confirm(`确认将批次 ${preview.batch_code} 的系统库位调整为 ${repairTotalG.value}g / ${repairTotalUnits.value}件吗？请确保已按现场实物核对。`)) return
  repairSaving.value = true
  repairError.value = ''
  try {
    const result = await apiSend('/api/stock/material-batch-locations/repair', { body })
    ok.value = `${result.batch_code} 库位已核对，调整预留 ${result.moved_reservations} 项`
    closeLocationRepair()
    await loadLocations()
  } catch (err) {
    repairError.value = err.message || '库位修复失败，请刷新预览后重试'
  } finally {
    repairSaving.value = false
  }
}

function handleLocationPaginationChange({ page, pageSize }) {
  locationLimit.value = normalizePageSize(pageSize)
  loadLocationPage(page)
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    await loadOptions()
    applyViewParams()
    await loadLocations()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function submit() {
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiSend('/api/stock/material-transfers', { body: form })
    ok.value = data.transfer_no
    form.qty_g = 0
    form.note = ''
    await loadLocations()
  } catch (err) {
    error.value = err.message || '提交失败'
  } finally {
    saving.value = false
  }
}

watch(() => props.viewParams, applyViewParams, { deep: true })

onMounted(loadAll)
</script>

<style scoped>
.stock-operation-page { padding:16px; display:grid; gap:16px; }
.stock-operation-page.embedded { padding:0; }
.panel { border:1px solid #e5e7eb; border-radius:8px; padding:12px; background:#fff; }
.panel-head { display:flex; justify-content:space-between; align-items:flex-start; gap:12px; margin-bottom:12px; }
.panel-head h2 { margin:0 0 4px; font-size:18px; }
.panel-head p { margin:0; color:#6b7280; font-size:13px; }
.operation-grid { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:10px; align-items:end; }
.filters { display:grid; grid-template-columns:minmax(220px,1fr) 150px 90px; gap:10px; align-items:end; margin-bottom:12px; }
.span-2 { grid-column:span 2; }
label { min-width:0; position:relative; }
label span { display:block; color:#666; font-size:12px; margin-bottom:5px; }
input, select, button { font:inherit; min-height:38px; border-radius:6px; }
input, select { width:100%; border:1px solid #d1d5db; padding:7px 9px; }
button { padding:8px 12px; cursor:pointer; }
button:disabled { cursor:not-allowed; opacity:.6; }
.primary { border:1px solid #111; background:#111; color:#fff; }
.secondary { border:1px solid #9ca3af; background:#fff; color:#111; }
.table-wrap { overflow:auto; }
table { width:100%; min-width:980px; border-collapse:collapse; }
th, td { border-bottom:1px solid #f0f0f0; padding:8px; text-align:left; font-size:13px; }
th { background:#fbfbfb; }
.muted { color:#666; text-align:center; }
.error, .ok { border-radius:8px; padding:10px; margin-bottom:12px; }
.error { background:#ffecec; border:1px solid #ffb9b9; }
.ok { background:#e9ffe9; border:1px solid #b8f5b8; }
.context-summary { border:1px solid #bfdbfe; border-radius:8px; background:#eff6ff; color:#1d4ed8; padding:8px 10px; margin-bottom:12px; font-size:13px; }
.compact { min-height:32px; padding:5px 9px; }
.repair-backdrop { position:fixed; inset:0; z-index:130; display:flex; justify-content:flex-end; background:rgba(15,23,42,.42); }
.repair-dialog { width:min(720px,100vw); height:100%; overflow:auto; padding:20px; background:#fff; box-shadow:-12px 0 32px rgba(15,23,42,.18); }
.repair-dialog h2 { margin:0 0 5px; font-size:20px; }
.repair-dialog h3 { margin:18px 0 8px; font-size:15px; }
.repair-warning { padding:10px 12px; border-radius:8px; color:#854d0e; background:#fffbeb; font-size:13px; }
.repair-rows { display:grid; gap:8px; }
.repair-row { display:grid; grid-template-columns:1.5fr 1fr 1fr; gap:8px; }
.repair-reservations { margin-top:14px; }
.repair-reservations p { color:#667085; font-size:13px; }
.reservation-row { display:grid; grid-template-columns:1fr 1fr; gap:10px; align-items:center; padding:8px 0; border-top:1px solid #eee; }
.repair-reason { display:block; margin-top:16px; }
.repair-reason textarea { width:100%; min-height:74px; border:1px solid #d1d5db; border-radius:6px; padding:8px; font:inherit; }
.repair-total { position:sticky; bottom:0; padding:12px 0; background:#fff; font-weight:700; }
.repair-error { margin:10px 0; }
.drawer-actions { display:flex; gap:8px; margin-top:10px; }
@media (max-width:900px){ .stock-operation-page{padding:12px;} .operation-grid,.filters{grid-template-columns:1fr;} .span-2{grid-column:auto;} }
@media (max-width:560px){ .repair-row,.reservation-row{grid-template-columns:1fr;} .repair-dialog{padding:14px;} }
</style>
