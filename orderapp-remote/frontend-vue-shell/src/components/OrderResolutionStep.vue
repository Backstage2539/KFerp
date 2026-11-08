<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { apiSend } from '../api/client'
import { blankResolutionStep, resolutionStepPayload } from '../lib/order-resolution'
const props = defineProps({ orderId: Number, requestId: Number, step: Object, options: Object, permissions: Object, targets: { type: Array, default: () => [] } })
const emit = defineEmits(['changed','refresh'])
const form = reactive(blankResolutionStep()), busy = ref(false), error = ref(''), expanded = ref(false)
let key = ''
watch(form, () => { key = '' }, { deep: true, flush: 'sync' })
const allowed = computed(() => Boolean(props.permissions?.[props.step.permission]) && (props.step.kind !== 'period' || props.permissions?.['finance.close']))
const choices = computed(() => {
 if (props.step.kind === 'funds') return props.options?.funds || []
 if (props.step.kind === 'external') return [{value:'none',label:'已核实：未开票且无外部入账'},{value:'recorded',label:'已完成冲减或更正并登记凭据'}]
 if (props.step.kind === 'approval') return [{value:'approve',label:'批准本次订单处理'}]
 if (props.step.kind === 'period') return [{value:'approve',label:'批准历史账期更正，保留原快照'}]
 if (props.step.kind === 'production_finance') return (props.options?.scrap_cost || []).filter(v=>v.value===props.step.choice)
 if (props.step.kind === 'production_stock') return (props.options?.production || []).filter(v=>v.value==='stop').map(v=>({...v,label:'确认余料、在制品、预留和产出的原停工方案'}))
 return (props.options?.[props.step.kind] || []).filter(v => v.value === props.step.choice)
})
async function submit() {
 if (busy.value || !allowed.value) return
 busy.value = true; error.value = ''
 try {
  if (!key) key = crypto.randomUUID()
  await apiSend(`/api/orders/${props.orderId}/resolution/${props.requestId}/steps/${props.step.id}`, { body: resolutionStepPayload(form, key) })
  Object.assign(form, blankResolutionStep()); expanded.value = false; emit('changed')
 } catch (e) { error.value = e.message; emit('refresh') } finally { busy.value = false }
}
</script>
<template>
 <div class="resolution-step">
  <div class="step-title"><strong>{{ step.label }}</strong><span>{{ ({pending:'待处理',completed:'已完成',failed:'登记失败待重试',superseded:'已被新方案替代'})[step.status] || step.status }}</span><button v-if="['pending','failed'].includes(step.status) && allowed" type="button" :disabled="busy" @click="expanded = !expanded">{{ expanded ? '收起' : '处理待办' }}</button></div>
  <p v-if="step.last_error" class="error">{{ step.last_error }}</p>
  <details v-if="step.failures?.length"><summary>失败登记记录（{{ step.failures.length }}）</summary><p v-for="(failure,index) in step.failures" :key="index" class="muted">{{ new Date(failure.created_at).toLocaleString('zh-CN') }} · {{ failure.actor }} · {{ failure.message }}</p></details>
  <p v-if="['pending','failed'].includes(step.status) && !allowed" class="muted">请由{{ ({'production.resolve':'生产负责人','stock.resolve':'仓库人员','finance.resolve':'财务人员','orders.resolve':'销售人员'})[step.permission] }}确认。</p>
  <p v-if="step.status === 'completed' && step.result?.reference" class="muted">凭据：{{ step.result.reference }}</p>
  <fieldset v-if="expanded && allowed" :disabled="busy">
   <label>确认处置<select v-model="form.choice"><option value="">请选择</option><option v-for="v in choices" :key="v.value" :value="v.value" :disabled="v.disabled">{{ v.label }}</option></select></label>
   <template v-if="step.kind === 'funds'">
    <p class="muted">按已完成的实际处理登记。可分次处理，系统重新核对可用金额；登记退款不会代为转账。</p>
    <label>本次金额（元）<input v-model="form.amount" inputmode="decimal" placeholder="填写实际金额" /></label>
    <label v-if="form.choice === 'refund'">实际付款账户<input v-model="form.account" placeholder="填写已付款的银行或现金账户" /></label>
    <label v-if="form.choice === 'offset'">抵扣应收<select v-model.number="form.target_id"><option :value="0">请选择同客户的应收</option><option v-for="v in targets" :key="v.id" :value="v.id">{{ v.label }} · 可抵扣 ¥{{ (v.available_cents / 100).toFixed(2) }}</option></select></label>
    <label>实际处理日期<input v-model="form.posting_date" type="date" /></label>
   </template>
   <label v-if="step.kind==='return_stock'">实际退回入库日期<input v-model="form.posting_date" type="date" /></label>
   <label v-if="step.kind==='return_stock'">退回验收凭据<input v-model="form.reference" placeholder="填写收货、验收凭据编号" /></label>
   <label v-if="['funds','external','period','billing','production_finance'].includes(step.kind) || step.choice==='scrap'">付款／更正／批准凭据<input v-model="form.reference" placeholder="填写可核对的凭据编号" /></label>
   <label>处理说明<textarea v-model="form.note" rows="2" /></label>
   <p v-if="error" class="error" role="alert">{{ error }}</p>
   <button type="button" :disabled="busy || !form.choice" @click="submit">确认本项处理</button>
  </fieldset>
 </div>
</template>
<style scoped>
.resolution-step{padding:12px 0;border-top:1px solid #d5e0d8}.step-title{display:flex;align-items:center;gap:12px;flex-wrap:wrap}.step-title span{font-size:13px;color:#617467}.step-title button{margin-left:auto}fieldset{margin-top:12px;border:1px solid #cbd9cf;border-radius:8px;padding:12px;display:grid;gap:12px;min-width:0}label{display:flex;flex-direction:column;gap:6px}input,select,textarea{box-sizing:border-box;width:100%;padding:9px;border:1px solid #b8cbbf;border-radius:6px;font:inherit;background:white;color:inherit}button{padding:8px 12px;background:white;border:1px solid #b8cbbf;border-radius:6px;color:#285e49;cursor:pointer}button:disabled{opacity:.5;cursor:default}.muted{font-size:13px;color:#617467}.error{color:#a52c23;background:#fff0ed;padding:10px}
</style>
