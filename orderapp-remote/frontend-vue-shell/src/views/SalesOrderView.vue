<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>销售单</h2>
        <div class="actions">
          <a class="secondary link-button" href="/vue-shell?view=orders">返回订单列表</a>
          <button class="secondary" type="button" @click="openCustomerDrawer" :disabled="!customerSummary.id">客户信息</button>
          <a v-if="documents.length" class="secondary link-button" :href="salesOrderDownloadUrl(orderID)" target="_blank" rel="noopener">下载最新版</a>
          <button class="primary" type="button" @click="generate" :disabled="generating || !orderID">{{ generating ? '生成中' : '生成销售单 PDF' }}</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="message" class="ok">{{ message }}</div>
      <div class="summary">
        <span>订单 ID：{{ orderID || '-' }}</span>
        <span>客户：{{ customerSummary.name || '-' }}</span>
        <span>公司：{{ customerSummary.company_name || customerSummary.name || '-' }}</span>
        <span>版本数：{{ documents.length }}</span>
      </div>
      <details class="manual">
        <summary>销售单手册</summary>
        <ul>
          <li>首次生成销售单为 V1，同一订单再次生成会创建 V2，不覆盖旧文件。</li>
          <li>生成前可点“客户信息”维护客户公司名称、公司地址、联系电话；公司名称为空时默认使用客户名。</li>
          <li>销售单内容按生成时的订单和设置保存快照，后续修改设置不会改动旧版本。</li>
          <li>需要给客户最新文件时使用“下载最新版”，需要追溯时下载指定历史版本。</li>
        </ul>
      </details>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>历史版本</h3>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <table>
        <thead>
          <tr><th>版本</th><th>订单号</th><th>生成时间</th><th>操作人</th><th>状态</th><th>操作</th></tr>
        </thead>
        <tbody>
          <tr v-for="doc in documents" :key="doc.id">
            <td>V{{ doc.version_no }}</td>
            <td>{{ doc.order_no }}</td>
            <td>{{ doc.created_at }}</td>
            <td>{{ doc.created_by || '-' }}</td>
            <td>{{ doc.is_latest ? '最新版' : '历史版本' }}</td>
            <td><a class="text-link" :href="doc.download_url" target="_blank" rel="noopener">下载</a></td>
          </tr>
          <tr v-if="!documents.length"><td colspan="6" class="muted">暂无销售单版本</td></tr>
        </tbody>
      </table>
    </section>

    <div v-if="drawerOpen" class="drawer-mask" @click.self="closeCustomerDrawer">
      <aside class="drawer" aria-label="客户信息">
        <div class="drawer-head">
          <h3>客户信息</h3>
          <button class="secondary" type="button" @click="closeCustomerDrawer">关闭</button>
        </div>
        <form class="drawer-form" @submit.prevent="saveCustomer">
          <label>
            <span>客户名</span>
            <input v-model.trim="customerForm.name" required />
          </label>
          <label>
            <span>公司名称</span>
            <input v-model.trim="customerForm.company_name" name="company_name" placeholder="不填则默认客户名" />
          </label>
          <label>
            <span>联系电话</span>
            <input v-model.trim="customerForm.company_phone" name="company_phone" />
          </label>
          <label>
            <span>联系人</span>
            <input v-model.trim="customerForm.contact" />
          </label>
          <label>
            <span>收货电话</span>
            <input v-model.trim="customerForm.phone" />
          </label>
          <label>
            <span>公司地址</span>
            <textarea v-model.trim="customerForm.company_address" name="company_address" rows="3"></textarea>
          </label>
          <label>
            <span>收货地址</span>
            <textarea v-model.trim="customerForm.address" rows="3"></textarea>
          </label>
          <div class="drawer-actions">
            <button class="primary" type="submit" :disabled="savingCustomer">{{ savingCustomer ? '保存中' : '保存客户信息' }}</button>
          </div>
        </form>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { salesOrderDownloadUrl } from '../lib/sales-order'

const loading = ref(false)
const generating = ref(false)
const error = ref('')
const message = ref('')
const documents = ref([])
const drawerOpen = ref(false)
const savingCustomer = ref(false)
const customerSummary = reactive(emptyCustomer())
const customerForm = reactive(emptyCustomer())

const orderID = computed(() => Number(new URL(window.location.href).searchParams.get('order_id') || 0))

async function load() {
  if (!orderID.value) return
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet(`/api/orders/${orderID.value}/sales-orders`)
    documents.value = data.rows || []
    assignCustomer(customerSummary, data.order?.customer || {})
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function generate() {
  if (!orderID.value) return
  generating.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiSend(`/api/orders/${orderID.value}/sales-orders`)
    message.value = `已生成 V${data.version_no}`
    await load()
  } catch (err) {
    error.value = err.message || '生成失败'
  } finally {
    generating.value = false
  }
}

function emptyCustomer() {
  return {
    id: 0,
    name: '',
    raw_name: '',
    company_name: '',
    company_address: '',
    company_phone: '',
    contact: '',
    phone: '',
    address: '',
    default_source_id: null,
    default_order_type_id: null,
    active: true,
  }
}

function assignCustomer(target, data = {}) {
  Object.assign(target, {
    id: Number(data.id || 0),
    name: data.name || '',
    raw_name: data.raw_name || '',
    company_name: data.company_name || '',
    company_address: data.company_address || '',
    company_phone: data.company_phone || '',
    contact: data.contact || '',
    phone: data.phone || '',
    address: data.address || '',
    default_source_id: data.default_source_id || null,
    default_order_type_id: data.default_order_type_id || null,
    active: data.active !== false,
  })
}

async function openCustomerDrawer() {
  if (!customerSummary.id) return
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet(`/api/customers/${customerSummary.id}`)
    assignCustomer(customerForm, data.customer || {})
    drawerOpen.value = true
  } catch (err) {
    error.value = err.message || '加载客户信息失败'
  } finally {
    loading.value = false
  }
}

function closeCustomerDrawer() {
  drawerOpen.value = false
}

async function saveCustomer() {
  if (!customerForm.id) return
  savingCustomer.value = true
  error.value = ''
  message.value = ''
  try {
    const data = await apiSend(`/api/customers/${customerForm.id}`, {
      method: 'PUT',
      body: {
        name: customerForm.name,
        raw_name: customerForm.raw_name,
        company_name: customerForm.company_name,
        company_address: customerForm.company_address,
        company_phone: customerForm.company_phone,
        contact: customerForm.contact,
        phone: customerForm.phone,
        address: customerForm.address,
        default_source_id: customerForm.default_source_id,
        default_order_type_id: customerForm.default_order_type_id,
        active: customerForm.active,
      },
    })
    assignCustomer(customerForm, data.customer || {})
    assignCustomer(customerSummary, data.customer || {})
    message.value = '客户信息已保存'
  } catch (err) {
    error.value = err.message || '保存客户信息失败'
  } finally {
    savingCustomer.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; max-width: 1180px; }
.panel-head, .actions, .summary { display: flex; align-items: center; gap: 12px; }
.panel-head { justify-content: space-between; margin-bottom: 12px; }
.actions { flex-wrap: wrap; justify-content: flex-end; }
h2, h3 { margin: 0; }
h2 { font-size: 20px; }
h3 { font-size: 16px; }
button, .link-button { min-height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 8px 12px; font: inherit; cursor: pointer; text-decoration: none; display: inline-flex; align-items: center; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.summary { color: #555; }
.manual { border-top: 1px solid #edf0f5; padding-top: 10px; margin-top: 12px; color: #4b5563; font-size: 13px; }
.manual summary { cursor: pointer; font-weight: 700; color: #111827; }
.manual ul { margin: 8px 0 0; padding-left: 18px; }
table { width: 100%; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; }
th { background: #fbfaf8; }
.text-link { color: #1f4f82; text-decoration: none; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
.drawer-mask { position: fixed; inset: 0; z-index: 40; display: flex; justify-content: flex-end; background: rgba(0, 0, 0, .24); }
.drawer { width: min(520px, 100vw); height: 100%; overflow: auto; background: #fff; border-left: 1px solid #e6e0d8; padding: 16px; box-shadow: -10px 0 24px rgba(0, 0, 0, .12); }
.drawer-head, .drawer-actions { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 14px; }
.drawer-form { display: grid; gap: 12px; }
.drawer-form label span { display: block; color: #555; font-size: 12px; margin-bottom: 5px; }
.drawer-form input, .drawer-form textarea { width: 100%; border: 1px solid #cfc8bf; border-radius: 6px; padding: 8px 10px; font: inherit; background: #fff; }
.drawer-form input { height: 38px; }
.drawer-form textarea { min-height: 82px; resize: vertical; }
.drawer-actions { justify-content: flex-end; margin: 4px 0 0; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .panel-head { align-items: stretch; flex-direction: column; }
  .actions { justify-content: flex-start; }
  table { min-width: 760px; }
  .panel { overflow: auto; }
  .drawer { width: 100vw; }
}
</style>
