<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head"><h2>库存调整单</h2><button class="secondary" @click="loadOptions" :disabled="loading">刷新</button></div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">调整单已提交：#{{ ok }}</div>
      <div class="form-grid">
        <label><span>类型</span><select v-model="form.item_type"><option value="material">物料</option><option value="finished_product">成品</option></select></label>
        <label><span>对象</span><select v-model.number="form.item_id"><option :value="0">请选择</option><option v-for="item in currentOptions" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
        <label><span>规格(g)</span><input type="number" min="0" step="1" v-model.number="form.spec_g" :disabled="form.item_type !== 'finished_product'" /></label>
        <label><span>仓库</span><select v-model="form.warehouse"><option v-for="row in warehouseOptions" :key="row.code" :value="row.code">{{ row.name }}</option></select></label>
        <label><span>目标(g/散装g)</span><input type="number" min="0" step="1" v-model.number="form.target_g" /></label>
        <label><span>目标件数</span><input type="number" min="0" step="1" v-model.number="form.target_units" /></label>
        <label class="wide"><span>原因</span><input v-model.trim="form.reason" placeholder="盘点调整/损耗/更正" /></label>
        <button class="primary" @click="submit" :disabled="saving">提交调整</button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
const materials = ref([])
const products = ref([])
const warehouses = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref('')
const form = reactive({ item_type: 'material', item_id: 0, spec_g: 0, warehouse: 'raw_materials', target_g: 0, target_units: 0, reason: '' })
const currentOptions = computed(() => form.item_type === 'material' ? materials.value : products.value)
const warehouseOptions = computed(() => {
  const kind = form.item_type === 'finished_product' ? 'finished' : ''
  const rows = warehouses.value.filter((row) => !kind || row.kind === kind)
  return rows.length ? rows : [{ code: form.item_type === 'finished_product' ? 'finished_goods' : 'raw_materials', name: form.item_type === 'finished_product' ? '成品仓' : '原料仓' }]
})
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
async function submit() {
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiSend('/api/stock/adjustments', { body: form })
    ok.value = data.adjustment_id
  } catch (err) { error.value = err.message || '提交失败' } finally { saving.value = false }
}
watch(() => form.item_type, () => {
  form.warehouse = form.item_type === 'finished_product' ? 'finished_goods' : 'raw_materials'
})
onMounted(loadOptions)
</script>

<style scoped>
.page { padding:16px; }
.panel { border:1px solid #eee; border-radius:8px; padding:12px; background:#fff; }
.panel-head { display:flex; justify-content:space-between; align-items:center; gap:12px; margin-bottom:12px; }
h2 { margin:0; font-size:18px; }
.form-grid { display:grid; grid-template-columns:120px minmax(220px,1.4fr) 100px 150px 130px 110px; gap:10px; align-items:end; }
.wide { grid-column: span 5; }
label span { display:block; color:#666; font-size:12px; margin-bottom:5px; }
input, select, button { font:inherit; min-height:36px; border-radius:6px; }
input, select { width:100%; border:1px solid #ddd; padding:7px 9px; }
button { padding:8px 12px; cursor:pointer; }
.primary { border:1px solid #111; background:#111; color:#fff; }
.secondary { border:1px solid #999; background:#fff; color:#111; }
.error, .ok { border-radius:8px; padding:10px; margin-bottom:12px; }
.error { background:#ffecec; border:1px solid #ffb9b9; }
.ok { background:#e9ffe9; border:1px solid #b8f5b8; }
@media (max-width:900px){ .page{padding:12px;} .form-grid{grid-template-columns:1fr;} .wide{grid-column:auto;} }
</style>
