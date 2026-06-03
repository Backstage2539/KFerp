<template>
  <div class="page">
    <div v-if="bomReturnNavigation" class="bom-return-banner">
      <button class="secondary bom-return-button" type="button" @click="returnToProductConfig">{{ bomReturnLabel }}</button>
      <span>完成 BOM 明细维护后可回到来源操作界面。</span>
    </div>
    <section class="panel">
      <div class="panel-head">
        <h2>生产 BOM（配方库）</h2>
        <div class="panel-actions">
          <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
      <div class="bom-sku-context-panel">
        <div>
          <div class="context-eyebrow">SKU归属</div>
          <h3>{{ bomSkuContextLabel }}</h3>
          <p class="muted left">生产 BOM 是可分组、可复制、可复用的配方档案；商品档案只绑定一个已发布版本。</p>
        </div>
        <div v-if="!isWorkspaceCustomerLocked" class="bom-sku-context-controls">
          <button class="secondary compact-action" type="button" @click="selectedBomCustomerSkuCustomerID = 0" :disabled="!selectedBomCustomerSkuCustomerID">
            公共SKU
          </button>
          <SearchableSelect
            class="bom-customer-select"
            v-model="selectedBomCustomerSkuCustomerID"
            :options="bomSkuCustomers"
            :option-label="customerOptionLabel"
            :option-meta="customerOptionMeta"
            :option-value="optionNumericValue"
            placeholder="选择客户SKU"
            empty-text="暂无自定义SKU客户" />
        </div>
        <p v-else class="muted left context-lock-note">客户账户模式下由顶部当前客户控制。</p>
        <div class="context-stats">
          <span>公共SKU BOM {{ publicBomRows.length }}</span>
          <span>当前SKU BOM {{ bomContextRows.length }}</span>
        </div>
      </div>
      <div class="filters">
        <label>
          <span>商品</span>
          <SearchableSelect
            v-model="selectedProductId"
            :options="bomContextProducts"
            :option-label="optionLabel"
            :option-meta="optionMeta"
            :option-value="optionNumericValue"
            placeholder="选择商品"
            :empty-text="selectedBomCustomerSkuCustomerID ? '没有匹配客户SKU' : '没有匹配公共SKU'"
            @select="selectProduct(optionNumericValue($event))" />
        </label>
        <div v-if="isBomProductFilterActive" class="bom-focus-filter">
          <span>已过滤到当前 SKU BOM</span>
          <button class="text-button" type="button" @click="clearBomProductFilter">显示全部 BOM</button>
        </div>
        <button class="secondary danger-outline" type="button" @click="deleteBom" :disabled="!selectedProductId || loading || !canEditCurrentBomProduct">失效当前 BOM</button>
      </div>
    </section>

    <div class="grid">
      <section class="panel list-panel">
        <div class="panel-head bom-list-head">
          <div>
            <div class="panel-title compact-title">商品 BOM列表</div>
            <p class="muted left">一个 BOM 只归入一个分组；全部分组显示所有 BOM，未分类显示未归组 BOM。</p>
          </div>
          <div class="bom-list-toolbar">
            <button class="primary" type="button" @click="openNewProductionBomRecord">新建生产 BOM</button>
            <label>
              <span>状态</span>
              <select v-model="productionBomStatusFilter">
                <option value="active">启用</option>
                <option value="inactive">已失效</option>
                <option value="all">全部</option>
              </select>
            </label>
            <label>
              <span>搜索 BOM</span>
              <input v-model.trim="productionBomSearchQuery" placeholder="按 BOM 名称、编号或商品搜索" />
            </label>
          </div>
        </div>
        <div class="bom-list-tabs">
          <button class="secondary compact-action" type="button" @click="openGroupDrawer">增加分组</button>
          <button
            :class="['secondary', 'compact-action', { active: selectedProductionBomGroupID === 0 }]"
            type="button"
            @click="selectProductionBomGroup(0)">
            全部分组
          </button>
          <button
            :class="['secondary', 'compact-action', { active: selectedProductionBomGroupID === -1 }]"
            type="button"
            @click="selectProductionBomGroup(-1)">
            未分类
          </button>
          <button
            v-for="group in productionBomGroups"
            :key="group.id"
            :class="['secondary', 'compact-action', { active: selectedProductionBomGroupID === Number(group.id || 0) }]"
            type="button"
            @click="selectProductionBomGroup(Number(group.id || 0))">
            {{ group.name }}
          </button>
        </div>
        <div class="bom-move-card">
          <div>
            <strong>移动到分组</strong>
            <p class="muted left">勾选商品 BOM 后移动分组；移动到其他分组会直接覆盖旧分组。</p>
          </div>
          <label>
            <span>目标分组</span>
            <select v-model.number="selectedBomMoveGroupID">
              <option :value="0">未分类</option>
              <option v-for="group in productionBomGroups" :key="group.id" :value="Number(group.id || 0)">{{ group.name }}</option>
            </select>
          </label>
          <button class="secondary" type="button" :disabled="!canMoveSelectedBoms || loading" @click="moveSelectedProductBomsToGroup">
            移动到分组
          </button>
          <span class="muted left">已选 {{ selectedBomRecordsForMove.length }} 个可移动 BOM</span>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th class="select-col"><input type="checkbox" :checked="isAllVisibleBomsSelected" :indeterminate.prop="isSomeVisibleBomsSelected" @change="toggleAllVisibleBoms" /></th>
                <th>商品</th>
                <th>生产 BOM</th>
                <th>分组</th>
                <th>状态</th>
                <th>物料数</th>
                <th>更新时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in bomContextRows"
                :key="row.product_id"
                :class="{ active: row.product_id === selectedProductId }"
                @click="selectProduct(row.product_id)">
                <td class="select-col">
                  <input
                    v-model="selectedBomRowKeys"
                    type="checkbox"
                    :value="bomRowKey(row)"
                    :disabled="!Number(row.production_bom_id || 0)"
                    @click.stop />
                </td>
                <td>{{ row.product }}</td>
                <td>
                  <button v-if="Number(row.production_bom_id || 0)" class="text-button bom-name-button" type="button" @click.stop="openEditProductionBomRecord(bomRecordFromRow(row))">
                    {{ productionBomLabel(row) }}
                  </button>
                  <div v-else>{{ productionBomLabel(row) }}</div>
                  <small v-if="productionBomVersionWarning(row)" class="bom-version-warning" data-warning-prefix="当前引用">{{ productionBomVersionWarning(row) }}</small>
                </td>
                <td>{{ bomGroupLabel(row) }}</td>
                <td><span :class="['status-pill', row.status === 'inactive' ? 'inactive' : '']">{{ bomStatusLabel(row.status) }}</span></td>
                <td>{{ row.item_count }}</td>
                <td>{{ row.updated_at }}</td>
                <td>
                  <button class="text-button" type="button" :disabled="!Number(row.production_bom_id || 0)" @click.stop="copyProductionBomRecord(bomRecordFromRow(row))">复制</button>
                  <button class="text-button danger-text" type="button" :disabled="!Number(row.production_bom_id || 0) || row.status === 'inactive'" @click.stop="deactivateProductionBomRecord(bomRecordFromRow(row))">失效</button>
                </td>
              </tr>
              <tr v-if="!bomContextRows.length">
                <td colspan="8" class="muted">{{ selectedBomCustomerSkuCustomerID ? '暂无客户SKU BOM' : '暂无公共SKU BOM' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="panel detail-panel">
        <div class="panel-title">配方明细</div>
        <div v-if="detail" class="summary">
          <div><span>商品</span><strong>{{ detail.product_name }}</strong></div>
          <div><span>生产 BOM</span><strong>{{ currentProductionBomLabel }}</strong></div>
          <div v-if="currentProductionBomWarning"><span>版本提示</span><strong class="warn">{{ currentProductionBomWarning }}</strong></div>
          <div><span>工艺参数</span><strong>{{ detail.roast_level || '-' }}</strong></div>
          <div><span>状态</span><strong :class="{ warn: detail.status === 'inactive' }">{{ bomStatusLabel(detail.status) }}</strong></div>
          <div><span>合计比例</span><strong :class="{ warn: detail.total_ratio > 100 }">{{ ratio(detail.total_ratio) }}</strong></div>
          <div><span>关联工艺</span><strong>{{ linkedProcessTemplates.length ? `${linkedProcessTemplates.length} 个模板` : '-' }}</strong></div>
        </div>
        <div v-if="detail && linkedProcessTemplates.length" class="linked-processes">
          <span v-for="template in linkedProcessTemplates" :key="template.id" :class="['status-pill', template.status === 'inactive' ? 'inactive' : '']">
            {{ template.name }} · {{ processStatusLabel(template.status) }}
          </span>
        </div>
        <div v-if="detail?.status === 'inactive'" class="warning-banner">当前 BOM 已失效，历史配方明细会保留；重新保存或启用版本后可恢复为有效 BOM。</div>
        <div v-if="!detail" class="muted empty">请选择商品</div>

        <form class="inline-form" @submit.prevent="saveItem">
          <label>
            <span>组件类型</span>
            <select v-model="itemForm.component_type" :disabled="!detail || !canEditCurrentBomProduct" @change="syncComponentTypeDefaults">
              <option value="material">物料</option>
              <option value="finished_product">成品</option>
            </select>
          </label>
          <label>
            <span>{{ itemForm.component_type === 'finished_product' ? '熟豆成品' : '物料' }}</span>
            <SearchableSelect
              v-if="itemForm.component_type === 'finished_product'"
              v-model="itemForm.component_product_id"
              :options="roastedBeanProducts"
              :option-label="optionLabel"
              :option-meta="optionMeta"
              :option-value="optionNumericValue"
              placeholder="选择熟豆成品"
              empty-text="没有可用熟豆成品"
              :disabled="!detail || !canEditCurrentBomProduct" />
            <SearchableSelect
              v-else
              v-model="itemForm.material_id"
              :options="materials"
              :option-label="optionLabel"
              :option-value="optionNumericValue"
              placeholder="选择物料"
              empty-text="没有匹配物料"
              :disabled="!detail || !canEditCurrentBomProduct" />
          </label>
          <label>
            <span>消耗单位</span>
            <select v-model="itemForm.consume_unit" :disabled="!detail || !canEditCurrentBomProduct || itemForm.component_type === 'finished_product'">
              <option v-for="unit in currentConsumeUnitOptions" :key="unit.value" :value="unit.value">{{ unit.label }}</option>
            </select>
          </label>
          <label v-if="itemForm.consume_unit === 'ratio_pct'">
            <span>比例 %</span>
            <input v-model.number="itemForm.ratio_pct" type="number" min="0.01" max="100" step="0.01" :disabled="!detail || !canEditCurrentBomProduct" />
          </label>
          <label v-else>
            <span>用量</span>
            <input v-model.number="itemForm.qty_per_unit" type="number" min="0.001" step="0.001" :disabled="!detail || !canEditCurrentBomProduct" />
          </label>
          <button class="primary" type="submit" :disabled="!detail || loading || !canEditCurrentBomProduct">保存组件</button>
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
                <td><button class="text-button" type="button" :disabled="!canEditCurrentBomProduct" @click="deleteItem(item.id)">删除</button></td>
              </tr>
              <tr v-if="!detailItems.length">
                <td colspan="4" class="muted">暂无组件</td>
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
          <input v-model.trim="versionNote" placeholder="例如 2026 春季豆单" :disabled="!selectedProductId || !canEditCurrentBomProduct" />
        </label>
        <button class="primary" type="button" @click="createVersion" :disabled="!selectedProductId || loading || !canEditCurrentBomProduct || !currentProductionBomID">复制为新版草稿</button>
      </div>
      <div class="table-wrap compact">
        <table>
          <thead>
            <tr>
              <th>版本</th>
              <th>状态</th>
              <th>物料数</th>
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
              @click="selectProductionBomVersion(version)">
              <td>{{ version.version_no }}</td>
              <td>{{ productionBomVersionStatusLabel(version.status) }}</td>
              <td>{{ version.item_count }}</td>
              <td>{{ version.note }}</td>
              <td>{{ version.published_at || version.created_at }}</td>
              <td>
                <button
                  v-if="version.status === 'draft'"
                  class="text-button"
                  type="button"
                  @click.stop="activateVersion(version.id)"
                  :disabled="!canEditCurrentBomProduct">
                  发布
                </button>
                <span v-else class="muted left">只读</span>
              </td>
            </tr>
            <tr v-if="!versions.length">
              <td colspan="6" class="muted">暂无版本</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section v-if="!isWorkspaceCustomerLocked" class="panel">
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

    <div v-if="bomDrawerOpen" class="drawer-mask" @click.self="closeBomDrawer">
      <aside class="drawer">
        <div class="drawer-head">
          <div>
            <h3>{{ bomFormTitle }}</h3>
            <p class="muted left">只维护配方库档案。版本和配方明细在右侧“BOM版本”和“配方明细”中处理。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeBomDrawer">关闭</button>
        </div>
        <form class="inline-form bom-record-form" @submit.prevent="saveProductionBomRecord">
          <label>
            <span>BOM名称</span>
            <input v-model.trim="bomForm.name" placeholder="例如 精品拼配" />
          </label>
          <label>
            <span>分组</span>
            <select v-model.number="bomForm.group_id">
              <option :value="0">未分类</option>
              <option v-for="group in productionBomGroups" :key="group.id" :value="Number(group.id || 0)">{{ group.name }}</option>
            </select>
          </label>
          <label v-if="bomForm.mode === 'edit'">
            <span>状态</span>
            <select v-model="bomForm.status">
              <option value="active">启用</option>
              <option value="inactive">已失效</option>
            </select>
          </label>
          <button class="primary" type="submit" :disabled="loading || !bomForm.name">{{ bomForm.mode === 'copy' ? '复制 BOM' : '保存 BOM' }}</button>
        </form>
      </aside>
    </div>

    <div v-if="groupDrawerOpen" class="drawer-mask" @click.self="closeGroupDrawer">
      <aside class="drawer">
        <div class="drawer-head">
          <div>
            <h3>管理分组</h3>
            <p class="muted left">分组只用于配方库归类。删除分组时，组内 BOM 会回到未分类。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="closeGroupDrawer">关闭</button>
        </div>
        <form class="inline-form" @submit.prevent="saveProductionBomGroup">
          <label>
            <span>分组名称</span>
            <input v-model.trim="groupForm.name" placeholder="例如 常用配方" />
          </label>
          <label>
            <span>排序</span>
            <input v-model.number="groupForm.sort_order" type="number" min="0" step="1" />
          </label>
          <button class="primary" type="submit" :disabled="loading || !groupForm.name">{{ groupForm.id ? '保存分组' : '新增分组' }}</button>
          <button class="secondary" type="button" @click="resetGroupForm">清空</button>
        </form>
        <div class="table-wrap compact">
          <table>
            <thead>
              <tr>
                <th>分组</th>
                <th>排序</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="group in managedProductionBomGroups" :key="group.id">
                <td>{{ group.name }}</td>
                <td>{{ group.sort_order }}</td>
                <td>
                  <button class="text-button" type="button" @click="editProductionBomGroup(group)">编辑</button>
                  <button class="text-button" type="button" @click="moveProductionBomGroup(group, -1)">上移</button>
                  <button class="text-button" type="button" @click="moveProductionBomGroup(group, 1)">下移</button>
                  <button
                    class="text-button danger-text"
                    type="button"
                    @click="deleteProductionBomGroup(group)">
                    DELETE
                  </button>
                </td>
              </tr>
              <tr v-if="!managedProductionBomGroups.length">
                <td colspan="3" class="muted">暂无分组</td>
              </tr>
            </tbody>
          </table>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import SearchableSelect from '../components/SearchableSelect.vue'
import { bomContextCustomerIDs, filterBomContextProducts, filterBomRowsByProductFocus, filterProductionBomCatalog, productionBomLabel, productionBomVersionWarning } from '../lib/bom'
import { componentTypeLabel, isDripProduct } from '../lib/drip-product'
import { FORM_DRAFT_SCOPES, readFormDraft, saveFormDraft } from '../lib/form-draft-cache'
import { replaceHistoryURL } from '../lib/url-state'
import { CUSTOMER_WORKSPACE_MODE, workspaceCustomerChangeEvent } from '../lib/workspace-mode'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
  workspaceMode: { type: String, default: '' },
  customerContextId: { type: [Number, String], default: 0 },
  customerContextLabel: { type: String, default: '' },
})
const BOM_FORM_DRAFT_SCOPE = FORM_DRAFT_SCOPES.bom

const rows = ref([])
const products = ref([])
const customers = ref([])
const materials = ref([])
const mappings = ref([])
const versions = ref([])
const processTemplates = ref([])
const productionBomGroups = ref([])
const productionBomDetail = ref(null)
const detail = ref(null)
const selectedProductId = ref(0)
const selectedBomCustomerSkuCustomerID = ref(0)
const selectedProductionBomGroupID = ref(0)
const selectedProductionBomVersionID = ref(0)
const selectedBomMoveGroupID = ref(0)
const selectedBomRowKeys = ref([])
const pendingUrlProductId = ref(0)
const bomFilterProductId = ref(0)
const bomReturnNavigation = computed(() => props.viewParams?.return_navigation || null)
const bomReturnProductID = computed(() => Number(bomReturnNavigation.value?.params?.open_product_config_id || 0))
const bomReturnLabel = computed(() => String(bomReturnNavigation.value?.label || '返回商品档案配置'))
const groupDrawerOpen = ref(false)
const bomDrawerOpen = ref(false)
const productionBomStatusFilter = ref('active')
const productionBomSearchQuery = ref('')
const managedProductionBomGroups = ref([])
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
const mappingForm = reactive({ spec_g: 227, material_id: 0 })
const groupForm = reactive({ id: 0, name: '', sort_order: 100 })
const bomForm = reactive({ id: 0, source_id: 0, mode: 'create', name: '', group_id: 0, status: 'active' })
const versionNote = ref('')

const detailItems = computed(() => detail.value?.items || [])
const bomContextCustomerID = computed(() => Number(selectedBomCustomerSkuCustomerID.value || 0))
const isWorkspaceCustomerLocked = computed(() => props.workspaceMode === CUSTOMER_WORKSPACE_MODE && Number(props.customerContextId || 0) > 0)
const bomSkuContextLabel = computed(() => {
  const customerID = bomContextCustomerID.value
  if (!customerID) return '公共SKU'
  return `${customerName(customerID) || `客户 #${customerID}`} SKU`
})
const publicBomRows = computed(() => filterBomContextProducts(rows.value, 0))
const customBomProductCustomerIDs = computed(() => bomContextCustomerIDs(products.value, rows.value))
const bomSkuCustomers = computed(() => customers.value
  .filter((customer) => customBomProductCustomerIDs.value.has(Number(customer.id || 0)))
  .sort((a, b) => customerOptionLabel(a).localeCompare(customerOptionLabel(b))))
const bomContextProducts = computed(() => filterBomContextProducts(products.value, bomContextCustomerID.value))
const allBomContextRows = computed(() => filterBomContextProducts(rows.value, bomContextCustomerID.value))
const groupFilteredBomContextRows = computed(() => {
  return filterProductionBomCatalog(allBomContextRows.value, {
    status: productionBomStatusFilter.value,
    query: productionBomSearchQuery.value,
    groupID: Number(selectedProductionBomGroupID.value || 0),
  })
})
const bomContextRows = computed(() => filterBomRowsByProductFocus(groupFilteredBomContextRows.value, bomFilterProductId.value))
const isBomProductFilterActive = computed(() => Number(bomFilterProductId.value || 0) > 0)
const linkedProcessTemplates = computed(() => processTemplates.value.filter((template) => Number(template.product_id || 0) === Number(selectedProductId.value || 0)))
const selectedProduct = computed(() => productByID(selectedProductId.value))
const selectedBomRow = computed(() => rows.value.find((row) => Number(row.product_id || 0) === Number(selectedProductId.value || 0)) || null)
const currentProductionBomLabel = computed(() => productionBomLabel(detail.value || selectedBomRow.value || selectedProduct.value || {}))
const currentProductionBomWarning = computed(() => productionBomVersionWarning(detail.value || selectedBomRow.value || selectedProduct.value || {}))
const currentProductionBomID = computed(() => Number(detail.value?.production_bom_id || selectedBomRow.value?.production_bom_id || 0))
const bomFormTitle = computed(() => ({
  create: '新建生产 BOM',
  edit: '编辑 BOM',
  copy: '复制 BOM',
})[bomForm.mode] || '生产 BOM')
const selectedProductionBomVersion = computed(() => versions.value.find((version) => Number(version.id || 0) === Number(selectedProductionBomVersionID.value || 0)) || null)
const canEditCurrentBomProduct = computed(() => {
  if (!selectedProductId.value) return true
  if (detail.value?.can_edit_bom === false) return false
  if (selectedBomRow.value?.can_edit_bom === false) return false
  if (!bomContextCustomerID.value) return true
  return Number(selectedProduct.value?.customer_id || 0) === bomContextCustomerID.value
})
const roastedBeanProducts = computed(() => products.value.filter((product) => {
  if (Number(product.id || 0) === Number(selectedProductId.value || 0)) return false
  return (product.product_kind || 'roasted_bean') === 'roasted_bean'
}))
const materialConsumeUnitOptions = [
  { value: 'ratio_pct', label: '比例 %' },
  { value: 'g_per_bag', label: '克/袋' },
  { value: 'unit_per_bag', label: '个/袋' },
  { value: 'unit_per_box', label: '个/盒' },
]
const finishedProductConsumeUnitOptions = [
  { value: 'g_per_bag', label: '克/袋' },
]
const currentConsumeUnitOptions = computed(() => itemForm.component_type === 'finished_product'
  ? finishedProductConsumeUnitOptions
  : materialConsumeUnitOptions)
const visibleMovableBomRows = computed(() => bomContextRows.value.filter((row) => Number(row.production_bom_id || 0) > 0))
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
  return allBomContextRows.value.filter((row) => selected.has(bomRowKey(row)) && Number(row.production_bom_id || 0) > 0)
})
const selectedBomRecordsForMove = computed(() => {
  const byBomID = new Map()
  for (const row of selectedBomRows.value) {
    const record = bomRecordFromRow(row)
    if (record.id > 0 && !byBomID.has(record.id)) byBomID.set(record.id, record)
  }
  return [...byBomID.values()]
})
const canMoveSelectedBoms = computed(() => {
  const targetGroupID = Number(selectedBomMoveGroupID.value || 0)
  return selectedBomRecordsForMove.value.some((bom) => Number(bom.group_id || 0) !== targetGroupID)
})

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
  return String(row?.product_id || row?.id || '')
}

function optionLabel(option) {
  return option?.name || ''
}

function optionMeta(option) {
  const parts = []
  if (Number(option?.customer_id || 0) > 0) parts.push(customerName(option.customer_id) || `客户 #${option.customer_id}`)
  else parts.push('公共SKU')
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
		selectedBomCustomerSkuCustomerID: selectedBomCustomerSkuCustomerID.value,
		selectedProductId: selectedProductId.value,
		itemForm: { ...itemForm },
		mappingForm: { ...mappingForm },
		versionNote: versionNote.value,
	})
}

async function restoreBomFormDraft() {
  const params = new URL(window.location.href).searchParams
  if (Number(params.get('product_id') || 0) > 0) return
  const draft = readFormDraft(bomFormDraftKey())
  if (!draft) return
  selectedBomCustomerSkuCustomerID.value = Number(draft.selectedBomCustomerSkuCustomerID || 0)
  selectedProductId.value = Number(draft.selectedProductId || 0)
  Object.assign(itemForm, {
    component_type: 'material',
    material_id: 0,
    component_product_id: 0,
    component_spec_g: 0,
    consume_unit: 'ratio_pct',
    qty_per_unit: '',
    ratio_pct: '',
	}, draft.itemForm || {})
	Object.assign(mappingForm, { spec_g: 227, material_id: 0 }, draft.mappingForm || {})
	versionNote.value = draft.versionNote || ''
	if (syncSelectedProductToBomContext()) {
    await loadDetail(selectedProductId.value)
  }
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

function normalizeBomProduct(product) {
  return {
    ...product,
    id: Number(product.id || 0),
    customer_id: Number(product.customer_id || 0),
  }
}

function normalizeBomRow(row) {
  return {
    ...row,
    product_id: Number(row.product_id || 0),
    customer_id: Number(row.customer_id || 0),
  }
}

function bomContextProductFilter(product) {
  return filterBomContextProducts([product], bomContextCustomerID.value).length > 0
}

function productByID(productId) {
  const id = Number(productId || 0)
  return products.value.find((product) => Number(product.id || 0) === id) || null
}

function consumeUnitLabel(unit) {
  return materialConsumeUnitOptions.find((option) => option.value === unit)?.label || unit || '-'
}

function componentItemName(item) {
  if (item?.component_type === 'finished_product') return item.component_product_name || `成品 #${item.component_product_id}`
  return item?.material_name || `物料 #${item?.material_id || 0}`
}

function itemQuantityDisplay(item) {
  if ((item?.consume_unit || 'ratio_pct') === 'ratio_pct') return ratio(item.ratio_pct)
  return `${qty(item.qty_per_unit)} ${consumeUnitLabel(item.consume_unit)}`
}

function productionBomVersionStatusLabel(status) {
  if (status === 'draft') return '草稿'
  if (status === 'published' || status === 'active') return '已发布'
  if (status === 'archived') return '已归档'
  return status || '-'
}

function selectProductionBomGroup(groupID) {
  selectedProductionBomGroupID.value = Number(groupID || 0)
  const groupOnlyRows = filterProductionBomCatalog(allBomContextRows.value, {
    status: 'all',
    query: '',
    groupID: Number(selectedProductionBomGroupID.value || 0),
  })
  const visibleBomIDs = new Set(groupOnlyRows.map((bom) => Number(bom.id || 0)))
  const selectedBomID = Number(selectedBomRow.value?.production_bom_id || detail.value?.production_bom_id || 0)
  if (selectedBomID && !visibleBomIDs.has(selectedBomID)) {
    clearSelectedProduct()
  }
}

function bomGroupLabel(row) {
  return String(row?.production_bom_group_name || row?.group_name || '').trim() || '未分类'
}

function bomRecordFromRow(row = {}) {
  return {
    id: Number(row.production_bom_id || row.id || 0),
    code: row.production_bom_code || row.code || '',
    name: row.production_bom_name || row.name || row.product || '',
    group_id: Number(row.production_bom_group_id ?? row.group_id ?? 0),
    group_name: row.production_bom_group_name || row.group_name || '',
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

function selectProductionBomVersion(version) {
  selectedProductionBomVersionID.value = Number(version?.id || 0)
}

function syncSelectedProductionBomVersion() {
  if (!versions.value.length) {
    selectedProductionBomVersionID.value = 0
    return
  }
  const existing = versions.value.find((version) => Number(version.id || 0) === Number(selectedProductionBomVersionID.value || 0))
  const selected = existing || versions.value.find((version) => version.status === 'draft') || versions.value.find((version) => version.is_latest) || versions.value[0]
  selectProductionBomVersion(selected)
}

function resetGroupForm() {
  groupForm.id = 0
  groupForm.name = ''
  groupForm.sort_order = 100
}

function resetBomForm() {
  bomForm.id = 0
  bomForm.source_id = 0
  bomForm.mode = 'create'
  bomForm.name = ''
  bomForm.group_id = Number(selectedProductionBomGroupID.value || 0) > 0 ? Number(selectedProductionBomGroupID.value || 0) : 0
  bomForm.status = 'active'
}

function openNewProductionBomRecord() {
  resetBomForm()
  bomForm.mode = 'create'
  bomDrawerOpen.value = true
}

function openEditProductionBomRecord(bom) {
  resetBomForm()
  bomForm.mode = 'edit'
  bomForm.id = Number(bom?.id || 0)
  bomForm.name = bom?.name || ''
  bomForm.group_id = Number(bom?.group_id || 0)
  bomForm.status = bom?.status === 'inactive' ? 'inactive' : 'active'
  bomDrawerOpen.value = true
}

function copyProductionBomRecord(bom) {
  resetBomForm()
  bomForm.mode = 'copy'
  bomForm.source_id = Number(bom?.id || 0)
  bomForm.name = `${bom?.name || '生产 BOM'} 副本`
  bomForm.group_id = Number(bom?.group_id || 0)
  bomForm.status = 'active'
  bomDrawerOpen.value = true
}

function closeBomDrawer() {
  bomDrawerOpen.value = false
  resetBomForm()
}

function editProductionBomGroup(group) {
  groupForm.id = Number(group?.id || 0)
  groupForm.name = group?.name || ''
  groupForm.sort_order = Number(group?.sort_order || 0)
}

function syncComponentTypeDefaults() {
  if (itemForm.component_type === 'finished_product') {
    itemForm.material_id = 0
    itemForm.consume_unit = 'g_per_bag'
    itemForm.ratio_pct = ''
    if (!Number(itemForm.qty_per_unit || 0) && isDripProduct(selectedProduct.value)) {
      itemForm.qty_per_unit = Number(selectedProduct.value.drip_bag_grams || 0) || ''
    }
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

function syncBomContextFromUrlProduct() {
  const productID = Number(pendingUrlProductId.value || 0)
  if (!productID) return
  const product = productByID(productID)
  pendingUrlProductId.value = 0
  if (!product) return
  selectedBomCustomerSkuCustomerID.value = Number(product.customer_id || 0)
}

function syncSelectedBomCustomerSkuCustomer() {
  if (!selectedBomCustomerSkuCustomerID.value) return
  const selectedCustomerID = Number(selectedBomCustomerSkuCustomerID.value)
  const existsInCustomerMaster = customers.value.some((customer) => Number(customer.id || 0) === selectedCustomerID)
  if (!existsInCustomerMaster && !customBomProductCustomerIDs.value.has(selectedCustomerID)) {
    selectedBomCustomerSkuCustomerID.value = 0
  }
}

function applyWorkspaceCustomerContext() {
  const customerID = Number(props.customerContextId || 0)
  if (customerID > 0 && Number(selectedBomCustomerSkuCustomerID.value || 0) !== customerID) {
    selectedBomCustomerSkuCustomerID.value = customerID
  }
}

function notifyWorkspaceCustomerChanged(customerID) {
  if (props.workspaceMode !== CUSTOMER_WORKSPACE_MODE || Number(customerID || 0) <= 0) return
  if (Number(customerID || 0) === Number(props.customerContextId || 0)) return
  window.dispatchEvent(workspaceCustomerChangeEvent(customerID))
}

function clearSelectedProduct() {
  selectedProductId.value = 0
	bomFilterProductId.value = 0
	detail.value = null
  productionBomDetail.value = null
	versions.value = []
  selectedProductionBomVersionID.value = 0
	updateUrl()
}

function syncSelectedProductToBomContext() {
  if (!selectedProductId.value) {
    detail.value = null
    productionBomDetail.value = null
    versions.value = []
    selectedProductionBomVersionID.value = 0
    updateUrl()
    return false
  }
  const product = productByID(selectedProductId.value)
  if (!product || !bomContextProductFilter(product)) {
    clearSelectedProduct()
    return false
  }
  return true
}

function updateUrl() {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'bom')
  if (selectedProductId.value) url.searchParams.set('product_id', String(selectedProductId.value))
  else url.searchParams.delete('product_id')
  if (bomFilterProductId.value && Number(bomFilterProductId.value) === Number(selectedProductId.value)) {
    url.searchParams.set('bom_filter_product_id', String(bomFilterProductId.value))
  } else {
    url.searchParams.delete('bom_filter_product_id')
  }
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

async function loadAll() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const [listData, productData, materialData, mappingData, customerData, processData, productionGroupData] = await Promise.all([
      apiGet('/api/bom/list'),
      apiGet('/api/bom/products'),
      apiGet('/api/bom/materials'),
      apiGet('/api/bom/bag-spec-mappings'),
      apiGet('/api/customers?limit=200'),
      apiGet('/api/process-templates'),
      apiGet('/api/production-bom-groups'),
    ])
    const customerID = Number(props.customerContextId || 0)
    const isCustomerLocked = isWorkspaceCustomerLocked.value && customerID > 0

    rows.value = (listData || []).map(normalizeBomRow)
    // 客户账户模式下只显示该客户的 BOM 行
    if (isCustomerLocked) {
      rows.value = rows.value.filter((row) => Number(row?.customer_id || 0) === customerID)
    }
    products.value = (productData || []).map(normalizeBomProduct)
    materials.value = materialData || []
    mappings.value = mappingData || []
    customers.value = (customerData.rows || []).filter((row) => row.active !== false)
    processTemplates.value = processData.rows || []
    productionBomGroups.value = productionGroupData || []
    syncBomContextFromUrlProduct()
    applyWorkspaceCustomerContext()
    syncSelectedBomCustomerSkuCustomer()
    if (syncSelectedProductToBomContext()) await loadDetail(selectedProductId.value)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadDetail(productId) {
	if (!productId) {
		detail.value = null
    productionBomDetail.value = null
    versions.value = []
    selectedProductionBomVersionID.value = 0
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
    productionBomDetail.value = null
    selectedProductionBomVersionID.value = 0
    return
  }
  const bomID = currentProductionBomID.value
  if (bomID) {
    productionBomDetail.value = await apiGet(`/api/production-boms/${bomID}`)
    versions.value = productionBomDetail.value?.versions || []
    syncSelectedProductionBomVersion()
    return
  }
  productionBomDetail.value = null
  const data = await apiGet(`/api/bom/versions?product_id=${productId}`)
  versions.value = data.rows || []
  syncSelectedProductionBomVersion()
}

async function selectProduct(productId) {
  const nextProductId = Number(productId || 0)
  const nextProduct = productByID(nextProductId)
  if (nextProductId && (!nextProduct || !bomContextProductFilter(nextProduct))) {
    clearSelectedProduct()
    return
  }
  if (Number(bomFilterProductId.value || 0) > 0 && nextProductId !== Number(bomFilterProductId.value || 0)) {
    bomFilterProductId.value = 0
  }
  selectedProductId.value = nextProductId
  error.value = ''
  ok.value = ''
  try {
    await loadDetail(selectedProductId.value)
  } catch (err) {
    error.value = err.message || '加载失败'
  }
}

function clearBomProductFilter() {
  bomFilterProductId.value = 0
  updateUrl()
}

function bomStatusLabel(status) {
  if (status === 'inactive') return '已失效'
  if (status === 'missing') return '未维护'
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

async function deleteBom() {
  if (!selectedProductId.value) return
  if (!canEditCurrentBomProduct.value) return
  const okToDeactivate = window.confirm('确认失效当前 BOM？配方明细会保留，后续依赖该 BOM 的策略会提示 BOM 已失效。')
  if (!okToDeactivate) return
  await mutate(async () => {
    await apiSend(`/api/bom/${selectedProductId.value}`, { method: 'DELETE' })
    resetItemForm()
    ok.value = '当前 BOM 已失效'
    await loadAll()
  })
}

async function saveItem() {
  if (!canEditCurrentBomProduct.value) return
  await mutate(async () => {
    await apiSend('/api/bom/item/save', {
      body: {
        product_id: selectedProductId.value,
        component_type: itemForm.component_type,
        material_id: Number(itemForm.material_id || 0),
        component_product_id: Number(itemForm.component_product_id || 0),
        component_spec_g: Number(itemForm.component_spec_g || 0),
        consume_unit: itemForm.consume_unit,
        qty_per_unit: Number(itemForm.qty_per_unit || 0),
        ratio_pct: Number(itemForm.ratio_pct || 0),
      },
    })
    resetItemForm()
    ok.value = '已保存'
    await loadAll()
  })
}

async function deleteItem(id) {
  if (!canEditCurrentBomProduct.value) return
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

async function loadProductionBomGroupsForManagement() {
  managedProductionBomGroups.value = await apiGet('/api/production-bom-groups') || []
}

async function openGroupDrawer() {
  groupDrawerOpen.value = true
  resetGroupForm()
  await mutate(async () => {
    await loadProductionBomGroupsForManagement()
  })
}

function closeGroupDrawer() {
  groupDrawerOpen.value = false
  resetGroupForm()
}

async function saveProductionBomGroup() {
  const payload = { name: groupForm.name, sort_order: Number(groupForm.sort_order || 0) }
  await mutate(async () => {
    if (groupForm.id) {
      await apiSend(`/api/production-bom-groups/${groupForm.id}`, { method: 'PUT', body: payload })
      ok.value = '已保存分组'
    } else {
      await apiSend('/api/production-bom-groups', { body: payload })
      ok.value = '已新增分组'
    }
    resetGroupForm()
    await Promise.all([loadProductionBomGroupsForManagement(), loadAll()])
  })
}

async function deleteProductionBomGroup(group) {
  const groupID = Number(group?.id || 0)
  if (!groupID) return
  const okToDelete = window.confirm(`确认删除分组「${group?.name || groupID}」？分组下 BOM 会移到未分类，配方和商品绑定不受影响。`)
  if (!okToDelete) return
  await mutate(async () => {
    await apiSend(`/api/production-bom-groups/${group.id}`, { method: 'DELETE' })
    ok.value = '已删除分组'
    if (selectedProductionBomGroupID.value === groupID) selectedProductionBomGroupID.value = 0
    await Promise.all([loadProductionBomGroupsForManagement(), loadAll()])
  })
}

async function moveSelectedProductBomsToGroup() {
  const targetGroupID = Number(selectedBomMoveGroupID.value || 0)
  const records = selectedBomRecordsForMove.value.filter((bom) => Number(bom.group_id || 0) !== targetGroupID)
  if (!records.length) return
  await mutate(async () => {
    for (const bom of records) {
      await apiSend(`/api/production-boms/${bom.id}`, {
        method: 'PUT',
        body: {
          name: bom.name,
          group_id: targetGroupID,
          status: bom.status === 'inactive' ? 'inactive' : 'active',
        },
      })
    }
    ok.value = `已移动 ${records.length} 个 BOM`
    selectedBomRowKeys.value = []
    await loadAll()
  })
}

async function moveProductionBomGroup(group, direction) {
  const groupID = Number(group?.id || 0)
  if (!groupID) return
  await mutate(async () => {
    await apiSend(`/api/production-bom-groups/${groupID}/move`, {
      body: { sort_order: Math.max(0, Number(group?.sort_order || 0) + Number(direction || 0)) },
    })
    ok.value = '已调整分组顺序'
    await Promise.all([loadProductionBomGroupsForManagement(), loadAll()])
  })
}

async function saveProductionBomRecord() {
  const name = String(bomForm.name || '').trim()
  if (!name) return
  const payload = {
    name,
    group_id: Number(bomForm.group_id || 0),
    status: bomForm.status === 'inactive' ? 'inactive' : 'active',
  }
  await mutate(async () => {
    if (bomForm.mode === 'edit') {
      await apiSend(`/api/production-boms/${bomForm.id}`, { method: 'PUT', body: payload })
      ok.value = '已保存生产 BOM'
    } else if (bomForm.mode === 'copy') {
      await apiSend(`/api/production-boms/${bomForm.source_id}/copy`, { body: { name: payload.name, group_id: payload.group_id } })
      ok.value = '已复制生产 BOM'
    } else {
      await apiSend('/api/production-boms', { body: { name: payload.name, group_id: payload.group_id } })
      ok.value = '已新建生产 BOM'
    }
    closeBomDrawer()
    await loadAll()
  })
}

async function deactivateProductionBomRecord(bom) {
  const bomID = Number(bom?.id || 0)
  if (!bomID || bom?.status === 'inactive') return
  const okToDeactivate = window.confirm(`确认失效生产 BOM「${bom?.name || bomID}」？已启用商品引用时，后端会拒绝并提示。`)
  if (!okToDeactivate) return
  await mutate(async () => {
    await apiSend(`/api/production-boms/${bomID}`, {
      method: 'PUT',
      body: {
        name: bom?.name || '',
        group_id: Number(bom?.group_id || 0),
        status: 'inactive',
      },
    })
    ok.value = '已失效生产 BOM'
    await loadAll()
  })
}

async function createVersion() {
  if (!canEditCurrentBomProduct.value) return
  await mutate(async () => {
    const bomID = currentProductionBomID.value
    if (bomID) {
      await apiSend(`/api/production-boms/${bomID}/versions`, { body: { note: versionNote.value } })
    } else {
      await apiSend('/api/bom/versions', { body: { product_id: selectedProductId.value, note: versionNote.value } })
    }
    versionNote.value = ''
    ok.value = bomID ? '已复制为新版草稿' : '已保存版本'
    await loadVersions(selectedProductId.value)
  })
}

async function activateVersion(id) {
  if (!canEditCurrentBomProduct.value) return
  await mutate(async () => {
    if (currentProductionBomID.value) {
      await apiSend(`/api/production-bom-versions/${id}/publish`, { body: {} })
      ok.value = '已发布版本'
    } else {
      await apiSend(`/api/bom/versions/${id}/activate`, { body: {} })
      ok.value = '已启用版本'
    }
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

onMounted(async () => {
  const params = new URL(window.location.href).searchParams
  selectedProductId.value = Number(props.viewParams?.product_id || params.get('product_id') || 0)
  bomFilterProductId.value = Number(props.viewParams?.bom_filter_product_id || params.get('bom_filter_product_id') || 0)
  pendingUrlProductId.value = selectedProductId.value
  await loadAll()
  await restoreBomFormDraft()
})

onBeforeUnmount(saveBomFormDraft)

watch(selectedBomCustomerSkuCustomerID, (customerID) => {
  selectedBomRowKeys.value = []
  syncSelectedProductToBomContext()
  notifyWorkspaceCustomerChanged(customerID)
})

watch(selectedProductionBomGroupID, (groupID) => {
  selectedBomRowKeys.value = []
  selectedBomMoveGroupID.value = Number(groupID || 0) > 0 ? Number(groupID || 0) : 0
})

watch([productionBomStatusFilter, productionBomSearchQuery], () => {
  selectedBomRowKeys.value = []
})

watch(() => props.customerContextId, applyWorkspaceCustomerContext, { immediate: true })

watch(selectedProductId, () => {
  if (itemForm.component_type === 'finished_product' && isDripProduct(selectedProduct.value)) {
    itemForm.qty_per_unit = Number(selectedProduct.value.drip_bag_grams || 0) || ''
  }
})
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
.bom-list-head { align-items: flex-end; }
.bom-list-toolbar { display: flex; align-items: flex-end; gap: 10px; flex-wrap: wrap; justify-content: flex-end; }
.bom-list-tabs { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin: 10px 0 12px; }
.bom-move-card { display: flex; align-items: flex-end; gap: 12px; flex-wrap: wrap; border: 1px solid #eee8df; border-radius: 8px; background: #fbfaf8; padding: 12px; margin-bottom: 12px; }
.bom-move-card strong { display: block; margin-bottom: 4px; }
.bom-name-button { height: auto; min-height: 30px; text-align: left; font-weight: 700; }
.bom-record-form { align-items: flex-end; }
.bom-group-strip { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; width: 100%; }
.bom-group-strip > span { color: #666; font-size: 12px; font-weight: 700; }
.bom-focus-filter { align-self: stretch; display: inline-flex; align-items: center; gap: 8px; padding: 8px 10px; border: 1px solid #dbeafe; border-radius: 8px; background: #eff6ff; color: #1d4ed8; font-size: 13px; }
.bom-sku-context-panel { border: 1px solid #eee8df; border-radius: 8px; padding: 12px; margin-bottom: 12px; display: grid; grid-template-columns: minmax(240px, 1fr) minmax(260px, auto); gap: 10px 14px; align-items: center; background: #fbfaf8; }
.context-eyebrow { color: #7a4d1a; font-size: 12px; font-weight: 700; }
.bom-sku-context-controls { display: flex; align-items: center; justify-content: flex-end; gap: 8px; flex-wrap: wrap; }
.bom-customer-select { min-width: min(320px, 100%); }
.context-stats { grid-column: 1 / -1; display: flex; flex-wrap: wrap; gap: 8px; }
.context-stats span { border: 1px solid #e0d7cc; border-radius: 999px; padding: 4px 9px; background: #fff; color: #333; font-size: 12px; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 640px; border-collapse: collapse; }
.compact table { min-width: 520px; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
.select-col { width: 42px; text-align: center; }
.select-col input { min-width: 0; width: 16px; height: 16px; }
tbody tr.active { background: #f3f7fb; }
.list-panel tbody tr { cursor: pointer; }
.summary { align-items: stretch; margin-bottom: 12px; }
.summary div { min-width: 120px; border: 1px solid #eee8df; border-radius: 6px; padding: 9px; }
.summary strong { font-size: 16px; }
.linked-processes { display: flex; flex-wrap: wrap; gap: 8px; margin: -4px 0 12px; }
.version-attrs-panel { border: 1px solid #eee8df; border-radius: 8px; padding: 12px; margin-top: 12px; background: #fbfaf8; }
.section-title-row { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; margin-bottom: 12px; }
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
  .bom-sku-context-panel { grid-template-columns: 1fr; }
  .bom-sku-context-controls { justify-content: flex-start; }
  .attrs-grid { grid-template-columns: 1fr; }
  table { min-width: 620px; }
}
</style>
