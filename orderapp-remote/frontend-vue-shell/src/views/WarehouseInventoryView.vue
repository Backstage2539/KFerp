<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>仓库库存</h2>
          <p>按仓库查看原料、包材、WIP 和成品库存，批次作为明细维度展开。</p>
        </div>
        <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label><span>搜索</span><input v-model.trim="q" placeholder="物品/批次" @keyup.enter="loadInventory" /></label>
        <label>
          <span>类型</span>
          <select v-model="itemType" @change="loadInventory">
            <option value="">全部</option>
            <option value="material">原料/包材</option>
            <option value="finished_product">成品</option>
          </select>
        </label>
        <button class="primary" type="button" @click="loadInventory" :disabled="loading">查询</button>
      </div>
    </section>

    <div class="workspace">
      <aside class="panel warehouse-panel">
        <div class="panel-title">仓库</div>
        <button class="warehouse" :class="{ active: selectedWarehouse === '' }" type="button" @click="selectWarehouse('')">
          <strong>全部仓库</strong>
          <small>跨仓库汇总查询</small>
        </button>
        <button
          v-for="row in warehouses"
          :key="row.code"
          class="warehouse"
          :class="{ active: selectedWarehouse === row.code }"
          type="button"
          @click="selectWarehouse(row.code)">
          <strong>{{ row.name }}</strong>
          <small>{{ kindLabel(row.kind) }} · {{ row.description || row.code }}</small>
        </button>
      </aside>

      <section class="panel table-panel">
        <div class="summary">
          <div><span>当前仓库</span><strong>{{ currentWarehouseName }}</strong></div>
          <div><span>库存行</span><strong>{{ rows.length }}</strong></div>
          <div><span>合计(g)</span><strong>{{ totalG.toLocaleString('zh-CN') }}</strong></div>
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
                <th>数量(g)</th>
                <th>件数</th>
                <th>单位成本</th>
                <th>更新</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in rows" :key="rowKey(row)">
                <td>{{ row.warehouse_name || row.warehouse }}</td>
                <td><span class="pill">{{ typeLabel(row.item_type) }}</span></td>
                <td>{{ row.item_name }}</td>
                <td>{{ row.spec_g ? `${row.spec_g}g` : '-' }}</td>
                <td>{{ row.batch_code || '-' }}</td>
                <td>{{ Number(row.qty_g || 0).toLocaleString('zh-CN') }}</td>
                <td>{{ row.qty_units || '-' }}</td>
                <td>{{ money(row.unit_cost) }}</td>
                <td>{{ row.updated_at || '-' }}</td>
              </tr>
              <tr v-if="!rows.length"><td colspan="9" class="muted">暂无库存</td></tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet } from '../api/client'

const warehouses = ref([])
const rows = ref([])
const q = ref('')
const itemType = ref('')
const selectedWarehouse = ref('')
const loading = ref(false)
const error = ref('')

const currentWarehouseName = computed(() => {
  if (!selectedWarehouse.value) return '全部仓库'
  return warehouses.value.find((row) => row.code === selectedWarehouse.value)?.name || selectedWarehouse.value
})
const totalG = computed(() => rows.value.reduce((sum, row) => sum + Number(row.qty_g || 0), 0))

function kindLabel(kind) {
  return {
    raw: '原料仓',
    packaging: '包材仓',
    wip: 'WIP仓',
    finished: '成品仓',
    loss: '损耗仓',
  }[kind] || '仓库'
}

function typeLabel(type) {
  return type === 'finished_product' ? '成品' : '原料/包材'
}

function money(value) {
  return Number(value || 0).toFixed(2)
}

function rowKey(row) {
  return `${row.warehouse}-${row.item_type}-${row.item_id}-${row.spec_g || 0}-${row.batch_id || row.batch_code || 'summary'}`
}

function selectWarehouse(code) {
  selectedWarehouse.value = code
  loadInventory()
}

async function loadWarehouses() {
  const data = await apiGet('/api/stock/warehouses')
  warehouses.value = data.rows || []
}

async function loadInventory() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/stock/warehouse-inventory', window.location.origin)
    if (q.value) url.searchParams.set('q', q.value)
    if (selectedWarehouse.value) url.searchParams.set('warehouse', selectedWarehouse.value)
    if (itemType.value) url.searchParams.set('item_type', itemType.value)
    const data = await apiGet(url)
    rows.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    await loadWarehouses()
    await loadInventory()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(loadAll)
</script>

<style scoped>
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #e5e7eb;border-radius:8px;background:#fff;padding:12px}.panel-head{display:flex;justify-content:space-between;align-items:flex-start;gap:12px;margin-bottom:12px}.panel-head h2{margin:0 0 4px;font-size:18px}.panel-head p{margin:0;color:#6b7280;font-size:13px}.filters{display:grid;grid-template-columns:minmax(220px,1fr) 150px 90px;gap:10px;align-items:end}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}input,select,button{font:inherit;min-height:36px;border-radius:6px}input,select{width:100%;border:1px solid #d1d5db;padding:7px 9px}button{cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff;padding:8px 12px}.secondary{border:1px solid #9ca3af;background:#fff;color:#111;padding:8px 12px}.workspace{display:grid;grid-template-columns:260px minmax(0,1fr);gap:16px}.warehouse-panel{align-self:start}.panel-title{font-weight:700;margin-bottom:10px}.warehouse{width:100%;text-align:left;border:1px solid #e5e7eb;background:#fff;border-radius:8px;padding:9px;margin-bottom:8px}.warehouse strong{display:block}.warehouse small{display:block;color:#6b7280;margin-top:3px;line-height:1.35}.warehouse.active{border-color:#111;background:#111;color:#fff}.warehouse.active small{color:#e5e7eb}.summary{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px;margin-bottom:12px}.summary div{border:1px solid #e5e7eb;border-radius:8px;padding:10px}.summary span{display:block;color:#6b7280;font-size:12px;margin-bottom:4px}.summary strong{font-size:18px}.table-wrap{overflow:auto}table{width:100%;min-width:980px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px}th{background:#fbfbfb}.pill{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px;background:#f9fafb}.muted{color:#666;text-align:center}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}
@media (max-width:900px){.page{padding:12px}.filters,.workspace,.summary{grid-template-columns:1fr}}
</style>
