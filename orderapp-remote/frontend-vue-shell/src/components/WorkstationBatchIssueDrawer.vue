<template>
  <div class="batch-issue-backdrop">
    <section class="batch-issue-drawer" role="dialog" aria-modal="true" aria-labelledby="batch-issue-title">
      <header>
        <div><h2 id="batch-issue-title">{{ isReplenishment ? '补料转仓' : '合并领料' }}</h2><p>{{ workstation }} · 已选 {{ taskIds.length }} 个任务</p></div>
        <button type="button" :disabled="saving || uncertain" @click="$emit('close')">关闭</button>
      </header>
      <p class="intro">{{ isReplenishment ? '先生成待确认转仓单，核对来源和批次后在此确认调入 WIP。' : '核对实际数量后，一次领到 WIP 并过账。多领部分留在 WIP，生产消耗另行记录。' }}</p>
      <p v-if="uncertain" class="warning">存在未确认的领料，请使用原账号重试同一次请求；不要重新领料。</p>
      <div v-if="startShortages.length" class="warning"><p v-for="item in startShortages" :key="item.key">本批开工缺口：{{ item.name }} {{ item.shortage_qty }} {{ item.unit }}</p><p>缺口为建议调入量，可在可调库存内增加数量，多调部分留在 WIP。</p></div>
      <p v-if="loading" role="status">正在汇总物料与可领库存…</p>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <section v-if="draftReceipt" class="result" role="status"><strong>待确认转仓单</strong><div v-for="row in draftReceipt.groups" :key="row.key" class="result-row"><strong>{{ row.name }}</strong><span>数量 {{ format(row, row.quantity) }}</span><span v-for="doc in row.documents" :key="doc.id">{{ doc.entry_no }}</span></div><p>确认后才调入 WIP。</p></section>
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
          <button type="button" :disabled="saving || uncertain || !!draftReceipt" @click="refresh">刷新领料建议</button>
        </div>
        <p v-if="!preview.groups.length && !preview.blocked.length" class="empty">{{ isReplenishment ? '当前没有可生成的补料单。请刷新核对 WIP 是否已补齐，或检查来源仓库存。' : '所选任务当前没有待领物料。' }}</p>
        <article v-for="group in preview.groups" :key="group.key" class="material-group" :class="{ unavailable: !groupCanIssue(group) }">
          <div class="group-title">
            <label><input v-model="selected" type="checkbox" :value="group.key" :disabled="locked || !groupCanIssue(group)" :aria-label="`选择 ${group.name}`" /><strong>{{ group.name }}</strong></label>
            <small>{{ group.owner_name }} · {{ (group.source_warehouses || [group.from_warehouse]).map(warehouse).join('、') }} → WIP<template v-if="group.bom_spec_id"> · 规格 #{{ group.bom_spec_id }}</template></small>
          </div>
          <dl class="quantities">
            <div><dt>需求</dt><dd>{{ format(group, group.required) }}</dd></div>
            <div><dt>已领 / WIP 已到位</dt><dd>{{ format(group, group.wip) }}</dd></div>
            <div><dt>待领</dt><dd>{{ format(group, group.remaining) }}</dd></div>
            <div><dt>本次可领</dt><dd>{{ format(group, group.available) }}</dd></div>
          </dl>
          <div class="actual-row">
            <label>实际领用量 <input v-model.number="quantities[group.key]" type="number" min="0" :step="group.quantity_basis === 'weight' ? 1 / batchIssueFactor(group) : 1" :disabled="locked || !groupCanIssue(group) || !selected.includes(group.key)" :aria-label="`${group.name} 实际领用量`" /> {{ group.unit }}</label>
            <small>含额外 WIP 最多可领 {{ format(group, group.max_quantity) }}</small>
          </div>
          <p v-if="selected.includes(group.key) && groupIssueValidation(group, quantities[group.key])" class="error">{{ groupIssueValidation(group, quantities[group.key]) }}</p>
          <p v-if="group.reason" class="warning">{{ group.reason }}；未领部分继续保留待办。</p>
          <details :open="isReplenishment"><summary>查看 {{ group.allocations.length }} 条工单及批次分配</summary>
            <div v-for="(row, index) in batchIssueAllocations(group, quantities[group.key])" :key="`${row.job_card_id}:${row.binding_id}:${index}`" class="allocation">
              <span>{{ row.work_order_no }} · 任务 #{{ row.job_card_id }}</span><span>{{ warehouse(row.from_warehouse || group.from_warehouse) }} · 批次 {{ row.batch_code || '待前道产出' }} · 可调 {{ format(group, Number(row.available || 0) + Number(row.extra_available || 0)) }}</span>
              <span>本次 {{ format(group, selected.includes(group.key) ? row.actual : 0) }}<template v-if="row.extra && selected.includes(group.key)">（额外 WIP {{ format(group, row.extra) }}）</template></span>
            </div>
          </details>
        </article>
        <section v-if="preview.blocked.length" class="blocked"><h3>需单独处理</h3>
          <div v-for="row in preview.blocked" :key="`${row.job_card_id}:${row.draft_id || 0}:${row.reason}`"><span>{{ row.work_order_no }} · {{ row.reason }}</span><button v-if="row.draft_id" type="button" :disabled="saving || uncertain || !!draftReceipt" @click="restoreDraft(row)">恢复补料草稿</button><button v-if="row.draft_id" type="button" :disabled="saving || uncertain || !!draftReceipt" @click="openDocument({id: row.draft_id, work_order_id: 0, job_card_id: row.job_card_id})">查看原草稿</button></div>
        </section>
      </template>
      <footer>
        <span v-if="uncertain">提交结果尚未确认，暂不可关闭，请重试查询同一次领料结果。</span>
        <span v-else-if="result">刷新建议后可继续处理剩余物料。</span>
        <span v-else>待前道产出或暂不可领的物料会继续保留待办。</span>
        <button v-if="draftReceipt" type="button" :disabled="saving" @click="cancelDraft">{{ cancelUncertain ? '重试取消补料草稿' : '取消补料草稿' }}</button>
        <button class="primary" type="button" :disabled="saving || loading || cancelUncertain || (!uncertain && !draftReceipt && (!selected.length || invalid || !!result)) || !!result" @click="submit">{{ saving ? '处理中…' : isReplenishment ? (draftReceipt ? '确认调入 WIP' : uncertain ? '重试生成转仓单' : '生成补料转仓单') : uncertain ? '重试确认领料结果' : '一键领料并过账' }}</button>
      </footer>
    </section>
  </div>
</template>
<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client.js'
import { batchIssueAllocations, batchIssueFactor, batchIssuePayload, batchIssueQuantity, batchIssueValidation, mergeBatchIssueGroups, shortageIssueDefaults } from '../lib/workstation-batch-issue.js'
import { issueSessionStorage, readPendingIssue, savePendingIssue, clearPendingIssue, validIssueResult } from '../lib/workstation-issue-recovery.js'
const props = defineProps({ taskIds: { type: Array, required: true }, workstation: String, returnParams: Object, replenishment:{type:Boolean,default:false}, startShortages: { type: Array, default: () => [] }, startInput: { type: Object, default: null } })
const emit = defineEmits(['close', 'posted', 'cancelled'])
const loading = ref(false), saving = ref(false), error = ref(''), preview = ref(null), result = ref(null), draftReceipt = ref(null), pendingWorkflow = ref('')
const selected = ref([]), quantities = ref({}), uncertain = ref(false), cancelUncertain = ref(false)
const warehouseNames = ref(new Map())
let retryPayload = null
const isReplenishment = computed(() => props.replenishment || props.startShortages.length > 0 || pendingWorkflow.value === 'replenishment')
const locked = computed(() => saving.value || uncertain.value || !!result.value || !!draftReceipt.value)
const shortageLimit = group => shortageIssueDefaults([group], props.startShortages).quantities[group.key] || 0
const groupCanIssue = group => props.startShortages.length ? shortageLimit(group) > 0 : Number(group.available) > 0
const groupIssueValidation = (group, quantity) => {
  const error = batchIssueValidation(group, quantity)
  if (error) return error
  return ''
}
const eligibleGroups = computed(() => (preview.value?.groups || []).filter(groupCanIssue))
const allGroupsSelected = computed(() => eligibleGroups.value.length > 0 && eligibleGroups.value.every(group => selected.value.includes(group.key)))
const someGroupsSelected = computed(() => selected.value.length > 0 && !allGroupsSelected.value)
const invalid = computed(() => (preview.value?.groups || []).some(group => selected.value.includes(group.key) && groupIssueValidation(group, quantities.value[group.key])))
const format = (group, qty) => `${batchIssueQuantity(group, qty).toLocaleString('zh-CN', { maximumFractionDigits: 6 })} ${group.unit || ''}`
const warehouse = value => { const code = String(value || ''); return warehouseNames.value.get(code) || ({ raw_materials: '原料仓', packaging: '包材仓', wip: 'WIP在制仓', finished_goods: '成品仓' })[code] || (code ? `仓库名待加载（${code}）` : '待确认来源') }
async function loadWarehouseNames() { try { const data = await apiGet('/api/stock/warehouses'); const rows = Array.isArray(data) ? data : (data.rows || data.warehouses || []); warehouseNames.value = new Map(rows.filter(row => row?.code).map(row => [String(row.code), String(row.name || row.code)])) } catch { warehouseNames.value = new Map() } }
function toggleGroups() { selected.value = allGroupsSelected.value ? [] : eligibleGroups.value.map(group => group.key) }
async function refresh() {
  loading.value = true; error.value = ''; result.value = null; retryPayload = null
  try {
    preview.value = await apiSend('/api/production/workstation/material-issue/preview', { body: { job_card_ids: props.taskIds, ...(isReplenishment.value ? {workflow:'replenishment', ...(props.startInput ? {start_input:props.startInput} : {})} : {}) } })
    preview.value.groups = mergeBatchIssueGroups(preview.value.groups)
    if (props.startShortages.length) {
      const defaults = shortageIssueDefaults(preview.value.groups, props.startShortages)
      selected.value = defaults.selected; quantities.value = defaults.quantities
    } else {
      selected.value = preview.value.groups.filter(group => group.available > 0).map(group => group.key)
      quantities.value = Object.fromEntries(preview.value.groups.map(group => [group.key, batchIssueQuantity(group, group.available)]))
    }
  } catch (err) { preview.value = null; error.value = err.message || '领料建议加载失败' }
  finally { loading.value = false }
}
async function submit() {
  if (saving.value || (!uncertain.value && !draftReceipt.value && (!selected.value.length || invalid.value))) return
  saving.value = true; error.value = ''
  let sent = false
  try {
    if (!retryPayload) {
      const candidate = batchIssuePayload(preview.value, selected.value, quantities.value, crypto.randomUUID())
      if (isReplenishment.value) { candidate.workflow = 'replenishment'; if (props.startInput) candidate.start_input = props.startInput }
      savePendingIssue(issueSessionStorage(), candidate)
      retryPayload = candidate
    }
    sent = true
    if (retryPayload.workflow === 'replenishment' && !draftReceipt.value) {
      const draft = await apiSend('/api/production/workstation/material-issue/draft', { body: retryPayload })
      if (draft.request_id !== retryPayload.request_id || !Array.isArray(draft.groups) || draft.groups.length !== retryPayload.groups.length || draft.groups.some(row => row.status !== 'draft' || !row.documents?.length)) throw new Error('转仓单结果不完整')
      draftReceipt.value = draft; uncertain.value = false; pendingWorkflow.value = 'replenishment'; return
    }
    const receipt = await apiSend(retryPayload.workflow === 'replenishment' ? '/api/production/workstation/material-issue/confirm' : '/api/production/workstation/material-issue/submit', { body: retryPayload.workflow === 'replenishment' ? {request_id:retryPayload.request_id} : retryPayload })
    if (!validIssueResult(receipt, retryPayload)) throw new Error('领料结果不完整，请重试确认')
    clearPendingIssue(issueSessionStorage(), retryPayload)
    result.value = receipt; uncertain.value = false; retryPayload = null; draftReceipt.value = null; pendingWorkflow.value = ''; emit('posted', result.value)
  } catch (err) {
    if (!sent) { error.value = err.message || '无法保存领料重试记录，本次未提交。'; return }
    // This endpoint reports even transaction-commit errors as HTTP 400, so a
    // status code cannot prove that a sent request did not take effect.
    uncertain.value = true
    error.value = draftReceipt.value ? `确认调入未完成：${err.message || '请重试同一次请求'}` : '领料结果尚未确认，请使用原账号重试同一次请求；如反复失败，请联系管理员核对。'
  } finally { saving.value = false }
}
async function cancelDraft() {
  if (saving.value || !draftReceipt.value || !retryPayload) return
  saving.value=true; error.value=''
  try {
    const receipt=await apiSend('/api/production/workstation/material-issue/cancel',{body:{request_id:retryPayload.request_id}})
    if (receipt?.request_id!==retryPayload.request_id || receipt.cancelled!==true) throw new Error('取消结果不完整')
    clearPendingIssue(issueSessionStorage(),retryPayload)
    retryPayload=null; draftReceipt.value=null; uncertain.value=false; cancelUncertain.value=false; pendingWorkflow.value=''
    emit('cancelled'); await refresh()
  } catch(err) {
    uncertain.value=true; cancelUncertain.value=!/已提交|已过账/.test(err.message||'')
    error.value=cancelUncertain.value ? '取消结果尚未确认，请重试取消原草稿。' : '该单已提交，请重试确认以获取结果。'
  } finally { saving.value=false }
}
function openDocument(doc) {
  if (saving.value || uncertain.value) return
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'stockOperations', params: { tab: 'stockEntries', stock_entry_id: doc.id }, returnNavigation: { key: 'workstationView', params: { ...props.returnParams, workstation: props.workstation }, label: '返回工位' } } }))
}
async function restoreDraft(row) {
  if (saving.value || uncertain.value || draftReceipt.value) return
  saving.value = true; error.value = ''
  try {
    const id = Number(row?.job_card_id)
    if (!Number.isSafeInteger(id) || id <= 0 || !props.taskIds.map(Number).includes(id)) throw new Error('任务已变化，请重新打开')
    const data = await apiGet(`/api/production/workstation/material-issue/draft?job_card_id=${id}`)
    if (!data?.found) throw new Error('该单据不是可恢复的补料草稿，请查看原草稿')
    const command = {...data.command, workflow:'replenishment'}
    if (!command.job_card_ids?.map(Number).includes(id) || !data.result || data.result.request_id !== command.request_id || !data.result.groups?.every(g=>g.status==='draft' && g.documents?.length)) throw new Error('补料草稿信息不完整，请查看原草稿')
    savePendingIssue(issueSessionStorage(), command)
    retryPayload=command; pendingWorkflow.value='replenishment'; draftReceipt.value=data.result
  } catch(err) { error.value=err.message || '无法恢复补料草稿' }
  finally { saving.value=false }
}
function restorePending() {
  try {
    const pending = readPendingIssue(issueSessionStorage())
    if (!pending) return false
    retryPayload = pending; pendingWorkflow.value = pending.workflow || ''; uncertain.value = true
    error.value = '已恢复未确认的领料，请使用原账号重试同一次请求。'
    return true
  } catch (err) { error.value = err.message; return true }
}
function guardNavigation(event) { if (saving.value || uncertain.value) event.preventDefault() }
function guardUnload(event) { if (saving.value || uncertain.value) { event.preventDefault(); event.returnValue = '' } }
onMounted(() => { window.addEventListener('kferp:before-navigate',guardNavigation); window.addEventListener('beforeunload',guardUnload); void loadWarehouseNames(); if (!restorePending()) void refresh() })
onBeforeUnmount(() => { window.removeEventListener('kferp:before-navigate',guardNavigation); window.removeEventListener('beforeunload',guardUnload) })
</script>
<style scoped>
.batch-issue-backdrop{position:fixed;inset:0;z-index:1100;background:#0006;display:flex;justify-content:flex-end}.batch-issue-drawer{width:min(100%,980px);height:100%;overflow:auto;background:#fff;padding:24px;box-sizing:border-box;color:#27332b}.batch-issue-drawer header{display:flex;align-items:flex-start;justify-content:space-between;gap:16px}.batch-issue-drawer h2{margin:0;font-size:23px}.batch-issue-drawer p{line-height:1.55}.batch-issue-drawer header p,.intro,small,dt{color:#67736c}.batch-issue-drawer button{border:1px solid #ccd5ce;border-radius:7px;padding:8px 12px;background:#fff;cursor:pointer}.batch-issue-drawer button:disabled{opacity:.5;cursor:not-allowed}.batch-issue-drawer .primary{background:#243d30;color:white;border-color:#243d30;font-weight:650}.batch-issue-drawer input[type=checkbox]{width:17px;height:17px;vertical-align:middle;margin-right:8px;accent-color:#315740}.selection-bar{display:flex;gap:18px;align-items:center;flex-wrap:wrap;margin:20px 0}.selection-bar button{margin-left:auto}.material-group{border:1px solid #dce3dd;border-radius:10px;padding:18px;margin:12px 0}.material-group.unavailable{background:#f8f9f7}.group-title{display:flex;flex-direction:column;gap:7px}.group-title strong{font-size:16px}.quantities{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin:20px 0}dt{font-size:12px}dd{margin:7px 0 0;font-weight:650}.actual-row{display:flex;align-items:center;gap:18px;flex-wrap:wrap}.actual-row input{width:125px;padding:8px;border:1px solid #b7c5bc;border-radius:6px;font:inherit}.actual-row input:disabled{background:#f4f5f3}.warning{color:#9b640d;font-size:13px}.error{color:#a6382b;font-size:13px}.material-group details{margin-top:16px;font-size:13px}.material-group summary{cursor:pointer;color:#456451}.allocation{display:flex;justify-content:space-between;gap:10px;flex-wrap:wrap;padding:10px 0;border-bottom:1px solid #edf0eb}.result{background:#f2f7f3;padding:16px;border-radius:10px;margin:15px 0}.result-row{display:flex;gap:8px;flex-wrap:wrap;margin:12px 0}.result-row strong{width:100%}.document-link{color:#275139}.blocked{border:1px solid #ead8ad;padding:16px;margin:16px 0;border-radius:8px}.blocked>div{display:flex;justify-content:space-between;gap:12px;margin:10px 0}.blocked h3{font-size:16px;margin:0}.batch-issue-drawer footer{position:sticky;bottom:-24px;background:#fff;box-shadow:0 -8px 16px #ffffffd9;border-top:1px solid #dce3dd;display:flex;justify-content:space-between;align-items:center;gap:16px;padding:18px 0;margin-top:20px;font-size:13px}.empty{padding:35px;text-align:center;color:#67736c}@media(max-width:650px){.batch-issue-drawer{padding:16px}.quantities{grid-template-columns:repeat(2,1fr)}.batch-issue-drawer footer{bottom:-16px;align-items:stretch;flex-direction:column}.blocked>div{flex-direction:column}}
</style>
