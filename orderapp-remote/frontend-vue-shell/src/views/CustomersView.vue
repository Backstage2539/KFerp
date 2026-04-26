<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>客户档案</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="filters">
        <label>
          <span>搜索</span>
          <input v-model.trim="q" placeholder="客户/联系人/电话/地址" @keyup.enter="loadPage(1)" />
        </label>
        <button class="primary" type="button" @click="loadPage(1)" :disabled="loading">查询</button>
        <a class="button-link" href="/customers/new?legacy=1">新增客户</a>
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
            <tr v-for="row in rows" :key="row.id">
              <td>{{ row.name }}</td>
              <td>{{ row.contact || '' }}</td>
              <td>{{ row.phone || '' }}</td>
              <td class="address">{{ row.address || '' }}</td>
              <td>{{ optionName(sources, row.default_source_id) }}</td>
              <td>{{ optionName(orderTypes, row.default_order_type_id) }}</td>
              <td>{{ row.active ? '启用' : '停用' }}</td>
              <td>{{ row.updated }}</td>
              <td><a :href="`/customers/${row.id}?legacy=1`">编辑</a></td>
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
import { onMounted, ref } from 'vue'

const rows = ref([])
const sources = ref([])
const orderTypes = ref([])
const q = ref('')
const page = ref(1)
const hasPrev = ref(false)
const hasNext = ref(false)
const loading = ref(false)
const error = ref('')

function optionName(options, id) {
  const item = options.find((x) => Number(x.id) === Number(id))
  return item?.name || ''
}

function applyUrl() {
  const params = new URL(window.location.href).searchParams
  q.value = params.get('q') || ''
  page.value = Math.max(1, Number(params.get('page') || 1))
}

function updateUrl() {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'customers')
  if (q.value) url.searchParams.set('q', q.value)
  else url.searchParams.delete('q')
  url.searchParams.set('page', String(page.value))
  window.history.replaceState({}, '', url.toString())
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
    const res = await fetch(url)
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载失败')
    rows.value = data.rows || []
    sources.value = data.sources || []
    orderTypes.value = data.order_types || []
    hasPrev.value = !!data.has_prev
    hasNext.value = !!data.has_next
    page.value = Number(data.page || page.value)
    updateUrl()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  applyUrl()
  load()
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head, .filters, .pager { display: flex; align-items: center; gap: 10px; }
.panel-head { justify-content: space-between; margin-bottom: 12px; }
h2 { margin: 0; font-size: 20px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input { width: min(420px, 70vw); height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; }
button, .button-link { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; display: inline-flex; align-items: center; text-decoration: none; }
.primary { background: #1f1f1f; color: #fff; }
.secondary, .button-link { background: #fff; color: #1f1f1f; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 1080px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
a { color: #1f4f82; text-decoration: none; }
.address { max-width: 300px; white-space: pre-wrap; }
.muted { color: #666; text-align: center; }
.pager { justify-content: flex-end; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; border-radius: 6px; padding: 9px; margin-bottom: 12px; color: #8a1f1f; }
@media (max-width: 900px) { .page { padding: 12px; } .filters { align-items: end; flex-wrap: wrap; } table { min-width: 940px; } }
</style>
