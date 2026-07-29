export const productionTopNavItems = [
  { key: 'productionOverview', label: '生产视图' },
  { key: 'workstationView', label: '工位视图' },
  { key: 'productionFlow', label: '生产流程' },
  { key: 'produceRunning', label: '生产中' },
  { key: 'produceLogs', label: '日志' },
]

export function navItemsWithProductionBadges(items = productionTopNavItems, badges = {}) {
  return (items || []).map((item) => ({
    ...item,
    badge: productionNavBadgeText(badges[item.key]),
  }))
}

function productionNavBadgeText(badge) {
  if (!badge) return ''
  const pending = Number(badge.pending || 0)
  const blocked = Number(badge.blocked || 0)
  const running = Number(badge.running || 0)
  return `待${pending} 阻${blocked} 中${running}`
}

export function stockOperationContextParams(task = {}) {
  const params = { tab: 'stockEntries', action: 'issue', return_source: 'work_order' }
  for (const key of ['work_order_id', 'job_card_id', 'running_item_id', 'material_id', 'shortage_g']) {
    const value = Number(task?.[key] || 0)
    if (value > 0) params[key] = value
  }
  return params
}

const runningStatuses = new Set(['running'])
const pendingStatuses = new Set(['pending', 'ready', 'released'])

export function workstationTaskSections(tasks = []) {
  const groups = new Map()
  for (const task of tasks || []) {
    const workstation = task.workstation || task.work_center || '未分配工位'
    if (!groups.has(workstation)) groups.set(workstation, [])
    groups.get(workstation).push(task)
  }
  return Array.from(groups.entries()).map(([workstation, rows]) => {
    const sorted = [...rows].sort(compareProductionTasks)
    const currentTask = sorted.find((task) => runningStatuses.has(task.status) || task.is_blocked || task.blocking_reason) || sorted[0] || null
    const nextTask = sorted.find((task) => pendingStatuses.has(task.status) && task !== currentTask) || null
    const blocked = sorted.find((task) => task.blocking_reason)
    return {
      workstation,
      tasks: sorted,
      currentTask,
      nextTask,
      blockingReason: blocked?.blocking_reason || '',
    }
  }).sort((a, b) => {
    const aBlocked = a.blockingReason ? 1 : 0
    const bBlocked = b.blockingReason ? 1 : 0
    if (aBlocked !== bBlocked) return bBlocked - aBlocked
    return a.workstation.localeCompare(b.workstation, 'zh-Hans-CN')
  })
}

export function compareProductionTasks(a, b) {
  const priority = Number(b?.priority || 0) - Number(a?.priority || 0)
  if (priority !== 0) return priority
  const aStart = a?.planned_start_at || '9999-99-99 99:99'
  const bStart = b?.planned_start_at || '9999-99-99 99:99'
  if (aStart !== bStart) return aStart.localeCompare(bStart)
  return Number(a?.job_card_id || a?.work_order_id || 0) - Number(b?.job_card_id || b?.work_order_id || 0)
}

export function productionTaskActionEndpoint(task, action) {
  const id = Number(task?.job_card_id || 0)
  if (!id) return ''
  switch (action) {
    case 'start':
    case 'pause':
    case 'resume':
    case 'complete':
      return `/api/job-cards/${id}/${action}`
    case 'report_exception':
      return `/api/production/workstation/tasks/${id}/exception`
    case 'material_call':
      return `/api/production/workstation/tasks/${id}/material-call`
    default:
      return ''
  }
}

export function workstationVisibleActions(task = {}) {
  return (task.available_actions || []).filter((action) => action !== 'partial_finish')
}

function completionQuantity(value, label) {
  const quantity = Number(value ?? 0)
  if (!Number.isFinite(quantity) || quantity < 0) {
    throw new Error(`${label}必须为大于等于 0 的数字`)
  }
  return quantity
}

function completionCount(value, label) {
  const quantity = completionQuantity(value, label)
  if (!Number.isInteger(quantity)) throw new Error(`${label}必须为整数`)
  return quantity
}

export function productionCompletionOutputQty({
  actualOutputQty = 0,
  finishedUnits = 0,
  inventoryQtyPerSalesUnit = 0,
} = {}) {
  const outputQty = completionQuantity(actualOutputQty, '实际产出')
  const units = completionCount(finishedUnits, '成品件数')
  if (outputQty > 0 && units > 0) {
    throw new Error('实际产出和成品件数只能填写一项')
  }
  if (outputQty <= 0 && units <= 0) {
    throw new Error('请填写实际产出或成品件数')
  }
  if (units <= 0) return outputQty
  const quantityPerUnit = completionQuantity(inventoryQtyPerSalesUnit, '每件库存数量')
  if (quantityPerUnit <= 0) {
    throw new Error('当前规格缺少库存单位换算，不能按成品件数计算实际产出')
  }
  return units * quantityPerUnit
}

export function productionCompletionMetrics({
  inventoryUnit = '',
  leftoverQty = 0,
  note = '',
  warehouse = '',
  finishedUnits = 0,
} = {}) {
  const normalizedUnit = String(inventoryUnit || '').trim()
  if (!normalizedUnit) throw new Error('缺少库存单位，无法保存工序实际记录')
  return {
    quantity_basis: 'inventory_unit',
    inventory_unit: normalizedUnit,
    leftover_qty: completionQuantity(leftoverQty, '余料'),
    note: String(note || '').trim(),
    warehouse: String(warehouse || '').trim(),
    finished_units: completionCount(finishedUnits, '成品件数'),
  }
}

export function productionTaskActionErrorMessage(error, action = '') {
  const raw = String(error?.message || '').trim()
  const normalized = raw.toLowerCase()
  const actionLabel = {
    start: '开始',
    pause: '暂停',
    resume: '继续',
    complete: '完成本工序',
    report_exception: '上报异常',
    material_call: '呼叫补料',
  }[action] || '操作'
  if (
    raw.startsWith('实际产出')
    || raw.startsWith('成品件数')
    || raw.startsWith('请填写实际产出')
    || raw.startsWith('当前规格缺少库存单位换算')
    || raw.startsWith('缺少库存单位')
    || raw.startsWith('余料')
  ) {
    return raw
  }
  if (
    normalized.includes('permission denied')
    || normalized.includes('forbidden')
    || normalized.includes('unauthorized')
  ) {
    return '当前账号没有执行此操作的权限，请联系管理员'
  }
  if (normalized.includes('not found')) return '工序卡不存在或已失效，请刷新后重试'
  if (
    normalized.includes('invalid job card status transition')
    || normalized.includes('invalid job card action')
    || normalized.includes('invalid status')
    || normalized.includes('status transition')
  ) {
    return `当前工序状态不允许${actionLabel}，请刷新后按最新状态操作`
  }
  if (normalized.includes('work order must be running before job card start')) {
    return '请先从工单执行枢纽开始生产，再在工位开始本工序'
  }
  if (normalized.includes('work order must be released')) return '工单必须先下达后才能执行工序'
  if (
    normalized.includes('work order must be running')
    || normalized.includes('work order is not running')
  ) {
    return '工单尚未开始生产，请先从执行枢纽开始生产'
  }
  if (
    normalized.includes('actual input')
    || normalized.includes('actual output')
    || normalized.includes('input/output')
    || normalized.includes('input and output')
  ) {
    return '实际投入或实际产出数量不正确，请检查后重试'
  }
  if (normalized.includes('quantity') || normalized.includes('qty')) {
    return '数量不正确，请检查后重试'
  }
  if (normalized.includes('operator required')) return '当前登录人缺少操作员信息，请联系管理员补充后重试'
  return `${actionLabel}失败，请稍后重试；如持续失败请联系管理员`
}

export function taskTitle(task) {
  if (!task) return '-'
  const op = task.operation || ''
  const product = task.product_name || task.work_order_no || ''
  if (op && product) return `${op} / ${product}`
  return product || op || '-'
}
