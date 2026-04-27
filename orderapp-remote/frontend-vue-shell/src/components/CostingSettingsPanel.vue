<template>
  <div class="settings-panel" :class="{ compact }">
    <div v-if="showHeader" class="settings-head">
      <div>
        <h2>成本参数设置</h2>
        <p>按公式用途分类维护，保存后成本试算使用新参数。</p>
      </div>
      <button class="secondary" type="button" :disabled="loading" @click="load">刷新</button>
    </div>

    <div v-if="error" class="error">{{ error }}</div>
    <div v-if="ok" class="ok">已保存，成本试算会使用新参数</div>

    <div class="groups">
      <section v-for="group in groups" :key="group.key" class="group">
        <div class="group-head">
          <h3>{{ group.title }}</h3>
          <p>{{ group.summary }}</p>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>参数</th>
                <th>值</th>
                <th>单位</th>
                <th>更新时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in group.rows" :key="row.key">
                <td class="label-cell">
                  <div class="label">{{ row.label }}</div>
                  <div class="hint">{{ row.description }}</div>
                  <div class="key">{{ row.key }}</div>
                </td>
                <td><input type="number" min="0" step="0.000001" v-model.number="row.value" /></td>
                <td>{{ row.unit }}</td>
                <td class="muted">{{ row.updated_at || '-' }}</td>
                <td>
                  <button class="secondary" type="button" :disabled="savingKey === row.key" @click="save(row)">保存</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
      <div v-if="!loading && !groups.length" class="muted empty">暂无参数</div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { enrichCostingSetting, groupCostingSettings } from '../lib/costing-settings'

const props = defineProps({
  compact: { type: Boolean, default: false },
  showHeader: { type: Boolean, default: true },
})
const emit = defineEmits(['saved', 'loaded'])

const rows = ref([])
const loading = ref(false)
const savingKey = ref('')
const error = ref('')
const ok = ref(false)

const groups = computed(() => groupCostingSettings(rows.value))

function normalize(row) {
  return enrichCostingSetting(row || {})
}

async function load() {
  loading.value = true
  error.value = ''
  ok.value = false
  try {
    const data = await apiGet('/api/costing/settings')
    rows.value = (data.rows || []).map(normalize)
    emit('loaded', rows.value)
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
    emit('saved', next)
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    savingKey.value = ''
  }
}

onMounted(load)
defineExpose({ load })
</script>

<style scoped>
* { box-sizing: border-box; }
.settings-panel { color: #171717; display: grid; gap: 12px; }
.settings-head { border: 1px solid #eee; border-radius: 8px; padding: 12px; background: #fff; display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.settings-head h2 { margin: 0; font-size: 20px; font-weight: 700; }
.settings-head p, .group-head p { color: #666; font-size: 12px; line-height: 1.45; margin: 4px 0 0; }
.groups { display: grid; gap: 12px; }
.group { border: 1px solid #eee; border-radius: 8px; background: #fff; padding: 12px; }
.group-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 10px; }
.group-head h3 { margin: 0; font-size: 16px; font-weight: 700; }
.table-wrap { overflow: auto; }
table { width: 100%; border-collapse: collapse; min-width: 900px; }
th, td { border-bottom: 1px solid #f1f1f1; padding: 9px 10px; text-align: left; vertical-align: middle; }
th { color: #555; background: #fafafa; font-weight: 700; }
tr:last-child td { border-bottom: 0; }
.label-cell { min-width: 280px; }
.label { font-weight: 650; }
.hint { color: #666; font-size: 12px; line-height: 1.45; margin-top: 3px; white-space: normal; }
.key, .muted { color: #777; font-size: 12px; }
.key { margin-top: 3px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.empty { text-align: center; padding: 18px; border: 1px solid #eee; border-radius: 8px; background: #fff; }
input, button { font: inherit; }
input { width: 150px; max-width: 100%; border: 1px solid #ddd; border-radius: 6px; padding: 8px; min-height: 36px; }
button { border-radius: 8px; padding: 9px 12px; cursor: pointer; white-space: nowrap; }
button:disabled { opacity: .45; cursor: not-allowed; }
.secondary { border: 1px solid #999; background: #fff; color: #111; }
.error, .ok { border-radius: 8px; padding: 10px; }
.error { background: #ffecec; border: 1px solid #ffb9b9; }
.ok { background: #e9ffe9; border: 1px solid #b8f5b8; }

.compact { gap: 10px; }
.compact .group { padding: 10px; }
.compact .group-head { display: block; margin-bottom: 8px; }
.compact table { min-width: 0; }
.compact thead { display: none; }
.compact tr { display: grid; grid-template-columns: 128px 64px minmax(96px, 1fr); gap: 8px; border-bottom: 1px solid #f1f1f1; padding: 9px 0; }
.compact tr:last-child { border-bottom: 0; }
.compact td { border-bottom: 0; padding: 0; }
.compact td:nth-child(4) { display: none; }
.compact td:nth-child(2) { grid-column: 1 / 2; }
.compact td:nth-child(3) { grid-column: 2 / 3; align-self: center; }
.compact td:nth-child(5) { grid-column: 3 / 4; }
.compact .label-cell { min-width: 0; grid-column: 1 / 4; }
.compact input { width: 128px; }
.compact button { width: 100%; }

@media (max-width: 900px) {
  .settings-head { align-items: flex-start; flex-direction: column; }
  table { min-width: 760px; }
  .compact tr { grid-template-columns: 1fr; }
  .compact td, .compact td:nth-child(5), .compact .label-cell { grid-column: auto; }
  .compact td:nth-child(4) { display: block; }
  .compact input, .compact button { width: 100%; }
}
</style>
