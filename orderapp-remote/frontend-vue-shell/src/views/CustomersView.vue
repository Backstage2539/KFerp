<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>客户档案</h2>
        <div class="actions">
          <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
          <button class="primary" type="button" @click="startNew">新增客户</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">{{ ok }}</div>
      <div class="filters">
        <label>
          <span>搜索</span>
          <input v-model.trim="q" placeholder="客户/公司/联系人/联系电话/地址" @keyup.enter="loadPage(1)" />
        </label>
        <label>
          <span>客户类型</span>
          <select v-model="customerTypeFilter">
            <option value="">全部类型</option>
            <option v-for="item in customerTypeOptionsForSelect" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </label>
        <label>
          <span>启用状态</span>
          <select v-model="activeFilter">
            <option value="">全部</option>
            <option value="true">启用</option>
            <option value="false">停用</option>
          </select>
        </label>
        <button class="primary" type="button" @click="loadPage(1)" :disabled="loading">查询</button>
      </div>
    </section>

    <div v-if="customerDrawerOpen" class="drawer-mask" @click.self="closeCustomerDrawer">
      <aside class="customer-drawer" role="dialog" aria-modal="true">
      <div class="drawer-head">
        <h3>{{ editingId ? '编辑客户' : '新增客户' }}</h3>
        <button class="secondary" type="button" @click="closeCustomerDrawer">关闭</button>
      </div>
      <form class="form-grid" @submit.prevent="saveCustomer">
        <label class="wide">
          <span>粘贴收件信息</span>
          <textarea v-model.trim="customerPaste" rows="2" placeholder="张三 13800138000 云南省普洱市思茅区咖啡路 88 号"></textarea>
        </label>
        <button class="secondary parse-button" type="button" :disabled="addressParsing || !addressParseAvailable" @click="applyAddressParse">
          {{ addressParsing ? '解析中...' : '地址解析' }}
        </button>
        <label>
          <span>客户名</span>
          <input v-model.trim="form.name" required />
        </label>
        <label>
          <span>客户类型</span>
          <div class="select-with-add">
            <select v-model="form.customer_type" required>
              <option v-for="item in customerTypeOptionsForSelect" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
            <button class="icon-button" type="button" title="新增客户类型" aria-label="新增客户类型" @click="createCustomerTypeOption">+</button>
          </div>
        </label>
        <label>
          <span>公司名称</span>
          <input v-model.trim="form.company_name" placeholder="不填则销售单默认使用客户名" />
        </label>
        <label>
          <span>联系电话</span>
          <input v-model.trim="form.company_phone" />
        </label>
        <label>
          <span>联系人</span>
          <input v-model.trim="form.contact" />
        </label>
        <label>
          <span>来源</span>
          <select v-model.number="form.default_source_id" required>
            <option v-for="item in sources" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
        <label>
          <span>订单类型</span>
          <div class="select-with-add">
            <select v-model.number="form.default_order_type_id" required>
              <option v-for="item in orderTypes" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
            <button class="icon-button" type="button" title="新增订单类型" aria-label="新增订单类型" @click="createOrderTypeOption">+</button>
          </div>
        </label>
        <label>
          <span>负责人</span>
          <select v-model.number="form.responsible_employee_id">
            <option :value="0">选择员工</option>
            <option v-for="employee in employees" :key="employee.id" :value="employee.id">{{ employee.name }}</option>
          </select>
        </label>
        <label class="wide">
          <span>地址</span>
          <textarea v-model.trim="form.address" rows="3"></textarea>
        </label>
        <label class="check">
          <input v-model="form.active" type="checkbox" />
          <span>启用</span>
        </label>
        <label class="check">
          <input v-model="form.portal_enabled" type="checkbox" />
          <span>开通客户门户/工作台</span>
        </label>
        <div class="form-actions">
          <button class="primary" type="submit" :disabled="loading || addressParsing">保存</button>
        </div>
      </form>

      <div v-if="editingId" class="customer-extra">
        <div class="stats">
          <div><span>订单</span><strong>{{ dashboard.total_orders }}</strong></div>
          <div><span>未收款</span><strong>{{ dashboard.unpaid_orders }}</strong></div>
          <div><span>未发货</span><strong>{{ dashboard.unshipped_orders }}</strong></div>
          <div><span>生产中</span><strong>{{ dashboard.in_production }}</strong></div>
          <div><span>发货中</span><strong>{{ dashboard.in_shipping }}</strong></div>
          <div><span>已完成</span><strong>{{ dashboard.completed }}</strong></div>
        </div>

        <form class="asset-form" @submit.prevent="uploadAsset">
          <label>
            <span>附件类型</span>
            <select v-model="assetKind">
              <option v-for="kind in assetKinds" :key="kind.value" :value="kind.value">{{ kind.label }}</option>
            </select>
          </label>
          <label>
            <span>文件</span>
            <input ref="assetInput" type="file" accept="image/jpeg,image/png,image/webp" />
          </label>
          <button class="primary" type="submit" :disabled="loading">上传</button>
        </form>

        <div class="assets">
          <div v-for="asset in assets" :key="asset.id" class="asset">
            <a :href="asset.url" target="_blank">
              <img :src="asset.url" alt="" />
            </a>
            <div class="asset-meta">
              <strong>{{ assetKindLabel(asset.kind) }}</strong>
              <span>{{ asset.created_at }} · {{ bytes(asset.bytes) }}</span>
            </div>
            <button class="text-button" type="button" @click="deleteAsset(asset.id)">删除</button>
          </div>
          <div v-if="!assets.length" class="muted">暂无附件</div>
        </div>
      </div>
      </aside>
    </div>

    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th class="sortable" @click="setSort('name')">
                客户
                <span class="sort-icons">
                  <span :class="{ active: sortBy === 'name' && sortDirection === 'asc' }">▲</span>
                  <span :class="{ active: sortBy === 'name' && sortDirection === 'desc' }">▼</span>
                </span>
              </th>
              <th>类型</th>
              <th>公司</th>
              <th>联系电话</th>
              <th>联系人</th>
              <th>地址</th>
              <th>来源</th>
              <th>订单类型</th>
              <th>负责人</th>
              <th>门户/工作台</th>
              <th>状态</th>
              <th class="sortable" @click="setSort('updated')">
                更新时间
                <span class="sort-icons">
                  <span :class="{ active: sortBy === 'updated' && sortDirection === 'asc' }">▲</span>
                  <span :class="{ active: sortBy === 'updated' && sortDirection === 'desc' }">▼</span>
                </span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id" :class="{ active: row.id === editingId }">
              <td>
                <button class="name-button" type="button" @click="openCustomerDrawer(row.id)">{{ row.name }}</button>
              </td>
              <td>{{ displayCustomerTypeLabel(row.customer_type) }}</td>
              <td>{{ row.company_name || row.name }}</td>
              <td>{{ customerPhoneForERPForm(row) }}</td>
              <td>{{ row.contact || '' }}</td>
              <td class="address">{{ row.address || '' }}</td>
              <td>{{ optionName(sources, row.default_source_id) }}</td>
              <td>{{ optionName(orderTypes, row.default_order_type_id) }}</td>
              <td>{{ row.responsible_employee_name || employeeName(row.responsible_employee_id) }}</td>
              <td>{{ row.portal_enabled ? '已开通' : '未开通' }}</td>
              <td>{{ row.active ? '启用' : '停用' }}</td>
              <td>{{ row.updated }}</td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="12" class="muted">暂无客户</td>
            </tr>
          </tbody>
        </table>
      </div>
      <PaginationControls
        :page="page"
        :page-size="pageSize"
        :total="totalCustomers"
        :disabled="loading"
        @change="handlePaginationChange"
      />
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import PaginationControls from '../components/PaginationControls.vue'
import { customerPhoneForERPForm, customerRecipientFieldSnapshot, mergeCustomerRecipientFields } from '../lib/customer-recipient-merge'
import { customerTypeLabel, customerTypeOptions, mergeCustomerTypeOptions, normalizeCustomerType, validCustomerType } from '../lib/customer-types'
import { normalizePageSize, paginationFromApi } from '../lib/pagination'
import { replaceHistoryURL } from '../lib/url-state'

const rows = ref([])
const sources = ref([])
const orderTypes = ref([])
const customerTypeOptionsForSelect = ref(mergeCustomerTypeOptions(customerTypeOptions))
const employees = ref([])
const q = ref('')
const page = ref(1)
const pageSize = ref(10)
const totalCustomers = ref(0)
const hasPrev = ref(false)
const hasNext = ref(false)
const customerTypeFilter = ref('')
const activeFilter = ref('')
const sortBy = ref('name')
const sortDirection = ref('asc')
const loading = ref(false)
const error = ref('')
const ok = ref('')
const customerDrawerOpen = ref(false)
const editingId = ref(0)
const assets = ref([])
const dashboard = reactive({ total_orders: 0, unpaid_orders: 0, unshipped_orders: 0, in_production: 0, in_shipping: 0, completed: 0 })
const assetKind = ref('label_front')
const assetInput = ref(null)
const customerPaste = ref('')
const addressParsing = ref(false)
const form = reactive(emptyForm())
const addressParseAvailable = computed(() => Boolean(String(customerPaste.value || form.address || '').trim()))
let addressParseSequence = 0
const assetKinds = [
  { value: 'label_front', label: '标签-正面' },
  { value: 'label_back', label: '标签-反面' },
  { value: 'bag', label: '豆袋' },
  { value: 'drip_box', label: '挂耳盒' },
  { value: 'print_requirement', label: '印刷需求' },
]

function emptyForm() {
  return {
    name: '',
    customer_type: 'retail',
    company_name: '',
    company_phone: '',
    contact: '',
    address: '',
    default_source_id: 0,
    default_order_type_id: 0,
    responsible_employee_id: 0,
    active: true,
    portal_enabled: false,
  }
}

function assignForm(data) {
  Object.assign(form, {
    name: data?.name || '',
    customer_type: normalizeCustomerType(data?.customer_type, customerTypeOptionsForSelect.value),
    company_name: data?.company_name || '',
    company_phone: customerPhoneForERPForm(data),
    contact: data?.contact || '',
    address: data?.address || '',
    default_source_id: Number(data?.default_source_id || 0),
    default_order_type_id: Number(data?.default_order_type_id || 0),
    responsible_employee_id: Number(data?.responsible_employee_id || 0),
    active: data?.active !== false,
    portal_enabled: data?.portal_enabled === true,
  })
}

function assignDashboard(data = {}) {
  Object.assign(dashboard, {
    total_orders: Number(data.total_orders || 0),
    unpaid_orders: Number(data.unpaid_orders || 0),
    unshipped_orders: Number(data.unshipped_orders || 0),
    in_production: Number(data.in_production || 0),
    in_shipping: Number(data.in_shipping || 0),
    completed: Number(data.completed || 0),
  })
}

function optionName(options, id) {
  const item = options.find((x) => Number(x.id) === Number(id))
  return item?.name || ''
}

function assignCustomerTypeOptions(data = {}) {
  customerTypeOptionsForSelect.value = mergeCustomerTypeOptions(data.customer_type_options || customerTypeOptionsForSelect.value)
}

function displayCustomerTypeLabel(value) {
  return customerTypeLabel(value, customerTypeOptionsForSelect.value)
}

function employeeName(id) {
  const item = employees.value.find((x) => Number(x.id) === Number(id))
  return item?.name || ''
}

function defaultOptionID(options, labels) {
  const row = (options || []).find((item) => labels.some((label) => String(item.name || '').includes(label)))
  return Number(row?.id || options?.[0]?.id || 0)
}

function defaultSourceID() {
  return defaultOptionID(sources.value, ['微信', 'Wechat', 'wechat'])
}

function defaultOrderTypeID() {
  return defaultOptionID(orderTypes.value, ['批发', 'Wholesale', 'wholesale'])
}

function applyFormDefaults() {
  if (!form.default_source_id && sources.value.length) form.default_source_id = defaultSourceID()
  if (!form.default_order_type_id && orderTypes.value.length) form.default_order_type_id = defaultOrderTypeID()
}

function assetKindLabel(kind) {
  return assetKinds.find((item) => item.value === kind)?.label || kind
}

function bytes(value) {
  const n = Number(value || 0)
  if (n >= 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`
  if (n >= 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${n} B`
}

function applyUrl() {
  const params = new URL(window.location.href).searchParams
  q.value = params.get('q') || ''
  const parsedPage = Number(params.get('page') || 1)
  page.value = Number.isFinite(parsedPage) && parsedPage > 0 ? parsedPage : 1
  pageSize.value = normalizePageSize(params.get('limit') || pageSize.value)
  customerTypeFilter.value = normalizeListCustomerType(params.get('customer_type'))
  activeFilter.value = normalizeActiveFilter(params.get('active'))
  sortBy.value = normalizeCustomerSortBy(params.get('sort_by'))
  sortDirection.value = normalizeCustomerSortDirection(params.get('sort_direction'))
}

function normalizeListCustomerType(value) {
  const v = String(value || '').trim().toLowerCase()
  return v
}

function normalizeActiveFilter(value) {
  const v = String(value || '').trim().toLowerCase()
  if (v === 'true' || v === '1' || v === 'on' || v === 'yes' || v === 'enabled' || v === 'active' || v === 'y') return 'true'
  if (v === 'false' || v === '0' || v === 'off' || v === 'no' || v === 'disabled' || v === 'inactive' || v === 'n') return 'false'
  return ''
}

function normalizeCustomerSortBy(value) {
  const v = String(value || '').trim().toLowerCase()
  if (v === 'name' || v === 'updated') return v
  return 'name'
}

function normalizeCustomerSortDirection(value) {
  const v = String(value || '').trim().toLowerCase()
  if (v === 'desc') return 'desc'
  return 'asc'
}

function updateUrl(extra = {}) {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'customers')
  if (q.value) url.searchParams.set('q', q.value)
  else url.searchParams.delete('q')
  if (customerTypeFilter.value) url.searchParams.set('customer_type', customerTypeFilter.value)
  else url.searchParams.delete('customer_type')
  if (activeFilter.value) url.searchParams.set('active', activeFilter.value)
  else url.searchParams.delete('active')
  url.searchParams.set('sort_by', sortBy.value)
  url.searchParams.set('sort_direction', sortDirection.value)
  url.searchParams.set('page', String(page.value))
  url.searchParams.set('limit', String(pageSize.value))
  url.searchParams.delete('mode')
  url.searchParams.delete('edit_id')
  if (extra.mode) url.searchParams.set('mode', extra.mode)
  if (extra.edit_id) url.searchParams.set('edit_id', String(extra.edit_id))
  replaceHistoryURL(url)
}

async function loadPage(nextPage) {
  page.value = Math.max(1, nextPage)
  await load()
}

async function handlePaginationChange({ page: nextPage, pageSize: nextPageSize }) {
  pageSize.value = normalizePageSize(nextPageSize)
  await loadPage(nextPage)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/customers', window.location.origin)
    if (q.value) url.searchParams.set('q', q.value)
    if (customerTypeFilter.value) url.searchParams.set('customer_type', customerTypeFilter.value)
    if (activeFilter.value) url.searchParams.set('active', activeFilter.value)
    url.searchParams.set('sort_by', sortBy.value)
    url.searchParams.set('sort_direction', sortDirection.value)
    url.searchParams.set('page', String(page.value))
    url.searchParams.set('limit', String(pageSize.value))
    const data = await apiGet(url)
    assignCustomerTypeOptions(data)
    rows.value = data.rows || []
    sources.value = data.sources || []
    orderTypes.value = data.order_types || []
    employees.value = data.employees || []
    if (customerDrawerOpen.value && !editingId.value) applyFormDefaults()
    const pagination = paginationFromApi(data)
    totalCustomers.value = pagination.total
    hasPrev.value = pagination.hasPrev
    hasNext.value = pagination.hasNext
    page.value = pagination.page
    pageSize.value = pagination.pageSize
    updateUrl(customerDrawerOpen.value ? (editingId.value ? { edit_id: editingId.value } : { mode: 'new' }) : {})
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function setSort(field) {
  if (sortBy.value !== field) {
    sortBy.value = field
    sortDirection.value = 'asc'
  } else if (sortDirection.value === 'asc') {
    sortDirection.value = 'desc'
  } else {
    sortDirection.value = 'asc'
  }
  await loadPage(1)
}

function startNew() {
  invalidateAddressParse()
  customerDrawerOpen.value = true
  editingId.value = 0
  assets.value = []
  assignDashboard()
  assignForm(emptyForm())
  applyFormDefaults()
  customerPaste.value = ''
  ok.value = ''
  error.value = ''
  updateUrl({ mode: 'new' })
}

function closeCustomerDrawer() {
  invalidateAddressParse()
  customerDrawerOpen.value = false
  editingId.value = 0
  assets.value = []
  customerPaste.value = ''
  updateUrl()
}

async function openCustomerDrawer(id) {
  invalidateAddressParse()
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiGet(`/api/customers/${id}`)
    customerDrawerOpen.value = true
    editingId.value = Number(data.customer.id)
    assignCustomerTypeOptions(data)
    sources.value = data.sources || sources.value
    orderTypes.value = data.order_types || orderTypes.value
    employees.value = data.employees || employees.value
    assignForm(data.customer)
    assets.value = data.assets || []
    assignDashboard(data.dashboard)
    customerPaste.value = ''
    updateUrl({ edit_id: editingId.value })
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function invalidateAddressParse() {
  addressParseSequence += 1
  addressParsing.value = false
}

function isCurrentAddressParse(sequence, targetEditingID, source) {
  const currentSource = String(customerPaste.value || form.address || '').trim()
  return sequence === addressParseSequence &&
    customerDrawerOpen.value &&
    Number(editingId.value || 0) === targetEditingID &&
    currentSource === source
}

async function applyAddressParse() {
  const source = String(customerPaste.value || form.address || '').trim()
  if (!source || addressParsing.value) return
  const sequence = ++addressParseSequence
  const targetEditingID = Number(editingId.value || 0)
  const targetFieldsAtRequest = customerRecipientFieldSnapshot(form)
  addressParsing.value = true
  error.value = ''
  try {
    const parsed = await apiSend('/api/customer-recipient/parse', { body: { text: source } })
    if (!isCurrentAddressParse(sequence, targetEditingID, source)) return
    Object.assign(form, mergeCustomerRecipientFields(form, parsed, targetFieldsAtRequest))
  } catch (err) {
    if (isCurrentAddressParse(sequence, targetEditingID, source)) error.value = err.message || '地址解析失败'
  } finally {
    if (sequence === addressParseSequence) addressParsing.value = false
  }
}

async function saveCustomer() {
  if (addressParsing.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    if (!String(form.name || '').trim()) throw new Error('请填写客户名称')
    if (!validCustomerType(form.customer_type, customerTypeOptionsForSelect.value)) throw new Error('请选择客户类型')
    if (!Number(form.default_source_id || 0)) throw new Error('请选择客户来源')
    if (!Number(form.default_order_type_id || 0)) throw new Error('请选择客户订单类型')
    if (!Number(form.responsible_employee_id || 0)) throw new Error('请选择客户负责人')
    const body = {
      name: form.name,
      raw_name: '',
      customer_type: normalizeCustomerType(form.customer_type, customerTypeOptionsForSelect.value),
      company_name: form.company_name,
      company_address: '',
      company_phone: form.company_phone,
      contact: form.contact,
      phone: form.company_phone,
      address: form.address,
      default_source_id: form.default_source_id || null,
      default_order_type_id: form.default_order_type_id || null,
      responsible_employee_id: form.responsible_employee_id || null,
      active: !!form.active,
      portal_enabled: !!form.portal_enabled,
    }
    const data = await apiSend(editingId.value ? `/api/customers/${editingId.value}` : '/api/customers', {
      method: editingId.value ? 'PUT' : 'POST',
      body,
    })
    customerDrawerOpen.value = true
    editingId.value = Number(data.customer.id)
    assignForm(data.customer)
    assets.value = data.assets || []
    assignDashboard(data.dashboard)
    ok.value = '已保存'
    await load()
    updateUrl({ edit_id: editingId.value })
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    loading.value = false
  }
}

async function createCustomerTypeOption() {
  const label = String(window.prompt('新增客户类型') || '').trim()
  if (!label) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const option = await apiSend('/api/customers/customer-types', {
      body: { label },
    })
    customerTypeOptionsForSelect.value = mergeCustomerTypeOptions([...customerTypeOptionsForSelect.value, option])
    form.customer_type = option.value
    ok.value = `已新增客户类型：${option.label || label}`
  } catch (err) {
    error.value = err.message || '新增客户类型失败'
  } finally {
    loading.value = false
  }
}

async function createOrderTypeOption() {
  const name = String(window.prompt('新增订单类型') || '').trim()
  if (!name) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const option = await apiSend('/api/customers/order-types', {
      body: { name },
    })
    if (!orderTypes.value.some((item) => Number(item.id) === Number(option.id))) {
      orderTypes.value = [...orderTypes.value, option]
    }
    form.default_order_type_id = Number(option.id || 0)
    ok.value = `已新增订单类型：${option.name || name}`
  } catch (err) {
    error.value = err.message || '新增订单类型失败'
  } finally {
    loading.value = false
  }
}

async function uploadAsset() {
  if (!editingId.value) return
  const file = assetInput.value?.files?.[0]
  if (!file) {
    error.value = '请选择文件'
    return
  }
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const fd = new FormData()
    fd.append('kind', assetKind.value)
    fd.append('file', file)
    await apiSend(`/customers/${editingId.value}/assets/upload`, { body: fd })
    assetInput.value.value = ''
    ok.value = '已上传'
    await openCustomerDrawer(editingId.value)
  } catch (err) {
    error.value = err.message || '上传失败'
  } finally {
    loading.value = false
  }
}

async function deleteAsset(id) {
  if (!editingId.value) return
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const body = new URLSearchParams({ asset_id: String(id) })
    await apiSend(`/customers/${editingId.value}/assets/delete`, { body })
    ok.value = '已删除附件'
    await openCustomerDrawer(editingId.value)
  } catch (err) {
    error.value = err.message || '删除失败'
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  applyUrl()
  const params = new URL(window.location.href).searchParams
  const editID = Number(params.get('edit_id') || 0)
  const newMode = params.get('mode') === 'new'
  await load()
  if (editID > 0) await openCustomerDrawer(editID)
  else if (newMode) startNew()
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .drawer-head, .filters, .actions, .asset-form { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.sortable { cursor: pointer; white-space: nowrap; user-select: none; }
.sort-icons { display: inline-flex; flex-direction: column; font-size: 10px; margin-left: 6px; vertical-align: middle; color: #bdb1a5; line-height: 1; }
.sort-icons span { opacity: 0.35; }
.sort-icons span.active { color: #1f4f82; opacity: 1; font-weight: 700; }
.sort-icons span + span { margin-top: 2px; }
.panel-head { justify-content: space-between; margin-bottom: 12px; }
.drawer-mask { position: fixed; inset: 0; z-index: 40; display: flex; justify-content: flex-end; background: rgba(20, 20, 20, .32); }
.customer-drawer { width: min(760px, 100vw); height: 100vh; overflow: auto; background: #fff; padding: 18px; box-shadow: -18px 0 38px rgba(20, 20, 20, .18); }
.drawer-head { position: sticky; top: 0; z-index: 2; justify-content: space-between; padding-bottom: 12px; margin-bottom: 14px; border-bottom: 1px solid #eee8df; background: #fff; }
.customer-drawer input, .customer-drawer select, .customer-drawer textarea { width: 100%; }
.select-with-add { display: grid; grid-template-columns: minmax(0, 1fr) 38px; gap: 6px; align-items: center; }
.icon-button { width: 38px; padding: 0; font-size: 18px; line-height: 1; font-weight: 700; }
h2, h3 { margin: 0; font-size: 20px; }
h3 { font-size: 18px; }
label span, .stats span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select, textarea { width: min(420px, 70vw); border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
input, select { height: 38px; }
textarea { min-height: 78px; resize: vertical; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.text-button { height: 30px; border: 0; background: transparent; color: #1f4f82; padding: 0; }
.name-button { height: auto; border: 0; background: transparent; color: #1f4f82; padding: 0; text-align: left; text-decoration: underline; text-underline-offset: 2px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(220px, 1fr)); gap: 12px; align-items: end; }
.wide { grid-column: 1 / -1; }
.wide textarea { width: 100%; }
.check { display: flex; align-items: center; gap: 8px; align-self: center; }
.check input { width: auto; height: auto; }
.check span { margin: 0; }
.form-actions { align-self: center; }
.customer-extra { margin-top: 14px; border-top: 1px solid #eee8df; padding-top: 14px; }
.stats { display: grid; grid-template-columns: repeat(6, minmax(88px, 1fr)); gap: 8px; margin-bottom: 14px; }
.stats div { border: 1px solid #eee8df; border-radius: 6px; padding: 9px; }
.stats strong { font-size: 18px; }
.assets { display: grid; grid-template-columns: repeat(auto-fill, minmax(190px, 1fr)); gap: 10px; margin-top: 12px; }
.asset { border: 1px solid #eee8df; border-radius: 8px; padding: 9px; }
.asset img { width: 100%; height: 150px; object-fit: contain; background: #f7f7f7; border-radius: 6px; }
.asset-meta { display: grid; gap: 3px; margin: 7px 0; }
.asset-meta span { color: #666; font-size: 12px; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 1080px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
tr.active { background: #f3f7fb; }
.address { max-width: 300px; white-space: pre-wrap; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .filters { align-items: end; }
  .form-grid { grid-template-columns: 1fr; }
  .stats { grid-template-columns: repeat(2, minmax(100px, 1fr)); }
  table { min-width: 940px; }
}
</style>
