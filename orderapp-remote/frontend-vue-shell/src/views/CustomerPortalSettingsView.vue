<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>客户门户配置</h2>
        <div class="panel-actions">
          <button class="secondary" type="button" @click="openTemplateSettings">客户能力模板</button>
          <button class="secondary" type="button" @click="loadCustomers" :disabled="loading">刷新</button>
        </div>
      </div>
      <div class="filters">
        <label>
          <span>客户搜索</span>
          <input v-model.trim="q" placeholder="客户名/手机号/公司名" @keyup.enter="loadCustomers" />
        </label>
        <button class="primary" type="button" @click="loadCustomers" :disabled="loading">查询</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
    </section>

    <section class="panel">
      <div class="list-head">
        <span>客户</span>
        <span>门户配置</span>
        <span>能力</span>
        <span>绑定用户</span>
      </div>

      <div v-if="!portalRows.length && !loading" class="muted empty">暂无客户</div>

      <div v-for="row in portalRows" :key="row.customer.id" class="portal-row">
        <div class="customer-cell">
          <strong>{{ row.customer.name }}</strong>
          <span>{{ row.customer.company_name || '未填公司' }}</span>
          <span>{{ row.customer.phone || '未填手机号' }}</span>
          <span>{{ row.customer.binding_count || 0 }} 个绑定用户</span>
        </div>

        <div class="config-cell">
          <label>
            <span>小程序显示名</span>
            <input v-model.trim="row.form.display_name" placeholder="默认使用客户名" />
          </label>
          <label>
            <span>代加工仓库</span>
            <input v-model.trim="row.form.processing_warehouse_code" placeholder="默认 cust_ID_processing" />
          </label>
          <label>
            <span>默认寄件人</span>
            <select v-model.number="row.form.default_sender_id">
              <option :value="0">使用系统默认寄件人</option>
              <option v-for="profile in senderProfiles" :key="profile.sender_id" :value="profile.sender_id">
                {{ senderProfileLabel(profile) }}
              </option>
            </select>
          </label>
          <div class="template-picker">
            <label>
              <span>能力模板</span>
              <select v-model="row.form.capability_template_key">
                <option value="">不使用模板</option>
                <option v-for="template in capabilityTemplates" :key="template.key" :value="template.key">
                  {{ template.label }}
                </option>
              </select>
            </label>
            <button class="secondary" type="button" @click="applyCapabilityTemplate(row)" :disabled="row.saving || row.loading || !row.form.capability_template_key">
              套用模板
            </button>
          </div>
          <label class="check">
            <input v-model="row.form.enabled" type="checkbox" />
            <span>{{ row.form.enabled ? '门户启用' : '门户停用' }}</span>
          </label>
          <div class="entry-picker">
            <span>小程序首页</span>
            <div class="entry-options">
              <button
                type="button"
                class="entry-option"
                :class="{ selected: row.form.miniapp_entry_mode === 'services' }"
                @click="row.form.miniapp_entry_mode = 'services'">
                订单处理
              </button>
              <button
                type="button"
                class="entry-option"
                :class="{ selected: row.form.miniapp_entry_mode === 'mall' }"
                @click="row.form.miniapp_entry_mode = 'mall'">
                商城
              </button>
            </div>
          </div>
          <div class="theme-picker">
            <span>小程序主题</span>
            <div class="theme-options">
              <button
                v-for="theme in customerPortalThemeOptions"
                :key="`${row.customer.id}-${theme.key}`"
                type="button"
                class="theme-option"
                :class="{ selected: row.form.theme_key === theme.key }"
                :title="theme.description"
                @click="row.form.theme_key = theme.key">
                <i :class="['theme-swatch', theme.swatchClass]"></i>
                <span>{{ theme.label }}</span>
              </button>
            </div>
          </div>
          <button class="primary" type="button" @click="saveVisibility(row)" :disabled="row.saving || row.loading">
            {{ row.saving ? '保存中' : '保存配置' }}
          </button>
        </div>

        <div class="capability-grid">
          <label v-for="capability in row.capabilities" :key="`${row.customer.id}-${capability.code}`" class="capability">
            <input v-model="capability.enabled" type="checkbox" />
            <strong>{{ capability.label }}</strong>
            <span>{{ capability.description }}</span>
          </label>
          <span v-if="row.loading" class="muted">加载能力中...</span>
        </div>

        <div class="binding-list">
          <div v-for="binding in row.bindings" :key="`${row.customer.id}-${binding.mini_user_id}`" class="binding-row">
            <strong>{{ binding.phone || binding.nickname || binding.mini_user_id }}</strong>
            <span>{{ binding.role }} / {{ binding.status }}</span>
          </div>
          <span v-if="!row.bindings.length && !row.loading" class="muted">暂无绑定用户</span>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { customerPortalThemeOptions, normalizeCustomerPortalThemeKey } from '../lib/customer-portal-theme'

const q = ref('')
const portalRows = ref([])
const senderProfiles = ref([])
const capabilityTemplates = ref([])
const loading = ref(false)
const error = ref('')
const ok = ref('')
const capabilityLabels = {
  bean_list: '我的豆单',
  mall: '商城下单',
  product_order: '现货下单',
  direct_ship: '一件代发',
  processing: '代加工',
  inventory_custody: '我的库存',
  settlement: '结算中心',
}

async function loadCustomers() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = new URL('/api/customer-portal/admin/customers', window.location.origin)
    if (q.value) url.searchParams.set('q', q.value)
    const data = await apiGet(url)
    portalRows.value = (data.rows || []).map(createPortalRow)
    await Promise.all(portalRows.value.map((row) => loadRowDetail(row)))
  } catch (err) {
    error.value = err.message || '加载客户失败'
  } finally {
    loading.value = false
  }
}

async function loadSenderProfiles() {
  try {
    const data = await apiGet('/api/settings/sender')
    senderProfiles.value = data.profiles || []
  } catch (err) {
    senderProfiles.value = []
  }
}

async function loadCapabilityTemplates() {
  try {
    const data = await apiGet('/api/customer-portal/admin/capability-templates')
    capabilityTemplates.value = data.templates || []
  } catch (err) {
    capabilityTemplates.value = []
  }
}

function senderProfileLabel(profile) {
  return profile?.sender_label || profile?.sender_name || `寄件人${profile?.sender_id || ''}`
}

function createPortalRow(customer) {
  return {
    customer,
    form: {
      display_name: customer.display_name || '',
      processing_warehouse_code: customer.processing_warehouse_code || '',
      default_sender_id: Number(customer.default_sender_id || 0),
      enabled: customer.portal_enabled !== false,
      theme_key: normalizeCustomerPortalThemeKey(customer.theme_key),
      miniapp_entry_mode: normalizeEntryMode(customer.miniapp_entry_mode),
      capability_template_key: normalizeTemplateKey(customer.capability_template_key),
    },
    capabilities: [],
    bindings: [],
    loading: false,
    saving: false,
  }
}

async function loadRowDetail(row) {
  row.loading = true
  try {
    const data = await apiGet(`/api/customer-portal/admin/customers/${row.customer.id}`)
    assignRowDetail(row, data)
  } finally {
    row.loading = false
  }
}

function assignRowDetail(row, data) {
  row.customer = data?.customer || row.customer
  row.form.display_name = row.customer.display_name || ''
  row.form.processing_warehouse_code = row.customer.processing_warehouse_code || ''
  row.form.default_sender_id = Number(row.customer.default_sender_id || 0)
  row.form.enabled = row.customer.portal_enabled !== false
  row.form.theme_key = normalizeCustomerPortalThemeKey(row.customer.theme_key)
  row.form.miniapp_entry_mode = normalizeEntryMode(row.customer.miniapp_entry_mode)
  row.form.capability_template_key = normalizeTemplateKey(row.customer.capability_template_key)
  row.bindings = data?.bindings || []
  row.capabilities = (data?.capabilities || []).map((item) => ({
    code: item.code,
    label: item.label || capabilityLabels[item.code] || item.code,
    description: item.description || '',
    enabled: !!item.enabled,
    config: item.config || {},
  }))
}

async function applyCapabilityTemplate(row) {
  if (!row?.customer?.id || !row.form.capability_template_key) return
  row.saving = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiSend(`/api/customer-portal/admin/customers/${row.customer.id}/capability-template`, {
      body: { template_key: row.form.capability_template_key },
    })
    assignRowDetail(row, data)
    ok.value = `已套用 ${templateLabel(row.form.capability_template_key)}`
  } catch (err) {
    error.value = err.message || '套用能力模板失败'
  } finally {
    row.saving = false
  }
}

async function saveVisibility(row) {
  if (!row?.customer?.id) return
  row.saving = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiSend(`/api/customer-portal/admin/customers/${row.customer.id}/visibility`, {
      method: 'PUT',
      body: {
        display_name: row.form.display_name,
        processing_warehouse_code: row.form.processing_warehouse_code,
        default_sender_id: Number(row.form.default_sender_id || 0),
        enabled: !!row.form.enabled,
        theme_key: normalizeCustomerPortalThemeKey(row.form.theme_key),
        miniapp_entry_mode: normalizeEntryMode(row.form.miniapp_entry_mode),
        capability_template_key: normalizeTemplateKey(row.form.capability_template_key),
        capabilities: row.capabilities.map((item) => ({
          code: item.code,
          enabled: !!item.enabled,
          config: item.config || {},
        })),
      },
    })
    assignRowDetail(row, data)
    ok.value = `已保存 ${row.customer.name} 的客户门户配置`
  } catch (err) {
    error.value = err.message || '保存配置失败'
  } finally {
    row.saving = false
  }
}

function templateLabel(key) {
  const template = capabilityTemplates.value.find((item) => item.key === key)
  return template?.label || key || '模板'
}

function normalizeEntryMode(value) {
  return value === 'mall' ? 'mall' : 'services'
}

function normalizeTemplateKey(value) {
  const key = String(value || '').trim()
  if (key === 'processing_fulfillment' || key === 'public_sku_direct_ship') return key
  return capabilityTemplates.value.some((template) => template.key === key) ? key : ''
}

function openTemplateSettings() {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'customerCapabilityTemplates')
  window.location.assign(url.toString())
}

onMounted(async () => {
  await Promise.all([loadCapabilityTemplates(), loadSenderProfiles()])
  loadCustomers()
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e1e5ea; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .filters { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.panel-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.filters { justify-content: flex-start; margin-top: 12px; }
h2 { margin: 0; font-size: 20px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select { width: min(420px, 70vw); height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.list-head, .portal-row { display: grid; grid-template-columns: 210px 300px minmax(320px, 1fr) 220px; gap: 14px; align-items: start; }
.list-head { padding: 8px 10px; background: #f8fafc; border-bottom: 1px solid #eef1f4; color: #666; font-size: 13px; font-weight: 700; }
.portal-row { padding: 14px 10px; border-bottom: 1px solid #eef1f4; }
.portal-row:last-child { border-bottom: 0; }
.customer-cell, .config-cell, .binding-list { display: flex; flex-direction: column; gap: 8px; }
.customer-cell strong { font-size: 15px; }
.customer-cell span, .binding-row span { color: #666; font-size: 13px; line-height: 1.4; }
.config-cell input, .config-cell select { width: 100%; }
.template-picker { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; align-items: end; }
.template-picker .secondary { white-space: nowrap; }
.check { display: inline-flex; align-items: center; gap: 8px; }
.check input, .capability input { width: auto; height: auto; }
.check span { margin: 0; color: #333; font-size: 13px; }
.entry-picker { display: flex; flex-direction: column; gap: 5px; }
.entry-picker > span { color: #666; font-size: 12px; }
.entry-options { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 4px; }
.entry-option {
  min-height: 38px;
  height: auto;
  padding: 6px 8px;
  border-color: #e4e7ec;
  background: #fff;
  color: #171717;
}
.entry-option.selected { border-color: #1f1f1f; background: #f7f8f9; box-shadow: 0 0 0 2px rgba(31,31,31,.07); }
.theme-picker { display: flex; flex-direction: column; gap: 5px; }
.theme-picker > span { color: #666; font-size: 12px; }
.theme-options { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 4px; }
.theme-option {
  min-height: 44px;
  display: grid;
  grid-template-columns: 14px minmax(0, 1fr);
  column-gap: 5px;
  align-items: center;
  width: 100%;
  height: auto;
  padding: 5px 6px;
  border: 1px solid #e4e7ec;
  border-radius: 6px;
  background: #fff;
  color: #171717;
  text-align: left;
}
.theme-option.selected { border-color: #1f1f1f; box-shadow: 0 0 0 2px rgba(31,31,31,.08); }
.theme-option span { min-width: 0; color: #333; font-size: 12px; line-height: 1.2; overflow-wrap: anywhere; }
.theme-swatch { width: 14px; height: 14px; border-radius: 999px; }
.theme-swatch-coffee { background: linear-gradient(135deg, #2b2118, #9b7141); }
.theme-swatch-clean { background: linear-gradient(135deg, #e7f0eb, #28624a); }
.theme-swatch-premium { background: linear-gradient(135deg, #111, #b88a46); }
.capability-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 8px; }
.capability { min-height: 74px; border: 1px solid #e4e7ec; border-radius: 8px; padding: 9px; display: grid; grid-template-columns: auto 1fr; column-gap: 8px; row-gap: 4px; align-items: start; }
.capability strong { font-size: 14px; line-height: 1.3; }
.capability span { grid-column: 2; font-size: 12px; color: #666; line-height: 1.45; margin: 0; }
.binding-row { border: 1px solid #e4e7ec; border-radius: 8px; padding: 8px; }
.binding-row strong { font-size: 13px; }
.muted { color: #666; }
.empty { min-height: 80px; display: flex; align-items: center; justify-content: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
@media (max-width: 1100px) {
  .list-head { display: none; }
  .portal-row { grid-template-columns: 1fr; }
  .capability-grid { grid-template-columns: 1fr; }
  .template-picker { grid-template-columns: 1fr; }
}
</style>
