<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>WIP在制仓</h2>
        <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已提交：{{ ok }}</div>
      <div class="form-grid">
        <label>
          <span>操作</span>
          <select v-model="mode" @change="applyModeWarehouses">
            <option value="issue">领料到WIP</option>
            <option value="return">退回原料仓</option>
          </select>
        </label>
        <label>
          <span>物料</span>
          <select v-model.number="form.material_id">
            <option :value="0">请选择</option>
            <option v-for="m in materials" :key="m.id" :value="m.id">{{ m.name }}</option>
          </select>
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
        <label>
          <span>数量(g)</span>
          <input type="number" min="1" step="1" v-model.number="form.qty_g" />
        </label>
        <label class="wide">
          <span>备注</span>
          <input v-model.trim="form.note" />
        </label>
        <button class="primary" type="button" @click="submit" :disabled="saving">提交</button>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h2>WIP批次库存</h2>
        <button class="secondary" type="button" @click="loadLocations" :disabled="loading">刷新库存</button>
      </div>
      <div class="filters">
        <label><span>搜索</span><input v-model.trim="filters.q" placeholder="批次/物料" @keyup.enter="loadLocations" /></label>
        <label><span>仓库</span><select v-model="filters.warehouse"><option value="wip">WIP在制仓</option><option value="raw_materials">原料仓</option><option value="">全部</option></select></label>
        <button class="primary" type="button" @click="loadLocations" :disabled="loading">查询</button>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>仓库</th><th>批次号</th><th>物料</th><th>数量(g)</th><th>入库时间</th><th>更新时间</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in locations" :key="`${row.material_batch_id}-${row.warehouse}`">
              <td>{{ row.warehouse_name || row.warehouse }}</td>
              <td>{{ row.batch_code }}</td>
              <td>{{ row.material_name }}</td>
              <td>{{ row.qty_g }}</td>
              <td>{{ row.received_at || '-' }}</td>
              <td>{{ row.updated_at || '-' }}</td>
            </tr>
            <tr v-if="!locations.length"><td colspan="6" class="muted">暂无库存</td></tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

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

function applyModeWarehouses() {
  if (mode.value === 'return') {
    form.from_warehouse = 'wip'
    form.to_warehouse = 'raw_materials'
    return
  }
  form.from_warehouse = 'raw_materials'
  form.to_warehouse = 'wip'
}

async function loadOptions() {
  const [mat, wh] = await Promise.all([
    apiGet('/api/materials?limit=500'),
    apiGet('/api/stock/warehouses'),
  ])
  materials.value = (mat.rows || []).filter((m) => (m.kind || m.Kind) !== 'pack')
  warehouses.value = wh.rows || []
}

async function loadLocations() {
  const url = new URL('/api/stock/material-batch-locations', window.location.origin)
  if (filters.q) url.searchParams.set('q', filters.q)
  if (filters.warehouse) url.searchParams.set('warehouse', filters.warehouse)
  url.searchParams.set('active_only', '1')
  const data = await apiGet(url)
  locations.value = data.rows || []
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    await loadOptions()
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

onMounted(loadAll)
</script>

<style scoped>
.page { padding:16px; display:grid; gap:16px; }
.panel { border:1px solid #eee; border-radius:8px; padding:12px; background:#fff; }
.panel-head { display:flex; justify-content:space-between; align-items:center; gap:12px; margin-bottom:12px; }
h2 { margin:0; font-size:18px; }
.form-grid { display:grid; grid-template-columns:130px minmax(220px,1.5fr) 150px 150px 110px; gap:10px; align-items:end; }
.wide { grid-column:span 4; }
.filters { display:grid; grid-template-columns:minmax(220px,1fr) 150px 90px; gap:10px; align-items:end; margin-bottom:12px; }
label span { display:block; color:#666; font-size:12px; margin-bottom:5px; }
input, select, button { font:inherit; min-height:36px; border-radius:6px; }
input, select { width:100%; border:1px solid #ddd; padding:7px 9px; }
button { padding:8px 12px; cursor:pointer; }
.primary { border:1px solid #111; background:#111; color:#fff; }
.secondary { border:1px solid #999; background:#fff; color:#111; }
.table-wrap { overflow:auto; }
table { width:100%; min-width:860px; border-collapse:collapse; }
th, td { border-bottom:1px solid #f0f0f0; padding:8px; text-align:left; font-size:13px; }
th { background:#fbfbfb; }
.muted { color:#666; text-align:center; }
.error, .ok { border-radius:8px; padding:10px; margin-bottom:12px; }
.error { background:#ffecec; border:1px solid #ffb9b9; }
.ok { background:#e9ffe9; border:1px solid #b8f5b8; }
@media (max-width:900px){ .page{padding:12px;} .form-grid,.filters{grid-template-columns:1fr;} .wide{grid-column:auto;} }
</style>
