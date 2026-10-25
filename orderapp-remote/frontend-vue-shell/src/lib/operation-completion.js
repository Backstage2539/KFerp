export function weightUnit(unit) { return ['g','kg','mg','克','千克','公斤','毫克'].includes(String(unit).toLowerCase()) }
export function routeExecution(op = {}, last = false, fallbackKey = '') {
  let saved = {}; try { saved = JSON.parse(op.execution_json || '{}') } catch { /* API validates on save. */ }
  return { version: 1, node_key: fallbackKey, receipt_enabled: last, receipt_warehouse: '', source_warehouse: 'wip', output_unit: last ? '' : 'g', ...saved }
}
export function completionQuantityLabel(unit) { return unit === '袋' ? '本次完成袋数' : `本次完成数量（${unit || '-'}）` }
export function suggestedMaterialQty(preview, material, batchOutputQty = preview?.planned_qty || 0) {
  const total = Number(material?.total_qty || 0)
  const planned = Number(preview?.total_planned_qty || 0)
  if (total > 0 && planned > 0) {
    const cumulative = Number(preview?.completed_output_qty || 0) + Number(batchOutputQty || 0)
    const target = weightUnit(material.unit)
      ? Math.round(total * Math.min(1, cumulative / planned) * 1e6) / 1e6
      : Math.ceil(total * Math.min(1, cumulative / planned) - 1e-9)
    return Math.max(0, target - Number(material.consumed_qty || 0))
  }
  const ratio = Number(preview?.planned_qty || 0) > 0 ? Number(batchOutputQty || 0) / Number(preview.planned_qty) : 0
  const required = Number(material?.qty || 0) * Math.max(0, ratio)
  return weightUnit(material.unit) ? Math.round(required * 1e6) / 1e6 : Math.ceil(required - 1e-9)
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
  return input
}
export function completionDraft(preview) {
  const maxOutput = Number.isFinite(Number(preview.max_output_qty)) ? Math.min(Number(preview.planned_qty || 0), Number(preview.max_output_qty)) : Number(preview.planned_qty || 0)
  const draft = {actual_input_qty:suggestedInputQty(preview,maxOutput),actual_output_qty:maxOutput,actual_minutes:preview.actual_minutes || 0,difference_reason:'',finish_operation:maxOutput >= Number(preview.planned_qty || 0),...(preview.draft || {})}
  const savedMaterials = new Map((preview.draft?.materials || []).map(row => [Number(row.reservation_id), row]))
  draft.materials = (preview.materials || []).map(row => ({...row,...(savedMaterials.get(Number(row.reservation_id)) || {}),qty:savedMaterials.has(Number(row.reservation_id)) ? savedMaterials.get(Number(row.reservation_id)).qty : suggestedMaterialQty(preview,row,draft.actual_output_qty)}))
  return draft
}
function quantity(value,unit) {
  if (value === '' || value == null) throw new Error('请填写实际数量')
  const n=Number(value);if(!Number.isFinite(n)||n<0) throw new Error('数量必须大于等于零')
  if(!weightUnit(unit)&&!Number.isInteger(n)) throw new Error(`${unit}数必须是整数`)
  return n
}
export function completionPayload(preview,draft,id) {
  const output=quantity(draft.actual_output_qty,preview.output_unit)
  const finishOperation = draft.finish_operation !== false
  if((output>Number(preview.planned_qty)||finishOperation&&output!==Number(preview.planned_qty))&&!draft.difference_reason?.trim()) throw new Error('请填写与计划不同的原因')
  return {request_id:id,expected_version:preview.version,input_unit:preview.input_unit,output_unit:preview.output_unit,actual_input_qty:quantity(draft.actual_input_qty,preview.input_unit),actual_output_qty:output,actual_minutes:quantity(draft.actual_minutes,'分钟'),difference_reason:draft.difference_reason || '',finish_operation:finishOperation,materials:draft.materials.map(row=>({...row,qty:quantity(row.qty,row.unit)}))}
}
export function completionRetry() {
  let attempt=null,uncertain=false
  return {get uncertain(){return uncertain},begin(payload,makeID){if(!attempt)attempt=JSON.parse(JSON.stringify({...payload,request_id:makeID()}));return attempt},fail(err){uncertain=!!attempt&&(!err.status||err.status>=500);if(!uncertain)attempt=null},clear(){attempt=null;uncertain=false}}
}
export function progressPayload(preview,draft) {
  return {expected_version:preview.version,input_unit:preview.input_unit,output_unit:preview.output_unit,actual_input_qty:Number(draft.actual_input_qty)||0,actual_output_qty:Number(draft.actual_output_qty)||0,actual_minutes:Number(draft.actual_minutes)||0,difference_reason:draft.difference_reason||'',materials:draft.materials.map(row=>({...row,qty:Number(row.qty)||0}))}
}
