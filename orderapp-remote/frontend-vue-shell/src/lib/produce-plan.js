const WEIGHT_UNITS = new Set(['g', 'kg', '克', '千克'])

function normalizeRatioPct(value) {
  const ratio = Number(value || 0)
  if (ratio > 0 && ratio <= 1) return ratio * 100
  return ratio
}

export function normalizedYieldRate(value) {
  const rate = Number(value || 0)
  if (rate <= 0) return 0
  if (rate > 1) return rate / 100
  return rate
}

export function roastExpectedFinishedG(row) {
  const finalInputG = Number(row?.final_input_g || 0)
  const yieldRate = normalizedYieldRate(row?.yield_rate)
  if (finalInputG <= 0 || yieldRate <= 0) return 0
  return Math.round(finalInputG * yieldRate)
}

export function gramsToKgString(value, digits = 2) {
  const grams = Number(value || 0)
  if (grams <= 0) return '0'
  return (grams / 1000).toFixed(digits)
}

export function describeProducePlanRow(row) {
  if (!row || row.production_kind !== 'drip_bag') return []
  const labels = ['挂耳生产']
  const needBags = Number(row.need_bags || row.need_units || 0)
  if (needBags > 0) labels.push(`需求 ${needBags} 袋`)
  const componentShortage = Number(row.finished_product_component_shortage_g || row.upstream_shortage_g || 0)
  if (componentShortage > 0) labels.push(`熟豆组件缺口 ${componentShortage}g`)
  const upstreamDemand = Number(row.upstream_roast_demand_g || 0)
  if (upstreamDemand > 0) labels.push(`上游烘焙需求 ${upstreamDemand}g`)
  return labels
}

export function producePlanKey(productId, specG) {
  return `${productId}-${specG}`
}

function defaultSelectionKey(row) {
  return producePlanKey(row.product_id, row.spec_g)
}

export function insufficientSelectionState(rows, selected, keyForRow = defaultSelectionKey) {
  const keys = (rows || []).map(keyForRow)
  const selectedCount = keys.filter((key) => !!selected?.[key]).length
  const total = keys.length
  return {
    checked: total > 0 && selectedCount === total,
    indeterminate: selectedCount > 0 && selectedCount < total,
    selectedCount,
    total,
  }
}

export function buildInsufficientSelection(rows, checked, keyForRow = defaultSelectionKey) {
  if (!checked) return {}
  return Object.fromEntries((rows || []).map((row) => [keyForRow(row), true]))
}

function toPositiveInteger(value, fallback = 1) {
  const n = Number(value)
  if (!Number.isFinite(n) || n < 1) return fallback
  return Math.round(n)
}

export function syncRoastPlanRow(row, patch = {}) {
  if (!row || typeof row !== 'object') return row

  if (Object.prototype.hasOwnProperty.call(patch, 'machine')) {
    row.machine = String(patch.machine || '').trim()
  }
  if (Object.prototype.hasOwnProperty.call(patch, 'batch_g')) {
    row.batch_g = patch.batch_g
  }
  if (Object.prototype.hasOwnProperty.call(patch, 'batch_count')) {
    row.batch_count = patch.batch_count
  }

  row.batch_g = toPositiveInteger(row.batch_g, 1)
  row.batch_count = toPositiveInteger(row.batch_count, 1)
  row.final_input_g = row.batch_g * row.batch_count
  return row
}

export function normalizeRoastPlans(roastPlans) {
  return (roastPlans || []).map((row) =>
    syncRoastPlanRow({
      ...row,
      machine: String(row?.machine || '').trim(),
      batch_g: Number(row?.batch_g || 0),
      batch_count: Number(row?.batch_count || 0),
      final_input_g: Number(row?.final_input_g || 0),
    }),
  )
}

export function buildFinalInputMap(roastPlans) {
  const out = {}
  for (const row of roastPlans || []) {
    out[row.key] = Number(row.final_input_g || 0)
  }
  return out
}

export function rebuildPlanRows(planRows, roastPlans) {
  const finalInputByKey = buildFinalInputMap(roastPlans)
  return (planRows || []).map((row) => ({
    ...row,
    input_g: finalInputByKey[producePlanKey(row.product_id, row.spec_g)] || Number(row.input_g || 0),
  }))
}

export function buildMaterialSummary(planRows, roastPlans, materialRatios, initialMaterials) {
  const out = new Map()
  const dynamicNames = new Set()
  const availabilityByName = new Map()

  for (const item of initialMaterials || []) {
    availabilityByName.set(`${item.name}::${String(item.unit || '').trim().toLowerCase()}`, item)
  }

  for (const ratio of materialRatios || []) {
    if (WEIGHT_UNITS.has(String(ratio.material_unit || '').trim().toLowerCase())) {
      dynamicNames.add(ratio.material_name)
    }
  }

  for (const item of initialMaterials || []) {
    if (dynamicNames.has(item.name)) continue
    out.set(`${item.name}::${item.unit}`, { ...item, qty: Number(item.qty || 0) })
  }

  const roastByKey = new Map((roastPlans || []).map((row) => [row.key, row]))
  for (const ratio of materialRatios || []) {
    const unit = String(ratio.material_unit || '').trim() || 'g'
    const normalized = unit.toLowerCase()
    if (!WEIGHT_UNITS.has(normalized)) continue

    const roast = roastByKey.get(ratio.key)
    const finalInputG = Number(roast?.final_input_g || 0)
    if (finalInputG <= 0) continue

    let qty = 0
    if (normalized === 'kg' || normalized === '千克') {
      qty = Math.ceil((finalInputG * normalizeRatioPct(ratio.ratio_pct)) / 100 / 1000)
    } else {
      qty = Math.ceil((finalInputG * normalizeRatioPct(ratio.ratio_pct)) / 100)
    }
    if (qty <= 0) continue

    const mapKey = `${ratio.material_name}::${unit}`
    const initial = availabilityByName.get(`${ratio.material_name}::${normalized}`) || {}
    const existing = out.get(mapKey) || {
      ...initial,
      name: ratio.material_name,
      unit,
      qty: 0,
    }
    existing.qty += qty
    out.set(mapKey, existing)
  }

  return [...out.values()].sort((a, b) => {
    if (a.unit !== b.unit) return String(a.unit).localeCompare(String(b.unit), 'zh-CN')
    return String(a.name).localeCompare(String(b.name), 'zh-CN')
  })
}

export function buildStartPayload(filters, selectedKeys, roastPlans, planRows) {
  const finalInputByKey = buildFinalInputMap(roastPlans)
  const fallbackByKey = Object.fromEntries((planRows || []).map((row) => [producePlanKey(row.product_id, row.spec_g), Number(row.input_g || 0)]))
  const input_by_key = {}
  for (const key of selectedKeys) {
    input_by_key[key] = finalInputByKey[key] || fallbackByKey[key] || 0
  }
  return {
    from: filters.from || '',
    to: filters.to || '',
    customer_id: Number(filters.customer_id || 0),
    selected: selectedKeys,
    input_by_key,
  }
}

export function buildProductionPlanCreatePayload(filters, selectedKeys, roastPlans, planRows) {
  return {
    ...buildStartPayload(filters || {}, selectedKeys || [], roastPlans || [], planRows || []),
    source_type: 'erp_order',
  }
}

const PRODUCTION_PLAN_STATUS = {
  draft: { label: '草稿', tone: 'draft' },
  submitted: { label: '已提交工单', tone: 'submitted' },
  in_progress: { label: '生产中', tone: 'in-progress' },
  completed: { label: '已完成', tone: 'completed' },
  cancelled: { label: '已取消', tone: 'cancelled' },
}

const PRODUCTION_PLAN_TIME_FIELDS = new Set(['created_at', 'submitted_at', 'completed_at'])

export function productionPlanStatusLabel(status) {
  const key = String(status || '').trim()
  return PRODUCTION_PLAN_STATUS[key]?.label || key || '-'
}

export function productionPlanStatusTone(status) {
  const key = String(status || '').trim()
  return PRODUCTION_PLAN_STATUS[key]?.tone || 'unknown'
}

export function productionPlanSelectable(plan) {
  return Number(plan?.id || 0) > 0 && String(plan?.status || '').trim() === 'draft'
}

function productionPlanSelectionKeys(plans) {
  return (plans || []).filter(productionPlanSelectable).map((plan) => String(Number(plan.id)))
}

export function productionPlanSelectionState(plans, selected) {
  const keys = productionPlanSelectionKeys(plans)
  const selectedCount = keys.filter((key) => !!selected?.[key]).length
  const total = keys.length
  return {
    checked: total > 0 && selectedCount === total,
    indeterminate: selectedCount > 0 && selectedCount < total,
    selectedCount,
    total,
  }
}

export function buildProductionPlanSelection(plans, checked) {
  if (!checked) return {}
  return Object.fromEntries(productionPlanSelectionKeys(plans).map((key) => [key, true]))
}

export function buildProductionPlanBatchSubmitPayload(selected) {
  const ids = Object.keys(selected || {})
    .filter((key) => !!selected[key])
    .map((key) => Number(key))
    .filter((id) => Number.isInteger(id) && id > 0)
  return { ids }
}

export function productionPlanBatchSubmitEndpoint() {
  return '/api/production-plans/submit'
}

export function buildProductionPlanListQuery(filters = {}) {
  const params = new URLSearchParams()
  const status = String(filters.status || '').trim()
  const timeField = PRODUCTION_PLAN_TIME_FIELDS.has(String(filters.time_field || '').trim())
    ? String(filters.time_field || '').trim()
    : 'created_at'
  const from = String(filters.from || '').trim()
  const to = String(filters.to || '').trim()
  let limit = Number(filters.limit || 50)
  if (!Number.isFinite(limit) || limit <= 0) limit = 50
  if (limit > 500) limit = 500

  if (status) params.set('status', status)
  params.set('time_field', timeField)
  if (from) params.set('from', from)
  if (to) params.set('to', to)
  params.set('limit', String(Math.round(limit)))
  return `/api/production-plans?${params.toString()}`
}

export function productionPlanSubmitEndpoint(plan) {
  const id = Number(plan?.id || 0)
  if (id <= 0) return ''
  return `/api/production-plans/${id}/submit`
}
