<template>
  <div class="manual-page">
    <section class="manual-head">
      <p class="eyebrow">操作手册</p>
      <h2>{{ manualTitle }}</h2>
      <div class="doc-name">{{ docName || '未配置手册文件' }}</div>
    </section>

    <div v-if="loading" class="status">加载中</div>
    <div v-else-if="error" class="status error">{{ error }}</div>
    <section v-else class="manual-body">
      <template v-for="(block, index) in blocks" :key="index">
        <h1 v-if="block.type === 'h1'">{{ block.text }}</h1>
        <h2 v-else-if="block.type === 'h2'">{{ block.text }}</h2>
        <h3 v-else-if="block.type === 'h3'">{{ block.text }}</h3>
        <blockquote v-else-if="block.type === 'quote'">{{ block.text }}</blockquote>
        <ul v-else-if="block.type === 'ul'">
          <li v-for="item in block.items" :key="item">{{ item }}</li>
        </ul>
        <ol v-else-if="block.type === 'ol'">
          <li v-for="item in block.items" :key="item">{{ item }}</li>
        </ol>
        <div v-else-if="block.type === 'table'" class="table-wrap">
          <table>
            <thead>
              <tr>
                <th v-for="header in block.headers" :key="header">{{ header }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, rowIndex) in block.rows" :key="rowIndex">
                <td v-for="(cell, cellIndex) in row" :key="cellIndex">{{ cell }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else>{{ block.text }}</p>
      </template>
    </section>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { apiFetch } from '../api/client.js'
import { manualDocNameForView, manualTitleForView, parseManualMarkdown } from '../lib/operation-manuals.js'

const props = defineProps({
  title: { type: String, default: '' },
  viewKey: { type: String, default: '' },
})

const raw = ref('')
const loading = ref(false)
const error = ref('')

const docName = computed(() => manualDocNameForView(props.viewKey))
const manualTitle = computed(() => manualTitleForView(props.viewKey, props.title || '操作手册'))
const blocks = computed(() => parseManualMarkdown(raw.value))

watch(docName, loadManual, { immediate: true })

async function loadManual(name) {
  raw.value = ''
  error.value = ''
  if (!name) {
    error.value = '当前菜单未配置操作手册。'
    return
  }
  loading.value = true
  try {
    const res = await apiFetch(`/docs/${name}?raw=1`)
    if (!res.ok) throw new Error(`手册读取失败：HTTP ${res.status}`)
    raw.value = await res.text()
  } catch (err) {
    error.value = err.message || '手册读取失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
* { box-sizing: border-box; }
.manual-page { min-height: 100%; padding: 18px; color: #171717; background: #f7f7f5; }
.manual-head { border: 1px solid #d8dee4; border-radius: 8px; background: #fff; padding: 16px; margin-bottom: 14px; }
.eyebrow { margin: 0 0 8px; color: #64748b; font-size: 13px; font-weight: 800; }
h1, h2, h3 { letter-spacing: 0; }
.manual-head h2 { margin: 0; font-size: 22px; }
.doc-name { margin-top: 8px; color: #64748b; font-size: 13px; }
.status { border: 1px solid #d8dee4; border-radius: 8px; background: #fff; padding: 16px; color: #64748b; }
.status.error { color: #9f1239; border-color: #fecdd3; background: #fff1f2; }
.manual-body { border: 1px solid #d8dee4; border-radius: 8px; background: #fff; padding: 18px; }
.manual-body h1 { margin: 0 0 16px; font-size: 24px; }
.manual-body h2 { margin: 24px 0 10px; font-size: 18px; }
.manual-body h3 { margin: 18px 0 8px; font-size: 16px; }
.manual-body p, .manual-body li, blockquote { line-height: 1.75; }
.manual-body p { margin: 8px 0; color: #334155; }
blockquote { margin: 12px 0; border-left: 4px solid #1f2937; background: #f8fafc; padding: 10px 12px; color: #334155; }
ul, ol { margin: 8px 0 12px; padding-left: 24px; color: #334155; }
.table-wrap { overflow-x: auto; margin: 10px 0 14px; }
table { width: 100%; border-collapse: collapse; min-width: 560px; }
th, td { border: 1px solid #e2e8f0; padding: 9px 10px; text-align: left; vertical-align: top; font-size: 14px; line-height: 1.6; }
th { background: #f8fafc; color: #1f2937; }
@media (max-width: 900px) {
  .manual-page { padding: 12px; }
  .manual-body { padding: 14px; }
}
</style>
