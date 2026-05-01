function toNumber(value) {
  const n = Number(value || 0)
  return Number.isFinite(n) ? n : 0
}

function normalizeYieldRate(value) {
  const n = Number(value || 0)
  return n > 0 ? n : 0.8
}

export function finishedTotalG(row, input) {
  return toNumber(row?.spec_g) * toNumber(input?.finished_units) + toNumber(input?.finished_loose_g)
}

export function buildFinishInput(row, warehouse = 'finished_goods') {
  return {
    finished_units: toNumber(row?.plan_units),
    finished_loose_g: toNumber(row?.plan_loose_g),
    consumed_input_g: toNumber(row?.input_g),
    partial: false,
    warehouse,
    yield_dirty: false,
  }
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
  return {
    id: toNumber(row?.id),
    finished_units: toNumber(input?.finished_units),
    finished_loose_g: toNumber(input?.finished_loose_g),
    consumed_input_g: toNumber(input?.consumed_input_g),
    partial: !!input?.partial,
    warehouse: input?.warehouse || 'finished_goods',
  }
}
