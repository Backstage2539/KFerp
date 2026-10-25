export function weightUnit(unit) { return ['g','kg','mg','克','千克','公斤','毫克'].includes(String(unit).toLowerCase()) }
export function routeExecution(op = {}, last = false, fallbackKey = '') {
  let saved = {}; try { saved = JSON.parse(op.execution_json || '{}') } catch { /* API validates on save. */ }
  return { version: 1, node_key: fallbackKey, receipt_enabled: last, receipt_warehouse: '', source_warehouse: 'wip', output_unit: last ? '' : 'g', ...saved }
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
  return {actual_output_qty:Number(preview.planned_qty || 0)}
}
function quantity(value,unit) {
  if (value === '' || value == null) throw new Error('请填写实际数量')
  const n=Number(value);if(!Number.isFinite(n)||n<0) throw new Error('数量必须大于等于零')
  if(!weightUnit(unit)&&!Number.isInteger(n)) throw new Error(`${unit}数必须是整数`)
  return n
}
export function completionPayload(preview,draft,id) {
  const output=quantity(draft.actual_output_qty,preview.output_unit)
  return {request_id:id,expected_version:preview.version,output_unit:preview.output_unit,actual_output_qty:output,finish_operation:true}
}
export function completionRetry() {
  let attempt=null,uncertain=false
  return {get uncertain(){return uncertain},begin(payload,makeID){if(!attempt)attempt=JSON.parse(JSON.stringify({...payload,request_id:makeID()}));return attempt},fail(err){uncertain=!!attempt&&(!err.status||err.status>=500);if(!uncertain)attempt=null},clear(){attempt=null;uncertain=false}}
}
export function progressPayload(preview,draft) {
  return {expected_version:preview.version,output_unit:preview.output_unit,actual_output_qty:Number(draft.actual_output_qty)||0}
}
