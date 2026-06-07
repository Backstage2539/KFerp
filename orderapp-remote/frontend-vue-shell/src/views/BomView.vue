<template>
  <div class="page">
    <div v-if="bomReturnNavigation" class="bom-return-banner">
      <button class="secondary bom-return-button" type="button" @click="returnToProductConfig">{{ bomReturnLabel }}</button>
      <span>完成 BOM 明细维护后可回到来源操作界面。</span>
    </div>
    <section class="panel">
      <div class="panel-head">
        <h2>生产 BOM（制造主档）</h2>
        <div class="panel-actions">
          <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
      <p class="muted left">生产 BOM 声明产出商品、产出数量和组件清单；商品档案只做库存、销售和反查。</p>
    </section>

    <div class="grid">
      <section class="panel list-panel">
        <div class="panel-head bom-list-head" data-pr442-bom-business-groups>
          <div>
            <div class="panel-title compact-title">生产 BOM列表</div>
            <p class="muted left">生产 BOM 是生产端主档案；分组来自泛化分组管理，保存到 /api/business-group-assignments，usage_key=production_bom。</p>
          </div>
        </div>
        <div class="bom-list-tabs-row">
          <div class="bom-list-tabs">
            <button
              :class="['secondary', 'compact-action', { active: selectedProductionBomGroupItemID === 0 }]"
              type="button"
              @click="selectProductionBomGroupItem(0)">
              全部分组
            </button>
            <button
              :class="['secondary', 'compact-action', { active: selectedProductionBomGroupItemID === -1 }]"
              type="button"
              @click="selectProductionBomGroupItem(-1)">
              未分类
            </button>
            <button
              v-for="option in productionBomGroupOptions"
              :key="option.id"
              :class="['secondary', 'compact-action', { active: selectedProductionBomGroupItemID === Number(option.group_item_id || 0) }]"
              type="button"
              @click="selectProductionBomGroupItem(Number(option.group_item_id || 0))">
              {{ option.label }}
            </button>
          </div>
          <button class="primary compact-action" type="button" @click="openNewProductionBomRecord">新建生产 BOM</button>
        </div>
        <div class="bom-list-toolbar">
          <button class="secondary compact-action" type="button" @click="openBusinessGroupManagement">前往分组管理</button>
          <button class="secondary compact-action" type="button" :disabled="!canMoveSelectedBoms || loading" @click="moveSelectedProductBomsToGroup">
            移动到分组
          </button>
          <label>
            <span>目标分组</span>
            <select v-model.number="selectedBomMoveGroupItemID">
              <option :value="0">未分组</option>
              <option v-for="option in productionBomGroupOptions" :key="option.id" :value="Number(option.group_item_id || 0)">{{ option.label }}</option>
            </select>
          </label>
          <span class="muted left">已选 {{ selectedBomRecordsForMove.length }} 个可移动 BOM</span>
        </div>
        <div class="bom-list-filters">
          <label>
            <span>状态</span>
            <select v-model="productionBomStatusFilter">
              <option value="active">启用</option>
              <option value="inactive">已失效</option>
              <option value="all">全部</option>
            </select>
          </label>
          <label class="bom-search-field">
            <span>搜索 BOM</span>
            <input v-model.trim="productionBomSearchQuery" placeholder="按 BOM 名称或编号搜索" />
          </label>
          <button class="secondary compact-action danger-outline bom-batch-deactivate-action" type="button" :disabled="!canDeactivateSelectedBoms || loading" @click="deactivateSelectedProductionBoms">
            批量失效
          </button>
        </div>
        <div class="table-wrap bom-list-panel-scroll">
          <table>
            <thead>
              <tr>
                <th class="select-col"><input type="checkbox" :checked="isAllVisibleBomsSelected" :indeterminate.prop="isSomeVisibleBomsSelected" @change="toggleAllVisibleBoms" /></th>
                <th>BOM名称</th>
                <th>分组</th>
                <th>状态</th>
                <th>组件数</th>
                <th>产出商品</th>
                <th>更新时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in productionBomRows"
                :key="bomRowKey(row)"
                :class="{ active: bomRowKey(row) === activeBomRowKey }"
                @click="selectBomRow(row)">
                <td class="select-col">
                  <input
                    v-model="selectedBomRowKeys"
                    type="checkbox"
                    :value="bomRowKey(row)"
                    :disabled="!isMovableBomRow(row)"
                    @click.stop />
                </td>
                <td>
                  <button class="text-button bom-name-button" type="button" @click.stop="openBomRowPrimary(row)">
                    {{ productionBomLabel(row) }}
                  </button>
                  <small v-if="productionBomVersionWarning(row)" class="bom-version-warning" data-warning-prefix="当前引用">{{ productionBomVersionWarning(row) }}</small>
                </td>
                <td>{{ bomGroupLabel(row) }}</td>
                <td><span :class="['status-pill', row.status === 'inactive' ? 'inactive' : '']">{{ bomStatusLabel(row.status) }}</span></td>
                <td>{{ row.item_count || row.material_count || 0 }}</td>
                <td>{{ row.output_product_name || '-' }}</td>
                <td>{{ row.updated_at }}</td>
                <td>
                  <button class="text-button" type="button" :disabled="!Number(row.production_bom_id || row.id || 0)" @click.stop="copyProductionBomRecord(bomRecordFromRow(row))">复制</button>
                  <button class="text-button danger-text" type="button" :disabled="!Number(row.production_bom_id || row.id || 0) || row.status === 'inactive'" @click.stop="deactivateProductionBomRecord(bomRecordFromRow(row))">失效</button>
                </td>
              </tr>
              <tr v-if="!productionBomRows.length">
                <td colspan="8" class="muted">暂无配方档案</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="panel detail-panel">
        <div class="panel-title">BOM 明细</div>
        <div v-if="detail" class="summary">
          <div><span>生产 BOM</span><strong>{{ currentProductionBomLabel }}</strong></div>
          <div><span>产出商品</span><strong>{{ outputProductLabel(detail) }}</strong></div>
          <div><span>产出数量</span><strong>{{ currentOutputBasisLabel }}</strong></div>
          <div><span>多层展开</span><strong>{{ usedByBoms.length ? `${usedByBoms.length} 个上层 BOM` : '可作为库存件' }}</strong></div>
          <div v-if="currentProductionBomWarning"><span>版本提示</span><strong class="warn">{{ currentProductionBomWarning }}</strong></div>
          <div><span>工艺参数</span><strong>{{ detail.roast_level || '-' }}</strong></div>
          <div><span>状态</span><strong :class="{ warn: detail.status === 'inactive' }">{{ bomStatusLabel(detail.status) }}</strong></div>
          <div><span>关联工艺</span><strong>{{ linkedProcessTemplates.length ? `${linkedProcessTemplates.length} 个模板` : '-' }}</strong></div>
        </div>
        <div v-if="detail && linkedProcessTemplates.length" class="linked-processes">
          <span v-for="template in linkedProcessTemplates" :key="template.id" :class="['status-pill', template.status === 'inactive' ? 'inactive' : '']">
            {{ template.name }} · {{ processStatusLabel(template.status) }}
          </span>
        </div>
        <div v-if="detail && referencedProducts.length" class="linked-processes referenced-products">
          <button
            v-for="product in referencedProducts"
            :key="referencedProductKey(product)"
            type="button"
            class="status-pill referenced-product-button"
            @click="openReferencedProductConfig(product)">
            {{ product.product_name || `商品 #${product.product_id}` }}<template v-if="product.product_code"> · {{ product.product_code }}</template>
          </button>
        </div>
        <div v-if="detail && usedByBoms.length" class="detail-subpanel compact-subpanel">
          <div class="section-title-row">
            <div>
              <h3>被哪些 BOM 使用</h3>
              <p class="muted left">这些上层 BOM 会把当前产出商品当作商品组件消耗。</p>
            </div>
          </div>
          <div class="table-wrap compact">
            <table>
              <thead>
                <tr>
                  <th>上层 BOM</th>
                  <th>产出商品</th>
                  <th>组件用量</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in usedByBoms" :key="`${row.bom_id}:${row.bom_version_id}:${row.consume_unit}`">
                  <td>{{ row.bom_code }} {{ row.bom_name }}<small>{{ row.bom_version_no }}</small></td>
                  <td>{{ row.output_product_name || '-' }}</td>
                  <td>{{ qty(row.qty_per_unit) }} {{ consumeUnitLabel(row.consume_unit) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div v-if="detail?.status === 'inactive'" class="warning-banner">当前 BOM 已失效，历史配方明细会保留；重新保存或启用版本后可恢复为有效 BOM。</div>
        <div v-if="!detail" class="muted empty">请选择生产 BOM</div>

        <div v-if="detail" class="detail-subpanel bom-version-panel">
          <div class="section-title-row">
            <div>
              <h3>BOM版本</h3>
              <p class="muted left">{{ currentProductionBomLabel }}</p>
            </div>
            <div class="inline-actions">
              <input v-model.trim="versionNote" placeholder="版本备注，例如 2026 春季豆单" :disabled="!currentProductionBomID || !canEditCurrentBomProduct" />
              <button class="primary compact-action" type="button" @click="copyVersionAsDraft()" :disabled="loading || !canCopyCurrentVersionAsDraft">复制为新版草稿</button>
            </div>
          </div>
          <div v-if="selectedProductionBomVersion?.status === 'published'" class="warning-banner">已发布版本只读，复制为新版草稿后编辑</div>
          <div class="table-wrap compact">
            <table>
              <thead>
                <tr>
                  <th>版本</th>
                  <th>状态</th>
                  <th>组件数</th>
                  <th>产出基准</th>
                  <th>备注</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="version in versions"
                  :key="version.id"
                  :class="{ active: Number(version.id || 0) === Number(selectedProductionBomVersionID || 0) }"
                  @click="selectProductionBomVersion(version, { reload: true })">
                  <td>{{ version.version_no }}</td>
                  <td>{{ productionBomVersionStatusLabel(version.status) }}</td>
                  <td>{{ version.item_count }}</td>
                  <td>{{ qty(version.output_qty || 1) }} {{ version.output_unit || 'unit' }}</td>
                  <td>{{ version.note }}</td>
                  <td>{{ version.published_at || version.created_at }}</td>
                  <td>
                    <button
                      v-if="version.status === 'draft'"
                      class="text-button"
                      type="button"
                      @click.stop="activateVersion(version.id)"
                      :disabled="!canEditCurrentBomProduct">
                      发布版本
                    </button>
                    <button
                      v-else-if="version.status === 'published'"
                      class="text-button"
                      type="button"
                      @click.stop="copyVersionAsDraft(version)"
                      :disabled="!canEditCurrentBomProduct">
                      复制为新版草稿
                    </button>
                    <span v-else class="muted left">只读</span>
                  </td>
                </tr>
                <tr v-if="!versions.length">
                  <td colspan="7" class="muted">暂无版本</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="version-recipe-panel">
            <div class="section-title-row">
              <div>
                <h3>配方明细</h3>
                <p class="muted left">当前编辑版本：{{ selectedProductionBomVersion?.version_no || '-' }} · {{ productionBomVersionStatusLabel(selectedProductionBomVersion?.status || '') }}</p>
              </div>
              <div class="version-ratio-box">
                <span>合计比例</span>
                <strong :class="{ warn: detail.total_ratio > 100 }">{{ ratio(detail.total_ratio) }}</strong>
              </div>
            </div>
            <form class="inline-form" @submit.prevent="saveItem">
              <label>
                <span>组件来源</span>
                <select v-model="itemForm.component_type" :disabled="!detail || !canEditCurrentBomItems" @change="syncComponentTypeDefaults">
                  <option value="material">物料</option>
                  <option value="product">商品组件</option>
                </select>
              </label>
              <label>
                <span>{{ itemForm.component_type === 'product' ? '商品组件' : '物料' }}</span>
                <SearchableSelect
                  v-if="itemForm.component_type === 'product'"
                  v-model="itemForm.component_product_id"
                  :options="productComponentOptions"
                  :option-label="optionLabel"
                  :option-meta="optionMeta"
                  :option-value="optionNumericValue"
                  placeholder="选择半成品或成品商品"
                  empty-text="没有可用商品组件"
                  :disabled="!detail || !canEditCurrentBomItems" />
                <SearchableSelect
                  v-else
                  v-model="itemForm.material_id"
                  :options="materials"
                  :option-label="optionLabel"
                  :option-value="optionNumericValue"
                  placeholder="选择物料"
                  empty-text="没有匹配物料"
                  :disabled="!detail || !canEditCurrentBomItems" />
              </label>
              <label>
                <span>消耗单位</span>
                <select v-model="itemForm.consume_unit" :disabled="!detail || !canEditCurrentBomItems">
                  <option v-for="unit in currentConsumeUnitOptions" :key="unit.value" :value="unit.value">{{ unit.label }}</option>
                </select>
              </label>
              <label v-if="itemForm.consume_unit === 'ratio_pct'">
                <span>比例 %</span>
                <input v-model.number="itemForm.ratio_pct" type="number" min="0.01" max="100" step="0.01" :disabled="!detail || !canEditCurrentBomItems" />
              </label>
              <label v-else>
                <span>用量</span>
                <input v-model.number="itemForm.qty_per_unit" type="number" min="0.001" step="0.001" :disabled="!detail || !canEditCurrentBomItems" />
              </label>
              <button class="primary" type="submit" :disabled="!detail || loading || !canEditCurrentBomItems">保存组件</button>
            </form>

            <div class="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>类型</th>
                    <th>组件</th>
                    <th>用量</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in detailItems" :key="item.id">
                    <td>{{ componentTypeLabel(item.component_type) }}</td>
                    <td>{{ componentItemName(item) }}</td>
                    <td>{{ itemQuantityDisplay(item) }}</td>
                    <td><button class="text-button" type="button" :disabled="!canEditCurrentBomItems" @click="deleteItem(item.id)">删除</button></td>
                  </tr>
                  <tr v-if="!detailItems.length">
                    <td colspan="4" class="muted">暂无组件</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

      </section>
    </div>

    <div v-if="bomDrawerOpen" class="drawer-mask" @click.self="closeBomDrawer">
      <aside class="drawer">
        <div class="drawer-head">
          <div>
            <h3>{{ bomFormTitle }}</h3>
            <p class="muted left">先声明产出商品；版本和组件清单在右侧维护。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeBomDrawer">关闭</button>
        </div>
        <form class="inline-form bom-record-form" @submit.prevent="saveProductionBomRecord">
          <label>
            <span>BOM名称</span>
            <input v-model.trim="bomForm.name" placeholder="例如 精品拼配" />
          </label>
          <label>
            <span>产出商品</span>
            <SearchableSelect
              v-model="bomForm.output_product_id"
              :options="outputProductOptions"
              :option-label="optionLabel"
              :option-meta="optionMeta"
              :option-value="optionNumericValue"
              placeholder="选择产出商品"
              empty-text="没有可用商品档案" />
          </label>
          <label>
            <span>产出数量</span>
            <input v-model.number="bomForm.output_qty" type="number" min="0.001" step="0.001" placeholder="例如 1" :disabled="!canEditBomFormOutputBasis" />
          </label>
          <label>
            <span>产出单位</span>
            <input v-model.trim="bomForm.output_unit" placeholder="例如 盒 / 条 / kg" :disabled="!canEditBomFormOutputBasis" />
          </label>
          <label v-if="bomForm.mode === 'edit'">
            <span>状态</span>
            <select v-model="bomForm.status">
              <option value="active">启用</option>
              <option value="inactive">已失效</option>
            </select>
          </label>
          <button class="primary" type="submit" :disabled="loading || !bomForm.name || !Number(bomForm.output_product_id || 0)">{{ bomForm.mode === 'copy' ? '复制 BOM' : '保存 BOM' }}</button>
        </form>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import SearchableSelect from '../components/SearchableSelect.vue'
import { bomProductOptionLabel, filterProductionBomCatalog, isBomProductCandidate, productionBomDetailAsRecipeDetail, productionBomLabel, productionBomVersionWarning } from '../lib/bom'
import { componentTypeLabel } from '../lib/drip-product'
import { FORM_DRAFT_SCOPES, readFormDraft, saveFormDraft } from '../lib/form-draft-cache'
import { businessGroupAssignmentLabel, businessGroupItemMoveOptions, buildBusinessGroupAssignmentPayload, isSystemDefaultBusinessGroup } from '../lib/product-settings'
import { replaceHistoryURL } from '../lib/url-state'
import { CUSTOMER_WORKSPACE_MODE } from '../lib/workspace-mode'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})
const BOM_FORM_DRAFT_SCOPE = FORM_DRAFT_SCOPES.bom

const productionBoms = ref([])
const products = ref([])
const materials = ref([])
const productUnitDefinitions = ref([])
const versions = ref([])
const processTemplates = ref([])
const productionBomBusinessGroups = ref([])
const productionBomDetail = ref(null)
const selectedProductionBomRecord = ref(null)
const detail = ref(null)
const selectedProductId = ref(0)
const selectedProductionBomGroupItemID = ref(0)
const selectedProductionBomVersionID = ref(0)
const selectedBomMoveGroupItemID = ref(0)
const selectedBomRowKeys = ref([])
const pendingProductionBomID = ref(0)
const bomReturnNavigation = computed(() => props.viewParams?.return_navigation || null)
const bomReturnProductID = computed(() => Number(bomReturnNavigation.value?.params?.open_product_config_id || 0))
const bomReturnLabel = computed(() => String(bomReturnNavigation.value?.label || '返回商品档案配置'))
const bomDrawerOpen = ref(false)
const productionBomStatusFilter = ref('active')
const productionBomSearchQuery = ref('')
const loading = ref(false)
const error = ref('')
const ok = ref('')
const itemForm = reactive({
  component_type: 'material',
  material_id: 0,
  component_product_id: 0,
  component_spec_g: 0,
  consume_unit: 'ratio_pct',
  qty_per_unit: '',
  ratio_pct: '',
})
const bomForm = reactive({ id: 0, source_id: 0, mode: 'create', name: '', output_product_id: 0, output_qty: 1, output_unit: 'unit', status: 'active' })
const versionNote = ref('')

const detailItems = computed(() => detail.value?.items || [])
const isWorkspaceCustomerLocked = computed(() => props.workspaceMode === CUSTOMER_WORKSPACE_MODE && Number(props.customerContextId || 0) > 0)
const productionBomGroupOptions = computed(() => businessGroupItemMoveOptions(productionBomBusinessGroups.value, 'production_bom'))
const productionBomRows = computed(() => {
  return filterProductionBomCatalog(productionBoms.value, {
    status: productionBomStatusFilter.value,
    query: productionBomSearchQuery.value,
    groupItemID: Number(selectedProductionBomGroupItemID.value || 0),
  })
})
const linkedProcessTemplates = computed(() => processTemplates.value.filter((template) => Number(template.product_id || 0) === Number(selectedProductId.value || 0)))
const selectedProduct = computed(() => productByID(selectedProductId.value))
const rawReferencedProducts = computed(() => detail.value?.referenced_products || productionBomDetail.value?.referenced_products || [])
const referencedProducts = computed(() => {
  const seen = new Set()
  const rows = []
  for (const product of rawReferencedProducts.value || []) {
    if (!isActiveReferencedProduct(product)) continue
    const key = referencedProductKey(product)
    if (seen.has(key)) continue
    seen.add(key)
    rows.push(product)
  }
  return rows
})
const usedByBoms = computed(() => detail.value?.used_by_boms || productionBomDetail.value?.used_by_boms || [])
const referencedProductsLabel = computed(() => {
  const count = referencedProducts.value.length
  if (count > 0) return `${count} 个商品`
  if ((rawReferencedProducts.value || []).length > 0) return '未被商品引用'
  const summaryCount = Number(selectedProductionBomRecord.value?.reference_product_count || detail.value?.reference_product_count || productionBomDetail.value?.reference_product_count || 0)
  return summaryCount > 0 ? `${summaryCount} 个商品` : '未被商品引用'
})
const currentProductionBomLabel = computed(() => productionBomLabel(detail.value || selectedProductionBomRecord.value || {}))
const currentProductionBomWarning = computed(() => productionBomVersionWarning(detail.value || selectedProductionBomRecord.value || {}))
const currentProductionBomID = computed(() => Number(detail.value?.production_bom_id || selectedProductionBomRecord.value?.production_bom_id || selectedProductionBomRecord.value?.id || 0))
const currentOutputBasisLabel = computed(() => `${qty(selectedProductionBomVersion.value?.output_qty || 1)} ${selectedProductionBomVersion.value?.output_unit || 'unit'}`)
const bomFormTitle = computed(() => ({
  create: '新建生产 BOM',
  edit: '编辑 BOM',
  copy: '复制 BOM',
})[bomForm.mode] || '生产 BOM')
const selectedProductionBomVersion = computed(() => versions.value.find((version) => Number(version.id || 0) === Number(selectedProductionBomVersionID.value || 0)) || null)
const activeBomRowKey = computed(() => {
  if (selectedProductionBomRecord.value) return `bom:${Number(selectedProductionBomRecord.value.id || selectedProductionBomRecord.value.production_bom_id || 0)}`
  return ''
})
const canEditCurrentBomProduct = computed(() => {
  if (selectedProductionBomRecord.value) return true
  if (!selectedProductId.value) return true
  if (detail.value?.can_edit_bom === false) return false
  return true
})
const selectedProductionBomDraftVersion = computed(() => versions.value.find((version) => Number(version.id || 0) === Number(selectedProductionBomVersionID.value || 0) && version.status === 'draft') || null)
const canEditCurrentBomItems = computed(() => canEditCurrentBomProduct.value && Number(selectedProductionBomDraftVersion.value?.id || 0) > 0)
const canCopyCurrentVersionAsDraft = computed(() => canEditCurrentBomProduct.value && currentProductionBomID.value > 0 && selectedProductionBomVersion.value?.status === 'published')
const canEditBomFormOutputBasis = computed(() => bomForm.mode !== 'edit' || canEditCurrentBomItems.value)
const outputProductOptions = computed(() => products.value.filter(isBomProductCandidate))
const productComponentOptions = computed(() => products.value.filter(isBomProductCandidate).filter((product) => Number(product.id || 0) !== Number(detail.value?.output_product_id || productionBomDetail.value?.output_product_id || 0)))
const ratioConsumeUnitOption = { value: 'ratio_pct', label: '比例 %' }
const legacyConsumeUnitLabels = {
  g_per_bag: '克/袋',
  unit_per_bag: '个/袋',
  unit_per_box: '个/盒',
  fixed_qty: '固定数量',
}
const unitDictionaryConsumeUnitOptions = computed(() => productUnitDefinitions.value
  .filter((unit) => unit.active !== false)
  .map((unit) => {
    const value = String(unit.code || '').trim()
    return value ? { value, label: unit.name ? `${unit.name}（${value}）` : value } : null
  })
  .filter(Boolean)
  .sort((a, b) => String(a.label || '').localeCompare(String(b.label || ''))))
const currentConsumeUnitOptions = computed(() => itemForm.component_type === 'product'
  ? consumeUnitOptionsWithCurrent(false, itemForm.consume_unit)
  : consumeUnitOptionsWithCurrent(true, itemForm.consume_unit))
const visibleMovableBomRows = computed(() => productionBomRows.value.filter(isMovableBomRow))
const isAllVisibleBomsSelected = computed(() => {
  const keys = visibleMovableBomRows.value.map(bomRowKey)
  if (!keys.length) return false
  const selected = new Set(selectedBomRowKeys.value)
  return keys.every((key) => selected.has(key))
})
const isSomeVisibleBomsSelected = computed(() => {
  const keys = visibleMovableBomRows.value.map(bomRowKey)
  if (!keys.length) return false
  const selected = new Set(selectedBomRowKeys.value)
  const count = keys.filter((key) => selected.has(key)).length
  return count > 0 && count < keys.length
})
const selectedBomRows = computed(() => {
  const selected = new Set(selectedBomRowKeys.value)
  return productionBoms.value.filter((row) => selected.has(bomRowKey(row)) && isMovableBomRow(row))
})
const selectedBomRecordsForMove = computed(() => {
  const byBomID = new Map()
  for (const row of selectedBomRows.value) {
    const record = bomRecordFromRow(row)
    if (record.id > 0 && !byBomID.has(record.id)) byBomID.set(record.id, record)
  }
  return [...byBomID.values()]
})
const selectedActiveBomRecordsForDeactivate = computed(() => {
  const byBomID = new Map()
  for (const row of selectedBomRows.value) {
    const record = bomRecordFromRow(row)
    if (record.id > 0 && record.status !== 'inactive' && !byBomID.has(record.id)) byBomID.set(record.id, record)
  }
  return [...byBomID.values()]
})
const canMoveSelectedBoms = computed(() => {
  if (!selectedBomRecordsForMove.value.length) return false
  const targetGroupItemID = Number(selectedBomMoveGroupItemID.value || 0)
  const targetOption = targetGroupItemID > 0 ? productionBomGroupOptionByItemID(targetGroupItemID) : null
  if (targetGroupItemID > 0 && !targetOption) return false
  return selectedBomRecordsForMove.value.some((bom) => {
    if (!targetOption) return productionBomGroupID(bom) > 0 || productionBomGroupItemID(bom) > 0
    return productionBomGroupID(bom) !== Number(targetOption.group_id || 0) || productionBomGroupItemID(bom) !== Number(targetOption.group_item_id || 0)
  })
})
const canDeactivateSelectedBoms = computed(() => selectedActiveBomRecordsForDeactivate.value.length > 0)

function ratio(value) {
  const n = Number(value || 0)
  return `${n.toFixed(2)}%`
}

function qty(value) {
  const n = Number(value || 0)
  if (!n) return '-'
  return Number.isInteger(n) ? String(n) : n.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
}

function bomRowKey(row) {
  const bomID = Number(row?.production_bom_id || row?.id || 0)
  if (bomID > 0) return `bom:${bomID}`
  return `product:${Number(row?.product_id || 0)}`
}

function productionBomGroupID(row = {}) {
  return Number(row.business_group_id ?? row.group_id ?? row.production_bom_group_id ?? 0) || 0
}

function productionBomGroupItemID(row = {}) {
  return Number(row.group_item_id ?? row.business_group_item_id ?? row.group_category_id ?? row.production_bom_group_category_id ?? 0) || 0
}

function productionBomGroupOptionByItemID(groupItemID) {
  const id = Number(groupItemID || 0)
  return productionBomGroupOptions.value.find((option) => Number(option.group_item_id || 0) === id) || null
}

function productionBomBusinessGroupAssignment(row = {}) {
  const groupID = productionBomGroupID(row)
  const group = productionBomBusinessGroups.value.find((item) => Number(item.id || 0) === groupID) || null
  if (!group || isSystemDefaultBusinessGroup(group)) {
    return { usage_key: 'production_bom', object_key: 'production_bom', object_id: Number(row.production_bom_id || row.id || 0), group_id: 0, group_item_id: 0 }
  }
  return {
    usage_key: 'production_bom',
    object_key: 'production_bom',
    object_id: Number(row.production_bom_id || row.id || 0),
    group_id: groupID,
    group_item_id: productionBomGroupItemID(row),
  }
}

function isInactiveMarker(value) {
  if (value === false || value === 0) return true
  const normalized = String(value ?? '').trim().toLowerCase()
  return ['inactive', 'disabled', 'deprecated', 'deactivated', 'archived', 'false', '0', '失效'].includes(normalized)
}

function isActiveReferencedProduct(product = {}) {
  return !isInactiveMarker(product.active) && !isInactiveMarker(product.status) && !isInactiveMarker(product.product_status)
}

function referencedProductKey(product = {}) {
  const productID = Number(product.product_id || product.id || 0)
  if (productID > 0) return `product:${productID}`
  const code = String(product.product_code || product.code || '').trim()
  const name = String(product.product_name || product.name || '').trim()
  return `product:${code}:${name}`
}

function isMovableBomRow(row = {}) {
  return Number(row.production_bom_id || 0) > 0
}

function optionLabel(option) {
  return bomProductOptionLabel(option)
}

function optionMeta(option) {
  const parts = []
  parts.push('商品档案')
  if (option?.number) parts.push(option.number)
  if (option?.roast_level) parts.push(option.roast_level)
  return parts.join(' / ')
}

function optionNumericValue(option) {
  return Number(option?.id || 0)
}

function bomFormDraftKey() {
  const workspace = props.workspaceMode || 'factory'
  const customerID = Number(props.customerContextId || 0)
  return `${BOM_FORM_DRAFT_SCOPE}:${workspace}:${customerID || 'all'}`
}

function saveBomFormDraft() {
  saveFormDraft(bomFormDraftKey(), {
    selectedProductionBomID: currentProductionBomID.value,
    itemForm: { ...itemForm },
    versionNote: versionNote.value,
  })
}

async function restoreBomFormDraft() {
  const params = new URL(window.location.href).searchParams
  if (Number(params.get('product_id') || 0) > 0) return
  if (Number(params.get('production_bom_id') || params.get('bom_id') || 0) > 0) return
  const draft = readFormDraft(bomFormDraftKey())
  if (!draft) return
  pendingProductionBomID.value = Number(draft.selectedProductionBomID || 0)
  Object.assign(itemForm, {
    component_type: 'material',
    material_id: 0,
    component_product_id: 0,
    component_spec_g: 0,
    consume_unit: 'ratio_pct',
    qty_per_unit: '',
    ratio_pct: '',
  }, draft.itemForm || {})
  versionNote.value = draft.versionNote || ''
  if (pendingProductionBomID.value > 0) {
    await selectProductionBomRecordByID(pendingProductionBomID.value)
  }
}

function normalizeBomProduct(product) {
  return {
    ...product,
    id: Number(product.id || 0),
    customer_id: Number(product.customer_id || 0),
  }
}

function normalizeProductionBomRecord(row = {}) {
  const groupID = Number(row.business_group_id ?? row.group_id ?? row.production_bom_group_id ?? 0) || 0
  const groupItemID = Number(row.group_item_id ?? row.business_group_item_id ?? row.group_category_id ?? row.production_bom_group_category_id ?? 0) || 0
  return {
    ...row,
    id: Number(row.id || row.production_bom_id || 0),
    production_bom_id: Number(row.production_bom_id || row.id || 0),
    output_product_id: Number(row.output_product_id || 0),
    output_product_name: row.output_product_name || '',
    output_product_code: row.output_product_code || '',
    business_group_id: groupID,
    business_group_name: row.business_group_name || row.group_name || row.production_bom_group_name || '',
    group_id: groupID,
    production_bom_group_id: groupID,
    group_item_id: groupItemID,
    business_group_item_id: groupItemID,
    group_category_id: groupItemID,
    production_bom_group_category_id: groupItemID,
    group_item_name: row.group_item_name || row.group_category_name || row.production_bom_group_category_name || '',
    group_category_name: row.group_item_name || row.group_category_name || row.production_bom_group_category_name || '',
    production_bom_group_category_name: row.production_bom_group_category_name || row.group_item_name || row.group_category_name || '',
    item_count: Number(row.item_count || row.material_count || 0),
    reference_product_count: Number(row.reference_product_count || 0),
    latest_version_status: row.latest_version_status || '',
    status: row.status === 'inactive' ? 'inactive' : 'active',
  }
}

function productByID(productId) {
  const id = Number(productId || 0)
  return products.value.find((product) => Number(product.id || 0) === id) || null
}

function defaultDictionaryConsumeUnit() {
  return unitDictionaryConsumeUnitOptions.value[0]?.value || 'unit'
}

function consumeUnitOptionsWithCurrent(includeRatio, currentUnit) {
  const options = includeRatio ? [ratioConsumeUnitOption, ...unitDictionaryConsumeUnitOptions.value] : [...unitDictionaryConsumeUnitOptions.value]
  const current = String(currentUnit || '').trim()
  if (current && current !== 'ratio_pct' && !options.some((option) => option.value === current)) {
    options.push({ value: current, label: legacyConsumeUnitLabels[current] || current })
  }
  if (!options.length) options.push({ value: 'unit', label: 'unit' })
  return options
}

function consumeUnitLabel(unit) {
  const value = String(unit || '').trim()
  if (value === ratioConsumeUnitOption.value) return ratioConsumeUnitOption.label
  return unitDictionaryConsumeUnitOptions.value.find((option) => option.value === value)?.label || legacyConsumeUnitLabels[value] || value || '-'
}

function componentItemName(item) {
  if (item?.component_type === 'product' || item?.component_type === 'finished_product') return item.component_product_name || `商品 #${item.component_product_id}`
  return item?.material_name || `物料 #${item?.material_id || 0}`
}

function itemQuantityDisplay(item) {
  if ((item?.consume_unit || 'ratio_pct') === 'ratio_pct') return ratio(item.ratio_pct)
  return `${qty(item.qty_per_unit)} ${consumeUnitLabel(item.consume_unit)}`
}

function productionBomDraftItemFromItem(item = {}) {
  return {
    material_id: Number(item.material_id || 0),
    component_type: (item.component_type === 'product' || item.component_type === 'finished_product') ? 'product' : 'material',
    component_product_id: Number(item.component_product_id || 0),
    component_spec_g: Number(item.component_spec_g || 0),
    consume_unit: item.consume_unit || 'ratio_pct',
    qty_per_unit: Number(item.qty_per_unit || 0),
    ratio_pct: Number(item.ratio_pct || 0),
  }
}

function productionBomDraftItemFromForm() {
  return {
    material_id: Number(itemForm.material_id || 0),
    component_type: itemForm.component_type === 'product' ? 'product' : 'material',
    component_product_id: Number(itemForm.component_product_id || 0),
    component_spec_g: Number(itemForm.component_spec_g || 0),
    consume_unit: itemForm.consume_unit || 'ratio_pct',
    qty_per_unit: Number(itemForm.qty_per_unit || 0),
    ratio_pct: Number(itemForm.ratio_pct || 0),
  }
}

async function saveProductionBomDraftItems(items, basis = {}) {
  const draftVersionID = Number(selectedProductionBomDraftVersion.value?.id || 0)
  if (!draftVersionID) throw new Error('请先复制为新版草稿后再编辑配方明细')
  await apiSend(`/api/production-bom-versions/${draftVersionID}/draft`, {
    method: 'PUT',
    body: {
      output_qty: Number(basis.output_qty || selectedProductionBomVersion.value?.output_qty || 1),
      output_unit: String(basis.output_unit || selectedProductionBomVersion.value?.output_unit || 'unit').trim() || 'unit',
      items,
    },
  })
}

function productionBomVersionStatusLabel(status) {
  if (status === 'draft') return '草稿'
  if (status === 'published' || status === 'active') return '已发布'
  if (status === 'archived') return '已归档'
  return status || '-'
}

function selectProductionBomGroupItem(groupItemID) {
  selectedProductionBomGroupItemID.value = Number(groupItemID || 0)
  const groupOnlyRows = filterProductionBomCatalog(productionBoms.value, {
    status: 'all',
    query: '',
    groupItemID: Number(selectedProductionBomGroupItemID.value || 0),
  })
  const visibleBomIDs = new Set(groupOnlyRows.map((bom) => Number(bom.production_bom_id || bom.id || 0)))
  const selectedBomID = Number(selectedProductionBomRecord.value?.id || selectedProductionBomRecord.value?.production_bom_id || detail.value?.production_bom_id || 0)
  if (selectedBomID && !visibleBomIDs.has(selectedBomID)) {
    clearSelectedProductionBom()
  }
}

function bomGroupLabel(row) {
  return businessGroupAssignmentLabel(productionBomBusinessGroupAssignment(row), productionBomBusinessGroups.value)
}

function bomRecordFromRow(row = {}) {
  const groupID = productionBomGroupID(row)
  const groupItemID = productionBomGroupItemID(row)
  return {
    id: Number(row.production_bom_id || row.id || 0),
    code: row.production_bom_code || row.code || '',
    name: row.production_bom_name || row.name || row.product || '',
    output_product_id: Number(row.output_product_id || 0),
    output_product_name: row.output_product_name || '',
    output_product_code: row.output_product_code || '',
    business_group_id: groupID,
    business_group_name: row.business_group_name || row.production_bom_group_name || row.group_name || '',
    group_id: groupID,
    group_name: row.business_group_name || row.production_bom_group_name || row.group_name || '',
    group_item_id: groupItemID,
    group_category_id: groupItemID,
    group_item_name: row.group_item_name || row.production_bom_group_category_name || row.group_category_name || '',
    group_category_name: row.group_item_name || row.production_bom_group_category_name || row.group_category_name || '',
    status: row.status === 'inactive' ? 'inactive' : 'active',
  }
}

function toggleAllVisibleBoms(event) {
  const shouldSelect = Boolean(event?.target?.checked)
  const next = new Set(selectedBomRowKeys.value)
  for (const row of visibleMovableBomRows.value) {
    const key = bomRowKey(row)
    if (shouldSelect) next.add(key)
    else next.delete(key)
  }
  selectedBomRowKeys.value = [...next]
}

async function selectProductionBomVersion(version, options = {}) {
  const versionID = Number(version?.id || version || 0)
  selectedProductionBomVersionID.value = versionID
  if (options.reload && currentProductionBomID.value > 0 && versionID > 0) {
    await loadProductionBomDetailForVersion(currentProductionBomID.value, versionID)
  }
}

function syncSelectedProductionBomVersion() {
  if (!versions.value.length) {
    selectedProductionBomVersionID.value = 0
    return
  }
  const existing = versions.value.find((version) => Number(version.id || 0) === Number(selectedProductionBomVersionID.value || 0))
  const selected = existing || versions.value.find((version) => version.status === 'draft') || versions.value.find((version) => version.is_latest) || versions.value[0]
  selectedProductionBomVersionID.value = Number(selected?.id || 0)
}

function resetBomForm() {
  bomForm.id = 0
  bomForm.source_id = 0
  bomForm.mode = 'create'
  bomForm.name = ''
  bomForm.output_product_id = 0
  bomForm.output_qty = 1
  bomForm.output_unit = 'unit'
  bomForm.status = 'active'
}

function openNewProductionBomRecord() {
  resetBomForm()
  bomForm.mode = 'create'
  bomDrawerOpen.value = true
}

async function openEditProductionBomRecord(bom) {
  resetBomForm()
  const record = bomRecordFromRow(bom)
  bomForm.mode = 'edit'
  bomForm.id = record.id
  bomForm.name = record.name || ''
  bomForm.output_product_id = Number(record.output_product_id || 0)
  bomForm.status = record.status === 'inactive' ? 'inactive' : 'active'
  bomDrawerOpen.value = true
  if (record.id > 0 && Number(currentProductionBomID.value || 0) !== record.id) {
    await selectUnboundProductionBom(record)
  }
  bomForm.output_qty = Number(selectedProductionBomVersion.value?.output_qty || 1)
  bomForm.output_unit = selectedProductionBomVersion.value?.output_unit || defaultDictionaryConsumeUnit()
}

function copyProductionBomRecord(bom) {
  resetBomForm()
  bomForm.mode = 'copy'
  bomForm.source_id = Number(bom?.id || 0)
  bomForm.name = `${bom?.name || '生产 BOM'} 副本`
  bomForm.output_product_id = Number(bom?.output_product_id || 0)
  bomForm.output_qty = Number(selectedProductionBomVersion.value?.output_qty || 1)
  bomForm.output_unit = selectedProductionBomVersion.value?.output_unit || 'unit'
  bomForm.status = 'active'
  bomDrawerOpen.value = true
}

function closeBomDrawer() {
  bomDrawerOpen.value = false
  resetBomForm()
}

function syncComponentTypeDefaults() {
  if (itemForm.component_type === 'product') {
    itemForm.material_id = 0
    itemForm.consume_unit = defaultDictionaryConsumeUnit()
    itemForm.ratio_pct = ''
    return
  }
  itemForm.component_product_id = 0
  itemForm.component_spec_g = 0
  itemForm.consume_unit = 'ratio_pct'
  itemForm.qty_per_unit = ''
}

function resetItemForm() {
  itemForm.component_type = 'material'
  itemForm.material_id = 0
  itemForm.component_product_id = 0
  itemForm.component_spec_g = 0
  itemForm.consume_unit = 'ratio_pct'
  itemForm.qty_per_unit = ''
  itemForm.ratio_pct = ''
}

function clearSelectedProductionBom() {
  selectedProductId.value = 0
  detail.value = null
  productionBomDetail.value = null
  selectedProductionBomRecord.value = null
  versions.value = []
  selectedProductionBomVersionID.value = 0
  updateUrl()
}

function updateUrl() {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'bom')
  const bomID = currentProductionBomID.value
  if (bomID) url.searchParams.set('production_bom_id', String(bomID))
  else url.searchParams.delete('production_bom_id')
  url.searchParams.delete('product_id')
  url.searchParams.delete('bom_filter_product_id')
  replaceHistoryURL(url)
}

function returnToProductConfig() {
  const navigation = bomReturnNavigation.value
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: String(navigation?.key || 'productMaster'),
      params: navigation?.params || (bomReturnProductID.value > 0 ? { open_product_config_id: bomReturnProductID.value } : {}),
    },
  }))
}

function openReferencedProductConfig(product) {
  const productID = Number(product?.product_id || product?.id || 0)
  if (!productID) return
  const bomID = currentProductionBomID.value
  const labelProductName = product?.product_name || `商品 #${productID}`
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: 'productMaster',
      params: { open_product_config_id: productID },
      returnNavigation: {
        key: 'bom',
        label: `返回BOM编辑：${currentProductionBomLabel.value || '生产 BOM'}`,
        params: bomID > 0 ? { production_bom_id: bomID } : {},
        source_label: `商品档案配置：${labelProductName}`,
        targetKey: 'productMaster',
      },
    },
  }))
}

async function loadProductUnitDefinitions() {
  const rows = await apiGet('/api/product-settings/units')
  return Array.isArray(rows) ? rows : []
}

async function loadAll() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const [productData, materialData, unitData, processData, productionGroupData, productionBomData] = await Promise.all([
      apiGet('/api/bom/products'),
      apiGet('/api/bom/materials'),
      loadProductUnitDefinitions(),
      apiGet('/api/process-templates'),
      apiGet('/api/business-groups?usage_key=production_bom'),
      apiGet('/api/production-boms?status=all'),
    ])

    products.value = (productData || []).map(normalizeBomProduct)
    materials.value = materialData || []
    productUnitDefinitions.value = unitData || []
    processTemplates.value = processData.rows || []
    productionBomBusinessGroups.value = Array.isArray(productionGroupData?.rows) ? productionGroupData.rows : (Array.isArray(productionGroupData) ? productionGroupData : [])
    productionBoms.value = (productionBomData.rows || productionBomData || []).map(normalizeProductionBomRecord)
    if (pendingProductionBomID.value > 0) {
      const pendingID = pendingProductionBomID.value
      pendingProductionBomID.value = 0
      await selectProductionBomRecordByID(pendingID)
      return
    }
    const selectedBomID = currentProductionBomID.value
    if (selectedBomID > 0) {
      const current = productionBoms.value.find((bom) => Number(bom.id || bom.production_bom_id || 0) === selectedBomID)
      if (current) await selectUnboundProductionBom(current)
      else clearSelectedProductionBom()
    }
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadProductionBomVersions(bomID) {
  const id = Number(bomID || 0)
  if (!id) {
    productionBomDetail.value = null
    versions.value = []
    selectedProductionBomVersionID.value = 0
    return
  }
  productionBomDetail.value = await apiGet(`/api/production-boms/${id}`)
  versions.value = productionBomDetail.value?.versions || []
  syncSelectedProductionBomVersion()
}

async function loadProductionBomDetailForVersion(bomID, versionID = 0, fallbackRow = null) {
  const id = Number(bomID || 0)
  if (!id) return
  const query = Number(versionID || 0) > 0 ? `?version_id=${Number(versionID || 0)}` : ''
  productionBomDetail.value = await apiGet(`/api/production-boms/${id}${query}`)
  versions.value = productionBomDetail.value?.versions || []
  if (Number(versionID || 0) > 0) selectedProductionBomVersionID.value = Number(versionID || 0)
  else syncSelectedProductionBomVersion()
  const row = fallbackRow || selectedProductionBomRecord.value || {}
  detail.value = productionBomDetailAsRecipeDetail(productionBomDetail.value || {}, row)
  selectedProductionBomRecord.value = {
    ...bomRecordFromRow({ ...row, ...productionBomDetail.value }),
    id: detail.value.production_bom_id,
    production_bom_id: detail.value.production_bom_id,
    production_bom_code: detail.value.production_bom_code,
    production_bom_name: detail.value.production_bom_name,
    production_bom_group_id: detail.value.production_bom_group_id,
    production_bom_group_name: detail.value.production_bom_group_name,
    production_bom_group_category_id: detail.value.production_bom_group_category_id,
    production_bom_group_category_name: detail.value.production_bom_group_category_name,
    latest_bom_version_no: detail.value.latest_bom_version_no,
    production_bom_version_no: detail.value.production_bom_version_no,
    reference_product_count: productionBomDetail.value?.reference_product_count || row.reference_product_count || 0,
  }
}

async function selectProductionBomRecordByID(bomID) {
  const id = Number(bomID || 0)
  if (!id) return
  const record = productionBoms.value.find((row) => Number(row.id || row.production_bom_id || 0) === id) || { id, production_bom_id: id }
  await selectUnboundProductionBom(record)
}

async function selectUnboundProductionBom(row) {
  const record = bomRecordFromRow(row)
  if (!record.id) return
  selectedProductId.value = 0
  error.value = ''
  ok.value = ''
  selectedProductionBomRecord.value = {
    ...record,
    production_bom_id: record.id,
    production_bom_code: record.code,
    production_bom_name: record.name,
    production_bom_group_id: record.group_id,
    production_bom_group_name: record.group_name,
    production_bom_group_category_id: record.group_category_id,
    production_bom_group_category_name: record.group_category_name,
    latest_bom_version_no: row.latest_bom_version_no || row.latest_version_no || '',
    production_bom_version_no: row.production_bom_version_no || '',
  }
  try {
    await loadProductionBomDetailForVersion(record.id, 0, row)
  } catch (err) {
    detail.value = null
    productionBomDetail.value = null
    versions.value = []
    selectedProductionBomVersionID.value = 0
    error.value = err.message || '加载生产 BOM 配方失败'
  } finally {
    updateUrl()
  }
}

async function selectBomRow(row) {
  await selectUnboundProductionBom(row)
}

async function openBomRowPrimary(row) {
  await openEditProductionBomRecord(bomRecordFromRow(row))
}

function bomStatusLabel(status) {
  if (status === 'inactive') return '已失效'
  return '有效'
}

function productionBomRecordStatusLabel(status) {
  return status === 'inactive' ? '已失效' : '启用'
}

function processStatusLabel(status) {
  if (status === 'draft') return '草稿'
  if (status === 'active') return '已发布'
  if (status === 'inactive') return '已停用'
  return status || '-'
}

async function saveItem() {
  if (!canEditCurrentBomItems.value) return
  await mutate(async () => {
    const versionID = Number(selectedProductionBomVersionID.value || 0)
    const nextItems = detailItems.value.map(productionBomDraftItemFromItem)
    nextItems.push(productionBomDraftItemFromForm())
    await saveProductionBomDraftItems(nextItems)
    resetItemForm()
    ok.value = '已保存'
    await loadProductionBomDetailForVersion(currentProductionBomID.value, versionID)
  })
}

async function deleteItem(id) {
  if (!canEditCurrentBomItems.value) return
  await mutate(async () => {
    const versionID = Number(selectedProductionBomVersionID.value || 0)
    const deleteID = Number(id || 0)
    const nextItems = detailItems.value
      .filter((item) => Number(item.id || 0) !== deleteID)
      .map(productionBomDraftItemFromItem)
    await saveProductionBomDraftItems(nextItems)
    ok.value = '已删除'
    await loadProductionBomDetailForVersion(currentProductionBomID.value, versionID)
  })
}

function openBusinessGroupManagement() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: 'groupManagement',
      returnNavigation: {
        key: 'bom',
        label: '返回生产 BOM',
      },
    },
  }))
}

async function clearProductionBomBusinessGroupAssignment(bomID) {
  const id = Number(bomID || 0)
  if (!id) return
  const url = new URL('/api/business-group-assignments', window.location.origin)
  url.searchParams.set('usage_key', 'production_bom')
  url.searchParams.set('object_key', 'production_bom')
  url.searchParams.set('object_id', String(id))
  const data = await apiGet(url)
  const rows = Array.isArray(data?.rows) ? data.rows : (Array.isArray(data?.assignments) ? data.assignments : [])
  await Promise.all(rows.map((row) => apiSend(`/api/business-group-assignments/${row.id}`, { method: 'DELETE' })))
}

async function moveSelectedProductBomsToGroup() {
  const targetGroupItemID = Number(selectedBomMoveGroupItemID.value || 0)
  const targetOption = targetGroupItemID > 0 ? productionBomGroupOptionByItemID(targetGroupItemID) : null
  if (targetGroupItemID > 0 && !targetOption) return
  const records = selectedBomRecordsForMove.value.filter((bom) => {
    if (!targetOption) return productionBomGroupID(bom) > 0 || productionBomGroupItemID(bom) > 0
    return productionBomGroupID(bom) !== Number(targetOption.group_id || 0) || productionBomGroupItemID(bom) !== Number(targetOption.group_item_id || 0)
  })
  if (!records.length) return
  await mutate(async () => {
    for (const bom of records) {
      if (!targetOption) {
        await clearProductionBomBusinessGroupAssignment(bom.id)
        continue
      }
      await apiSend('/api/business-group-assignments', {
        body: buildBusinessGroupAssignmentPayload({
          usage_key: 'production_bom',
          object_key: 'production_bom',
          object_id: Number(bom.id || 0),
          group_id: Number(targetOption.group_id || 0),
          group_item_id: Number(targetOption.group_item_id || 0),
          sort_order: 100,
        }),
      })
    }
    ok.value = `已移动 ${records.length} 个 BOM`
    selectedBomRowKeys.value = []
    await loadAll()
  })
}

async function deactivateProductionBomRecords(records, successText) {
  await mutate(async () => {
    for (const bom of records) {
      await apiSend(`/api/production-boms/${bom.id}`, {
        method: 'PUT',
        body: {
          name: bom?.name || '',
          status: 'inactive',
        },
      })
    }
    ok.value = successText
    selectedBomRowKeys.value = []
    await loadAll()
    if (currentProductionBomID.value) await selectProductionBomRecordByID(currentProductionBomID.value)
  })
}

async function deactivateSelectedProductionBoms() {
  const records = selectedActiveBomRecordsForDeactivate.value
  if (!records.length) return
  await deactivateProductionBomRecords(records, `已失效 ${records.length} 个生产 BOM`)
}

async function saveProductionBomRecord() {
  const name = String(bomForm.name || '').trim()
  const outputProductID = Number(bomForm.output_product_id || 0)
  if (!name || outputProductID <= 0) return
  const payload = {
    name,
    output_product_id: outputProductID,
    output_qty: Number(bomForm.output_qty || 1),
    output_unit: String(bomForm.output_unit || 'unit').trim() || 'unit',
    status: bomForm.status === 'inactive' ? 'inactive' : 'active',
  }
	await mutate(async () => {
	  if (bomForm.mode === 'edit') {
	    await apiSend(`/api/production-boms/${bomForm.id}`, { method: 'PUT', body: payload })
	    if (canEditCurrentBomItems.value) {
	      await saveProductionBomDraftItems(detailItems.value.map(productionBomDraftItemFromItem), {
	        output_qty: payload.output_qty,
	        output_unit: payload.output_unit,
	      })
	    }
	    ok.value = '已保存生产 BOM'
	  } else if (bomForm.mode === 'copy') {
      const copied = await apiSend(`/api/production-boms/${bomForm.source_id}/copy`, { body: { name: payload.name, output_product_id: payload.output_product_id } })
      ok.value = '已复制生产 BOM'
      pendingProductionBomID.value = Number(copied?.id || 0)
    } else {
      const created = await apiSend('/api/production-boms', { body: { name: payload.name, output_product_id: payload.output_product_id, output_qty: payload.output_qty, output_unit: payload.output_unit } })
      ok.value = '已新建生产 BOM'
      pendingProductionBomID.value = Number(created?.id || 0)
    }
    closeBomDrawer()
    await loadAll()
  })
}

async function deactivateProductionBomRecord(bom) {
  const bomID = Number(bom?.id || 0)
  if (!bomID || bom?.status === 'inactive') return
  await deactivateProductionBomRecords([bom], '已失效生产 BOM')
}

async function createVersion() {
  await copyVersionAsDraft()
}

async function copyVersionAsDraft(version = selectedProductionBomVersion.value) {
  if (!canEditCurrentBomProduct.value) return
  await mutate(async () => {
    const bomID = currentProductionBomID.value
    const sourceVersionID = Number(version?.id || 0)
    if (bomID) {
      const created = await apiSend(`/api/production-boms/${bomID}/versions`, { body: { note: versionNote.value, source_version_id: sourceVersionID } })
      selectedProductionBomVersionID.value = Number(created?.id || 0)
    } else {
      await apiSend('/api/bom/versions', { body: { product_id: selectedProductId.value, note: versionNote.value } })
    }
    versionNote.value = ''
    ok.value = bomID ? '已复制为新版草稿' : '已保存版本'
    if (bomID) await loadProductionBomDetailForVersion(bomID, selectedProductionBomVersionID.value)
  })
}

async function activateVersion(id) {
  if (!canEditCurrentBomProduct.value) return
  await mutate(async () => {
    const bomID = currentProductionBomID.value
    if (currentProductionBomID.value) {
      await apiSend(`/api/production-bom-versions/${id}/publish`, { body: {} })
      ok.value = '已发布版本'
    } else {
      await apiSend(`/api/bom/versions/${id}/activate`, { body: {} })
      ok.value = '已启用版本'
    }
    await loadAll()
    if (bomID) await loadProductionBomDetailForVersion(bomID, id)
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

onMounted(async () => {
  const params = new URL(window.location.href).searchParams
  pendingProductionBomID.value = Number(props.viewParams?.production_bom_id || props.viewParams?.bom_id || params.get('production_bom_id') || params.get('bom_id') || 0)
  await loadAll()
  await restoreBomFormDraft()
})

onBeforeUnmount(saveBomFormDraft)

watch(selectedProductionBomGroupItemID, (groupItemID) => {
  selectedBomRowKeys.value = []
  selectedBomMoveGroupItemID.value = Number(groupItemID || 0) > 0 ? Number(groupItemID || 0) : 0
})

watch([productionBomStatusFilter, productionBomSearchQuery], () => {
  selectedBomRowKeys.value = []
})

function outputProductLabel(row = {}) {
  const productID = Number(row.output_product_id || 0)
  const name = row.output_product_name || productByID(productID)?.name || ''
  const code = row.output_product_code || ''
  if (name && code) return `${name} / ${code}`
  if (name) return name
  return productID > 0 ? `商品 #${productID}` : '-'
}
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.bom-return-banner { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 12px; padding: 10px 12px; border: 1px solid #c7d2fe; border-radius: 8px; background: #eef2ff; color: #1e3a8a; }
.bom-return-banner span { font-size: 13px; }
.bom-return-button { border-color: #1e40af; color: #1e40af; font-weight: 700; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .filters, .inline-form, .summary { display: flex; align-items: end; gap: 10px; flex-wrap: wrap; }
.panel-head { justify-content: space-between; align-items: center; margin-bottom: 12px; }
.panel-title { font-size: 16px; font-weight: 700; margin-bottom: 10px; }
.compact-title { margin-bottom: 4px; }
h2 { margin: 0; font-size: 20px; }
h3 { margin: 2px 0 4px; font-size: 18px; }
.grid { display: grid; grid-template-columns: minmax(360px, 0.9fr) minmax(420px, 1.1fr); gap: 14px; align-items: start; }
label span, .summary span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select, textarea { border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; min-width: 180px; }
input, select { height: 38px; }
textarea { width: 100%; min-height: 148px; resize: vertical; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 13px; line-height: 1.45; }
textarea[readonly] { background: #f8f7f5; color: #555; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.secondary.active { background: #1f1f1f; color: #fff; }
.compact-action { height: 32px; padding: 0 10px; font-size: 12px; }
.danger-outline { border-color: #9d2626; color: #9d2626; }
.danger-text { color: #9d2626; margin-left: 10px; }
.text-button { height: 30px; border: 0; background: transparent; color: #1f4f82; padding: 0; }
.text-button + .text-button { margin-left: 10px; }
.bom-action-row { display: flex; align-items: flex-end; gap: 12px; flex-wrap: wrap; }
.bom-search-field input { min-width: min(340px, 100%); }
.bom-product-filter { min-width: min(280px, 100%); }
.bom-list-head { align-items: flex-start; }
.bom-list-filters { display: flex; align-items: flex-end; gap: 12px; flex-wrap: wrap; margin: 6px 0 12px; padding-top: 10px; border-top: 1px solid #eee8df; }
.bom-batch-deactivate-action { margin-left: auto; }
.bom-list-tabs-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; margin: 8px 0; }
.bom-list-toolbar { display: flex; align-items: flex-end; gap: 10px; flex-wrap: wrap; padding: 10px; margin: 8px 0; border: 1px solid #eee8df; border-radius: 8px; background: #fbfaf8; }
.bom-list-tabs { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.bom-list-panel-scroll { max-height: min(62vh, 720px); overflow: auto; }
.bom-name-button { height: auto; min-height: 30px; text-align: left; font-weight: 700; }
.bom-record-form { align-items: flex-end; }
.bom-group-strip { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; width: 100%; }
.bom-group-strip > span { color: #666; font-size: 12px; font-weight: 700; }
.bom-focus-filter { align-self: stretch; display: inline-flex; align-items: center; gap: 8px; padding: 8px 10px; border: 1px solid #dbeafe; border-radius: 8px; background: #eff6ff; color: #1d4ed8; font-size: 13px; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 640px; border-collapse: collapse; }
.compact table { min-width: 520px; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
.select-col { width: 42px; text-align: center; }
.select-col input { min-width: 0; width: 16px; height: 16px; }
tbody tr.active { background: #f3f7fb; }
.list-panel tbody tr { cursor: pointer; }
.classification-group-row td { background: #f8f7f5; color: #333; border-top: 1px solid #e6e0d8; }
.classification-group-row strong { margin: 0 8px; }
.category-toggle { font-size: 12px; }
.summary { align-items: stretch; margin-bottom: 12px; }
.summary div { min-width: 120px; border: 1px solid #eee8df; border-radius: 6px; padding: 9px; }
.summary strong { font-size: 16px; }
.linked-processes { display: flex; flex-wrap: wrap; gap: 8px; margin: -4px 0 12px; }
.referenced-product-button { cursor: pointer; font: inherit; }
.referenced-product-button:hover, .referenced-product-button:focus-visible { border-color: #1f4f82; color: #1f4f82; background: #eef6ff; outline: none; }
.version-attrs-panel { border: 1px solid #eee8df; border-radius: 8px; padding: 12px; margin-top: 12px; background: #fbfaf8; }
.section-title-row { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; margin-bottom: 12px; }
.detail-subpanel { border: 1px solid #eee8df; border-radius: 8px; padding: 12px; margin: 12px 0; background: #fbfaf8; }
.detail-subpanel h3 { margin: 0 0 4px; font-size: 16px; }
.version-recipe-panel { border-top: 1px solid #e6e0d8; margin-top: 14px; padding-top: 14px; }
.version-ratio-box { min-width: 120px; border: 1px solid #eee8df; border-radius: 6px; padding: 8px 10px; background: #fff; }
.version-ratio-box span { display: block; color: #666; font-size: 12px; margin-bottom: 4px; }
.version-ratio-box strong { font-size: 16px; }
.inline-actions { display: flex; gap: 8px; align-items: flex-end; flex-wrap: wrap; justify-content: flex-end; }
.inline-actions input { min-width: min(280px, 100%); }
.compact-form { align-items: end; }
.compact-action { min-height: 34px; }
.attrs-grid { display: grid; grid-template-columns: repeat(2, minmax(260px, 1fr)); gap: 12px; align-items: start; }
.warning-banner { border: 1px solid #e8c28f; border-radius: 6px; background: #fff8eb; color: #8a4b00; padding: 9px; margin-bottom: 12px; }
.bom-source-banner { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.inline-form { margin: 12px 0; }
.muted { color: #666; text-align: center; }
.muted.left { text-align: left; margin: 0; font-size: 13px; }
.empty { padding: 22px; border: 1px dashed #d8d0c7; border-radius: 8px; }
.warn { color: #a13b00; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
.status-pill { display: inline-flex; align-items: center; min-height: 24px; border: 1px solid #cfd8cf; border-radius: 999px; padding: 2px 8px; color: #27602e; background: #f2fbf2; white-space: nowrap; }
.status-pill.inactive { border-color: #e1b6b6; color: #8a1f1f; background: #fff0f0; }
.status-pill.readonly { border-color: #d6d0c7; color: #5f5a54; background: #f8f7f5; }
.drawer-mask { position: fixed; inset: 0; z-index: 80; background: rgba(20, 20, 20, .28); display: flex; justify-content: flex-end; }
.drawer { width: min(560px, 100vw); height: 100%; background: #fff; box-shadow: -8px 0 28px rgba(0,0,0,.16); padding: 18px; overflow: auto; }
.drawer-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 14px; }
@media (max-width: 1100px) { .grid { grid-template-columns: 1fr; } }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .attrs-grid { grid-template-columns: 1fr; }
  table { min-width: 620px; }
}
</style>
