<template>
  <div class="stock-operation-page" :class="{ embedded: props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>成品转仓</h2>
          <p>把已入库成品从默认成品仓调拨到门店、展会或其他成品仓。</p>
        </div>
        <button class="secondary" type="button" @click="loadOptions" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">转仓单已提交：{{ ok }}</div>
      <div class="operation-grid">
        <label class="span-2">
          <span>成品</span>
          <SearchableSelect
            v-model="form.product_id"
            :options="products"
            :option-label="productLabel"
            placeholder="输入成品名称 / 编号"
            empty-text="没有匹配成品"
          />
        </label>
        <label v-if="selectedProductUsesBOMSpecs">
          <span>BOM 规格</span>
          <select v-model.number="form.bom_spec_id">
            <option v-for="row in selectedProductBOMSpecs" :key="row.bom_spec_id" :value="row.bom_spec_id">
              {{ row.name }}（{{ row.unit }}）
            </option>
          </select>
        </label>
        <label v-else><span>规格(g)</span><input v-model.number="form.spec_g" type="number" min="1" step="1" /></label>
        <label>
          <span>出库仓</span>
          <select v-model="form.from_warehouse">
            <option v-for="row in finishedWarehouses" :key="row.code" :value="row.code">{{ row.name }}</option>
          </select>
        </label>
        <label>
          <span>入库仓</span>
          <select v-model="form.to_warehouse">
            <option v-for="row in finishedWarehouses" :key="row.code" :value="row.code">{{ row.name }}</option>
          </select>
        </label>
        <label><span>{{ selectedProductUsesBOMSpecs ? `数量（${selectedProductBOMSpec?.unit || '规格单位'}）` : '件数' }}</span><input v-model.number="form.qty_units" type="number" min="0" step="1" /></label>
        <label v-if="!selectedProductUsesBOMSpecs"><span>散装(g)</span><input v-model.number="form.qty_loose_g" type="number" min="0" step="1" /></label>
        <label class="span-3"><span>备注</span><input v-model.trim="form.note" placeholder="门店备货/展会备货/仓库整理" /></label>
        <button class="primary" type="button" @click="submit" :disabled="saving">提交转仓</button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import SearchableSelect from '../components/SearchableSelect.vue'
import {
  buildFinishedTransferPayload,
  currentBOMSpecs,
  defaultBOMSpecID,
  selectedBOMSpec,
  usesCurrentBOMSpecs,
} from '../lib/stock-bom-spec'

const props = defineProps({
  embedded: { type: Boolean, default: false },
})

const products = ref([])
const warehouses = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref('')
const form = reactive({
  product_id: 0,
  bom_spec_id: 0,
  spec_g: 454,
  from_warehouse: 'finished_goods',
  to_warehouse: 'finished_shop',
  qty_units: 0,
  qty_loose_g: 0,
  note: '',
})

const finishedWarehouses = computed(() => {
  const rows = warehouses.value.filter((row) => row.kind === 'finished')
  return rows.length ? rows : [{ code: 'finished_goods', name: '成品仓' }]
})
const selectedProduct = computed(() => products.value.find((row) => Number(row.id || row.product_id || 0) === Number(form.product_id || 0)) || null)
const selectedProductUsesBOMSpecs = computed(() => usesCurrentBOMSpecs(selectedProduct.value || {}))
const selectedProductBOMSpecs = computed(() => currentBOMSpecs(selectedProduct.value || {}))
const selectedProductBOMSpec = computed(() => selectedBOMSpec(selectedProduct.value || {}, form.bom_spec_id))

function productLabel(row) {
  const name = String(row?.name || row?.Name || '').trim()
  const code = String(row?.code || row?.Code || '').trim()
  return code ? `${name} (${code})` : name
}

async function loadOptions() {
  loading.value = true
  error.value = ''
  try {
    const [prod, wh] = await Promise.all([apiGet('/api/products/inventory?limit=200'), apiGet('/api/stock/warehouses')])
    products.value = prod.rows || prod.products || []
    warehouses.value = wh.rows || []
    onProductChange()
    if (!finishedWarehouses.value.some((row) => row.code === form.from_warehouse)) {
      form.from_warehouse = finishedWarehouses.value[0]?.code || 'finished_goods'
    }
    if (!finishedWarehouses.value.some((row) => row.code === form.to_warehouse)) {
      form.to_warehouse = finishedWarehouses.value.find((row) => row.code !== form.from_warehouse)?.code || finishedWarehouses.value[0]?.code || 'finished_goods'
    }
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
    if (selectedProductUsesBOMSpecs.value && !selectedProductBOMSpec.value) throw new Error('请选择当前默认 BOM 的规格')
    const body = buildFinishedTransferPayload(form, selectedProduct.value || {})
    const data = await apiSend('/api/stock/finished-transfers', { body })
    ok.value = data.transfer_no || data.transfer_id
  } catch (err) {
    error.value = err.message || '提交失败'
  } finally {
    saving.value = false
  }
}

function onProductChange() {
  const selectedStillExists = selectedProductBOMSpecs.value.some((row) => Number(row.bom_spec_id) === Number(form.bom_spec_id))
  form.bom_spec_id = selectedProductUsesBOMSpecs.value
    ? (selectedStillExists ? Number(form.bom_spec_id) : defaultBOMSpecID(selectedProduct.value || {}))
    : 0
  if (selectedProductUsesBOMSpecs.value) form.qty_loose_g = 0
}

watch(() => form.product_id, onProductChange)

onMounted(loadOptions)
</script>

<style scoped>
.stock-operation-page { padding:16px; display:grid; gap:16px; }
.stock-operation-page.embedded { padding:0; }
.panel { border:1px solid #e5e7eb; border-radius:8px; background:#fff; padding:12px; }
.panel-head { display:flex; justify-content:space-between; align-items:flex-start; gap:12px; margin-bottom:12px; }
.panel-head h2 { margin:0 0 4px; font-size:18px; }
.panel-head p { margin:0; color:#6b7280; font-size:13px; }
.operation-grid { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:10px; align-items:end; }
.span-2 { grid-column:span 2; }
.span-3 { grid-column:span 3; }
label { min-width:0; position:relative; }
label span { display:block; color:#666; font-size:12px; margin-bottom:5px; }
input, select, button { font:inherit; min-height:38px; border-radius:6px; }
input, select { width:100%; border:1px solid #d1d5db; padding:7px 9px; }
button { padding:8px 12px; cursor:pointer; }
button:disabled { cursor:not-allowed; opacity:.6; }
.primary { border:1px solid #111; background:#111; color:#fff; }
.secondary { border:1px solid #9ca3af; background:#fff; color:#111; }
.error, .ok { border-radius:8px; padding:10px; margin-bottom:12px; }
.error { background:#ffecec; border:1px solid #ffb9b9; }
.ok { background:#e9ffe9; border:1px solid #b8f5b8; }
@media (max-width:900px){ .stock-operation-page{padding:12px;} .operation-grid{grid-template-columns:1fr;} .span-2,.span-3{grid-column:auto;} }
</style>
