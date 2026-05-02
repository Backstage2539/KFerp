<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>操作日志</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label>
          <span>开始日期</span>
          <input v-model.trim="filters.from" placeholder="YYYY-MM-DD" />
        </label>
        <label>
          <span>结束日期</span>
          <input v-model.trim="filters.to" placeholder="YYYY-MM-DD" />
        </label>
        <label>
          <span>类型</span>
          <select v-model="filters.type">
            <option value="">全部</option>
            <optgroup label="订单销售">
              <option value="order">订单</option>
              <option value="customer">客户</option>
              <option value="customer_asset">客户附件</option>
              <option value="sales_order_settings">销售单设置</option>
              <option value="sales_order_asset">销售单素材</option>
              <option value="sales_order_payment_code">收款二维码</option>
              <option value="sales_order_document">销售单文件</option>
              <option value="sales_order_image">销售单图片</option>
            </optgroup>
            <optgroup label="库存管理">
              <option value="material">物料</option>
              <option value="material_receipt">原料入库单</option>
              <option value="material_transfer">原料转仓单</option>
              <option value="finished_product_transfer">成品转仓单</option>
              <option value="finished_inventory">成品库存</option>
            </optgroup>
            <optgroup label="商品与配方">
              <option value="product">产品</option>
              <option value="product_category">产品分类</option>
              <option value="bean_list_publication">豆单发布</option>
            </optgroup>
            <optgroup label="生产管理">
              <option value="produce_batch">生产批次</option>
              <option value="produce_running">生产任务</option>
              <option value="wip_reservation">WIP占用</option>
              <option value="costing_run">成本试算</option>
            </optgroup>
            <optgroup label="设置/系统">
              <option value="company_profile">公司信息</option>
              <option value="cost_parameter">成本参数</option>
              <option value="auth">登录</option>
              <option value="auth_account">员工账号</option>
              <option value="operation">操作</option>
              <option value="import">导入</option>
              <option value="system">系统</option>
            </optgroup>
          </select>
        </label>
        <label>
          <span>搜索</span>
          <input v-model.trim="filters.q" placeholder="操作者/字段/内容" @keyup.enter="load" />
        </label>
        <button class="primary" type="button" @click="load" :disabled="loading">筛选</button>
      </div>
    </section>

    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>时间</th>
              <th>操作者</th>
              <th>菜单</th>
              <th>功能</th>
              <th>摘要</th>
              <th>对象</th>
              <th>动作</th>
              <th>字段</th>
              <th>旧值</th>
              <th>新值</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="`${row.ts}-${row.actor}-${row.summary}`">
              <td>{{ row.ts }}</td>
              <td>{{ row.actor }}</td>
              <td>{{ row.menu }}</td>
              <td>{{ row.feature }}</td>
              <td class="summary">{{ row.summary }}</td>
              <td>
                <a v-if="row.entity_url" :href="row.entity_url">{{ row.entity_label || row.entity_type }}</a>
                <span v-else>{{ row.entity_label || row.entity_type }}</span>
              </td>
              <td>{{ row.action }}</td>
              <td>{{ row.field || '' }}</td>
              <td class="value">{{ row.old_value || '' }}</td>
              <td class="value">{{ row.new_value || '' }}</td>
            </tr>
            <tr v-if="!rows.length">
              <td colspan="10" class="muted">暂无日志</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { apiGet } from '../api/client'
import { replaceHistoryURL } from '../lib/url-state'

const loading = ref(false)
const error = ref('')
const rows = ref([])
const filters = reactive({
  from: '',
  to: '',
  type: '',
  q: '',
})

function applyUrlFilters() {
  const params = new URL(window.location.href).searchParams
  filters.from = params.get('from') || ''
  filters.to = params.get('to') || ''
  filters.type = params.get('type') || ''
  filters.q = params.get('q') || ''
}

function updateUrl() {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'audit')
  for (const key of ['from', 'to', 'type', 'q']) {
    if (filters[key]) url.searchParams.set(key, filters[key])
    else url.searchParams.delete(key)
  }
  replaceHistoryURL(url)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const url = new URL('/api/audit', window.location.origin)
    for (const key of ['from', 'to', 'type', 'q']) {
      if (filters[key]) url.searchParams.set(key, filters[key])
    }
    const data = await apiGet(url)
    rows.value = data.rows || []
    updateUrl()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  applyUrlFilters()
  load()
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
.filters { display: grid; grid-template-columns: repeat(4, minmax(140px, 1fr)) 90px; gap: 10px; align-items: end; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 1280px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
a { color: #1f4f82; text-decoration: none; }
.summary { min-width: 240px; }
.value { max-width: 220px; white-space: pre-wrap; }
.muted { color: #666; text-align: center; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; border-radius: 6px; padding: 9px; margin-bottom: 12px; color: #8a1f1f; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .filters { grid-template-columns: 1fr; }
  table { min-width: 1100px; }
}
</style>
