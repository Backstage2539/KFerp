<template>
  <div class="page">
    <div v-if="materialReturnNavigation" class="material-return-banner">
      <button class="secondary" type="button" @click="returnToMaterialSource">{{ materialReturnLabel }}</button>
      <span>完成物料档案查看后可返回来源操作。</span>
    </div>
    <section class="panel compact-head">
      <div class="panel-head">
        <div>
          <h2>物料档案</h2>
          <p>按分类维护原料、包材和其他物料；单位来自全局单位字典，库存数量通过库存补录或库存调整修正。</p>
        </div>
        <button class="secondary" type="button" @click="loadAll" :disabled="loading || materialCategoryMoveActive">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <div class="materials-layout">
      <section class="panel material-list-panel">
        <div class="panel-title">物料列表</div>
        <BusinessGroupInlineWorkspace
          v-model:collapsed-keys="collapsedMaterialCategoryKeys"
          data-pr513-material-business-groups
          :groups="paginatedMaterialGroups"
          :move-active="materialCategoryMoveActive"
          :selected-count="selectedMaterialRowsForMove.length"
          :can-move="canMoveSelectedMaterialsToBusinessGroup"
          :loading="loading"
          count-unit="个"
          all-label="全部物料"
          manage-label="前往分组模板"
          configure-label="设置分组模板"
          @move="startMaterialCategoryMove"
          @cancel="cancelMaterialCategoryMove"
          @target="handleMaterialCategoryMoveTarget"
          @manage="openMaterialBusinessGroupManagement"
          @configure="openMaterialGroupFeatureSelectionDrawer">
          <template #filters>
            <div class="material-list-toolbar">
              <label>
                <span>搜索</span>
                <input v-model.trim="q" placeholder="名称/编码/批次号" @keyup.enter="applyMaterialFilters" />
              </label>
              <label>
                <span>状态</span>
                <select v-model="filters.active" @change="applyMaterialFilters">
                  <option value="active">启用</option>
                  <option value="inactive">失效</option>
                  <option value="all">全部</option>
                </select>
              </label>
              <label>
                <span>半成品标识</span>
                <select v-model="filters.semiFinished" @change="applySemiFinishedFilter">
                  <option value="all">全部</option>
                  <option value="semi_finished">半成品</option>
                  <option value="non_semi_finished">非半成品</option>
                </select>
              </label>
              <button class="primary" type="button" @click="applyMaterialFilters" :disabled="loading">查询</button>
              <span class="spacer"></span>
              <button class="primary" type="button" @click="createMaterial">新建物料</button>
              <button class="danger" type="button" @click="deprecateSelectedMaterials" :disabled="!selectedMaterialIDs.length || loading">批量失效</button>
            </div>
          </template>

          <template #group="{ group }">
            <div class="table-wrap">
              <table class="materials-table" data-auto-pagination="off">
                <colgroup>
                  <col class="select-col" />
                  <col class="name-col" />
                  <col class="unit-col" />
                  <col class="stock-col" />
                  <col class="manufacture-col" />
                  <col class="status-col" />
                </colgroup>
                <thead>
                  <tr>
                    <th>
                      <input
                        type="checkbox"
                        title="全选物料"
                        aria-label="全选物料"
                        :checked="areRowsSelected(group.rows)"
                        :disabled="!group.rows.length || loading || materialCategoryMoveActive"
                        @change="toggleMaterialRows(group.rows)" />
                    </th>
                    <th>物料名称</th>
                    <th>单位</th>
                    <th>库存数量</th>
                    <th>制造属性</th>
                    <th>状态</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="row in group.rows"
                    :key="row.id"
                    :class="{ active: selected?.id === row.id }"
                    :style="businessGroupItemIndentStyle(group)">
                    <td>
                      <input
                        type="checkbox"
                        :checked="selectedMaterialIDs.includes(row.id)"
                        :disabled="loading || materialCategoryMoveActive"
                        @change="toggleMaterialSelection(row.id)" />
                    </td>
                    <td class="material-name-cell">
                      <button class="material-name-button" type="button" :disabled="loading || materialCategoryMoveActive" @click.stop="selectMaterial(row)">{{ row.name }}</button>
                      <small>{{ row.code || '-' }}</small>
                    </td>
                    <td>{{ unitDisplay(row.unit) }}</td>
                    <td>{{ stockQty(row) }} {{ unitDisplay(row.unit) }}</td>
                    <td>
                      <span v-if="row.is_semi_finished" class="pill">半成品标识</span>
                      <small>{{ row.can_manufacture ? '可制造' : '无默认发布 BOM' }}</small>
                    </td>
                    <td><span :class="row.deprecated_at ? 'pill muted-pill' : 'pill ok-pill'">{{ row.deprecated_at ? '失效' : '启用' }}</span></td>
                  </tr>
                  <tr v-if="!group.rows.length"><td colspan="6" class="muted">暂无物料</td></tr>
                </tbody>
              </table>
            </div>
            <PaginationControls
              :key="`${group.key}-pagination-${group.pageSize}-${group.total}`"
              :page="group.page"
              :page-size="group.pageSize"
              :total="group.total"
              :disabled="loading || materialCategoryMoveActive"
              @change="handleMaterialGroupPaginationChange(group.key, $event)" />
          </template>
        </BusinessGroupInlineWorkspace>
      </section>
    </div>

    <div v-if="materialDetailDrawerOpen && draft" class="drawer-mask material-detail-drawer-mask" @click.self="closeMaterialDetailDrawer">
      <aside class="drawer material-detail-drawer" data-material-detail-drawer aria-label="物料详情">
        <div class="drawer-head">
          <div>
            <h3>{{ draftMode ? '新建物料' : '物料详情' }}</h3>
            <p>新建、编辑、失效和分类移动都会写操作日志。</p>
          </div>
          <div class="actions">
            <button v-if="!draftMode" class="secondary" type="button" @click="openStockBackfill" :disabled="loading">库存补录</button>
            <button class="secondary" type="button" @click="closeMaterialDetailDrawer" :disabled="loading">关闭</button>
          </div>
        </div>

        <form class="detail-form" @submit.prevent="saveMaterial">
          <section class="form-section">
            <div class="section-title">基础信息</div>
            <div class="form-grid">
              <label><span>编码</span><input v-model.trim="draft.code" /></label>
              <label><span>名称</span><input v-model.trim="draft.name" /></label>
              <label>
                <span>库存单位（全局单位字典）</span>
                <select v-model="draft.unit" :disabled="materialInventoryUnitLocked">
                  <option v-for="unit in unitOptions" :key="unit.code" :value="unit.code">{{ unit.label || unit.name || unit.code }}</option>
                </select>
                <small>重量物料库存统一使用 kg；BOM 配方仍可按 g 录入并自动换算。采购价、批次单位成本和 BOM 成本试算均按库存单位计价。</small>
                <small v-if="materialInventoryUnitLocked">库存单位保存后不可修改；如需调整，请新建物料档案。</small>
              </label>
              <label><span>批次号</span><input v-model.trim="draft.batch_no" /></label>
              <label v-if="!draft.is_semi_finished"><span>采购价（元/{{ draft.unit }}）</span><input type="number" min="0" step="0.01" v-model.number="draft.purchase_price" /></label>
              <label class="boolean-field">
                <span>是否半成品</span>
                <input v-model="draft.is_semi_finished" type="checkbox" />
                <small>半成品只允许通过生产入库；勾选后采购价自动清零，且不再出现在采购和普通入库候选中。</small>
              </label>
              <label><span>更新时间</span><input :value="draft.updated_at || '-'" disabled /></label>
            </div>
          </section>

          <section v-if="!draftMode" class="form-section material-bom-links">
            <div class="section-title">制造 BOM 关联</div>
            <div class="bom-link-group">
              <strong>产出该物料的 BOM</strong>
              <span class="manufacturing-status" :class="draft.can_manufacture ? 'manufacturing-status-ready' : 'manufacturing-status-missing'">{{ draft.can_manufacture ? '可制造（已有默认发布 BOM）' : '不可制造（无默认发布 BOM）' }}</span>
              <button v-for="bom in producedByBoms" :key="`produced-${bom.id}`" class="secondary subtle" type="button" @click="openMaterialBom(bom)">{{ materialBomLabel(bom) }}</button>
              <span v-if="!materialBomReferencesLoading && !producedByBoms.length" class="muted left">暂无</span>
            </div>
            <div class="bom-link-group">
              <strong>使用该物料的 BOM</strong>
              <button v-for="bom in usedByBoms" :key="`used-${bom.id}`" class="secondary subtle" type="button" @click="openMaterialBom(bom)">{{ materialBomLabel(bom) }}</button>
              <span v-if="!materialBomReferencesLoading && !usedByBoms.length" class="muted left">暂无</span>
            </div>
            <span v-if="materialBomReferencesLoading" class="muted left">正在加载 BOM 关联...</span>
          </section>

          <section class="form-section">
            <div class="section-title">库存</div>
            <div class="form-grid">
              <label><span>库存数量（库存单位）</span><input type="number" :value="stockQty(draft)" disabled /></label>
              <label><span>警戒线（库存单位）</span><input type="number" min="0" step="0.001" v-model.number="draft.min_level_qty" /></label>
            </div>
          </section>

          <section class="form-section">
            <div class="section-title">行业字段</div>
            <div class="form-grid">
              <label class="wide">
                <span>行业字段模板</span>
                <select v-model.number="draft.industry_field_template_id" @change="syncIndustryFieldsFromTemplate">
                  <option :value="0">不使用模板</option>
                  <option v-for="tpl in activeIndustryTemplates" :key="tpl.id" :value="tpl.id">{{ tpl.name }}</option>
                </select>
              </label>
              <template v-if="selectedIndustryTemplateFields.length">
                <label v-for="field in selectedIndustryTemplateFields" :key="field.field_key">
                  <span>{{ field.field_key }}</span>
                  <select v-if="field.field_type === 'select'" :value="industryFieldValue(field.field_key)" @change="setIndustryFieldValue(field.field_key, $event.target.value)">
                    <option value="">默认不填</option>
                    <option v-for="option in fieldOptions(field)" :key="option" :value="option">{{ option }}</option>
                  </select>
                  <input v-else :value="industryFieldValue(field.field_key)" placeholder="默认不填" @input="setIndustryFieldValue(field.field_key, $event.target.value)" />
                </label>
              </template>
              <div v-else class="empty muted wide">选择行业字段模板后，在这里维护字段值。</div>
            </div>
          </section>

          <div class="form-actions">
            <button class="primary" type="submit" :disabled="loading">{{ draftMode ? '保存新物料' : '保存物料' }}</button>
          </div>
        </form>
      </aside>
    </div>

    <div v-if="materialGroupFeatureDrawerOpen" class="drawer-mask" @click.self="materialGroupFeatureDrawerOpen = false">
      <aside class="drawer material-group-feature-drawer" aria-label="物料档案分组模板设置">
        <div class="drawer-head">
          <div>
            <h3>物料档案分组模板</h3>
            <p>选择物料档案用于浏览和移动归类的分组模板。</p>
          </div>
          <button class="secondary compact-action" type="button" @click="materialGroupFeatureDrawerOpen = false">关闭</button>
        </div>
        <div class="feature-group-selection" data-feature-key="material_catalog">
          <div class="feature-group-selection-copy">
            <strong>物料档案使用的分组模板</strong>
            <small>可多选；保存后列表会按全部已选模板合并展示分类层级。取消全部后按物料平铺展示。</small>
          </div>
          <div class="feature-group-selection-options">
            <label v-for="template in selectableMaterialGroupTemplates" :key="template.id" class="feature-group-selection-option">
              <input
                v-model="materialGroupFeatureSelectionDraft"
                type="checkbox"
                :value="Number(template.id || 0)"
                :disabled="materialGroupFeatureSelectionSaving || loading" />
              <span>{{ template.label }}</span>
            </label>
            <span v-if="!selectableMaterialGroupTemplates.length" class="muted left">暂无可选分组模板，请先维护模板。</span>
          </div>
          <div class="feature-group-selection-actions">
            <button class="secondary compact-action" type="button" :disabled="materialGroupFeatureSelectionSaving || loading" @click="materialGroupFeatureDrawerOpen = false">取消</button>
            <button class="primary compact-action" type="button" :disabled="materialGroupFeatureSelectionSaving || loading || !materialGroupFeatureSelectionHasChanges" @click="saveMaterialGroupFeatureSelection">
              {{ materialGroupFeatureSelectionSaving ? '保存中' : '保存模板选择' }}
            </button>
          </div>
        </div>
      </aside>
    </div>

    <div v-if="stockBackfill.open" class="modal-mask stock-backfill-mask" @click.self="closeStockBackfill">
      <section class="modal-panel">
        <div class="modal-head">
          <div>
            <h3>库存补录</h3>
            <p>{{ selected?.name || '-' }} · {{ selected?.batch_no || '-' }}</p>
          </div>
          <button class="secondary" type="button" @click="closeStockBackfill">关闭</button>
        </div>
        <form class="detail-form" @submit.prevent="submitStockBackfill">
          <div class="form-grid">
            <label><span>当前库存数量（{{ selectedUnitLabel }}）</span><input type="number" :value="stockQty(selected)" disabled /></label>
            <label><span>目标库存数量（{{ selectedUnitLabel }}）</span><input type="number" min="0" step="0.001" v-model.number="stockBackfill.target_qty" /></label>
            <label class="wide"><span>补录说明</span><textarea v-model.trim="stockBackfill.reason" rows="3" required></textarea></label>
          </div>
          <div class="form-actions">
            <button class="secondary" type="button" @click="closeStockBackfill">取消</button>
            <button class="primary" type="submit" :disabled="loading">提交补录</button>
          </div>
        </form>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import BusinessGroupInlineWorkspace from '../components/BusinessGroupInlineWorkspace.vue'
import PaginationControls from '../components/PaginationControls.vue'
import {
  businessGroupControlOptions,
  businessGroupFeatureSelectionIDs,
  businessGroupFeatureSelectionPayload,
  businessGroupInlineListState,
  businessGroupItemIndentStyle,
  businessGroupMoveAssignmentPayload,
  businessGroupRowsForFeatureSelection,
  groupRowsByBusinessGroupTemplates,
} from '../lib/business-grouping'
import { normalizePageSize } from '../lib/pagination'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
})

const MATERIAL_CATALOG_USAGE = 'material_catalog'
const MATERIAL_OBJECT_KEY = 'material'

const rows = ref([])
const materialBusinessGroups = ref([])
const materialBusinessGroupAssignments = ref([])
const industryFieldTemplates = ref([])
const productUnitDefinitions = ref([])
const q = ref('')
const filters = reactive({ active: 'active', semiFinished: 'all' })
const loading = ref(false)
const error = ref('')
const ok = ref('')
const selected = ref(null)
const draft = ref(null)
const draftMode = ref(false)
const selectedMaterialIDs = ref([])
const collapsedMaterialCategoryKeys = ref([])
const materialGroupPagination = ref({})
const materialCategoryMoveActive = ref(false)
const materialGroupFeatureSelectionTemplateIDs = ref([])
const materialGroupFeatureSelectionDraft = ref([])
const materialGroupFeatureSelectionSaving = ref(false)
const materialGroupFeatureDrawerOpen = ref(false)
const materialDetailDrawerOpen = ref(false)
const stockBackfill = ref({ open: false, target_qty: 0, reason: '' })
const producedByBoms = ref([])
const usedByBoms = ref([])
const materialBomReferencesLoading = ref(false)
const materialReturnNavigation = computed(() => props.viewParams?.return_navigation || null)
const materialReturnLabel = computed(() => String(materialReturnNavigation.value?.label || '返回来源操作'))

const activeIndustryTemplates = computed(() => industryFieldTemplates.value.filter((tpl) => !tpl.deactivated_at && tpl.active !== false))
const selectableMaterialGroupTemplates = computed(() => businessGroupControlOptions(materialBusinessGroups.value).templateOptions)
const materialGroupFeatureSelectionHasChanges = computed(() => (
  JSON.stringify(businessGroupFeatureSelectionIDs({ group_template_ids: materialGroupFeatureSelectionDraft.value }))
  !== JSON.stringify(businessGroupFeatureSelectionIDs({ group_template_ids: materialGroupFeatureSelectionTemplateIDs.value }))
))
const materialCatalogBusinessGroups = computed(() => businessGroupRowsForFeatureSelection(
  materialBusinessGroups.value,
  materialGroupFeatureSelectionTemplateIDs.value,
))
const filteredMaterialRows = computed(() => {
  if (filters.semiFinished === 'semi_finished') return rows.value.filter((row) => row.is_semi_finished)
  if (filters.semiFinished === 'non_semi_finished') return rows.value.filter((row) => !row.is_semi_finished)
  return rows.value
})
const materialDisplayGroups = computed(() => groupRowsByBusinessGroupTemplates(filteredMaterialRows.value, {
  templates: materialCatalogBusinessGroups.value,
  assignments: materialBusinessGroupAssignments.value,
  usageKey: MATERIAL_CATALOG_USAGE,
  objectKey: MATERIAL_OBJECT_KEY,
  objectIDForRow: (row) => Number(row.id || 0),
  allLabel: '全部物料',
}))
const materialInlineListState = computed(() => businessGroupInlineListState(
  materialDisplayGroups.value,
  materialGroupPagination.value,
  { defaultPageSize: 10 },
))
const paginatedMaterialGroups = computed(() => materialInlineListState.value.groups)
const selectedMaterialRowsForMove = computed(() => {
  const selectedIds = new Set(selectedMaterialIDs.value.map((id) => Number(id || 0)).filter(Boolean))
  return rows.value.filter((row) => selectedIds.has(Number(row.id || 0)))
})
const canMoveSelectedMaterialsToBusinessGroup = computed(() => Boolean(
  materialCatalogBusinessGroups.value.length && selectedMaterialRowsForMove.value.length,
))
const MATERIAL_WEIGHT_UNIT_CODES = new Set([
  'g', 'gram', 'grams', '克',
  'kg', 'kgs', 'kilogram', 'kilograms', '千克', '公斤',
  'lb', 'lbs', 'pound', 'pounds', '磅',
  'oz', 'ounce', 'ounces', '盎司',
])

function normalizeMaterialUnitCode(unitCode) {
  return String(unitCode || '').trim().toLowerCase()
}

function isMaterialWeightUnitType(unitType) {
  const normalized = String(unitType || '').trim().toLowerCase()
  return normalized === 'weight' || normalized === '重量'
}

function isCanonicalMaterialInventoryUnit(unitCode, unitType) {
  const normalized = normalizeMaterialUnitCode(unitCode)
  return normalized === 'kg' || (!isMaterialWeightUnitType(unitType) && !MATERIAL_WEIGHT_UNIT_CODES.has(normalized))
}

const unitOptions = computed(() => {
  const rows = productUnitDefinitions.value
    .filter((row) => row.active !== false)
    .filter((row) => isCanonicalMaterialInventoryUnit(row.code, row.unit_type))
  const kgSource = rows.find((row) => normalizeMaterialUnitCode(row.code) === 'kg')
  const kg = kgSource ? { ...kgSource, code: 'kg' } : { code: 'kg', name: 'kg', label: 'kg', active: true }
  const nonWeightRows = rows.filter((row) => normalizeMaterialUnitCode(row.code) !== 'kg')
  return [kg, ...nonWeightRows]
})
const selectedIndustryTemplate = computed(() => activeIndustryTemplates.value.find((tpl) => tpl.id === Number(draft.value?.industry_field_template_id || 0)) || null)
const selectedIndustryTemplateFields = computed(() => (selectedIndustryTemplate.value?.fields || []).slice().sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0)))
const selectedUnitLabel = computed(() => unitDisplay(selected.value?.unit || ''))
const materialIndustryFields = computed(() => draft.value?.industry_fields || [])

function normalizeRow(row) {
  const fields = row.IndustryFields || row.industry_fields || []
  const unit = row.Unit ?? row.unit ?? 'g'
  return {
    id: Number(row.ID ?? row.id ?? 0),
    code: row.Code ?? row.code ?? '',
    name: row.Name ?? row.name ?? '',
    kind: row.Kind ?? row.kind ?? 'other',
    is_semi_finished: Boolean(row.IsSemiFinished ?? row.is_semi_finished ?? false),
    can_manufacture: Boolean(row.CanManufacture ?? row.can_manufacture ?? false),
    unit,
    cost_unit: unit,
    batch_no: row.BatchNo ?? row.batch_no ?? '',
    purchase_price: Number(row.PurchasePrice ?? row.purchase_price ?? 0),
    onhand_g: Number(row.OnhandG ?? row.onhand_g ?? 0),
    onhand_units: Number(row.OnhandUnits ?? row.onhand_units ?? 0),
    stock_qty: Number(row.StockQty ?? row.stock_qty ?? 0),
    min_level_g: Number(row.MinLevelG ?? row.min_level_g ?? 0),
    min_level_units: Number(row.MinLevelUnits ?? row.min_level_units ?? 0),
    min_level_qty: Number(row.MinLevelQty ?? row.min_level_qty ?? 0),
    industry_field_template_id: Number(row.IndustryFieldTemplateID ?? row.industry_field_template_id ?? 0),
    industry_fields: fields.map((field) => ({
      field_key: field.FieldKey ?? field.field_key ?? '',
      value_text: field.ValueText ?? field.value_text ?? '',
    })).filter((field) => field.field_key),
    classification_group_id: Number(row.ClassificationGroupID ?? row.classification_group_id ?? 0),
    classification_group_name: row.ClassificationGroupName ?? row.classification_group_name ?? '',
    classification_category_id: Number(row.ClassificationCategoryID ?? row.classification_category_id ?? 0),
    classification_category_name: row.ClassificationCategoryName ?? row.classification_category_name ?? '',
    updated_at: row.UpdatedAt ?? row.updated_at ?? '',
    deprecated_at: row.DeprecatedAt ?? row.deprecated_at ?? '',
  }
}

function normalizeTemplate(row) {
  return {
    id: Number(row.ID ?? row.id ?? 0),
    name: row.Name ?? row.name ?? '',
    active: row.Active ?? row.active ?? true,
    deactivated_at: row.DeactivatedAt ?? row.deactivated_at ?? '',
    fields: (row.Fields || row.fields || []).map((field) => ({
      field_key: field.FieldKey ?? field.field_key ?? '',
      field_type: normalizeFieldType(field.FieldType ?? field.field_type ?? 'text'),
      options_json: field.OptionsJSON ?? field.options_json ?? '[]',
      sort_order: Number(field.SortOrder ?? field.sort_order ?? 100),
    })).filter((field) => field.field_key),
  }
}

function normalizeUnit(row) {
  return {
    code: row.code ?? row.Code ?? '',
    name: row.name ?? row.Name ?? row.code ?? row.Code ?? '',
    label: row.name ?? row.Name ?? row.code ?? row.Code ?? '',
    unit_type: row.unit_type ?? row.UnitType ?? 'other',
    active: row.active ?? row.Active ?? true,
  }
}

function cloneDraft(row) {
  return JSON.parse(JSON.stringify(row))
}

function blankDraft() {
  const unit = unitOptions.value.find((row) => normalizeMaterialUnitCode(row.code) === 'kg')?.code || 'kg'
  return {
    id: 0,
    code: nextMaterialCode(),
    name: '',
    kind: 'other',
    is_semi_finished: false,
    can_manufacture: false,
    unit,
    cost_unit: unit,
    batch_no: '',
    purchase_price: 0,
    onhand_g: 0,
    onhand_units: 0,
    stock_qty: 0,
    min_level_g: 0,
    min_level_units: 0,
    min_level_qty: 0,
    industry_field_template_id: 0,
    industry_fields: [],
    updated_at: '',
    deprecated_at: '',
  }
}

function nextMaterialCode() {
  const maxID = rows.value.reduce((max, row) => Math.max(max, row.id), 0) + 1
  return `MAT-${String(maxID).padStart(6, '0')}`
}

async function loadAll() {
  await Promise.all([loadOptions(), loadMaterialBusinessGroupConfiguration(), loadMaterialBusinessGroupAssignments(), loadMaterials()])
}

async function loadOptions() {
  const [settings, industry] = await Promise.all([
    apiGet('/api/product-settings'),
    apiGet('/api/industry-field-templates'),
  ])
  productUnitDefinitions.value = (settings.product_unit_definitions || []).map(normalizeUnit).filter((row) => row.code)
  industryFieldTemplates.value = (industry.rows || []).map(normalizeTemplate)
}

async function loadMaterialBusinessGroupConfiguration() {
  const [groupData, selectionData] = await Promise.all([
    apiGet('/api/business-groups'),
    apiGet('/api/business-group-feature-selections/material_catalog'),
  ])
  materialBusinessGroups.value = Array.isArray(groupData?.rows) ? groupData.rows : (Array.isArray(groupData) ? groupData : [])
  materialGroupFeatureSelectionTemplateIDs.value = businessGroupFeatureSelectionIDs(selectionData)
  materialGroupFeatureSelectionDraft.value = [...materialGroupFeatureSelectionTemplateIDs.value]
}

async function loadMaterialBusinessGroupAssignments() {
  const url = new URL('/api/business-group-assignments', window.location.origin)
  url.searchParams.set('usage_key', MATERIAL_CATALOG_USAGE)
  url.searchParams.set('object_key', MATERIAL_OBJECT_KEY)
  const data = await apiGet(`${url.pathname}${url.search}`)
  materialBusinessGroupAssignments.value = Array.isArray(data?.rows) ? data.rows : (Array.isArray(data?.assignments) ? data.assignments : [])
}

async function loadMaterials({ resetPagination = false } = {}) {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/materials', window.location.origin)
    url.searchParams.set('limit', '500')
    url.searchParams.set('active', filters.active)
    if (q.value) url.searchParams.set('q', q.value)
    const data = await apiGet(`${url.pathname}${url.search}`)
    rows.value = (data.rows || []).map(normalizeRow)
    selectedMaterialIDs.value = selectedMaterialIDs.value.filter((id) => rows.value.some((row) => row.id === id))
    if (selected.value?.id) {
      const next = rows.value.find((row) => row.id === selected.value.id)
      if (next) {
        selectMaterial(next, { quiet: true, openDrawer: materialDetailDrawerOpen.value })
      } else {
        selected.value = null
        draft.value = null
        draftMode.value = false
        materialDetailDrawerOpen.value = false
        closeStockBackfill()
      }
    }
    if (resetPagination) resetMaterialGroupPages()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function applyMaterialFilters() {
  return loadMaterials({ resetPagination: true })
}

function applySemiFinishedFilter() {
  const visibleIDs = new Set(filteredMaterialRows.value.map((row) => Number(row.id || 0)).filter(Boolean))
  selectedMaterialIDs.value = selectedMaterialIDs.value.filter((id) => visibleIDs.has(Number(id || 0)))
  resetMaterialGroupPages()
}

function createMaterial() {
  selected.value = null
  draft.value = blankDraft()
  draftMode.value = true
  materialDetailDrawerOpen.value = true
  error.value = ''
  ok.value = ''
}

function selectMaterial(row, options = {}) {
  selected.value = row
  draft.value = cloneDraft(row)
  draftMode.value = false
  closeStockBackfill()
  if (options.openDrawer !== false) materialDetailDrawerOpen.value = true
  loadMaterialBomReferences(row.id)
  if (!options.quiet) {
    error.value = ''
    ok.value = ''
  }
}

function closeMaterialDetailDrawer() {
  materialDetailDrawerOpen.value = false
  closeStockBackfill()
}

function normalizeMaterialBomReference(row = {}) {
  return {
    ...row,
    id: Number(row.production_bom_id || row.bom_id || row.id || 0),
    name: row.production_bom_name || row.bom_name || row.name || '',
    code: row.production_bom_code || row.bom_code || row.code || '',
  }
}

async function loadMaterialBomReferences(materialID) {
  const id = Number(materialID || 0)
  producedByBoms.value = []
  usedByBoms.value = []
  if (!id) return
  materialBomReferencesLoading.value = true
  try {
    const [produced, used] = await Promise.all([
      apiGet(`/api/production-boms?status=all&output_type=material&output_id=${id}`),
      apiGet(`/api/production-boms?status=all&component_type=material&component_id=${id}`),
    ])
    producedByBoms.value = (produced?.rows || produced || []).map(normalizeMaterialBomReference)
    usedByBoms.value = (used?.rows || used || []).map(normalizeMaterialBomReference)
  } catch (err) {
    error.value = err.message || '加载物料 BOM 关联失败'
  } finally {
    materialBomReferencesLoading.value = false
  }
}

function materialBomLabel(row = {}) {
  return [row.code, row.name].filter(Boolean).join(' · ') || `BOM #${Number(row.id || 0)}`
}

function openMaterialBom(row = {}) {
  const bomID = Number(row.id || row.production_bom_id || 0)
  const materialID = Number(selected.value?.id || draft.value?.id || 0)
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: 'productionConfig',
      params: bomID > 0 ? { tab: 'bom', production_bom_id: bomID } : { tab: 'bom' },
      returnNavigation: {
        key: 'materials',
        label: `返回物料档案：${draft.value?.name || selected.value?.name || ''}`,
        params: materialID > 0 ? { open_material_id: materialID } : {},
        source_label: `生产 BOM：${materialBomLabel(row)}`,
      },
    },
  }))
}

function returnToMaterialSource() {
  const navigation = materialReturnNavigation.value
  if (!navigation) return
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: { key: String(navigation.key || 'productionConfig'), params: navigation.params || {} },
  }))
}

function openMaterialFromViewParams(params = props.viewParams) {
  const id = Number(params?.open_material_id || 0)
  if (!id) return
  const row = rows.value.find((item) => Number(item.id || 0) === id)
  if (row) selectMaterial(row, { quiet: true })
}

function toggleMaterialSelection(id) {
  const next = new Set(selectedMaterialIDs.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedMaterialIDs.value = Array.from(next)
}

function rowIDsForSelection(sourceRows) {
  return (sourceRows || []).map((row) => Number(row.id || 0)).filter(Boolean)
}

function areRowsSelected(sourceRows) {
  const ids = rowIDsForSelection(sourceRows)
  if (!ids.length) return false
  const selectedSet = new Set(selectedMaterialIDs.value)
  return ids.every((id) => selectedSet.has(id))
}

function toggleMaterialRows(sourceRows) {
  const ids = rowIDsForSelection(sourceRows)
  if (!ids.length) return
  const next = new Set(selectedMaterialIDs.value)
  const shouldClear = ids.every((id) => next.has(id))
  for (const id of ids) {
    if (shouldClear) next.delete(id)
    else next.add(id)
  }
  selectedMaterialIDs.value = Array.from(next)
}

function syncMaterialGroupPaginationState() {
  const normalized = materialInlineListState.value.pagination
  if (JSON.stringify(normalized) !== JSON.stringify(materialGroupPagination.value)) {
    materialGroupPagination.value = normalized
  }
}

function resetMaterialGroupPages() {
  materialGroupPagination.value = Object.fromEntries(
    Object.entries(materialInlineListState.value.pagination).map(([key, value]) => [key, {
      page: 1,
      pageSize: normalizePageSize(value?.pageSize || 10),
    }]),
  )
}

function handleMaterialGroupPaginationChange(groupKey, { page, pageSize }) {
  const key = String(groupKey || '')
  if (!key) return
  materialGroupPagination.value = {
    ...materialGroupPagination.value,
    [key]: {
      page: Number(page || 1),
      pageSize: normalizePageSize(pageSize || 10),
    },
  }
}

function materialBusinessGroupAssignment(row = {}) {
  const id = Number(row.id || 0)
  return materialBusinessGroupAssignments.value.find((assignment) => (
    String(assignment.usage_key || '').toLowerCase() === MATERIAL_CATALOG_USAGE
    && String(assignment.object_key || '').toLowerCase() === MATERIAL_OBJECT_KEY
    && Number(assignment.object_id || 0) === id
  )) || null
}

function materialBusinessGroupID(row = {}) {
  return Number(materialBusinessGroupAssignment(row)?.group_id || 0)
}

function materialBusinessGroupItemID(row = {}) {
  return Number(materialBusinessGroupAssignment(row)?.group_item_id || 0)
}

function openMaterialGroupFeatureSelectionDrawer() {
  materialGroupFeatureSelectionDraft.value = [...materialGroupFeatureSelectionTemplateIDs.value]
  materialGroupFeatureDrawerOpen.value = true
}

async function saveMaterialGroupFeatureSelection() {
  const payload = businessGroupFeatureSelectionPayload(MATERIAL_CATALOG_USAGE, materialGroupFeatureSelectionDraft.value)
  materialGroupFeatureSelectionSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend('/api/business-group-feature-selections/material_catalog', {
      method: 'PUT',
      body: payload,
    })
    materialGroupFeatureSelectionTemplateIDs.value = businessGroupFeatureSelectionIDs(result)
    materialGroupFeatureSelectionDraft.value = [...materialGroupFeatureSelectionTemplateIDs.value]
    collapsedMaterialCategoryKeys.value = []
    materialGroupPagination.value = {}
    materialCategoryMoveActive.value = false
    materialGroupFeatureDrawerOpen.value = false
    ok.value = payload.group_template_ids.length
      ? `物料档案已选择 ${payload.group_template_ids.length} 个分组模板`
      : '物料档案已改为平铺展示'
  } catch (err) {
    error.value = err.message || '保存物料档案分组模板失败'
  } finally {
    materialGroupFeatureSelectionSaving.value = false
  }
}

async function clearMaterialBusinessGroupAssignment(materialID) {
  const id = Number(materialID || 0)
  if (!id) return
  const url = new URL('/api/business-group-assignments', window.location.origin)
  url.searchParams.set('usage_key', MATERIAL_CATALOG_USAGE)
  url.searchParams.set('object_key', MATERIAL_OBJECT_KEY)
  url.searchParams.set('object_id', String(id))
  const data = await apiGet(`${url.pathname}${url.search}`)
  const rows = Array.isArray(data?.rows) ? data.rows : (Array.isArray(data?.assignments) ? data.assignments : [])
  await Promise.all(rows.map((row) => apiSend(`/api/business-group-assignments/${row.id}`, { method: 'DELETE' })))
}

function startMaterialCategoryMove() {
  if (!canMoveSelectedMaterialsToBusinessGroup.value || loading.value) return
  error.value = ''
  ok.value = ''
  materialCategoryMoveActive.value = true
}

function cancelMaterialCategoryMove() {
  materialCategoryMoveActive.value = false
}

async function handleMaterialCategoryMoveTarget(target = {}) {
  if (!materialCategoryMoveActive.value || loading.value) return false
  const targetGroupID = Number(target.group_id || 0)
  const targetGroupItemID = Number(target.group_item_id || 0)
  const targetIsUnclassified = Boolean(target.unclassified)
  if (!targetIsUnclassified && (!(targetGroupID > 0) || !(targetGroupItemID > 0))) return false
  const targetOption = targetIsUnclassified ? null : {
    group_id: targetGroupID,
    group_item_id: targetGroupItemID,
  }
  const materialsToMove = selectedMaterialRowsForMove.value.filter((row) => {
    if (!targetOption) return materialBusinessGroupID(row) > 0 || materialBusinessGroupItemID(row) > 0
    return materialBusinessGroupID(row) !== Number(targetOption.group_id || 0) || materialBusinessGroupItemID(row) !== Number(targetOption.group_item_id || 0)
  })
  if (!materialsToMove.length) {
    error.value = selectedMaterialRowsForMove.value.length
      ? '所选物料已在该分类，请选择其他分类'
      : '请先勾选物料'
    return false
  }
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    for (const row of materialsToMove) {
      if (!targetOption) {
        await clearMaterialBusinessGroupAssignment(row.id)
        continue
      }
      await apiSend('/api/business-group-assignments', {
        body: businessGroupMoveAssignmentPayload({
          usageKey: MATERIAL_CATALOG_USAGE,
          objectKey: MATERIAL_OBJECT_KEY,
          objectID: Number(row.id || 0),
          option: targetOption,
          sortOrder: 100,
        }),
      })
    }
    await loadMaterialBusinessGroupAssignments()
    ok.value = `已移动 ${materialsToMove.length} 个物料到分类`
    selectedMaterialIDs.value = []
    materialCategoryMoveActive.value = false
    return true
  } catch (err) {
    error.value = err.message || '移动物料分类失败，请重试'
    return false
  } finally {
    loading.value = false
  }
}

function openMaterialBusinessGroupManagement() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: 'groupTemplates',
      returnNavigation: {
        key: 'materials',
        label: '返回物料档案',
      },
    },
  }))
}
const materialInventoryUnitLocked = computed(() => Boolean(!draftMode.value && selected.value?.id))

function payloadFromDraft() {
  const sourceStock = draftMode.value ? { onhand_g: 0, onhand_units: 0 } : (selected.value || draft.value)
  return {
    code: draft.value.code,
    name: draft.value.name,
    kind: draft.value.kind || 'other',
    is_semi_finished: Boolean(draft.value.is_semi_finished),
    unit: draftMode.value ? draft.value.unit : (selected.value?.unit || draft.value.unit),
    cost_unit: draftMode.value ? draft.value.unit : (selected.value?.unit || draft.value.unit),
    batch_no: draft.value.batch_no,
    purchase_price: draft.value.is_semi_finished ? 0 : Number(draft.value.purchase_price || 0),
    onhand_g: Number(sourceStock.onhand_g || 0),
    onhand_units: Number(sourceStock.onhand_units || 0),
    min_level_qty: Number(draft.value.min_level_qty || 0),
    industry_field_template_id: Number(draft.value.industry_field_template_id || 0),
    industry_fields: selectedIndustryTemplateFields.value.map((field) => ({
      field_key: field.field_key,
      value_text: industryFieldValue(field.field_key),
    })),
  }
}

async function saveMaterial() {
  if (!draft.value) return
  await mutate(async () => {
    const creating = draftMode.value
    const url = creating ? '/api/materials' : `/api/materials/${draft.value.id}`
    const data = await apiSend(url, { body: payloadFromDraft() })
    const row = normalizeRow(data)
    draftMode.value = false
    await loadMaterials()
    const next = rows.value.find((item) => item.id === row.id) || row
    selectMaterial(next, { quiet: true })
    ok.value = creating ? '已保存新物料' : '已保存物料'
  })
}

function openStockBackfill() {
  if (!selected.value || draftMode.value) return
  stockBackfill.value = { open: true, target_qty: stockQty(selected.value), reason: '' }
  error.value = ''
  ok.value = ''
}

function closeStockBackfill() {
  stockBackfill.value.open = false
}

async function submitStockBackfill() {
  if (!selected.value || draftMode.value) return
  if (!stockBackfill.value.reason) {
    error.value = '补录说明必填'
    return
  }
  await mutate(async () => {
    const data = await apiSend('/api/stock/adjustments', {
      body: {
        item_type: 'material',
        item_id: selected.value.id,
        spec_g: 0,
        warehouse: '',
        target_qty: Number(stockBackfill.value.target_qty || 0),
        unit_code: selected.value.unit,
        reason: stockBackfill.value.reason,
      },
    })
    stockBackfill.value.open = false
    await loadMaterials()
    const next = rows.value.find((item) => item.id === selected.value?.id)
    if (next) selectMaterial(next, { quiet: true })
    ok.value = `库存补录已提交：#${data.adjustment_id || '-'}`
  })
}

async function deprecateSelectedMaterials() {
  const ids = selectedMaterialIDs.value.slice()
  if (!ids.length) {
    error.value = '请先勾选物料'
    return
  }
  if (!window.confirm(`批量失效 ${ids.length} 个物料？`)) return
  await mutate(async () => {
    for (const id of ids) {
      await apiSend(`/api/materials/${id}/deprecate`)
    }
    ok.value = `已失效 ${ids.length} 个物料`
    selectedMaterialIDs.value = []
    if (selected.value && ids.includes(selected.value.id)) {
      selected.value = null
      draft.value = null
      draftMode.value = false
      materialDetailDrawerOpen.value = false
    }
    await loadMaterials()
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

function stockQty(row) {
  if (!row) return 0
  if (Number.isFinite(Number(row.stock_qty)) && Number(row.stock_qty) !== 0) return Number(row.stock_qty)
  switch (String(row.unit || '').toLowerCase()) {
    case 'kg': return Number(row.onhand_g || 0) / 1000
    case 'lb': return Number(row.onhand_g || 0) / 453.59237
    case 'oz': return Number(row.onhand_g || 0) / 28.349523125
    case 'g': return Number(row.onhand_g || 0)
    default: return Number(row.onhand_units || 0)
  }
}

function unitDisplay(unitCode) {
  const row = unitOptions.value.find((unit) => unit.code === unitCode)
  return row?.name || unitCode || '-'
}

function normalizeFieldType(value) {
  return ['select', 'dropdown', 'option'].includes(String(value)) ? 'select' : 'text'
}

function fieldOptions(field) {
  try {
    const parsed = JSON.parse(field.options_json || '[]')
    return Array.isArray(parsed) ? parsed.map((item) => String(item).trim()).filter(Boolean) : []
  } catch {
    return String(field.options_json || '').split(/\s+/).map((item) => item.trim()).filter(Boolean)
  }
}

function industryFieldValue(fieldKey) {
  const field = materialIndustryFields.value.find((item) => item.field_key === fieldKey)
  return field?.value_text || ''
}

function setIndustryFieldValue(fieldKey, value) {
  if (!draft.value) return
  const next = (draft.value.industry_fields || []).filter((item) => item.field_key !== fieldKey)
  next.push({ field_key: fieldKey, value_text: String(value || '') })
  draft.value.industry_fields = next
}

function syncIndustryFieldsFromTemplate() {
  if (!draft.value) return
  const existing = new Map((draft.value.industry_fields || []).map((field) => [field.field_key, field.value_text]))
  draft.value.industry_fields = selectedIndustryTemplateFields.value.map((field) => ({
    field_key: field.field_key,
    value_text: existing.get(field.field_key) || defaultFieldValue(field),
  }))
}

function defaultFieldValue(field) {
  if (field.field_type === 'select') return ''
  try {
    const parsed = JSON.parse(field.options_json || '[]')
    return Array.isArray(parsed) ? String(parsed[0] || '') : ''
  } catch {
    return ''
  }
}

watch(materialDisplayGroups, syncMaterialGroupPaginationState, { deep: true, immediate: true })
watch(() => draft.value?.is_semi_finished, (isSemiFinished) => {
  if (isSemiFinished && draft.value) draft.value.purchase_price = 0
})
watch(() => props.viewParams?.open_material_id, () => openMaterialFromViewParams(), { flush: 'post' })

onMounted(async () => {
  q.value = new URL(window.location.href).searchParams.get('q') || ''
  await loadAll()
  openMaterialFromViewParams()
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.compact-head { padding: 12px 14px; }
.panel-head, .filters, .material-list-toolbar, .actions, .form-actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.panel-head { justify-content: space-between; margin-bottom: 10px; }
.panel-head h2 { margin: 0; font-size: 20px; }
.panel-head p { margin: 4px 0 0; color: #666; font-size: 12px; }
.panel-title { font-size: 16px; font-weight: 700; margin-bottom: 10px; }
.spacer { flex: 1 1 auto; }
.materials-layout { display: grid; grid-template-columns: minmax(0, 1fr); gap: 14px; align-items: start; }
.material-list-panel { min-width: 0; }
.material-return-banner { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; padding: 10px; border: 1px solid #d9e2ec; border-radius: 8px; background: #f8fbff; }
.material-list-toolbar { margin-bottom: 12px; padding-bottom: 12px; border-bottom: 1px solid #eee8df; }
.material-list-toolbar label { min-width: 150px; }
.material-list-toolbar label:first-child { flex: 1 1 220px; }
.material-list-toolbar label:nth-child(2) { flex: 0 0 130px; }
.feature-group-selection { display: grid; grid-template-columns: minmax(190px, .9fr) minmax(260px, 1.5fr) auto; gap: 10px; align-items: center; margin-bottom: 12px; padding: 10px; border: 1px solid #d9e2ec; border-radius: 8px; background: #f8fbff; }
.feature-group-selection-copy { display: grid; gap: 3px; }
.feature-group-selection-copy small { color: #607086; line-height: 1.4; }
.feature-group-selection-options, .feature-group-selection-actions { display: flex; align-items: center; gap: 8px 12px; flex-wrap: wrap; }
.feature-group-selection-actions { justify-content: flex-end; }
.feature-group-selection-option { display: inline-flex; align-items: center; gap: 6px; white-space: nowrap; }
.feature-group-selection-option input { width: auto; min-height: 0; }
.feature-group-empty { margin-bottom: 12px; padding: 10px; border: 1px dashed #d6d3d1; border-radius: 8px; color: #666; }
.left { text-align: left; }
.table-wrap { width: 100%; max-width: 100%; overflow-x: auto; overflow-y: visible; }
.table-wrap :deep(table) { width: 100%; border-collapse: collapse; table-layout: fixed; min-width: 660px; }
.table-wrap :deep(col.select-col) { width: 48px; }
.table-wrap :deep(col.name-col) { width: 130px; }
.table-wrap :deep(col.unit-col) { width: 100px; }
.table-wrap :deep(col.stock-col) { width: 170px; }
.table-wrap :deep(col.status-col) { width: 110px; }
.table-wrap :deep(th), .table-wrap :deep(td) { border-bottom: 1px solid #eee8df; padding: 10px 8px; text-align: left; font-size: 14px; vertical-align: top; }
.table-wrap :deep(th) { background: #fbfaf8; position: sticky; top: 0; }
.table-wrap :deep(.materials-table th), .table-wrap :deep(.materials-table td) { white-space: nowrap; }
.table-wrap :deep(.materials-table th:nth-child(2)), .table-wrap :deep(.materials-table td:nth-child(2)) { width: 130px; max-width: 130px; }
.table-wrap :deep(.materials-table td:nth-child(2)) { padding-left: var(--classification-item-indent, 8px); }
.table-wrap :deep(tbody tr.active) { background: #f3f7fb; }
.table-wrap :deep(td strong), .table-wrap :deep(td small) { display: block; }
.table-wrap :deep(td small) { color: #666; margin-top: 4px; max-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.table-wrap :deep(.pill) { display: inline-flex; min-height: 24px; align-items: center; border: 1px solid #d8d0c7; border-radius: 999px; padding: 2px 8px; background: #fbfaf8; font-size: 12px; }
.table-wrap :deep(.ok-pill) { border-color: #cce7d2; background: #effaf2; color: #1f6a3f; }
.table-wrap :deep(.muted-pill) { color: #777; }
.detail-form { display: grid; gap: 12px; }
.form-section { border: 1px solid #eee8df; border-radius: 8px; padding: 12px; }
.section-title { font-size: 14px; font-weight: 700; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 10px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select, textarea { width: 100%; min-height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
input:disabled, select:disabled { background: #f6f4f1; color: #555; }
textarea { resize: vertical; line-height: 1.45; }
.wide { grid-column: 1 / -1; }
button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; background: #fff; }
button:disabled { cursor: not-allowed; opacity: .55; }
.material-name-button { width: 100%; min-height: 0; display: block; border: 0; padding: 0; background: transparent; color: #1f1f1f; font-weight: 700; line-height: 1.35; white-space: normal; overflow-wrap: anywhere; text-align: left; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.danger { border-color: #b23b3b; background: #fff; color: #9b2020; }
.subtle { min-height: 32px; }
.form-actions { justify-content: flex-end; }
.boolean-field input[type="checkbox"] { width: auto; min-height: 0; }
.bom-link-group { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-top: 8px; }
.manufacturing-status { display: inline-flex; align-items: center; min-height: 28px; border: 1px solid #d8d0c7; border-radius: 999px; padding: 3px 9px; font-size: 12px; }
.manufacturing-status-ready { border-color: #cce7d2; background: #effaf2; color: #1f6a3f; }
.manufacturing-status-missing { background: #f6f4f1; color: #666; }
.muted { color: #666; text-align: center; }
.empty { padding: 22px; border: 1px dashed #d8d0c7; border-radius: 8px; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
.drawer-mask { position: fixed; inset: 0; z-index: 80; display: flex; justify-content: flex-end; background: rgba(0,0,0,.28); }
.drawer { width: min(560px, 100vw); height: 100%; overflow: auto; border-left: 1px solid #d8d0c7; background: #fff; padding: 18px; box-shadow: -8px 0 28px rgba(0,0,0,.16); }
.drawer-head { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; margin-bottom: 14px; }
.drawer-head h3 { margin: 0; font-size: 18px; }
.drawer-head p { margin: 4px 0 0; color: #666; font-size: 12px; }
.material-detail-drawer { width: min(760px, 100vw); }
.material-group-feature-drawer .feature-group-selection { grid-template-columns: 1fr; align-items: stretch; }
.material-group-feature-drawer .feature-group-selection-options { display: grid; gap: 8px; }
.material-group-feature-drawer .feature-group-selection-actions { justify-content: flex-end; padding-top: 8px; border-top: 1px solid #d9e2ec; }
.modal-mask { position: fixed; inset: 0; z-index: 50; display: grid; place-items: center; padding: 18px; background: rgba(0,0,0,.28); }
.stock-backfill-mask { z-index: 90; }
.modal-panel { width: min(640px, 100%); max-height: calc(100vh - 36px); overflow: auto; border-radius: 8px; background: #fff; border: 1px solid #d8d0c7; padding: 16px; box-shadow: 0 18px 50px rgba(0,0,0,.18); }
.modal-head { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; margin-bottom: 14px; }
.modal-head h3 { margin: 0; font-size: 18px; }
.modal-head p { margin: 4px 0 0; color: #666; font-size: 12px; }
@media (max-width: 1100px) { .feature-group-selection { grid-template-columns: 1fr; } .feature-group-selection-actions { justify-content: flex-start; } }
@media (max-width: 760px) { .page { padding: 12px; } .form-grid { grid-template-columns: 1fr; } .wide { grid-column: span 1; } }
</style>
