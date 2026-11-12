<template>
  <section class="batch-preview" aria-label="开工前批次预览">
    <h4>开工前批次预览</h4>
    <p v-if="loading">正在读取批次安排…</p>
    <p v-else-if="error" role="alert">{{ error }}</p>
    <template v-else>
      <p v-if="totalOutput !== null">本次安排预计产出 {{ format(totalOutput) }} {{ outputUnit }}</p>
      <p v-if="!rows.length">{{ data.reason || '暂无可信的批次安排，请核对原计划。' }}</p>
      <div v-if="rows.length" class="batch-table">
        <div class="batch-row batch-head"><span>批次</span><span>计划投入</span><span>预计产出</span><span>计划工时</span><span>状态</span></div>
        <div v-for="row in rows" :key="row.batch_no" class="batch-row">
          <strong>第 {{ row.batch_no }} 批</strong>
          <span>投入 {{ format(row.planned_input_qty) }} {{ row.input_unit }}</span>
          <span>产出 {{ row.planned_output_qty == null ? '未记录' : `${format(row.planned_output_qty)} ${row.output_unit}` }}</span>
          <span>工时 {{ row.planned_minutes == null ? '未记录' : `${row.planned_minutes} 分钟` }}</span>
          <span>{{ row.status === 'completed' ? '已完成' : '待开工' }}</span>
          <small v-if="row.default_unavailable_reason">{{ row.default_unavailable_reason }}</small>
        </div>
      </div>
      <p class="hint">以上为原计划预览，确认开工后按批次记录实际数据。</p>
    </template>
  </section>
</template>

<script setup>
import {computed, ref, watch} from 'vue'
import {apiGet} from '../api/client'
const props = defineProps({jobCardId: {type: Number, required: true}})
const data = ref({}), loading = ref(false), error = ref('')
let request = 0
watch(() => props.jobCardId, async id => {
  const current = ++request
  loading.value = true; error.value = ''; data.value = {}
  try { const result = await apiGet(`/api/job-cards/${id}/execution-batches`); if (current === request) data.value = result || {} }
  catch (e) { if (current === request) error.value = e.message || '无法读取批次安排，请重试。' }
  finally { if (current === request) loading.value = false }
}, {immediate: true})
const rows = computed(() => data.value.batches?.length ? data.value.batches : (data.value.preview_batches || []))
const outputUnit = computed(() => rows.value[0]?.output_unit || '')
const totalOutput = computed(() => rows.value.length && rows.value.every(row => row.planned_output_qty != null && row.output_unit === outputUnit.value) ? rows.value.reduce((sum, row) => sum + Number(row.planned_output_qty), 0) : null)
function format(value) { return Number(value).toLocaleString('zh-CN', {maximumFractionDigits: 6}) }
</script>

<style scoped>
.batch-preview{grid-column:1/-1;padding:16px;margin:12px 0;background:#f4f9f6;border:1px solid #d6e8dc;border-radius:10px}.batch-preview h4{margin:0 0 10px}.batch-preview p{color:#536e62}.batch-table{display:grid;gap:5px}.batch-row{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:8px;padding:9px;background:white;border:1px solid #e1e9e4;border-radius:6px}.batch-head{background:transparent;border:0;color:#637b70}.batch-row small{grid-column:1/-1;color:#986100}.hint{font-size:13px}@media (max-width: 760px){.batch-head{display:none}.batch-row{grid-template-columns:repeat(2,minmax(0,1fr))}.batch-row strong,.batch-row small{grid-column:1/-1}}
</style>
