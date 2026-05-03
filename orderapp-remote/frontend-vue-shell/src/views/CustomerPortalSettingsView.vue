<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>客户门户配置</h2>
        <button class="secondary" type="button" @click="loadCustomers" :disabled="loading">刷新</button>
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
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>客户</th><th>公司</th><th>手机号</th><th>门户名</th><th>绑定用户</th><th>状态</th><th>操作</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in customers" :key="row.id" :class="{ active: detail?.customer?.id === row.id }">
              <td>{{ row.name }}</td>
              <td>{{ row.company_name || '' }}</td>
              <td>{{ row.phone || '' }}</td>
              <td>{{ row.display_name || row.name }}</td>
              <td>{{ row.binding_count || 0 }}</td>
              <td>{{ row.portal_enabled ? '启用' : '停用' }}</td>
              <td><button class="text-button" type="button" @click="openDetail(row.id)">配置</button></td>
            </tr>
            <tr v-if="!customers.length">
              <td colspan="7" class="muted">暂无客户</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section v-if="detail" class="panel">
      <div class="panel-head">
        <h3>{{ detail.customer.name }} 可见内容</h3>
        <button class="primary" type="button" @click="saveVisibility" :disabled="saving">保存配置</button>
      </div>
      <form class="form-grid" @submit.prevent="saveVisibility">
        <label>
          <span>小程序显示名</span>
          <input v-model.trim="form.display_name" placeholder="默认使用客户名" />
        </label>
        <label class="check">
          <input v-model="form.enabled" type="checkbox" />
          <span>{{ form.enabled ? '门户启用' : '门户停用' }}</span>
        </label>
      </form>

      <div class="capabilities">
        <label v-for="capability in capabilities" :key="capability.code" class="capability">
          <input v-model="capability.enabled" type="checkbox" />
          <strong>{{ capability.label }}</strong>
          <span>{{ capability.description }}</span>
        </label>
      </div>

      <div class="subhead">已绑定小程序用户</div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>Mini User ID</th><th>手机号</th><th>昵称</th><th>角色</th><th>绑定状态</th><th>绑定时间</th></tr>
          </thead>
          <tbody>
            <tr v-for="binding in bindings" :key="binding.mini_user_id">
              <td>{{ binding.mini_user_id }}</td>
              <td>{{ binding.phone }}</td>
              <td>{{ binding.nickname }}</td>
              <td>{{ binding.role }}</td>
              <td>{{ binding.status }}</td>
              <td>{{ binding.created_at }}</td>
            </tr>
            <tr v-if="!bindings.length"><td colspan="6" class="muted">暂无绑定用户</td></tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const q = ref('')
const customers = ref([])
const detail = ref(null)
const capabilities = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref('')
const form = reactive({ display_name: '', enabled: true })
const capabilityLabels = {
  bean_list: '我的豆单',
  product_order: '现货下单',
  direct_ship: '一件代发',
  processing: '代加工',
  inventory_custody: '我的库存',
  shipping_query: '物流查询',
  settlement: '结算中心',
}

const bindings = computed(() => detail.value?.bindings || [])

async function loadCustomers() {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const url = new URL('/api/customer-portal/admin/customers', window.location.origin)
    if (q.value) url.searchParams.set('q', q.value)
    const data = await apiGet(url)
    customers.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载客户失败'
  } finally {
    loading.value = false
  }
}

async function openDetail(customerId) {
  loading.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiGet(`/api/customer-portal/admin/customers/${customerId}`)
    assignDetail(data)
  } catch (err) {
    error.value = err.message || '加载配置失败'
  } finally {
    loading.value = false
  }
}

function assignDetail(data) {
  detail.value = data
  form.display_name = data?.customer?.display_name || ''
  form.enabled = data?.customer?.portal_enabled !== false
  capabilities.value = (data?.capabilities || []).map((item) => ({
    code: item.code,
    label: item.label || capabilityLabels[item.code] || item.code,
    description: item.description || '',
    enabled: !!item.enabled,
    config: item.config || {},
  }))
}

async function saveVisibility() {
  if (!detail.value?.customer?.id) return
  saving.value = true
  error.value = ''
  ok.value = ''
  try {
    const data = await apiSend(`/api/customer-portal/admin/customers/${detail.value.customer.id}/visibility`, {
      method: 'PUT',
      body: {
        display_name: form.display_name,
        enabled: !!form.enabled,
        capabilities: capabilities.value.map((item) => ({
          code: item.code,
          enabled: !!item.enabled,
          config: item.config || {},
        })),
      },
    })
    assignDetail(data)
    ok.value = '已保存客户门户配置'
    await loadCustomers()
  } catch (err) {
    error.value = err.message || '保存配置失败'
  } finally {
    saving.value = false
  }
}

onMounted(loadCustomers)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e1e5ea; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .filters { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.filters { justify-content: flex-start; margin-top: 12px; }
h2, h3 { margin: 0; font-size: 20px; }
h3 { font-size: 18px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input { width: min(420px, 70vw); height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.text-button { height: 30px; border: 0; background: transparent; color: #1f4f82; padding: 0; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 860px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eef1f4; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #f8fafc; }
tr.active { background: #f3f7fb; }
.form-grid { display: grid; grid-template-columns: minmax(220px, 420px) minmax(160px, 1fr); gap: 12px; align-items: end; margin: 12px 0; }
.check { display: inline-flex; align-items: center; gap: 8px; align-self: center; }
.check input, .capability input { width: auto; height: auto; }
.check span { margin: 0; }
.capabilities { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 10px; margin: 12px 0 18px; }
.capability { min-height: 86px; border: 1px solid #e4e7ec; border-radius: 8px; padding: 10px; display: grid; grid-template-columns: auto 1fr; column-gap: 8px; row-gap: 4px; align-items: start; }
.capability strong { font-size: 15px; line-height: 1.3; }
.capability span { grid-column: 2; font-size: 12px; color: #666; line-height: 1.5; }
.subhead { font-size: 15px; font-weight: 700; margin: 8px 0; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff6; border: 1px solid #a9d8ba; color: #1f6a3f; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .form-grid { grid-template-columns: 1fr; }
  .capabilities { grid-template-columns: 1fr; }
  table { min-width: 760px; }
}
</style>
