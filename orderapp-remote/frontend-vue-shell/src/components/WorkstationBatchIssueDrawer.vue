<template>
  <div class="batch-issue-backdrop">
    <section class="batch-issue-drawer" role="dialog" aria-modal="true" aria-labelledby="batch-issue-title">
      <header>
        <div><h2 id="batch-issue-title">合并领料</h2><p>{{ workstation }} · 已选 {{ taskIds.length }} 个任务</p></div>
        <button type="button" :disabled="saving" @click="$emit('close')">关闭</button>
      </header>
      <p class="intro">核对实际数量后，一次领到 WIP 并过账。多领部分留在 WIP，生产消耗另行记录。</p>
      <p v-if="loading" role="status">正在汇总物料与可领库存…</p>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <section v-if="result" class="result" role="status">
        <strong>已领 {{ result.success_count }} 组 · 未完成 {{ result.failed_count }} 组</strong>
        <div v-for="row in result.groups" :key="row.key" class="result-row">
          <strong>{{ row.name || '所选物料' }}</strong>
          <span v-if="row.status === 'submitted'">已领 {{ format(row, row.quantity) }}<template v-if="row.extra">，其中额外 WIP {{ format(row, row.extra) }}</template>；仍待领 {{ format(row, row.remaining) }}</span>
          <span v-else class="error">{{ row.error }}</span>
          <button v-for="doc in row.documents" :key="doc.id" type="button" class="document-link" @click="openDocument(doc)">{{ doc.entry_no }}</button>
        </div>
      </section>
      <template v-if="preview && !loading">
        <div class="selection-bar">
          <label><input type="checkbox" aria-label="全选可领物料" :checked="allGroupsSelected" :indeterminate="someGroupsSelected" :disabled="locked || !eligibleGroups.length" @change="toggleGroups" /> 全选可领物料</label>
          <span>已选 {{ selected.length }} 组</span>
          <button type="button" :disabled="saving || uncertain" @click="refresh">刷新领料建议</button>
        </div>
        <p v-if="!preview.groups.length && !preview.blocked.length" class="empty">所选任务当前没有待领物料。</p>
        <article v-for="group in preview.groups" :key="group.key" class="material-group" :class="{ unavailable: !group.available }">
          <div class="group-title">
            <label><input v-model="selected" type="checkbox" :value="group.key" :disabled="locked || !group.available" :aria-label="`选择 ${group.name}`" /><strong>{{ group.name }}</strong></label>
            <small>{{ group.owner_name }} · {{ warehouse(group.from_warehouse) }} → WIP<template v-if="group.bom_spec_id"> · 规格 #{{ group.bom_spec_id }}</template></small>
          </div>
          <dl class="quantities">
            <div><dt>需求</dt><dd>{{ format(group, group.required) }}</dd></div>
            <div><dt>已领 / WIP 已到位</dt><dd>{{ format(group, group.wip) }}</dd></div>
            <div><dt>待领</dt><dd>{{ format(group, group.remaining) }}</dd></div>
            <div><dt>本次可领</dt><dd>{{ format(group, group.available) }}</dd></div>
          </dl>
          <div class="actual-row">
            <label>实际领用量 <input v-model.number="quantities[group.key]" type="number" min="0" :step="group.quantity_basis === 'weight' ? 1 / batchIssueFactor(group) : 1" :disabled="locked || !group.available || !selected.includes(group.key)" :aria-label="`${group.name} 实际领用量`" /> {{ group.unit }}</label>
            <small>含额外 WIP 最多可领 {{ format(group, group.max_quantity) }}</small>
          </div>
          <p v-if="selected.includes(group.key) && batchIssueValidation(group, quantities[group.key])" class="error">{{ batchIssueValidation(group, quantities[group.key]) }}</p>
          <p v-if="group.reason" class="warning">{{ group.reason }}；未领部分继续保留待办。</p>
          <details><summary>查看 {{ group.allocations.length }} 条工单及批次分配</summary>
            <div v-for="(row, index) in batchIssueAllocations(group, quantities[group.key])" :key="`${row.job_card_id}:${row.binding_id}:${index}`" class="allocation">
              <span>{{ row.work_order_no }} · 任务 #{{ row.job_card_id }}</span><span>批次 {{ row.batch_code || '待前道产出' }}</span>
              <span>本次 {{ format(group, selected.includes(group.key) ? row.actual : 0) }}<template v-if="row.extra && selected.includes(group.key)">（额外 WIP {{ format(group, row.extra) }}）</template></span>
            </div>
          </details>
        </article>
        <section v-if="preview.blocked.length" class="blocked"><h3>需单独处理</h3>
          <div v-for="row in preview.blocked" :key="row.job_card_id"><span>{{ row.work_order_no }} · {{ row.reason }}</span><button v-if="row.draft_id" type="button" :disabled="saving" @click="openDocument({id: row.draft_id, work_order_id: 0, job_card_id: row.job_card_id})">查看原草稿</button></div>
        </section>
      </template>
      <footer>
        <span v-if="uncertain">提交结果尚未确认，请重试查询同一次领料结果。</span>
        <span v-else-if="result">刷新建议后可继续处理剩余物料。</span>
        <span v-else>待前道产出或暂不可领的物料会继续保留待办。</span>
        <button class="primary" type="button" :disabled="saving || loading || (!uncertain && (!selected.length || invalid || !!result))" @click="submit">{{ saving ? '正在领料…' : uncertain ? '重试确认领料结果' : '一键领料并过账' }}</button>
      </footer>
    </section>
  </div>
</template>
<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiSend } from '../api/client.js'
import { batchIssueAllocations, batchIssueFactor, batchIssuePayload, batchIssueQuantity, batchIssueValidation } from '../lib/workstation-batch-issue.js'
const props = defineProps({ taskIds: { type: Array, required: true }, workstation: String, returnParams: Object })
const emit = defineEmits(['close', 'posted'])
const loading = ref(false), saving = ref(false), error = ref(''), preview = ref(null), result = ref(null)
const selected = ref([]), quantities = ref({}), uncertain = ref(false)
let retryPayload = null
const locked = computed(() => saving.value || uncertain.value || !!result.value)
const eligibleGroups = computed(() => (preview.value?.groups || []).filter(group => group.available > 0))
const allGroupsSelected = computed(() => eligibleGroups.value.length > 0 && eligibleGroups.value.every(group => selected.value.includes(group.key)))
const someGroupsSelected = computed(() => selected.value.length > 0 && !allGroupsSelected.value)
const invalid = computed(() => (preview.value?.groups || []).some(group => selected.value.includes(group.key) && batchIssueValidation(group, quantities.value[group.key])))
const format = (group, qty) => `${batchIssueQuantity(group, qty).toLocaleString('zh-CN', { maximumFractionDigits: 6 })} ${group.unit || ''}`
const warehouse = value => ({ raw_materials: '原料仓', packaging: '包材仓', wip: 'WIP', finished_goods: '成品仓' })[value] || value || '待确认来源'
function toggleGroups() { selected.value = allGroupsSelected.value ? [] : eligibleGroups.value.map(group => group.key) }
async function refresh() {
  loading.value = true; error.value = ''; result.value = null; retryPayload = null
  try {
    preview.value = await apiSend('/api/production/workstation/material-issue/preview', { body: { job_card_ids: props.taskIds } })
    selected.value = preview.value.groups.filter(group => group.available > 0).map(group => group.key)
    quantities.value = Object.fromEntries(preview.value.groups.map(group => [group.key, batchIssueQuantity(group, group.available)]))
  } catch (err) { preview.value = null; error.value = err.message || '领料建议加载失败' }
  finally { loading.value = false }
}
async function submit() {
  if (saving.value || (!uncertain.value && (!selected.value.length || invalid.value))) return
  saving.value = true; error.value = ''
  retryPayload ||= batchIssuePayload(preview.value, selected.value, quantities.value, crypto.randomUUID())
  try {
    result.value = await apiSend('/api/production/workstation/material-issue/submit', { body: retryPayload })
    uncertain.value = false; retryPayload = null; emit('posted', result.value)
  } catch (err) {
    uncertain.value = !err.status || err.status >= 500
    if (!uncertain.value) retryPayload = null
    error.value = err.message || '领料结果暂未确认，请重试'
  } finally { saving.value = false }
}
function openDocument(doc) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'stockOperations', params: { tab: 'stockEntries', stock_entry_id: doc.id }, returnNavigation: { key: 'workstationView', params: { ...props.returnParams, workstation: props.workstation }, label: '返回工位' } } }))
}
onMounted(refresh)
</script>
<style scoped>
.batch-issue-backdrop{position:fixed;inset:0;z-index:1100;background:#0006;display:flex;justify-content:flex-end}.batch-issue-drawer{width:min(100%,980px);height:100%;overflow:auto;background:#fff;padding:24px;box-sizing:border-box;color:#27332b}.batch-issue-drawer header{display:flex;align-items:flex-start;justify-content:space-between;gap:16px}.batch-issue-drawer h2{margin:0;font-size:23px}.batch-issue-drawer p{line-height:1.55}.batch-issue-drawer header p,.intro,small,dt{color:#67736c}.batch-issue-drawer button{border:1px solid #ccd5ce;border-radius:7px;padding:8px 12px;background:#fff;cursor:pointer}.batch-issue-drawer button:disabled{opacity:.5;cursor:not-allowed}.batch-issue-drawer .primary{background:#243d30;color:white;border-color:#243d30;font-weight:650}.batch-issue-drawer input[type=checkbox]{width:17px;height:17px;vertical-align:middle;margin-right:8px;accent-color:#315740}.selection-bar{display:flex;gap:18px;align-items:center;flex-wrap:wrap;margin:20px 0}.selection-bar button{margin-left:auto}.material-group{border:1px solid #dce3dd;border-radius:10px;padding:18px;margin:12px 0}.material-group.unavailable{background:#f8f9f7}.group-title{display:flex;flex-direction:column;gap:7px}.group-title strong{font-size:16px}.quantities{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin:20px 0}dt{font-size:12px}dd{margin:7px 0 0;font-weight:650}.actual-row{display:flex;align-items:center;gap:18px;flex-wrap:wrap}.actual-row input{width:125px;padding:8px;border:1px solid #b7c5bc;border-radius:6px;font:inherit}.actual-row input:disabled{background:#f4f5f3}.warning{color:#9b640d;font-size:13px}.error{color:#a6382b;font-size:13px}.material-group details{margin-top:16px;font-size:13px}.material-group summary{cursor:pointer;color:#456451}.allocation{display:flex;justify-content:space-between;gap:10px;flex-wrap:wrap;padding:10px 0;border-bottom:1px solid #edf0eb}.result{background:#f2f7f3;padding:16px;border-radius:10px;margin:15px 0}.result-row{display:flex;gap:8px;flex-wrap:wrap;margin:12px 0}.result-row strong{width:100%}.document-link{color:#275139}.blocked{border:1px solid #ead8ad;padding:16px;margin:16px 0;border-radius:8px}.blocked>div{display:flex;justify-content:space-between;gap:12px;margin:10px 0}.blocked h3{font-size:16px;margin:0}.batch-issue-drawer footer{position:sticky;bottom:-24px;background:#fff;box-shadow:0 -8px 16px #ffffffd9;border-top:1px solid #dce3dd;display:flex;justify-content:space-between;align-items:center;gap:16px;padding:18px 0;margin-top:20px;font-size:13px}.empty{padding:35px;text-align:center;color:#67736c}@media(max-width:650px){.batch-issue-drawer{padding:16px}.quantities{grid-template-columns:repeat(2,1fr)}.batch-issue-drawer footer{bottom:-16px;align-items:stretch;flex-direction:column}.blocked>div{flex-direction:column}}
</style>
