<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>销售单</h2>
        <div class="actions">
          <a class="secondary link-button" href="/vue-shell?view=orders">返回订单列表</a>
          <a v-if="documents.length" class="secondary link-button" :href="salesOrderDownloadUrl(orderID)" target="_blank" rel="noopener">下载最新版</a>
          <button class="primary" type="button" @click="generate" :disabled="generating || !orderID">{{ generating ? '生成中' : '生成销售单 PDF' }}</button>
        </div>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="message" class="ok">{{ message }}</div>
      <div class="summary">
        <span>订单 ID：{{ orderID || '-' }}</span>
        <span>版本数：{{ documents.length }}</span>
      </div>
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
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { salesOrderDownloadUrl } from '../lib/sales-order'

const loading = ref(false)
const generating = ref(false)
const error = ref('')
const message = ref('')
const documents = ref([])

const orderID = computed(() => Number(new URL(window.location.href).searchParams.get('order_id') || 0))

async function load() {
  if (!orderID.value) return
  loading.value = true
  error.value = ''
  try {
    const res = await fetch(`/api/orders/${orderID.value}/sales-orders`)
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '加载失败')
    documents.value = data.rows || []
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
    const res = await fetch(`/api/orders/${orderID.value}/sales-orders`, { method: 'POST' })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '生成失败')
    message.value = `已生成 V${data.version_no}`
    await load()
  } catch (err) {
    error.value = err.message || '生成失败'
  } finally {
    generating.value = false
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
table { width: 100%; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; }
th { background: #fbfaf8; }
.text-link { color: #1f4f82; text-decoration: none; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-bottom: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) {
  .page { padding: 12px; }
  .panel-head { align-items: stretch; flex-direction: column; }
  .actions { justify-content: flex-start; }
  table { min-width: 760px; }
  .panel { overflow: auto; }
}
</style>
