export const productionTopNavItems = [
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
    if (['completed', 'cancelled'].includes(String(task.work_order_status || '').toLowerCase())) continue
    const workstation = task.workstation || task.work_center || '未分配工位'
    if (!groups.has(workstation)) groups.set(workstation, [])
    groups.get(workstation).push(task)
  }
  return Array.from(groups.entries()).map(([workstation, rows]) => {
    const sorted = [...rows].sort(compareProductionTasks)
    const currentTask = sorted.find((task) => runningStatuses.has(task.status)) || sorted.find((task) => task.is_blocked || task.blocking_reason) || sorted[0] || null
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

// Combine packaging cards only when the backend has asserted that their
// frozen production identity is compatible. Sales customer and BOM variant
// are intentionally not grouping keys; actual material identity and owner
// have already been accounted for by packaging_group_key.
export function workstationPackagingGroups(tasks = []) {
  const groups = new Map()
  for (const task of tasks || []) {
    const key = String(task?.packaging_group_key || '').trim()
    if (!key || !/包装|分装|装盒|装袋/.test(String(task?.operation || ''))) continue
    if (['completed', 'cancelled'].includes(String(task?.status || '').toLowerCase())) continue
    if (['completed', 'cancelled'].includes(String(task.work_order_status || '').toLowerCase())) continue
    if (!groups.has(key)) groups.set(key, [])
    groups.get(key).push(task)
  }
  return Array.from(groups.entries()).filter(([, rows]) => rows.length > 1).map(([key, rows]) => {
    const sorted = [...rows].sort(compareProductionTasks)
    const materialMap = new Map()
    for (const task of sorted) {
      for (const material of task.material_readiness || []) {
        const identity = workstationPackagingMaterialIdentity(material)
        let aggregate = materialMap.get(identity)
        if (!aggregate) {
          aggregate = { ...material, identity, required_g: 0, wip_available_g: 0, shortage_g: 0, required_units: 0, wip_available_units: 0, shortage_units: 0, source_available_g: 0, source_available_units: 0 }
          materialMap.set(identity, aggregate)
        }
        for (const field of ['required_g', 'wip_available_g', 'shortage_g', 'required_units', 'wip_available_units', 'shortage_units']) {
          aggregate[field] += Number(material[field] || 0)
        }
      }
    }
    return {
      key,
      tasks: sorted,
      planned_output_qty: sorted.reduce((sum, task) => sum + Number(task.planned_output_inventory_qty || 0), 0),
      completed_output_qty: sorted.reduce((sum, task) => sum + Number(task.actual_output_qty || 0), 0),
      remaining_output_qty: sorted.reduce((sum, task) => sum + Math.max(0, Number(task.planned_output_inventory_qty || 0) - Number(task.actual_output_qty || 0)), 0),
      product_name: sorted[0]?.product_name || '',
      spec_g: Number(sorted[0]?.spec_g || 0),
      inventory_unit: sorted[0]?.inventory_unit || '',
      target_warehouse_name: sorted[0]?.target_warehouse_name || sorted[0]?.target_warehouse || '',
      materials: Array.from(materialMap.values()),
    }
  }).sort((a, b) => compareProductionTasks(a.tasks[0], b.tasks[0]))
}

export function workstationPackagingMaterialIdentity(material = {}) {
  return [material.item_type || 'material', material.item_id || material.material_id || 0, material.bom_spec_id || 0, material.owner_customer_id || 0, material.unit || '', material.spec_g || 0].join(':')
}

export function workstationGroupOutputAllocations(items = [], actualQty = 0) {
  let left = Math.max(0, Number(actualQty || 0))
  const allocations = (items || []).map(item => {
    const qty = Math.min(Math.max(0, Number(item.remaining_qty || 0)), left)
    left -= qty
    return { ...item, qty }
  })
  if (left > 0 && allocations.length) {
    const last = allocations.length - 1
    allocations[last] = { ...allocations[last], qty: allocations[last].qty + left }
  }
  return allocations
}

export function workstationGroupMaterialSuggestions(preview = {}, actualOutputQty = 0) {
  const allocations = new Map(workstationGroupOutputAllocations(preview.items, actualOutputQty).map(item => [Number(item.job_card_id), item.qty]))
  const items = new Map((preview.items || []).map(item => [Number(item.job_card_id), item]))
  const factor = (unit) => ({ g: 1, 克: 1, kg: 1000, 千克: 1000, 公斤: 1000, mg: 0.001, 毫克: 0.001 })[String(unit || '').toLowerCase()] || 0
  return (preview.materials || []).map(material => {
    let qty = 0
    for (const source of material.sources || []) {
      const cardID = Number(source.job_card_id || 0)
      const remaining = Number(items.get(cardID)?.remaining_qty || 0)
      if (remaining <= 0) continue
      const sourceQty = Number(source.qty || 0) * Number(allocations.get(cardID) || 0) / remaining
      const unitFactor = factor(source.unit)
      qty += unitFactor > 0 ? Math.round(sourceQty * unitFactor) / unitFactor : Math.ceil(sourceQty - 1e-9)
    }
    return { key: material.key, qty }
  })
}

export function workstationGroupCompletionValidation({ outputQty, maxOutputQty, remainingQty, outputUnit = '袋', finishGroup = false, differenceReason = '', materials = [] } = {}) {
  const output = Number(outputQty)
  const maxOutput = Number(maxOutputQty ?? remainingQty ?? 0)
  if (!Number.isFinite(output) || output <= 0 || (operationUnitFactor(outputUnit) === 0 && !Number.isInteger(output))) return '本次完成数量必须为正数；袋、盒等计数单位须填整数'
  if (output > maxOutput + 1e-8) return `当前物料最多支持完成 ${quantityText(maxOutput)} ${outputUnit}`
  for (const material of materials || []) {
    const quantity = Number(material?.qty)
    const available = Number(material?.available_qty)
    if (!Number.isFinite(quantity) || quantity < 0 || (operationUnitFactor(material?.unit) === 0 && !Number.isInteger(quantity))) return `${material?.name || '物料'}数量无效，请检查单位和数量`
    if (Number.isFinite(available) && quantity > available + 1e-8) return `${material?.name || '物料'}用量超过可用库存 ${quantityText(available)} ${material?.unit || ''}`.trim()
  }
  if ((finishGroup || output > Number(remainingQty || 0)) && !String(differenceReason || '').trim()) return '请填写数量差异原因'
  return ''
}

function operationUnitFactor(unit) {
  return ({ g: 1, 克: 1, kg: 1000, 千克: 1000, 公斤: 1000, mg: 0.001, 毫克: 0.001 })[String(unit || '').toLowerCase()] || 0
}

function quantityText(value) {
  const quantity = Number(value || 0)
  if (!Number.isFinite(quantity)) return '0'
  return quantity.toLocaleString('zh-CN', { maximumFractionDigits: 3 })
}

export function taskQuantityLines(task = {}) {
  if (task.has_predecessor) {
    if (!task.predecessor_completed) return ['前序工序未完成，完成后交接']
    return [`前序实际产出 ${quantityText(task.predecessor_output_qty)} ${task.inventory_unit || ''}`.trim(), '本工序按实际交接量处理']
  }
  const lines = []
  const operation = String(task.operation || '')
  const unit = String(task.inventory_unit || '').trim()
  const inputQuantity = Number(task.planned_input_inventory_qty || 0)
  const totalInput = Number(task.operation_total_input_qty || 0)
  const outputTotal = Number(task.planned_output_inventory_qty || 0)
  const batchOutput = totalInput > 0 ? outputTotal * Number(task.planned_input_qty || 0) / totalInput : 0
  const packaging = /包装|分装|装盒|装袋/.test(operation)
  if (Number(task.spec_g || 0) > 0) lines.push(`规格 ${quantityText(task.spec_g)}g`)
  if (packaging && batchOutput > 0 && unit) {
    lines.push(`本批生产 ${quantityText(batchOutput)} ${unit}`)
    return lines
  }
  if (inputQuantity > 0 && unit) lines.push(`本批投料 ${quantityText(inputQuantity)} ${unit}`)
  if (batchOutput > 0 && unit) lines.push(`目标产出 ${quantityText(batchOutput)} ${unit}`)
  if (!lines.length) lines.push('本批数量待核对')
  return lines
}

// A multi-batch operation records each task's actual input; never prefill the
// whole preceding operation's output into every batch.
export function taskDefaultActualInput(task = {}) {
  if (Number(task.actual_input_qty || 0) > 0) return Number(task.actual_input_qty)
  if (task.has_predecessor) {
    return task.predecessor_completed && Number(task.batch_count || 0) === 1
      ? Number(task.predecessor_output_qty || 0) : 0
  }
  return Number(task.planned_input_inventory_qty || 0)
}

export function materialReadinessState(task = {}) {
  const explicit = String(task.material_readiness_state || '').trim()
  if (['ready', 'shortage', 'unknown'].includes(explicit)) return explicit
  if (task.material_data_complete !== true) return 'unknown'
  return (task.material_readiness || []).some((row) => Number(row.shortage_g || 0) > 0 || Number(row.shortage_units || 0) > 0) ? 'shortage' : 'ready'
}

export function workstationCanOpenIssue(task = {}) {
  return String(task.stock_action || '') === 'issue'
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
    case 'claim':
      return `/api/production/workstation/tasks/${id}/claim`
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
  finishedUnits = 0,
} = {}) {
  const normalizedUnit = String(inventoryUnit || '').trim()
  if (!normalizedUnit) throw new Error('缺少库存单位，无法保存工序实际记录')
  return {
    quantity_basis: 'inventory_unit',
    inventory_unit: normalizedUnit,
    leftover_qty: completionQuantity(leftoverQty, '余料'),
    note: String(note || '').trim(),
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
  if ([
    '本任务',
    '前序',
    '后续工序',
    '工单',
    '质检',
    '负责人',
    '执行人',
    '工位',
  ].some((prefix) => raw.startsWith(prefix))) {
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
    return '本任务暂不能开始，请刷新任务状态后重试'
  }
  if (normalized.includes('work order must be released')) return '工单必须先下达后才能执行工序'
  if (
    normalized.includes('work order must be running')
    || normalized.includes('work order is not running')
  ) {
    return '本任务暂不能开始，请刷新任务状态后重试'
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
  const code = String(error?.code || '').trim()
    || (/(network|failed to fetch|load failed)/i.test(raw) ? 'NETWORK' : '')
    || (Number(error?.status || 0) > 0 ? `HTTP-${Number(error.status)}` : 'CLIENT')
  return `${actionLabel}失败（错误编号：${code}），请稍后重试；如持续失败请联系管理员`
}

export function taskTitle(task) {
  if (!task) return '-'
  const op = task.operation || ''
  const product = task.product_name || task.work_order_no || ''
  if (op && product) return `${op} / ${product}`
  return product || op || '-'
}
