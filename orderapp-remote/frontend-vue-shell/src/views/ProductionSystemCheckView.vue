<template>
  <div class="page">
    <section class="panel head"><div><h2>生产系统检查</h2><p>供管理员核对生产基础数据和服务状态，不参与现场完工入库。</p></div><button type="button" @click="load" :disabled="loading">刷新检查</button></section>
    <div v-if="error" class="error">{{ error }}</div>
    <section class="panel rows">
      <article v-for="row in rows" :key="row.code"><div><strong>{{ row.title }}</strong><span>{{ row.detail || row.code }}</span></div><span :class="['status', row.status]">{{ statusLabel(row.status) }} · {{ Number(row.count || 0).toLocaleString('zh-CN') }}</span><button type="button" :disabled="!row.view" @click="openView(row)">打开</button></article>
      <p v-if="!rows.length" class="empty">暂无检查结果</p>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiGet } from '../api/client'
const rows = ref([]); const loading = ref(false); const error = ref('')
function statusLabel(value) { return ({ ok: '已具备', error: '异常', todo: '待补' }[value] || '待补') }
function openView(row) { if (row?.view) window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: row.view, params: row.view_params || {} } })) }
async function load() { loading.value = true; error.value = ''; try { const data = await apiGet('/api/produce/acceptance-smoke'); rows.value = data.rows || [] } catch (err) { error.value = err.message || '检查失败' } finally { loading.value = false } }
onMounted(load)
</script>

<style scoped>
.page{padding:16px;display:grid;gap:14px;background:#f7f8fa}.panel{border:1px solid #e0e5e2;border-radius:10px;background:#fff;padding:14px}.head{display:flex;justify-content:space-between;gap:12px}.head h2{margin:0}.head p{margin:5px 0 0;color:#6b7280}.rows{display:grid;gap:8px}.rows article{display:grid;grid-template-columns:1fr auto auto;gap:12px;align-items:center;border-bottom:1px solid #edf0ee;padding:10px}.rows article div{display:grid;gap:4px}.rows article span{font-size:12px;color:#6b7280}.status{border:1px solid #ddd;border-radius:999px;padding:4px 8px}.status.ok{color:#24704a;border-color:#acd4ba}.status.error{color:#9e3328;border-color:#efb1aa}button{min-height:36px;border:1px solid #bec9c3;border-radius:8px;background:#fff;padding:7px 12px}.error{padding:10px;border:1px solid #efb1aa;background:#fff1ef}.empty{text-align:center;color:#6b7280}@media(max-width:620px){.head,.rows article{grid-template-columns:1fr;display:grid}}
</style>
