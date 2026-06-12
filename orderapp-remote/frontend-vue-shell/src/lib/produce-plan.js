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

export function buildProductionPlanCreatePayload(filters, selectedKeys) {
  filters = filters || {}
  selectedKeys = selectedKeys || []
  return {
    from: filters.from || '',
    to: filters.to || '',
    customer_id: Number(filters.customer_id || 0),
    selected: selectedKeys,
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

export function buildCurrentProductionPlanSubmitPayload(plan) {
  const id = Number(plan?.id || 0)
  if (!Number.isInteger(id) || id <= 0) return { ids: [] }
  return { ids: [id] }
}

export function productionPlanBatchSubmitEndpoint() {
  return '/api/production-plans/submit'
}

export function productionPlanDetailEndpoint(plan) {
  const id = Number(plan?.id || 0)
  if (id <= 0) return ''
  return `/api/production-plans/${id}`
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
