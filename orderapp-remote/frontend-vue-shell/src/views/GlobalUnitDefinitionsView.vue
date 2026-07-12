<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>全局单位字典</h2>
          <p>基础单位在这里维护；SKU 设置里的单位模板只引用这些单位并配置换算关系。</p>
        </div>
        <button class="secondary" type="button" :disabled="loading" @click="load">刷新</button>
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
          <label><span>单位编码</span><input v-model.trim="unitForm.code" :disabled="Boolean(unitEditingCode)" placeholder="box" /></label>
          <label><span>单位名称</span><input v-model.trim="unitForm.name" placeholder="盒" /></label>
          <label>
            <span>单位类型</span>
            <select v-model="unitForm.unit_type">
              <option value="weight">重量</option>
              <option value="package">包装</option>
              <option value="count">数量</option>
              <option value="other">其他</option>
            </select>
          </label>
          <label class="checkline"><input v-model="unitForm.allow_decimal" type="checkbox" /><span>允许小数</span></label>
          <label class="checkline"><input v-model="unitForm.active" type="checkbox" /><span>启用</span></label>
          <div class="actions unit-actions">
            <button v-if="unitEditingCode" class="text-button danger-text" type="button" :disabled="unitSaving || loading" @click="deleteGlobalUnitDefinition">删除</button>
            <button class="primary" type="submit" :disabled="unitSaving || loading">{{ unitSaving ? '保存中' : (unitEditingCode ? '保存' : '新增') }}</button>
          </div>
        </form>
      </div>
      <div v-if="ok" class="notice ok">{{ ok }}</div>
      <div v-if="error" class="notice error">{{ error }}</div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { buildProductUnitDefinitionPayload, visibleNonDeletedRows } from '../lib/product-settings'

const loading = ref(false)
const unitSaving = ref(false)
const ok = ref('')
const error = ref('')
const productUnitDefinitions = ref([])
const visibleProductUnitDefinitions = computed(() => visibleNonDeletedRows(productUnitDefinitions.value))
const unitEditingCode = ref('')
const unitForm = reactive(defaultGlobalUnitDefinition())

function defaultGlobalUnitDefinition(unit = {}) {
  return { code: unit.code || '', name: unit.name || '', unit_type: unit.unit_type || 'package', allow_decimal: Boolean(unit.allow_decimal), active: unit.active !== false }
}

function unitTypeLabel(value) {
  return { weight: '重量', package: '包装', count: '数量', other: '其他' }[value] || '其他'
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

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet('/api/product-settings')
    productUnitDefinitions.value = (data.product_unit_definitions || []).map(defaultGlobalUnitDefinition)
  } catch (err) {
    error.value = err.message || '加载全局单位失败'
  } finally {
    loading.value = false
  }
}

async function saveGlobalUnitDefinition() {
  const payload = buildProductUnitDefinitionPayload(unitForm)
  if (!String(payload.code || '').trim() || !String(payload.name || '').trim()) {
    error.value = !String(payload.code || '').trim() ? '请填写单位编码' : '请填写单位名称'
    return
  }
  unitSaving.value = true
  error.value = ''
  ok.value = ''
  try {
    const editingCode = unitEditingCode.value
    await apiSend(editingCode ? `/api/product-settings/units/${encodeURIComponent(editingCode)}` : '/api/product-settings/units', {
      method: editingCode ? 'PUT' : 'POST',
      body: payload,
    })
    ok.value = '全局单位已保存，可在商品配置的单位模板中引用'
    resetGlobalUnitDefinitionForm()
    await load()
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
    await load()
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
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e1e5ea; border-radius: 8px; background: #fff; padding: 14px; max-width: 1080px; }
.panel-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 16px; }
h2 { margin: 0; font-size: 22px; }
p { margin: 5px 0 0; color: #666; font-size: 13px; }
button { border: 1px solid #d7dde6; border-radius: 6px; background: #fff; padding: 8px 12px; cursor: pointer; }
button.primary { background: #111827; color: #fff; border-color: #111827; }
button:disabled { opacity: .55; cursor: not-allowed; }
.unit-layout { display: grid; grid-template-columns: minmax(260px, .8fr) minmax(320px, 1.2fr); gap: 14px; align-items: start; }
.unit-list { display: flex; flex-wrap: wrap; gap: 8px; align-content: flex-start; }
.unit-chip { min-height: 48px; display: grid; gap: 2px; text-align: left; border-color: #d9d2c8; background: #fbfaf8; }
.unit-chip small { color: #666; font-size: 12px; }
.unit-chip.inactive { opacity: .55; }
.unit-definition-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; border: 1px solid #e6eaf0; border-radius: 8px; padding: 12px; background: #fbfcfe; }
.unit-form-head { grid-column: 1 / -1; display: flex; justify-content: space-between; align-items: center; gap: 10px; flex-wrap: wrap; }
.unit-form-head div { display: grid; gap: 2px; }
.unit-form-head small { color: #667085; font-size: 12px; }
.unit-definition-form label { display: grid; gap: 5px; font-size: 13px; color: #333; }
.unit-definition-form input, .unit-definition-form select { min-height: 36px; border: 1px solid #d7dde6; border-radius: 6px; padding: 6px 8px; background: #fff; }
.checkline { display: flex !important; align-items: center; gap: 8px; min-height: 36px; }
.checkline input { width: auto; min-height: 0; }
.actions { display: flex; align-items: center; gap: 10px; }
.unit-actions { grid-column: 1 / -1; justify-content: flex-end; }
.text-button { border: 0; background: transparent; padding: 0; }
.danger-text, .error { color: #b91c1c; }
.ok { color: #0f766e; }
.notice { margin-top: 12px; font-size: 13px; }
.muted { color: #777; }
@media (max-width: 760px) {
  .panel-head { flex-direction: column; }
  .unit-layout, .unit-definition-form { grid-template-columns: 1fr; }
}
</style>
