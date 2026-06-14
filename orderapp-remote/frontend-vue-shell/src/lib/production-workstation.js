export const productionTopNavItems = [
  { key: 'productionOverview', label: '生产视图' },
  { key: 'workstationView', label: '工位视图' },
  { key: 'producePlan', label: '生产计划' },
  { key: 'workOrders', label: '工单' },
  { key: 'jobCards', label: '工序卡' },
  { key: 'qualityInspections', label: '质检' },
  { key: 'produceLogs', label: '日志' },
  { key: 'productionCosts', label: '成本' },
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
  const params = { tab: 'wip' }
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

export function taskTitle(task) {
  if (!task) return '-'
  const op = task.operation || ''
  const product = task.product_name || task.work_order_no || ''
  if (op && product) return `${op} / ${product}`
  return product || op || '-'
}
