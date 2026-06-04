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
      <div class="filters">
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
        <button class="danger" type="button" @click="deprecateSelectedMaterial" :disabled="!selected || draftMode || loading">失效物料</button>
      </div>
    </section>

    <div class="materials-layout">
      <section class="panel material-list-panel">
        <div class="classification-row">
          <div class="tabs">
            <button type="button" :class="{ active: activeTab === 'all' }" @click="activeTab = 'all'">全部分类</button>
            <button type="button" :class="{ active: activeTab === 'unclassified' }" @click="activeTab = 'unclassified'">未分类</button>
            <button
              v-for="group in classificationGroups"
              :key="group.id"
              type="button"
              :class="{ active: activeTab === groupTabKey(group.id) }"
              @click="activeTab = groupTabKey(group.id)">
              {{ group.name }}
            </button>
          </div>
          <div class="group-create">
            <input v-model.trim="newGroupName" placeholder="新大类名称" @keyup.enter="createClassificationGroup" />
            <button class="secondary" type="button" @click="createClassificationGroup" :disabled="!newGroupName">增加分类</button>
          </div>
        </div>

        <div class="move-row">
          <template v-if="activeGroup">
            <input v-model.trim="newCategoryName" placeholder="新小类名称" @keyup.enter="createGroupCategory" />
            <button class="secondary" type="button" @click="createGroupCategory" :disabled="!newCategoryName">新增小分类</button>
            <select v-model.number="moveCategoryID" :disabled="!selectedMaterialIDs.length">
              <option :value="0">未分类</option>
              <option v-for="category in activeGroup.categories" :key="category.id" :value="category.id">{{ category.name }}</option>
            </select>
            <button class="secondary" type="button" @click="moveSelectedToCategory" :disabled="!selectedMaterialIDs.length">移动到小分类</button>
          </template>
          <template v-else>
            <select v-model.number="moveGroupID" :disabled="!selectedMaterialIDs.length">
              <option :value="0">未分类</option>
              <option v-for="group in classificationGroups" :key="group.id" :value="group.id">{{ group.name }}</option>
            </select>
            <button class="secondary" type="button" @click="moveSelectedToGroup" :disabled="!selectedMaterialIDs.length">移动到分类</button>
          </template>
          <span class="muted">已选 {{ selectedMaterialIDs.length }} 个物料</span>
        </div>

        <div v-if="activeGroup" class="inner-categories">
          <div class="inner-category-card">
            <strong>组内分类</strong>
            <p class="muted left">删除小分类后，物料回到该大类下的未分类。</p>
          </div>
          <div v-for="category in activeGroup.categories" :key="category.id" class="inner-category-card">
            <input v-model.trim="category.name" @change="saveGroupCategory(category)" />
            <input type="number" v-model.number="category.sort_order" @change="saveGroupCategory(category)" />
            <button class="danger subtle" type="button" @click="deleteGroupCategory(category)">删除</button>
          </div>
        </div>

        <div class="panel-title">物料列表</div>
        <template v-if="activeGroup">
          <div v-for="section in groupedMaterialSections" :key="section.key" class="material-section">
            <button class="section-toggle" type="button" @click="toggleSection(section.key)">
              <strong>{{ section.name }}</strong><span>{{ section.rows.length }} 个</span>
            </button>
            <MaterialRowsTable
              v-if="!collapsedSections[section.key]"
              :rows="section.rows"
              :selected="selected"
              :selected-ids="selectedMaterialIDs"
              @toggle="toggleMaterialSelection"
              @select="(row) => selectMaterial(row)" />
          </div>
        </template>
        <MaterialRowsTable
          v-else
          :rows="visibleRows"
          :selected="selected"
          :selected-ids="selectedMaterialIDs"
          @toggle="toggleMaterialSelection"
          @select="(row) => selectMaterial(row)" />
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
                <span>单位（全局单位字典）</span>
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
              <label><span>库存数量（物料单位）</span><input type="number" :value="stockQty(draft)" disabled /></label>
              <label><span>警戒线（物料单位）</span><input type="number" min="0" step="0.001" v-model.number="draft.min_level_qty" /></label>
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
import { computed, defineComponent, h, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const MaterialRowsTable = defineComponent({
  props: {
    rows: { type: Array, default: () => [] },
    selected: { type: Object, default: null },
    selectedIds: { type: Array, default: () => [] },
  },
  emits: ['toggle', 'select'],
  setup(props, { emit }) {
    return () => h('div', { class: 'table-wrap' }, [
      h('table', [
        h('thead', [h('tr', [
          h('th', ''),
          h('th', '物料类型'),
          h('th', '物料名称'),
          h('th', '单位'),
          h('th', '库存数量'),
          h('th', '状态'),
        ])]),
        h('tbody', props.rows.length
          ? props.rows.map((row) => h('tr', {
              key: row.id,
              class: { active: props.selected?.id === row.id },
              onClick: () => emit('select', row),
            }, [
              h('td', [h('input', {
                type: 'checkbox',
                checked: props.selectedIds.includes(row.id),
                onClick: (event) => event.stopPropagation(),
                onChange: () => emit('toggle', row.id),
              })]),
              h('td', [h('span', { class: 'pill' }, materialTypeLabel(row))]),
              h('td', [h('strong', row.name), h('small', row.code || '-')]),
              h('td', unitDisplay(row.unit)),
              h('td', `${stockQty(row)} ${unitDisplay(row.unit)}`),
              h('td', [h('span', { class: row.deprecated_at ? 'pill muted-pill' : 'pill ok-pill' }, row.deprecated_at ? '失效' : '启用')]),
            ]))
          : [h('tr', [h('td', { colspan: 6, class: 'muted' }, '暂无物料')])]),
      ]),
    ])
  },
})

const rows = ref([])
const classificationGroups = ref([])
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
const activeTab = ref('all')
const newGroupName = ref('')
const newCategoryName = ref('')
const moveGroupID = ref(0)
const moveCategoryID = ref(0)
const collapsedSections = ref({})
const stockBackfill = ref({ open: false, target_qty: 0, reason: '' })

const activeIndustryTemplates = computed(() => industryFieldTemplates.value.filter((tpl) => !tpl.deactivated_at && tpl.active !== false))
const activeGroup = computed(() => {
  const match = /^group:(\d+)$/.exec(activeTab.value)
  if (!match) return null
  const id = Number(match[1])
  return classificationGroups.value.find((group) => group.id === id) || null
})
const visibleRows = computed(() => {
  if (activeTab.value === 'unclassified') return rows.value.filter((row) => !row.classification_group_id)
  if (activeGroup.value) return rows.value.filter((row) => row.classification_group_id === activeGroup.value.id)
  return rows.value
})
const groupedMaterialSections = computed(() => {
  if (!activeGroup.value) return []
  const sections = [
    { key: `group:${activeGroup.value.id}:unclassified`, name: '未分类', rows: visibleRows.value.filter((row) => !row.classification_category_id) },
  ]
  for (const category of activeGroup.value.categories || []) {
    sections.push({
      key: `category:${category.id}`,
      name: category.name,
      rows: visibleRows.value.filter((row) => row.classification_category_id === category.id),
    })
  }
  return sections
})
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

function groupTabKey(id) {
  return `group:${id}`
}

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

function normalizeGroup(row) {
  return {
    id: Number(row.ID ?? row.id ?? 0),
    name: row.Name ?? row.name ?? '',
    sort_order: Number(row.SortOrder ?? row.sort_order ?? 100),
    categories: (row.Categories || row.categories || []).map((category) => ({
      id: Number(category.ID ?? category.id ?? 0),
      group_id: Number(category.GroupID ?? category.group_id ?? 0),
      name: category.Name ?? category.name ?? '',
      sort_order: Number(category.SortOrder ?? category.sort_order ?? 100),
    })),
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
  await Promise.all([loadOptions(), loadClassificationGroups(), loadMaterials()])
}

async function loadOptions() {
  const [settings, industry] = await Promise.all([
    apiGet('/api/product-settings'),
    apiGet('/api/industry-field-templates'),
  ])
  productUnitDefinitions.value = (settings.product_unit_definitions || []).map(normalizeUnit).filter((row) => row.code)
  industryFieldTemplates.value = (industry.rows || []).map(normalizeTemplate)
}

async function loadClassificationGroups() {
  const data = await apiGet('/api/material-classification-groups')
  classificationGroups.value = (data.rows || []).map(normalizeGroup)
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

function toggleSection(key) {
  collapsedSections.value = { ...collapsedSections.value, [key]: !collapsedSections.value[key] }
}

async function createClassificationGroup() {
  if (!newGroupName.value) return
  await mutate(async () => {
    const group = await apiSend('/api/material-classification-groups', { body: { name: newGroupName.value, sort_order: 100 } })
    newGroupName.value = ''
    await loadClassificationGroups()
    activeTab.value = groupTabKey(Number(group.id || group.ID || 0))
    ok.value = '已增加分类'
  })
}

async function createGroupCategory() {
  if (!activeGroup.value || !newCategoryName.value) return
  await mutate(async () => {
    await apiSend(`/api/material-classification-groups/${activeGroup.value.id}/categories`, { body: { name: newCategoryName.value, sort_order: 100 } })
    newCategoryName.value = ''
    await loadClassificationGroups()
    ok.value = '已新增小分类'
  })
}

async function saveGroupCategory(category) {
  await mutate(async () => {
    await apiSend(`/api/material-classification-group-categories/${category.id}`, {
      method: 'PUT',
      body: { group_id: activeGroup.value?.id || category.group_id, name: category.name, sort_order: Number(category.sort_order || 100) },
    })
    await loadClassificationGroups()
    ok.value = '已保存小分类'
  })
}

async function deleteGroupCategory(category) {
  if (!window.confirm(`删除小分类「${category.name}」？该小分类下物料会回到未分类。`)) return
  await mutate(async () => {
    await apiSend(`/api/material-classification-group-categories/${category.id}`, { method: 'DELETE' })
    await Promise.all([loadClassificationGroups(), loadMaterials()])
    ok.value = '已删除小分类'
  })
}

async function moveSelectedToGroup() {
  await assignSelected(moveGroupID.value, 0)
}

async function moveSelectedToCategory() {
  if (!activeGroup.value) return
  await assignSelected(activeGroup.value.id, moveCategoryID.value)
}

async function assignSelected(groupID, categoryID) {
  if (!selectedMaterialIDs.value.length) {
    error.value = '请先勾选物料'
    return
  }
  const targetName = categoryID ? (activeGroup.value?.categories || []).find((row) => row.id === categoryID)?.name : (groupID ? '未分类' : '未分类')
  if (!window.confirm(`移动 ${selectedMaterialIDs.value.length} 个物料到「${targetName || '未分类'}」？`)) return
  await mutate(async () => {
    await apiSend('/api/material-classification-assignments', {
      body: { material_ids: selectedMaterialIDs.value, group_id: Number(groupID || 0), category_id: Number(categoryID || 0) },
    })
    selectedMaterialIDs.value = []
    await loadMaterials()
    ok.value = '已移动分类'
  })
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

async function deprecateSelectedMaterial() {
  if (!selected.value || draftMode.value) return
  if (!window.confirm(`失效物料：${selected.value.name}？`)) return
  await mutate(async () => {
    const data = await apiSend(`/api/materials/${selected.value.id}/deprecate`)
    ok.value = `已失效：${data.name || selected.value.name}`
    selected.value = null
    draft.value = null
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

function materialTypeLabel(row) {
  if (row.classification_group_name) {
    return row.classification_category_name ? `${row.classification_group_name} / ${row.classification_category_name}` : `${row.classification_group_name} / 未分类`
  }
  return '未分类'
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
.panel-head, .filters, .detail-head, .actions, .form-actions, .classification-row, .move-row, .inner-category-card { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.panel-head { justify-content: space-between; margin-bottom: 10px; }
.panel-head h2 { margin: 0; font-size: 20px; }
.panel-head p, .detail-head p { margin: 4px 0 0; color: #666; font-size: 12px; }
.panel-title { font-size: 16px; font-weight: 700; margin-bottom: 10px; }
.spacer { flex: 1 1 auto; }
.materials-layout { display: grid; grid-template-columns: minmax(480px, .9fr) minmax(520px, 1.1fr); gap: 14px; align-items: start; }
.classification-row { justify-content: space-between; margin-bottom: 10px; }
.tabs { display: flex; gap: 8px; flex-wrap: wrap; }
.tabs button.active, .section-toggle { background: #1f1f1f; color: #fff; }
.group-create, .move-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.group-create input, .move-row input { width: 150px; }
.move-row { border: 1px solid #eee8df; border-radius: 8px; padding: 10px; margin-bottom: 10px; }
.inner-categories { display: grid; gap: 8px; margin-bottom: 12px; }
.inner-category-card { border: 1px solid #eee8df; border-radius: 8px; padding: 8px; }
.inner-category-card input[type="number"] { width: 90px; }
.left { text-align: left; }
.material-section { margin-bottom: 10px; }
.section-toggle { width: 100%; justify-content: space-between; border: 0; display: flex; }
.table-wrap { overflow: auto; }
table { width: 100%; border-collapse: collapse; min-width: 680px; }
th, td { border-bottom: 1px solid #eee8df; padding: 10px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
.material-list-panel tbody tr { cursor: pointer; }
tbody tr.active { background: #f3f7fb; }
td strong, td small { display: block; }
td small { color: #666; margin-top: 4px; max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pill { display: inline-flex; min-height: 24px; align-items: center; border: 1px solid #d8d0c7; border-radius: 999px; padding: 2px 8px; background: #fbfaf8; font-size: 12px; }
.ok-pill { border-color: #cce7d2; background: #effaf2; color: #1f6a3f; }
.muted-pill { color: #777; }
.detail-head { justify-content: space-between; align-items: flex-start; margin-bottom: 12px; }
.detail-form { display: grid; gap: 12px; }
.form-section { border: 1px solid #eee8df; border-radius: 8px; padding: 12px; }
.section-title { font-size: 14px; font-weight: 700; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select, textarea { width: 100%; min-height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
input:disabled, select:disabled { background: #f6f4f1; color: #555; }
textarea { resize: vertical; line-height: 1.45; }
.wide { grid-column: span 2; }
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
