<template>
  <div class="page">
    <section class="panel compact-head">
      <div class="panel-head">
        <div>
          <h2>物料档案</h2>
          <p>按分类维护原料、包材和其他物料；单位来自全局单位字典，库存数量通过库存补录或库存调整修正。</p>
        </div>
        <button class="secondary" type="button" @click="loadAll" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <div class="materials-layout">
      <section class="panel material-list-panel">
        <div class="panel-title">物料列表</div>
        <div class="material-list-toolbar">
          <label>
            <span>搜索</span>
            <input v-model.trim="q" placeholder="名称/编码/批次号" @keyup.enter="loadMaterials" />
          </label>
          <label>
            <span>状态</span>
            <select v-model="activeFilter" @change="loadMaterials">
              <option value="active">启用</option>
              <option value="inactive">失效</option>
              <option value="all">全部</option>
            </select>
          </label>
          <button class="primary" type="button" @click="loadMaterials" :disabled="loading">查询</button>
          <span class="spacer"></span>
          <button class="primary" type="button" @click="createMaterial">新建物料</button>
          <button class="danger" type="button" @click="deprecateSelectedMaterials" :disabled="!selectedMaterialIDs.length || loading">批量失效</button>
        </div>

        <BusinessGroupControls
          v-model="selectedMaterialGroupTemplateID"
          v-model:move-model-value="selectedMaterialMoveGroupItemID"
          class="classification-view-toolbar material-business-group-controls"
          data-pr513-material-business-groups
          :template-options="materialGroupTemplateOptions"
          :move-options="materialGroupItemOptions"
          :selected-template="selectedMaterialGroupTemplate"
          :selected-count="selectedMaterialRowsForMove.length"
          :can-move="canMoveSelectedMaterialsToBusinessGroup"
          :can-select-target="canSelectMaterialMoveTarget"
          :loading="loading"
          @manage="openMaterialBusinessGroupManagement"
          @move="moveSelectedMaterialsToGroup" />

        <div class="material-section-list">
          <div v-for="section in materialDisplayGroups" :key="section.key" class="material-section">
            <button class="section-toggle" type="button" @click="toggleSection(section.key)">
              <strong :style="businessGroupHeaderIndentStyle(section)" :title="section.path_label || section.label">{{ section.label }}</strong><span>{{ section.rows.length }} 个</span>
            </button>
            <MaterialRowsTable
              v-if="!collapsedSections[section.key]"
              :rows="section.rows"
              :row-style="businessGroupItemIndentStyle(section)"
              :selected="selected"
              :selected-ids="selectedMaterialIDs"
              :all-selected="areRowsSelected(section.rows)"
              @toggle="toggleMaterialSelection"
              @toggle-all="toggleMaterialRows(section.rows)"
              @select="(row) => selectMaterial(row)" />
          </div>
        </div>
      </section>

      <section class="panel material-detail-panel">
        <div class="detail-head">
          <div>
            <div class="panel-title">物料详情</div>
            <p v-if="selected || draftMode">新建、编辑、失效和分类移动都会写操作日志。</p>
          </div>
          <div class="actions" v-if="selected || draftMode">
            <button v-if="!draftMode" class="secondary" type="button" @click="openStockBackfill" :disabled="loading">库存补录</button>
          </div>
        </div>

        <div v-if="!draft" class="empty muted">请选择左侧物料，或点击“新建物料”。</div>

        <form v-else class="detail-form" @submit.prevent="saveMaterial">
          <section class="form-section">
            <div class="section-title">基础信息</div>
            <div class="form-grid">
              <label><span>编码</span><input v-model.trim="draft.code" /></label>
              <label><span>名称</span><input v-model.trim="draft.name" /></label>
              <label>
                <span>库存单位（全局单位字典）</span>
                <select v-model="draft.unit">
                  <option v-for="unit in unitOptions" :key="unit.code" :value="unit.code">{{ unit.label || unit.name || unit.code }}</option>
                </select>
              </label>
              <label><span>批次号</span><input v-model.trim="draft.batch_no" /></label>
              <label><span>采购价</span><input type="number" min="0" step="0.01" v-model.number="draft.purchase_price" /></label>
              <label><span>更新时间</span><input :value="draft.updated_at || '-'" disabled /></label>
            </div>
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
      </section>
    </div>

    <div v-if="stockBackfill.open" class="modal-mask" @click.self="closeStockBackfill">
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
import { computed, defineComponent, h, onMounted, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import BusinessGroupControls from '../components/BusinessGroupControls.vue'
import {
  businessGroupControlOptions,
  businessGroupHeaderIndentStyle,
  businessGroupItemIndentStyle,
  businessGroupMoveAssignmentPayload,
  groupRowsByBusinessGroupTemplate,
  preferredBusinessGroupTemplateID,
} from '../lib/business-grouping'

const MATERIAL_CATALOG_USAGE = 'material_catalog'
const MATERIAL_OBJECT_KEY = 'material'

const MaterialRowsTable = defineComponent({
  props: {
    rows: { type: Array, default: () => [] },
    selected: { type: Object, default: null },
    selectedIds: { type: Array, default: () => [] },
    allSelected: { type: Boolean, default: false },
    rowStyle: { type: Object, default: () => ({}) },
  },
  emits: ['toggle', 'toggle-all', 'select'],
  setup(props, { emit }) {
    return () => h('div', { class: 'table-wrap' }, [
      h('table', { class: 'materials-table' }, [
        h('colgroup', [
          h('col', { class: 'select-col' }),
          h('col', { class: 'name-col' }),
          h('col', { class: 'unit-col' }),
          h('col', { class: 'stock-col' }),
          h('col', { class: 'status-col' }),
        ]),
        h('thead', [h('tr', [
          h('th', [h('input', {
            type: 'checkbox',
            title: '全选物料',
            'aria-label': '全选物料',
            checked: props.allSelected,
            disabled: !props.rows.length,
            onClick: (event) => event.stopPropagation(),
            onChange: () => emit('toggle-all'),
          })]),
          h('th', '物料名称'),
          h('th', '单位'),
          h('th', '库存数量'),
          h('th', '状态'),
        ])]),
        h('tbody', props.rows.length
          ? props.rows.map((row) => h('tr', {
              key: row.id,
              class: { active: props.selected?.id === row.id },
              style: props.rowStyle,
              onClick: () => emit('select', row),
            }, [
              h('td', [h('input', {
                type: 'checkbox',
                checked: props.selectedIds.includes(row.id),
                onClick: (event) => event.stopPropagation(),
                onChange: () => emit('toggle', row.id),
              })]),
              h('td', { class: 'material-name-cell' }, [h('strong', row.name), h('small', row.code || '-')]),
              h('td', unitDisplay(row.unit)),
              h('td', `${stockQty(row)} ${unitDisplay(row.unit)}`),
              h('td', [h('span', { class: row.deprecated_at ? 'pill muted-pill' : 'pill ok-pill' }, row.deprecated_at ? '失效' : '启用')]),
            ]))
          : [h('tr', [h('td', { colspan: 5, class: 'muted' }, '暂无物料')])]),
      ]),
    ])
  },
})

const rows = ref([])
const materialBusinessGroups = ref([])
const materialBusinessGroupAssignments = ref([])
const industryFieldTemplates = ref([])
const productUnitDefinitions = ref([])
const q = ref('')
const activeFilter = ref('active')
const loading = ref(false)
const error = ref('')
const ok = ref('')
const selected = ref(null)
const draft = ref(null)
const draftMode = ref(false)
const selectedMaterialIDs = ref([])
const selectedMaterialGroupTemplateID = ref(0)
const selectedMaterialMoveGroupItemID = ref(0)
const collapsedSections = ref({})
const stockBackfill = ref({ open: false, target_qty: 0, reason: '' })

const activeIndustryTemplates = computed(() => industryFieldTemplates.value.filter((tpl) => !tpl.deactivated_at && tpl.active !== false))
const materialBusinessGroupControls = computed(() => businessGroupControlOptions(materialBusinessGroups.value, {
  selectedTemplateID: selectedMaterialGroupTemplateID.value,
  usageKey: MATERIAL_CATALOG_USAGE,
}))
const materialGroupTemplateOptions = computed(() => materialBusinessGroupControls.value.templateOptions)
const selectedMaterialGroupTemplate = computed(() => materialBusinessGroupControls.value.selectedTemplate)
const materialGroupItemOptions = computed(() => materialBusinessGroupControls.value.moveOptions)
const materialDisplayGroups = computed(() => groupRowsByBusinessGroupTemplate(rows.value, {
  template: selectedMaterialGroupTemplate.value,
  assignments: materialBusinessGroupAssignments.value,
  usageKey: MATERIAL_CATALOG_USAGE,
  objectKey: MATERIAL_OBJECT_KEY,
  objectIDForRow: (row) => Number(row.id || 0),
}))
const selectedMaterialRowsForMove = computed(() => {
  const selectedIds = new Set(selectedMaterialIDs.value.map((id) => Number(id || 0)).filter(Boolean))
  return rows.value.filter((row) => selectedIds.has(Number(row.id || 0)))
})
const canMoveSelectedMaterialsToBusinessGroup = computed(() => {
  if (!selectedMaterialGroupTemplate.value || !selectedMaterialRowsForMove.value.length) return false
  const targetGroupItemID = Number(selectedMaterialMoveGroupItemID.value || 0)
  const targetOption = targetGroupItemID > 0 ? materialGroupOptionByItemID(targetGroupItemID) : null
  if (targetGroupItemID > 0 && !targetOption) return false
  return selectedMaterialRowsForMove.value.some((row) => {
    if (!targetOption) return materialBusinessGroupID(row) > 0 || materialBusinessGroupItemID(row) > 0
    return materialBusinessGroupID(row) !== Number(targetOption.group_id || 0) || materialBusinessGroupItemID(row) !== Number(targetOption.group_item_id || 0)
  })
})
const canSelectMaterialMoveTarget = computed(() => Boolean(selectedMaterialGroupTemplate.value && selectedMaterialRowsForMove.value.length))
const unitOptions = computed(() => {
  const rows = productUnitDefinitions.value.filter((row) => row.active !== false)
  if (rows.length) return rows
  return [
    { code: 'g', name: 'g', label: 'g' },
    { code: 'kg', name: 'kg', label: 'kg' },
    { code: 'unit', name: '个', label: '个' },
  ]
})
const selectedIndustryTemplate = computed(() => activeIndustryTemplates.value.find((tpl) => tpl.id === Number(draft.value?.industry_field_template_id || 0)) || null)
const selectedIndustryTemplateFields = computed(() => (selectedIndustryTemplate.value?.fields || []).slice().sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0)))
const selectedUnitLabel = computed(() => unitDisplay(selected.value?.unit || ''))
const materialIndustryFields = computed(() => draft.value?.industry_fields || [])

function normalizeRow(row) {
  const fields = row.IndustryFields || row.industry_fields || []
  return {
    id: Number(row.ID ?? row.id ?? 0),
    code: row.Code ?? row.code ?? '',
    name: row.Name ?? row.name ?? '',
    kind: row.Kind ?? row.kind ?? 'other',
    unit: row.Unit ?? row.unit ?? 'g',
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
    active: row.active ?? row.Active ?? true,
  }
}

function cloneDraft(row) {
  return JSON.parse(JSON.stringify(row))
}

function blankDraft() {
  const unit = unitOptions.value[0]?.code || 'g'
  return {
    id: 0,
    code: nextMaterialCode(),
    name: '',
    kind: 'other',
    unit,
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
  await Promise.all([loadOptions(), loadMaterialBusinessGroups(), loadMaterialBusinessGroupAssignments(), loadMaterials()])
}

async function loadOptions() {
  const [settings, industry] = await Promise.all([
    apiGet('/api/product-settings'),
    apiGet('/api/industry-field-templates'),
  ])
  productUnitDefinitions.value = (settings.product_unit_definitions || []).map(normalizeUnit).filter((row) => row.code)
  industryFieldTemplates.value = (industry.rows || []).map(normalizeTemplate)
}

async function loadMaterialBusinessGroups() {
  const data = await apiGet('/api/business-groups')
  materialBusinessGroups.value = Array.isArray(data?.rows) ? data.rows : (Array.isArray(data) ? data : [])
  const nextTemplateID = preferredMaterialGroupTemplateID()
  if (nextTemplateID && nextTemplateID !== Number(selectedMaterialGroupTemplateID.value || 0)) {
    selectedMaterialGroupTemplateID.value = nextTemplateID
  }
}

async function loadMaterialBusinessGroupAssignments() {
  const url = new URL('/api/business-group-assignments', window.location.origin)
  url.searchParams.set('usage_key', MATERIAL_CATALOG_USAGE)
  url.searchParams.set('object_key', MATERIAL_OBJECT_KEY)
  const data = await apiGet(`${url.pathname}${url.search}`)
  materialBusinessGroupAssignments.value = Array.isArray(data?.rows) ? data.rows : (Array.isArray(data?.assignments) ? data.assignments : [])
}

async function loadMaterials() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/materials', window.location.origin)
    url.searchParams.set('limit', '500')
    url.searchParams.set('active', activeFilter.value)
    if (q.value) url.searchParams.set('q', q.value)
    const data = await apiGet(`${url.pathname}${url.search}`)
    rows.value = (data.rows || []).map(normalizeRow)
    selectedMaterialIDs.value = selectedMaterialIDs.value.filter((id) => rows.value.some((row) => row.id === id))
    if (selected.value?.id) {
      const next = rows.value.find((row) => row.id === selected.value.id)
      if (next) selectMaterial(next, { quiet: true })
    }
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function createMaterial() {
  selected.value = null
  draft.value = blankDraft()
  draftMode.value = true
  error.value = ''
  ok.value = ''
}

function selectMaterial(row, options = {}) {
  selected.value = row
  draft.value = cloneDraft(row)
  draftMode.value = false
  closeStockBackfill()
  if (!options.quiet) {
    error.value = ''
    ok.value = ''
  }
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

function toggleSection(key) {
  collapsedSections.value = { ...collapsedSections.value, [key]: !collapsedSections.value[key] }
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

function materialGroupOptionByItemID(groupItemID) {
  const id = Number(groupItemID || 0)
  return materialGroupItemOptions.value.find((option) => Number(option.group_item_id || 0) === id) || null
}

function preferredMaterialGroupTemplateID() {
  return preferredBusinessGroupTemplateID(materialBusinessGroups.value, {
    selectedTemplateID: selectedMaterialGroupTemplateID.value,
    usageKey: MATERIAL_CATALOG_USAGE,
    preferredNames: ['物料档案分组', '物料分类'],
    preferredNameIncludes: ['物料', '材料', '原料'],
  })
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

async function moveSelectedMaterialsToGroup() {
  const targetGroupItemID = Number(selectedMaterialMoveGroupItemID.value || 0)
  const targetOption = targetGroupItemID > 0 ? materialGroupOptionByItemID(targetGroupItemID) : null
  if (targetGroupItemID > 0 && !targetOption) return
  const materialsToMove = selectedMaterialRowsForMove.value.filter((row) => {
    if (!targetOption) return materialBusinessGroupID(row) > 0 || materialBusinessGroupItemID(row) > 0
    return materialBusinessGroupID(row) !== Number(targetOption.group_id || 0) || materialBusinessGroupItemID(row) !== Number(targetOption.group_item_id || 0)
  })
  if (!materialsToMove.length) {
    error.value = '请先勾选物料'
    return
  }
  await mutate(async () => {
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
    ok.value = `已移动 ${materialsToMove.length} 个物料到分类`
    selectedMaterialIDs.value = []
    selectedMaterialMoveGroupItemID.value = 0
    await loadMaterialBusinessGroupAssignments()
  })
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

function payloadFromDraft() {
  const sourceStock = draftMode.value ? { onhand_g: 0, onhand_units: 0 } : (selected.value || draft.value)
  return {
    code: draft.value.code,
    name: draft.value.name,
    kind: draft.value.kind || 'other',
    unit: draft.value.unit,
    batch_no: draft.value.batch_no,
    purchase_price: Number(draft.value.purchase_price || 0),
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

watch(selectedMaterialGroupTemplateID, () => {
  selectedMaterialMoveGroupItemID.value = 0
  collapsedSections.value = {}
})

watch(materialGroupItemOptions, (options) => {
  const selected = Number(selectedMaterialMoveGroupItemID.value || 0)
  if (selected > 0 && !options.some((option) => Number(option.group_item_id || 0) === selected)) {
    selectedMaterialMoveGroupItemID.value = 0
  }
})

onMounted(() => {
  q.value = new URL(window.location.href).searchParams.get('q') || ''
  loadAll()
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.compact-head { padding: 12px 14px; }
.panel-head, .filters, .material-list-toolbar, .detail-head, .actions, .form-actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.panel-head { justify-content: space-between; margin-bottom: 10px; }
.panel-head h2 { margin: 0; font-size: 20px; }
.panel-head p, .detail-head p { margin: 4px 0 0; color: #666; font-size: 12px; }
.panel-title { font-size: 16px; font-weight: 700; margin-bottom: 10px; }
.spacer { flex: 1 1 auto; }
.materials-layout { display: grid; grid-template-columns: minmax(0, .95fr) minmax(360px, 1.05fr); gap: 14px; align-items: start; }
.material-list-panel, .material-detail-panel { min-width: 0; }
.material-list-toolbar { margin-bottom: 12px; padding-bottom: 12px; border-bottom: 1px solid #eee8df; }
.material-list-toolbar label { min-width: 150px; }
.material-list-toolbar label:first-child { flex: 1 1 220px; }
.material-list-toolbar label:nth-child(2) { flex: 0 0 130px; }
.section-toggle { background: #1f1f1f; color: #fff; }
.material-business-group-controls { margin-bottom: 12px; padding: 10px; border: 1px solid #eee8df; border-radius: 8px; background: #fbfaf8; }
.material-section-list { display: grid; gap: 10px; }
.material-section-list, .material-section { min-width: 0; }
.left { text-align: left; }
.section-toggle { width: 100%; justify-content: space-between; border: 0; display: flex; }
.section-toggle strong { padding-left: var(--classification-group-indent, 0); }
.table-wrap { width: 100%; max-width: 100%; overflow-x: auto; overflow-y: visible; }
.table-wrap :deep(table) { width: 100%; border-collapse: collapse; min-width: 920px; }
.table-wrap :deep(col.select-col) { width: 48px; }
.table-wrap :deep(col.name-col) { width: 390px; }
.table-wrap :deep(col.unit-col) { width: 100px; }
.table-wrap :deep(col.stock-col) { width: 170px; }
.table-wrap :deep(col.status-col) { width: 110px; }
.table-wrap :deep(th), .table-wrap :deep(td) { border-bottom: 1px solid #eee8df; padding: 10px 8px; text-align: left; font-size: 14px; vertical-align: top; }
.table-wrap :deep(th) { background: #fbfaf8; position: sticky; top: 0; }
.table-wrap :deep(.materials-table th), .table-wrap :deep(.materials-table td) { white-space: nowrap; }
.table-wrap :deep(.materials-table td:nth-child(2)) { padding-left: var(--classification-item-indent, 8px); }
.table-wrap :deep(.material-name-cell strong) { white-space: normal; line-height: 1.35; }
.material-list-panel :deep(tbody tr) { cursor: pointer; }
.table-wrap :deep(tbody tr.active) { background: #f3f7fb; }
.table-wrap :deep(td strong), .table-wrap :deep(td small) { display: block; }
.table-wrap :deep(td small) { color: #666; margin-top: 4px; max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.table-wrap :deep(.pill) { display: inline-flex; min-height: 24px; align-items: center; border: 1px solid #d8d0c7; border-radius: 999px; padding: 2px 8px; background: #fbfaf8; font-size: 12px; }
.table-wrap :deep(.ok-pill) { border-color: #cce7d2; background: #effaf2; color: #1f6a3f; }
.table-wrap :deep(.muted-pill) { color: #777; }
.detail-head { justify-content: space-between; align-items: flex-start; margin-bottom: 12px; }
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
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.danger { border-color: #b23b3b; background: #fff; color: #9b2020; }
.subtle { min-height: 32px; }
.form-actions { justify-content: flex-end; }
.muted { color: #666; text-align: center; }
.empty { padding: 22px; border: 1px dashed #d8d0c7; border-radius: 8px; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
.modal-mask { position: fixed; inset: 0; z-index: 50; display: grid; place-items: center; padding: 18px; background: rgba(0,0,0,.28); }
.modal-panel { width: min(640px, 100%); max-height: calc(100vh - 36px); overflow: auto; border-radius: 8px; background: #fff; border: 1px solid #d8d0c7; padding: 16px; box-shadow: 0 18px 50px rgba(0,0,0,.18); }
.modal-head { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; margin-bottom: 14px; }
.modal-head h3 { margin: 0; font-size: 18px; }
.modal-head p { margin: 4px 0 0; color: #666; font-size: 12px; }
@media (max-width: 1100px) { .materials-layout { grid-template-columns: 1fr; } }
@media (max-width: 760px) { .page { padding: 12px; } .form-grid { grid-template-columns: 1fr; } .wide { grid-column: span 1; } }
</style>
