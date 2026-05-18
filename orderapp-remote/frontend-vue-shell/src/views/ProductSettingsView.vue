<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>SKU设置</h2>
          <p>维护公共 SKU、客户专属 SKU、商品分类和梯度模板；豆单生成请进入产品豆单。</p>
        </div>
        <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <section class="settings-grid">
      <div class="panel sku-context-panel">
        <div class="sku-context-main">
          <div>
            <div class="context-eyebrow">SKU归属</div>
            <h3>{{ selectedSkuContextLabel }}</h3>
            <p class="muted">产品列表、商品分类和梯度模板会按当前归属切换。</p>
          </div>
          <div class="sku-context-controls">
            <button class="secondary compact-action" type="button" @click="selectedCustomerSkuCustomerID = 0" :disabled="!selectedCustomerSkuCustomerID">
              公共SKU
            </button>
            <SearchableSelect
              class="sku-customer-select"
              v-model="selectedCustomerSkuCustomerID"
              :options="customerSkuCustomers"
              :option-label="customerOptionLabel"
              :option-meta="customerOptionMeta"
              :option-value="optionNumericValue"
              placeholder="选择客户SKU"
              empty-text="暂无自定义SKU客户" />
          </div>
        </div>
        <div class="context-stats">
          <span>公共SKU {{ publicSkuRows.length }}</span>
          <span>当前SKU {{ displaySkuRows.length }}</span>
          <span>商品分类 {{ categoryTreeForSkuContext.length }}</span>
        </div>
      </div>

      <div class="panel public-product-panel">
        <div class="panel-title">
          <span>新增公共产品</span>
        </div>
        <form class="product-create-form" @submit.prevent="createProduct">
          <label class="wide-field">
            <span>商品名称</span>
            <input v-model.trim="productForm.name" placeholder="如 花魁 SOE" />
          </label>
          <label>
            <span>产品形态</span>
            <select v-model="productForm.product_kind">
              <option value="roasted">熟豆</option>
              <option value="green_bean">生豆</option>
            </select>
          </label>
          <label v-if="productForm.product_kind !== 'green_bean'">
            <span>烘焙度</span>
            <select v-model="productForm.roast_level">
              <option v-for="level in roastLevels" :key="level" :value="level">{{ level }}</option>
            </select>
          </label>
          <label v-if="productForm.product_kind !== 'green_bean'">
            <span>BOM出品率</span>
            <div class="yield-editor">
              <input
                class="yield-input"
                v-model.number="productForm.yield_percent"
                type="number"
                min="1"
                max="100"
                step="0.01" />
              <span>%</span>
            </div>
          </label>
          <label v-if="productForm.product_kind === 'green_bean'">
            <span>生豆属性</span>
            <select v-model="productForm.green_bean_type">
              <option v-for="option in greenBeanTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </label>
          <label v-if="productForm.product_kind === 'green_bean'" class="wide-field">
            <span>绑定熟豆</span>
            <SearchableSelect
              v-model="productForm.green_bean_bom_product_id"
              :options="roastedBomProducts"
              :option-label="baseProductOptionLabel"
              :option-meta="baseProductOptionMeta"
              :option-value="optionNumericValue"
              placeholder="选择对应熟豆"
              empty-text="暂无熟豆产品" />
          </label>
          <div class="form-actions">
            <button class="primary" type="submit" :disabled="productSaving">创建公共产品</button>
          </div>
        </form>
      </div>

      <div class="panel custom-product-panel">
        <div class="panel-title">
          <span>客户专属 SKU</span>
        </div>
        <form class="custom-product-form" @submit.prevent="createCustomProduct">
          <label>
            <span>客户</span>
            <SearchableSelect
              v-model="customForm.customer_id"
              :options="customers"
              :option-label="customerOptionLabel"
              :option-meta="customerOptionMeta"
              :option-value="optionNumericValue"
              placeholder="输入客户名/拼音"
              empty-text="没有匹配客户"
              @select="fillCustomProductName" />
          </label>
          <label>
            <span>基础产品</span>
            <SearchableSelect
              v-model="customForm.base_product_id"
              :options="baseProducts"
              :option-label="baseProductOptionLabel"
              :option-meta="baseProductOptionMeta"
              :option-value="optionNumericValue"
              placeholder="输入产品名"
              empty-text="没有匹配产品"
              @select="fillCustomProductName" />
          </label>
          <label>
            <span>定制类型</span>
            <select v-model="customForm.custom_type">
              <option value="public_sku_alias">公共 SKU 改名</option>
              <option value="custom_roast">定制烘焙度</option>
              <option value="custom_blend">定制拼配 BOM</option>
            </select>
          </label>
          <label>
            <span>烘焙度</span>
            <select v-model="customForm.roast_level" @change="fillCustomProductName">
              <option v-for="level in roastLevels" :key="level" :value="level">{{ level }}</option>
            </select>
          </label>
          <label class="wide-field">
            <span>专属 SKU 名称</span>
            <input v-model.trim="customForm.name" placeholder="如 客户A-暖阳拼配-中深烘" />
          </label>
          <label class="checkline">
            <input v-model="customForm.copy_bom" type="checkbox" />
            <span>复制基础产品 BOM</span>
          </label>
          <label class="checkline">
            <input v-model="customForm.copy_price_tiers" type="checkbox" />
            <span>复制基础产品价格梯度</span>
          </label>
          <div class="form-actions">
            <button class="primary" type="submit" :disabled="customSaving">创建专属 SKU</button>
          </div>
        </form>
      </div>

      <div class="panel gradient-template-panel">
        <div class="panel-title">
          <span>梯度模板</span>
          <button class="secondary compact-action" type="button" @click="resetGradientTemplateForm">新建模板</button>
        </div>
        <div class="gradient-template-layout">
          <div class="template-list">
            <button
              v-for="template in gradientTemplates"
              :key="template.id"
              type="button"
              :class="['template-row', { active: Number(template.id) === Number(templateForm.id), inactive: template.active === false }]"
              @click="startGradientTemplateEdit(template)">
              <strong>{{ template.name }}</strong>
              <small>{{ gradientDisplayUnitLabel(template.display_unit) }} · {{ template.tiers.length }} 档</small>
            </button>
            <p v-if="!gradientTemplates.length" class="muted">暂无梯度模板</p>
          </div>
          <form class="template-editor" @submit.prevent="saveGradientTemplate">
            <div class="template-editor-grid">
              <label>
                <span>模板名称</span>
                <input v-model.trim="templateForm.name" placeholder="如 工厂量单模板" />
              </label>
              <label>
                <span>展示单位</span>
                <select v-model="templateForm.display_unit">
                  <option v-for="unit in gradientDisplayUnitOptions" :key="unit.value" :value="unit.value">{{ unit.label }}</option>
                </select>
              </label>
            </div>
            <div class="template-tier-head">
              <strong>梯度档位</strong>
              <button class="secondary compact-action" type="button" @click="addGradientTemplateTier">新增档位</button>
            </div>
            <div class="template-tier-list">
              <div v-for="(tier, index) in templateForm.tiers" :key="`tier-${index}`" class="template-tier-row">
                <label>
                  <span>区间名</span>
                  <input v-model.trim="tier.label" placeholder="24-49kg" />
                </label>
                <label>
                  <span>最小数量（{{ gradientDisplayQuantityUnitLabel(templateForm.display_unit) }}）</span>
                  <input v-model.number="tier.min_display_qty" type="number" min="0" :step="gradientDisplayQuantityStep(templateForm.display_unit)" />
                </label>
                <label>
                  <span>最大数量（{{ gradientDisplayQuantityUnitLabel(templateForm.display_unit) }}）</span>
                  <input v-model="tier.max_display_qty" type="number" min="0" :step="gradientDisplayQuantityStep(templateForm.display_unit)" placeholder="无上限" />
                </label>
                <label>
                  <span>利润率</span>
                  <input v-model.number="tier.margin_rate" type="number" min="0" step="0.001" />
                </label>
                <button class="text-button danger-text" type="button" @click="removeGradientTemplateTier(index)">删除</button>
              </div>
            </div>
            <div class="form-actions">
              <button class="primary" type="submit" :disabled="templateSaving">保存模板</button>
              <button v-if="templateForm.id" class="secondary" type="button" :disabled="templateSaving" @click="deactivateGradientTemplate(templateForm.id)">停用模板</button>
            </div>
          </form>
        </div>
      </div>

      <div class="panel category-panel">
        <div class="panel-title">
          <span>商品分类 · {{ selectedSkuContextLabel }}</span>
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
              v-for="primary in categoryTreeForSkuContext"
              :key="primary.id"
              class="primary-category"
              :data-primary-id="primary.id"
              @dragover.prevent="handlePrimaryCategoryDragOver($event, primary)"
              @drop.prevent="dropCategoryOnCurrentTarget(primary)">
              <form v-if="editingCategoryId === primary.id" class="inline-form sub-form" @submit.prevent="saveCategoryName(primary)">
                <input v-model.trim="editingCategoryName" placeholder="一级分类名称" />
                <button class="secondary" type="submit">保存</button>
              </form>
              <div v-else class="category-head">
                <strong>{{ primary.number }}. {{ primary.name }}</strong>
                <div class="category-actions">
                  <button class="text-button" type="button" @click="startCategoryEdit(primary)">改名</button>
                  <button class="text-button" type="button" @click="startAddingSecondary(primary)">新增二级</button>
                  <button class="text-button danger-text" type="button" @click="deleteCategory(primary)">删除</button>
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
                  :class="{ dragging: isDraggingCategory(secondary), 'pointer-dragging': isPointerDraggingCategory(secondary) }"
                  :data-secondary-id="secondary.id"
                  :data-secondary-position="index + 1"
                  @pointerdown="startCategoryPointerDrag($event, primary, index + 1, secondary)"
                  @dragenter.prevent.stop="handleSecondaryCategoryDragOver($event, primary, index + 1)"
                  @dragover.prevent.stop="handleSecondaryCategoryDragOver($event, primary, index + 1)"
                  @drop.prevent.stop="dropCategoryOrProductOnSecondary(primary, index + 1, secondary)">
                  <form v-if="editingCategoryId === secondary.id" class="inline-form sub-form" @submit.prevent="saveCategoryName(secondary)">
                    <input v-model.trim="editingCategoryName" placeholder="二级分类名称" />
                    <button class="secondary" type="submit">保存</button>
                  </form>
                  <div v-else class="secondary-head">
                    <span>{{ secondary.number }}</span>
                    <b>{{ secondary.name }}</b>
                    <small>{{ secondary.products.length }} 款</small>
                    <select
                      class="template-select"
                      :value="secondary.gradient_template_id || 0"
                      @pointerdown.stop
                      @change.stop="bindCategoryGradientTemplate(secondary, $event.target.value)">
                      <option value="0">未绑定模板</option>
                      <option v-for="template in activeGradientTemplates" :key="template.id" :value="template.id">
                        {{ template.name }} · {{ gradientDisplayUnitLabel(template.display_unit) }}
                      </option>
                    </select>
                    <button class="text-button" type="button" @click="startCategoryEdit(secondary)">改名</button>
                    <button class="text-button danger-text" type="button" @click="deleteCategory(secondary)">删除</button>
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
        <div class="panel-title sku-panel-title">
          <span>客户SKU列表 · {{ selectedSkuContextLabel }}</span>
          <div class="panel-actions sku-panel-actions">
            <button class="secondary compact-action" type="button" @click="deactivateProducts(selectedProductIds)" :disabled="!selectedProductIds.length || loading">
              失效选中产品
            </button>
            <button class="toggle-section" type="button" @click="productsCollapsed = !productsCollapsed">
              {{ productsCollapsed ? '展开' : '收起' }}
            </button>
          </div>
        </div>
        <div v-show="!productsCollapsed" class="table-wrap">
          <div class="sku-filters">
            <label>
              <span>形态</span>
              <select v-model="skuFilters.productKind">
                <option value="all">全部形态</option>
                <option value="roasted">熟豆</option>
                <option value="green_bean">生豆</option>
              </select>
            </label>
            <label>
              <span>名称</span>
              <input v-model.trim="skuFilters.query" placeholder="搜索商品名称" />
            </label>
            <label>
              <span>一级分类</span>
              <select v-model="skuFilters.primaryCategory">
                <option value="">全部一级分类</option>
                <option v-for="name in skuPrimaryCategoryOptions" :key="name" :value="name">{{ name }}</option>
              </select>
            </label>
            <label>
              <span>二级分类</span>
              <select v-model="skuFilters.secondaryCategory">
                <option value="">全部二级分类</option>
                <option v-for="name in skuSecondaryCategoryOptions" :key="name" :value="name">{{ name }}</option>
              </select>
            </label>
          </div>
          <table>
            <thead>
              <tr>
                <th class="select-col">
                  <input type="checkbox" :checked="allProductRowsSelected" :disabled="!displaySkuRows.length" @change="toggleAllProductRows($event.target.checked)" />
                </th>
                <th>一级分类</th>
                <th>二级分类</th>
                <th>商品编号</th>
                <th>商品</th>
                <th>形态</th>
                <th>归属</th>
                <th>类型</th>
                <th>烘焙度</th>
                <th>BOM出品率</th>
                <th v-if="!selectedCustomerSkuCustomerID">利润率覆盖</th>
                <th>BOM状态</th>
                <th>BOM</th>
                <th>处理</th>
              </tr>
            </thead>
            <tbody>
              <template v-for="row in displaySkuRows" :key="row.id">
                <tr>
                  <td class="select-col">
                    <input type="checkbox" :checked="isProductSelected(row)" @change="toggleProductSelection(row, $event.target.checked)" />
                  </td>
                  <td>{{ categoryLabel(row, 1) }}</td>
                  <td>{{ categoryLabel(row, 2) }}</td>
                  <td>{{ row.number || '' }}</td>
                  <td>{{ row.name }}</td>
                  <td><span class="kind-badge" :class="productKindBadgeClass(row)">{{ productKindLabel(row) }}</span></td>
                  <td>{{ ownerLabel(row) }}</td>
                  <td>{{ customTypeLabel(row.custom_type) }}</td>
                  <td>
                    <select class="roast-select" v-model="row.roast_level" :disabled="row.product_kind === 'green_bean'" @change="saveProductBasics(row)">
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
                        :disabled="row.product_kind === 'green_bean'"
                        @change="saveProductBasics(row)" />
                      <span>%</span>
                    </div>
                  </td>
                  <td v-if="!selectedCustomerSkuCustomerID">
                    <input
                      class="margin-input"
                      v-model="row.margin_rate_override_input"
                      type="number"
                      min="0"
                      step="0.001"
                      placeholder="留空继承分类模板"
                      @change="saveProductMarginOverride(row)" />
                  </td>
                  <td>
                    <span :class="['status-pill', row.bom_status === 'inactive' ? 'inactive' : '']">{{ bomStatusLabel(row.bom_status) }}</span>
                  </td>
                  <td>
                    <button class="text-button" type="button" @click="openProductBom(row)">维护 BOM</button>
                  </td>
                  <td>
                    <button class="text-button danger-text" type="button" @click="deactivateProducts([row.id])">失效</button>
                  </td>
                </tr>
                <tr v-if="row.product_kind === 'green_bean'" class="green-bean-detail-row">
                  <td :colspan="selectedCustomerSkuCustomerID ? 13 : 14">
                    <div class="green-bean-detail-fields">
                      <label>
                        <span>生豆属性</span>
                        <select
                          class="green-type-select"
                          v-model="row.green_bean_type"
                          @change="saveProductBasics(row)">
                          <option v-for="option in greenBeanTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                        </select>
                      </label>
                      <label class="green-bom-field">
                        <span>绑定熟豆</span>
                        <SearchableSelect
                          class="bom-product-select"
                          v-model="row.green_bean_bom_product_id"
                          :options="roastedBomProducts"
                          :option-label="baseProductOptionLabel"
                          :option-meta="baseProductOptionMeta"
                          :option-value="optionNumericValue"
                          placeholder="选择熟豆"
                          empty-text="暂无熟豆产品"
                          @select="saveProductBasics(row)" />
                      </label>
                    </div>
                  </td>
                </tr>
              </template>
              <tr v-if="!displaySkuRows.length">
                <td :colspan="selectedCustomerSkuCustomerID ? 13 : 14" class="muted">{{ selectedCustomerSkuCustomerID ? '暂无客户SKU' : '暂无公共SKU' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>

  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import SearchableSelect from '../components/SearchableSelect.vue'
import {
  buildGradientTemplatePayload,
  gradientDisplayQuantityStep,
  gradientDisplayQuantityUnitLabel,
  gradientDisplayUnitOptions,
  gradientDisplayUnitLabel,
  normalizeGradientTemplate,
  validateGradientTemplate,
} from '../lib/gradient-templates'
import {
  buildProductBasicsPayload,
  buildProductCreatePayload,
  filterSkuRows,
  greenBeanTypeOptions,
  normalizedProductKind,
  primaryCategoryOptions,
  roastedBomProductOptions,
  secondaryCategoryOptions,
} from '../lib/product-settings'

const categories = ref([])
const products = ref([])
const gradientTemplates = ref([])
const customers = ref([])
const loading = ref(false)
const productSaving = ref(false)
const customSaving = ref(false)
const templateSaving = ref(false)
const error = ref('')
const ok = ref('')
const newPrimaryName = ref('')
const newSecondaryName = ref('')
const addingSecondaryFor = ref(0)
const dragging = ref(null)
const pointerDrag = ref(null)
const categoryDropTarget = ref(null)
const editingCategoryId = ref(0)
const editingCategoryName = ref('')
const categoryCollapsed = ref(false)
const productsCollapsed = ref(false)
const selectedCustomerSkuCustomerID = ref(0)
const selectedProductIds = ref([])
const skuFilters = ref(defaultSkuFilters())
const roastLevels = ['浅烘', '中烘', '中深烘', '深烘']
const productForm = ref(defaultProductForm())
const customForm = ref(defaultCustomForm())
const templateForm = ref(defaultGradientTemplateForm())

const skuContextCustomerID = computed(() => Number(selectedCustomerSkuCustomerID.value || 0))
const selectedSkuContextLabel = computed(() => {
  const customerID = skuContextCustomerID.value
  if (!customerID) return '公共SKU'
  return `${customerName(customerID) || `客户 #${customerID}`} SKU`
})
const categoryTreeForSkuContext = computed(() => categories.value
  .filter(categoryBelongsToSkuContext)
  .map((primary, primaryIndex) => {
    const primaryName = primary.name || ''
    const primaryProducts = (primary.products || [])
      .filter(skuContextProductFilter)
      .map((product, index) => ({
        ...product,
        number: index + 1,
        primary_name: primaryName,
        secondary_name: '',
      }))
    const children = (primary.children || [])
      .filter(categoryBelongsToSkuContext)
      .map((secondary, secondaryIndex) => ({
        ...secondary,
        number: secondaryIndex + 1,
        products: (secondary.products || [])
          .filter(skuContextProductFilter)
          .map((product, productIndex) => ({
            ...product,
            number: productIndex + 1,
            primary_name: primaryName,
            secondary_name: secondary.name || '',
          })),
      }))
    return {
      ...primary,
      number: primaryIndex + 1,
      products: primaryProducts,
      children,
    }
  }))
const productRows = computed(() => {
  const rows = []
  for (const primary of categoryTreeForSkuContext.value) {
    for (const product of primary.products || []) {
      rows.push(product)
    }
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

const contextCategorizedProductIDs = computed(() => {
  const ids = new Set()
  for (const primary of categoryTreeForSkuContext.value) {
    for (const product of primary.products || []) ids.add(Number(product.id))
    for (const secondary of primary.children || []) {
      for (const product of secondary.products || []) ids.add(Number(product.id))
    }
  }
  return ids
})

const uncategorizedProducts = computed(() => products.value
  .filter(skuContextProductFilter)
  .filter((product) => !contextCategorizedProductIDs.value.has(Number(product.id)))
  .slice()
  .sort((a, b) => ownerLabel(a).localeCompare(ownerLabel(b)) || a.name.localeCompare(b.name)))
const baseProducts = computed(() => products.value.filter((product) => Number(product.customer_id || 0) === 0 && productVisibility(product) === 'public'))
const publicSkuRows = computed(() => productRows.value.filter((product) => Number(product.customer_id || 0) === 0))
const customProductCustomerIDs = computed(() => {
  const ids = new Set()
  for (const product of products.value) {
    const customerID = Number(product.customer_id || 0)
    if (customerID > 0) ids.add(customerID)
  }
  return ids
})
const customerSkuCustomers = computed(() => customers.value
  .filter((customer) => customProductCustomerIDs.value.has(Number(customer.id || 0)))
  .sort((a, b) => customerOptionLabel(a).localeCompare(customerOptionLabel(b))))
const customerSkuRows = computed(() => productRows.value
  .filter((product) => Number(product.customer_id || 0) > 0)
  .filter((product) => !selectedCustomerSkuCustomerID.value || Number(product.customer_id || 0) === Number(selectedCustomerSkuCustomerID.value))
  .sort((a, b) => ownerLabel(a).localeCompare(ownerLabel(b)) || a.name.localeCompare(b.name)))
const unfilteredDisplaySkuRows = computed(() => selectedCustomerSkuCustomerID.value ? customerSkuRows.value : publicSkuRows.value)
const displaySkuRows = computed(() => filterSkuRows(unfilteredDisplaySkuRows.value, skuFilters.value))
const allProductRowsSelected = computed(() => displaySkuRows.value.length > 0 && displaySkuRows.value.every((row) => selectedProductIds.value.includes(Number(row.id))))
const activeGradientTemplates = computed(() => gradientTemplates.value.filter((template) => template.active !== false))
const skuPrimaryCategoryOptions = computed(() => primaryCategoryOptions(unfilteredDisplaySkuRows.value))
const skuSecondaryCategoryOptions = computed(() => secondaryCategoryOptions(unfilteredDisplaySkuRows.value, skuFilters.value.primaryCategory))
const roastedBomProducts = computed(() => roastedBomProductOptions(products.value))

function defaultSkuFilters() {
  return {
    productKind: 'all',
    query: '',
    primaryCategory: '',
    secondaryCategory: '',
  }
}

function defaultProductForm() {
  return {
    name: '',
    product_kind: 'roasted',
    green_bean_type: 'single_origin',
    green_bean_bom_product_id: 0,
    roast_level: '中烘',
    yield_percent: 80,
  }
}

function productKindLabel(row = {}) {
  return normalizedProductKind(row) === 'green_bean' ? '生豆' : '熟豆'
}

function productKindBadgeClass(row = {}) {
  return normalizedProductKind(row) === 'green_bean' ? 'kind-green' : 'kind-roasted'
}

function categoryLabel(row, level) {
  return level === 1 ? row.primary_name || '' : row.secondary_name || ''
}

function defaultCustomForm() {
  return {
    customer_id: 0,
    base_product_id: 0,
    name: '',
    roast_level: '中烘',
    custom_type: 'public_sku_alias',
    copy_bom: true,
    copy_price_tiers: true,
  }
}

function defaultGradientTemplateForm() {
  return normalizeGradientTemplate({
    name: '',
    display_unit: 'lb',
    tiers: [
      { label: '2磅-13磅', min_display_qty: 2, max_display_qty: 13, margin_rate: 0.5421052631578949, position: 1 },
    ],
  })
}

function decorateProduct(product) {
  const yieldRate = Number(product.yield_rate || 0.8)
  const productKind = normalizedProductKind(product)
  const marginRateOverride = normalizeBackendMarginRateOverride(product.margin_rate_override)
  return {
    ...product,
    product_kind: productKind,
    green_bean_type: product.green_bean_type || 'single_origin',
    green_bean_bom_product_id: Number(product.green_bean_bom_product_id || 0),
    roast_level: productKind === 'green_bean' ? '' : roastLevels.includes(product.roast_level) ? product.roast_level : '中烘',
    yield_rate: productKind === 'green_bean' ? 0 : yieldRate,
    yield_percent: productKind === 'green_bean' ? 0 : Number((yieldRate * 100).toFixed(2)),
    default_price: Number(product.default_price || 0),
    margin_rate_override: marginRateOverride,
    margin_rate_override_input: marginRateOverride === null ? '' : marginRateOverride,
    customer_id: Number(product.customer_id || 0),
    base_product_id: Number(product.base_product_id || 0),
    visibility: productVisibility(product),
    custom_type: product.custom_type || '',
    bom_item_count: Number(product.bom_item_count || 0),
    bom_status: product.bom_status || (Number(product.bom_item_count || 0) > 0 ? 'active' : 'missing'),
  }
}

function normalizeBackendMarginRateOverride(value) {
  if (value === null || typeof value === 'undefined' || value === '') return null
  const numberValue = Number(value)
  return Number.isFinite(numberValue) && numberValue >= 0 ? numberValue : null
}

function decorateCategory(category) {
  return {
    ...category,
    customer_id: Number(category.customer_id || 0),
    gradient_template_id: Number(category.gradient_template_id || 0),
    children: (category.children || []).map(decorateCategory),
    products: (category.products || []).map(decorateProduct),
  }
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [data, customerData] = await Promise.all([
      apiGet('/api/product-settings'),
      apiGet('/api/customers?limit=200'),
    ])
    categories.value = (data.categories || []).map(decorateCategory)
    products.value = (data.products || []).map(decorateProduct)
    gradientTemplates.value = (data.gradient_templates || []).map(normalizeGradientTemplate)
    customers.value = (customerData.rows || []).filter((row) => row.active !== false)
    syncSelectedCustomerSkuCustomer()
    pruneSelectedProducts(displaySkuRows.value)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function resetGradientTemplateForm() {
  templateForm.value = defaultGradientTemplateForm()
}

function startGradientTemplateEdit(template) {
  templateForm.value = normalizeGradientTemplate(JSON.parse(JSON.stringify(template)))
}

function addGradientTemplateTier() {
  templateForm.value.tiers.push({
    id: 0,
    label: '',
    min_display_qty: 0,
    max_display_qty: null,
    min_weight_g: 0,
    max_weight_g: null,
    margin_rate: 0,
    position: templateForm.value.tiers.length + 1,
  })
}

function removeGradientTemplateTier(index) {
  templateForm.value.tiers.splice(index, 1)
  templateForm.value.tiers.forEach((tier, tierIndex) => {
    tier.position = tierIndex + 1
  })
}

async function saveGradientTemplate() {
  const payload = buildGradientTemplatePayload(templateForm.value)
  const errors = validateGradientTemplate(payload)
  if (errors.length) {
    error.value = errors[0]
    return
  }
  templateSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = payload.id ? `/api/pricing-gradient-templates/${payload.id}` : '/api/pricing-gradient-templates'
    const method = payload.id ? 'PUT' : 'POST'
    await apiSend(url, { method, body: payload })
    ok.value = '梯度模板已保存'
    resetGradientTemplateForm()
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存梯度模板失败'
  } finally {
    templateSaving.value = false
  }
}

async function deactivateGradientTemplate(id) {
  if (!id) return
  templateSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/pricing-gradient-templates/${id}/deactivate`)
    ok.value = '梯度模板已停用'
    resetGradientTemplateForm()
    await loadAll()
  } catch (err) {
    error.value = err.message || '停用梯度模板失败'
  } finally {
    templateSaving.value = false
  }
}

async function bindCategoryGradientTemplate(category, templateID) {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/product-settings/categories/${category.id}/gradient-template`, {
      body: { gradient_template_id: Number(templateID || 0) },
    })
    ok.value = '分类梯度模板已更新，未发布预览会自动按新模板更新'
    await loadAll()
  } catch (err) {
    error.value = err.message || '绑定梯度模板失败'
  } finally {
    loading.value = false
  }
}

function productVisibility(product) {
  const customerID = Number(product?.customer_id || 0)
  return product?.visibility || (customerID > 0 ? 'customer_only' : 'public')
}

function categoryBelongsToSkuContext(category) {
  return Number(category?.customer_id || 0) === skuContextCustomerID.value
}

function skuContextProductFilter(product) {
  const productCustomerID = Number(product?.customer_id || 0)
  return skuContextCustomerID.value > 0
    ? productCustomerID === skuContextCustomerID.value
    : productCustomerID === 0
}

function customerName(id) {
  return customers.value.find((customer) => Number(customer.id) === Number(id))?.name || ''
}

function customerOptionLabel(customer) {
  return customer?.name || ''
}

function customerOptionMeta(customer) {
  const parts = []
  if (customer?.company_name && customer.company_name !== customer?.name) parts.push(customer.company_name)
  if (customer?.contact) parts.push(customer.contact)
  if (customer?.phone || customer?.company_phone) parts.push(customer.phone || customer.company_phone)
  return parts.join(' / ')
}

function optionNumericValue(option) {
  return Number(option?.id || 0)
}

function baseProductOptionLabel(product) {
  return product?.name || ''
}

function baseProductOptionMeta(product) {
  const parts = []
  if (product?.number) parts.push(`编号 ${product.number}`)
  if (product?.roast_level) parts.push(product.roast_level)
  return parts.join(' / ')
}

function ownerLabel(row) {
  if (Number(row.customer_id || 0) <= 0) return '公共'
  return customerName(row.customer_id) || `客户 #${row.customer_id}`
}

function customTypeLabel(value) {
  if (value === 'custom_blend') return '定制拼配'
  if (value === 'custom_roast') return '定制烘焙'
  if (value === 'public_sku_alias') return '公共 SKU 改名'
  return '标准'
}

function bomStatusLabel(value) {
  if (value === 'inactive') return '已失效'
  if (value === 'missing') return '未维护'
  return '有效'
}

function pruneSelectedProducts(sourceProducts) {
  const validIDs = new Set((sourceProducts || []).map((product) => Number(product.id || 0)).filter(Boolean))
  selectedProductIds.value = selectedProductIds.value.filter((id) => validIDs.has(Number(id)))
}

function syncSelectedCustomerSkuCustomer() {
  if (!selectedCustomerSkuCustomerID.value) return
  if (!customProductCustomerIDs.value.has(Number(selectedCustomerSkuCustomerID.value))) {
    selectedCustomerSkuCustomerID.value = 0
  }
}

function isProductSelected(row) {
  return selectedProductIds.value.includes(Number(row.id))
}

function toggleProductSelection(row, checked) {
  const id = Number(row.id || 0)
  if (!id) return
  const current = selectedProductIds.value
  selectedProductIds.value = checked
    ? Array.from(new Set([...current, id]))
    : current.filter((item) => item !== id)
}

function toggleAllProductRows(checked) {
  selectedProductIds.value = checked ? displaySkuRows.value.map((row) => Number(row.id)).filter(Boolean) : []
}

function selectedBaseProduct() {
  return products.value.find((product) => Number(product.id) === Number(customForm.value.base_product_id)) || null
}

function baseProductName(id) {
  return products.value.find((product) => Number(product.id) === Number(id))?.name || '-'
}

function fillCustomProductName() {
  if (customForm.value.name) return
  const customer = customerName(customForm.value.customer_id)
  const base = selectedBaseProduct()
  if (!customer || !base) return
  customForm.value.name = `${customer}-${base.name}-${customForm.value.roast_level}`
}

function openProductBom(row) {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'bom')
  url.searchParams.set('product_id', String(row.id))
  window.location.href = url.toString()
}

async function createProduct() {
  if (!productForm.value.name) {
    error.value = '请填写商品名称'
    return
  }
  const yieldPercent = Number(productForm.value.yield_percent || 0)
  if (productForm.value.product_kind !== 'green_bean' && (yieldPercent <= 0 || yieldPercent > 100)) {
    error.value = 'BOM出品率必须在 1% 到 100% 之间'
    return
  }
  if (productForm.value.product_kind === 'green_bean' && Number(productForm.value.green_bean_bom_product_id || 0) <= 0) {
    error.value = '生豆 SKU 必须绑定对应熟豆 BOM'
    return
  }
  productSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend('/api/product-settings/products', {
      body: buildProductCreatePayload(productForm.value),
    })
    ok.value = '公共产品已创建'
    productForm.value = defaultProductForm()
    await loadAll()
  } catch (err) {
    error.value = err.message || '创建公共产品失败'
  } finally {
    productSaving.value = false
  }
}

async function createCustomProduct() {
  if (!customForm.value.customer_id) {
    error.value = '请选择客户'
    return
  }
  if (!customForm.value.base_product_id) {
    error.value = '请选择基础产品'
    return
  }
  if (!customForm.value.name) {
    fillCustomProductName()
  }
  if (!customForm.value.name) {
    error.value = '请填写专属 SKU 名称'
    return
  }
  customSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend('/api/product-settings/custom-products', { body: customForm.value })
    ok.value = '客户专属 SKU 已创建'
    customForm.value = defaultCustomForm()
    await loadAll()
  } catch (err) {
    error.value = err.message || '创建专属 SKU 失败'
  } finally {
    customSaving.value = false
  }
}

async function savePrimaryCategory() {
  if (!newPrimaryName.value) return
  await saveCategory({
    name: newPrimaryName.value,
    parent_id: 0,
    customer_id: selectedCustomerSkuCustomerID.value,
    position: categoryTreeForSkuContext.value.length + 1,
  })
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
    customer_id: selectedCustomerSkuCustomerID.value,
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
        customer_id: Number(category.customer_id || selectedCustomerSkuCustomerID.value || 0),
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

async function deleteCategory(category) {
  const name = category.name || '该分类'
  const okToDelete = window.confirm(`确认删除「${name}」？删除分类后，分类内商品会回到未分类。`)
  if (!okToDelete) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/product-settings/categories/${category.id}`, { method: 'DELETE' })
    ok.value = '分类已删除，相关商品已回到未分类'
    await loadAll()
  } catch (err) {
    error.value = err.message || '删除分类失败'
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

function startCategoryPointerDrag(event, primary, visualPosition, category) {
  if (event.button !== 0 || isInteractiveDragSource(event.target)) return
  pointerDrag.value = {
    pointerID: event.pointerId,
    startX: event.clientX,
    startY: event.clientY,
    primaryID: Number(primary.id),
    visualPosition: Number(visualPosition),
    category,
    active: false,
    element: event.currentTarget,
  }
  event.currentTarget.setPointerCapture?.(event.pointerId)
  event.currentTarget.addEventListener('pointermove', handleCategoryPointerMove)
  event.currentTarget.addEventListener('pointerup', handleCategoryPointerUp)
  event.currentTarget.addEventListener('pointercancel', cancelCategoryPointerDrag)
}

function isInteractiveDragSource(target) {
  return Boolean(target?.closest?.('button,input,select,textarea,a,.product-chip'))
}

function handleCategoryPointerMove(event) {
  const state = pointerDrag.value
  if (!state || state.pointerID !== event.pointerId) return
  const moved = Math.abs(event.clientX - state.startX) + Math.abs(event.clientY - state.startY)
  if (!state.active) {
    if (moved < 5) return
    state.active = true
    startCategoryDrag(state.category)
  }
  event.preventDefault()
  const target = resolveCategoryPointerTarget(event.clientX, event.clientY, state.primaryID)
  if (!target) return
  categoryDropTarget.value = { parentID: Number(target.primary.id), position: Number(target.position) }
}

async function handleCategoryPointerUp(event) {
  const state = pointerDrag.value
  if (!state || state.pointerID !== event.pointerId) return
  const target = categoryDropTarget.value
  const active = state.active
  cleanupCategoryPointerDrag()
  if (!active) {
    clearDrag()
    return
  }
  event.preventDefault()
  const primary = categoryTreeForSkuContext.value.find((item) => Number(item.id) === Number(target?.parentID))
  if (!primary || !target) {
    clearDrag()
    return
  }
  await dropCategoryAtPosition(primary, target.position)
}

function cancelCategoryPointerDrag() {
  cleanupCategoryPointerDrag()
  clearDrag()
}

function cleanupCategoryPointerDrag() {
  const state = pointerDrag.value
  if (state?.element) {
    state.element.removeEventListener('pointermove', handleCategoryPointerMove)
    state.element.removeEventListener('pointerup', handleCategoryPointerUp)
    state.element.removeEventListener('pointercancel', cancelCategoryPointerDrag)
  }
  pointerDrag.value = null
}

function resolveCategoryPointerTarget(clientX, clientY, fallbackPrimaryID) {
  const elements = document.elementsFromPoint(clientX, clientY)
  const primaryElement = elements.find((item) => item.classList?.contains('primary-category'))
    || document.querySelector(`[data-primary-id="${fallbackPrimaryID}"]`)
  if (!primaryElement) return null
  const primaryID = Number(primaryElement.dataset.primaryId || fallbackPrimaryID)
  const primary = categoryTreeForSkuContext.value.find((item) => Number(item.id) === primaryID)
  if (!primary) return null
  const secondaryElements = Array.from(primaryElement.querySelectorAll('.secondary-category'))
  if (!secondaryElements.length) return { primary, position: 1 }
  for (let index = 0; index < secondaryElements.length; index += 1) {
    const rect = secondaryElements[index].getBoundingClientRect()
    if (clientY < rect.top + rect.height / 2) {
      return { primary, position: index + 1 }
    }
  }
  return { primary, position: secondaryElements.length + 1 }
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

function isPointerDraggingCategory(category) {
  return pointerDrag.value?.active && Number(pointerDrag.value.category?.id) === Number(category.id)
}

function setCategoryDropTarget(primary, position) {
  if (dragging.value?.type !== 'category') return
  categoryDropTarget.value = { parentID: Number(primary.id), position: Number(position) }
}

function handlePrimaryCategoryDragOver(event, primary) {
  if (dragging.value?.type !== 'category') return
  const target = resolveCategoryPointerTarget(event.clientX, event.clientY, Number(primary.id))
  if (target) {
    categoryDropTarget.value = { parentID: Number(target.primary.id), position: Number(target.position) }
  }
}

function handleSecondaryCategoryDragOver(event, primary, visualPosition) {
  if (dragging.value?.type !== 'category') return
  const rect = event.currentTarget.getBoundingClientRect()
  const position = event.clientY > rect.top + rect.height / 2 ? Number(visualPosition) + 1 : Number(visualPosition)
  categoryDropTarget.value = { parentID: Number(primary.id), position }
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

async function dropCategoryOnCurrentTarget(primary) {
  if (dragging.value?.type !== 'category') return
  const parentID = Number(primary.id)
  const position = categoryDropTarget.value?.parentID === parentID
    ? categoryDropTarget.value.position
    : Number(primary.children?.length || 0) + 1
  await dropCategoryAtPosition(primary, position)
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
  const product = products.value.find((item) => Number(item.id) === Number(drag.id))
  if (!product || !skuContextProductFilter(product)) {
    error.value = '只能调整当前 SKU 归属下的商品分类'
    clearDrag()
    return
  }
  const categoryID = Number(secondary.id || 0)
  if (categoryID > 0 && Number(secondary.customer_id || 0) !== skuContextCustomerID.value) {
    error.value = '只能移动到当前客户自己的商品分类'
    clearDrag()
    return
  }
  try {
    await apiSend(`/api/product-settings/products/${drag.id}/category`, {
      body: { category_id: categoryID, position: Number(secondary.products?.length || 0) + 1 },
    })
    ok.value = '商品分类已保存'
    await loadAll()
  } catch (err) {
    error.value = err.message || '移动商品失败'
  } finally {
    clearDrag()
  }
}

function normalizeMarginRateOverride(row) {
  const raw = row.margin_rate_override_input
  if (raw === '' || raw === null || typeof raw === 'undefined') return { ok: true, value: null }
  const value = Number(raw)
  if (!Number.isFinite(value) || value < 0) return { ok: false, value: null }
  return { ok: true, value: Number(value.toFixed(6)) }
}

async function saveProductMarginOverride(row) {
  await saveProductBasics(row, '产品级利润率覆盖已保存')
}

async function saveProductBasics(row, successMessage = '商品基础信息已保存') {
  const yieldPercent = Number(row.yield_percent || 0)
  if (row.product_kind !== 'green_bean' && (yieldPercent <= 0 || yieldPercent > 100)) {
    error.value = 'BOM出品率必须在 1% 到 100% 之间'
    return
  }
  if (row.product_kind === 'green_bean' && Number(row.green_bean_bom_product_id || 0) <= 0) {
    error.value = '生豆 SKU 必须绑定对应熟豆 BOM'
    return
  }
  const marginOverride = normalizeMarginRateOverride(row)
  if (!marginOverride.ok) {
    error.value = '利润率覆盖必须为 0 或正数'
    return
  }
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/products/${row.id}`, {
      method: 'PUT',
      body: buildProductBasicsPayload(row, marginOverride.value),
    })
    row.margin_rate_override = marginOverride.value
    row.margin_rate_override_input = marginOverride.value === null ? '' : marginOverride.value
    ok.value = successMessage
    await loadAll()
  } catch (err) {
    error.value = err.message || '保存商品失败'
  } finally {
    loading.value = false
  }
}

async function deactivateProducts(productIds) {
  const ids = Array.from(new Set((productIds || []).map((id) => Number(id || 0)).filter((id) => id > 0)))
  if (!ids.length) return
  const message = ids.length > 1
    ? `确认失效选中的 ${ids.length} 个产品？对应 BOM 会同步失效，历史配方明细会保留。`
    : '确认失效该产品？对应 BOM 会同步失效，历史配方明细会保留。'
  if (!window.confirm(message)) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend('/api/product-settings/products/deactivate', { body: { product_ids: ids } })
    selectedProductIds.value = []
    ok.value = ids.length > 1 ? '选中产品已失效，对应 BOM 已同步失效' : '产品已失效，对应 BOM 已同步失效'
    await loadAll()
  } catch (err) {
    error.value = err.message || '产品失效失败'
  } finally {
    loading.value = false
  }
}

watch(selectedCustomerSkuCustomerID, () => {
  skuFilters.value = defaultSkuFilters()
  pruneSelectedProducts(displaySkuRows.value)
})

watch(() => skuFilters.value.primaryCategory, () => {
  if (!skuSecondaryCategoryOptions.value.includes(skuFilters.value.secondaryCategory)) {
    skuFilters.value.secondaryCategory = ''
  }
})

watch(displaySkuRows, (rows) => {
  pruneSelectedProducts(rows)
})

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
.panel-actions, .row-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
.sub-title { width: 100%; font-weight: 700; font-size: 13px; }
button, input, select { font: inherit; min-height: 36px; border-radius: 6px; }
input, select { border: 1px solid #cfc8bf; padding: 7px 9px; background: #fff; width: 100%; }
button { border: 1px solid #1f1f1f; background: #fff; padding: 0 12px; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #111; color: #fff; }
.secondary, .toggle-section { background: #fff; color: #111; }
.toggle-section { min-height: 30px; padding: 0 10px; }
.compact-action { min-height: 30px; padding: 0 10px; font-size: 12px; }
.text-button { border: 0; background: transparent; color: #1f4f82; padding: 0; min-height: 28px; }
.danger-text { color: #a33; }
.settings-grid { display: grid; grid-template-columns: minmax(300px, 420px) minmax(0, 1fr); gap: 14px; align-items: start; }
.sku-context-panel { grid-column: 1 / -1; display: grid; gap: 10px; }
.sku-context-main { display: flex; align-items: flex-start; justify-content: space-between; gap: 14px; }
.sku-context-main h3 { margin: 2px 0 4px; font-size: 18px; }
.context-eyebrow { color: #7a4d1a; font-size: 12px; font-weight: 700; }
.sku-context-controls { display: flex; align-items: center; justify-content: flex-end; gap: 8px; flex-wrap: wrap; min-width: min(460px, 100%); }
.context-stats { display: flex; flex-wrap: wrap; gap: 8px; }
.context-stats span { border: 1px solid #e6e0d8; border-radius: 999px; padding: 4px 9px; background: #fbfaf8; color: #333; font-size: 12px; }
.product-create-form { display: grid; grid-template-columns: repeat(2, minmax(140px, 1fr)); gap: 10px; align-items: end; }
.custom-product-form { display: grid; grid-template-columns: repeat(3, minmax(150px, 1fr)); gap: 10px; align-items: end; }
.product-create-form label, .custom-product-form label { display: grid; gap: 5px; font-size: 13px; }
.product-create-form .wide-field, .custom-product-form .wide-field { grid-column: span 2; }
.gradient-template-panel { grid-column: 1 / -1; }
.gradient-template-layout { display: grid; grid-template-columns: minmax(220px, 280px) minmax(0, 1fr); gap: 12px; align-items: start; }
.template-list { display: grid; gap: 8px; }
.template-row { min-height: 50px; display: grid; gap: 3px; align-content: center; text-align: left; border: 1px solid #e2ddd6; background: #fbfaf8; padding: 8px 10px; }
.template-row.active { border-color: #1f4f82; background: #eef6ff; }
.template-row.inactive { opacity: .58; }
.template-row small { color: #666; font-size: 12px; }
.template-editor { border: 1px solid #eee8df; border-radius: 8px; background: #fbfaf8; padding: 12px; }
.template-editor-grid { display: grid; grid-template-columns: minmax(0, 1fr) 160px; gap: 10px; }
.template-editor label { display: grid; gap: 5px; font-size: 13px; }
.template-tier-head { display: flex; justify-content: space-between; align-items: center; gap: 10px; margin: 12px 0 8px; }
.template-tier-list { display: grid; gap: 8px; }
.template-tier-row { display: grid; grid-template-columns: minmax(130px, 1.1fr) minmax(130px, .9fr) minmax(130px, .9fr) minmax(100px, .7fr) auto; gap: 8px; align-items: end; border: 1px solid #e2ddd6; border-radius: 8px; background: #fff; padding: 10px; }
.template-select { width: min(280px, 100%); min-height: 30px; padding: 4px 8px; font-size: 12px; }
.sku-panel-title { align-items: flex-start; }
.sku-panel-actions { flex: 1; }
.sku-customer-select { min-width: 220px; max-width: 320px; flex: 1 1 220px; font-weight: 400; }
.sku-filters { display: grid; grid-template-columns: 150px minmax(180px, 1fr) 180px 180px; gap: 8px; margin-bottom: 10px; align-items: end; }
.sku-filters label { display: grid; gap: 5px; font-size: 12px; color: #333; }
.checkline { display: flex !important; align-items: center; gap: 8px; min-height: 36px; }
.checkline input { width: auto; min-height: 0; }
.form-actions { display: flex; justify-content: flex-end; }
.inline-form { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; margin-bottom: 10px; }
.sub-form { margin: 8px 0; }
.category-tree { display: grid; gap: 10px; }
.primary-category { border: 1px solid #eee8df; border-radius: 8px; padding: 10px; background: #fbfaf8; }
.category-head, .secondary-head, .category-actions { display: flex; align-items: center; gap: 8px; justify-content: space-between; }
.secondary-head { flex-wrap: wrap; justify-content: flex-start; }
.secondary-head b { min-width: 120px; }
.secondary-category { border: 1px solid #ddd; border-radius: 8px; padding: 9px; background: #fff; cursor: grab; user-select: none; touch-action: none; }
.secondary-category.dragging { opacity: .45; }
.secondary-category.pointer-dragging { cursor: grabbing; }
.secondary-head span { display: inline-flex; width: 24px; height: 24px; align-items: center; justify-content: center; border: 1px solid #ddd; border-radius: 6px; }
.secondary-head small { color: #666; }
.category-drop-line { height: 16px; border-top: 2px solid transparent; margin: 2px 0; transition: border-color .12s ease, background .12s ease; }
.category-drop-line.active { border-top-color: #1f4f82; background: #edf5ff; }
.product-chip-list, .uncategorized { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 8px; }
.product-chip { border: 1px solid #ddd; border-radius: 8px; padding: 5px 8px; background: #fff; font-size: 12px; cursor: grab; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 1280px; border-collapse: collapse; }
.compact-table table { min-width: 760px; }
th, td { border-bottom: 1px solid #eee8df; padding: 8px; text-align: left; font-size: 13px; vertical-align: middle; }
th { background: #fbfaf8; position: sticky; top: 0; }
.select-col { width: 42px; text-align: center; }
.select-col input { width: 16px; min-height: 16px; }
.roast-select { min-width: 92px; }
.green-type-select { min-width: 92px; }
.bom-product-select { min-width: 220px; }
.green-bean-detail-row td { background: #f7fbf8; padding-top: 6px; padding-bottom: 12px; }
.green-bean-detail-fields { display: flex; align-items: flex-end; gap: 14px; padding-left: 296px; }
.green-bean-detail-fields label { display: grid; gap: 5px; }
.green-bean-detail-fields label span { color: #667085; font-size: 12px; font-weight: 600; }
.green-bom-field { min-width: 360px; }
.yield-editor { display: flex; align-items: center; gap: 6px; max-width: 130px; }
.yield-input { width: 90px; }
.kind-badge { display: inline-flex; align-items: center; min-height: 20px; padding: 1px 7px; border-radius: 4px; font-size: 12px; font-weight: 600; }
.kind-roasted { color: #8a4b12; background: #fff3df; border: 1px solid #f3c67c; }
.kind-green { color: #12613a; background: #e8f7ee; border: 1px solid #8bd4a6; }
.margin-input { width: 150px; }
.status-pill { display: inline-flex; align-items: center; min-height: 24px; border: 1px solid #cfd8cf; border-radius: 999px; padding: 2px 8px; color: #27602e; background: #f2fbf2; white-space: nowrap; }
.status-pill.inactive { border-color: #e1b6b6; color: #8a1f1f; background: #fff0f0; }
.error, .ok { border-radius: 6px; padding: 9px; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
.muted { color: #666; font-size: 12px; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .settings-grid, .inline-form, .product-create-form, .custom-product-form, .gradient-template-layout, .template-editor-grid, .template-tier-row, .sku-filters { grid-template-columns: 1fr; }
  .sku-context-main { display: grid; }
  .sku-context-controls { justify-content: flex-start; min-width: 0; }
  .panel-actions { justify-content: flex-start; }
  .sku-panel-actions { width: 100%; }
  .sku-customer-select { max-width: none; }
  .product-create-form .wide-field, .custom-product-form .wide-field { grid-column: auto; }
  .template-select { width: 100%; }
  table { min-width: 1280px; }
}
</style>
