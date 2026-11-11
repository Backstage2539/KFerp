<template>
  <section class="stock-panel" aria-label="可用半成品">
    <div class="stock-heading"><div><h2>可用半成品</h2><p>库存可替代部分生产；选择本身不会占用或移动库存。</p></div><button type="button" :disabled="busy || disabled" @click="open">{{ items.some(item => Number(item.stock_covered_qty || 0) > 0) ? '修改用量' : '查看来源 / 选择使用' }}</button></div>
    <p v-if="disabled" class="hint">请先保存当前修改，再调整半成品使用量。</p>
    <p v-if="loadError" class="error" role="alert">{{ loadError }} <button type="button" @click="load">重试</button></p>
    <div v-if="!loadError && !loading && !items.length" class="hint">本计划暂无可选择的半成品。</div>
    <div v-for="item in loadError ? [] : items" :key="item.key" class="stock-summary">
      <strong class="stock-name">{{ item.name }}</strong>
      <div class="stock-equation">
        <div><span>总需求</span><strong>{{ qty(item.required_qty, item.unit) }}</strong></div><b aria-hidden="true">−</b>
        <div><span>本次使用</span><strong>{{ qty(item.stock_covered_qty, item.unit) }}</strong></div>
        <template v-if="Number(item.inflight_covered_qty || 0) > 0"><b aria-hidden="true">−</b><div><span>在产供应</span><strong>{{ qty(item.inflight_covered_qty, item.unit) }}</strong></div></template>
        <b aria-hidden="true">=</b><div class="manufacture"><span>本次需生产</span><strong>{{ qty(item.manufacture_qty, item.unit) }}</strong></div>
      </div>
      <small>可用库存 {{ qty(available(item), item.unit) }}</small>
      <small v-if="item.legacy && item.stock_covered_qty">原计划已抵扣，批次尚未固定；修改使用量时请选择批次。</small>
    </div>
    <p v-if="allowTransfer && selectedWipReady" class="wip-ready">已在 WIP 仓，可直接使用。</p>
    <div v-if="allowTransfer && selectedTransferSources.length" class="transfer-status">
      <div v-for="source in selectedTransferSources" :key="source.id"><strong>{{ source.component_name }}</strong><span> · 待转入 WIP 仓</span><div v-for="allocation in source.allocations || []" :key="`${allocation.warehouse}-${allocation.owner_customer_id}`"><template v-if="allocation.warehouse !== 'wip'"><span>{{ allocation.warehouse_name || allocation.warehouse }}</span><span v-for="batch in allocation.batches || []" :key="batch.batch_id"> · {{ batch.batch_code }} · {{ sourceAmount(source, batch) }}</span></template></div></div>
      <p class="transfer-hint">转入 WIP 后可继续核对并提交。</p>
      <button type="button" class="transfer-button" :disabled="busy || disabled || loading || !!loadError || !draftToken" @click="transferToWIP">{{ busy ? '正在转入…' : '转入 WIP 仓' }}</button>
    </div>
    <p v-if="allowTransfer && transferError" class="error" role="alert">{{ transferError }}</p>
    <Teleport to="body">
      <div v-if="opened" class="stock-overlay" @click.self="close">
        <section class="stock-drawer" role="dialog" aria-modal="true" aria-label="选择半成品库存" @keydown.esc.stop="close">
          <header><div><h2>选择半成品库存</h2><p>选择本次使用的批次及数量；确认仅保存用量，不会转仓。</p></div><button type="button" :disabled="busy" @click="close">关闭</button></header>
          <div class="drawer-body">
            <p v-if="loadError || actionError" class="error" role="alert">{{ actionError || loadError }} <button v-if="loadError" type="button" :disabled="loading" @click="load">重新读取库存</button></p>
            <p v-if="loading">正在读取库存…</p>
            <p v-else-if="!loadError && !items.length">本计划暂无可选择的半成品。</p>
            <article v-for="item in loadError ? [] : items" :key="item.key" class="item">
              <h3>{{ item.name }}</h3><p>需求 {{ qty(item.required_qty, item.unit) }} · 已抵扣 {{ qty(item.stock_covered_qty, item.unit) }} · 在产 {{ qty(item.inflight_covered_qty, item.unit) }} · 当前需生产 {{ qty(item.manufacture_qty, item.unit) }}</p>
              <p v-if="item.legacy && item.stock_covered_qty" class="hint">原计划已抵扣，批次尚未固定。修改时请重新选择对应批次。</p>
              <p v-if="!item.batches?.length">暂无可用批次</p>
              <div v-for="batch in item.batches || []" :key="stockBatchKey(item, batch)" class="batch">
                <div><strong>{{ batch.batch_code || '未记录批次' }}</strong><p>{{ batch.warehouse_name || batch.warehouse || '未记录仓库' }} · 货主 {{ batch.owner_name || '未记录' }} · 质检 {{ transferQualityLabel(batch.quality_status) }}</p><p>可用 {{ qty(batch.available_qty, item.unit) }}</p><details><summary>查看来源</summary><p>账面 {{ qty(batch.onhand_qty, item.unit) }} · 占用 {{ qty(batch.reserved_qty, item.unit) }}</p><p>入库 {{ batch.received_at || '未记录' }} · 来源工单 {{ batch.source_work_order_no || '未记录' }} · 入库单 {{ batch.source_entry_no || '未记录' }}</p></details></div>
                <label>本次使用（{{ item.unit }}）<input v-model="values[stockBatchKey(item, batch)]" type="number" min="0" step="any" :disabled="busy" /></label>
              </div>
            </article>
            <p v-if="validationError" class="error" role="alert">{{ validationError }}</p>
          </div>
          <footer><button type="button" :disabled="busy" @click="close">取消</button><button type="button" :disabled="busy || loading || !!loadError || !draftToken || !!validationError" @click="commit">确认使用</button></footer>
        </section>
      </div>
    </Teleport>
  </section>
</template>
<script setup>
import { computed, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { initialStockSelection, stockBatchKey, stockSelectionError, stockSelectionPayload } from '../lib/semi-finished-stock'
import { transferQualityLabel } from '../lib/production-plan-transfer'
const props = defineProps({ detail: { type: Object, required: true }, disabled: { type: Boolean, default: false }, allowTransfer: { type: Boolean, default: false } })
const emit = defineEmits(['updated', 'editing'])
const items = ref([]), values = ref({}), loading = ref(false), busy = ref(false), opened = ref(false), loadError = ref(''), actionError = ref(''), transferError = ref(''), draftToken = ref('')
const validationError = computed(() => stockSelectionError(items.value, values.value))
const selectedSources = computed(() => (props.detail?.component_sources || []).filter(source => source.allocation_mode === 'semi_finished_selected'))
const selectedTransferSources = computed(() => selectedSources.value.filter(source => Number(source.transfer_g || 0) > 0 || Number(source.transfer_units || 0) > 0))
const selectedWipReady = computed(() => selectedSources.value.some(source => Number(source.wip_covered_g || 0) > 0 || Number(source.wip_covered_units || 0) > 0) && selectedSources.value.every(source => !Number(source.transfer_g || 0) && !Number(source.transfer_units || 0) && !Number(source.shortage_g || 0) && !Number(source.shortage_units || 0)))
const sourceAmount = (source, batch) => Number(batch.qty_g || 0) > 0 ? qty(String(source.unit || '').toLowerCase() === 'kg' ? Number(batch.qty_g) / 1000 : batch.qty_g, source.unit || 'g') : qty(batch.qty_units, source.unit || '件')
let transferKey = '', transferFingerprint = ''
function transferRequestKey() { const fingerprint = JSON.stringify([Number(props.detail.id), String(props.detail.draft_token || '')]); if (!transferKey || fingerprint !== transferFingerprint) { transferKey = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`; transferFingerprint = fingerprint } return transferKey }
const endpoint = () => `/api/production-plans/${Number(props.detail.id)}/semi-finished-stock`
const qty = (value, unit) => `${Number(value || 0).toLocaleString('zh-CN', { maximumFractionDigits: 6 })} ${unit || ''}`
const available = item => (item.batches || []).reduce((sum, batch) => sum + Number(batch.available_qty || 0), 0)
let loadSerial = 0
async function load() {
  if (!props.detail?.id) return
  const planID = Number(props.detail.id)
  const serial = ++loadSerial
  loading.value = true; loadError.value = ''
  try {
    const data = await apiGet(endpoint())
    if (serial !== loadSerial || Number(props.detail?.id) !== planID) return
    if (!Array.isArray(data?.items) || typeof data.draft_token !== 'string' || !data.draft_token.trim() || data.items.some(item => !item?.key || (item.batches != null && !Array.isArray(item.batches)))) throw new Error('库存数据或计划版本不完整，请重新读取。')
    items.value = data.items; draftToken.value = data.draft_token; values.value = initialStockSelection(items.value)
  }
  catch (err) { if (serial === loadSerial && Number(props.detail?.id) === planID) loadError.value = err.message || '读取半成品库存失败' }
  finally { if (serial === loadSerial) loading.value = false }
}
async function open() { if (props.disabled || busy.value) return; opened.value = true; emit('editing', true); actionError.value = ''; await load() }
function close() { if (busy.value) return; opened.value = false; emit('editing', false); actionError.value = '' }
async function commit() {
  if (validationError.value || busy.value || loading.value || loadError.value || !draftToken.value) return
  busy.value = true; actionError.value = ''
  const planID = Number(props.detail.id), token = draftToken.value, selections = stockSelectionPayload(items.value, values.value)
  try {
    const checked = await apiSend(endpoint(), { body: { draft_token: token, check_only: true, selections } })
    if (typeof checked?.preview_token !== 'string' || !checked.preview_token.trim() || !Array.isArray(checked.changes) || checked.changes.some(change => !Array.isArray(change.operations))) throw new Error('校验结果不完整，请重新确认。')
    if (Number(props.detail.id) !== planID) throw new Error('计划已变化，请重新选择。')
    const result = await apiSend(endpoint(), { body: { draft_token: token, preview_token: checked.preview_token, check_only: false, selections } })
    if (Number(result?.plan?.id) !== planID) throw new Error('保存结果无法确认，请刷新计划核对后再操作。')
    opened.value = false; emit('editing', false); emit('updated', result.plan); await load()
  } catch (err) { actionError.value = (err.message || '保存失败，输入已保留，请重新核对。').replace('请重新预览后确认', '请重新核对后确认使用') }
  finally { busy.value = false }
}
async function transferToWIP() {
  if (!props.allowTransfer || props.disabled || busy.value || loading.value || loadError.value || !draftToken.value || !selectedTransferSources.value.length) return
  const planID = Number(props.detail.id), token = String(props.detail.draft_token || '')
  if (!token) { transferError.value = '计划版本不完整，请刷新后重试。'; return }
  const key = transferRequestKey()
  busy.value = true; transferError.value = ''; emit('editing', true)
  try {
    const result = await apiSend(`${endpoint()}/transfer-to-wip`, { body: { draft_token: token, idempotency_key: key } })
    if (Number(result?.plan?.id) !== planID) throw new Error('转仓结果无法确认，请刷新计划核对后再操作。')
    transferKey = ''; transferFingerprint = ''
    emit('updated', result.plan, 'transfer'); await load()
  } catch (err) { transferError.value = err.message || '转仓失败，请核对后重试。' }
  finally { busy.value = false; emit('editing', false) }
}
watch(() => [props.detail?.id, props.detail?.draft_token], () => { if (!opened.value) void load() }, { immediate: true })
</script>
<style scoped>
.stock-panel{margin:16px 28px;padding:16px 20px;border:1px solid #dce9e2;border-radius:12px;background:#fff}.stock-heading{display:flex;align-items:center;justify-content:space-between;gap:16px}.stock-heading h2,.stock-drawer h2{font-size:17px;margin:0}.stock-heading p,.hint,.stock-drawer p{font-size:13px;color:#596d65}.stock-panel button,.stock-drawer button{border:1px solid #cbd8d2;border-radius:7px;background:#fff;padding:8px 12px;color:#176b46;font-weight:700;cursor:pointer}.stock-panel button:disabled,.stock-drawer button:disabled{opacity:.5;cursor:not-allowed}.stock-summary{padding:14px 0;border-top:1px solid #e6eeea;font-size:13px}.stock-name{display:block;font-size:16px;margin-bottom:10px}.stock-equation{display:flex;align-items:center;gap:22px;flex-wrap:wrap}.stock-equation div{display:grid;gap:4px}.stock-equation span{color:#596d65}.stock-equation strong{font-size:20px}.stock-equation .manufacture strong{color:#16834f;font-size:24px}.stock-summary small{display:block;margin-top:8px;color:#687d73}.wip-ready{color:#16834f;font-weight:700}.error{color:#ad3c29!important}.stock-overlay{position:fixed;inset:0;z-index:1000;background:#12261f80;display:flex;justify-content:flex-end}.stock-drawer{height:100%;width:min(780px,100%);background:#fff;display:flex;flex-direction:column;box-shadow:-8px 0 28px #112e211f}.stock-drawer header,.stock-drawer footer{padding:20px;display:flex;justify-content:space-between;align-items:center;gap:8px;border-bottom:1px solid #dde8e2}.stock-drawer footer{border-top:1px solid #dde8e2;border-bottom:0;justify-content:flex-end}.drawer-body{overflow:auto;flex:1;padding:20px}.item{margin-bottom:18px;border:1px solid #dce9e2;border-radius:10px;padding:16px}.item h3{margin:0}.batch{display:grid;grid-template-columns:1fr 140px;gap:12px;border-top:1px solid #e6eeea;padding:12px 0}.batch p{margin:5px 0}.batch label{font-size:12px;color:#596d65}.batch input{box-sizing:border-box;width:100%;padding:8px;border:1px solid #bdccc4;border-radius:6px}.transfer-status{display:flex;justify-content:space-between;align-items:center;gap:12px;padding:12px 0;border-top:1px solid #e6eeea;color:#5d655f;font-size:13px}.transfer-hint{font-size:12px;color:#596d65}.stock-panel .transfer-button{background:#16834f;color:#fff;border-color:#16834f;white-space:nowrap}@media(max-width:700px){.stock-panel{margin:12px 15px}.stock-heading{align-items:flex-start;flex-direction:column}.batch{grid-template-columns:1fr}.transfer-status{align-items:flex-start;flex-direction:column}.stock-equation{gap:10px}.stock-equation strong{font-size:16px}.stock-equation .manufacture strong{font-size:19px}.stock-drawer footer{flex-wrap:wrap}}
</style>
<style scoped>
.stock-drawer header > button { flex-shrink: 0; }
</style>
