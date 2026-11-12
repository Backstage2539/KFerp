<template>
  <Teleport to="body">
    <div class="transfer-overlay" @click.self="close">
      <section class="transfer-drawer" role="dialog" aria-modal="true" :aria-label="title" @keydown.esc.stop="close">
        <header><div><small>{{ detail.plan_no || '生产计划' }}</small><h2>{{ title }}</h2></div><button type="button" class="close" aria-label="关闭" :disabled="busy" @click="close">×</button></header>
        <main>
          <div v-if="error" class="error" role="alert">{{ error }}</div>
          <p v-if="adjustmentNotice" class="hint" role="status">{{ adjustmentNotice }}</p>
          <template v-if="mode !== 'bulk' && currentSource">
            <h3>{{ currentSource.component_name }}</h3>
            <div class="summary">
              <div><span>本项需求</span><strong>{{ amount(currentSource.demand_g ?? currentSource.required_g, currentSource.demand_units ?? currentSource.required_units) }} {{ unit }}</strong></div>
              <div><span>WIP库存</span><strong>{{ amount(currentSource.wip_covered_g, currentSource.wip_covered_units) }} {{ unit }}</strong></div>
              <div><span>缺料</span><strong class="missing">{{ shortage.missing }} {{ unit }}</strong></div>
            </div>
            <p v-if="mode === 'single'" class="muted">已生成待处理：{{ shortage.pending }} {{ unit }} · 建议本次调拨：{{ shortage.remaining }} {{ unit }}；可按批次可用库存调整数量</p>
            <div v-if="pendingDocuments.length" class="pending"><strong>关联调拨单</strong><p v-for="doc in pendingDocuments" :key="doc.id">{{ doc.entry_no || doc.id }} · {{ doc.status_label || transferDocumentStatus(doc.status) }} · {{ amount(doc.qty_g, doc.qty_units) }} {{ unit }}</p></div>
            <p v-if="Number(currentSource.upstream_g || 0) || Number(currentSource.upstream_units || 0)" class="hint">本计划上游供应 {{ amount(currentSource.upstream_g, currentSource.upstream_units) }} {{ unit }}；未产出的物料不能生成现货调拨单。</p>
            <label class="field"><span>{{ mode === 'single' ? '选择来源仓库' : '筛选仓库' }}</span><select v-model="warehouse" :disabled="busy" @change="loadOptions"><option value="">{{ mode === 'stock' ? '全部仓库' : '请选择仓库' }}</option><option v-for="item in warehouseOptions" :key="item.code" :value="item.code">{{ item.name }}（{{ item.code }}）</option></select></label>
            <p v-if="mode === 'single'" class="muted">调拨去向：WIP在制仓（wip）；货主保持不变。</p>
            <h4>{{ mode === 'single' ? '选择批次与数量' : '库存批次' }}</h4>
            <div v-if="loading" class="empty">正在加载库存…</div>
            <div v-else-if="mode === 'single' && !warehouse" class="empty">暂无可调拨的来源仓库；请核对供应或等待上游入库。</div>
            <div v-else-if="!rows.length" class="empty">该仓库没有匹配的可用批次；请核对供应或其他仓库。</div>
            <div v-else class="table-scroll"><p class="scroll-hint">左右滑动查看完整信息</p><table><thead><tr><th>批次</th><th v-if="mode === 'stock'">仓库</th><th>货主</th><th>质量</th><th>库存</th><th>占用</th><th v-if="mode === 'single'">本计划待确认</th><th>{{ mode === 'single' ? '可调' : '可用' }}</th><th v-if="mode === 'single'">本次调拨</th></tr></thead><tbody><tr v-for="row in rows" :key="`${row.batch_id}-${row.owner_customer_id}`"><td>{{ row.batch_code }}<small v-if="transferRowIssue(currentSource, row)" class="missing">{{ transferRowIssue(currentSource, row) }}</small></td><td v-if="mode === 'stock'">{{ warehouseOptions.find(item => item.code === row.warehouse)?.name || row.warehouse || '—' }}</td><td>{{ row.owner_name || (row.owner_customer_id ? `客户 #${row.owner_customer_id}` : '工厂') }}</td><td>{{ transferQualityLabel(row.quality_status) }}</td><td>{{ amount(row.qty_g, row.qty_units) }}</td><td>{{ reservedAmount(currentSource, row) ?? '—' }}</td><td v-if="mode === 'single'">{{ amount(row.pending_plan_g, row.pending_plan_units) }} {{ unit }}</td><td>{{ mode === 'single' ? transferUnitAmount(currentSource, row.transferable_g ?? row.available_g, row.transferable_units ?? row.available_units) : amount(row.available_g, row.available_units) }} {{ unit }}</td><td v-if="mode === 'single'"><input v-model.number="row.quantity" :disabled="busy || !!transferRowIssue(currentSource, row)" type="number" min="0" :step="isWeight ? '0.001' : '1'" :aria-label="`${row.batch_code}调拨数量`" @input="preview = null" /></td></tr></tbody></table></div>
            <template v-if="mode === 'stock' && editable">
              <h4>本次备料来源</h4><p class="muted">这里调整来源建议，不会转仓。</p>
              <label v-for="row in adjustmentRows" :key="`${row.warehouse}-${row.owner_customer_id}`" class="adjust-row"><span>{{ row.warehouse_name || row.warehouse }} · {{ row.owner_name || '工厂' }}（可用 {{ amount(row.available_g, row.available_units) }} {{ unit }}）</span><input v-model.number="row.quantity" type="number" min="0" :step="isWeight ? '0.001' : '1'" /></label>
              <div class="actions"><button type="button" :disabled="busy" @click="saveAdjustment">{{ adjusting ? '正在保存…' : '保存来源建议' }}</button><button v-if="currentSource.allocation_mode === 'manual'" type="button" :disabled="busy" @click="sendAdjustment(null)">恢复自动建议</button></div>
            </template>
          </template>
          <template v-else-if="mode === 'bulk'">
            <div class="summary"><div><span>本次可生成</span><strong>{{ previewItems.length }} 项</strong></div><div><span>暂不生成</span><strong>{{ unavailable.length }} 项</strong></div></div>
            <p class="muted">沿用已保存的备料来源；已有待处理单据不重复生成。</p>
            <h4>本次生成</h4>
            <p v-if="loading" class="empty">正在核对库存…</p>
            <p v-else-if="!previewItems.length" class="empty">当前没有可生成的调拨单。</p>
            <div v-else class="table-scroll"><p class="scroll-hint">左右滑动查看完整信息</p><table><thead><tr><th>物料</th><th>WIP缺料</th><th>来源仓库</th><th>货主</th><th>来源批次及数量</th><th>操作</th></tr></thead><tbody><tr v-for="(item, i) in previewItems" :key="`${item.source_id}-${i}`"><td><label><input type="checkbox" :checked="!excludedItems.includes(itemKey(item))" :aria-label="`选择${sourceName(item.source_id)}`" @change="toggleBulkItem(item, $event.target.checked)" />{{ item.component_name || sourceName(item.source_id) }}</label></td><td>{{ sourceShortage(item.source_id) }}</td><td>{{ item.warehouse_name || item.warehouse }}</td><td>{{ item.owner_name || (item.owner_customer_id ? `客户 #${item.owner_customer_id}` : '工厂') }}</td><td><div v-for="(batch, j) in item.batches || []" :key="`${batch.batch_code}-${j}`">{{ batch.batch_code }} · {{ sourceBatchAmount(item.source_id, batch) }}</div></td><td><button type="button" :disabled="busy" @click="beginBulkEdit(i)">调整来源</button></td></tr></tbody></table></div>
            <section v-if="bulkEditIndex !== null" class="bulk-edit">
              <h4>调整 {{ bulkSource?.component_name || '物料' }} 来源</h4>
              <label class="field"><span>来源仓库</span><select v-model="bulkWarehouse" :disabled="busy" @change="loadBulkOptions"><option value="">请选择来源仓库</option><option v-for="item in warehouseOptions" :key="item.code" :value="item.code">{{ item.name }}（{{ item.code }}）</option></select></label>
              <p v-if="bulkPendingDocuments.length" class="muted">该物料已有待处理单据，建议数量已扣除该部分；可按批次可用库存调整。</p>
              <p v-if="!bulkRows.length" class="empty">没有匹配的可用批次，请选择其他仓库。</p>
              <div v-else class="table-scroll"><p class="scroll-hint">左右滑动查看完整信息</p><table><thead><tr><th>批次</th><th>货主</th><th>本计划待确认</th><th>可调</th><th>本次调拨</th></tr></thead><tbody><tr v-for="row in bulkRows" :key="`${row.batch_id}-${row.owner_customer_id}`"><td>{{ row.batch_code }}<small v-if="transferRowIssue(bulkSource, row)" class="missing">{{ transferRowIssue(bulkSource, row) }}</small></td><td>{{ row.owner_name || (row.owner_customer_id ? `客户 #${row.owner_customer_id}` : '工厂') }}</td><td>{{ transferUnitAmount(bulkSource, row.pending_plan_g, row.pending_plan_units) }} {{ transferDisplayUnit(bulkSource) }}</td><td>{{ transferUnitAmount(bulkSource, row.transferable_g ?? row.available_g, row.transferable_units ?? row.available_units) }} {{ transferDisplayUnit(bulkSource) }}</td><td><input v-model.number="row.quantity" :disabled="busy || !!transferRowIssue(bulkSource, row)" type="number" min="0" :step="isTransferWeight(bulkSource) ? '0.001' : '1'" :aria-label="`${row.batch_code}调拨数量`" /></td></tr></tbody></table></div>
              <div class="actions"><button type="button" :disabled="busy" @click="applyBulkAdjustment">重新核对</button><button type="button" :disabled="busy" @click="bulkEditIndex = null">取消调整</button></div>
            </section>
            <section v-if="bulkPendingDocuments.length && bulkEditIndex === null" class="pending"><strong>已有关联调拨单</strong><p v-for="doc in bulkPendingDocuments" :key="`${doc.id}-${doc.source_id}`">{{ doc.entry_no || doc.id }} · {{ doc.status_label || transferDocumentStatus(doc.status) }} · {{ sourceName(doc.source_id) }} {{ sourceBatchAmount(doc.source_id, doc) }}</p></section>
            <h4>暂不生成的物料</h4><div v-if="unavailable.length" class="table-scroll"><p class="scroll-hint">左右滑动查看完整信息</p><table><thead><tr><th>物料</th><th>WIP缺料</th><th>原因与处理</th></tr></thead><tbody><tr v-for="(item, index) in unavailable" :key="`${item.source_id}-${index}`"><td>{{ item.component_name || sourceName(item.source_id) }}</td><td>{{ sourceShortage(item.source_id) }}</td><td>{{ item.reason || item.message || '暂无可用来源' }}</td></tr></tbody></table></div><p v-else class="muted">暂无。</p>
          </template>
          <div v-if="mode !== 'stock'" class="hint"><strong>生成后由仓库确认</strong><p>本次只生成待处理调拨单，不立即移动库存。仓库核对并确认后，WIP库存自动更新；超出本计划需求的部分留作 WIP 可用库存。</p></div>
        </main>
        <footer><span v-if="mode === 'single'">{{ total }} {{ unit }}</span><span v-else-if="mode === 'bulk'">{{ documentCount ? `将生成 ${documentCount} 张调拨单` : '尚无可生成单据' }}</span><span v-else></span><div class="actions"><button type="button" :disabled="busy" @click="close">取消</button><button v-if="mode === 'single'" class="primary" type="button" :disabled="busy || !rows.length" @click="submitSingle">生成调拨单</button><button v-if="mode === 'bulk'" class="primary" type="button" :disabled="busy || bulkEditIndex !== null || !documentCount || !previewItems.length" @click="submitBulk">生成 {{ documentCount }} 张调拨单</button></div></footer>
      </section>
    </div>
  </Teleport>
</template>
<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client.js'
import { remainingTransfer, transferSelections, validateTransferSelections, transferUnitAmount, transferGrams, replaceTransferSelection, pendingTransferAmount, reservedAmount, transferQualityLabel, transferDocumentStatus, isTransferWeight, transferDisplayUnit, transferRowIssue, suggestedTransferRows } from '../lib/production-plan-transfer.js'
const props = defineProps({ detail: { type: Object, required: true }, source: { type: Object, default: null }, mode: { type: String, default: 'stock' }, warehouses: { type: Array, default: () => [] }, editable: { type: Boolean, default: true } })
const emit = defineEmits(['close', 'updated', 'adjust'])
const title = computed(() => ({stock: '物料库存', single: '生成调拨单', bulk: '生成所有缺料调拨单'})[props.mode] || '物料库存')
const warehouse = ref(''), rows = ref([]), serverSource = ref(null), serverWarehouses = ref(null), pendingDocuments = ref([]), loading = ref(false), saving = ref(false), adjusting = ref(false), adjustmentNotice = ref(''), error = ref(''), preview = ref(null), adjustmentRows = ref([])
const busy = computed(() => loading.value || saving.value || adjusting.value)
const currentSource = computed(() => serverSource.value || props.source)
const isWeight = computed(() => isTransferWeight(currentSource.value))
const unit = computed(() => transferDisplayUnit(currentSource.value))
const pending = computed(() => pendingTransferAmount(pendingDocuments.value, currentSource.value))
const shortage = computed(() => remainingTransfer(currentSource.value, pending.value))
const amount = (g, n) => transferUnitAmount(currentSource.value, g, n)
const total = computed(() => Number(rows.value.reduce((n, row) => n + (Number(row.quantity) || 0), 0).toFixed(6)))
const warehouseOptions = computed(() => {
  const list = props.mode === 'bulk' && bulkSource.value ? (bulkSource.value.options || []) : (serverWarehouses.value ?? props.warehouses)
  const distinct = new Map()
  for (const item of list) { const code = item.code || item.warehouse; if (code && (props.mode === 'stock' || code !== 'wip')) distinct.set(code, { code, name: item.name || item.warehouse_name || code }) }
  return [...distinct.values()]
})
const previewItems = computed(() => (preview.value?.items || preview.value?.groups?.flatMap((group) => group.items || []) || []))
const unavailable = computed(() => preview.value?.unavailable || [])
const excludedItems = ref([])
const itemKey = (item) => `${Number(item.source_id || 0)}:${item.warehouse}:${Number(item.owner_customer_id || 0)}`
const chosenPreviewItems = computed(() => previewItems.value.filter((item) => !excludedItems.value.includes(itemKey(item))))
const documentCount = computed(() => new Set(chosenPreviewItems.value.map((item) => `${item.warehouse}:${Number(item.owner_customer_id || 0)}`)).size)
const sourceFor = (id) => props.detail.component_sources?.find((row) => Number(row.id) === Number(id))
const sourceName = (id) => sourceFor(id)?.component_name || `物料 #${id}`
const sourceShortage = (id) => { const source = sourceFor(id); return source ? `${remainingTransfer(source).missing} ${transferDisplayUnit(source)}` : '—' }
const sourceBatchAmount = (id, batch) => { const source = sourceFor(id); return source ? `${transferUnitAmount(source, batch.qty_g, batch.qty_units)} ${transferDisplayUnit(source)}` : (batch.qty_g ? `${batch.qty_g} g` : `${batch.qty_units} 件`) }
function toggleBulkItem(item, checked) { const key = itemKey(item); excludedItems.value = checked ? excludedItems.value.filter((id) => id !== key) : [...new Set([...excludedItems.value, key])] }
const bulkEditIndex = ref(null), bulkWarehouse = ref(''), bulkRows = ref([]), bulkSource = ref(null), bulkPendingDocuments = ref([])
let requestKey = '', requestFingerprint = '', submittedSingleKey = '', optionsRequestSequence = 0
function keyFor(items) { const fingerprint = JSON.stringify([props.detail.draft_token || '', items]); if (!requestKey || fingerprint !== requestFingerprint) { requestKey = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`; requestFingerprint = fingerprint } return requestKey }
function bodyFor(items) { return { draft_token: String(props.detail.draft_token || ''), idempotency_key: keyFor(items), items } }
function close() { if (!busy.value) emit('close') }
function initialiseAdjustments(source) { adjustmentRows.value = (source?.options || []).map((row) => { const fixed = (source.manual_allocations || []).find((a) => a.warehouse === row.warehouse && Number(a.owner_customer_id || 0) === Number(row.owner_customer_id || 0)); return { ...row, quantity: fixed ? amount(fixed.qty_g, fixed.qty_units) : 0 } }) }
function sendAdjustment(allocations) {
  if (busy.value) return
  adjusting.value = true; adjustmentNotice.value = ''; error.value = ''
  emit('adjust', props.source || currentSource.value, allocations, (failure) => {
    adjusting.value = false
    if (failure) { error.value = failure instanceof Error ? failure.message : String(failure); return }
    serverSource.value = { ...currentSource.value, allocation_mode: allocations === null ? 'auto' : 'manual', manual_allocations: allocations || [] }
    initialiseAdjustments(serverSource.value)
    adjustmentNotice.value = allocations === null ? '已恢复自动建议' : '来源建议已保存'
  })
}
function saveAdjustment() {
  const source = currentSource.value
  if (adjustmentRows.value.some((row) => !Number.isFinite(Number(row.quantity)) || Number(row.quantity) < 0 || (!isWeight.value && !Number.isInteger(Number(row.quantity))))) { error.value = '请填写有效数量，计件物料须为整数'; return }
  const allocations = adjustmentRows.value.filter((row) => Number(row.quantity) > 0).map((row) => ({ warehouse: row.warehouse, owner_customer_id: Number(row.owner_customer_id || 0), qty_g: isWeight.value ? transferGrams(source, row.quantity) : 0, qty_units: isWeight.value ? 0 : Number(row.quantity) }))
  if (allocations.some((row) => !Number.isSafeInteger(row.qty_g))) { error.value = '数量须精确到克'; return }
  if (allocations.reduce((n,row) => n + row.qty_g, 0) > Number(source.required_g || 0) || allocations.reduce((n,row) => n + row.qty_units, 0) > Number(source.required_units || 0)) { error.value = '固定数量不能超过本项待落实需求'; return }
  sendAdjustment(allocations)
}
async function loadOptions() {
  const sequence = ++optionsRequestSequence
  rows.value = []; preview.value = null; if (!props.source?.id) { loading.value = false; return }
  loading.value = true; error.value = ''
  try {
    const q = new URLSearchParams({ source_id: String(props.source.id) })
    if (warehouse.value) q.set('warehouse', warehouse.value)
    const result = await apiGet(`/api/production-plans/${props.detail.id}/material-transfer-options?${q}`)
    if (sequence !== optionsRequestSequence) return
    serverSource.value = result.source || null
    serverWarehouses.value = Array.isArray(result.warehouses) ? result.warehouses : null
    pendingDocuments.value = result.pending_documents || []
    if (props.mode === 'single' && !warehouse.value) warehouse.value = warehouseOptions.value[0]?.code || ''
    const candidates = (result.options || []).filter((row) => !warehouse.value || !row.warehouse || row.warehouse === warehouse.value)
    rows.value = props.mode === 'single' && currentSource.value?.allocation_mode !== 'semi_finished_selected' ? suggestedTransferRows(currentSource.value, candidates, pending.value) : candidates.map((row) => ({ ...row, quantity: 0 }))
    initialiseAdjustments(currentSource.value)
  } catch (err) { if (sequence === optionsRequestSequence) error.value = err.message || '库存查询失败，请重试。' } finally { if (sequence === optionsRequestSequence) loading.value = false }
}
async function loadBulkPreview() {
  loading.value = true; error.value = ''
  try { preview.value = await apiSend(`/api/production-plans/${props.detail.id}/material-transfers/preview`, { body: bodyFor([]) }); bulkPendingDocuments.value = preview.value?.pending_documents || [] }
  catch (err) { error.value = err.message || '调拨预览失败，请重试。' } finally { loading.value = false }
}
async function beginBulkEdit(index) {
  const item = previewItems.value[index]
  const source = (props.detail.component_sources || []).find((row) => Number(row.id) === Number(item.source_id))
  if (!source) { error.value = '计划物料已变化，请重新核对。'; return }
  bulkEditIndex.value = index; bulkSource.value = source; bulkWarehouse.value = item.warehouse || ''; bulkRows.value = []
  await loadBulkOptions()
}
async function loadBulkOptions() {
  bulkRows.value = []
  if (!bulkWarehouse.value || !bulkSource.value?.id) return
  loading.value = true; error.value = ''
  try {
    const query = new URLSearchParams({ source_id: String(bulkSource.value.id), warehouse: bulkWarehouse.value })
    const result = await apiGet(`/api/production-plans/${props.detail.id}/material-transfer-options?${query}`)
    bulkSource.value = result.source || bulkSource.value
    bulkPendingDocuments.value = result.pending_documents || []
    const existing = previewItems.value[bulkEditIndex.value]
    bulkRows.value = (result.options || []).map((row) => {
      const batch = existing?.warehouse === bulkWarehouse.value && Number(existing.owner_customer_id || 0) === Number(row.owner_customer_id || 0) ? (existing.batches || []).find((b) => b.batch_code === row.batch_code) : null
      return { ...row, quantity: batch ? transferUnitAmount(bulkSource.value, batch.qty_g, batch.qty_units) : 0 }
    })
  } catch (err) { error.value = err.message || '库存查询失败，请重试。' } finally { loading.value = false }
}
async function applyBulkAdjustment() {
  const selected = transferSelections(bulkSource.value, bulkWarehouse.value, bulkRows.value)
  const invalid = validateTransferSelections(bulkSource.value, selected, pendingTransferAmount(bulkPendingDocuments.value, bulkSource.value))
  if (invalid) { error.value = invalid; return }
  const items = replaceTransferSelection(previewItems.value, bulkEditIndex.value, selected)
  loading.value = true; error.value = ''
  try {
    const checked = await apiSend(`/api/production-plans/${props.detail.id}/material-transfers/preview`, { body: bodyFor(items) })
    const rejected = (checked.unavailable || []).find((row) => Number(row.source_id) === Number(selected.source_id))
    if (rejected) throw new Error(rejected.reason || rejected.message || '本次物料无法生成调拨单，请重新核对。')
    if (!(checked.groups || []).some((group) => (group.items || []).some((row) => Number(row.source_id) === Number(selected.source_id)))) throw new Error('本次物料未通过调拨预览，请重新核对。')
    preview.value = checked
    bulkPendingDocuments.value = preview.value?.pending_documents || []
    bulkEditIndex.value = null
  } catch (err) { error.value = err.message || '调拨预览失败，请重新核对。' } finally { loading.value = false }
}
async function submitSingle() {
  const selected = transferSelections(currentSource.value, warehouse.value, rows.value)
  const invalid = validateTransferSelections(currentSource.value, selected, pending.value)
  if (invalid) { error.value = invalid; return }
  const { _rows, ...item } = selected
  saving.value = true; error.value = ''
  try {
    const body = bodyFor([item])
    if (submittedSingleKey !== body.idempotency_key) {
      const checked = await apiSend(`/api/production-plans/${props.detail.id}/material-transfers/preview`, { body })
      const rejected = (checked.unavailable || []).find((row) => !row.source_id || Number(row.source_id) === Number(item.source_id))
      if (rejected) throw new Error(rejected.reason || rejected.message || '本次物料无法生成调拨单，请重新核对。')
      if (!(checked.groups || []).some((group) => (group.items || []).some((row) => Number(row.source_id) === Number(item.source_id)))) throw new Error('本次物料未通过调拨预览，请重新核对。')
    }
    // A response can be lost after creation. Reuse the validated request and
    // its key on retry so the server can return the existing document.
    submittedSingleKey = body.idempotency_key
    await apiSend(`/api/production-plans/${props.detail.id}/material-transfers`, { body })
    emit('updated'); emit('close')
  } catch (err) { error.value = err.message || '生成调拨单失败，请重新核对。' } finally { saving.value = false }
}
async function submitBulk() {
  if (!chosenPreviewItems.value.length) return
  saving.value = true; error.value = ''
  try { await apiSend(`/api/production-plans/${props.detail.id}/material-transfers`, { body: bodyFor(chosenPreviewItems.value) }); emit('updated'); emit('close') }
  catch (err) { error.value = err.message || '生成调拨单失败，请重新核对。' } finally { saving.value = false }
}
watch(() => props.source, (source) => { serverSource.value = null; serverWarehouses.value = null; initialiseAdjustments(source) })
onMounted(() => { initialiseAdjustments(props.source); if (props.mode === 'bulk') void loadBulkPreview(); else void loadOptions() })
</script>
<style scoped>
.scroll-hint{display:none}@media(max-width:1000px){.scroll-hint{display:block;position:sticky;left:0;color:#657771;font-size:12px;margin:5px 0}}
.table-scroll input[type="checkbox"]{width:auto;max-width:none;margin-right:6px}
.table-scroll{min-width:0;max-width:100%;overflow-x:auto}.table-scroll table{min-width:940px}.table-scroll th,.table-scroll td,.table-scroll button{white-space:nowrap}
.transfer-overlay{position:fixed;inset:0;background:#182c2766;z-index:1200;display:flex;justify-content:flex-end}.transfer-drawer{width:min(1000px,95vw);height:100%;background:#fff;display:flex;flex-direction:column;box-shadow:-8px 0 24px #0002;color:#172f29}.transfer-drawer header,.transfer-drawer footer{display:flex;justify-content:space-between;align-items:center;gap:16px;padding:18px 24px;border-bottom:1px solid #dfe7e3}.transfer-drawer footer{border-top:1px solid #dfe7e3;border-bottom:0}.transfer-drawer header small,.muted{font-size:12px;color:#657771}.transfer-drawer h2{margin:4px 0 0}.transfer-drawer h3{margin:0 0 12px}.transfer-drawer h4{margin:24px 0 10px}.transfer-drawer main{padding:22px 24px;overflow:auto;flex:1}.close{border:0;background:none;font-size:26px;cursor:pointer}.summary{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px;padding:15px;border-radius:8px;background:#ecf7f1}.summary div{display:grid;gap:5px}.summary span{color:#657771;font-size:12px}.summary strong{font-size:18px}.missing,.error{color:#ad3f30}.error{padding:10px;background:#fff0eb;border-radius:6px;margin-bottom:12px}.field{display:grid;gap:6px;margin:18px 0}.field>span{font-weight:700}select,input{box-sizing:border-box;border:1px solid #cfdad5;border-radius:6px;padding:8px;background:#fff;color:#172f29}select{width:100%}input{max-width:100px;width:100%}.table-scroll{overflow:auto}table{border-collapse:collapse;min-width:650px;width:100%;font-size:13px}th,td{text-align:left;border-bottom:1px solid #e5ece8;padding:10px}th{background:#f2f6f4;color:#657771}td small{display:block}.empty{padding:15px;color:#657771}.hint{background:#ecf7f1;padding:12px;border-radius:7px;margin-top:20px;font-size:13px}.hint p{margin:6px 0 0}.actions{display:flex;gap:8px;flex-wrap:wrap}button:not(.close){border:1px solid #c5d8ce;background:#fff;border-radius:6px;padding:9px 14px;color:#176342;cursor:pointer}button.primary{background:#238653;color:white;border-color:#238653}button:disabled{opacity:.5;cursor:not-allowed}.adjust-row{display:flex;justify-content:space-between;gap:12px;align-items:center;margin:10px 0}.pending{background:#f5f8f6;padding:10px;border-radius:6px}.pending p{margin:6px 0;font-size:12px}@media(max-width:720px){.transfer-drawer{width:100vw}.transfer-drawer header,.transfer-drawer footer,.transfer-drawer main{padding-left:15px;padding-right:15px}.summary{grid-template-columns:1fr 1fr}.transfer-drawer footer{align-items:flex-start;flex-direction:column}}
</style>
