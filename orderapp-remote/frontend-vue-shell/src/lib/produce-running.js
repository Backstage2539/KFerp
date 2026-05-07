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
