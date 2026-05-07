<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>生产验收</h2>
          <p>部署后先看这张检查表，再按生产流程跑一条真实工单。</p>
        </div>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="summary">
        <div><span>检查项</span><strong>{{ rows.length }}</strong></div>
        <div><span>已具备</span><strong>{{ okCount }}</strong></div>
        <div><span>待补数据</span><strong>{{ todoCount }}</strong></div>
      </div>
    </section>

    <section class="panel table-wrap">
      <table>
        <thead>
          <tr>
            <th>检查项</th>
            <th>状态</th>
            <th>数量</th>
            <th>说明</th>
            <th>入口</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.code">
            <td><strong>{{ row.title }}</strong><small>{{ row.code }}</small></td>
            <td><span class="status" :class="row.status">{{ statusLabel(row.status) }}</span></td>
            <td>{{ Number(row.count || 0).toLocaleString('zh-CN') }}</td>
            <td>{{ row.detail || '-' }}</td>
            <td><button class="link" type="button" @click="openView(row)" :disabled="!row.view">打开</button></td>
          </tr>
          <tr v-if="!rows.length"><td colspan="5" class="muted">暂无检查结果</td></tr>
        </tbody>
      </table>
    </section>

    <section class="panel">
      <div class="section-title">验收主线</div>
      <div class="steps">
        <div v-for="(step, index) in steps" :key="step" class="step">
          <span>{{ index + 1 }}</span>
          <strong>{{ step }}</strong>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet } from '../api/client'

const rows = ref([])
const loading = ref(false)
const error = ref('')

const okCount = computed(() => rows.value.filter((row) => row.status === 'ok').length)
const todoCount = computed(() => rows.value.filter((row) => row.status !== 'ok').length)
const steps = [
  '原料入库并确认原料仓库存',
  '把要生产的原料领到 WIP',
  '在生产计划判断库存充足或库存不足',
  '库存充足订单标记无需生产并直接发货',
  '开始生产并确认工单生成 WIP占用',
  '生产中部分完工或最终完工',
  '记录生产质检',
  '在仓库库存用 FP 成品批次追溯',
]

function statusLabel(status) {
  if (status === 'ok') return '已具备'
  if (status === 'error') return '异常'
  return '待补'
}

function openView(row) {
  if (!row?.view) return
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: {
      key: row.view,
      params: row.view_params || {},
    },
  }))
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet('/api/produce/acceptance-smoke')
    rows.value = data.rows || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.page{padding:16px;display:grid;gap:16px}.panel{border:1px solid #e5e7eb;border-radius:8px;background:#fff;padding:12px}.panel-head{display:flex;justify-content:space-between;align-items:flex-start;gap:12px;margin-bottom:12px}.panel-head h2{margin:0 0 4px;font-size:18px}.panel-head p{margin:0;color:#6b7280;font-size:13px}.secondary{border:1px solid #9ca3af;background:#fff;color:#111;padding:8px 12px;border-radius:6px;min-height:36px;cursor:pointer}.summary{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px}.summary div{border:1px solid #e5e7eb;border-radius:8px;padding:10px}.summary span{display:block;color:#6b7280;font-size:12px;margin-bottom:4px}.summary strong{font-size:20px}.table-wrap{overflow:auto}table{width:100%;min-width:760px;border-collapse:collapse}th,td{border-bottom:1px solid #f0f0f0;padding:8px;text-align:left;font-size:13px;vertical-align:top}th{background:#fbfbfb}td small{display:block;color:#6b7280;margin-top:3px}.status{display:inline-flex;border:1px solid #d1d5db;border-radius:999px;padding:2px 8px;background:#f9fafb}.status.ok{border-color:#9bc6a6;background:#f1f9f3}.status.error{border-color:#ef9a9a;background:#fff1f1}.link{border:0;background:transparent;color:#111;text-decoration:underline;padding:0;cursor:pointer}.muted{color:#666;text-align:center}.error{background:#ffecec;border:1px solid #ffb9b9;border-radius:8px;padding:10px}.section-title{font-weight:700;margin-bottom:10px}.steps{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:10px}.step{border:1px solid #e5e7eb;border-radius:8px;padding:10px;display:grid;gap:6px}.step span{width:24px;height:24px;border-radius:50%;display:inline-grid;place-items:center;background:#111;color:#fff;font-size:12px}.step strong{font-size:13px}@media(max-width:800px){.page{padding:12px}.summary{grid-template-columns:1fr}}
</style>
