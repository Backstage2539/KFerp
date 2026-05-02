<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>成品库存</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存</div>
      <div class="filters">
        <label><span>搜索</span><input v-model.trim="q" placeholder="商品名" @keyup.enter="load" /></label>
        <button class="primary" type="button" @click="load" :disabled="loading">查询</button>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">新增/覆盖库存</div>
      <div class="form-grid">
        <label>
          <span>商品</span>
          <select v-model.number="form.product_id">
            <option :value="0">请选择</option>
            <option v-for="p in products" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </label>
        <label><span>规格(g)</span><input v-model.number="form.spec_g" type="number" min="1" /></label>
        <label><span>件数</span><input v-model.number="form.units" type="number" min="0" /></label>
        <label><span>散装(g)</span><input v-model.number="form.loose_g" type="number" min="0" /></label>
        <button class="primary" type="button" @click="save" :disabled="saving">保存</button>
      </div>
    </section>

    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead><tr><th>商品</th><th>规格(g)</th><th>件数</th><th>散装(g)</th><th>合计(g)</th><th>更新时间</th></tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="`${row.product_id}-${row.spec_g}`">
              <td>{{ row.product }}</td>
              <td>{{ row.spec_g }}</td>
              <td>{{ row.units }}</td>
              <td>{{ row.loose_g }}</td>
              <td>{{ row.total_g }}</td>
              <td>{{ row.updated_at }}</td>
            </tr>
            <tr v-if="!rows.length"><td colspan="6" class="muted">暂无库存</td></tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const rows = ref([])
const products = ref([])
const q = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const form = reactive({ product_id: 0, spec_g: 227, units: 0, loose_g: 0 })

async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/products/inventory', window.location.origin)
    if (q.value) url.searchParams.set('q', q.value)
    const data = await apiGet(url)
    rows.value = data.rows || []
    products.value = data.products || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    await apiSend('/api/products/inventory', { body: form })
    ok.value = true
    await load()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .filters { display: flex; align-items: center; gap: 10px; }
.panel-head { justify-content: space-between; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
.section-title { font-weight: 700; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: 1fr 110px 110px 110px 84px; gap: 10px; align-items: end; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 760px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; }
th { background: #fbfaf8; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) { .page { padding: 12px; } .form-grid { grid-template-columns: 1fr; } }
</style>
