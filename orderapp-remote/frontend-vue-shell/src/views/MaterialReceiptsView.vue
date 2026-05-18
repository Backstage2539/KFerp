<template>
  <div class="stock-operation-page" :class="{ embedded: props.embedded }">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>原料入库</h2>
          <p>原料入库后生成原料批次、库存批次和原料仓库存。</p>
        </div>
        <button class="secondary" type="button" @click="loadMaterials" :disabled="loading">刷新物料</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已入库：{{ ok }}</div>
      <div class="operation-grid">
        <label class="span-2">
          <span>物料</span>
          <SearchableSelect
            v-model="form.material_id"
            :options="materialOptions"
            :option-label="receiptMaterialLabel"
            placeholder="输入物料名称 / 编号"
            empty-text="没有匹配原料"
          />
        </label>
        <label><span>供应商</span><input v-model.trim="form.supplier" /></label>
        <label><span>数量(g)</span><input type="number" min="1" step="1" v-model.number="form.qty_g" /></label>
        <label><span>成本/千克</span><input type="number" min="0" step="0.01" v-model.number="form.unit_cost" /></label>
        <label><span>产季</span><input v-model.trim="form.crop_season" placeholder="2025/26" /></label>
        <label><span>产地</span><input v-model.trim="form.origin" placeholder="云南保山" /></label>
        <label class="span-2"><span>产家风味描述</span><input v-model.trim="form.producer_flavor_description" placeholder="供应商/产家描述的风味" /></label>
        <label class="span-2"><span>备注</span><input v-model.trim="form.note" /></label>
        <button class="primary" type="button" @click="submit" :disabled="saving">提交入库</button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import SearchableSelect from '../components/SearchableSelect.vue'
import { receiptMaterialLabel, selectableReceiptMaterials } from '../lib/material-receipts'

const props = defineProps({
  embedded: { type: Boolean, default: false },
})

const materials = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref('')
const form = reactive({ material_id: 0, supplier: '', qty_g: 0, unit_cost: 0, crop_season: '', origin: '', producer_flavor_description: '', note: '' })
const materialOptions = computed(() => selectableReceiptMaterials(materials.value))

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
.stock-operation-page { padding:16px; display:grid; gap:16px; }
.stock-operation-page.embedded { padding:0; }
.panel { border:1px solid #e5e7eb; border-radius:8px; padding:12px; background:#fff; }
.panel-head { display:flex; justify-content:space-between; align-items:flex-start; gap:12px; margin-bottom:12px; }
.panel-head h2 { margin:0 0 4px; font-size:18px; }
.panel-head p { margin:0; color:#6b7280; font-size:13px; }
.operation-grid { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:10px; align-items:end; }
.span-2 { grid-column:span 2; }
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
@media (max-width:900px){ .stock-operation-page{padding:12px;} .operation-grid{grid-template-columns:1fr;} .span-2{grid-column:auto;} }
</style>
