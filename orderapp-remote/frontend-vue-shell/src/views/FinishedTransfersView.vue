<template>
  <div class="page">
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
      <div class="form-grid">
        <label>
          <span>成品</span>
          <select v-model.number="form.product_id">
            <option :value="0">请选择</option>
            <option v-for="item in products" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
        <label><span>规格(g)</span><input v-model.number="form.spec_g" type="number" min="1" step="1" /></label>
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
        <label><span>件数</span><input v-model.number="form.qty_units" type="number" min="0" step="1" /></label>
        <label><span>散装(g)</span><input v-model.number="form.qty_loose_g" type="number" min="0" step="1" /></label>
        <label class="wide"><span>备注</span><input v-model.trim="form.note" placeholder="门店备货/展会备货/仓库整理" /></label>
        <button class="primary" type="button" @click="submit" :disabled="saving">提交转仓</button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const products = ref([])
const warehouses = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref('')
const form = reactive({
  product_id: 0,
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

async function loadOptions() {
  loading.value = true
  error.value = ''
  try {
    const [prod, wh] = await Promise.all([apiGet('/api/products'), apiGet('/api/stock/warehouses')])
    products.value = prod.rows || prod.products || []
    warehouses.value = wh.rows || []
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
    const data = await apiSend('/api/stock/finished-transfers', { body: form })
    ok.value = data.transfer_no || data.transfer_id
  } catch (err) {
    error.value = err.message || '提交失败'
  } finally {
    saving.value = false
  }
}

onMounted(loadOptions)
</script>

<style scoped>
.page{padding:16px}.panel{border:1px solid #e5e7eb;border-radius:8px;background:#fff;padding:12px}.panel-head{display:flex;justify-content:space-between;align-items:flex-start;gap:12px;margin-bottom:12px}.panel-head h2{margin:0 0 4px;font-size:18px}.panel-head p{margin:0;color:#6b7280;font-size:13px}.form-grid{display:grid;grid-template-columns:minmax(220px,1.3fr) 100px 150px 150px 90px 100px;gap:10px;align-items:end}.wide{grid-column:span 5}label span{display:block;color:#666;font-size:12px;margin-bottom:5px}input,select,button{font:inherit;min-height:36px;border-radius:6px}input,select{width:100%;border:1px solid #d1d5db;padding:7px 9px}button{padding:8px 12px;cursor:pointer}.primary{border:1px solid #111;background:#111;color:#fff}.secondary{border:1px solid #9ca3af;background:#fff;color:#111}.error,.ok{border-radius:8px;padding:10px;margin-bottom:12px}.error{background:#ffecec;border:1px solid #ffb9b9}.ok{background:#e9ffe9;border:1px solid #b8f5b8}
@media (max-width:900px){.page{padding:12px}.form-grid{grid-template-columns:1fr}.wide{grid-column:auto}}
</style>
