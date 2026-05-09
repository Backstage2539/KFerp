<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>BOM配方维护</h2>
        <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
      <div class="filters">
        <label>
          <span>商品</span>
          <SearchableSelect
            v-model="selectedProductId"
            :options="products"
            :option-label="optionLabel"
            :option-value="optionNumericValue"
            placeholder="选择商品"
            empty-text="没有匹配商品"
            @select="selectProduct(optionNumericValue($event))" />
        </label>
        <button class="primary" type="button" @click="saveBom" :disabled="!selectedProductId || loading">同步出品率</button>
        <button class="secondary danger-outline" type="button" @click="deleteBom" :disabled="!selectedProductId || loading">失效当前 BOM</button>
      </div>
    </section>

    <div class="grid">
      <section class="panel list-panel">
        <div class="panel-title">商品 BOM</div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>商品</th>
                <th>烘焙度</th>
                <th>出品率</th>
                <th>状态</th>
                <th>物料数</th>
                <th>更新时间</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in rows"
                :key="row.product_id"
                :class="{ active: row.product_id === selectedProductId }"
                @click="selectProduct(row.product_id)">
                <td>{{ row.product }}</td>
                <td>{{ row.roast_level || '-' }}</td>
                <td>{{ pct(row.yield_rate) }}</td>
                <td><span :class="['status-pill', row.status === 'inactive' ? 'inactive' : '']">{{ bomStatusLabel(row.status) }}</span></td>
                <td>{{ row.item_count }}</td>
                <td>{{ row.updated_at }}</td>
              </tr>
              <tr v-if="!rows.length">
                <td colspan="6" class="muted">暂无商品</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="panel detail-panel">
        <div class="panel-title">配方明细</div>
        <div v-if="detail" class="summary">
          <div><span>商品</span><strong>{{ detail.product_name }}</strong></div>
          <div><span>烘焙度</span><strong>{{ detail.roast_level || '-' }}</strong></div>
          <div><span>出品率</span><strong>{{ pct(detail.yield_rate) }}</strong></div>
          <div><span>状态</span><strong :class="{ warn: detail.status === 'inactive' }">{{ bomStatusLabel(detail.status) }}</strong></div>
          <div><span>合计比例</span><strong :class="{ warn: detail.total_ratio > 100 }">{{ ratio(detail.total_ratio) }}</strong></div>
        </div>
        <div v-if="detail?.status === 'inactive'" class="warning-banner">当前 BOM 已失效，历史配方明细会保留；重新保存或启用版本后可恢复为有效 BOM。</div>
        <div v-if="!detail" class="muted empty">请选择商品</div>

        <form class="inline-form" @submit.prevent="saveItem">
          <label>
            <span>物料</span>
            <SearchableSelect
              v-model="itemForm.material_id"
              :options="materials"
              :option-label="optionLabel"
              :option-value="optionNumericValue"
              placeholder="选择物料"
              empty-text="没有匹配物料"
              :disabled="!detail" />
          </label>
          <label>
            <span>比例 %</span>
            <input v-model.number="itemForm.ratio_pct" type="number" min="0.01" max="100" step="0.01" :disabled="!detail" />
          </label>
          <button class="primary" type="submit" :disabled="!detail || loading">保存物料</button>
        </form>

        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>物料</th>
                <th>比例</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in detailItems" :key="item.id">
                <td>{{ item.material_name }}</td>
                <td>{{ ratio(item.ratio_pct) }}</td>
                <td><button class="text-button" type="button" @click="deleteItem(item.id)">删除</button></td>
              </tr>
              <tr v-if="!detailItems.length">
                <td colspan="3" class="muted">暂无物料</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>

    <section class="panel">
      <div class="panel-title">BOM版本</div>
      <div class="inline-form">
        <label>
          <span>版本备注</span>
          <input v-model.trim="versionNote" placeholder="例如 2026 春季豆单" :disabled="!selectedProductId" />
        </label>
        <button class="primary" type="button" @click="createVersion" :disabled="!selectedProductId || loading">保存当前为版本</button>
      </div>
      <div class="table-wrap compact">
        <table>
          <thead>
            <tr>
              <th>版本</th>
              <th>状态</th>
              <th>出品率</th>
              <th>物料数</th>
              <th>备注</th>
              <th>创建时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="version in versions" :key="version.id">
              <td>{{ version.version_no }}</td>
              <td>{{ version.status }}</td>
              <td>{{ pct(version.yield_rate) }}</td>
              <td>{{ version.item_count }}</td>
              <td>{{ version.note }}</td>
              <td>{{ version.created_at }}</td>
              <td><button class="text-button" type="button" @click="activateVersion(version.id)" :disabled="version.status === 'active'">启用</button></td>
            </tr>
            <tr v-if="!versions.length">
              <td colspan="7" class="muted">暂无版本</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="panel-title">规格袋材映射</div>
      <form class="inline-form" @submit.prevent="saveMapping">
        <label>
          <span>规格 g</span>
          <input v-model.number="mappingForm.spec_g" type="number" min="1" step="1" />
        </label>
        <label>
          <span>袋材物料</span>
          <select v-model.number="mappingForm.material_id">
            <option :value="0">选择物料</option>
            <option v-for="material in materials" :key="material.id" :value="material.id">{{ material.name }}</option>
          </select>
        </label>
        <button class="primary" type="submit" :disabled="loading">保存映射</button>
      </form>
      <div class="table-wrap compact">
        <table>
          <thead>
            <tr>
              <th>规格</th>
              <th>袋材物料</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="mapping in mappings" :key="mapping.spec_g">
              <td>{{ mapping.spec_g }}g</td>
              <td>{{ mapping.material_name }}</td>
              <td><button class="text-button" type="button" @click="deleteMapping(mapping.spec_g)">删除</button></td>
            </tr>
            <tr v-if="!mappings.length">
              <td colspan="3" class="muted">暂无映射</td>
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
import SearchableSelect from '../components/SearchableSelect.vue'
import { replaceHistoryURL } from '../lib/url-state'

const rows = ref([])
const products = ref([])
const materials = ref([])
const mappings = ref([])
const versions = ref([])
const detail = ref(null)
const selectedProductId = ref(0)
const loading = ref(false)
const error = ref('')
const ok = ref('')
const itemForm = reactive({ material_id: 0, ratio_pct: '' })
const mappingForm = reactive({ spec_g: 227, material_id: 0 })
const versionNote = ref('')

const detailItems = computed(() => detail.value?.items || [])

function pct(value) {
  const n = Number(value || 0) * 100
  return n ? `${n.toFixed(1)}%` : '-'
}

function ratio(value) {
  const n = Number(value || 0)
  return `${n.toFixed(2)}%`
}

function optionLabel(option) {
  return option?.name || ''
}

function optionNumericValue(option) {
  return Number(option?.id || 0)
}

function updateUrl() {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'bom')
  if (selectedProductId.value) url.searchParams.set('product_id', String(selectedProductId.value))
  else url.searchParams.delete('product_id')
  replaceHistoryURL(url)
}

async function loadAll() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const [listData, productData, materialData, mappingData] = await Promise.all([
      apiGet('/api/bom/list'),
      apiGet('/api/bom/products'),
      apiGet('/api/bom/materials'),
      apiGet('/api/bom/bag-spec-mappings'),
    ])
    rows.value = listData || []
    products.value = productData || []
    materials.value = materialData || []
    mappings.value = mappingData || []
    if (selectedProductId.value) await loadDetail(selectedProductId.value)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadDetail(productId) {
  if (!productId) {
    detail.value = null
    updateUrl()
    return
  }
  detail.value = await apiGet(`/api/bom/detail/${productId}`)
  await loadVersions(productId)
  updateUrl()
}

async function loadVersions(productId) {
  if (!productId) {
    versions.value = []
    return
  }
  const data = await apiGet(`/api/bom/versions?product_id=${productId}`)
  versions.value = data.rows || []
}

async function selectProduct(productId) {
  selectedProductId.value = Number(productId || 0)
  error.value = ''
  ok.value = ''
  try {
    await loadDetail(selectedProductId.value)
  } catch (err) {
    error.value = err.message || '加载失败'
  }
}

async function saveBom() {
  if (!selectedProductId.value) return
  await mutate(async () => {
    await apiSend('/api/bom/save', { body: { product_id: selectedProductId.value } })
    ok.value = '已同步'
    await loadAll()
  })
}

function bomStatusLabel(status) {
  if (status === 'inactive') return '已失效'
  if (status === 'missing') return '未维护'
  return '有效'
}

async function deleteBom() {
  if (!selectedProductId.value) return
  const okToDeactivate = window.confirm('确认失效当前 BOM？配方明细会保留，后续依赖该 BOM 的策略会提示 BOM 已失效。')
  if (!okToDeactivate) return
  await mutate(async () => {
    await apiSend(`/api/bom/${selectedProductId.value}`, { method: 'DELETE' })
    itemForm.material_id = 0
    itemForm.ratio_pct = ''
    ok.value = '当前 BOM 已失效'
    await loadAll()
  })
}

async function saveItem() {
  await mutate(async () => {
    await apiSend('/api/bom/item/save', {
      body: {
        product_id: selectedProductId.value,
        material_id: Number(itemForm.material_id || 0),
        ratio_pct: Number(itemForm.ratio_pct || 0),
      },
    })
    itemForm.material_id = 0
    itemForm.ratio_pct = ''
    ok.value = '已保存'
    await loadAll()
  })
}

async function deleteItem(id) {
  await mutate(async () => {
    await apiSend('/api/bom/item/delete', { body: { product_id: selectedProductId.value, id } })
    ok.value = '已删除'
    await loadAll()
  })
}

async function saveMapping() {
  await mutate(async () => {
    await apiSend('/api/bom/bag-spec-mappings/save', {
      body: {
        spec_g: Number(mappingForm.spec_g || 0),
        material_id: Number(mappingForm.material_id || 0),
      },
    })
    mappingForm.material_id = 0
    ok.value = '已保存映射'
    await loadAll()
  })
}

async function deleteMapping(specG) {
  await mutate(async () => {
    await apiSend('/api/bom/bag-spec-mappings/delete', { body: { spec_g: specG } })
    ok.value = '已删除映射'
    await loadAll()
  })
}

async function createVersion() {
  await mutate(async () => {
    await apiSend('/api/bom/versions', { body: { product_id: selectedProductId.value, note: versionNote.value } })
    versionNote.value = ''
    ok.value = '已保存版本'
    await loadVersions(selectedProductId.value)
  })
}

async function activateVersion(id) {
  await mutate(async () => {
    await apiSend(`/api/bom/versions/${id}/activate`, { body: {} })
    ok.value = '已启用版本'
    await loadAll()
  })
}

async function mutate(action) {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await action()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  const params = new URL(window.location.href).searchParams
  selectedProductId.value = Number(params.get('product_id') || 0)
  loadAll()
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .filters, .inline-form, .summary { display: flex; align-items: end; gap: 10px; flex-wrap: wrap; }
.panel-head { justify-content: space-between; align-items: center; margin-bottom: 12px; }
.panel-title { font-size: 16px; font-weight: 700; margin-bottom: 10px; }
h2 { margin: 0; font-size: 20px; }
.grid { display: grid; grid-template-columns: minmax(360px, 0.9fr) minmax(420px, 1.1fr); gap: 14px; align-items: start; }
label span, .summary span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select { height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; min-width: 180px; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.danger-outline { border-color: #9d2626; color: #9d2626; }
.text-button { height: 30px; border: 0; background: transparent; color: #1f4f82; padding: 0; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 640px; border-collapse: collapse; }
.compact table { min-width: 520px; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
tbody tr.active { background: #f3f7fb; }
.list-panel tbody tr { cursor: pointer; }
.summary { align-items: stretch; margin-bottom: 12px; }
.summary div { min-width: 120px; border: 1px solid #eee8df; border-radius: 6px; padding: 9px; }
.summary strong { font-size: 16px; }
.warning-banner { border: 1px solid #e8c28f; border-radius: 6px; background: #fff8eb; color: #8a4b00; padding: 9px; margin-bottom: 12px; }
.inline-form { margin: 12px 0; }
.muted { color: #666; text-align: center; }
.empty { padding: 22px; border: 1px dashed #d8d0c7; border-radius: 8px; }
.warn { color: #a13b00; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
.status-pill { display: inline-flex; align-items: center; min-height: 24px; border: 1px solid #cfd8cf; border-radius: 999px; padding: 2px 8px; color: #27602e; background: #f2fbf2; white-space: nowrap; }
.status-pill.inactive { border-color: #e1b6b6; color: #8a1f1f; background: #fff0f0; }
@media (max-width: 1100px) { .grid { grid-template-columns: 1fr; } }
@media (max-width: 900px) { .page { padding: 12px; } table { min-width: 620px; } }
</style>
