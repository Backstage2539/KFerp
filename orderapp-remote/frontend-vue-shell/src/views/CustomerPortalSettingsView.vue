<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>客户门户配置</h2>
        <div class="panel-actions">
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
        <span>客户配置</span>
        <span>引用模板</span>
        <span>ERP账号</span>
        <span>绑定用户</span>
      </div>

      <div v-if="!portalRows.length && !loading" class="muted empty">暂无客户</div>

      <div v-for="row in portalRows" :key="row.customer.id" class="portal-row">
        <div class="customer-cell">
          <strong>{{ row.customer.name }}</strong>
          <span>{{ row.customer.company_name || '未填公司' }}</span>
          <span>{{ row.customer.phone || '未填手机号' }}</span>
          <span>{{ customerTypeLabel(row.customer.customer_type) }}</span>
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
                <option value="">请选择模板</option>
                <option v-for="template in capabilityTemplates" :key="template.key" :value="template.key">
                  {{ template.label }}
                </option>
              </select>
            </label>
          </div>
          <label class="check">
            <input v-model="row.form.enabled" type="checkbox" />
            <span>{{ row.form.enabled ? '门户启用' : '门户停用' }}</span>
          </label>
          <button class="primary" type="button" @click="saveVisibility(row)" :disabled="!canSaveRow(row)">
            {{ row.saving ? '保存中' : '保存并应用模板' }}
          </button>
        </div>

        <div class="template-summary">
          <template v-if="selectedTemplate(row)">
            <strong>{{ selectedTemplate(row).label }}</strong>
            <span>{{ selectedTemplate(row).description || '未填写模板说明' }}</span>
            <div class="summary-meta">
              <i>{{ entryModeLabel(selectedTemplate(row).miniapp_entry_mode) }}</i>
              <i>{{ themeLabel(selectedTemplate(row).theme_key) }}</i>
            </div>
            <div class="summary-block">
              <b>能力</b>
              <div class="chips">
                <i v-for="capability in enabledTemplateCapabilities(row)" :key="`${row.customer.id}-${capability.code}`">{{ capability.label || capabilityLabels[capability.code] || capability.code }}</i>
                <span v-if="!enabledTemplateCapabilities(row).length" class="muted">未开启能力</span>
              </div>
            </div>
            <div class="summary-block">
              <b>规则</b>
              <div class="chips">
                <i v-for="rule in templateRuleLabels(row)" :key="`${row.customer.id}-${rule}`">{{ rule }}</i>
                <span v-if="!templateRuleLabels(row).length" class="muted">无特殊规则</span>
              </div>
            </div>
            <div class="summary-block">
              <b>ERP 客户页面</b>
              <div class="chips">
                <i v-for="key in selectedTemplate(row).erp_view_keys || []" :key="`${row.customer.id}-${key}`">{{ viewLabel(key) }}</i>
              </div>
            </div>
          </template>
          <span v-else class="muted">请选择能力模板；客户的门户能力、规则和客户侧 ERP 页面都从模板继承。</span>
          <span v-if="row.loading" class="muted">加载模板中...</span>
        </div>

        <div class="erp-binding">
          <div v-if="row.customer.erp_binding" class="binding-row active-binding">
            <strong>{{ row.customer.erp_binding.employee_name || row.customer.erp_binding.phone || row.customer.erp_binding.employee_id }}</strong>
            <span>{{ row.customer.erp_binding.phone || '未填手机号' }}</span>
          </div>
          <span v-else class="muted">未绑定ERP账号</span>
          <select v-model.number="row.form.erp_employee_id">
            <option :value="0">选择渠道客户账号</option>
            <option v-for="account in channelAccounts" :key="account.employee_id" :value="account.employee_id">
              {{ accountLabel(account) }}
            </option>
          </select>
          <button class="secondary" type="button" @click="saveERPBinding(row)" :disabled="row.saving || !row.form.erp_employee_id">绑定ERP账号</button>
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
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const q = ref('')
const portalRows = ref([])
const senderProfiles = ref([])
const capabilityTemplates = ref([])
const authAccounts = ref([])
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
const themeLabels = {
  coffee_factory: '咖啡工厂专业风',
  clean_ops: '清爽业务工具风',
  premium_partner: '品牌会员高级风',
}

const channelAccounts = computed(() => (authAccounts.value || []).filter((row) => row.account_type === 'channel_customer'))

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

async function loadAuthAccounts() {
  try {
    const data = await apiGet('/api/auth/accounts')
    authAccounts.value = data.rows || []
  } catch (err) {
    authAccounts.value = []
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

function accountLabel(account) {
  const name = account?.name || account?.phone || `账号${account?.employee_id || ''}`
  const phone = account?.phone ? ` / ${account.phone}` : ''
  return `${name}${phone}`
}

function customerTypeLabel(value) {
  return {
    wholesale: '批发客户',
    retail: '零售客户',
    ecommerce: '电商客户',
  }[value] || '零售客户'
}

function createPortalRow(customer) {
  return {
    customer,
    form: {
      display_name: customer.display_name || '',
      processing_warehouse_code: customer.processing_warehouse_code || '',
      default_sender_id: Number(customer.default_sender_id || 0),
      enabled: customer.portal_enabled !== false,
      capability_template_key: normalizeTemplateKey(customer.capability_template_key),
      erp_employee_id: Number(customer.erp_binding?.employee_id || 0),
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
  row.form.capability_template_key = normalizeTemplateKey(row.customer.capability_template_key)
  row.form.erp_employee_id = Number(row.customer.erp_binding?.employee_id || 0)
  row.bindings = data?.bindings || []
  row.capabilities = (data?.capabilities || []).map((item) => ({
    code: item.code,
    label: item.label || capabilityLabels[item.code] || item.code,
    description: item.description || '',
    enabled: !!item.enabled,
    config: item.config || {},
  }))
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
        capability_template_key: normalizeTemplateKey(row.form.capability_template_key),
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

async function saveERPBinding(row) {
  if (!row?.customer?.id || !row.form.erp_employee_id) return
  row.saving = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiSend(`/api/customer-portal/admin/customers/${row.customer.id}/erp-binding`, {
      method: 'PUT',
      body: {
        employee_id: Number(row.form.erp_employee_id),
        status: 'active',
      },
    })
    assignRowDetail(row, data)
    ok.value = `已绑定 ${row.customer.name} 的ERP账号`
  } catch (err) {
    error.value = err.message || '绑定ERP账号失败'
  } finally {
    row.saving = false
  }
}

function normalizeTemplateKey(value) {
  const key = String(value || '').trim()
  if (key === 'processing_fulfillment' || key === 'public_sku_direct_ship' || key === 'retail_mall') return key
  return capabilityTemplates.value.some((template) => template.key === key) ? key : ''
}

function selectedTemplate(row) {
  const key = normalizeTemplateKey(row?.form?.capability_template_key)
  return capabilityTemplates.value.find((template) => template.key === key) || null
}

function enabledTemplateCapabilities(row) {
  return (selectedTemplate(row)?.capabilities || []).filter((capability) => capability.enabled)
}

function templateRuleLabels(row) {
  const rules = []
  const capabilities = selectedTemplate(row)?.capabilities || []
  const productOrder = capabilities.find((capability) => capability.code === 'product_order')
  const directShip = capabilities.find((capability) => capability.code === 'direct_ship')
  if (productOrder?.config?.public_sku_aliases || directShip?.config?.public_sku_aliases) {
    rules.push('公共 SKU 可使用客户自定义名称')
  }
  if (directShip?.config?.customer_sender) {
    rules.push('代发默认使用客户寄件人')
  }
  if (directShip?.config?.external_recipients) {
    rules.push('代发收件人不进入系统客户列表')
  }
  const smallBatch = directShip?.config?.small_batch_price_rule
  if (smallBatch?.enabled) {
    rules.push(`小于 ${smallBatch.threshold_lb || 14} 磅按 ${smallBatch.tier_min_lb || 15}-${smallBatch.tier_max_lb || 28} 磅梯度`)
  }
  return rules
}

function entryModeLabel(value) {
  return value === 'mall' ? '小程序首页：商城' : '小程序首页：订单处理'
}

function themeLabel(value) {
  return `小程序主题：${themeLabels[value] || themeLabels.coffee_factory}`
}

function viewLabel(key) {
  if (key === 'customerProcessingPortal') return '客户履约工作台'
  return key
}

function canSaveRow(row) {
  return !row.saving && !row.loading && (!row.form.enabled || !!normalizeTemplateKey(row.form.capability_template_key))
}

onMounted(async () => {
  await Promise.all([loadCapabilityTemplates(), loadSenderProfiles(), loadAuthAccounts()])
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
.list-head, .portal-row { display: grid; grid-template-columns: 190px 280px minmax(320px, 1fr) 220px 210px; gap: 14px; align-items: start; }
.list-head { padding: 8px 10px; background: #f8fafc; border-bottom: 1px solid #eef1f4; color: #666; font-size: 13px; font-weight: 700; }
.portal-row { padding: 14px 10px; border-bottom: 1px solid #eef1f4; }
.portal-row:last-child { border-bottom: 0; }
.customer-cell, .config-cell, .binding-list, .erp-binding { display: flex; flex-direction: column; gap: 8px; }
.customer-cell strong { font-size: 15px; }
.customer-cell span, .binding-row span { color: #666; font-size: 13px; line-height: 1.4; }
.config-cell input, .config-cell select { width: 100%; }
.template-picker { display: grid; gap: 8px; align-items: end; }
.check { display: inline-flex; align-items: center; gap: 8px; }
.check input { width: auto; height: auto; }
.check span { margin: 0; color: #333; font-size: 13px; }
.template-summary { min-height: 132px; border: 1px solid #e4e7ec; border-radius: 8px; padding: 10px; display: grid; gap: 8px; align-content: start; }
.template-summary strong { font-size: 15px; line-height: 1.3; }
.template-summary > span { color: #666; font-size: 13px; line-height: 1.45; }
.summary-meta, .chips { display: flex; flex-wrap: wrap; gap: 6px; }
.summary-meta i, .chips i { font-style: normal; border: 1px solid #dde3ea; border-radius: 999px; background: #f8fafc; padding: 4px 8px; color: #333; font-size: 12px; line-height: 1.3; max-width: 100%; overflow-wrap: anywhere; }
.summary-block { display: grid; gap: 5px; }
.summary-block b { font-size: 12px; color: #555; }
.binding-row { border: 1px solid #e4e7ec; border-radius: 8px; padding: 8px; }
.binding-row strong { font-size: 13px; }
.erp-binding select { width: 100%; }
.active-binding { border-color: #b7d9c2; background: #f7fff9; }
.muted { color: #666; }
.empty { min-height: 80px; display: flex; align-items: center; justify-content: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
@media (max-width: 1100px) {
  .list-head { display: none; }
  .portal-row { grid-template-columns: 1fr; }
  .template-picker { grid-template-columns: 1fr; }
}
</style>
