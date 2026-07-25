<template>
  <div class="stock-operation-page" :class="{ embedded: props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>盘点调整</h2>
          <p>只用于实物盘点数量和批次成本修正，不替代正常发料、转仓或工艺损耗。</p>
        </div>
        <button class="secondary" type="button" @click="loadOptions" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">调整单已提交：#{{ ok }}</div>
      <div class="operation-grid">
        <label>
          <span>调整类型</span>
          <select v-model="form.adjustment_type">
            <option value="quantity">库存数量</option>
            <option value="material_cost">批次成本</option>
          </select>
        </label>
        <label>
          <span>类型</span>
          <select v-model="form.item_type" :disabled="isMaterialCostAdjustment">
            <option value="material">物料</option>
            <option value="finished_product">成品</option>
          </select>
        </label>
        <label :class="isMaterialCostAdjustment ? 'span-2' : ''">
          <span>对象</span>
          <SearchableSelect
            v-model="form.item_id"
            :options="currentOptions"
            :option-label="itemLabel"
            placeholder="输入名称 / 编号"
            empty-text="没有匹配对象"
          />
        </label>
        <label v-if="isMaterialCostAdjustment" class="span-2">
          <span>原料批次</span>
          <SearchableSelect
            v-model="form.material_batch_id"
            :options="materialBatches"
            :option-label="batchLabel"
            placeholder="选择要修正成本的批次"
            empty-text="当前物料没有可用批次"
          />
        </label>
        <label v-if="isMaterialCostAdjustment"><span>目标成本（元/{{ selectedMaterialCostUnitLabel }}）</span><input type="number" min="0" step="0.0001" v-model.number="form.target_unit_cost" /><small>批次成本调整当前只支持重量物料。</small></label>
        <label v-if="!isMaterialCostAdjustment && form.item_type === 'finished_product'"><span>规格(g)</span><input type="number" min="0" step="1" v-model.number="form.spec_g" /></label>
        <label v-if="!isMaterialCostAdjustment">
          <span>仓库</span>
          <select v-model="form.warehouse">
            <option v-for="row in warehouseOptions" :key="row.code" :value="row.code">{{ row.name }}</option>
          </select>
        </label>
        <label v-if="!isMaterialCostAdjustment && form.item_type === 'material'"><span>目标数量（{{ selectedMaterialUnitLabel }}）</span><input type="number" min="0" step="0.001" v-model.number="form.target_qty" /></label>
        <label v-if="!isMaterialCostAdjustment && form.item_type === 'finished_product'"><span>目标散装g</span><input type="number" min="0" step="1" v-model.number="form.target_g" /></label>
        <label v-if="!isMaterialCostAdjustment && form.item_type === 'finished_product'"><span>成品目标件数</span><input type="number" min="0" step="1" v-model.number="form.target_units" /></label>
        <label v-if="!isMaterialCostAdjustment && form.item_type === 'material'"><span>补录成本（元/{{ selectedMaterialCostUnitLabel }}）</span><input type="number" min="0" step="0.0001" v-model.number="form.target_unit_cost" placeholder="不填则用物料默认采购价" /></label>
        <label class="span-3"><span>原因</span><input v-model.trim="form.reason" placeholder="实物盘点差异 / 批次成本更正" /></label>
        <button class="primary" type="button" @click="submit" :disabled="saving || (isMaterialCostAdjustment && !isSelectedMaterialWeight)">{{ isMaterialCostAdjustment ? '提交成本调整' : '提交调整' }}</button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import SearchableSelect from '../components/SearchableSelect.vue'

const props = defineProps({
  embedded: { type: Boolean, default: false },
})

const materials = ref([])
const products = ref([])
const warehouses = ref([])
const materialBatches = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref('')
const form = reactive({ adjustment_type: 'quantity', item_type: 'material', item_id: 0, spec_g: 0, warehouse: 'raw_materials', target_qty: 0, target_g: 0, target_units: 0, material_batch_id: 0, target_unit_cost: 0, reason: '' })
const isMaterialCostAdjustment = computed(() => form.adjustment_type === 'material_cost')
const currentOptions = computed(() => form.item_type === 'material'
  ? (isMaterialCostAdjustment.value ? materials.value.filter((material) => isMaterialWeight(material)) : materials.value)
  : products.value)
const selectedMaterial = computed(() => materials.value.find((row) => Number(row.id || row.ID || 0) === Number(form.item_id || 0)) || null)
const selectedMaterialUnitLabel = computed(() => selectedMaterial.value?.unit || selectedMaterial.value?.Unit || '库存单位')
const selectedMaterialCostUnitLabel = computed(() => materialCostUnit(selectedMaterial.value))
const isSelectedMaterialWeight = computed(() => isMaterialWeight(selectedMaterial.value))
const warehouseOptions = computed(() => {
  const kind = form.item_type === 'finished_product' ? 'finished' : ''
  const rows = warehouses.value.filter((row) => !kind || row.kind === kind)
  return rows.length ? rows : [{ code: form.item_type === 'finished_product' ? 'finished_goods' : 'raw_materials', name: form.item_type === 'finished_product' ? '成品仓' : '原料仓' }]
})

function itemLabel(row) {
  const name = String(row?.name || row?.Name || '').trim()
  const code = String(row?.code || row?.Code || '').trim()
  return code ? `${name} (${code})` : name
}

function batchLabel(row) {
  if (!row) return ''
  const kg = Number(row.remaining_g || 0) / 1000
  return `${row.batch_code} · 剩余${kg.toFixed(3)}kg · ${Number(row.unit_cost || 0).toFixed(2)}元/${selectedMaterialCostUnitLabel.value}`
}

function materialCostUnit(material) {
  const costUnit = String(material?.cost_unit || material?.CostUnit || '').trim()
  if (costUnit) return costUnit
  const inventoryUnit = String(material?.unit || material?.Unit || '').trim()
  if (!inventoryUnit) return '成本计价单位'
  return ['g', 'kg', 'lb', 'oz', '克', '千克'].includes(inventoryUnit.toLowerCase()) ? 'kg' : inventoryUnit
}

function isMaterialWeight(material) {
  const inventoryUnit = String(material?.unit || material?.Unit || '').trim().toLowerCase()
  return ['g', 'kg', 'lb', 'oz', '克', '千克'].includes(inventoryUnit)
}

async function loadOptions() {
  loading.value = true
  error.value = ''
  try {
    const [mat, prod, wh] = await Promise.all([apiGet('/api/materials?limit=500'), apiGet('/api/products'), apiGet('/api/stock/warehouses')])
    materials.value = mat.rows || []
    products.value = prod.rows || prod.products || []
    warehouses.value = wh.rows || []
  } catch (err) { error.value = err.message || '加载失败' } finally { loading.value = false }
}

async function loadMaterialBatches() {
  materialBatches.value = []
  form.material_batch_id = 0
  if (!isMaterialCostAdjustment.value || !form.item_id || !isSelectedMaterialWeight.value) return
  try {
    const url = new URL('/api/stock/material-batches', window.location.origin)
    url.searchParams.set('material_id', String(form.item_id))
    url.searchParams.set('active_only', '1')
    url.searchParams.set('limit', '500')
    const data = await apiGet(url)
    materialBatches.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载批次失败'
  }
}

async function submit() {
  if (isMaterialCostAdjustment.value && !isSelectedMaterialWeight.value) {
    error.value = '批次成本调整当前只支持重量物料；件、袋、盒等离散物料请通过数量补录或原料入库处理。'
    return
  }
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    const body = isMaterialCostAdjustment.value ? {
      adjustment_type: 'material_cost',
      item_type: 'material',
      item_id: form.item_id,
      material_batch_id: form.material_batch_id,
      target_unit_cost: Number(form.target_unit_cost || 0),
      reason: form.reason,
    } : form.item_type === 'material' ? {
      adjustment_type: 'quantity',
      item_type: 'material',
      item_id: form.item_id,
      warehouse: form.warehouse,
      target_qty: Number(form.target_qty || 0),
      unit_code: selectedMaterial.value?.unit || selectedMaterial.value?.Unit || '',
      target_unit_cost: Number(form.target_unit_cost || 0),
      reason: form.reason,
    } : {
      adjustment_type: 'quantity',
      item_type: form.item_type,
      item_id: form.item_id,
      spec_g: form.spec_g,
      warehouse: form.warehouse,
      target_g: form.target_g,
      target_units: form.target_units,
      target_unit_cost: form.item_type === 'material' ? Number(form.target_unit_cost || 0) : 0,
      reason: form.reason,
    }
    const data = await apiSend('/api/stock/adjustments', { body })
    ok.value = data.adjustment_id
  } catch (err) { error.value = err.message || '提交失败' } finally { saving.value = false }
}

watch(() => form.item_type, () => {
  form.item_id = 0
  form.warehouse = form.item_type === 'finished_product' ? 'finished_goods' : 'raw_materials'
  form.target_qty = 0
  form.material_batch_id = 0
  materialBatches.value = []
})

watch(() => form.adjustment_type, () => {
  materialBatches.value = []
  form.material_batch_id = 0
  if (isMaterialCostAdjustment.value) {
    const keepMaterialID = form.item_type === 'material' && isSelectedMaterialWeight.value ? form.item_id : 0
    form.item_type = 'material'
    form.item_id = keepMaterialID
    form.spec_g = 0
    form.warehouse = 'raw_materials'
    form.target_g = 0
    form.target_units = 0
  }
  loadMaterialBatches()
})

watch(() => form.item_id, loadMaterialBatches)

onMounted(loadOptions)
</script>

<style scoped>
.stock-operation-page { padding:16px; display:grid; gap:16px; }
.stock-operation-page.embedded { padding:0; }
.panel { border:1px solid #e5e7eb; border-radius:8px; padding:12px; background:#fff; }
.panel-head { display:flex; justify-content:space-between; align-items:flex-start; gap:12px; margin-bottom:12px; }
.panel-head h2 { margin:0 0 4px; font-size:18px; }
.panel-head p { margin:0; color:#6b7280; font-size:13px; }
.operation-grid { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:10px; align-items:end; }
.span-2 { grid-column:span 2; }
.span-3 { grid-column:span 3; }
label { min-width:0; position:relative; }
label span { display:block; color:#666; font-size:12px; margin-bottom:5px; }
label small { display:block; color:#6b7280; font-size:12px; line-height:1.4; margin-top:4px; }
input, select, button { font:inherit; min-height:38px; border-radius:6px; }
input, select { width:100%; border:1px solid #d1d5db; padding:7px 9px; }
input:disabled { background:#f9fafb; color:#6b7280; }
button { padding:8px 12px; cursor:pointer; }
button:disabled { cursor:not-allowed; opacity:.6; }
.primary { border:1px solid #111; background:#111; color:#fff; }
.secondary { border:1px solid #9ca3af; background:#fff; color:#111; }
.error, .ok { border-radius:8px; padding:10px; margin-bottom:12px; }
.error { background:#ffecec; border:1px solid #ffb9b9; }
.ok { background:#e9ffe9; border:1px solid #b8f5b8; }
@media (max-width:900px){ .stock-operation-page{padding:12px;} .operation-grid{grid-template-columns:1fr;} .span-2,.span-3{grid-column:auto;} }
</style>
