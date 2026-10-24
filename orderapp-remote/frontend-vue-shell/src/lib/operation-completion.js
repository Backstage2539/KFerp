export function weightUnit(unit) { return ['g','kg','mg','克','千克','公斤','毫克'].includes(String(unit).toLowerCase()) }
export function routeExecution(op = {}, last = false, fallbackKey = '') {
  let saved = {}; try { saved = JSON.parse(op.execution_json || '{}') } catch { /* API validates on save. */ }
  return { version: 1, node_key: fallbackKey, receipt_enabled: last, receipt_warehouse: '', source_warehouse: 'wip', output_unit: last ? '' : 'g', ...saved }
}
export function completionQuantityLabel(unit) { return unit === '袋' ? '本次完成袋数' : `本次完成数量（${unit || '-'}）` }
export function completionDraft(preview) {
  return {actual_input_qty:preview.input_qty || 0,actual_output_qty:preview.planned_qty || 0,actual_minutes:preview.actual_minutes || 0,difference_reason:'',materials:(preview.materials || []).map(row=>({...row})),...(preview.draft || {})}
}
function quantity(value,unit) {
  if (value === '' || value == null) throw new Error('请填写实际数量')
  const n=Number(value);if(!Number.isFinite(n)||n<0) throw new Error('数量必须大于等于零')
  if(!weightUnit(unit)&&!Number.isInteger(n)) throw new Error(`${unit}数必须是整数`)
  return n
}
export function completionPayload(preview,draft,id) {
  const output=quantity(draft.actual_output_qty,preview.output_unit)
  if(output!==Number(preview.planned_qty)&&!draft.difference_reason?.trim()) throw new Error('请填写与计划不同的原因')
  return {request_id:id,expected_version:preview.version,input_unit:preview.input_unit,output_unit:preview.output_unit,actual_input_qty:quantity(draft.actual_input_qty,preview.input_unit),actual_output_qty:output,actual_minutes:quantity(draft.actual_minutes,'分钟'),difference_reason:draft.difference_reason || '',materials:draft.materials.map(row=>({...row,qty:quantity(row.qty,row.unit)}))}
}
export function completionRetry() {
  let attempt=null,uncertain=false
  return {get uncertain(){return uncertain},begin(payload,makeID){if(!attempt)attempt=JSON.parse(JSON.stringify({...payload,request_id:makeID()}));return attempt},fail(err){uncertain=!!attempt&&(!err.status||err.status>=500);if(!uncertain)attempt=null},clear(){attempt=null;uncertain=false}}
}
export function progressPayload(preview,draft) {
  return {expected_version:preview.version,input_unit:preview.input_unit,output_unit:preview.output_unit,actual_input_qty:Number(draft.actual_input_qty)||0,actual_output_qty:Number(draft.actual_output_qty)||0,actual_minutes:Number(draft.actual_minutes)||0,difference_reason:draft.difference_reason||'',materials:draft.materials.map(row=>({...row,qty:Number(row.qty)||0}))}
}
