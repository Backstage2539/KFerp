<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>{{ printMode ? '报价导出' : '商品基础信息' }}</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <section v-if="editorVisible && !printMode" class="panel">
      <div class="panel-head">
        <h3>编辑商品基础信息</h3>
        <button class="secondary" type="button" @click="closeEditor">关闭</button>
      </div>
      <form class="form" @submit.prevent="saveProduct">
        <div class="form-row">
          <label>
            <span>商品</span>
            <input v-model="form.name" disabled />
          </label>
          <label>
            <span>烘焙度</span>
            <select v-model="form.roast_level" required>
              <option v-for="level in roastLevels" :key="level" :value="level">{{ level }}</option>
            </select>
          </label>
        </div>

        <div class="price-grid">
          <label>
            <span>100g 零售价</span>
            <input v-model.number="form.retail_price_100g" type="number" min="0" step="0.01" />
          </label>
          <label>
            <span>200g 零售价</span>
            <input v-model.number="form.retail_price_200g" type="number" min="0" step="0.01" />
          </label>
          <label>
            <span>227g 零售价</span>
            <input v-model.number="form.retail_price_227g" type="number" min="0" step="0.01" />
          </label>
          <label>
            <span>250g 零售价</span>
            <input v-model.number="form.retail_price_250g" type="number" min="0" step="0.01" />
          </label>
        </div>

        <p class="hint">商用阶梯价由产品设置里的成本核算发布生成，这里只维护商品基础售价。</p>
        <div class="form-actions">
          <button class="primary" type="submit" :disabled="loading">保存</button>
        </div>
      </form>
    </section>

    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>商品</th>
              <th>烘焙度</th>
              <th>默认价</th>
              <th>100g</th>
              <th>200g</th>
              <th>227g</th>
              <th>250g</th>
              <th v-if="!printMode">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id" :class="{ active: row.id === editingId }">
              <td>{{ row.name }}</td>
              <td>{{ row.roast_level }}</td>
              <td>{{ money(row.default_price) }}</td>
              <td>{{ money(row.retail_price_100g) }}</td>
              <td>{{ money(row.retail_price_200g) }}</td>
              <td>{{ money(row.retail_price_227g) }}</td>
              <td>{{ money(row.retail_price_250g) }}</td>
              <td v-if="!printMode"><button class="text-button" type="button" @click="editProduct(row.id)">编辑基础信息</button></td>
            </tr>
            <tr v-if="!rows.length">
              <td :colspan="printMode ? 7 : 8" class="muted">暂无商品</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { replaceHistoryURL } from '../lib/url-state'

const props = defineProps({
  viewKey: { type: String, default: 'products' },
})

const rows = ref([])
const loading = ref(false)
const error = ref('')
const ok = ref('')
const editorVisible = ref(false)
const editingId = ref(0)
const roastLevels = ['浅烘', '中烘', '中深烘', '深烘']
const form = reactive({
  name: '',
  roast_level: '中烘',
  retail_price_100g: 0,
  retail_price_200g: 0,
  retail_price_227g: 0,
  retail_price_250g: 0,
})
const printMode = computed(() => props.viewKey === 'quotePrint')

function money(value) {
  const n = Number(value || 0)
  return n > 0 ? n.toFixed(2) : ''
}

function updateUrl() {
  const url = new URL(window.location.href)
  url.searchParams.set('view', props.viewKey)
  if (editingId.value) url.searchParams.set('edit_id', String(editingId.value))
  else url.searchParams.delete('edit_id')
  replaceHistoryURL(url)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet('/api/products')
    rows.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function editProduct(id) {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiGet(`/api/products/${id}`)
    const product = data.product
    editorVisible.value = true
    editingId.value = Number(product.id)
    form.name = product.name || ''
    form.roast_level = roastLevels.includes(product.roast_level) ? product.roast_level : '中烘'
    form.retail_price_100g = Number(product.retail_price_100g || 0)
    form.retail_price_200g = Number(product.retail_price_200g || 0)
    form.retail_price_227g = Number(product.retail_price_227g || 0)
    form.retail_price_250g = Number(product.retail_price_250g || 0)
    updateUrl()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function closeEditor() {
  editorVisible.value = false
  editingId.value = 0
  updateUrl()
}

async function saveProduct() {
  if (!editingId.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const body = {
      roast_level: form.roast_level,
      retail_price_100g: Number(form.retail_price_100g || 0),
      retail_price_200g: Number(form.retail_price_200g || 0),
      retail_price_227g: Number(form.retail_price_227g || 0),
      retail_price_250g: Number(form.retail_price_250g || 0),
    }
    await apiSend(`/api/products/${editingId.value}`, { method: 'PUT', body })
    ok.value = '已保存'
    await load()
    await editProduct(editingId.value)
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  const params = new URL(window.location.href).searchParams
  const editID = Number(params.get('edit_id') || 0)
  await load()
  if (!printMode.value && editID > 0) await editProduct(editID)
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
h2, h3 { margin: 0; font-size: 20px; }
h3 { font-size: 18px; }
.hint { margin: 0; color: #666; font-size: 13px; line-height: 1.45; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.text-button { height: 30px; border: 0; background: transparent; color: #1f4f82; padding: 0; }
.form { display: grid; gap: 14px; }
.form-row, .price-grid { display: grid; grid-template-columns: repeat(2, minmax(220px, 1fr)); gap: 12px; }
.price-grid { grid-template-columns: repeat(4, minmax(150px, 1fr)); }
.form-actions { display: flex; justify-content: flex-end; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 860px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
tr.active { background: #f3f7fb; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .form-row, .price-grid { grid-template-columns: 1fr; }
  table { min-width: 900px; }
}
</style>
