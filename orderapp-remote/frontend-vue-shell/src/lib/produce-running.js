function toNumber(value) {
  const n = Number(value || 0)
  return Number.isFinite(n) ? n : 0
}

function normalizeYieldRate(value) {
  const n = Number(value || 0)
  return n > 0 ? n : 0.8
}

export function finishedTotalG(row, input) {
  if (Array.isArray(input?.outputs) && input.outputs.length) {
    return input.outputs.reduce((sum, output) => sum + toNumber(output.spec_g) * toNumber(output.finished_units) + toNumber(output.finished_loose_g), 0)
  }
  return toNumber(row?.spec_g) * toNumber(input?.finished_units) + toNumber(input?.finished_loose_g)
}

export function buildFinishInput(row, warehouse = 'finished_goods') {
  const outputs = Array.isArray(row?.outputs)
    ? row.outputs.map((output) => ({
      spec_g: toNumber(output.spec_g),
      finished_units: toNumber(output.plan_units),
      finished_loose_g: toNumber(output.plan_loose_g),
    }))
    : []
  const input = {
    finished_units: toNumber(row?.plan_units),
    finished_loose_g: toNumber(row?.plan_loose_g),
    consumed_input_g: toNumber(row?.input_g),
    partial: false,
    warehouse,
    yield_dirty: false,
  }
  if (outputs.length) input.outputs = outputs
  return input
}

export function markYieldDirty(input) {
  if (input) input.yield_dirty = true
}

export function actualYieldRate(row, input) {
  if (!input?.yield_dirty) return normalizeYieldRate(row?.bom_yield_rate)

  const inputG = toNumber(input.consumed_input_g)
  if (inputG <= 0) return normalizeYieldRate(row?.bom_yield_rate)
  return finishedTotalG(row, input) / inputG
}

export function formatActualYield(row, input) {
  return `${(actualYieldRate(row, input) * 100).toFixed(2)}%`
}

export function buildFinishPanelModel(row, input, mode = 'complete') {
  const partial = mode === 'partial'
  const panelInput = {
    ...(input || {}),
    partial,
  }
  return {
    title: partial ? '部分完成' : '完成生产',
    primaryLabel: partial ? '记录部分完成' : '完成并入库',
    fields: ['投料', '成品件数', '余料', '入库仓', '异常/备注'],
    payload: buildFinishPayload(row, panelInput),
  }
}

export function productionFinishErrorDetail(value) {
  const text = String(value || '').trim()
  const lower = text.toLowerCase()
  if (lower.includes('wip stock insufficient') || text.includes('WIP库存不足') || (lower.includes('wip') && lower.includes('insufficient'))) {
    return {
      reason: 'WIP库存不足',
      affectedObject: cleanErrorAffectedObject(text, ['WIP stock insufficient:', 'WIP库存不足：', 'WIP库存不足:']),
      action: '打开库存作业',
      actionKey: 'stockOperations',
    }
  }
  if (lower.includes('quality hold') || text.includes('质检冻结') || text.includes('冻结')) {
    return {
      reason: '质检冻结',
      affectedObject: cleanErrorAffectedObject(text, ['quality hold:', '质检冻结：', '质检冻结:']),
      action: '打开质检',
      actionKey: 'qualityInspections',
    }
  }
  return {
    reason: text || '操作失败',
    affectedObject: '',
    action: '',
    actionKey: '',
  }
}

function cleanErrorAffectedObject(text, prefixes) {
  let out = String(text || '').trim()
  for (const prefix of prefixes) {
    if (out.toLowerCase().startsWith(prefix.toLowerCase())) {
      out = out.slice(prefix.length).trim()
    }
  }
  return out
}

export function buildFinishPayload(row, input) {
  const outputs = Array.isArray(input?.outputs)
    ? input.outputs.map((output) => ({
      spec_g: toNumber(output.spec_g),
      finished_units: toNumber(output.finished_units),
      finished_loose_g: toNumber(output.finished_loose_g),
    }))
    : []
  const payload = {
    id: toNumber(row?.id),
    finished_units: toNumber(input?.finished_units),
    finished_loose_g: toNumber(input?.finished_loose_g),
    consumed_input_g: toNumber(input?.consumed_input_g),
    partial: !!input?.partial,
    warehouse: input?.warehouse || 'finished_goods',
  }
  if (outputs.length) payload.outputs = outputs
  return payload
}
