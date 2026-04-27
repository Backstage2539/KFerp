<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>产品设置</h2>
          <p>合并商品基础信息、产品分类和价格试算。商用阶梯价由价格发布生成。</p>
        </div>
        <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <section class="settings-grid">
      <div class="panel category-panel">
        <div class="panel-title">
          <span>商品分类</span>
          <button class="toggle-section" type="button" @click="categoryCollapsed = !categoryCollapsed">
            {{ categoryCollapsed ? '展开' : '收起' }}
          </button>
        </div>
        <div v-show="!categoryCollapsed">
          <form class="inline-form" @submit.prevent="savePrimaryCategory">
            <input v-model.trim="newPrimaryName" placeholder="新增一级分类，如 咖啡豆" />
            <button class="secondary" type="submit">新增一级分类</button>
          </form>

          <div class="category-tree">
            <div
              v-for="primary in categories"
              :key="primary.id"
              class="primary-category"
              @dragover.prevent
              @drop="dropCategoryOnPrimary(primary)">
              <form v-if="editingCategoryId === primary.id" class="inline-form sub-form" @submit.prevent="saveCategoryName(primary)">
                <input v-model.trim="editingCategoryName" placeholder="一级分类名称" />
                <button class="secondary" type="submit">保存</button>
              </form>
              <div v-else class="category-head">
                <strong>{{ primary.number }}. {{ primary.name }}</strong>
                <div class="category-actions">
                  <button class="text-button" type="button" @click="startCategoryEdit(primary)">改名</button>
                  <button class="text-button" type="button" @click="startAddingSecondary(primary)">新增二级</button>
                </div>
              </div>

              <form v-if="addingSecondaryFor === primary.id" class="inline-form sub-form" @submit.prevent="saveSecondaryCategory(primary)">
                <input v-model.trim="newSecondaryName" placeholder="新增二级分类，如 意式拼配" />
                <button class="secondary" type="submit">保存</button>
              </form>

              <div
                class="category-drop-line"
                :class="{ active: isCategoryDropTarget(primary, 1) }"
                @dragover.prevent.stop="setCategoryDropTarget(primary, 1)"
                @drop.prevent.stop="dropCategoryAtPosition(primary, 1)">
              </div>

              <template v-for="(secondary, index) in primary.children" :key="secondary.id">
                <div
                  class="secondary-category"
                  :class="{ dragging: isDraggingCategory(secondary) }"
                  draggable="true"
                  @dragstart="startCategoryDrag(secondary)"
                  @dragend="scheduleClearDrag"
                  @dragover.prevent.stop
                  @drop.prevent.stop="dropCategoryOrProductOnSecondary(primary, index + 1, secondary)">
                  <form v-if="editingCategoryId === secondary.id" class="inline-form sub-form" @submit.prevent="saveCategoryName(secondary)">
                    <input v-model.trim="editingCategoryName" placeholder="二级分类名称" />
                    <button class="secondary" type="submit">保存</button>
                  </form>
                  <div v-else class="secondary-head">
                    <span>{{ secondary.number }}</span>
                    <b>{{ secondary.name }}</b>
                    <small>{{ secondary.products.length }} 款</small>
                    <button class="text-button" type="button" @click="startCategoryEdit(secondary)">改名</button>
                  </div>
                  <div class="product-chip-list">
                    <span
                      v-for="product in secondary.products"
                      :key="product.id"
                      class="product-chip"
                      draggable="true"
                      @dragstart.stop="startProductDrag(product)"
                      @dragend="scheduleClearDrag">
                      {{ product.number }}. {{ product.name }}
                    </span>
                  </div>
                </div>
                <div
                  class="category-drop-line"
                  :class="{ active: isCategoryDropTarget(primary, index + 2) }"
                  @dragover.prevent.stop="setCategoryDropTarget(primary, index + 2)"
                  @drop.prevent.stop="dropCategoryAtPosition(primary, index + 2)">
                </div>
              </template>
            </div>
          </div>

          <div class="uncategorized" @dragover.prevent @drop="dropProductOnSecondary({ id: 0 })">
            <div class="sub-title">未分类商品</div>
            <span
              v-for="product in uncategorizedProducts"
              :key="product.id"
              class="product-chip"
              draggable="true"
              @dragstart="startProductDrag(product)"
              @dragend="scheduleClearDrag">
              {{ product.name }}
            </span>
            <p v-if="!uncategorizedProducts.length" class="muted">暂无未分类商品</p>
          </div>
        </div>
      </div>

      <div class="panel product-panel">
        <div class="panel-title">
          <span>商品基础信息</span>
          <button class="toggle-section" type="button" @click="productsCollapsed = !productsCollapsed">
            {{ productsCollapsed ? '展开' : '收起' }}
          </button>
        </div>
        <div v-show="!productsCollapsed" class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>一级分类</th>
                <th>二级分类</th>
                <th>商品编号</th>
                <th>商品</th>
                <th>烘焙度</th>
                <th>BOM出品率</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in productRows" :key="row.id">
                <td>{{ categoryLabel(row, 1) }}</td>
                <td>{{ categoryLabel(row, 2) }}</td>
                <td>{{ row.number || '' }}</td>
                <td>{{ row.name }}</td>
                <td>
                  <select class="roast-select" v-model="row.roast_level" @change="saveProductBasics(row)">
                    <option v-for="level in roastLevels" :key="level" :value="level">{{ level }}</option>
                  </select>
                </td>
                <td>
                  <div class="yield-editor">
                    <input
                      class="yield-input"
                      v-model.number="row.yield_percent"
                      type="number"
                      min="1"
                      max="100"
                      step="0.01"
                      @change="saveProductBasics(row)" />
                    <span>%</span>
                  </div>
                </td>
              </tr>
              <tr v-if="!productRows.length">
                <td colspan="6" class="muted">暂无商品</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>

    <section class="panel costing-panel">
      <CostingView />
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import CostingView from './CostingView.vue'

const categories = ref([])
const products = ref([])
const loading = ref(false)
const error = ref('')
const ok = ref('')
const newPrimaryName = ref('')
const newSecondaryName = ref('')
const addingSecondaryFor = ref(0)
const dragging = ref(null)
const categoryDropTarget = ref(null)
const editingCategoryId = ref(0)
const editingCategoryName = ref('')
const categoryCollapsed = ref(false)
const productsCollapsed = ref(false)
const roastLevels = ['浅烘', '中烘', '中深烘', '深烘']

const productRows = computed(() => {
  const rows = []
  for (const primary of categories.value) {
    for (const secondary of primary.children || []) {
      for (const product of secondary.products || []) {
        rows.push({ ...product, primary_name: primary.name, secondary_name: secondary.name })
      }
    }
  }
  for (const product of uncategorizedProducts.value) {
    rows.push(product)
  }
  return rows
})

const categorizedProductIDs = computed(() => {
  const ids = new Set()
  for (const primary of categories.value) {
    for (const secondary of primary.children || []) {
      for (const product of secondary.products || []) ids.add(Number(product.id))
    }
  }
  return ids
})

const uncategorizedProducts = computed(() => products.value.filter((product) => !categorizedProductIDs.value.has(Number(product.id))))

function categoryLabel(row, level) {
  return level === 1 ? row.primary_name || '' : row.secondary_name || ''
}

function decorateProduct(product) {
  const yieldRate = Number(product.yield_rate || 0.8)
  return {
    ...product,
    roast_level: roastLevels.includes(product.roast_level) ? product.roast_level : '中烘',
    yield_rate: yieldRate,
    yield_percent: Number((yieldRate * 100).toFixed(2)),
  }
}

function decorateCategory(category) {
  return {
    ...category,
    children: (category.children || []).map(decorateCategory),
    products: (category.products || []).map(decorateProduct),
  }
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet('/api/product-settings')
    categories.value = (data.categories || []).map(decorateCategory)
    products.value = (data.products || []).map(decorateProduct)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function savePrimaryCategory() {
  if (!newPrimaryName.value) return
  await saveCategory({ name: newPrimaryName.value, parent_id: 0, position: categories.value.length + 1 })
  newPrimaryName.value = ''
}

function startAddingSecondary(primary) {
  addingSecondaryFor.value = Number(primary.id)
  newSecondaryName.value = ''
}

async function saveSecondaryCategory(primary) {
  if (!newSecondaryName.value) return
  await saveCategory({
    name: newSecondaryName.value,
    parent_id: Number(primary.id),
    position: Number(primary.children?.length || 0) + 1,
  })
  newSecondaryName.value = ''
  addingSecondaryFor.value = 0
}

async function saveCategory(body) {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend('/api/product-settings/categories', { body })
    ok.value = '分类已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存分类失败'
  } finally {
    loading.value = false
  }
}

function startCategoryEdit(category) {
  editingCategoryId.value = Number(category.id)
  editingCategoryName.value = category.name || ''
}

async function saveCategoryName(category) {
  if (!editingCategoryName.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/product-settings/categories/${category.id}`, {
      method: 'PUT',
      body: {
        name: editingCategoryName.value,
        parent_id: Number(category.parent_id || 0),
        position: Number(category.position || category.number || 1),
      },
    })
    editingCategoryId.value = 0
    editingCategoryName.value = ''
    ok.value = '分类已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存分类失败'
  } finally {
    loading.value = false
  }
}

function startCategoryDrag(category) {
  dragging.value = {
    type: 'category',
    id: Number(category.id),
    parentID: Number(category.parent_id || 0),
    position: Number(category.number || category.position || 0),
  }
  categoryDropTarget.value = null
}

function startProductDrag(product) {
  dragging.value = { type: 'product', id: Number(product.id) }
  categoryDropTarget.value = null
}

function clearDrag() {
  dragging.value = null
  categoryDropTarget.value = null
}

function scheduleClearDrag() {
  window.setTimeout(clearDrag, 0)
}

function isDraggingCategory(category) {
  return dragging.value?.type === 'category' && dragging.value.id === Number(category.id)
}

function setCategoryDropTarget(primary, position) {
  if (dragging.value?.type !== 'category') return
  categoryDropTarget.value = { parentID: Number(primary.id), position: Number(position) }
}

function isCategoryDropTarget(primary, position) {
  return dragging.value?.type === 'category'
    && categoryDropTarget.value?.parentID === Number(primary.id)
    && categoryDropTarget.value?.position === Number(position)
}

async function dropCategoryAtPosition(primary, visualPosition) {
  const drag = dragging.value
  if (drag?.type !== 'category') return
  const parentID = Number(primary.id)
  let position = Number(visualPosition)
  if (drag.parentID === parentID && drag.position > 0 && drag.position < visualPosition) {
    position -= 1
  }
  if (position <= 0) position = 1
  try {
    await apiSend(`/api/product-settings/categories/${drag.id}/move`, {
      body: { parent_id: parentID, position },
    })
    ok.value = '分类顺序已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '移动分类失败'
  } finally {
    clearDrag()
  }
}

async function dropCategoryOnPrimary(primary) {
  if (dragging.value?.type !== 'category') return
  await dropCategoryAtPosition(primary, Number(primary.children?.length || 0) + 1)
}

async function dropCategoryOrProductOnSecondary(primary, visualPosition, secondary) {
  const drag = dragging.value
  if (drag?.type === 'category') {
    const parentID = Number(primary.id)
    const position = categoryDropTarget.value?.parentID === parentID
      ? categoryDropTarget.value.position
      : visualPosition
    await dropCategoryAtPosition(primary, position)
    return
  }
  await dropProductOnSecondary(secondary)
}

async function dropProductOnSecondary(secondary) {
  const drag = dragging.value
  if (drag?.type !== 'product') return
  try {
    await apiSend(`/api/product-settings/products/${drag.id}/category`, {
      body: { category_id: Number(secondary.id || 0), position: Number(secondary.products?.length || 0) + 1 },
    })
    ok.value = '商品分类已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '移动商品失败'
  } finally {
    clearDrag()
  }
}

async function saveProductBasics(row) {
  const yieldPercent = Number(row.yield_percent || 0)
  if (yieldPercent <= 0 || yieldPercent > 100) {
    error.value = 'BOM出品率必须在 1% 到 100% 之间'
    return
  }
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/products/${row.id}`, {
      method: 'PUT',
      body: {
        roast_level: row.roast_level,
        yield_rate: Number((yieldPercent / 100).toFixed(4)),
        retail_price_100g: Number(row.retail_price_100g || 0),
        retail_price_200g: Number(row.retail_price_200g || 0),
        retail_price_227g: Number(row.retail_price_227g || 0),
        retail_price_250g: Number(row.retail_price_250g || 0),
      },
    })
    ok.value = '商品基础信息已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存商品失败'
  } finally {
    loading.value = false
  }
}

onMounted(loadAll)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; display: grid; gap: 14px; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; }
.panel-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
.panel-head h2 { margin: 0 0 4px; font-size: 20px; }
.panel-head p { margin: 0; color: #666; font-size: 13px; }
.panel-title { display: flex; align-items: center; justify-content: space-between; gap: 10px; font-weight: 700; margin-bottom: 10px; }
.sub-title { width: 100%; font-weight: 700; font-size: 13px; }
button, input, select { font: inherit; min-height: 36px; border-radius: 6px; }
input, select { border: 1px solid #cfc8bf; padding: 7px 9px; background: #fff; width: 100%; }
button { border: 1px solid #1f1f1f; background: #fff; padding: 0 12px; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #111; color: #fff; }
.secondary, .toggle-section { background: #fff; color: #111; }
.toggle-section { min-height: 30px; padding: 0 10px; }
.text-button { border: 0; background: transparent; color: #1f4f82; padding: 0; min-height: 28px; }
.settings-grid { display: grid; grid-template-columns: minmax(280px, 380px) minmax(0, 1fr); gap: 14px; align-items: start; }
.inline-form { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; margin-bottom: 10px; }
.sub-form { margin: 8px 0; }
.category-tree { display: grid; gap: 10px; }
.primary-category { border: 1px solid #eee8df; border-radius: 8px; padding: 10px; background: #fbfaf8; }
.category-head, .secondary-head, .category-actions { display: flex; align-items: center; gap: 8px; justify-content: space-between; }
.secondary-category { border: 1px solid #ddd; border-radius: 8px; padding: 9px; background: #fff; }
.secondary-category.dragging { opacity: .45; }
.secondary-head span { display: inline-flex; width: 24px; height: 24px; align-items: center; justify-content: center; border: 1px solid #ddd; border-radius: 6px; }
.secondary-head small { color: #666; }
.category-drop-line { height: 16px; border-top: 2px solid transparent; margin: 2px 0; transition: border-color .12s ease, background .12s ease; }
.category-drop-line.active { border-top-color: #1f4f82; background: #edf5ff; }
.product-chip-list, .uncategorized { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 8px; }
.product-chip { border: 1px solid #ddd; border-radius: 8px; padding: 5px 8px; background: #fff; font-size: 12px; cursor: grab; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 760px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 8px; text-align: left; font-size: 13px; vertical-align: middle; }
th { background: #fbfaf8; position: sticky; top: 0; }
.roast-select { min-width: 92px; }
.yield-editor { display: flex; align-items: center; gap: 6px; max-width: 130px; }
.yield-input { width: 90px; }
.costing-panel { padding: 0; overflow: hidden; }
.error, .ok { border-radius: 6px; padding: 9px; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
.muted { color: #666; font-size: 12px; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .settings-grid, .inline-form { grid-template-columns: 1fr; }
  table { min-width: 720px; }
}
</style>
