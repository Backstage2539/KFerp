<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>仓库库存</h2>
          <p>按仓库查看原料、包材、WIP 和成品库存，批次作为明细维度展开。</p>
        </div>
        <div class="head-actions">
          <button v-if="!isCustomerInventoryContext" class="secondary" type="button" @click="openReservationDrawer">WIP占用</button>
          <button v-if="!isCustomerInventoryContext" class="secondary" type="button" @click="openTraceDrawer('')">批次追溯</button>
          <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label><span>搜索</span><input v-model.trim="q" placeholder="物品/批次" @keyup.enter="loadInventoryPage(1)" /></label>
        <label>
          <span>类型</span>
          <select v-model="itemType" @change="loadInventoryPage(1)">
            <option value="">全部</option>
            <option value="material">原料/包材</option>
            <option value="finished_product">成品</option>
          </select>
        </label>
        <button class="primary" type="button" @click="loadInventoryPage(1)" :disabled="loading">查询</button>
      </div>
    </section>

    <div class="workspace">
      <aside class="panel warehouse-panel">
        <div class="panel-title">仓库</div>
        <BusinessGroupControls
          v-model="selectedWarehouseGroupTemplateID"
          v-model:move-model-value="selectedWarehouseMoveGroupItemID"
          class="warehouse-group-controls"
          data-pr442-warehouse-business-groups
          :template-options="warehouseGroupTemplateOptions"
          :move-options="warehouseGroupItemOptions"
          :selected-template="selectedWarehouseGroupTemplate"
          :selected-count="selectedWarehouseRowsForMove.length"
          :can-move="canMoveSelectedWarehouses"
          :can-select-target="canSelectWarehouseMoveTarget"
          :loading="loading"
          @manage="openWarehouseBusinessGroupManagement"
          @move="moveSelectedWarehousesToGroup" />
        <button class="warehouse" :class="{ active: selectedWarehouse === '' }" type="button" @click="selectWarehouse('')">
          <strong>全部仓库</strong>
          <small>跨仓库汇总查询</small>
        </button>
        <div class="warehouse-group-list">
          <template v-for="group in warehouseDisplayGroups" :key="group.key">
            <button
              class="warehouse-section-toggle"
              type="button"
              @click="toggleWarehouseGroup(group.key)">
              <span>{{ group.label }}</span><b>{{ group.rows.length }}</b>
            </button>
            <template v-if="!isWarehouseGroupCollapsed(group.key)">
              <p v-if="!group.rows.length" class="warehouse-empty-row">暂无仓库</p>
              <div
                v-for="row in group.rows"
                :key="`${group.key}-${row.code}`"
                class="warehouse-row"
                :style="businessGroupItemIndentStyle(group)">
                <label class="warehouse-select">
                  <input type="checkbox" :checked="isWarehouseSelected(row)" @change="toggleWarehouseSelection(row, $event.target.checked)" />
                </label>
                <button
                  class="warehouse"
                  :class="{ active: selectedWarehouse === row.code }"
                  type="button"
                  @click="selectWarehouse(row.code)">
                  <strong>{{ row.name }}</strong>
                  <small>{{ kindLabel(row.kind) }} · {{ row.description || row.code }}</small>
                  <small v-if="row.customer_name">绑定客户：{{ row.customer_name }}</small>
                </button>
              </div>
            </template>
          </template>
        </div>
      </aside>

      <section class="panel table-panel">
        <div class="summary">
          <div><span>当前仓库</span><strong>{{ currentWarehouseName }}</strong></div>
          <div><span>库存行</span><strong>{{ rows.length }}</strong></div>
          <div><span>合计重量(g)</span><strong>{{ totalG.toLocaleString('zh-CN') }}</strong></div>
          <div v-if="!isCustomerInventoryContext" class="summary-action">
            <span>仓库设置</span>
            <button class="secondary" type="button" :disabled="!selectedWarehouse" @click="openWarehouseSettingsDrawer">
              仓库设置
            </button>
          </div>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>仓库</th>
                <th>类型</th>
                <th>物品</th>
                <th>规格</th>
                <th>批次</th>
                <th>质检</th>
                <th>库存数量</th>
                <th>单位成本</th>
                <th>更新</th>
                <th v-if="!isCustomerInventoryContext">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in rows" :key="rowKey(row)">
                <td>{{ row.warehouse_name || row.warehouse }}</td>
                <td><span class="pill">{{ typeLabel(row.item_type) }}</span></td>
                <td>{{ row.item_name }}</td>
                <td>{{ row.spec_g ? `${row.spec_g}g` : '-' }}</td>
                <td>{{ row.batch_code || '-' }}</td>
                <td><span class="quality-pill" :class="qualityClass(row.quality_status)">{{ qualityLabel(row.quality_status) }}</span></td>
                <td>{{ inventoryQtyLabel(row) }}</td>
                <td>{{ money(row.unit_cost) }}</td>
                <td>{{ row.updated_at || '-' }}</td>
                <td v-if="!isCustomerInventoryContext"><button class="link" type="button" @click="openTraceDrawer(row.batch_code || '')">追溯</button></td>
              </tr>
              <tr v-if="!rows.length"><td :colspan="isCustomerInventoryContext ? 9 : 10" class="muted">暂无库存</td></tr>
            </tbody>
          </table>
        </div>
        <PaginationControls
          :page="page"
          :page-size="limit"
          :total="total"
          :disabled="loading"
          @change="handleInventoryPaginationChange"
        />
      </section>
    </div>

    <div v-if="warehouseSettingsDrawerOpen && !isCustomerInventoryContext" class="drawer-mask warehouse-settings-drawer" @click.self="warehouseSettingsDrawerOpen = false">
      <aside class="drawer">
        <div class="drawer-head">
          <h3>仓库设置</h3>
          <button class="secondary" type="button" @click="warehouseSettingsDrawerOpen = false">关闭</button>
        </div>
        <div class="settings-form">
          <label>
            <span>当前仓库</span>
            <input :value="currentWarehouseName" disabled />
          </label>
          <template v-if="isExternalWarehouse">
            <label>
              <span>绑定客户</span>
              <div class="customer-search-wrap">
                <input
                  v-model.trim="customerSearch"
                  placeholder="输入客户名/手机号/公司名搜索"
                  autocomplete="off"
                  @input="filterCustomerOptions"
                />
                <div v-if="customerSearch" class="customer-search-results">
                  <button
                    v-for="customer in filteredCustomerOptions"
                    :key="customer.id"
                    type="button"
                    class="customer-option"
                    @click="selectCustomerForBinding(customer)"
                  >
                    {{ customerOptionLabel(customer) }}
                  </button>
                  <div v-if="!filteredCustomerOptions.length" class="muted">没有匹配客户</div>
                </div>
              </div>
            </label>
            <p class="muted setting-note">绑定客户后，只有该客户可查看此外部库存。</p>
          </template>
          <div class="binding-status" v-if="warehouseBindCustomerID">
            已绑定客户：{{ selectedBindCustomerName }}
            <button class="link" type="button" @click="clearCustomerBinding">取消绑定</button>
          </div>
          <button class="primary" type="button" @click="saveWarehouseSettings" :disabled="warehouseBindingSaving">
            {{ warehouseBindingSaving ? '保存中' : '保存仓库设置' }}
          </button>
        </div>
      </aside>
    </div>

    <div v-if="traceDrawerOpen && !isCustomerInventoryContext" class="drawer-mask" @click.self="traceDrawerOpen = false">
      <aside class="drawer">
        <div class="drawer-head">
          <h3>批次追溯</h3>
          <button class="secondary" type="button" @click="traceDrawerOpen = false">关闭</button>
        </div>
        <div class="trace-search">
          <label><span>批次号</span><input v-model.trim="traceBatch" placeholder="FP-0000000042 / MB-0000000002 / LEGACY-MAT-0000000001" @keyup.enter="loadTrace" /></label>
          <button class="primary" type="button" @click="loadTrace" :disabled="traceLoading">查询</button>
        </div>
        <div v-if="traceError" class="error">{{ traceError }}</div>
        <div v-if="traceResult" class="trace-block">
          <template v-if="traceResult.trace_type === 'material_batch'">
            <div class="trace-title">{{ traceResult.material_batch?.material_name || '-' }}</div>
            <div v-if="isLegacyMaterialBatch(traceResult.material_batch?.batch_code)" class="tip">
              LEGACY-MAT 是系统升级时按物料现有库存生成的期初原料批次，用来让旧库存可以分仓、转入 WIP 和继续生产扣减。
            </div>
            <dl>
              <div><dt>原料批次</dt><dd>{{ traceResult.material_batch?.batch_code || '-' }}</dd></div>
              <div><dt>质检状态</dt><dd><span class="quality-pill" :class="qualityClass(traceResult.material_batch?.quality_status)">{{ qualityLabel(traceResult.material_batch?.quality_status) }}</span></dd></div>
              <div><dt>供应商</dt><dd>{{ traceResult.material_batch?.supplier || '-' }}</dd></div>
              <div><dt>入库单</dt><dd>{{ traceResult.material_batch?.receipt_id || '-' }}</dd></div>
              <div><dt>数量</dt><dd>{{ traceMaterialBatchQtyLabel(traceResult.material_batch) }}</dd></div>
              <div><dt>备注</dt><dd>{{ traceResult.material_batch?.note || '-' }}</dd></div>
            </dl>
            <h4>当前仓库位置</h4>
            <table class="trace-table">
              <thead><tr><th>仓库</th><th>批次</th><th>质检</th><th>数量</th></tr></thead>
              <tbody>
                <tr v-for="item in traceResult.material_locations || []" :key="`${item.material_batch_id}-${item.warehouse}`">
                  <td>{{ item.warehouse_name || warehouseName(item.warehouse) }}</td>
                  <td>{{ item.batch_code }}</td>
                  <td><span class="quality-pill" :class="qualityClass(item.quality_status)">{{ qualityLabel(item.quality_status) }}</span></td>
                  <td>{{ inventoryQtyLabel({ ...item, item_type: 'material' }) }}</td>
                </tr>
                <tr v-if="!(traceResult.material_locations || []).length"><td colspan="4" class="muted">暂无仓库库存</td></tr>
              </tbody>
            </table>
          </template>
          <template v-else>
            <div class="trace-title">{{ traceResult.finished_batch?.product_name || '-' }}</div>
            <dl>
              <div><dt>成品批次</dt><dd>{{ traceResult.finished_batch?.batch_code || '-' }}</dd></div>
              <div><dt>质检状态</dt><dd><span class="quality-pill" :class="qualityClass(traceResult.finished_batch?.quality_status)">{{ qualityLabel(traceResult.finished_batch?.quality_status) }}</span></dd></div>
              <div><dt>工单</dt><dd>{{ traceResult.production?.work_order_no || '-' }}</dd></div>
              <div><dt>生产批次</dt><dd>{{ traceResult.production?.batch_id || '-' }}</dd></div>
              <div><dt>入库仓</dt><dd>{{ warehouseName(traceResult.finished_batch?.warehouse) }}</dd></div>
              <div><dt>产出</dt><dd>{{ traceResult.finished_batch?.qty_g || 0 }}g / {{ traceResult.finished_batch?.qty_units || 0 }} 件</dd></div>
            </dl>
            <h4>消耗原料批次</h4>
            <table class="trace-table">
              <thead><tr><th>原料</th><th>批次</th><th>扣减</th></tr></thead>
              <tbody>
                <tr v-for="item in traceResult.materials || []" :key="`${item.material_id}-${item.material_batch_code}`">
                  <td>{{ item.material_name }}</td>
                  <td>{{ item.material_batch_code || '-' }}</td>
                  <td>{{ item.deduct_g ? `${item.deduct_g}g` : `${item.deduct_units}件` }}</td>
                </tr>
                <tr v-if="!(traceResult.materials || []).length"><td colspan="3" class="muted">暂无消耗记录</td></tr>
              </tbody>
            </table>
          </template>
        </div>
      </aside>
    </div>

    <div v-if="reservationDrawerOpen && !isCustomerInventoryContext" class="drawer-mask" @click.self="reservationDrawerOpen = false">
      <aside class="drawer wide">
        <div class="drawer-head">
          <h3>WIP占用</h3>
          <button class="secondary" type="button" @click="reservationDrawerOpen = false">关闭</button>
        </div>
        <div class="trace-search">
          <label><span>工单号</span><input v-model.trim="reservationWorkOrderNo" placeholder="WO-0000000020" @keyup.enter="loadReservations" /></label>
          <button class="primary" type="button" @click="loadReservations" :disabled="reservationLoading">查询</button>
        </div>
        <div v-if="reservationError" class="error">{{ reservationError }}</div>
        <div class="summary reservation-summary">
          <div><span>占用行</span><strong>{{ reservations.length }}</strong></div>
          <div><span>剩余占用(g)</span><strong>{{ Number(reservationTotals.total_remaining_g || 0).toLocaleString('zh-CN') }}</strong></div>
          <div><span>已消耗(g)</span><strong>{{ Number(reservationTotals.total_consumed_g || 0).toLocaleString('zh-CN') }}</strong></div>
        </div>
        <table class="trace-table reservation-table">
          <thead><tr><th>工单/物料</th><th>占用</th><th>调整为(g)</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="row in reservations" :key="row.id">
              <td><strong>{{ row.work_order_no }}</strong><small>{{ row.material_name }}</small></td>
              <td>
                <strong>{{ Number(row.remaining_reserved_g || 0).toLocaleString('zh-CN') }}g</strong>
                <small>已占 {{ row.reserved_g || 0 }}g / 已耗 {{ row.consumed_g || 0 }}g</small>
                <small>WIP {{ row.wip_g || 0 }}g / 可用 {{ row.available_g || 0 }}g</small>
              </td>
              <td><input v-model.number="row.adjust_reserved_g" type="number" min="0" step="1" /></td>
              <td>
                <button class="link" type="button" @click="adjustReservation(row)">调整</button>
                <button class="link danger" type="button" @click="releaseReservation(row)">释放</button>
              </td>
            </tr>
            <tr v-if="!reservations.length"><td colspan="4" class="muted">暂无WIP占用</td></tr>
          </tbody>
        </table>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import BusinessGroupControls from '../components/BusinessGroupControls.vue'
import PaginationControls from '../components/PaginationControls.vue'
import {
  businessGroupControlOptions,
  businessGroupItemIndentStyle,
  businessGroupMoveAssignmentPayload,
  groupRowsByBusinessGroupTemplate,
  preferredBusinessGroupTemplateID,
} from '../lib/business-grouping'
import { normalizePageSize, paginationFromApi } from '../lib/pagination'
import { CUSTOMER_WORKSPACE_MODE } from '../lib/workspace-mode'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})

const warehouses = ref([])
const warehouseBusinessGroups = ref([])
const customerOptions = ref([])
const rows = ref([])
const q = ref('')
const itemType = ref('')
const selectedWarehouse = ref('')
const selectedWarehouseGroupTemplateID = ref(0)
const selectedWarehouseMoveGroupItemID = ref(0)
const selectedWarehouseKeys = ref([])
const collapsedWarehouseGroups = ref([])
const page = ref(1)
const limit = ref(50)
const total = ref(0)
const loading = ref(false)
const error = ref('')
const traceDrawerOpen = ref(false)
const traceBatch = ref('')
const traceResult = ref(null)
const traceLoading = ref(false)
const traceError = ref('')
const reservationDrawerOpen = ref(false)
const reservationLoading = ref(false)
const reservationError = ref('')
const reservationWorkOrderNo = ref('')
const reservations = ref([])
const reservationTotals = ref({})
const warehouseBindCustomerID = ref(0)
const warehouseBindingSaving = ref(false)
const warehouseSettingsDrawerOpen = ref(false)
const customerSearch = ref('')
const contextCustomerID = computed(() => Number(props.customerContextId || 0))
const isCustomerInventoryContext = computed(() => props.workspaceMode === CUSTOMER_WORKSPACE_MODE && contextCustomerID.value > 0)

const currentWarehouseName = computed(() => {
  if (!selectedWarehouse.value) return '全部仓库'
  return warehouses.value.find((row) => row.code === selectedWarehouse.value)?.name || selectedWarehouse.value
})
const selectedWarehouseRow = computed(() => warehouses.value.find((row) => row.code === selectedWarehouse.value) || null)
const isExternalWarehouse = computed(() => {
  const row = selectedWarehouseRow.value
  return row && (String(row.kind || '') === 'external' || String(row.kind || '') === 'customer_processing')
})
const selectedBindCustomerName = computed(() => {
  const id = Number(warehouseBindCustomerID.value || 0)
  if (!id) return ''
  const customer = customerOptions.value.find((c) => Number(c.id) === id)
  return customer ? customerOptionLabel(customer) : ''
})
const totalG = computed(() => rows.value.reduce((sum, row) => sum + Number(row.qty_g || 0), 0))
function inventoryQtyLabel(row) {
  const qtyG = Number(row?.qty_g || 0)
  const qtyUnits = Number(row?.qty_units || 0)
  if (qtyUnits && qtyG) return `${qtyUnits.toLocaleString('zh-CN')} 件 / ${qtyG.toLocaleString('zh-CN')}g`
  if (qtyUnits) return `${qtyUnits.toLocaleString('zh-CN')} ${row?.item_type === 'finished_product' ? '件' : '库存单位'}`
  if (qtyG) return `${qtyG.toLocaleString('zh-CN')}g`
  return '-'
}
function traceMaterialBatchQtyLabel(row) {
  if (!row) return '-'
  return `${inventoryQtyLabel({ qty_g: row.qty_g, qty_units: row.qty_units, item_type: 'material' })} / 剩余 ${inventoryQtyLabel({ qty_g: row.remaining_g, qty_units: row.remaining_units, item_type: 'material' })}`
}
const warehouseBusinessGroupControls = computed(() => businessGroupControlOptions(warehouseBusinessGroups.value, {
  selectedTemplateID: selectedWarehouseGroupTemplateID.value,
  usageKey: 'warehouse_inventory',
}))
const warehouseGroupTemplateOptions = computed(() => warehouseBusinessGroupControls.value.templateOptions)
const selectedWarehouseGroupTemplate = computed(() => warehouseBusinessGroupControls.value.selectedTemplate)
const warehouseGroupItemOptions = computed(() => warehouseBusinessGroupControls.value.moveOptions)
const warehouseDisplayGroups = computed(() => groupRowsByBusinessGroupTemplate(warehouses.value, {
  template: selectedWarehouseGroupTemplate.value,
  usageKey: 'warehouse_inventory',
  objectKey: 'warehouse',
  objectRefForRow: (row) => String(row.code || '').trim(),
  rowAssignment: warehouseBusinessGroupAssignment,
}))
const selectedWarehouseRowsForMove = computed(() => {
  const selected = new Set(selectedWarehouseKeys.value)
  return warehouses.value.filter((row) => selected.has(warehouseRowKey(row)))
})
const canMoveSelectedWarehouses = computed(() => {
  if (!selectedWarehouseRowsForMove.value.length) return false
  const targetGroupItemID = Number(selectedWarehouseMoveGroupItemID.value || 0)
  const targetOption = targetGroupItemID > 0 ? warehouseGroupOptionByItemID(targetGroupItemID) : null
  if (targetGroupItemID > 0 && !targetOption) return false
  return selectedWarehouseRowsForMove.value.some((row) => {
    if (!targetOption) return warehouseGroupID(row) > 0 || warehouseGroupItemID(row) > 0
    return warehouseGroupID(row) !== Number(targetOption.group_id || 0) || warehouseGroupItemID(row) !== Number(targetOption.group_item_id || 0)
  })
})
const canSelectWarehouseMoveTarget = computed(() => Boolean(selectedWarehouseGroupTemplate.value && selectedWarehouseRowsForMove.value.length))

function kindLabel(kind) {
  return {
    raw: '原料仓',
    packaging: '包材仓',
    wip: 'WIP在制仓',
    finished: '成品仓',
    loss: '损耗仓',
    external: '外部库存',
    customer_processing: '外部库存',
  }[kind] || kind || '仓库'
}

function typeLabel(type) {
  return type === 'finished_product' ? '成品' : '原料/包材'
}

function qualityLabel(status) {
  return {
    pass: '通过',
    hold: '待处理',
    reject: '不通过',
    unchecked: '未检',
  }[status || 'unchecked'] || '未检'
}

function qualityClass(status) {
  return `quality-${status || 'unchecked'}`
}

function money(value) {
  return Number(value || 0).toFixed(2)
}

function warehouseName(code) {
  if (!code) return '-'
  return warehouses.value.find((row) => row.code === code)?.name || code
}

function customerOptionLabel(customer) {
  const name = customer?.name || `客户 #${customer?.id || ''}`
  return customer?.company_name && customer.company_name !== name ? `${name} · ${customer.company_name}` : name
}

function rowKey(row) {
  return `${row.warehouse}-${row.item_type}-${row.item_id}-${row.spec_g || 0}-${row.batch_id || row.batch_code || 'summary'}`
}

function warehouseRowKey(row = {}) {
  return String(row.code || '').trim()
}

function warehouseGroupID(row = {}) {
  return Number(row.business_group_id ?? row.group_id ?? 0) || 0
}

function warehouseGroupItemID(row = {}) {
  return Number(row.group_item_id ?? row.business_group_item_id ?? 0) || 0
}

function warehouseBusinessGroupAssignment(row = {}) {
  return {
    usage_key: 'warehouse_inventory',
    object_key: 'warehouse',
    object_ref: warehouseRowKey(row),
    group_id: warehouseGroupID(row),
    group_item_id: warehouseGroupItemID(row),
  }
}

function warehouseGroupOptionByItemID(groupItemID) {
  const id = Number(groupItemID || 0)
  return warehouseGroupItemOptions.value.find((option) => Number(option.group_item_id || 0) === id) || null
}

function preferredWarehouseGroupTemplateID() {
  return preferredBusinessGroupTemplateID(warehouseBusinessGroups.value, {
    selectedTemplateID: selectedWarehouseGroupTemplateID.value,
    usageKey: 'warehouse_inventory',
    preferredNames: ['库存分组', '仓库库存'],
    preferredNameIncludes: ['库存', '仓库'],
  })
}

function isWarehouseGroupCollapsed(key) {
  return collapsedWarehouseGroups.value.includes(String(key || ''))
}

function toggleWarehouseGroup(key) {
  const groupKey = String(key || '')
  if (!groupKey) return
  collapsedWarehouseGroups.value = isWarehouseGroupCollapsed(groupKey)
    ? collapsedWarehouseGroups.value.filter((item) => item !== groupKey)
    : [...collapsedWarehouseGroups.value, groupKey]
}

function isWarehouseSelected(row = {}) {
  const key = warehouseRowKey(row)
  return Boolean(key && selectedWarehouseKeys.value.includes(key))
}

function toggleWarehouseSelection(row = {}, checked = false) {
  const key = warehouseRowKey(row)
  if (!key) return
  const next = new Set(selectedWarehouseKeys.value)
  if (checked) next.add(key)
  else next.delete(key)
  selectedWarehouseKeys.value = [...next]
}

function selectWarehouse(code) {
  selectedWarehouse.value = code
  syncWarehouseBinding()
  loadInventoryPage(1)
}

function applyViewParams(params = {}) {
  const nextWarehouse = typeof params.warehouse === 'string' ? params.warehouse : ''
  const nextItemType = typeof params.item_type === 'string' ? params.item_type : ''
  const nextBatch = typeof params.batch === 'string' ? params.batch : ''
  const changed = nextWarehouse !== selectedWarehouse.value || nextItemType !== itemType.value
  selectedWarehouse.value = nextWarehouse
  syncWarehouseBinding()
  itemType.value = nextItemType
  if (nextBatch && !isCustomerInventoryContext.value) {
    traceBatch.value = nextBatch
    traceDrawerOpen.value = true
  }
  if (changed) loadInventoryPage(1)
}

async function loadWarehouses() {
  const url = new URL('/api/stock/warehouses', window.location.origin)
  if (isCustomerInventoryContext.value) url.searchParams.set('customer_id', String(contextCustomerID.value))
  const data = await apiGet(url)
  warehouses.value = data.rows || []
  syncWarehouseBinding()
}

async function loadWarehouseBusinessGroups() {
  if (isCustomerInventoryContext.value) {
    warehouseBusinessGroups.value = []
    return
  }
  const data = await apiGet('/api/business-groups')
  warehouseBusinessGroups.value = Array.isArray(data?.rows) ? data.rows : (Array.isArray(data) ? data : [])
  const nextTemplateID = preferredWarehouseGroupTemplateID()
  if (nextTemplateID && nextTemplateID !== Number(selectedWarehouseGroupTemplateID.value || 0)) {
    selectedWarehouseGroupTemplateID.value = nextTemplateID
  }
}

async function loadCustomers() {
  if (isCustomerInventoryContext.value) return
  try {
    const data = await apiGet('/api/customers?limit=200&active=true')
    customerOptions.value = data.rows || []
  } catch {
    customerOptions.value = []
  }
}

function filterCustomerOptions() {
  // filter is computed inline in template via filteredCustomerOptions
}

const filteredCustomerOptions = computed(() => {
  const q = customerSearch.value.toLowerCase()
  if (!q) return []
  return customerOptions.value.filter((customer) => {
    const name = (customer.name || '').toLowerCase()
    const phone = (customer.phone || '').toLowerCase()
    const company = (customer.company_name || '').toLowerCase()
    return name.includes(q) || phone.includes(q) || company.includes(q)
  }).slice(0, 20)
})

function selectCustomerForBinding(customer) {
  warehouseBindCustomerID.value = Number(customer.id || 0)
  customerSearch.value = customerOptionLabel(customer)
}

function clearCustomerBinding() {
  warehouseBindCustomerID.value = 0
  customerSearch.value = ''
}

function syncWarehouseBinding() {
  const customerID = Number(selectedWarehouseRow.value?.customer_id || 0)
  warehouseBindCustomerID.value = customerID
  customerSearch.value = ''
  if (customerID > 0) {
    const customer = customerOptions.value.find((c) => Number(c.id) === customerID)
    if (customer) customerSearch.value = customerOptionLabel(customer)
  }
}

function openWarehouseSettingsDrawer() {
  if (!selectedWarehouse.value) return
  syncWarehouseBinding()
  warehouseSettingsDrawerOpen.value = true
}

function openWarehouseBusinessGroupManagement() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: 'groupTemplates',
      returnNavigation: {
        key: 'warehouseInventory',
        label: '返回仓库库存',
      },
    },
  }))
}

async function saveWarehouseCustomerBinding() {
  if (!selectedWarehouse.value) return
  const customerID = Number(warehouseBindCustomerID.value || 0)
  warehouseBindingSaving.value = true
  error.value = ''
  try {
    const row = await apiSend(`/api/stock/warehouses/${encodeURIComponent(selectedWarehouse.value)}/customer`, {
      method: 'PUT',
      body: { customer_id: customerID },
    })
    warehouses.value = warehouses.value.map((item) => (item.code === row.code ? row : item))
    syncWarehouseBinding()
    warehouseSettingsDrawerOpen.value = false
  } catch (err) {
    error.value = err.message || '保存仓库客户绑定失败'
  } finally {
    warehouseBindingSaving.value = false
  }
}

async function clearWarehouseBusinessGroupAssignment(warehouseCode) {
  const code = String(warehouseCode || '').trim()
  if (!code) return
  const url = new URL('/api/business-group-assignments', window.location.origin)
  url.searchParams.set('usage_key', 'warehouse_inventory')
  url.searchParams.set('object_key', 'warehouse')
  url.searchParams.set('object_ref', code)
  const data = await apiGet(url)
  const assignments = Array.isArray(data?.rows) ? data.rows : []
  await Promise.all(assignments.map((row) => apiSend(`/api/business-group-assignments/${row.id}`, { method: 'DELETE' })))
}

async function moveSelectedWarehousesToGroup() {
  const targetGroupItemID = Number(selectedWarehouseMoveGroupItemID.value || 0)
  const targetOption = targetGroupItemID > 0 ? warehouseGroupOptionByItemID(targetGroupItemID) : null
  if (targetGroupItemID > 0 && !targetOption) return
  const rowsToMove = selectedWarehouseRowsForMove.value.filter((row) => {
    if (!targetOption) return warehouseGroupID(row) > 0 || warehouseGroupItemID(row) > 0
    return warehouseGroupID(row) !== Number(targetOption.group_id || 0) || warehouseGroupItemID(row) !== Number(targetOption.group_item_id || 0)
  })
  if (!rowsToMove.length) return
  loading.value = true
  error.value = ''
  try {
    for (const row of rowsToMove) {
      const code = warehouseRowKey(row)
      if (!code) continue
      if (!targetOption) {
        await clearWarehouseBusinessGroupAssignment(code)
        continue
      }
      await apiSend('/api/business-group-assignments', {
        body: businessGroupMoveAssignmentPayload({
          usageKey: 'warehouse_inventory',
          objectKey: 'warehouse',
          objectRef: code,
          option: targetOption,
          sortOrder: 100,
        }),
      })
    }
    selectedWarehouseKeys.value = []
    selectedWarehouseMoveGroupItemID.value = 0
    await loadWarehouses()
  } catch (err) {
    error.value = err.message || '移动仓库分类失败'
  } finally {
    loading.value = false
  }
}

async function saveWarehouseSettings() {
  if (!selectedWarehouse.value) return
  warehouseBindingSaving.value = true
  error.value = ''
  try {
    if (isExternalWarehouse.value) {
      const customerID = Number(warehouseBindCustomerID.value || 0)
      const row = await apiSend(`/api/stock/warehouses/${encodeURIComponent(selectedWarehouse.value)}/customer`, {
        method: 'PUT',
        body: { customer_id: customerID },
      })
      warehouses.value = warehouses.value.map((item) => (item.code === row.code ? row : item))
    }
    await loadWarehouses()
    syncWarehouseBinding()
    warehouseSettingsDrawerOpen.value = false
  } catch (err) {
    error.value = err.message || '保存仓库设置失败'
  } finally {
    warehouseBindingSaving.value = false
  }
}

async function loadInventory() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/stock/warehouse-inventory', window.location.origin)
    if (q.value) url.searchParams.set('q', q.value)
    if (selectedWarehouse.value) url.searchParams.set('warehouse', selectedWarehouse.value)
    if (itemType.value) url.searchParams.set('item_type', itemType.value)
    if (isCustomerInventoryContext.value) url.searchParams.set('customer_id', String(contextCustomerID.value))
    url.searchParams.set('page', String(page.value))
    url.searchParams.set('limit', String(limit.value))
    const data = await apiGet(url)
    const pagination = paginationFromApi(data)
    rows.value = data.rows || []
    page.value = pagination.page
    limit.value = pagination.pageSize
    total.value = pagination.total
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadInventoryPage(nextPage) {
  page.value = Math.max(1, Number(nextPage || 1))
  await loadInventory()
}

function handleInventoryPaginationChange({ page: nextPage, pageSize }) {
  limit.value = normalizePageSize(pageSize)
  loadInventoryPage(nextPage)
}

function openTraceDrawer(batch) {
  traceDrawerOpen.value = true
  traceError.value = ''
  traceResult.value = null
  if (batch) traceBatch.value = batch
}

async function loadTrace() {
  traceLoading.value = true
  traceError.value = ''
  traceResult.value = null
  try {
    const batch = traceBatch.value.trim()
    if (!batch) throw new Error('请填写批次号')
    const url = new URL('/api/stock/trace', window.location.origin)
    url.searchParams.set('batch', batch)
    traceResult.value = await apiGet(url)
  } catch (err) {
    traceError.value = err.message || '追溯失败'
  } finally {
    traceLoading.value = false
  }
}

function isLegacyMaterialBatch(batchCode) {
  return String(batchCode || '').startsWith('LEGACY-MAT')
}

function openReservationDrawer() {
  reservationDrawerOpen.value = true
  loadReservations()
}

async function loadReservations() {
  reservationLoading.value = true
  reservationError.value = ''
  try {
    const url = new URL('/api/produce/wip-reservations', window.location.origin)
    url.searchParams.set('status', 'reserved')
    if (reservationWorkOrderNo.value) url.searchParams.set('work_order_no', reservationWorkOrderNo.value)
    const data = await apiGet(url)
    reservationTotals.value = data || {}
    reservations.value = (data.rows || []).map((row) => ({ ...row, adjust_reserved_g: row.reserved_g }))
  } catch (err) {
    reservationError.value = err.message || '加载WIP占用失败'
  } finally {
    reservationLoading.value = false
  }
}

async function adjustReservation(row) {
  reservationError.value = ''
  try {
    await apiSend('/api/produce/wip-reservations/adjust', {
      body: { reservation_id: row.id, reserved_g: Number(row.adjust_reserved_g || 0), note: '仓库库存页调整' },
    })
    await loadReservations()
  } catch (err) {
    reservationError.value = err.message || '调整失败'
  }
}

async function releaseReservation(row) {
  reservationError.value = ''
  try {
    await apiSend('/api/produce/wip-reservations/release', {
      body: { running_item_id: row.running_item_id, work_order_no: row.work_order_no, note: '仓库库存页释放' },
    })
    await loadReservations()
  } catch (err) {
    reservationError.value = err.message || '释放失败'
  }
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    await Promise.all([loadWarehouseBusinessGroups(), loadWarehouses(), loadCustomers()])
    await loadInventory()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

watch(() => props.viewParams, (params) => applyViewParams(params), { deep: true, immediate: true })
watch(() => props.customerContextId, () => {
  if (isCustomerInventoryContext.value) {
    traceDrawerOpen.value = false
    reservationDrawerOpen.value = false
  }
  loadAll()
})

watch(selectedWarehouseGroupTemplateID, () => {
  selectedWarehouseMoveGroupItemID.value = 0
  selectedWarehouseKeys.value = []
  collapsedWarehouseGroups.value = []
})

watch(warehouseGroupItemOptions, (options) => {
  const selected = Number(selectedWarehouseMoveGroupItemID.value || 0)
  if (selected > 0 && !options.some((option) => Number(option.group_item_id || 0) === selected)) {
    selectedWarehouseMoveGroupItemID.value = 0
  }
})

onMounted(loadAll)
</script>

<style scoped>
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #e5e7eb;border-radius:8px;background:#fff;padding:12px}.panel-head{display:flex;justify-content:space-between;align-items:flex-start;gap:12px;margin-bottom:12px}.panel-head h2{margin:0 0 4px;font-size:18px}.panel-head p{margin:0;color:#6b7280;font-size:13px}.head-actions{display:flex;gap:8px;align-items:center}.filters{display:grid;grid-template-columns:minmax(220px,1fr) 140px 90px;gap:10px;align-items:end}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}input,select,button{font:inherit;min-height:36px;border-radius:6px}input,select{width:100%;border:1px solid #d1d5db;padding:7px 9px}button{cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff;padding:8px 12px}.secondary{border:1px solid #9ca3af;background:#fff;color:#111;padding:8px 12px}.link{border:0;background:transparent;color:#111;text-decoration:underline;padding:0;min-height:0}.workspace{display:grid;grid-template-columns:300px minmax(0,1fr);gap:16px}.warehouse-panel{align-self:start}.panel-title{font-weight:700;margin-bottom:10px}.warehouse{width:100%;text-align:left;border:1px solid #e5e7eb;background:#fff;border-radius:8px;padding:9px;margin-bottom:8px}.warehouse strong{display:block}.warehouse small{display:block;color:#6b7280;margin-top:3px;line-height:1.35}.warehouse.active{border-color:#111;background:#111;color:#fff}.warehouse.active small{color:#e5e7eb}.warehouse-group-controls{display:grid;gap:8px;margin-bottom:10px}.warehouse-group-list{display:grid;gap:4px;margin:8px 0 10px}.warehouse-section-toggle{width:100%;text-align:left;border:1px solid #e5e7eb;background:#f5f5f5;border-radius:8px;padding:7px 9px;margin-bottom:6px;display:flex;justify-content:space-between;align-items:center;font-weight:600;font-size:13px}.warehouse-section-toggle b{font-size:11px;color:#999}.warehouse-row{display:grid;grid-template-columns:28px minmax(0,1fr);align-items:flex-start;gap:6px;padding-left:var(--classification-item-indent, 0)}.warehouse-row .warehouse{margin-bottom:8px}.warehouse-select{display:flex;align-items:center;justify-content:center;margin-top:8px}.warehouse-select input{width:auto;min-height:0}.warehouse-empty-row{margin:0 0 8px;padding:6px 8px;color:#6b7280;font-size:13px;background:#fafafa;border-radius:6px}.warehouse-binding-card{display:grid;gap:8px;border:1px solid #e5e7eb;border-radius:8px;background:#f9fafb;padding:10px;margin-top:10px}.warehouse-binding-card button{width:100%}.summary{display:grid;grid-template-columns:repeat(3,minmax(0,1fr)) auto;gap:10px;margin-bottom:12px}.summary div{border:1px solid #e5e7eb;border-radius:8px;padding:10px}.summary span{display:block;color:#6b7280;font-size:12px;margin-bottom:4px}.summary strong{font-size:18px}.table-wrap{overflow:auto}table{width:100%;min-width:1100px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px}th{background:#fbfbfb}td small{display:block;color:#6b7280;margin-top:3px;line-height:1.35}.pill,.quality-pill{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px;background:#f9fafb;white-space:nowrap}.quality-pass{border-color:#bbf7d0;background:#f0fdf4;color:#166534}.quality-hold{border-color:#fde68a;background:#fffbeb;color:#92400e}.quality-reject{border-color:#fecaca;background:#fef2f2;color:#991b1b}.quality-unchecked{border-color:#d1d5db;background:#f9fafb;color:#4b5563}.muted{color:#666;text-align:center}.muted.left{text-align:left}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}.tip{border:1px solid #fde68a;background:#fffbeb;color:#92400e;border-radius:8px;padding:9px 10px;margin-bottom:12px;font-size:13px;line-height:1.45}.drawer-mask{position:fixed;inset:0;background:rgba(0,0,0,.22);display:flex;justify-content:flex-end;z-index:40}.drawer{width:min(460px,100%);height:100%;background:#fff;border-left:1px solid #d1d5db;padding:16px;overflow:auto}.drawer.wide{width:min(760px,100%)}.drawer-head{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:12px}.drawer h3{margin:0;font-size:18px}.trace-search{display:grid;grid-template-columns:1fr 84px;gap:10px;align-items:end;margin-bottom:12px}.trace-title{font-weight:700;margin-bottom:10px}dl{display:grid;gap:8px;margin:0 0 14px}dl div{display:grid;grid-template-columns:88px 1fr;gap:8px}dt{color:#6b7280}dd{margin:0}.trace-block h4{margin:14px 0 8px;font-size:14px}.trace-table{min-width:0}.reservation-summary{margin-bottom:12px}.reservation-table input{min-width:110px}.danger{color:#b91c1c;margin-left:8px}
@media (max-width:900px){.page{padding:12px}.filters,.workspace,.summary{grid-template-columns:1fr}}
</style>
