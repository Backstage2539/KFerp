<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client.js'
const props = defineProps({ versionId: { type: Number, required: true } })
const emit = defineEmits(['close', 'updated'])
const rows = ref([])
const selected = ref([])
const busy = ref(false)
const error = ref('')
const versionNo = ref('')
const resultNote = ref('')
const eligible = row => ['ready', 'failed'].includes(row.status)
const selectable = computed(() => rows.value.filter(eligible))
const allSelected = computed(() => selectable.value.length > 0 && selectable.value.every(row => selected.value.includes(row.bom_id)))
function toggleAll(event) { selected.value = event.target.checked ? selectable.value.map(row => row.bom_id) : [] }
const labels = { ready: '可下发', blocked: '需先处理', already_updated: '已更新', updated: '更新成功', failed: '失败，可重试' }
function valueText(value) { return value == null ? '无' : typeof value === 'object' ? JSON.stringify(value) : String(value) }
async function load() {
  if (busy.value) return
  busy.value = true; error.value = ''; resultNote.value = ''
  try {
    const data = await apiGet(`/api/production-bom-spec-template-versions/${props.versionId}/rollout-preview`)
    rows.value = data.rows || []; versionNo.value = data.template_version_no || ''
    selected.value = rows.value.filter(row => row.status === 'ready').map(row => row.bom_id)
  } catch (err) { error.value = err.message || '清单加载失败' }
  finally { busy.value = false }
}
async function apply() {
  if (busy.value || !selected.value.length) return
  busy.value = true; error.value = ''; resultNote.value = ''
  const targets = rows.value.filter(row => eligible(row) && selected.value.includes(row.bom_id)).map(row => ({ bom_id: row.bom_id, expected_version_id: row.current_version_id }))
  const completed = []
  try {
    for (let offset = 0; offset < targets.length; offset += 100) {
      const data = await apiSend(`/api/production-bom-spec-template-versions/${props.versionId}/rollout`, { body: { targets: targets.slice(offset, offset + 100) } })
      for (const result of data.rows || []) {
        const row = rows.value.find(row => row.bom_id === result.bom_id)
        if (row) Object.assign(row, Object.fromEntries(Object.entries(result).filter(([key, value]) => !['current_version_id', 'current_version_no', 'bom_name', 'product_name', 'source_template_version_no'].includes(key) || value)))
        completed.push(result)
      }
    }
    resultNote.value = `成功 ${completed.filter(row => row.status === 'updated').length} 项，已更新 ${completed.filter(row => row.status === 'already_updated').length} 项，需处理或失败 ${completed.filter(row => ['blocked', 'failed'].includes(row.status)).length} 项。`
  } catch (err) { error.value = err.message || '下发失败，请刷新清单核对后重试' }
  finally {
    selected.value = rows.value.filter(row => eligible(row) && targets.some(target => target.bom_id === row.bom_id)).map(row => row.bom_id)
    busy.value = false; emit('updated')
  }
}
onMounted(load)
</script>
<template>
  <div class="rollout-mask" @click.self="!busy && emit('close')">
    <section role="dialog" aria-modal="true" aria-label="下发规格模板更新" class="rollout-dialog">
      <header><div><h2>下发规格模板更新 {{ versionNo }}</h2><p>逐个更新并发布新 BOM 版本。保留局部修改，冲突项跳过；已发布报价和历史订单保持原样。</p></div><button type="button" :disabled="busy" @click="emit('close')">关闭</button></header>
      <p v-if="error" role="alert" class="error">{{ error }}</p><p v-if="resultNote" role="status">{{ resultNote }}</p>
      <div class="rollout-scroll"><table><thead><tr><th><input aria-label="全选可下发 BOM" type="checkbox" :checked="allSelected" :disabled="busy" @change="toggleAll" /></th><th>商品 / BOM</th><th>当前版本</th><th>待更新内容</th><th>状态及原因</th></tr></thead>
      <tbody><tr v-for="row in rows" :key="row.bom_id"><td><input v-model="selected" type="checkbox" :value="row.bom_id" :aria-label="`选择 ${row.bom_name}`" :disabled="busy || !eligible(row)" /></td><td>{{ row.product_name }}<br />{{ row.bom_name || `BOM #${row.bom_id}` }}</td><td>模板 {{ row.source_template_version_no || '来源不完整' }}<br />BOM {{ row.current_version_no }}<span v-if="row.new_version_no"> → {{ row.new_version_no }}</span></td><td><details v-if="row.changes?.length"><summary>{{ row.changes.length }} 项变化</summary><ul><li v-for="(change, index) in row.changes" :key="index">{{ change.spec_key }} · {{ change.field === '物流重量' ? '物流重量（g）' : change.field }}：{{ valueText(change.before) }} → {{ valueText(change.after) }}</li></ul></details><span v-else>{{ row.status === 'blocked' ? '处理阻塞后重新预览' : '无内容差异' }}</span></td><td>{{ labels[row.status] || row.status }}<p>{{ row.reason }}</p></td></tr></tbody></table><p v-if="!rows.length">{{ busy ? '正在加载影响清单…' : '没有引用该模板的已发布 BOM' }}</p></div>
      <footer><span>已选 {{ selected.length }} 项；可取消个别 BOM。删除规格和改变单位仍需通过库存及引用校验。</span><button type="button" :disabled="busy" @click="load">刷新清单</button><button type="button" class="primary" :disabled="busy || !selected.length" @click="apply">{{ busy ? '处理中…' : '下发并发布所选 BOM' }}</button></footer>
    </section>
  </div>
</template>
<style scoped>
.rollout-mask{position:fixed;inset:0;background:#0006;z-index:1300;display:grid;place-items:center;padding:20px}.rollout-dialog{background:#fff;border-radius:12px;padding:22px;width:min(1100px,96vw);max-height:90vh;display:flex;flex-direction:column;gap:14px;color:#242424}.rollout-dialog header,.rollout-dialog footer{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}.rollout-dialog h2{margin:0}.rollout-dialog p{margin:6px 0;color:#657080}.rollout-scroll{overflow:auto;min-height:120px}table{width:100%;border-collapse:collapse;text-align:left}th,td{padding:12px;border-bottom:1px solid #e3e6eb;vertical-align:top}td:nth-child(4){max-width:420px;overflow-wrap:anywhere}li{margin-bottom:8px}button{padding:9px 14px;border:1px solid #c6cfda;border-radius:7px;background:#fff;cursor:pointer}.primary{background:#215b9d;color:#fff}button:disabled{opacity:.5;cursor:default}.error{color:#b3261e!important}footer span{flex:1;min-width:240px;font-size:13px}
</style>
