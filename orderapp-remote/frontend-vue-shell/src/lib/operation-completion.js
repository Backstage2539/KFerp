export function weightUnit(unit) { return ['g','gram','grams','kg','kilogram','kilograms','mg','lb','lbs','pound','pounds','oz','ounce','ounces','克','千克','公斤','毫克','磅','盎司'].includes(String(unit).toLowerCase()) }
export function executionBatchInputQty(batch, inputUnit) {
 const unit=String(batch?.input_unit||inputUnit||'')
 const qty=Number(batch?.planned_input_qty)
 return unit===String(inputUnit||'') && Number.isFinite(qty) && qty>=0 ? qty : null
}
export function routeExecution(op = {}, last = false, fallbackKey = '') {
  let saved = {}; try { saved = JSON.parse(op.execution_json || '{}') } catch { /* API validates on save. */ }
  return { version: 1, node_key: fallbackKey, reporting_type: 'legacy', receipt_enabled: last, receipt_warehouse: '', source_warehouse: 'wip', output_unit: last ? '' : 'g', ...saved }
}
export function completionQuantityLabel(unit) { return unit === '袋' ? '本次完成袋数' : `本次完成数量（${unit || '-'}）` }
function ledgerWeight(qty,unit) {
  const factor={g:1,'克':1,kg:1000,'千克':1000,'公斤':1000,mg:.001,'毫克':.001}[String(unit).toLowerCase()]
  return factor ? Math.round(qty*factor)/factor : qty
}
export function suggestedMaterialQty(preview, material, batchOutputQty = preview?.planned_qty || 0) {
  const total = Number(material?.total_qty || 0)
  const planned = Number(preview?.total_planned_qty || 0)
  if (total > 0 && planned > 0) {
    const cumulative = Number(preview?.completed_output_qty || 0) + Number(batchOutputQty || 0)
    const target = weightUnit(material.unit)
      ? ledgerWeight(total * Math.min(1, cumulative / planned),material.unit)
      : Math.ceil(total * Math.min(1, cumulative / planned) - 1e-9)
    return ledgerWeight(Math.max(0, target - Number(material.consumed_qty || 0)),material.unit)
  }
  const ratio = Number(preview?.planned_qty || 0) > 0 ? Number(batchOutputQty || 0) / Number(preview.planned_qty) : 0
  const required = Number(material?.qty || 0) * Math.max(0, ratio)
  return weightUnit(material.unit) ? ledgerWeight(required,material.unit) : Math.ceil(required - 1e-9)
}
export function suggestedInputQty(preview, batchOutputQty = preview?.max_output_qty ?? preview?.planned_qty ?? 0) {
  const input = Number(preview?.input_qty || 0)
  const output = Number(batchOutputQty || 0)
  const planned = Number(preview?.planned_qty || 0)
  if (!planned) return 0
  if (String(preview?.input_unit || '').toLowerCase() === String(preview?.output_unit || '').toLowerCase()) {
    return Math.round(input * output / planned * 1e6) / 1e6
  }
  const inputFactor = {g:1,'克':1,kg:1000,'千克':1000,'公斤':1000,mg:.001,'毫克':.001}[String(preview?.input_unit || '').toLowerCase()]
  const outputFactor = {g:1,'克':1,kg:1000,'千克':1000,'公斤':1000,mg:.001,'毫克':.001}[String(preview?.output_unit || '').toLowerCase()]
  if (inputFactor && !outputFactor && Number(preview?.output_spec_g || 0) > 0) {
    return Math.round(output * Number(preview.output_spec_g) / inputFactor * 1e6) / 1e6
  }
  return Math.round(input * output / planned * 1e6) / 1e6
}
export function completionDraft(preview) {
  if(preview.material_consumption_policy==='bom_start_v1' && preview.execution?.reporting_type==='piece')return {actual_output_qty:Number(preview.planned_qty||0),actual_input_qty:Number(preview.planned_qty||0),actual_minutes:0,difference_reason:'',finish_operation:true,actual_remainder_g:packagingExpectedRemainder(preview,preview.planned_qty),remainder_warehouse:'wip',remainder_confirmed:false,selected_unissued_remainders:[]}
  if(preview.material_consumption_policy==='bom_start_v1')return {actual_output_qty:Number(preview.planned_qty||0),actual_input_qty:Number(preview.input_qty||0),waste_qty:0,actual_minutes:0,difference_reason:'',finish_operation:true}
  return {actual_output_qty:Number(preview.planned_qty || 0)}
}
function quantity(value,unit) {
  if (value === '' || value == null) throw new Error('请填写实际数量')
  const n=Number(value);if(!Number.isFinite(n)||n<0) throw new Error('数量必须大于等于零')
  if(!weightUnit(unit)&&!Number.isInteger(n)) throw new Error(`${unit}数必须是整数`)
  return n
}
export function completionPayload(preview,draft,id) {
  if(preview.material_consumption_policy==='bom_start_v1'){
   const minutes=Number(draft.actual_minutes||0);if(!Number.isInteger(minutes)||minutes<0)throw new Error('请填写有效工时')
   if(preview.execution?.reporting_type==='piece') {
    const output=quantity(draft.actual_output_qty,preview.output_unit)
    if(output<=0 || output>Number(preview.planned_qty))throw new Error('本次完成件数必须大于零且不能超过剩余目标')
    if(!draft.remainder_confirmed)throw new Error('请称重并确认实际余料')
    const remainder=quantity(draft.actual_remainder_g,'g')
    if(!Number.isInteger(remainder)||remainder>Number(preview.packaging_available_g))throw new Error('余料重量必须为不超过可用量的整数克')
    if(remainder!==packagingExpectedRemainder(preview,output)&&!String(draft.difference_reason||'').trim())throw new Error('实际余料与预计不一致，请填写差异原因')
    const seen=new Set()
    const selected=(draft.selected_unissued_remainders||[]).map(row=>{
      const key=`${row.batch_code}\u0000${row.warehouse}\u0000${Number(row.owner_customer_id||0)}`
      const candidate=(preview.packaging_unissued_candidates||[]).find(c=>`${c.batch_code}\u0000${c.warehouse}\u0000${Number(c.owner_customer_id||0)}`===key)
      const n=Number(row.quantity_g)
      if(seen.has(key)||!candidate||!Number.isInteger(n)||n<=0||n>Number(candidate.available_g))throw new Error('所选批次转移数量必须为正整数且不超过可用数量')
      seen.add(key);return {batch_code:row.batch_code,warehouse:row.warehouse,owner_customer_id:Number(row.owner_customer_id||0),quantity_g:n}
    })
    return {request_id:id,expected_version:preview.version,input_unit:preview.input_unit,output_unit:preview.output_unit,actual_input_qty:output,actual_output_qty:output,finish_operation:output===Number(preview.planned_qty),actual_minutes:minutes,difference_reason:draft.difference_reason||'',actual_remainder_g:remainder,remainder_warehouse:draft.remainder_warehouse||'wip',selected_unissued_remainders:selected}
   }
   const input=quantity(draft.actual_input_qty,preview.input_unit)
   const incoming=preview.execution?.reporting_type==='incoming'
   const waste=incoming ? quantity(draft.waste_qty,preview.input_unit) : 0
   if(incoming && waste>input)throw new Error('废料不能超过实际投入')
   if(incoming && waste>0 && !String(draft.difference_reason||'').trim())throw new Error('请填写废料原因')
   const output=incoming ? incomingOutput(input,waste,preview.input_unit,preview.output_unit) : quantity(draft.actual_output_qty,preview.output_unit)
   return {request_id:id,expected_version:preview.version,input_unit:preview.input_unit,output_unit:preview.output_unit,actual_input_qty:input,actual_output_qty:output,...(incoming?{waste_qty:waste}:{}),finish_operation:draft.finish_operation!==false,actual_minutes:minutes,difference_reason:draft.difference_reason||''}
  }
  const output=quantity(draft.actual_output_qty,preview.output_unit)
  return {request_id:id,expected_version:preview.version,output_unit:preview.output_unit,actual_output_qty:output,finish_operation:true}
}
export function packagingExpectedRemainder(preview,output) {
 return Math.max(0,Number(preview?.packaging_available_g||0)-Math.round(Number(output||0)*Number(preview?.packaging_standard_g_per_piece||0)))
}
export function incomingOutput(input,waste,inputUnit,outputUnit) {
 const factors={g:1,gram:1,grams:1,'克':1,kg:1000,kilogram:1000,kilograms:1000,'千克':1000,'公斤':1000,mg:.001,'毫克':.001,lb:453.59237,lbs:453.59237,pound:453.59237,pounds:453.59237,oz:28.349523125,ounce:28.349523125,ounces:28.349523125,'磅':453.59237,'盎司':28.349523125}
 const a=factors[String(inputUnit).toLowerCase()],b=factors[String(outputUnit).toLowerCase()]
 if(!a||!b)throw new Error('来料加工的投入和产出必须使用重量单位')
 return Math.round((Number(input)-Number(waste))*a/b*1e6)/1e6
}
export function completionRetry() {
  let attempt=null,uncertain=false
  return {get uncertain(){return uncertain},begin(payload,makeID){if(!attempt)attempt=JSON.parse(JSON.stringify({...payload,request_id:makeID()}));return attempt},fail(err){uncertain=!!attempt&&(!err.status||err.status>=500);if(!uncertain)attempt=null},clear(){attempt=null;uncertain=false}}
}
export function progressPayload(preview,draft) {
  return {expected_version:preview.version,output_unit:preview.output_unit,actual_output_qty:Number(draft.actual_output_qty)||0}
}
