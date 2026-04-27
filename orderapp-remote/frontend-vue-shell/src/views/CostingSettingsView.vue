<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>成本参数设置</h2>
        <button class="secondary" type="button" :disabled="loading" @click="load">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存，成本试算会使用新参数</div>
    </section>

    <section class="panel">
      <div class="section-title">Excel 迁移参数</div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>参数</th>
              <th>键</th>
              <th>值</th>
              <th>单位</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.key">
              <td class="label">{{ row.label }}</td>
              <td class="muted">{{ row.key }}</td>
              <td><input type="number" min="0" step="0.000001" v-model.number="row.value" /></td>
              <td>{{ row.unit }}</td>
              <td class="muted">{{ row.updated_at || '-' }}</td>
              <td>
                <button class="secondary" type="button" :disabled="savingKey === row.key" @click="save(row)">保存</button>
              </td>
            </tr>
            <tr v-if="!loading && !rows.length">
              <td colspan="6" class="muted empty">暂无参数</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'

const rows = ref([])
const loading = ref(false)
const savingKey = ref('')
const error = ref('')
const ok = ref(false)

function normalize(row) {
  return {
    key: row.key || '',
    label: row.label || row.key || '',
    value: Number(row.value || 0),
    unit: row.unit || '',
    updated_at: row.updated_at || '',
  }
}

async function load() {
  loading.value = true
  error.value = ''
  ok.value = false
  try {
    const data = await apiGet('/api/costing/settings')
    rows.value = (data.rows || []).map(normalize)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function save(row) {
  savingKey.value = row.key
  error.value = ''
  ok.value = false
  try {
    const data = await apiSend(`/api/costing/settings/${encodeURIComponent(row.key)}`, {
      body: { value: Number(row.value || 0) },
    })
    const next = normalize(data)
    const idx = rows.value.findIndex((item) => item.key === row.key)
    if (idx >= 0) rows.value[idx] = next
    ok.value = true
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    savingKey.value = ''
  }
}

onMounted(load)
</script>

<style scoped>
.page { padding: 16px; display: grid; gap: 16px; color: #171717; }
.panel { border: 1px solid #eee; border-radius: 8px; padding: 12px; background: #fff; }
.panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.panel-head h2, .section-title { margin: 0; font-size: 18px; font-weight: 700; }
.table-wrap { overflow: auto; margin-top: 10px; }
table { width: 100%; border-collapse: collapse; min-width: 940px; }
th, td { border-bottom: 1px solid #f1f1f1; padding: 8px 10px; text-align: left; vertical-align: middle; }
th { color: #555; background: #fafafa; font-weight: 700; }
.label { font-weight: 650; }
.muted { color: #666; font-size: 12px; }
.empty { text-align: center; padding: 18px; }
input, button { font: inherit; }
input { width: 160px; max-width: 100%; border: 1px solid #ddd; border-radius: 6px; padding: 8px; min-height: 36px; }
button { border-radius: 8px; padding: 9px 12px; cursor: pointer; white-space: nowrap; }
button:disabled { opacity: .45; cursor: not-allowed; }
.secondary { border: 1px solid #999; background: #fff; color: #111; }
.error, .ok { border-radius: 8px; padding: 10px; margin-bottom: 12px; }
.error { background: #ffecec; border: 1px solid #ffb9b9; }
.ok { background: #e9ffe9; border: 1px solid #b8f5b8; }
</style>
