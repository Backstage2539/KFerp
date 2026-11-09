<template>
  <section class="group-completion" aria-label="合并完成并入库">
    <div class="form-head"><div><strong>完成并入库</strong><p>本次数量按工单顺序分配；每张来源工单分别保留耗料、成本和入库单据。</p></div><button type="button" class="secondary" :disabled="retry.uncertain || busy" @click="$emit('close')">关闭</button></div>
    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="busy && !preview" class="muted">正在核对同组任务、用料和目标仓库…</p>
    <template v-if="preview">
      <div class="summary">{{ preview.items?.[0]?.product_name }} · {{ preview.items?.[0]?.spec_g }}g袋装 · 剩余 {{ qty(preview.remaining_qty) }} {{ preview.output_unit }} · 目标仓：{{ preview.warehouse_name }}</div>
      <div class="source-list"><span v-for="item in preview.items" :key="item.job_card_id">{{ item.work_order_no }}：已完成 {{ qty(item.completed_qty) }} / {{ qty(item.planned_qty) }} {{ item.unit }}</span></div>
      <fieldset :disabled="busy || retry.uncertain">
      <label class="quantity-field"><span>本次实际完成袋数</span><input v-model.number="actualOutputQty" type="number" min="1" step="1" inputmode="numeric" /><small>参考剩余量 {{ qty(preview.remaining_qty) }} 袋；实际数量按现场填写。</small></label>
      <label class="quantity-field"><span>本次实际工时（分钟，可选）</span><input v-model.number="actualMinutes" type="number" min="0" step="1" /></label>
      <div v-if="preview.reporting_type==='piece'" class="source-inputs">
        <h4>逐张工单称重余料</h4>
        <template v-for="item in preview.items" :key="item.job_card_id">
          <div v-if="allocationQty(item.job_card_id) > 0">
            <p>{{ item.work_order_no }}：本次 {{ qty(allocationQty(item.job_card_id)) }} {{ item.unit }} · 预计余料 {{ qty(groupRemainderSuggestion(item, allocationQty(item.job_card_id))) }} g</p>
            <label class="quantity-field">实际余料（g）<input v-model.number="remainders[item.job_card_id]" type="number" min="0" step="1" @input="touchedRemainders[item.job_card_id]=true;confirmedRemainders[item.job_card_id]=false" /></label>
            <label><input v-model="confirmedRemainders[item.job_card_id]" type="checkbox" /> 已称重并确认</label>
            <div v-if="allocationQty(item.job_card_id)===Number(item.remaining_qty) && item.packaging_unissued_candidates?.length">
              <p>同批次未领可用库存（共享）；仅移动明确选择的数量。</p>
              <div v-for="candidate in item.packaging_unissued_candidates" :key="candidateKey(candidate)">
                <label><input type="checkbox" :checked="!!selectedCandidate(item.job_card_id,candidate)" @change="toggleCandidate(item.job_card_id,candidate,$event.target.checked)" /> {{ candidate.batch_code }} · {{ candidate.warehouse }} · 货主 {{ candidate.owner_customer_id || '公司' }} · 可用 {{ qty(candidate.available_g) }} g</label>
                <label v-if="selectedCandidate(item.job_card_id,candidate)" class="quantity-field">确认数量（g）<input v-model.number="selectedCandidate(item.job_card_id,candidate).quantity_g" type="number" min="1" :max="candidate.available_g" step="1" /></label>
              </div>
            </div>
          </div>
        </template>
        <label class="quantity-field">余料去向<select v-model="remainderWarehouse"><option value="wip">WIP在制仓</option><option v-for="w in warehouses.filter(row => row.code !== 'wip')" :key="w.code" :value="w.code">{{ w.name }}</option></select><small>每张工单完成目标后才转移选定余料，未完成任务继续保留。</small></label>
      </div>
      <div v-else-if="preview.material_consumption_policy==='bom_start_v1'" class="source-inputs">
 <p>用料已在开工时扣减。逐项填写本次实际处理投入，完成时只结转成本。</p>
 <label v-for="item in preview.items" :key="item.job_card_id" class="quantity-field">{{item.work_order_no}} 实际处理投入（{{item.input_unit}}，剩余 {{item.input_qty}}）<input type="number" min="0" step="any" v-model.number="processedInputs[item.job_card_id]" /></label>
 </div>
 <div v-else class="materials">
        <h4>本次建议用料（可调整）</h4>
        <div v-for="(material, index) in materialInputs" :key="material.key" class="material-input-row">
          <label><span>{{ material.name }} · {{ material.warehouse_name || material.warehouse }}</span><input v-model.number="material.qty" type="number" min="0" :step="isCountUnit(material.unit) ? 1 : 0.001" @input="touchMaterial(index)" /></label>
          <small>按本次数量建议 {{ qty(material.suggested) }} {{ material.unit }} · 可用 {{ qty(material.available_qty) }} {{ material.unit }}<template v-if="material.shortage_qty > 0"> · 缺 {{ qty(material.shortage_qty) }} {{ material.unit }}</template></small>
        </div>
      </div>
      <label v-if="preview.reporting_type!=='piece'" class="finish-option"><input v-model="finishGroup" type="checkbox" /> 本组全部做完（即使少于剩余计划也结束）</label>
      <label v-if="preview.reporting_type==='piece' || finishGroup || actualOutputQty > preview.remaining_qty" class="reason-field"><span>数量差异原因</span><textarea v-model.trim="differenceReason" rows="2" /></label>
      <details class="allocation-details"><summary>来源工单数量分配预览</summary><div v-for="item in allocationPreview" :key="item.job_card_id">{{ item.work_order_no }}：{{ qty(item.qty) }} {{ item.unit }}</div></details>
      <p v-if="validationMessage" class="error" role="alert">{{ validationMessage }}<template v-if="actualOutputQty > preview.max_output_qty">；请先补齐物料或调低本次数量。</template></p>
      </fieldset>
      <button type="button" class="primary submit" :disabled="busy || !canSubmit" @click="submit">{{ busy ? '正在提交…' : retry.uncertain ? '重试本次完成' : '确认完成并入库' }}</button>
      <p class="muted">提交失败时保留本次填写内容；未改变内容重试会复用同一请求编号。</p>
      <div v-if="result" class="result"><strong>已入库 {{ qty(result.completed_qty) }} {{ result.output_unit }}</strong><div v-for="item in result.items" :key="item.job_card.id">{{ item.work_order.work_order_no }}：{{ qty(item.completion?.output_qty || 0) }} {{ item.completion?.output_unit }} · 单据 {{ item.completion?.stock_entry_ids?.join('、') || '无' }}</div><p>{{ result.remaining_qty > 0 ? `剩余 ${qty(result.remaining_qty)} 袋待继续` : '本组任务已完成' }}</p></div>
    </template>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { workstationGroupCompletionValidation, workstationGroupMaterialSuggestions, workstationGroupOutputAllocations } from '../lib/production-workstation.js'
import { groupRemainderSuggestion, validateGroupRemainders } from '../lib/packaging-group-remainder.js'
import { completionRetry } from '../lib/operation-completion.js'

const props = defineProps({ taskIds: { type: Array, required: true } })
const emit = defineEmits(['close', 'posted'])
const busy = ref(false)
const error = ref('')
const preview = ref(null)
const result = ref(null)
const actualOutputQty = ref(0)
const actualMinutes = ref(0)
const finishGroup = ref(false)
const differenceReason = ref('')
const materialInputs = ref([])
const processedInputs=ref({})
const remainders=ref({}), confirmedRemainders=ref({}), touchedRemainders=ref({}), warehouses=ref([]), remainderWarehouse=ref('wip')
const selectedUnissued=ref({})
const retry=reactive(completionRetry())
const unitFactor = (unit) => ({ g: 1, 克: 1, kg: 1000, 千克: 1000, 公斤: 1000, mg: 0.001, 毫克: 0.001 })[String(unit || '').toLowerCase()] || 0
const isCountUnit = (unit) => unitFactor(unit) === 0
const qty = (value) => Number(value || 0).toLocaleString('zh-CN', { maximumFractionDigits: 3 })
const validationMessage = computed(() => preview.value?.reporting_type === 'piece' ? (
  !Number.isInteger(Number(actualOutputQty.value)) || Number(actualOutputQty.value) <= 0 || Number(actualOutputQty.value) > Number(preview.value.remaining_qty) ? '完成件数必须为不超过剩余目标的正整数' : (validateGroupRemainders(preview.value.items || [], allocationPreview.value, remainders.value, confirmedRemainders.value, differenceReason.value) || validateSelectedUnissued())
) : workstationGroupCompletionValidation({
  outputQty: actualOutputQty.value,
  maxOutputQty: preview.value?.max_output_qty,
  remainingQty: preview.value?.remaining_qty,
  outputUnit: preview.value?.output_unit,
  finishGroup: finishGroup.value,
  differenceReason: differenceReason.value,
  materials: materialInputs.value,
}))
const canSubmit = computed(() => Boolean(preview.value) && !validationMessage.value)
const allocationPreview = computed(() => workstationGroupOutputAllocations(preview.value?.items || [], actualOutputQty.value))
function allocationQty(id) { return Number(allocationPreview.value.find(row => Number(row.job_card_id)===Number(id))?.qty || 0) }
const candidateKey=c=>`${c.batch_code}\u0000${c.warehouse}\u0000${Number(c.owner_customer_id||0)}`
function selectedCandidate(id,c){return (selectedUnissued.value[id]||[]).find(row=>candidateKey(row)===candidateKey(c))}
function toggleCandidate(id,c,checked){const rows=(selectedUnissued.value[id]||[]).filter(row=>candidateKey(row)!==candidateKey(c));if(checked)rows.push({batch_code:c.batch_code,warehouse:c.warehouse,owner_customer_id:Number(c.owner_customer_id||0),quantity_g:0});selectedUnissued.value[id]=rows}
function validateSelectedUnissued(){for(const item of preview.value?.items||[]){for(const row of selectedUnissued.value[item.job_card_id]||[]){const c=(item.packaging_unissued_candidates||[]).find(value=>candidateKey(value)===candidateKey(row));if(!c||!Number.isInteger(Number(row.quantity_g))||Number(row.quantity_g)<=0||Number(row.quantity_g)>Number(c.available_g))return '请为选定共享批次填写不超过可用量的整数克数'}}return ''}
function updateRemainders() {
  if (preview.value?.reporting_type !== 'piece') return
  for (const item of preview.value.items || []) {
    if (!touchedRemainders.value[item.job_card_id]) remainders.value[item.job_card_id]=groupRemainderSuggestion(item,allocationQty(item.job_card_id))
    confirmedRemainders.value[item.job_card_id]=false
    if(allocationQty(item.job_card_id)!==Number(item.remaining_qty))selectedUnissued.value[item.job_card_id]=[]
  }
}

function updateSuggestedMaterials() {
  if (!preview.value) return
  const suggestions = new Map(workstationGroupMaterialSuggestions(preview.value, actualOutputQty.value).map(row => [row.key, row.qty]))
  for (const row of materialInputs.value) {
    row.suggested = Number(suggestions.get(row.key) || 0)
    if (!row.touched) row.qty = row.suggested
  }
}

function touchMaterial(index) {
  if (materialInputs.value[index]) materialInputs.value[index].touched = true
}

watch(actualOutputQty, () => { updateSuggestedMaterials(); updateRemainders() })

async function load() {
  busy.value = true
  error.value = ''
  try {
    const data = await apiSend('/api/production/workstation/completion/preview', { body: { job_card_ids: props.taskIds.map(Number) } })
    preview.value = data
 processedInputs.value=Object.fromEntries((data.items||[]).map(i=>[i.job_card_id,i.input_qty]))
    actualOutputQty.value = Number(data.remaining_qty || 0)
    materialInputs.value = (data.materials || []).map(material => ({
      ...material,
      suggested: Number(material.required_qty || 0),
      qty: Number(material.required_qty || 0),
      touched: false,
    }))
    updateSuggestedMaterials()
    updateRemainders()
    if (data.reporting_type === 'piece') { try { const locations=await apiGet('/api/stock/warehouses'); warehouses.value=(locations.rows||[]).filter(w=>w.active!==false&&w.code) } catch { warehouses.value=[] } }
  } catch (err) { error.value = err.message || '无法读取合并完成建议' } finally { busy.value = false }
}

function materialRowsForSubmit() {
  const byKey = new Map(materialInputs.value.map(row => [row.key, Number(row.qty || 0)]))
  const outputByCard = new Map(allocationPreview.value.map(item => [Number(item.job_card_id), Number(item.qty || 0)]))
  const itemByCard = new Map((preview.value.items || []).map(item => [Number(item.job_card_id), item]))
  const rows = []
  for (const material of preview.value.materials || []) {
    const total = byKey.get(material.key) || 0
    const sources = material.sources || []
    const defaults = sources.map(source => {
      const item = itemByCard.get(Number(source.job_card_id))
      const output = outputByCard.get(Number(source.job_card_id)) || 0
      return item && Number(item.remaining_qty) > 0 ? Number(source.qty || 0) * output / Number(item.remaining_qty) : 0
    })
    const defaultTotal = defaults.reduce((sum, value) => sum + value, 0)
    let assigned = 0
    const activeIndexes = defaults.map((value, index) => value > 0 ? index : -1).filter(index => index >= 0)
    sources.forEach((source, index) => {
      if (!activeIndexes.includes(index)) { rows.push({ ...source, qty: 0 }); return }
      const lastActive = activeIndexes[activeIndexes.length - 1]
      const value = index === lastActive ? total - assigned : (defaultTotal > 0 ? total * defaults[index] / defaultTotal : 0)
      const normalized = isCountUnit(source.unit) ? Math.floor(value + 1e-8) : Math.round(value * 1000) / 1000
      assigned += normalized
      rows.push({ ...source, qty: normalized })
    })
  }
  return rows
}

async function submit() {
  if (!canSubmit.value || busy.value) return
  busy.value = true
  error.value = ''
  const payload = {
    job_card_ids: props.taskIds.map(Number),
    actual_output_qty: Number(actualOutputQty.value),
    output_unit: preview.value.output_unit,
    actual_minutes: Number(actualMinutes.value || 0),
    materials: preview.value.material_consumption_policy==='bom_start_v1'?null:materialRowsForSubmit(),
    processed_inputs:preview.value.material_consumption_policy==='bom_start_v1'&&preview.value.reporting_type!=='piece'?processedInputs.value:undefined,
    actual_remainders_g:preview.value.reporting_type==='piece'?remainders.value:undefined,
    selected_unissued_remainders:preview.value.reporting_type==='piece'?selectedUnissued.value:undefined,
    remainder_warehouse:preview.value.reporting_type==='piece'?remainderWarehouse.value:undefined,
    difference_reason: differenceReason.value.trim(),
    finish_group: preview.value.reporting_type==='piece' ? Number(actualOutputQty.value)===Number(preview.value.remaining_qty) : Boolean(finishGroup.value),
    expected_version: preview.value.version,
  }
  try {
    result.value = await apiSend('/api/production/workstation/completion/submit', { body: retry.begin(payload,()=>crypto.randomUUID()) })
    retry.clear()
    // Keep the result visible and let the parent refresh the workstation.
    emit('posted', result.value)
  } catch (err) { retry.fail(err); error.value = retry.uncertain ? '尚未收到完成结果，请重试原次操作，系统不会重复扣料。' : (err.message || '合并完成失败，可保持当前填写后重试') } finally { busy.value = false }
}

onMounted(load)
</script>

<style scoped>
.group-completion{grid-column:1/-1;padding:1rem;border:1px solid #93c5aa;border-top:4px solid #2b8f5b;border-radius:.8rem;background:#f7faf8}.group-completion fieldset{border:0;padding:0;min-width:0}.form-head{display:flex;justify-content:space-between;gap:1rem;align-items:flex-start}.form-head p,.muted{color:#718078;margin:.3rem 0}.summary{font-weight:700;margin:.8rem 0}.source-list{display:flex;flex-wrap:wrap;gap:.4rem .9rem;color:#66746d}.quantity-field,.reason-field{display:flex;flex-direction:column;gap:.35rem;margin:1rem 0}.quantity-field input,.reason-field textarea,.material-input-row input{font:inherit;border:1px solid #bccbc2;border-radius:.5rem;padding:.65rem}.quantity-field input{max-width:15rem;font-size:1.2rem}.materials{margin:1rem 0}.materials h4{margin-bottom:.5rem}.material-input-row{padding:.55rem 0;border-bottom:1px solid #dce5df}.material-input-row label{display:flex;justify-content:space-between;align-items:center;gap:1rem}.material-input-row input{width:10rem}.material-input-row small{color:#718078}.finish-option{display:flex;align-items:center;gap:.5rem;margin:1rem 0}.allocation-details{margin:.8rem 0}.allocation-details div{padding:.25rem 0}.submit{width:100%;margin-top:.8rem}.error{color:#a33327}.result{padding:.8rem;margin-top:.8rem;background:#eaf7ef;border-radius:.5rem}.result div{padding:.25rem 0}
</style>
