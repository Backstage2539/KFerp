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
          <input v-model.trim="q" placeholder="客户/联系人/电话/地址" @keyup.enter="loadPage(1)" />
        </label>
        <button class="primary" type="button" @click="loadPage(1)" :disabled="loading">查询</button>
      </div>
    </section>

    <section v-if="formVisible" class="panel">
      <div class="panel-head">
        <h3>{{ editingId ? '编辑客户' : '新增客户' }}</h3>
        <button class="secondary" type="button" @click="closeForm">关闭</button>
      </div>
      <form class="form-grid" @submit.prevent="saveCustomer">
        <label>
          <span>客户名</span>
          <input v-model.trim="form.name" required />
        </label>
        <label>
          <span>原始名称</span>
          <input v-model.trim="form.raw_name" />
        </label>
        <label>
          <span>联系人</span>
          <input v-model.trim="form.contact" />
        </label>
        <label>
          <span>电话</span>
          <input v-model.trim="form.phone" />
        </label>
        <label>
          <span>默认来源</span>
          <select v-model.number="form.default_source_id">
            <option :value="0">未设置</option>
            <option v-for="item in sources" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
        <label>
          <span>默认订单类型</span>
          <select v-model.number="form.default_order_type_id">
            <option :value="0">未设置</option>
            <option v-for="item in orderTypes" :key="item.id" :value="item.id">{{ item.name }}</option>
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
        <div class="form-actions">
          <button class="primary" type="submit" :disabled="loading">保存</button>
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
    </section>

    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>客户</th>
              <th>联系人</th>
              <th>电话</th>
              <th>地址</th>
              <th>默认来源</th>
              <th>默认订单类型</th>
              <th>状态</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id" :class="{ active: row.id === editingId }">
              <td>{{ row.name }}</td>
              <td>{{ row.contact || '' }}</td>
              <td>{{ row.phone || '' }}</td>
              <td class="address">{{ row.address || '' }}</td>
              <td>{{ optionName(sources, row.default_source_id) }}</td>
              <td>{{ optionName(orderTypes, row.default_order_type_id) }}</td>
              <td>{{ row.active ? '启用' : '停用' }}</td>
              <td>{{ row.updated }}</td>
              <td><button class="text-button" type="button" @click="editCustomer(row.id)">编辑</button></td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="9" class="muted">暂无客户</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="pager">
        <button class="secondary" type="button" @click="loadPage(page - 1)" :disabled="!hasPrev || loading">上一页</button>
        <span>第 {{ page }} 页</span>
        <button class="secondary" type="button" @click="loadPage(page + 1)" :disabled="!hasNext || loading">下一页</button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { replaceHistoryURL } from '../lib/url-state'

const rows = ref([])
const sources = ref([])
const orderTypes = ref([])
const q = ref('')
const page = ref(1)
const hasPrev = ref(false)
const hasNext = ref(false)
const loading = ref(false)
const error = ref('')
const ok = ref('')
const formVisible = ref(false)
const editingId = ref(0)
const assets = ref([])
const dashboard = reactive({ total_orders: 0, unpaid_orders: 0, unshipped_orders: 0, in_production: 0, in_shipping: 0, completed: 0 })
const assetKind = ref('label_front')
const assetInput = ref(null)
const form = reactive(emptyForm())
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
    raw_name: '',
    contact: '',
    phone: '',
    address: '',
    default_source_id: 0,
    default_order_type_id: 0,
    active: true,
  }
}

function assignForm(data) {
  Object.assign(form, {
    name: data?.name || '',
    raw_name: data?.raw_name || '',
    contact: data?.contact || '',
    phone: data?.phone || '',
    address: data?.address || '',
    default_source_id: Number(data?.default_source_id || 0),
    default_order_type_id: Number(data?.default_order_type_id || 0),
    active: data?.active !== false,
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
  page.value = Math.max(1, Number(params.get('page') || 1))
}

function updateUrl(extra = {}) {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'customers')
  if (q.value) url.searchParams.set('q', q.value)
  else url.searchParams.delete('q')
  url.searchParams.set('page', String(page.value))
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

async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/customers', window.location.origin)
    if (q.value) url.searchParams.set('q', q.value)
    url.searchParams.set('page', String(page.value))
    const data = await apiGet(url)
    rows.value = data.rows || []
    sources.value = data.sources || []
    orderTypes.value = data.order_types || []
    hasPrev.value = !!data.has_prev
    hasNext.value = !!data.has_next
    page.value = Number(data.page || page.value)
    updateUrl(formVisible.value ? (editingId.value ? { edit_id: editingId.value } : { mode: 'new' }) : {})
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function startNew() {
  formVisible.value = true
  editingId.value = 0
  assets.value = []
  assignDashboard()
  assignForm(emptyForm())
  ok.value = ''
  error.value = ''
  updateUrl({ mode: 'new' })
}

function closeForm() {
  formVisible.value = false
  editingId.value = 0
  assets.value = []
  updateUrl()
}

async function editCustomer(id) {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiGet(`/api/customers/${id}`)
    formVisible.value = true
    editingId.value = Number(data.customer.id)
    assignForm(data.customer)
    sources.value = data.sources || sources.value
    orderTypes.value = data.order_types || orderTypes.value
    assets.value = data.assets || []
    assignDashboard(data.dashboard)
    updateUrl({ edit_id: editingId.value })
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function saveCustomer() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const body = {
      name: form.name,
      raw_name: form.raw_name,
      contact: form.contact,
      phone: form.phone,
      address: form.address,
      default_source_id: form.default_source_id || null,
      default_order_type_id: form.default_order_type_id || null,
      active: !!form.active,
    }
    const data = await apiSend(editingId.value ? `/api/customers/${editingId.value}` : '/api/customers', {
      method: editingId.value ? 'PUT' : 'POST',
      body,
    })
    formVisible.value = true
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
    await editCustomer(editingId.value)
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
    await editCustomer(editingId.value)
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
  if (editID > 0) await editCustomer(editID)
  else if (newMode) startNew()
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .filters, .pager, .actions, .asset-form { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.panel-head { justify-content: space-between; margin-bottom: 12px; }
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
.pager { justify-content: flex-end; margin-top: 12px; }
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
