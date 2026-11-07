<script setup>
import { computed, reactive, ref, watch, toRaw, onBeforeUnmount, nextTick } from 'vue'
import { apiGet, apiSend } from '../api/client'
import OrderResolutionPolicy from './OrderResolutionPolicy.vue'
import OrderResolutionBalance from './OrderResolutionBalance.vue'
import OrderResolutionStep from './OrderResolutionStep.vue'
import { blankResolution, resolutionPayload, resolutionStatus, resolutionDependencyStatus } from '../lib/order-resolution'
const props = defineProps({ orderId: { type: Number, required: true } })
const emit = defineEmits(['changed'])
const open = ref(false), busy = ref(false), error = ref(''), notice = ref(''), loaded = ref(null), preview = ref(null)
const form = reactive(blankResolution())
const withdrawalNotes = reactive({})
const drawer = ref(null)
let previousFocus = null, previousOverflow = null
function restorePage() {
 if (previousOverflow !== null) { document.body.style.overflow = previousOverflow; previousOverflow = null }
 if (previousFocus?.isConnected) previousFocus.focus()
 previousFocus = null
}
function trapFocus(event) {
 if (event.key !== 'Tab') return
 const fields = Array.from(drawer.value?.querySelectorAll('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled),a[href],[tabindex="0"]') || []).filter(v => v.getClientRects().length)
 if (!fields.length) { event.preventDefault(); drawer.value?.focus(); return }
 if (event.shiftKey && (document.activeElement === fields[0] || document.activeElement === drawer.value)) { event.preventDefault(); fields.at(-1).focus() }
 else if (!event.shiftKey && (document.activeElement === fields.at(-1) || document.activeElement === drawer.value)) { event.preventDefault(); fields[0].focus() }
}
let generation = 0
let key = ''
const current = computed(() => loaded.value?.context)
const options = computed(() => loaded.value?.options || {})
const pending = computed(() => loaded.value?.history?.find(v => v.status === 'pending'))
const editable = computed(() => Boolean(loaded.value?.permissions?.['orders.resolve']) && !pending.value?.applied)
const simple = computed(() => ['hold', 'resume'].includes(form.action))
const liveDependencies = computed(() => (current.value?.dependencies || []).filter(v => !['cancelled', 'released'].includes(v.status)))
watch(form, () => { preview.value = null; key = '' }, { deep: true, flush: 'sync' })
watch(() => props.orderId, () => { generation++; open.value = false; loaded.value = null; restorePage() })
onBeforeUnmount(() => { generation++; restorePage() })
async function load(restore = false) {
 const g = ++generation; busy.value = true; error.value = ''
 try {
  const data = await apiGet(`/api/orders/${props.orderId}/resolution`)
  if (g !== generation) return
  loaded.value = data
  preview.value = null; key = ''
  if (restore) {
   Object.assign(form, blankResolution(), structuredClone(pending.value && !pending.value.applied ? toRaw(pending.value.command) : data.draft?.action ? data.draft : {}))
   for (const d of form.dispositions) if(d.amount_specified) d.amount=(Number(d.amount_cents)/100).toFixed(2)
   if (!form.lines.length) form.lines = data.context.lines.map(v => ({ item_id: v.id, quantity: 0 }))
   for (const v of data.context.dependencies.filter(v => !['cancelled', 'released'].includes(v.status))) {
    if (!form.dispositions.some(d => d.kind === v.kind && Number(d.id) === Number(v.id))) form.dispositions.push({ kind: v.kind, id: v.id, choice: '', target_id: 0, amount_cents: 0, reference: '' })
   }
  }
 } catch (e) { if (g === generation) error.value = e.message }
 finally { if (g === generation) busy.value = false }
}
async function show() { previousFocus = document.activeElement; previousOverflow = document.body.style.overflow; document.body.style.overflow = 'hidden'; open.value = true; notice.value = ''; preview.value = null; await nextTick(); drawer.value?.focus(); load(true) }
function close() { if (busy.value) return; generation++; open.value = false; restorePage() }
function disposition(dep) { return form.dispositions.find(v => v.kind === dep.kind && Number(v.id) === Number(dep.id)) }
function choices(dep) {
 return (options.value[dep.kind] || []).map(v => {
  let reason = v.disabled ? v.reason : ''
  if (!reason && dep.kind==='production' && dep.ownership_issue && ['keep_company_stock','continue_company'].includes(v.value)) reason=dep.ownership_issue
  if (!reason && dep.kind === 'production' && dep.status === 'completed' && !['keep_company_stock','allocate','scrap','defer'].includes(v.value)) reason='已完成生产须保留'
  if (!reason && dep.shared && ['cancel_unstarted','stop'].includes(v.value)) reason='共享生产不能影响其他订单'
  if (!reason && dep.kind==='production' && dep.status!=='completed' && ['keep_company_stock','allocate','scrap'].includes(v.value)) reason='请先明确未完成生产的处置'
  if (!reason && dep.kind==='fee' && dep.id>0 && dep.paid_cents>0 && !['keep','defer'].includes(v.value)) reason='该费用已有收款，须先核对费用退款'
  if (!reason && dep.kind==='direct_ship' && dep.relation==='package' && v.value!=='cancel') reason='当前包裹须整单处理，其他包裹分别选择'
  return {...v,disabled:Boolean(v.disabled || reason),reason}
 })
}
function body() { return resolutionPayload(toRaw(form), current.value?.version || '', key) }
async function saveDraft() { busy.value = true; error.value = ''; try { await apiSend(`/api/orders/${props.orderId}/resolution/draft`, { body: body() }); notice.value = '处理草稿已保存，订单尚未变更' } catch (e) { error.value = e.message } finally { busy.value = false } }
async function review() { busy.value = true; error.value = ''; try { const out = await apiSend(`/api/orders/${props.orderId}/resolution/preview`, { body: body() }); preview.value = out; key = crypto.randomUUID() } catch (e) { preview.value = null; error.value = e.message } finally { busy.value = false } }
async function submit() {
 if (!preview.value || busy.value || !editable.value) return
 busy.value = true; error.value = ''
 try { const out = await apiSend(pending.value ? `/api/orders/${props.orderId}/resolution/${pending.value.id}/refresh` : `/api/orders/${props.orderId}/resolution/submit`, { body: { ...body(), version: preview.value.version } }); notice.value = out.status === 'completed' ? '订单处理已完成' : '处理单已提交，请完成下方待办；未登记实际退款的金额不会标为已退款'; preview.value = null; emit('changed'); await load() }
 catch (e) { error.value = e.message; if (e.status === 409) preview.value = null }
 finally { busy.value = false }
}
async function withdraw(record) {
 busy.value = true; error.value = ''
 try { await apiSend(`/api/orders/${props.orderId}/resolution/${record.id}/withdraw`, { body: { note: withdrawalNotes[record.id] || '' } }); notice.value = '处理单已撤回'; emit('changed'); await load() }
 catch (e) { error.value = e.message } finally { busy.value = false }
}
async function stepChanged() { emit('changed'); notice.value = '本项处置已保存，请核对剩余待办'; await load() }
function relatedOrder(order) {
 close()
 window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'orders', params: { order_id: order.id }, returnNavigation: { key: 'orders', label: '返回原订单', params: { order_id: props.orderId } } } }))
}
function revise(record) {
 close()
 window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'order', params: { copy_id: props.orderId, amendment_id: record.amendment_id }, returnNavigation: { key: 'orders', label: '返回原订单', params: { order_id: props.orderId } } } }))
}
const money = cents => (Number(cents || 0) / 100).toFixed(2)
</script>
<template>
 <button type="button" class="resolution-entry" @click.stop="show">订单处理</button>
 <Teleport to="body">
  <div v-if="open" class="resolution-backdrop" @keydown.esc="close">
   <section ref="drawer" class="resolution-drawer" @keydown="trapFocus" role="dialog" aria-modal="true" aria-label="订单处理" tabindex="-1">
    <header><div><h2>订单处理</h2><span>{{ current?.order_no || '加载中…' }}</span><strong v-if="current">{{ resolutionStatus(current.status, current.paused) }}</strong></div><button type="button" :disabled="busy" @click="close">关闭</button></header>
    <p class="hint">先选择处理方式，再确认关联单据和影响。已完成的生产、库存与收款记录保留追溯。</p>
    <p v-if="error" class="error" role="alert">{{ error }}</p><p v-if="notice" class="notice" role="status">{{ notice }}</p>
    <div v-if="current" class="resolution-content">
     <OrderResolutionPolicy v-if="loaded.permissions?.['settings.write']" @changed="load()" />
     <p v-if="pending" class="hint">{{ pending.applied ? '订单变更已执行，请继续完成清算待办。' : '本订单已有处理中方案；刷新核对后可更新方案，已执行记录保留。' }}</p>
     <fieldset :disabled="busy || !editable">
      <legend>处理方式</legend>
      <div class="fields">
       <label>操作<select v-model="form.action" :disabled="Boolean(pending)"><option value="">请选择</option><option v-for="v in options.action" :key="v.value" :value="v.value" :disabled="v.disabled">{{ v.label }}{{ v.disabled ? `（${v.reason}）` : '' }}</option></select></label>
       <label>原因<select v-model="form.reason"><option value="">请选择</option><option v-for="v in options.reason" :key="v.value" :value="v.value">{{ v.label }}</option></select></label>
       <label class="wide">说明<textarea v-model="form.note" rows="2" placeholder="填写客户要求或需要更正的内容" /></label>
      </div>
     </fieldset>
     <fieldset v-if="!simple" :disabled="busy || !editable">
      <legend>商品范围</legend>
      <label>范围<select v-model="form.scope"><option value="">请选择</option><option v-for="v in options.scope" :key="v.value" :value="v.value">{{ v.label }}</option></select></label>
      <div class="table-scroll"><table><thead><tr><th>商品</th><th>原数量</th><th>已交付／入客户仓</th><th>已关闭</th><th v-if="form.scope === 'selected'">本次处理</th></tr></thead><tbody><tr v-for="line in current.lines" :key="line.id"><td>{{ line.name }}<small>{{ line.unit }}</small></td><td>{{ line.quantity }}</td><td>{{ line.delivered }}</td><td>{{ line.closed }}</td><td v-if="form.scope === 'selected'"><input v-if="form.lines.find(v => v.item_id === line.id)" v-model.number="form.lines.find(v => v.item_id === line.id).quantity" type="number" min="0" step="any" :aria-label="`${line.name}处理数量`" /></td></tr></tbody></table></div>
     </fieldset>
     <fieldset v-if="!simple && current.dependencies.length" :disabled="busy || !editable">
      <legend>关联单据与处置</legend>
      <div v-for="dep in current.dependencies" :key="`${dep.kind}-${dep.id}`" class="dependency">
       <div><strong>{{ dep.label }}{{ dep.kind === 'fee' ? ` · ¥${money(dep.amount_cents)}` : '' }}</strong><span>{{ resolutionDependencyStatus(dep.status) }}{{ dep.shared ? ' · 共享生产' : '' }}</span></div>
       <p v-if="dep.orders?.length" class="related-orders"><button v-for="order in dep.orders" :key="order.id" type="button" @click="relatedOrder(order)">{{ order.number || `订单 #${order.id}` }} · {{ resolutionDependencyStatus(order.status) }}{{ Number(order.id) === Number(orderId) ? '（当前订单）' : '' }}</button></p>
       <span v-if="['cancelled', 'released'].includes(dep.status)" class="hint">已取消或已释放，保留历史记录</span>
       <template v-else-if="disposition(dep)"><select v-model="disposition(dep).choice" :aria-label="`${dep.label}处置`"><option value="">请选择处置</option><option v-for="v in choices(dep)" :key="v.value" :value="v.value" :disabled="v.disabled">{{ v.label }}{{ v.disabled ? `（${v.reason}）` : '' }}</option></select><label v-if="dep.kind === 'fee' && ['partial_waiver','negotiated'].includes(disposition(dep).choice)">{{ disposition(dep).choice === 'negotiated' ? '协商后保留收费（元）' : '本次减免金额（元）' }}<input v-model="disposition(dep).amount" inputmode="decimal" placeholder="请明确填写金额" /></label><label v-if="dep.kind === 'stock_reservation' && disposition(dep).choice === 'release'">本批释放数量（{{ dep.unit }}）<input v-model.number="disposition(dep).quantity" type="number" min="0" :max="dep.quantity" step="any" placeholder="明确填写本批释放数量" /></label><p v-if="['replace_supply','allocate','reassign'].includes(disposition(dep).choice)" class="hint">提交前须选择具体目标并核验可用数量。</p></template>
      </div>
     </fieldset>
     <fieldset v-if="!simple" :disabled="busy || !editable">
      <legend>账款与处理时点</legend>
      <p>已收款 ¥{{ money(current.paid_cents) }}。关闭不会自动退款。</p>
      <div class="fields"><label v-for="[name,label] in [['timing','处理时点'],['external_status','开票及外部入账'],['posting_mode','账期处理']]" :key="name">{{ label }}<select v-model="form[name]"><option value="">请选择</option><option v-for="v in options[name]" :key="v.value" :value="v.value" :disabled="v.disabled">{{ v.label }}{{ v.disabled ? `（${v.reason}）` : '' }}</option></select></label><label>记账日期<input v-model="form.posting_date" type="date" /></label></div>
     </fieldset>
     <div class="actions"><button v-if="pending && !pending.applied" type="button" :disabled="busy" @click="load(true)">刷新当前方案</button><button type="button" :disabled="busy || !editable" @click="saveDraft">保存处理草稿</button><button type="button" class="primary" :disabled="busy || !editable || !form.action" @click="review">预览处理影响</button></div>
     <section v-if="preview" class="preview"><h3>确认本次处理</h3><p>处理后：{{ resolutionStatus(preview.plan.status, form.action === 'hold') }}；应收冲减 ¥{{ money(preview.plan.credit_cents) }}；实际退款需另行确认。</p><ul v-if="preview.plan.tasks.length"><li v-for="(task,i) in preview.plan.tasks" :key="i">待处理：{{ task.label }}</li></ul><button type="button" class="primary" :disabled="busy" @click="submit">{{ pending ? '确认更新处理方案' : '确认提交处理单' }}</button></section>
     <OrderResolutionBalance :order-id="orderId" :balances="loaded.balances || []" :history="loaded.balance_history || []" :targets="loaded.balance_targets || []" :permissions="loaded.permissions" :options="options" @changed="stepChanged" />
     <section v-if="loaded.history?.length"><h3>处理记录</h3><article v-for="r in loaded.history" :key="r.id" class="history"><strong>处理单 #{{ r.id }} · {{ ({pending:'处理中',completed:'已完成',withdrawn:'已撤回'})[r.status] || r.status }}</strong><p>{{ options.action.find(v => v.value === r.command.action)?.label }} · {{ options.reason.find(v => v.value === r.command.reason)?.label }} · {{ r.command.note }}</p><p v-if="r.applied && r.status === 'pending'" class="hint">履约已按本次方案处理，以下清算待办尚未全部完成。</p>
      <OrderResolutionStep v-for="step in r.steps" :key="step.id" :order-id="orderId" :request-id="r.id" :step="step" :options="options" :permissions="loaded.permissions" :targets="loaded.targets || []" @changed="stepChanged" @refresh="load()" />
      <div v-if="r.can_withdraw && loaded.permissions?.['orders.resolve']" class="withdraw"><label>撤回原因<input v-model="withdrawalNotes[r.id]" placeholder="仅在尚未执行处置时可撤回" /></label><button type="button" :disabled="busy || !withdrawalNotes[r.id]?.trim()" @click="withdraw(r)">撤回处理单</button></div>
      <button v-if="r.amendment_id && !r.revised_order_id" type="button" :disabled="busy" @click="revise(r)">打开关联修订草稿</button><p v-if="r.revised_order_id" class="hint">已生成修订单 #{{ r.revised_order_id }}，新单单独审核。</p></article></section>
    </div>
   </section>
  </div>
 </Teleport>
</template>
<style scoped>
.resolution-entry{border:1px solid #c5d5cd;background:#fff;color:#285e49;border-radius:6px;padding:6px 10px;cursor:pointer}.resolution-backdrop{position:fixed;inset:0;z-index:1200;background:#10251d66;display:flex;justify-content:flex-end}.resolution-drawer{width:min(820px,100vw);height:100%;box-sizing:border-box;overflow:auto;background:#f7f9f7;padding:24px;color:#20372c}.resolution-drawer header{position:sticky;top:-24px;z-index:2;background:#f7f9f7;padding:16px 0;display:flex;justify-content:space-between;align-items:flex-start;gap:16px}.resolution-drawer header strong{display:block;margin-top:6px}.resolution-drawer h2{margin:0 0 8px}.resolution-content{display:grid;gap:18px}.resolution-drawer fieldset{border:1px solid #d6e0d9;background:white;border-radius:10px;padding:16px;min-width:0}.resolution-drawer legend{font-weight:700}.fields{display:grid;grid-template-columns:1fr 1fr;gap:14px}.fields .wide{grid-column:1/-1}.resolution-drawer label{display:flex;flex-direction:column;gap:7px}.resolution-drawer input,.resolution-drawer select,.resolution-drawer textarea{box-sizing:border-box;max-width:100%;min-width:0;padding:9px;border:1px solid #bccdc1;border-radius:6px;background:#fff;color:inherit;font:inherit}.resolution-drawer button{padding:9px 14px;border:1px solid #b8cbbf;border-radius:6px;background:#fff;color:inherit;cursor:pointer}.resolution-drawer .primary{background:#286449;color:white}.resolution-drawer button:disabled{opacity:.5;cursor:default}.hint,small{color:#62776a;font-size:13px}.error{color:#ab3026;background:#fff0ed;padding:12px}.notice{background:#e6f3e9;padding:12px}.table-scroll{overflow:auto}table{width:100%;border-collapse:collapse;margin-top:12px}td,th{text-align:left;padding:9px;border-bottom:1px solid #e2e8e3;white-space:normal}td input{width:100px}small{display:block}.dependency{padding:12px 0;border-bottom:1px solid #e5ebe6}.dependency>div{display:flex;gap:12px;margin-bottom:8px}.dependency select{width:100%}.related-orders{display:flex;gap:10px;flex-wrap:wrap}.related-orders a{color:#285e49;font-size:13px}.actions{display:flex;justify-content:flex-end;gap:12px}.preview{border:2px solid #3f8060;padding:16px;border-radius:10px;background:#fff}.history{padding:12px;border-bottom:1px solid #d5e0d8}@media(max-width:600px){.fields{grid-template-columns:1fr}.resolution-drawer{padding:16px}.resolution-drawer header{top:-16px}.actions{flex-wrap:wrap}}
</style>
