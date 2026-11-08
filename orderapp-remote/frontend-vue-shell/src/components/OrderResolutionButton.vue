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
const returnStockStep = computed(() => pending.value?.steps?.find(s=>s.kind==='return_stock' && s.status==='completed'))
const returnStockPosted = computed(() => Boolean(returnStockStep.value))
const returnStockLabel = computed(() => returnStockStep.value?.label || '')
const editable = computed(() => Boolean(loaded.value?.permissions?.['orders.resolve']) && !pending.value?.applied)
const simple = computed(() => ['hold', 'resume'].includes(form.action))
const feesOnly = computed(() => form.action === 'credit' && form.scope === 'fees_only')
const visibleDependencies = computed(() => (current.value?.dependencies || []).filter(v => !feesOnly.value || v.kind === 'fee'))
const scopeOptions = computed(() => (options.value.scope || []).filter(v=>v.value !== 'fees_only' || form.action === 'credit'))
const timingOptions = computed(() => feesOnly.value ? (options.value.timing || []).filter(v=>v.value==='settle_first').map(v=>({...v,label:'完成费用确认和资金清算后执行'})) : options.value.timing || [])
watch(() => form.action, () => { if(form.action!=='credit' && form.scope==='fees_only') form.scope='' })
watch(feesOnly, value => { if(value && form.timing==='stop_first') form.timing='' })
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
   for (const v of data.context.dependencies.filter(v => !['cancelled', 'released', 'stopped'].includes(v.status))) {
    if (!form.dispositions.some(d => d.kind === v.kind && Number(d.id) === Number(v.id))) form.dispositions.push({ kind: v.kind, id: v.id, choice: '', target_id: 0, amount_cents: 0, reference: '' })
    const d=disposition(v)
    if(v.kind==='production') {d.stock_selections ||= [];d.cost_treatment ||= ''}
    if(v.stop_work_orders?.length)d.stops=v.stop_work_orders.map(w=>d.stops?.find(s=>Number(s.work_order_id)===Number(w.work_order_id))||{work_order_id:w.work_order_id,materials:'',wip:'',reservations:'',outputs:''})
    if(v.supply_reservations?.length){
     d.supplies=v.supply_reservations.map(s=>{
      const prior=d.supplies?.find(choice=>Number(choice.reservation_id)===Number(s.id))
      const choice=prior?{...prior,quantity:prior.quantity || ''}:{reservation_id:s.id,target_id:0,quantity:''}
      if(s.converted_quantity>0){choice.converted_quantity=s.converted_quantity;choice.physical_choice=prior?.physical_version===s.physical_version ? prior.physical_choice || '' : '';choice.physical_version=s.physical_version}
      else {delete choice.converted_quantity;delete choice.physical_choice;delete choice.physical_version}
      if(s.quantity===0&&s.converted_quantity>0){choice.converted_only=true;choice.target_id=0;choice.quantity=''}else delete choice.converted_only
      return choice
     })
    }
   }
  }
 } catch (e) { if (g === generation) error.value = e.message }
 finally { if (g === generation) busy.value = false }
}
async function show() { previousFocus = document.activeElement; previousOverflow = document.body.style.overflow; document.body.style.overflow = 'hidden'; open.value = true; notice.value = ''; preview.value = null; await nextTick(); drawer.value?.focus(); load(true) }
function close() { if (busy.value) return; generation++; open.value = false; restorePage() }
function disposition(dep) { return form.dispositions.find(v => v.kind === dep.kind && Number(v.id) === Number(dep.id)) }
function supplyChoice(dep,source) { return disposition(dep)?.supplies?.find(v=>Number(v.reservation_id)===Number(source.id)) }
function stockSource(dep,selection) { return dep.company_stock?.find(v=>Number(v.batch_id)===Number(selection.batch_id)) }
function stopSelection(dep,work) { return disposition(dep)?.stops?.find(s=>Number(s.work_order_id)===Number(work.work_order_id)) }
function addStockSelection(dep) { disposition(dep).stock_selections.push({batch_id:0,target_item_id:0,quantity:''}) }
function addReturnSelection() { if(returnStockPosted.value) return; form.return_selections.push({source_kind:'',source_id:0,batch_id:0,quantity:'',ownership:'',warehouse:'',quality:''}) }
function returnSource(row) { return current.value?.return_sources?.find(s=>s.kind===row.source_kind&&Number(s.id)===Number(row.source_id)&&Number(s.batch_id)===Number(row.batch_id)) }
function chooseReturnSource(row,key) {
 if(returnStockPosted.value) return
 const source=current.value?.return_sources?.find(s=>`${s.kind}:${s.id}`===key)
 Object.assign(row,{source_kind:source?.kind||'',source_id:source?.id||0,batch_id:source?.batch_id||0,quantity:'',ownership:'',warehouse:'',quality:''})
}
function returnOwners(row) { return (options.value.return_ownership||[]).filter(v=>returnSource(row)?.destinations?.some(d=>d.ownership===v.value)) }
function choices(dep) {
 return (options.value[dep.kind] || []).map(v => {
  let reason = v.disabled ? v.reason : ''
  if (!reason && dep.kind==='customer_stock' && v.value==='ownership_return' && form.action!=='return') reason='请选择退货，再明确货权退回数量'
  if (!reason && dep.kind==='customer_stock' && v.value==='ownership_return' && !current.value?.return_sources?.some(s=>s.kind==='ownership'&&Number(s.entitlement_id)===Number(dep.id)&&s.available_quantity>0)) reason='没有该笔权益可核对的原收货批次和未预留实物'
  if (!reason && dep.kind==='fee' && dep.amount_cents<0 && !['keep','defer'].includes(v.value)) reason='该项是已有负数费用调整，请保留原记录'
  if (!reason && dep.kind==='fee' && dep.cash_issue && !['keep','defer'].includes(v.value)) reason=dep.cash_issue
  if (!reason && dep.kind==='production' && v.value==='stop') reason=dep.shared?'共享生产须先拆分剩余需求':dep.stop_work_orders?.find(w=>w.issue)?.issue||(!dep.stop_work_orders?.length?'尚无可核对的工单余料与在制品':form.action!=='close'||form.scope!=='all'?'停止整批生产须选择关闭全部剩余履约':'')
  if (!reason && dep.kind==='production' && v.value==='scrap' && (dep.ownership_issue || form.action!=='close' || !dep.company_stock?.some(s=>s.available_quantity>0&&s.cost_record_id>0&&!s.scrap_issue))) reason=dep.ownership_issue || (form.action!=='close'?'报损请关闭剩余履约，保留原生产历史':dep.company_stock?.find(s=>s.scrap_issue)?.scrap_issue || '没有可核对原批次、公司货权和原生产成本的可用成品')
  if (!reason && dep.kind==='production' && v.value==='allocate' && (dep.ownership_issue || !dep.company_stock?.some(s=>s.available_quantity>0&&s.targets?.length))) reason=dep.ownership_issue || '尚无可核对的公司成品与匹配目标；请核对原批次、预留、库存单位及仓库'
  if (!reason && dep.kind==='production' && dep.ownership_issue && ['keep_company_stock','continue_company'].includes(v.value)) reason=dep.ownership_issue
  if (!reason && dep.kind === 'production' && dep.status === 'completed' && !['keep_company_stock','allocate','scrap','defer'].includes(v.value)) reason='已完成生产须保留'
  if (!reason && dep.shared && ['cancel_unstarted','stop'].includes(v.value)) reason='共享生产不能影响其他订单'
  if (!reason && dep.kind==='production' && dep.status!=='completed' && ['keep_company_stock','allocate','scrap'].includes(v.value)) reason='请先明确未完成生产的处置'
  if (!reason && dep.kind==='direct_ship' && v.value==='keep_fulfilled' && (form.action!=='return'||dep.relation!=='package')) reason='仅用于本包裹实际退货，保留原发货历史'
  if (!reason && dep.kind==='direct_ship' && dep.relation==='package' && (form.action==='return' ? !['keep_fulfilled','defer'].includes(v.value) : v.value!=='cancel')) reason=form.action==='return'?'退货须保留已发生的原包裹发货记录':'当前包裹须整单处理，其他包裹分别选择'
  if (!reason && dep.kind==='direct_ship' && v.value==='replace_supply' && dep.replacement_issue) reason=dep.replacement_issue
  if (!reason && dep.kind==='direct_ship' && v.value==='replace_supply' && !dep.supply_reservations?.some(s=>s.quantity>0)) reason='没有可切换的待入库预订'
  if (!reason && v.value==='replace_supply' && dep.supply_reservations?.some(s=>s.quantity>0&&!s.targets?.length)) reason='尚无客户、商品、冻结规格和仓库均匹配的替代货源'
  return {...v,disabled:Boolean(v.disabled || reason),reason}
 })
}
function body(draft=false) { return resolutionPayload(toRaw(form), current.value?.version || '', key, {draft}) }
async function saveDraft() { busy.value = true; error.value = ''; try { await apiSend(`/api/orders/${props.orderId}/resolution/draft`, { body: body(true) }); notice.value = '处理草稿已保存，订单尚未变更' } catch (e) { error.value = e.message } finally { busy.value = false } }
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
function relatedWork(work) {
 close()
 window.dispatchEvent(new CustomEvent('kferp:navigate-view',{detail:{key:'workOrders',params:{work_order_id:work.work_order_id},returnNavigation:{key:'orders',label:'返回原订单',params:{order_id:props.orderId}}}}))
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
      <label>范围<select v-model="form.scope" :disabled="returnStockPosted"><option value="">请选择</option><option v-for="v in scopeOptions" :key="v.value" :value="v.value">{{ v.label }}</option></select></label>
      <div class="table-scroll"><table><thead><tr><th>商品</th><th>原数量</th><th>已交付／入客户仓</th><th>已关闭</th><th>已退回</th><th v-if="form.scope === 'selected'">本次处理</th></tr></thead><tbody><tr v-for="line in current.lines" :key="line.id"><td>{{ line.name }}<small>{{ line.unit }}</small></td><td>{{ line.quantity }}</td><td>{{ line.delivered }}</td><td>{{ line.closed }}</td><td>{{ line.returned || 0 }}</td><td v-if="form.scope === 'selected'"><input v-if="form.lines.find(v => v.item_id === line.id)" v-model.number="form.lines.find(v => v.item_id === line.id).quantity" :disabled="returnStockPosted" type="number" min="0" step="any" :aria-label="`${line.name}处理数量`" /></td></tr></tbody></table></div>
     </fieldset>
     <fieldset v-if="form.action==='return'" :disabled="busy || !editable">
      <legend>实际退回来源与商品款</legend>
      <p class="hint">{{ current.delivery_mode === 'customer_stock' ? '按原客户收货记录核对货权退回；逐批选择公司接收仓与数量，原收货及共享生产记录保留。' : '按原出库记录逐批核对，保留原发货与客户货权。' }}实物入库、商品款冲减和实际退款分别确认。</p>
      <label>退回商品款处理<select v-model="form.return_settlement" :disabled="returnStockPosted"><option value="">请选择</option><option v-for="v in options.return_finance" :key="v.value" :value="v.value" :disabled="v.disabled">{{ v.label }}{{ v.disabled && v.reason ? `（${v.reason}）` : '' }}</option></select></label>
      <p v-if="returnStockPosted" class="hint">已验收入库：{{ returnStockLabel }}。实物记录已保存，请继续完成财务待办；已收回的批次与数量不能改选。</p>
      <template v-else>
      <div v-for="(row,index) in form.return_selections" :key="index" class="return-selection">
       <div class="fields">
        <label class="wide">原出库／退回来源<select :value="`${row.source_kind}:${row.source_id}`" @change="chooseReturnSource(row,$event.target.value)"><option value=":0">请选择原批次</option><option v-for="source in current.return_sources" :key="`${source.kind}:${source.id}`" :value="`${source.kind}:${source.id}`">{{ source.label }} · 可退 {{ source.available_quantity }} {{ source.unit }}</option></select></label>
        <label>本批退回数量<input v-model="row.quantity" type="number" min="1" step="1" placeholder="填写实际数量" /></label>
        <label>货权处置<select v-model="row.ownership" @change="row.warehouse='' "><option value="">请选择货权</option><option v-for="v in returnOwners(row)" :key="v.value" :value="v.value" :disabled="v.disabled">{{ v.label }}{{ v.disabled && v.reason ? `（${v.reason}）` : '' }}</option></select></label>
        <label>退回仓库<select v-model="row.warehouse"><option value="">请选择仓库</option><option v-for="d in returnSource(row)?.destinations?.filter(d=>d.ownership===row.ownership)||[]" :key="d.warehouse" :value="d.warehouse">{{ d.label }}</option></select></label>
        <label>验收后库存处置<select v-model="row.quality"><option value="">请选择验收处置</option><option v-for="v in options.return_quality" :key="v.value" :value="v.value" :disabled="v.disabled">{{ v.label }}{{ v.disabled && v.reason ? `（${v.reason}）` : '' }}</option></select></label>
       </div>
       <p v-if="row.source_id && !returnSource(row)" class="error">原来源已变化或已退回，请重新核对当前处理单与原入库记录。</p>
       <button type="button" @click="form.return_selections.splice(index,1)">移除此批</button>
      </div>
      <button type="button" @click="addReturnSelection">添加退回批次</button>
     </template>
     </fieldset>
     <fieldset v-if="!simple && current.dependencies.length" :disabled="busy || !editable">
      <legend>关联单据与处置</legend>
      <div v-for="dep in visibleDependencies" :key="`${dep.kind}-${dep.id}`" class="dependency">
       <div><strong>{{ dep.label }}{{ dep.kind === 'fee' ? ` · ¥${money(dep.amount_cents)}` : '' }}</strong><span>{{ resolutionDependencyStatus(dep.status) }}{{ dep.shared ? ' · 共享生产' : '' }}</span></div>
       <p v-if="dep.stop_work_orders?.length" class="related-orders"><button v-for="work in dep.stop_work_orders" :key="work.work_order_id" type="button" @click="relatedWork(work)">{{ work.label }} · 查看工单／办理退料</button></p>
       <p v-if="dep.kind==='fee' && dep.id>0" class="hint">原收款 ¥{{ money(dep.paid_cents) }} · 资金已处置 ¥{{ money(dep.settled_out_cents) }}；减免后由财务另行选择资金去向。</p>
       <p v-if="dep.orders?.length" class="related-orders"><button v-for="order in dep.orders" :key="order.id" type="button" @click="relatedOrder(order)">{{ order.number || `订单 #${order.id}` }} · {{ resolutionDependencyStatus(order.status) }}{{ Number(order.id) === Number(orderId) ? '（当前订单）' : '' }}</button></p>
       <span v-if="['cancelled', 'released', 'stopped'].includes(dep.status)" class="hint">已结束关联业务，保留历史记录；重新开启订单不会恢复已停工工单。</span>
       <template v-else-if="disposition(dep)"><select v-model="disposition(dep).choice" :aria-label="`${dep.label}处置`"><option value="">请选择处置</option><option v-for="v in choices(dep)" :key="v.value" :value="v.value" :disabled="v.disabled">{{ v.label }}{{ v.disabled ? `（${v.reason}）` : '' }}</option></select><label v-if="dep.kind === 'fee' && ['partial_waiver','negotiated'].includes(disposition(dep).choice)">{{ disposition(dep).choice === 'negotiated' ? '协商后保留收费（元）' : '本次减免金额（元）' }}<input v-model="disposition(dep).amount" inputmode="decimal" placeholder="请明确填写金额" /></label><label v-if="dep.kind === 'stock_reservation' && disposition(dep).choice === 'release'">本批释放数量（{{ dep.unit }}）<input v-model.number="disposition(dep).quantity" type="number" min="0" :max="dep.quantity" step="any" placeholder="明确填写本批释放数量" /></label>
        <template v-if="disposition(dep).choice === 'replace_supply'">
         <p class="hint">逐笔选择同客户、同商品、同冻结规格及同仓库的替代购货权益。仓库确认并建立新预订后，才会释放原预订。</p>
         <fieldset v-for="source in dep.supply_reservations" :key="source.id" class="supply-choice"><legend>{{ source.label }}</legend><template v-if="supplyChoice(dep,source)"><p>待入库预订 {{ source.quantity }} {{ source.unit }}</p><p v-if="source.converted_quantity>0">已入库原批次：{{ source.physical_batches }}</p><label v-if="source.converted_quantity>0">已入库 {{ source.converted_quantity }} {{ source.unit }} 的处置<select v-model="supplyChoice(dep,source).physical_choice"><option value="">请选择已入库部分的处置</option><option value="keep_reserved">保留原批次预留，继续用于本笔代发</option></select></label><p v-if="source.physical_issue" class="error">{{ source.physical_issue }}</p><label v-if="source.quantity>0">替代货源<select v-model.number="supplyChoice(dep,source).target_id"><option :value="0">请选择明确的替代货源</option><option v-for="target in source.targets" :key="target.id" :value="target.id">{{ target.label }} · 可用 {{ target.available_qty }} {{ source.unit }}</option></select></label><label v-if="source.quantity>0">本笔替换数量<input v-model="supplyChoice(dep,source).quantity" type="number" min="1" step="1" :max="source.quantity" placeholder="明确填写本笔待入库预订数量" /></label></template></fieldset>
        </template>
        <template v-if="dep.kind==='production' && disposition(dep).choice==='stop'">
         <p class="hint">生产负责人和仓库分别确认后停止剩余生产。已扣料和在制品保留原位置、货权及成本，不会自动变为可用库存；需要退还未加工余料时，先在工位办理退料，再刷新本方案。</p>
         <fieldset v-for="work in dep.stop_work_orders" :key="work.work_order_id" class="supply-choice"><legend>{{ work.label }}</legend>
          <p v-if="work.issue" class="error">{{ work.issue }}</p>
          <ul><li v-for="row in work.remainders" :key="`${row.kind}-${row.id}`">{{ row.label }} · {{ row.quantity }} {{ row.unit }} · {{ row.warehouse || '原批次记录' }} · {{ row.owner_customer_id ? `客户货权 #${row.owner_customer_id}` : '公司货权' }} · 成本 ¥{{ Number(row.cost || 0).toFixed(2) }}</li></ul>
          <template v-if="stopSelection(dep,work)">
           <label>已投余料<select v-model="stopSelection(dep,work).materials"><option value="">请选择余料处置</option><option v-if="work.has_materials" value="retain_original">保留原位及原货权，停止继续消耗</option><option v-else value="returned">已核对：没有未处理的已投余料</option></select></label>
           <label>在制品<select v-model="stopSelection(dep,work).wip"><option value="">请选择在制品处置</option><option v-if="work.has_wip" value="retain_original">保留原工序、位置、货权及成本</option><option v-else value="none">已核对：没有剩余在制品</option></select></label>
           <label>未耗用预留<select v-model="stopSelection(dep,work).reservations"><option value="">请选择预留处置</option><template v-if="work.has_reservations"><option value="keep">保留原预留</option><option value="release_unconsumed">释放未耗用部分，保留耗用记录</option></template><option v-else value="none">已核对：没有未耗用预留</option></select></label>
           <label>已完成产出<select v-model="stopSelection(dep,work).outputs"><option value="">请选择产出处置</option><option v-if="work.has_outputs" value="keep_original">保留原批次、货权、库存及成本</option><option v-else value="none">已核对：没有已完成产出</option></select></label>
          </template>
         </fieldset>
        </template>
        <template v-if="dep.kind==='production' && disposition(dep).choice==='allocate'">
         <p class="hint">明确选择原批次、普通销售订单及数量。仓库确认后建立目标预留；保留共享生产、批次和成本记录，不产生出库或转仓。目标已有生产、商品冻结规格或库存单位不同的订单须先核对。</p>
         <fieldset v-for="(selection,index) in disposition(dep).stock_selections" :key="index" class="supply-choice">
          <legend>成品分配 {{ index+1 }}</legend>
          <label>原成品批次<select v-model.number="selection.batch_id" @change="selection.target_item_id=0"><option :value="0">请选择原成品批次</option><option v-for="source in dep.company_stock" :key="source.batch_id" :value="source.batch_id">{{ source.label }} · 最多可分配 {{ source.available_quantity }}</option></select></label>
          <p v-if="stockSource(dep,selection)?.releases?.length" class="hint">本批有原订单预留；请在对应预留项目明确选择释放并填写数量，转配与释放同时生效。</p>
          <label>目标订单<select v-model.number="selection.target_item_id"><option :value="0">请选择明确的目标订单</option><option v-for="target in stockSource(dep,selection)?.targets || []" :key="target.order_item_id" :value="target.order_item_id">{{ target.label }} · 未预留需求 {{ target.remaining_quantity }}</option></select></label>
          <label>本次分配数量<input v-model="selection.quantity" type="number" min="1" step="1" :max="stockSource(dep,selection)?.available_quantity" placeholder="明确填写数量" /></label>
          <button type="button" @click="disposition(dep).stock_selections.splice(index,1)">移除此项</button>
         </fieldset>
         <button type="button" @click="addStockSelection(dep)">添加成品分配</button>
        </template>
        <template v-if="dep.kind==='production' && disposition(dep).choice==='scrap'">
         <p class="hint">填写实际报损依据，明确原批次和数量。财务批准后由仓库确认；所有必要处置完成后，报损出库与本次订单影响同时生效。其他订单和预留不能占用。</p>
         <label>报损原因／验收依据<textarea v-model="disposition(dep).reference" rows="2" placeholder="填写实际破损、变质等情况及依据" /></label>
         <label>原成本处理<select v-model="disposition(dep).cost_treatment"><option value="">请选择成本处理方式</option><option v-for="v in options.scrap_cost || []" :key="v.value" :value="v.value" :disabled="v.disabled">{{ v.label }}{{ v.disabled ? `（${v.reason}）` : '' }}</option></select></label>
         <p class="hint">保留原生产成本：报损记录单列数量和金额，不重复计费。重分类：在所选账期减记原成本、等额转为损失费用，原记录和总成本保留。</p>
         <fieldset v-for="(selection,index) in disposition(dep).stock_selections" :key="index" class="supply-choice">
          <legend>成品报损 {{ index+1 }}</legend>
          <label>原报损批次<select v-model.number="selection.batch_id" @change="selection.target_item_id=0"><option :value="0">请选择原成品批次</option><option v-for="source in dep.company_stock" :key="source.batch_id" :value="source.batch_id" :disabled="!source.cost_record_id || Boolean(source.scrap_issue)">{{ source.label }} · 最多 {{ source.available_quantity }}{{ source.scrap_issue ? `（${source.scrap_issue}）` : '' }}</option></select></label>
          <p v-if="stockSource(dep,selection)" class="hint">原成本账期 {{ stockSource(dep,selection).cost_month }} · 每库存单位 ¥{{ (stockSource(dep,selection).unit_cost_micros/1000000).toFixed(4) }}</p>
          <p v-if="stockSource(dep,selection)?.releases?.length" class="hint">本批有原订单预留，请在对应预留项目明确选择释放及数量；释放与报损同时生效。</p>
          <label>本次报损数量<input v-model="selection.quantity" type="number" min="1" step="1" :max="stockSource(dep,selection)?.available_quantity" placeholder="明确填写实际数量" /></label>
          <button type="button" @click="disposition(dep).stock_selections.splice(index,1)">移除此项</button>
         </fieldset>
         <button type="button" @click="addStockSelection(dep)">添加报损批次</button>
        </template>
        <p v-if="disposition(dep).choice==='reassign'" class="hint">提交前须选择具体目标并核验可用数量。</p>
       </template>
      </div>
     </fieldset>
     <fieldset v-if="!simple" :disabled="busy || !editable">
      <legend>账款与处理时点</legend>
      <p>订单款原收款 ¥{{ money(current.paid_cents) }}；独立费用收款见上方。关闭不会自动退款。</p>
      <div class="fields"><label v-for="[name,label] in [['timing','处理时点'],['external_status','开票及外部入账'],['posting_mode','账期处理']]" :key="name">{{ label }}<select v-model="form[name]"><option value="">请选择</option><option v-for="v in (name === 'timing' ? timingOptions : options[name])" :key="v.value" :value="v.value" :disabled="v.disabled">{{ v.label }}{{ v.disabled ? `（${v.reason}）` : '' }}</option></select></label><label>记账日期<input v-model="form.posting_date" type="date" /></label></div>
     </fieldset>
     <div class="actions"><button v-if="pending && !pending.applied" type="button" :disabled="busy" @click="load(true)">刷新当前方案</button><button type="button" :disabled="busy || !editable" @click="saveDraft">保存处理草稿</button><button type="button" class="primary" :disabled="busy || !editable || !form.action" @click="review">预览处理影响</button></div>
     <section v-if="preview" class="preview"><h3>确认本次处理</h3><p>处理后：{{ resolutionStatus(preview.plan.status, form.action === 'hold') }}；应收冲减 ¥{{ money(preview.plan.credit_cents) }}；实际退款需另行确认。</p><ul v-if="preview.plan.tasks.length"><li v-for="(task,i) in preview.plan.tasks" :key="i">待处理：{{ task.label }}</li></ul><button type="button" class="primary" :disabled="busy" @click="submit">{{ pending ? '确认更新处理方案' : '确认提交处理单' }}</button></section>
     <OrderResolutionBalance :order-id="orderId" :balances="loaded.balances || []" :history="loaded.balance_history || []" :targets="loaded.balance_targets || []" :permissions="loaded.permissions" :options="options" @changed="stepChanged" />
     <section v-if="loaded.history?.length"><h3>处理记录</h3><article v-for="r in loaded.history" :key="r.id" class="history"><strong>处理单 #{{ r.id }} · {{ ({pending:'处理中',completed:'已完成',withdrawn:'已撤回'})[r.status] || r.status }}</strong><p>{{ options.action.find(v => v.value === r.command.action)?.label }} · {{ options.reason.find(v => v.value === r.command.reason)?.label }} · {{ r.command.note }}</p><p v-if="r.applied && r.status === 'pending'" class="hint">履约已按本次方案处理，以下清算待办尚未全部完成。</p>
      <div v-for="loss in r.scraps || []" :key="loss.id" class="notice"><strong>已报损 · {{ loss.batch_code }}</strong><p>{{ loss.quantity }} {{ loss.unit }} · 原成本 ¥{{ money(loss.cost_cents) }} · {{ loss.posting_date }}</p><p>{{ options.scrap_cost?.find(v => v.value === loss.cost_treatment)?.label }} · {{ loss.reason }}</p></div>
      <OrderResolutionStep v-for="step in r.steps" :key="step.id" :order-id="orderId" :request-id="r.id" :step="step" :options="options" :permissions="loaded.permissions" :targets="loaded.targets || []" @changed="stepChanged" @refresh="load()" />
      <div v-if="r.can_withdraw && loaded.permissions?.['orders.resolve']" class="withdraw"><label>撤回原因<input v-model="withdrawalNotes[r.id]" placeholder="仅在尚未执行处置时可撤回" /></label><button type="button" :disabled="busy || !withdrawalNotes[r.id]?.trim()" @click="withdraw(r)">撤回处理单</button></div>
      <button v-if="r.amendment_id && !r.revised_order_id" type="button" :disabled="busy" @click="revise(r)">打开关联修订草稿</button><p v-if="r.revised_order_id" class="hint">已生成修订单 #{{ r.revised_order_id }}，新单单独审核。</p></article></section>
    </div>
   </section>
  </div>
 </Teleport>
</template>
<style scoped>
.resolution-entry{border:1px solid #c5d5cd;background:#fff;color:#285e49;border-radius:6px;padding:6px 10px;cursor:pointer}.resolution-backdrop{position:fixed;inset:0;z-index:1200;background:#10251d66;display:flex;justify-content:flex-end}.resolution-drawer{width:min(820px,100vw);height:100%;box-sizing:border-box;overflow:auto;background:#f7f9f7;padding:24px;color:#20372c}.resolution-drawer header{position:sticky;top:-24px;z-index:2;background:#f7f9f7;padding:16px 0;display:flex;justify-content:space-between;align-items:flex-start;gap:16px}.resolution-drawer header strong{display:block;margin-top:6px}.resolution-drawer h2{margin:0 0 8px}.resolution-content{display:grid;gap:18px}.resolution-drawer fieldset{border:1px solid #d6e0d9;background:white;border-radius:10px;padding:16px;min-width:0}.resolution-drawer legend{font-weight:700}.fields{display:grid;grid-template-columns:1fr 1fr;gap:14px}.fields .wide{grid-column:1/-1}.resolution-drawer label{display:flex;flex-direction:column;gap:7px}.resolution-drawer input,.resolution-drawer select,.resolution-drawer textarea{box-sizing:border-box;max-width:100%;min-width:0;padding:9px;border:1px solid #bccdc1;border-radius:6px;background:#fff;color:inherit;font:inherit}.resolution-drawer button{padding:9px 14px;border:1px solid #b8cbbf;border-radius:6px;background:#fff;color:inherit;cursor:pointer}.resolution-drawer .primary{background:#286449;color:white}.resolution-drawer button:disabled{opacity:.5;cursor:default}.hint,small{color:#62776a;font-size:13px}.error{color:#ab3026;background:#fff0ed;padding:12px}.notice{background:#e6f3e9;padding:12px}.return-selection{padding:14px 0;border-bottom:1px solid #e2e8e3}.return-selection>button{margin-top:10px}.table-scroll{overflow:auto}table{width:100%;border-collapse:collapse;margin-top:12px}td,th{text-align:left;padding:9px;border-bottom:1px solid #e2e8e3;white-space:normal}td input{width:100px}small{display:block}.dependency{padding:12px 0;border-bottom:1px solid #e5ebe6}.dependency>div{display:flex;gap:12px;margin-bottom:8px}.dependency select{width:100%}.related-orders{display:flex;gap:10px;flex-wrap:wrap}.related-orders a{color:#285e49;font-size:13px}.actions{display:flex;justify-content:flex-end;gap:12px}.preview{border:2px solid #3f8060;padding:16px;border-radius:10px;background:#fff}.history{padding:12px;border-bottom:1px solid #d5e0d8}@media(max-width:600px){.fields{grid-template-columns:1fr}.resolution-drawer{padding:16px}.resolution-drawer header{top:-16px}.actions{flex-wrap:wrap}}
</style>
