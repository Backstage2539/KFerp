import { weightUnit } from './operation-completion.js'
function actual(value, unit) {
  if (value === '' || value == null) throw new Error('请填写现场核实的数量')
  const n = Number(value)
  if (!Number.isFinite(n) || n < 0) throw new Error('数量必须大于等于零')
  if (!weightUnit(unit) && !Number.isInteger(n)) throw new Error(`${unit}数必须为整数`)
  return n
}
export function legacyReviewDraft(row) {
  return { actual_output_qty: '', actual_input_g: 0, reason: '', card_quantities: {}, materials: (row.materials || []).map(m => ({ ...m, qty: '' })) }
}
export function legacyReviewPayload(row, draft, requestID) {
  const output = actual(draft.actual_output_qty, row.output_unit)
  if (!draft.reason?.trim()) throw new Error('请填写核对原因')
  const cards = {}
  if ((row.correction_cards || []).length > 1) {
    for (const card of row.correction_cards) cards[card.id] = actual(draft.card_quantities[card.id], row.output_unit)
    if (Math.abs(Object.values(cards).reduce((a,b) => a+b,0) - output) > 1e-8) throw new Error('来源工序数量合计与核实总量不一致')
  }
  return { work_order_id: row.work_order_id, actual_output_qty: output, actual_input_g: actual(draft.actual_input_g,'g'), materials: draft.materials.map(m => ({...m,qty:actual(m.qty,m.unit)})), card_quantities: cards, reason: draft.reason.trim(), expected_version: row.version, request_id: requestID }
}
export function correctionPayload(record, draft, requestID) {
  const output = actual(draft.quantity, record.unit)
  if (!draft.reason?.trim()) throw new Error('请填写更正原因')
  const cards={}
  if((record.correction_cards||[]).length>1){for(const card of record.correction_cards)cards[card.id]=actual(draft.card_quantities?.[card.id],record.unit);if(Math.abs(Object.values(cards).reduce((a,b)=>a+b,0)-output)>1e-8)throw new Error('来源工序数量合计不一致')}
  return { event_id: Number(record.id || 0), actual_output_qty: output, card_quantities:cards, reason: draft.reason.trim(), expected_version: record.version, request_id: requestID }
}
