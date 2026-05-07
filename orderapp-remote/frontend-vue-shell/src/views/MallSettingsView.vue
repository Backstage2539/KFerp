<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>商城管理</h2>
        <div class="actions">
          <button class="secondary" type="button" @click="loadMallProducts" :disabled="loading">刷新</button>
          <button class="primary" type="button" @click="startNewProduct">新增商品</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <section class="workspace">
      <div class="product-list">
        <div class="list-head">
          <span>商品</span>
          <span>状态</span>
          <span>排序</span>
        </div>
        <button
          v-for="row in rows"
          :key="row.id || `${row.product_id}-${row.sort_order}`"
          type="button"
          class="product-row"
          :class="{ active: current?.id === row.id && row.id > 0 }"
          @click="selectProduct(row)">
          <span>
            <strong>{{ row.title || '未命名商品' }}</strong>
            <small>{{ row.subtitle || `${row.spec_g}g` }}</small>
          </span>
          <i :class="['status-dot', row.status]"></i>
          <span>{{ row.sort_order }}</span>
        </button>
        <div v-if="!rows.length && !loading" class="empty">暂无商品</div>
      </div>

      <form class="editor" @submit.prevent="saveProduct">
        <div class="editor-grid">
          <label>
            <span>ERP 产品</span>
            <select v-model.number="current.product_id" @change="applySelectedOption">
              <option :value="0">请选择产品</option>
              <option v-for="option in productOptions" :key="option.id" :value="option.id">
                {{ option.name }}
              </option>
            </select>
          </label>
          <label>
            <span>标题</span>
            <input v-model.trim="current.title" />
          </label>
          <label>
            <span>副标题</span>
            <input v-model.trim="current.subtitle" />
          </label>
          <label>
            <span>规格 g</span>
            <input v-model.number="current.spec_g" type="number" min="1" />
          </label>
          <label>
            <span>售价</span>
            <input v-model.number="current.unit_price" type="number" min="0" step="0.01" />
          </label>
          <label>
            <span>排序</span>
            <input v-model.number="current.sort_order" type="number" min="1" />
          </label>
          <label>
            <span>模板</span>
            <select v-model="current.template_key">
              <option v-for="option in mallTemplateOptions" :key="option.key" :value="option.key">
                {{ option.label }}
              </option>
            </select>
          </label>
          <label>
            <span>状态</span>
            <select v-model="current.status">
              <option v-for="option in mallStatusOptions" :key="option.key" :value="option.key">
                {{ option.label }}
              </option>
            </select>
          </label>
          <label class="wide">
            <span>图片 URL</span>
            <input v-model.trim="current.image_url" />
          </label>
          <label class="wide">
            <span>描述</span>
            <textarea v-model.trim="current.description" rows="4"></textarea>
          </label>
        </div>

        <div class="upload-row">
          <input :disabled="!current.id" type="file" accept="image/*" @change="uploadImage" />
          <button class="primary" type="submit" :disabled="saving">{{ saving ? '保存中' : '保存商品' }}</button>
        </div>
      </form>

      <aside class="preview">
        <h3>商城预览</h3>
        <div :class="['preview-card', preview.template_key]">
          <img v-if="preview.image_url" :src="preview.image_url" alt="" />
          <div v-else class="image-empty"></div>
          <div class="preview-body">
            <strong>{{ preview.title || '未命名商品' }}</strong>
            <span>{{ preview.subtitle || `${preview.spec_g}g` }}</span>
            <p>{{ preview.description || '商品描述' }}</p>
            <b>{{ formatMallPrice(preview.unit_price) }}</b>
          </div>
        </div>
      </aside>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import {
  createBlankMallProduct,
  formatMallPrice,
  mallStatusOptions,
  mallTemplateOptions,
  normalizeMallProduct,
  optionForProduct,
} from '../lib/customer-mall'

const rows = ref([])
const productOptions = ref([])
const current = ref(createBlankMallProduct())
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref('')

const preview = computed(() => normalizeMallProduct(current.value, productOptions.value))

async function loadMallProducts() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiGet('/api/customer-portal/admin/mall-products')
    productOptions.value = data.product_options || []
    rows.value = (data.rows || []).map((item) => normalizeMallProduct(item, productOptions.value))
    if (rows.value.length) {
      selectProduct(rows.value[0])
    } else {
      startNewProduct()
    }
  } catch (err) {
    error.value = err.message || '加载商城商品失败'
  } finally {
    loading.value = false
  }
}

function selectProduct(row) {
  current.value = normalizeMallProduct({ ...row }, productOptions.value)
}

function startNewProduct() {
  current.value = createBlankMallProduct(productOptions.value)
}

function applySelectedOption() {
  const option = optionForProduct(current.value.product_id, productOptions.value)
  if (!option) return
  if (!current.value.title) current.value.title = option.name || ''
  if (!current.value.unit_price) current.value.unit_price = Number(option.default_price || 0)
}

async function saveProduct() {
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    const body = normalizeMallProduct(current.value, productOptions.value)
    let saved
    if (current.value.id) {
      saved = await apiSend(`/api/customer-portal/admin/mall-products/${current.value.id}`, { method: 'PUT', body })
    } else {
      saved = await apiSend('/api/customer-portal/admin/mall-products', { body })
    }
    current.value = normalizeMallProduct(saved, productOptions.value)
    upsertRow(current.value)
    ok.value = '商城商品已保存'
  } catch (err) {
    error.value = err.message || '保存商城商品失败'
  } finally {
    saving.value = false
  }
}

async function uploadImage(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file || !current.value.id) return
  error.value = ''
  ok.value = ''
  const body = new FormData()
  body.append('file', file)
  try {
    const saved = await apiSend(`/api/customer-portal/admin/mall-products/${current.value.id}/image`, { body })
    current.value = normalizeMallProduct({ ...current.value, image_url: saved.image_url }, productOptions.value)
    upsertRow(current.value)
    ok.value = '商品图片已更新'
  } catch (err) {
    error.value = err.message || '上传图片失败'
  }
}

function upsertRow(row) {
  const normalized = normalizeMallProduct(row, productOptions.value)
  const index = rows.value.findIndex((item) => item.id === normalized.id)
  if (index >= 0) {
    rows.value.splice(index, 1, normalized)
  } else {
    rows.value.unshift(normalized)
  }
}

onMounted(loadMallProducts)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel, .workspace { border: 1px solid #e1e5ea; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.actions { display: flex; gap: 8px; flex-wrap: wrap; }
h2, h3 { margin: 0; }
h2 { font-size: 20px; }
h3 { font-size: 16px; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled, input:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.workspace { display: grid; grid-template-columns: 300px minmax(420px, 1fr) 320px; gap: 14px; align-items: start; }
.list-head, .product-row { display: grid; grid-template-columns: minmax(0, 1fr) 52px 48px; gap: 8px; align-items: center; }
.list-head { min-height: 34px; color: #666; font-size: 12px; font-weight: 700; border-bottom: 1px solid #eef1f4; }
.product-row { width: 100%; min-height: 64px; margin-top: 6px; border-color: #e4e7ec; background: #fff; text-align: left; color: #171717; }
.product-row.active { border-color: #1f1f1f; box-shadow: 0 0 0 2px rgba(31,31,31,.07); }
.product-row strong, .product-row small { display: block; min-width: 0; overflow-wrap: anywhere; }
.product-row small { margin-top: 4px; color: #666; font-size: 12px; }
.status-dot { width: 34px; height: 22px; border-radius: 999px; justify-self: start; background: #ccd4dd; }
.status-dot.published { background: #2c7a55; }
.editor { min-width: 0; }
.editor-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select, textarea { width: 100%; border: 1px solid #cfc8bf; border-radius: 6px; padding: 8px 9px; font: inherit; background: #fff; }
input, select { height: 38px; }
textarea { resize: vertical; line-height: 1.45; }
.wide { grid-column: 1 / -1; }
.upload-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; margin-top: 12px; }
.preview { min-width: 0; }
.preview-card { margin-top: 10px; border: 1px solid #e4e7ec; border-radius: 8px; overflow: hidden; background: #fafafa; }
.preview-card img, .image-empty { width: 100%; aspect-ratio: 4 / 3; object-fit: cover; background: linear-gradient(135deg, #eef1f4, #dbe7df); }
.preview-card.compact { display: grid; grid-template-columns: 120px minmax(0, 1fr); }
.preview-card.compact img, .preview-card.compact .image-empty { height: 100%; aspect-ratio: auto; }
.preview-card.wide img, .preview-card.wide .image-empty { aspect-ratio: 16 / 7; }
.preview-body { padding: 12px; display: flex; flex-direction: column; gap: 6px; }
.preview-body strong { font-size: 18px; line-height: 1.25; overflow-wrap: anywhere; }
.preview-body span, .preview-body p { color: #666; line-height: 1.45; margin: 0; }
.preview-body b { font-size: 20px; color: #8a4b1f; }
.empty { min-height: 80px; display: flex; align-items: center; justify-content: center; color: #666; }
.error, .ok { border-radius: 6px; padding: 9px; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
@media (max-width: 1180px) {
  .workspace { grid-template-columns: 1fr; }
  .product-list { order: 2; }
  .preview { order: 3; }
}
@media (max-width: 620px) {
  .page { padding: 12px; }
  .editor-grid { grid-template-columns: 1fr; }
  .preview-card.compact { grid-template-columns: 1fr; }
}
</style>
