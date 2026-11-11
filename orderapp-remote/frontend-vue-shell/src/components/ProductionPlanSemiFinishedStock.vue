<template>
  <section class="stock-panel" aria-label="可用半成品">
    <div class="stock-heading"><div><h2>可用半成品</h2><p>库存可替代部分生产；选择本身不会占用或移动库存。</p></div><button type="button" :disabled="busy || disabled" @click="open">查看来源 / 选择使用</button></div>
    <p v-if="disabled" class="hint">请先保存当前修改，再调整半成品使用量。</p>
    <p v-if="loadError" class="error" role="alert">{{ loadError }} <button type="button" @click="load">重试</button></p>
    <div v-if="!loadError && !loading && !items.length" class="hint">本计划暂无可选择的半成品。</div>
    <div v-for="item in loadError ? [] : items" :key="item.key" class="stock-summary">
      <strong>{{ item.name }}</strong>
      <span>总需求 {{ qty(item.required_qty, item.unit) }}</span>
      <span>可用库存 {{ qty(available(item), item.unit) }}</span>
      <span>本次使用 {{ qty(item.stock_covered_qty, item.unit) }}</span>
      <span>在产供应 {{ qty(item.inflight_covered_qty, item.unit) }}</span>
      <span>本次需生产 {{ qty(item.manufacture_qty, item.unit) }}</span>
      <small v-if="item.legacy && item.stock_covered_qty">原计划已抵扣，批次尚未固定；修改使用量时请选择批次。</small>
    </div>
    <Teleport to="body">
      <div v-if="opened" class="stock-overlay" @click.self="close">
        <section class="stock-drawer" role="dialog" aria-modal="true" aria-label="选择半成品库存" @keydown.esc.stop="close">
          <header><div><h2>选择半成品库存</h2><p>选择本次使用的批次及数量；保存后仍需仓库确认调拨。</p></div><button type="button" :disabled="busy" @click="close">关闭</button></header>
          <div class="drawer-body">
            <p v-if="loadError || actionError" class="error" role="alert">{{ actionError || loadError }} <button v-if="loadError" type="button" :disabled="loading" @click="load">重新读取库存</button></p>
            <p v-if="loading">正在读取库存…</p>
            <p v-else-if="!loadError && !items.length">本计划暂无可选择的半成品。</p>
            <article v-for="item in loadError ? [] : items" :key="item.key" class="item">
              <h3>{{ item.name }}</h3><p>需求 {{ qty(item.required_qty, item.unit) }} · 已抵扣 {{ qty(item.stock_covered_qty, item.unit) }} · 在产 {{ qty(item.inflight_covered_qty, item.unit) }} · 当前需生产 {{ qty(item.manufacture_qty, item.unit) }}</p>
              <p v-if="item.legacy && item.stock_covered_qty" class="hint">原计划已抵扣，批次尚未固定。修改时请重新选择对应批次。</p>
              <p v-if="!item.batches?.length">暂无可用批次</p>
              <div v-for="batch in item.batches || []" :key="stockBatchKey(item, batch)" class="batch">
                <div><strong>{{ batch.batch_code || '未记录批次' }}</strong><p>{{ batch.warehouse_name || batch.warehouse || '未记录仓库' }} · 货主 {{ batch.owner_name || '未记录' }} · 质检 {{ batch.quality_status || '未记录' }}</p><p>账面 {{ qty(batch.onhand_qty, item.unit) }} · 占用 {{ qty(batch.reserved_qty, item.unit) }} · 可用 {{ qty(batch.available_qty, item.unit) }}</p><p>入库 {{ batch.received_at || '未记录' }} · 来源工单 {{ batch.source_work_order_no || '未记录' }} · 入库单 {{ batch.source_entry_no || '未记录' }}</p></div>
                <label>本次使用（{{ item.unit }}）<input v-model="values[stockBatchKey(item, batch)]" type="number" min="0" step="any" :disabled="busy" @input="preview = null" /></label>
              </div>
            </article>
            <p v-if="validationError" class="error" role="alert">{{ validationError }}</p>
            <section v-if="preview" class="preview" aria-label="调整预览">
              <h3>调整预览</h3><p>确认后更新受影响的生产任务和产能安排，请再次核对工位与批次。</p>
              <p v-if="Array.isArray(preview.changes) && !preview.changes.length">生产数量没有变化。</p>
              <div v-for="(change, index) in preview.changes || []" :key="index">
                <p>{{ change.name }}：{{ qty(change.before_qty, change.unit) }} → {{ qty(change.after_qty, change.unit) }}</p>
                <p>预计投料：{{ change.before_input_qty == null ? '待核对' : qty(change.before_input_qty, change.input_unit) }} → {{ change.after_input_qty == null ? '待核对' : qty(change.after_input_qty, change.input_unit) }}</p>
                <p v-for="(operation, opIndex) in change.operations || []" :key="opIndex">{{ operation.name || '未记录工序' }} · 工位 {{ operation.workstation || '待安排' }} · 批次 {{ operation.batches || '待安排' }} · 预计用时 {{ operation.before_minutes ?? '待配置' }} → {{ operation.after_minutes ?? '待配置' }} 分钟 · 成本 {{ operation.before_cost ?? '待配置' }} → {{ operation.after_cost ?? '待配置' }} 元</p>
              </div>
              <p v-if="!previewComplete" class="error" role="alert">预览信息或校验凭据不完整，暂不能确认，请重新预览。</p>
            </section>
          </div>
          <footer><button type="button" :disabled="busy" @click="close">取消</button><button type="button" :disabled="busy || !!validationError || loading || !!loadError || !draftToken" @click="requestPreview">预览变化</button><button type="button" :disabled="busy || loading || !!loadError || !preview || !previewComplete || !!validationError" @click="commit">确认并更新安排</button></footer>
        </section>
      </div>
    </Teleport>
  </section>
</template>
<script setup>
import { computed, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { initialStockSelection, stockBatchKey, stockSelectionError, stockSelectionPayload } from '../lib/semi-finished-stock'
const props = defineProps({ detail: { type: Object, required: true }, disabled: { type: Boolean, default: false } })
const emit = defineEmits(['updated', 'editing'])
const items = ref([]), values = ref({}), loading = ref(false), busy = ref(false), opened = ref(false), loadError = ref(''), actionError = ref(''), preview = ref(null), draftToken = ref('')
const validationError = computed(() => stockSelectionError(items.value, values.value))
const previewComplete = computed(() => typeof preview.value?.preview_token === 'string' && !!preview.value.preview_token.trim() && Array.isArray(preview.value?.changes) && preview.value.changes.every(change => Array.isArray(change.operations)))
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
async function open() { if (props.disabled) return; opened.value = true; emit('editing', true); preview.value = null; actionError.value = ''; await load() }
function close() { if (busy.value) return; opened.value = false; emit('editing', false); actionError.value = ''; preview.value = null }
async function requestPreview() {
  if (validationError.value || busy.value || loading.value || loadError.value || !draftToken.value) return
  busy.value = true; actionError.value = ''
  try { preview.value = await apiSend(endpoint(), { body: { draft_token: draftToken.value, check_only: true, selections: stockSelectionPayload(items.value, values.value) } }) }
  catch (err) { actionError.value = err.message || '预览失败，请重新核对库存。' }
  finally { busy.value = false }
}
async function commit() {
  if (!preview.value || !previewComplete.value || validationError.value || busy.value || loading.value || loadError.value || !draftToken.value) return
  busy.value = true; actionError.value = ''
  try {
    const result = await apiSend(endpoint(), { body: { draft_token: draftToken.value, preview_token: preview.value.preview_token, check_only: false, selections: stockSelectionPayload(items.value, values.value) } })
    if (Number(result?.plan?.id) !== Number(props.detail.id)) throw new Error('保存结果无法确认，请刷新计划核对后再操作。')
    opened.value = false; emit('editing', false); emit('updated', result.plan); await load()
  } catch (err) { preview.value = null; actionError.value = err.message || '保存失败，输入已保留，请重新核对。' }
  finally { busy.value = false }
}
watch(() => [props.detail?.id, props.detail?.draft_token], () => { if (!opened.value) void load() }, { immediate: true })
</script>
<style scoped>
.stock-panel{margin:16px 28px;padding:16px 20px;border:1px solid #dce9e2;border-radius:12px;background:#fff}.stock-heading{display:flex;align-items:center;justify-content:space-between;gap:16px}.stock-heading h2,.stock-drawer h2{font-size:17px;margin:0}.stock-heading p,.hint,.stock-drawer p{font-size:13px;color:#596d65}.stock-panel button,.stock-drawer button{border:1px solid #cbd8d2;border-radius:7px;background:#fff;padding:8px 12px;color:#176b46;font-weight:700;cursor:pointer}.stock-panel button:disabled,.stock-drawer button:disabled{opacity:.5;cursor:not-allowed}.stock-summary{display:flex;align-items:center;gap:12px;flex-wrap:wrap;padding:10px 0;border-top:1px solid #e6eeea;font-size:13px}.stock-summary small{flex-basis:100%;color:#9a5b0a}.error{color:#ad3c29!important}.stock-overlay{position:fixed;inset:0;z-index:1000;background:#12261f80;display:flex;justify-content:flex-end}.stock-drawer{height:100%;width:min(780px,100%);background:#fff;display:flex;flex-direction:column;box-shadow:-8px 0 28px #112e211f}.stock-drawer header,.stock-drawer footer{padding:20px;display:flex;justify-content:space-between;align-items:center;gap:8px;border-bottom:1px solid #dde8e2}.stock-drawer footer{border-top:1px solid #dde8e2;border-bottom:0;justify-content:flex-end}.drawer-body{overflow:auto;flex:1;padding:20px}.item{margin-bottom:18px;border:1px solid #dce9e2;border-radius:10px;padding:16px}.item h3{margin:0}.batch{display:grid;grid-template-columns:1fr 140px;gap:12px;border-top:1px solid #e6eeea;padding:12px 0}.batch p{margin:5px 0}.batch label{font-size:12px;color:#596d65}.batch input{box-sizing:border-box;width:100%;padding:8px;border:1px solid #bdccc4;border-radius:6px}.preview{background:#edf8f2;padding:14px;border-radius:8px}.preview h3{margin:0}@media(max-width:700px){.stock-panel{margin:12px 15px}.stock-heading{align-items:flex-start;flex-direction:column}.batch{grid-template-columns:1fr}.stock-drawer footer{flex-wrap:wrap}}
</style>
<style scoped>
.stock-drawer header > button { flex-shrink: 0; }
</style>
