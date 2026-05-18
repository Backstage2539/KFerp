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
        <span>外部用户</span>
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
                <option v-if="unknownTemplateKey(row)" :value="unknownTemplateKey(row)">
                  未知模板：{{ unknownTemplateKey(row) }}
                </option>
                <option v-if="inactiveTemplateKey(row)" :value="inactiveTemplateKey(row)">
                  模板已失效：{{ selectedTemplate(row)?.label || inactiveTemplateKey(row) }}
                </option>
                <option v-for="template in activeTemplates" :key="template.key" :value="template.key">
                  {{ template.label }}
                </option>
              </select>
            </label>
          </div>
          <div class="bean-list-picker">
            <span>豆单展示版本</span>
            <label class="check">
              <input v-model="row.form.bean_list_mode" type="radio" value="latest" />
              <span>{{ row.beanListVersionOptions.length ? '展示客户最新版本' : '使用公共豆单' }}</span>
            </label>
            <label v-if="row.beanListVersionOptions.length" class="check">
              <input v-model="row.form.bean_list_mode" type="radio" value="fixed" />
              <span>固定指定版本</span>
            </label>
            <select
              v-if="row.beanListVersionOptions.length && row.form.bean_list_mode === 'fixed'"
              v-model.number="row.form.bean_list_publication_id"
            >
              <option v-for="item in row.beanListVersionOptions" :key="item.id" :value="item.id">
                {{ beanListVersionLabel(item) }}
              </option>
            </select>
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
          <template v-if="unknownTemplateKey(row)">
            <strong>未知能力模板</strong>
            <span>当前模板 key 无法识别，请重新选择系统模板；如果要停用门户并清空模板，请先选择“请选择模板”。</span>
          </template>
          <template v-else-if="inactiveTemplateKey(row)">
            <strong>模板已失效</strong>
            <span>当前客户引用的能力模板已经失效，请重新选择一个启用中的模板后保存。</span>
          </template>
          <template v-else-if="selectedTemplate(row)">
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
                <template v-for="key in selectedTemplate(row).erp_view_keys || []" :key="`${row.customer.id}-${key}`">
                  <button
                    v-if="key === 'customerProcessingPortal'"
                    type="button"
                    class="chip-link"
                    @click="openCustomerProcessingPortal"
                  >
                    {{ viewLabel(key) }}
                  </button>
                  <i v-else>{{ viewLabel(key) }}</i>
                </template>
              </div>
            </div>
          </template>
          <span v-else class="muted">请选择能力模板；客户的门户能力、规则和客户侧 ERP 页面都从模板继承。</span>
          <span v-if="row.loading" class="muted">加载模板中...</span>
        </div>

        <div class="external-user-cell">
          <form class="external-user-form" @submit.prevent="createExternalUser(row)">
            <label>
              <span>姓名</span>
              <input v-model.trim="row.externalUserForm.name" required />
            </label>
            <label>
              <span>手机号</span>
              <input v-model.trim="row.externalUserForm.phone" required />
            </label>
            <label>
              <span>初始密码</span>
              <input v-model.trim="row.externalUserForm.password" type="password" required />
            </label>
            <button class="secondary" type="submit" :disabled="row.externalUsersLoading">
              {{ row.externalUsersLoading ? '处理中' : '创建账号' }}
            </button>
          </form>
          <div class="external-user-list">
            <div v-for="user in row.externalUsers" :key="`${row.customer.id}-${user.employee_id}`" class="external-user-card">
              <div class="binding-row active-binding">
                <strong>{{ user.name || '-' }}</strong>
                <span>{{ user.phone || '未填手机号' }}</span>
              </div>
              <div class="chips">
                <i>{{ user.login_enabled ? '已启用登录' : '已停用登录' }}</i>
                <i>{{ user.has_password ? '已设密码' : '未设密码' }}</i>
                <i>{{ user.binding_status || '-' }}</i>
              </div>
              <div class="password-row">
                <input v-model.trim="row.externalUserPasswordMap[String(user.employee_id)]" type="password" placeholder="新密码" />
                <button class="secondary" type="button" @click="resetExternalUserPassword(row, user)" :disabled="row.externalUsersLoading || !row.externalUserPasswordMap[String(user.employee_id)]">
                  {{ user.has_password ? '重置密码' : '设置密码' }}
                </button>
              </div>
              <button class="secondary" type="button" @click="toggleExternalUserLogin(row, user)" :disabled="row.externalUsersLoading">
                {{ user.login_enabled ? '停用登录' : '启用登录' }}
              </button>
            </div>
            <span v-if="!row.externalUsers.length && !row.externalUsersLoading" class="muted">暂无外部用户</span>
            <span v-if="row.externalUsersLoading" class="muted">加载账号中...</span>
          </div>
          <p class="binding-hint">外部用户配置了手机号、密码并启用登录后，该客户会出现在客户履约运营台。</p>
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
import {
  createCustomerFulfillmentExternalUser,
  fetchCustomerFulfillmentExternalUsers,
  resetCustomerFulfillmentExternalUserPassword,
  setCustomerFulfillmentExternalUserLoginEnabled,
} from '../api/customer-fulfillment'

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
const themeLabels = {
  coffee_factory: '咖啡工厂专业风',
  clean_ops: '清爽业务工具风',
  premium_partner: '品牌会员高级风',
}

const activeTemplates = computed(() => (capabilityTemplates.value || []).filter((template) => template.active !== false))

async function loadCustomers() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = new URL('/api/customer-portal/admin/customers', window.location.origin)
    if (q.value) url.searchParams.set('q', q.value)
    const data = await apiGet(url)
    portalRows.value = (data.rows || []).map(createPortalRow)
    await Promise.all(portalRows.value.map((row) => Promise.all([loadRowDetail(row), loadRowExternalUsers(row)])))
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
      capability_template_key: trimTemplateKey(customer.capability_template_key),
      bean_list_mode: normalizeBeanListMode(customer.bean_list_mode),
      bean_list_publication_id: Number(customer.bean_list_publication_id || 0),
    },
    capabilities: [],
    bindings: [],
    beanListVersionOptions: [],
    externalUsers: [],
    externalUserForm: {
      name: '',
      phone: '',
      password: '',
    },
    externalUserPasswordMap: {},
    loading: false,
    saving: false,
    externalUsersLoading: false,
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
  row.form.capability_template_key = trimTemplateKey(row.customer.capability_template_key)
  row.form.bean_list_mode = normalizeBeanListMode(row.customer.bean_list_mode)
  row.form.bean_list_publication_id = Number(row.customer.bean_list_publication_id || 0)
  row.bindings = data?.bindings || []
  row.beanListVersionOptions = data?.bean_list_version_options || []
  syncRowBeanListVersion(row)
  row.capabilities = (data?.capabilities || []).map((item) => ({
    code: item.code,
    label: item.label || capabilityLabels[item.code] || item.code,
    description: item.description || '',
    enabled: !!item.enabled,
    config: item.config || {},
  }))
}

function assignExternalUsers(row, data) {
  row.externalUsers = Array.isArray(data?.users) ? data.users : []
}

async function loadRowExternalUsers(row) {
  row.externalUsersLoading = true
  try {
    const data = await fetchCustomerFulfillmentExternalUsers(row.customer.id)
    assignExternalUsers(row, data)
  } finally {
    row.externalUsersLoading = false
  }
}

async function saveVisibility(row) {
  if (!row?.customer?.id) return
  syncRowBeanListVersion(row)
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
        capability_template_key: trimTemplateKey(row.form.capability_template_key),
        bean_list_mode: row.form.bean_list_mode,
        bean_list_publication_id: Number(row.form.bean_list_publication_id || 0),
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

function normalizeTemplateKey(value) {
  const key = trimTemplateKey(value)
  return capabilityTemplates.value.some((template) => template.key === key) ? key : ''
}

function selectedTemplate(row) {
  const key = normalizeTemplateKey(row?.form?.capability_template_key)
  return capabilityTemplates.value.find((template) => template.key === key) || null
}

function trimTemplateKey(value) {
  return String(value || '').trim()
}

function normalizeBeanListMode(value) {
  return String(value || '').trim() === 'fixed' ? 'fixed' : 'latest'
}

function beanListVersionLabel(item) {
  const version = item?.version_no || `#${item?.id || ''}`
  const time = item?.published_at ? ` · ${item.published_at}` : ''
  return `${version}${time}`
}

function syncRowBeanListVersion(row) {
  if (!row?.beanListVersionOptions?.length) {
    row.form.bean_list_mode = 'latest'
    row.form.bean_list_publication_id = 0
    return
  }
  if (row.form.bean_list_mode !== 'fixed') {
    row.form.bean_list_publication_id = 0
    return
  }
  const currentID = Number(row.form.bean_list_publication_id || 0)
  if (row.beanListVersionOptions.some((item) => Number(item.id) === currentID)) return
  row.form.bean_list_publication_id = Number(row.beanListVersionOptions[0]?.id || 0)
}

function unknownTemplateKey(row) {
  const key = trimTemplateKey(row?.form?.capability_template_key)
  return key && !normalizeTemplateKey(key) ? key : ''
}

function inactiveTemplateKey(row) {
  const key = trimTemplateKey(row?.form?.capability_template_key)
  const template = capabilityTemplates.value.find((item) => item.key === key)
  return template && template.active === false ? key : ''
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

function openCustomerProcessingPortal() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: { key: 'customerProcessingPortal' },
  }))
}

function viewLabel(key) {
  if (key === 'customerProcessingPortal') return '客户履约工作台'
  return key
}

function canSaveRow(row) {
  return !row.saving && !row.loading && !unknownTemplateKey(row) && !inactiveTemplateKey(row) && (!row.form.enabled || !!selectedTemplate(row))
}

async function createExternalUser(row) {
  row.externalUsersLoading = true
  error.value = ''
  ok.value = ''
  try {
    const created = await createCustomerFulfillmentExternalUser(row.customer.id, {
      name: row.externalUserForm.name,
      phone: row.externalUserForm.phone,
      password: row.externalUserForm.password,
    })
    row.externalUserForm.name = ''
    row.externalUserForm.phone = ''
    row.externalUserForm.password = ''
    ok.value = `已为 ${row.customer.name} 创建外部用户 ${created.name || created.phone || ''}`.trim()
    await Promise.all([loadRowDetail(row), loadRowExternalUsers(row)])
  } catch (err) {
    error.value = err.message || '创建外部用户失败'
  } finally {
    row.externalUsersLoading = false
  }
}

async function resetExternalUserPassword(row, user) {
  const employeeID = Number(user?.employee_id || 0)
  const password = row.externalUserPasswordMap[String(employeeID)] || ''
  if (!employeeID || !password) return
  row.externalUsersLoading = true
  error.value = ''
  ok.value = ''
  try {
    await resetCustomerFulfillmentExternalUserPassword(row.customer.id, employeeID, password)
    row.externalUserPasswordMap[String(employeeID)] = ''
    ok.value = `已更新 ${user.name || user.phone || employeeID} 的密码`
    await Promise.all([loadRowDetail(row), loadRowExternalUsers(row)])
  } catch (err) {
    error.value = err.message || '密码更新失败'
  } finally {
    row.externalUsersLoading = false
  }
}

async function toggleExternalUserLogin(row, user) {
  const employeeID = Number(user?.employee_id || 0)
  if (!employeeID) return
  row.externalUsersLoading = true
  error.value = ''
  ok.value = ''
  try {
    const nextEnabled = !user.login_enabled
    await setCustomerFulfillmentExternalUserLoginEnabled(row.customer.id, employeeID, nextEnabled)
    ok.value = nextEnabled ? '已启用外部用户登录' : '已停用外部用户登录'
    await Promise.all([loadRowDetail(row), loadRowExternalUsers(row)])
  } catch (err) {
    error.value = err.message || '登录状态保存失败'
  } finally {
    row.externalUsersLoading = false
  }
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
.list-head, .portal-row { display: grid; grid-template-columns: 190px 280px minmax(320px, 1fr) minmax(300px, 360px) 210px; gap: 14px; align-items: start; }
.list-head { padding: 8px 10px; background: #f8fafc; border-bottom: 1px solid #eef1f4; color: #666; font-size: 13px; font-weight: 700; }
.portal-row { padding: 14px 10px; border-bottom: 1px solid #eef1f4; }
.portal-row:last-child { border-bottom: 0; }
.customer-cell, .config-cell, .binding-list, .external-user-cell { display: flex; flex-direction: column; gap: 8px; }
.customer-cell strong { font-size: 15px; }
.customer-cell span, .binding-row span { color: #666; font-size: 13px; line-height: 1.4; }
.config-cell input, .config-cell select { width: 100%; }
.template-picker { display: grid; gap: 8px; align-items: end; }
.bean-list-picker { display: grid; gap: 7px; border: 1px solid #e4e7ec; border-radius: 8px; padding: 8px; background: #f8fafc; }
.bean-list-picker > span { color: #555; font-size: 12px; margin: 0; }
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
.chip-link {
  font-style: normal;
  font-size: 12px;
  line-height: 1.3;
  border: 1px solid #dde3ea;
  border-radius: 999px;
  background: #f8fafc;
  color: #1f4f82;
  padding: 4px 8px;
  display: inline-flex;
  align-items: center;
  cursor: pointer;
  font-family: inherit;
}
.binding-row { border: 1px solid #e4e7ec; border-radius: 8px; padding: 8px; }
.binding-row strong { font-size: 13px; }
.external-user-form, .external-user-list { display: grid; gap: 8px; }
.external-user-form input { width: 100%; }
.external-user-card { border: 1px solid #e4e7ec; border-radius: 8px; padding: 8px; display: grid; gap: 8px; }
.password-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; align-items: center; }
.password-row input { width: 100%; }
.binding-hint { margin: 0; color: #8a5a16; background: #fff7e6; border: 1px solid #f0d7a0; border-radius: 6px; padding: 7px 8px; font-size: 12px; line-height: 1.45; }
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
