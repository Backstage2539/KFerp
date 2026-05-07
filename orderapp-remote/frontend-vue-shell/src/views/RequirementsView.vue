<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>{{ config.title }}</h2>
          <p>{{ total }} 条记录</p>
        </div>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存</div>
      <div class="tabs">
        <button
          v-for="item in tabs"
          :key="item.key"
          type="button"
          :class="{ active: item.key === viewKey }"
          @click="switchTab(item.key)">
          {{ item.label }}
        </button>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">新增记录</div>
      <div class="form-grid" :class="{ review: config.type === 'review' }">
        <label>
          <span>编号</span>
          <input v-model.trim="form.code" :placeholder="config.codeHint" />
        </label>
        <label v-if="config.type === 'review'">
          <span>产品需求编号</span>
          <input v-model.trim="form.pr_code" placeholder="PR-001" />
        </label>
        <label class="title-field">
          <span>标题</span>
          <input v-model.trim="form.title" placeholder="一句话描述" />
        </label>
        <label>
          <span>状态</span>
          <select v-model="form.status">
            <option value="todo">todo</option>
            <option value="doing">doing</option>
            <option value="review">review</option>
            <option value="done">done</option>
          </select>
        </label>
        <label>
          <span>负责人</span>
          <input v-model.trim="form.assignee" placeholder="Van/Codex" />
        </label>
        <button class="primary" type="button" @click="createRow" :disabled="saving">新增</button>
      </div>
    </section>

    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>编号</th>
              <th v-if="config.type === 'review'">PR</th>
              <th>标题</th>
              <th>状态</th>
              <th>负责人</th>
              <th>证据</th>
              <th>创建时间</th>
              <th v-if="config.type === 'review'">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>{{ row.code }}</td>
              <td v-if="config.type === 'review'">{{ row.pr_code }}</td>
              <td class="title-cell">{{ row.title }}</td>
              <td><span class="pill" :class="row.status">{{ row.status }}</span></td>
              <td>{{ row.assignee }}</td>
              <td class="evidence">{{ row.evidence }}</td>
              <td>{{ shortTime(row.created_at) }}</td>
              <td v-if="config.type === 'review'">
                <select v-model="statusDraft[row.code]">
                  <option value="todo">todo</option>
                  <option value="doing">doing</option>
                  <option value="done">done</option>
                </select>
                <button class="secondary small" type="button" @click="updateReview(row)" :disabled="saving">保存</button>
              </td>
            </tr>
            <tr v-if="!rows.length">
              <td :colspan="config.type === 'review' ? 8 : 6" class="muted">暂无记录</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="pager">
        <button class="secondary" type="button" @click="loadPage(page - 1)" :disabled="!hasPrev || loading">上一页</button>
        <span>第 {{ page }} / {{ totalPages }} 页</span>
        <button class="secondary" type="button" @click="loadPage(page + 1)" :disabled="!hasNext || loading">下一页</button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { replaceHistoryURL } from '../lib/url-state'

const props = defineProps({
  viewKey: { type: String, default: 'reqProduct' },
})

const tabs = [
  { key: 'reqProduct', label: '产品需求', type: 'product', title: '产品需求表', codeHint: 'PR-001' },
  { key: 'reqDev', label: '开发需求', type: 'dev', title: '开发需求表', codeHint: 'DEV-001' },
  { key: 'reqUnit', label: '单元测试', type: 'unit', title: '单元测试表', codeHint: 'UT-001' },
  { key: 'reqApi', label: 'API 测试', type: 'api', title: 'API 测试表', codeHint: 'API-001' },
  { key: 'reqReview', label: '需求审核', type: 'review', title: '需求审核表', codeHint: 'REV-001' },
]

const rows = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const page = ref(1)
const total = ref(0)
const totalPages = ref(1)
const hasPrev = ref(false)
const hasNext = ref(false)
const statusDraft = reactive({})
const form = reactive({ code: '', pr_code: '', title: '', status: 'todo', assignee: '' })

const config = computed(() => tabs.find((item) => item.key === props.viewKey) || tabs[0])

function resetForm() {
  form.code = ''
  form.pr_code = ''
  form.title = ''
  form.status = 'todo'
  form.assignee = ''
}

function shortTime(value) {
  if (!value) return ''
  return String(value).replace('T', ' ').replace(/\.\d+Z$/, '')
}

function updateUrl() {
  const url = new URL(window.location.href)
  url.searchParams.set('view', props.viewKey)
  url.searchParams.set('page', String(page.value))
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
    const url = new URL(`/api/req/${config.value.type}`, window.location.origin)
    url.searchParams.set('page', String(page.value))
    url.searchParams.set('limit', '10')
    const data = await apiGet(`${url.pathname}${url.search}`)
    rows.value = data.rows || []
    total.value = Number(data.total || 0)
    totalPages.value = Number(data.total_pages || 1)
    hasPrev.value = !!data.has_prev
    hasNext.value = !!data.has_next
    page.value = Number(data.page || page.value)
    for (const row of rows.value) {
      statusDraft[row.code] = row.status || 'todo'
    }
    updateUrl()
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function createRow() {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    await apiSend(`/api/req/${config.value.type}`, { body: form })
    ok.value = true
    resetForm()
    await loadPage(1)
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

async function updateReview(row) {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    await apiSend('/api/req/review/status', { body: { code: row.code, status: statusDraft[row.code] || row.status } })
    ok.value = true
    await load()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

function switchTab(key) {
  const url = new URL(window.location.href)
  url.searchParams.set('view', key)
  url.searchParams.delete('page')
  replaceHistoryURL(url)
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key } }))
}

watch(() => props.viewKey, () => {
  page.value = 1
  resetForm()
  load()
})

onMounted(() => {
  const params = new URL(window.location.href).searchParams
  page.value = Math.max(1, Number(params.get('page') || 1))
  load()
})
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
h2 { margin: 0 0 4px; font-size: 20px; }
p { margin: 0; color: #666; }
.tabs { display: flex; gap: 8px; flex-wrap: wrap; }
.tabs button.active { background: #1f1f1f; color: #fff; }
.section-title { font-weight: 700; margin-bottom: 10px; }
.form-grid { display: grid; grid-template-columns: 130px 1fr 120px 130px 84px; gap: 10px; align-items: end; }
.form-grid.review { grid-template-columns: 130px 130px 1fr 120px 130px 84px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input, select { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.small { height: 32px; margin-left: 6px; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 1120px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; vertical-align: top; }
th { background: #fbfaf8; position: sticky; top: 0; }
.title-cell { min-width: 260px; }
.evidence { max-width: 280px; white-space: pre-wrap; color: #555; }
.pill { display: inline-flex; align-items: center; height: 24px; border: 1px solid #ddd; border-radius: 6px; padding: 0 8px; background: #fafafa; }
.pill.done { border-color: #9fc7a3; background: #f2fff3; color: #27602d; }
.pill.doing, .pill.review { border-color: #d9c48a; background: #fff9df; color: #6a5313; }
.muted { color: #666; text-align: center; }
.pager { display: flex; gap: 10px; align-items: center; justify-content: flex-end; margin-top: 12px; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .form-grid, .form-grid.review { grid-template-columns: 1fr; }
  table { min-width: 960px; }
}
</style>
