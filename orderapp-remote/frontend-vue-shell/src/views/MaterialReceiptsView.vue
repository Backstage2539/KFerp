<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head"><h2>原料入库</h2><button class="secondary" @click="loadMaterials" :disabled="loading">刷新物料</button></div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已入库：{{ ok }}</div>
      <div class="form-grid">
        <label class="material-field">
          <span>物料</span>
          <input v-model.trim="materialQuery" placeholder="物料名称 / 编号" />
          <select v-model.number="form.material_id">
            <option :value="0">请选择</option>
            <option v-for="m in materialOptions" :key="m.id" :value="m.id">{{ receiptMaterialLabel(m) }}</option>
          </select>
        </label>
        <label><span>供应商</span><input v-model.trim="form.supplier" /></label>
        <label><span>数量(g)</span><input type="number" min="1" step="1" v-model.number="form.qty_g" /></label>
        <label><span>成本/千克</span><input type="number" min="0" step="0.01" v-model.number="form.unit_cost" /></label>
        <label class="wide"><span>备注</span><input v-model.trim="form.note" /></label>
        <button class="primary" @click="submit" :disabled="saving">提交入库</button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { filterReceiptMaterials, receiptMaterialLabel, selectableReceiptMaterials } from '../lib/material-receipts'
const materials = ref([])
const materialQuery = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref('')
const form = reactive({ material_id: 0, supplier: '', qty_g: 0, unit_cost: 0, note: '' })
const materialOptions = computed(() => {
  const filtered = filterReceiptMaterials(materials.value, materialQuery.value)
  const selected = materials.value.find((m) => Number(m.id) === Number(form.material_id))
  if (selected && !filtered.some((m) => Number(m.id) === Number(selected.id))) {
    return [selected, ...filtered]
  }
  return filtered
})
async function loadMaterials() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet('/api/materials?limit=500')
    materials.value = selectableReceiptMaterials(data.rows || [])
  } catch (err) { error.value = err.message || '加载失败' } finally { loading.value = false }
}
async function submit() {
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiSend('/api/stock/material-receipts', { body: form })
    ok.value = data.batch_code
    form.qty_g = 0
    form.note = ''
  } catch (err) { error.value = err.message || '提交失败' } finally { saving.value = false }
}
onMounted(loadMaterials)
</script>

<style scoped>
.page { padding:16px; }
.panel { border:1px solid #eee; border-radius:8px; padding:12px; background:#fff; }
.panel-head { display:flex; justify-content:space-between; align-items:center; gap:12px; margin-bottom:12px; }
h2 { margin:0; font-size:18px; }
.form-grid { display:grid; grid-template-columns: minmax(220px,1.4fr) 1fr 110px 120px; gap:10px; align-items:end; }
.wide { grid-column: span 3; }
label span { display:block; color:#666; font-size:12px; margin-bottom:5px; }
input, select, button { font:inherit; min-height:36px; border-radius:6px; }
input, select { width:100%; border:1px solid #ddd; padding:7px 9px; }
.material-field { display:grid; gap:6px; }
button { padding:8px 12px; cursor:pointer; }
.primary { border:1px solid #111; background:#111; color:#fff; }
.secondary { border:1px solid #999; background:#fff; color:#111; }
.error, .ok { border-radius:8px; padding:10px; margin-bottom:12px; }
.error { background:#ffecec; border:1px solid #ffb9b9; }
.ok { background:#e9ffe9; border:1px solid #b8f5b8; }
@media (max-width:900px){ .page{padding:12px;} .form-grid{grid-template-columns:1fr;} .wide{grid-column:auto;} }
</style>
