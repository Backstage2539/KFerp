<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>全局设置</h2>
          <p>维护 ERP 全局入口和跨商品模块共用的基础资料。</p>
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
            v-for="unit in productUnitDefinitions"
            :key="unit.code"
            class="unit-chip"
            :class="{ inactive: unit.active === false }"
            type="button"
            @click="startGlobalUnitDefinitionEdit(unit)">
            <strong>{{ unit.name || unit.code }}</strong>
            <small>{{ unit.code }} · {{ unitTypeLabel(unit.unit_type) }} · {{ unit.allow_decimal ? '允许小数' : '整数优先' }}</small>
          </button>
          <p v-if="!productUnitDefinitions.length" class="muted">暂无单位，先新增 kg、盒、箱等基础单位。</p>
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
import { onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { fetchUISettings, saveUISettings } from '../api/ui-settings'
import { buildProductUnitDefinitionPayload } from '../lib/product-settings'

const loading = ref(false)
const saving = ref(false)
const unitSaving = ref(false)
const ok = ref('')
const error = ref('')
const productUnitDefinitions = ref([])
const unitEditingCode = ref('')
const form = reactive({
  hide_customer_account_fulfillment: true,
})
const unitForm = reactive(defaultGlobalUnitDefinition())

function defaultGlobalUnitDefinition(unit = {}) {
  return {
    code: unit.code || '',
    name: unit.name || '',
    unit_type: unit.unit_type || 'package',
    allow_decimal: Boolean(unit.allow_decimal),
    active: unit.active !== false,
  }
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

function validateGlobalUnitDefinitionPayload(payload) {
  if (!String(payload.code || '').trim()) return '请填写单位编码'
  if (!String(payload.name || '').trim()) return '请填写单位名称'
  return ''
}

async function loadProductUnitDefinitions() {
  const data = await apiGet('/api/product-settings')
  productUnitDefinitions.value = (data.product_unit_definitions || []).map(defaultGlobalUnitDefinition)
}

async function load() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const [settings] = await Promise.all([
      fetchUISettings(),
      loadProductUnitDefinitions(),
    ])
    assignSettings(settings)
  } catch (err) {
    error.value = err.message || '加载全局设置失败'
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
    ok.value = '已保存全局设置'
  } catch (err) {
    error.value = err.message || '保存全局设置失败'
  } finally {
    saving.value = false
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
.panel { border: 1px solid #e1e5ea; border-radius: 8px; background: #fff; padding: 14px; max-width: 980px; }
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
.unit-layout { display: grid; grid-template-columns: minmax(260px, .8fr) minmax(320px, 1.2fr); gap: 14px; align-items: start; }
.unit-list { display: flex; flex-wrap: wrap; gap: 8px; align-content: flex-start; }
.unit-chip { min-height: 48px; display: grid; gap: 2px; text-align: left; border-color: #d9d2c8; background: #fbfaf8; }
.unit-chip small { color: #666; font-size: 12px; }
.unit-chip.inactive { opacity: .55; }
.unit-definition-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; border: 1px solid #e6eaf0; border-radius: 8px; padding: 12px; background: #fbfcfe; }
.unit-form-head { grid-column: 1 / -1; display: flex; justify-content: space-between; align-items: center; gap: 10px; flex-wrap: wrap; }
.unit-form-head div { display: grid; gap: 2px; }
.unit-form-head strong { color: #1f2937; }
.unit-form-head small { color: #667085; font-size: 12px; }
.unit-definition-form label { display: grid; gap: 5px; font-size: 13px; color: #333; }
.unit-definition-form input, .unit-definition-form select { min-height: 36px; border: 1px solid #d7dde6; border-radius: 6px; padding: 6px 8px; background: #fff; }
.checkline { display: flex !important; align-items: center; gap: 8px; min-height: 36px; }
.checkline input { width: auto; min-height: 0; }
.unit-actions { grid-column: 1 / -1; justify-content: flex-end; margin-top: 0; }
.muted { color: #777; }
@media (max-width: 760px) {
  .panel-head { flex-direction: column; }
  .unit-layout, .unit-definition-form { grid-template-columns: 1fr; }
}
</style>
