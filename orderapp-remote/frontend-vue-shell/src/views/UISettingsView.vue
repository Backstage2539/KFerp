<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>系统设置</h2>
          <p>维护 ERP 全局入口和跨商品、生产、库存共用的基础资料。</p>
        </div>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>

      <label class="toggle-row">
        <input v-model="form.hide_customer_account_fulfillment" type="checkbox" />
        <span>
          <strong>客户账户模式隐藏履约运营台</strong>
          <small>开启后，客户账户模式不显示内部履约运营台入口；页面和接口仍保留。</small>
        </span>
      </label>

      <div class="actions">
        <button class="primary" type="button" @click="save" :disabled="saving || loading">
          {{ saving ? '保存中' : '保存设置' }}
        </button>
        <span v-if="ok" class="ok">{{ ok }}</span>
        <span v-if="error" class="error">{{ error }}</span>
      </div>
    </section>

    <section class="panel group-template-panel" data-section-mode="groupTemplates">
      <div class="panel-head">
        <div>
          <h2>分组模板</h2>
          <p>这里只维护模板名、大类、小类；商品、BOM、仓库在各自页面选择模板后完成归类。</p>
        </div>
        <button class="secondary compact-action" type="button" @click="resetGroupTemplateForm">新增分组模板</button>
      </div>

      <div class="group-template-layout">
        <aside class="template-list" aria-label="分组模板列表">
          <button
            v-for="template in activeGroupTemplates"
            :key="template.id"
            class="template-chip"
            :class="{ active: Number(template.id || 0) === selectedGroupTemplateID, inactive: template.active === false }"
            type="button"
            @click="selectGroupTemplate(template)">
            <strong>{{ groupTemplateName(template) }}</strong>
            <small>{{ template.active === false ? '停用' : '启用' }} · {{ businessGroupItemsTree(template.items || []).length }} 个大类</small>
          </button>
          <p v-if="!activeGroupTemplates.length" class="muted">暂无模板，先新增一个分组模板，例如“咖啡挂耳”。</p>
        </aside>

        <form class="group-template-form" @submit.prevent="saveGroupTemplate">
          <div class="unit-form-head">
            <div>
              <strong>{{ groupTemplateForm.id ? '编辑分组模板' : '新增分组模板' }}</strong>
              <small>模板是系统基础资料，不在这里维护对象。</small>
            </div>
          </div>
          <label>
            <span>模板名</span>
            <input v-model.trim="groupTemplateForm.name" placeholder="咖啡挂耳" />
          </label>
          <label>
            <span>模板编码</span>
            <input v-model.trim="groupTemplateForm.code" placeholder="drip_bag" />
          </label>
          <label>
            <span>排序</span>
            <input v-model.number="groupTemplateForm.sort_order" type="number" min="0" step="1" />
          </label>
          <label class="checkline">
            <input v-model="groupTemplateForm.active" type="checkbox" />
            <span>启用</span>
          </label>
          <label class="wide-field">
            <span>备注</span>
            <input v-model.trim="groupTemplateForm.remark" placeholder="用于商品、BOM、库存选择分类" />
          </label>
          <div class="actions unit-actions">
            <button class="primary" type="submit" :disabled="groupTemplateSaving || loading">
              {{ groupTemplateSaving ? '保存中' : (groupTemplateForm.id ? '保存模板' : '新增分组模板') }}
            </button>
          </div>
        </form>
      </div>

      <div v-if="selectedGroupTemplate" class="category-editor">
        <div class="category-editor-head">
          <div>
            <strong>{{ groupTemplateName(selectedGroupTemplate) }} 分类</strong>
            <small>大类下可维护小类；业务对象只在对应页面移动。</small>
          </div>
          <button class="secondary compact-action" type="button" @click="startGroupTemplateCategoryCreate(0)">新增大类</button>
        </div>

        <div class="category-tree">
          <article v-for="primary in selectedGroupTemplateTree" :key="primary.id" class="category-node">
            <div class="category-node-head">
              <button class="text-button category-name" type="button" @click="startGroupTemplateCategoryEdit(primary)">
                <strong>{{ primary.name }}</strong>
                <small>大类 · {{ primary.children?.length || 0 }} 个小类</small>
              </button>
              <div class="category-actions">
                <button class="secondary compact-action" type="button" @click="startGroupTemplateCategoryCreate(primary.id)">新增小类</button>
                <button class="text-button danger-text" type="button" @click="deleteGroupTemplateCategory(primary)">停用</button>
              </div>
            </div>
            <div class="category-children">
              <button
                v-for="child in primary.children"
                :key="child.id"
                class="category-child"
                type="button"
                @click="startGroupTemplateCategoryEdit(child)">
                <span>{{ child.name }}</span>
                <small>小类</small>
              </button>
            </div>
          </article>
          <p v-if="!selectedGroupTemplateTree.length" class="muted">暂无大类，先新增大类。</p>
        </div>

        <form class="category-form" @submit.prevent="saveGroupTemplateCategory">
          <div class="unit-form-head">
            <div>
              <strong>{{ groupTemplateCategoryForm.id ? '编辑分类' : (groupTemplateCategoryForm.parent_id ? '新增小类' : '新增大类') }}</strong>
              <small>{{ groupTemplateCategoryForm.parent_id ? parentCategoryName(groupTemplateCategoryForm.parent_id) : '大类' }}</small>
            </div>
            <button class="secondary compact-action" type="button" @click="resetGroupTemplateCategoryForm">清空</button>
          </div>
          <label>
            <span>分类名称</span>
            <input v-model.trim="groupTemplateCategoryForm.name" placeholder="意式拼配" />
          </label>
          <label>
            <span>上级大类</span>
            <select v-model.number="groupTemplateCategoryForm.parent_id" :disabled="Boolean(groupTemplateCategoryForm.id && originalGroupTemplateCategoryParentID === 0)">
              <option :value="0">大类</option>
              <option v-for="primary in selectedGroupTemplateTree" :key="primary.id" :value="Number(primary.id || 0)">
                {{ primary.name }}
              </option>
            </select>
          </label>
          <label>
            <span>排序</span>
            <input v-model.number="groupTemplateCategoryForm.sort_order" type="number" min="0" step="1" />
          </label>
          <label class="checkline">
            <input v-model="groupTemplateCategoryForm.active" type="checkbox" />
            <span>启用</span>
          </label>
          <label class="wide-field">
            <span>备注</span>
            <input v-model.trim="groupTemplateCategoryForm.remark" placeholder="分类备注" />
          </label>
          <div class="actions unit-actions">
            <button class="primary" type="submit" :disabled="groupTemplateSaving || loading">
              {{ groupTemplateSaving ? '保存中' : '保存分类' }}
            </button>
          </div>
        </form>
      </div>
      <p v-else class="muted">选择一个分组模板后维护大类、小类。</p>
    </section>

    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>全局单位字典</h2>
          <p>基础单位在这里维护；SKU 设置里的单位模板只引用这些单位并配置换算关系。</p>
        </div>
      </div>

      <div class="unit-layout">
        <div class="unit-list">
          <button
            v-for="unit in visibleProductUnitDefinitions"
            :key="unit.code"
            class="unit-chip"
            :class="{ inactive: unit.active === false }"
            type="button"
            @click="startGlobalUnitDefinitionEdit(unit)">
            <strong>{{ unit.name || unit.code }}</strong>
            <small>{{ unit.code }} · {{ unitTypeLabel(unit.unit_type) }} · {{ unit.allow_decimal ? '允许小数' : '整数优先' }}</small>
          </button>
          <p v-if="!visibleProductUnitDefinitions.length" class="muted">暂无单位，先新增 kg、盒、箱等基础单位。</p>
        </div>

        <form class="unit-definition-form" @submit.prevent="saveGlobalUnitDefinition">
          <div class="unit-form-head">
            <div>
              <strong>{{ unitEditingCode ? '编辑基础单位' : '新增基础单位' }}</strong>
              <small>保存后刷新列表并回到空白表单。</small>
            </div>
            <button class="secondary compact-action" type="button" @click="resetGlobalUnitDefinitionForm">新增基础单位</button>
          </div>
          <label>
            <span>单位编码</span>
            <input v-model.trim="unitForm.code" :disabled="Boolean(unitEditingCode)" placeholder="box" />
          </label>
          <label>
            <span>单位名称</span>
            <input v-model.trim="unitForm.name" placeholder="盒" />
          </label>
          <label>
            <span>单位类型</span>
            <select v-model="unitForm.unit_type">
              <option value="weight">重量</option>
              <option value="package">包装</option>
              <option value="count">数量</option>
              <option value="other">其他</option>
            </select>
          </label>
          <label class="checkline">
            <input v-model="unitForm.allow_decimal" type="checkbox" />
            <span>允许小数</span>
          </label>
          <label class="checkline">
            <input v-model="unitForm.active" type="checkbox" />
            <span>启用</span>
          </label>
          <div class="actions unit-actions">
            <button v-if="unitEditingCode" class="text-button danger-text" type="button" :disabled="unitSaving || loading" @click="deleteGlobalUnitDefinition">删除</button>
            <button class="primary" type="submit" :disabled="unitSaving || loading">
              {{ unitSaving ? '保存中' : (unitEditingCode ? '保存' : '新增') }}
            </button>
          </div>
        </form>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { fetchUISettings, saveUISettings } from '../api/ui-settings'
import { businessGroupItemsTree, buildProductUnitDefinitionPayload, isSystemDefaultBusinessGroup, visibleNonDeletedRows } from '../lib/product-settings'

const loading = ref(false)
const saving = ref(false)
const unitSaving = ref(false)
const groupTemplateSaving = ref(false)
const ok = ref('')
const error = ref('')
const productUnitDefinitions = ref([])
const groupTemplates = ref([])
const selectedGroupTemplateID = ref(0)
const originalGroupTemplateCategoryParentID = ref(0)
const visibleProductUnitDefinitions = computed(() => visibleNonDeletedRows(productUnitDefinitions.value))
const activeGroupTemplates = computed(() => groupTemplates.value
  .filter((group) => !isSystemDefaultBusinessGroup(group))
  .slice()
  .sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0) || Number(a.id || 0) - Number(b.id || 0)))
const selectedGroupTemplate = computed(() => activeGroupTemplates.value.find((group) => Number(group.id || 0) === Number(selectedGroupTemplateID.value || 0)) || null)
const selectedGroupTemplateTree = computed(() => businessGroupItemsTree((selectedGroupTemplate.value?.items || []).filter((item) => item.active !== false)))
const unitEditingCode = ref('')
const form = reactive({
  hide_customer_account_fulfillment: true,
})
const unitForm = reactive(defaultGlobalUnitDefinition())
const groupTemplateForm = reactive(defaultGroupTemplate())
const groupTemplateCategoryForm = reactive(defaultGroupTemplateCategory())

function defaultGlobalUnitDefinition(unit = {}) {
  return {
    code: unit.code || '',
    name: unit.name || '',
    unit_type: unit.unit_type || 'package',
    allow_decimal: Boolean(unit.allow_decimal),
    active: unit.active !== false,
  }
}

function defaultGroupTemplate(group = {}) {
  return {
    id: Number(group.id || 0),
    name: String(group.name || '').trim(),
    code: String(group.code || '').trim(),
    remark: String(group.remark || '').trim(),
    active: group.active !== false,
    sort_order: Number(group.sort_order || 100),
  }
}

function defaultGroupTemplateCategory(item = {}) {
  return {
    id: Number(item.id || 0),
    group_id: Number(item.group_id || selectedGroupTemplateID.value || 0),
    parent_id: Number(item.parent_id || item.parentID || 0),
    name: String(item.name || '').trim(),
    code: String(item.code || '').trim(),
    remark: String(item.remark || '').trim(),
    active: item.active !== false,
    sort_order: Number(item.sort_order || 100),
  }
}

function groupTemplateName(group = {}) {
  return String(group.name || '').trim() || `分组模板 #${Number(group.id || 0)}`
}

function unitTypeLabel(value) {
  return {
    weight: '重量',
    package: '包装',
    count: '数量',
    other: '其他',
  }[value] || '其他'
}

function assignSettings(data) {
  const settings = data?.settings || data || {}
  form.hide_customer_account_fulfillment = settings.hide_customer_account_fulfillment !== false
}

function assignGlobalUnitDefinitionForm(unit = {}) {
  Object.assign(unitForm, defaultGlobalUnitDefinition(unit))
}

function resetGlobalUnitDefinitionForm() {
  assignGlobalUnitDefinitionForm()
  unitEditingCode.value = ''
}

function startGlobalUnitDefinitionEdit(unit) {
  assignGlobalUnitDefinitionForm(JSON.parse(JSON.stringify(unit || {})))
  unitEditingCode.value = String(unit?.code || '')
}

function assignGroupTemplateForm(group = {}) {
  Object.assign(groupTemplateForm, defaultGroupTemplate(group))
}

function resetGroupTemplateForm() {
  assignGroupTemplateForm()
}

function selectGroupTemplate(group) {
  const id = Number(group?.id || 0)
  selectedGroupTemplateID.value = id
  assignGroupTemplateForm(group)
  resetGroupTemplateCategoryForm()
}

function resetGroupTemplateCategoryForm() {
  Object.assign(groupTemplateCategoryForm, defaultGroupTemplateCategory())
  originalGroupTemplateCategoryParentID.value = 0
}

function startGroupTemplateCategoryCreate(parentID) {
  Object.assign(groupTemplateCategoryForm, defaultGroupTemplateCategory({
    group_id: selectedGroupTemplateID.value,
    parent_id: Number(parentID || 0),
    sort_order: 100,
    active: true,
  }))
  originalGroupTemplateCategoryParentID.value = Number(parentID || 0)
}

function startGroupTemplateCategoryEdit(item) {
  Object.assign(groupTemplateCategoryForm, defaultGroupTemplateCategory(item))
  originalGroupTemplateCategoryParentID.value = Number(item?.parent_id || item?.parentID || 0)
}

function parentCategoryName(parentID) {
  const id = Number(parentID || 0)
  if (!id) return '大类'
  const parent = selectedGroupTemplateTree.value.find((item) => Number(item.id || 0) === id)
  return parent ? `小类 / ${parent.name}` : '小类'
}

function validateGlobalUnitDefinitionPayload(payload) {
  if (!String(payload.code || '').trim()) return '请填写单位编码'
  if (!String(payload.name || '').trim()) return '请填写单位名称'
  return ''
}

async function loadProductUnitDefinitions() {
  const data = await apiGet('/api/product-settings')
  productUnitDefinitions.value = (data.product_unit_definitions || []).map(defaultGlobalUnitDefinition)
}

async function loadGroupTemplates() {
  const data = await apiGet('/api/business-groups')
  groupTemplates.value = Array.isArray(data?.rows) ? data.rows : (Array.isArray(data) ? data : [])
  if (!selectedGroupTemplateID.value && activeGroupTemplates.value.length) {
    selectGroupTemplate(activeGroupTemplates.value[0])
  } else if (selectedGroupTemplateID.value && !selectedGroupTemplate.value) {
    selectedGroupTemplateID.value = 0
    resetGroupTemplateForm()
    resetGroupTemplateCategoryForm()
  } else if (selectedGroupTemplate.value) {
    assignGroupTemplateForm(selectedGroupTemplate.value)
  }
}

async function load() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const [settings] = await Promise.all([
      fetchUISettings(),
      loadProductUnitDefinitions(),
      loadGroupTemplates(),
    ])
    assignSettings(settings)
  } catch (err) {
    error.value = err.message || '加载系统设置失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    assignSettings(await saveUISettings({
      hide_customer_account_fulfillment: !!form.hide_customer_account_fulfillment,
    }))
    ok.value = '已保存系统设置'
  } catch (err) {
    error.value = err.message || '保存系统设置失败'
  } finally {
    saving.value = false
  }
}

async function saveGroupTemplate() {
  const payload = {
    id: Number(groupTemplateForm.id || 0),
    name: String(groupTemplateForm.name || '').trim(),
    code: String(groupTemplateForm.code || '').trim(),
    remark: String(groupTemplateForm.remark || '').trim(),
    active: groupTemplateForm.active !== false,
    sort_order: Number(groupTemplateForm.sort_order || 100),
    usages: [],
  }
  if (!payload.name) {
    error.value = '请填写模板名'
    return
  }
  groupTemplateSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend(payload.id ? `/api/business-groups/${payload.id}` : '/api/business-groups', {
      method: payload.id ? 'PUT' : 'POST',
      body: payload,
    })
    const saved = result?.group || result || payload
    selectedGroupTemplateID.value = Number(saved.id || payload.id || 0)
    ok.value = '分组模板已保存'
    await loadGroupTemplates()
  } catch (err) {
    error.value = err.message || '保存分组模板失败'
  } finally {
    groupTemplateSaving.value = false
  }
}

async function saveGroupTemplateCategory() {
  const templateID = Number(selectedGroupTemplateID.value || 0)
  const payload = {
    id: Number(groupTemplateCategoryForm.id || 0),
    group_id: Number(groupTemplateCategoryForm.group_id || templateID),
    parent_id: Number(groupTemplateCategoryForm.parent_id || 0),
    name: String(groupTemplateCategoryForm.name || '').trim(),
    code: String(groupTemplateCategoryForm.code || '').trim(),
    remark: String(groupTemplateCategoryForm.remark || '').trim(),
    active: groupTemplateCategoryForm.active !== false,
    sort_order: Number(groupTemplateCategoryForm.sort_order || 100),
  }
  if (!payload.group_id) {
    error.value = '请先选择分组模板'
    return
  }
  if (!payload.name) {
    error.value = '请填写分类名称'
    return
  }
  groupTemplateSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const result = await apiSend(payload.id ? `/api/business-group-items/${payload.id}` : '/api/business-group-items', {
      method: payload.id ? 'PUT' : 'POST',
      body: payload,
    })
    ok.value = '分类已保存'
    const saved = result?.item || payload
    await loadGroupTemplates()
    startGroupTemplateCategoryEdit(saved)
  } catch (err) {
    error.value = err.message || '保存分类失败'
  } finally {
    groupTemplateSaving.value = false
  }
}

async function deleteGroupTemplateCategory(item) {
  const id = Number(item?.id || 0)
  if (!id) return
  if (typeof window !== 'undefined' && !window.confirm(`确认停用分类「${item.name || id}」？`)) return
  groupTemplateSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/business-group-items/${id}`, { method: 'DELETE' })
    ok.value = '分类已停用'
    resetGroupTemplateCategoryForm()
    await loadGroupTemplates()
  } catch (err) {
    error.value = err.message || '停用分类失败'
  } finally {
    groupTemplateSaving.value = false
  }
}

async function saveGlobalUnitDefinition() {
  const payload = buildProductUnitDefinitionPayload(unitForm)
  const validation = validateGlobalUnitDefinitionPayload(payload)
  if (validation) {
    error.value = validation
    return
  }
  unitSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const editingCode = unitEditingCode.value
    const url = editingCode ? `/api/product-settings/units/${encodeURIComponent(editingCode)}` : '/api/product-settings/units'
    const method = editingCode ? 'PUT' : 'POST'
    await apiSend(url, { method, body: payload })
    ok.value = '全局单位已保存，可在商品配置的单位模板中引用'
    resetGlobalUnitDefinitionForm()
    await loadProductUnitDefinitions()
  } catch (err) {
    error.value = err.message || '保存全局单位失败'
  } finally {
    unitSaving.value = false
  }
}

async function deleteGlobalUnitDefinition() {
  const editingCode = unitEditingCode.value
  if (!editingCode) return
  if (typeof window !== 'undefined' && !window.confirm(`确认删除全局单位「${unitForm.name || editingCode}」？已引用该单位的历史配置不会被物理删除。`)) return
  unitSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    await apiSend(`/api/product-settings/units/${encodeURIComponent(editingCode)}`, { method: 'DELETE' })
    ok.value = '全局单位已删除，新的单位模板将不再引用该单位'
    resetGlobalUnitDefinitionForm()
    await loadProductUnitDefinitions()
  } catch (err) {
    error.value = err.message || '删除全局单位失败'
  } finally {
    unitSaving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; display: grid; gap: 14px; }
.panel { border: 1px solid #e1e5ea; border-radius: 8px; background: #fff; padding: 14px; max-width: 1080px; }
.panel-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 16px; }
h2 { margin: 0; font-size: 22px; }
p { margin: 5px 0 0; color: #666; font-size: 13px; }
.toggle-row { display: flex; gap: 10px; align-items: flex-start; padding: 12px; border: 1px solid #e6eaf0; border-radius: 8px; background: #fbfcfe; cursor: pointer; }
.toggle-row input { margin-top: 3px; width: 18px; height: 18px; }
.toggle-row strong { display: block; font-size: 15px; }
.toggle-row small { display: block; color: #666; font-size: 13px; margin-top: 4px; }
.actions { display: flex; align-items: center; gap: 10px; margin-top: 14px; flex-wrap: wrap; }
button { border: 1px solid #d7dde6; border-radius: 6px; background: #fff; padding: 8px 12px; cursor: pointer; }
button.primary { background: #111827; color: #fff; border-color: #111827; }
button:disabled { opacity: .55; cursor: not-allowed; }
.secondary { background: #fff; color: #111827; }
.compact-action { min-height: 30px; padding: 5px 10px; font-size: 12px; }
.text-button { border: 0; background: transparent; padding: 0; color: #1f4f82; }
.danger-text { color: #a33; }
.ok { color: #0f766e; font-size: 13px; }
.error { color: #b91c1c; font-size: 13px; }
.group-template-layout, .unit-layout { display: grid; grid-template-columns: minmax(260px, .8fr) minmax(320px, 1.2fr); gap: 14px; align-items: start; }
.template-list, .unit-list { display: flex; flex-wrap: wrap; gap: 8px; align-content: flex-start; }
.template-chip, .unit-chip { min-height: 48px; display: grid; gap: 2px; text-align: left; border-color: #d9d2c8; background: #fbfaf8; }
.template-chip.active { border-color: #111827; box-shadow: 0 0 0 1px #111827 inset; }
.template-chip small, .unit-chip small { color: #666; font-size: 12px; }
.template-chip.inactive, .unit-chip.inactive { opacity: .55; }
.group-template-form, .category-form, .unit-definition-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; border: 1px solid #e6eaf0; border-radius: 8px; padding: 12px; background: #fbfcfe; }
.unit-form-head { grid-column: 1 / -1; display: flex; justify-content: space-between; align-items: center; gap: 10px; flex-wrap: wrap; }
.unit-form-head div { display: grid; gap: 2px; }
.unit-form-head strong { color: #1f2937; }
.unit-form-head small { color: #667085; font-size: 12px; }
.group-template-form label, .category-form label, .unit-definition-form label { display: grid; gap: 5px; font-size: 13px; color: #333; }
.group-template-form input, .group-template-form select, .category-form input, .category-form select, .unit-definition-form input, .unit-definition-form select { min-height: 36px; border: 1px solid #d7dde6; border-radius: 6px; padding: 6px 8px; background: #fff; }
.wide-field { grid-column: 1 / -1; }
.checkline { display: flex !important; align-items: center; gap: 8px; min-height: 36px; }
.checkline input { width: auto; min-height: 0; }
.unit-actions { grid-column: 1 / -1; justify-content: flex-end; margin-top: 0; }
.category-editor { display: grid; gap: 12px; margin-top: 14px; border-top: 1px solid #eef2f7; padding-top: 14px; }
.category-editor-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.category-editor-head div { display: grid; gap: 2px; }
.category-editor-head small { color: #667085; font-size: 12px; }
.category-tree { display: grid; gap: 8px; }
.category-node { border: 1px solid #e6eaf0; border-radius: 8px; background: #fff; padding: 10px; }
.category-node-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.category-name { text-align: left; display: grid; gap: 2px; }
.category-name small { color: #667085; }
.category-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.category-children { display: flex; flex-wrap: wrap; gap: 8px; padding-top: 8px; }
.category-child { display: grid; gap: 2px; min-height: 38px; text-align: left; background: #fbfcfe; }
.category-child small { color: #667085; font-size: 12px; }
.muted { color: #777; }
@media (max-width: 760px) {
  .panel-head { flex-direction: column; }
  .group-template-layout, .unit-layout, .group-template-form, .category-form, .unit-definition-form { grid-template-columns: 1fr; }
}
</style>
