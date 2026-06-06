export const FACTORY_VIEW_CONTEXT = 'factory'
export const CUSTOMER_VIEW_CONTEXT = 'customer'
export const ORDER_VIEW_CONTEXT = 'order'
export const EXTERNAL_CUSTOMER_VIEW_CONTEXT = 'external_customer'

export const FACTORY_WORKSPACE_MODE = FACTORY_VIEW_CONTEXT
export const CUSTOMER_WORKSPACE_MODE = CUSTOMER_VIEW_CONTEXT

const customerWorkspaceSpec = [
  {
    id: 'customerAccount',
    name: '客户账户',
    keys: ['customerFulfillment', 'order', 'orders', 'warehouseInventory'],
  },
  {
    id: 'customerGoods',
    name: '客户商品与配方',
    keys: ['productMaster', 'customerProductAliases', 'productPriceManagement', 'costing', 'bom'],
  },
  {
    id: 'customerFinance',
    name: '客户财务',
    keys: ['financeExpenses'],
  },
]

const factoryHiddenKeys = new Set(['customerFulfillment'])
const customerWorkspaceHiddenKeys = new Set([
  'workspaceModeManual',
  'productionManual',
  'productionAcceptance',
  'producePlan',
  'produceRunning',
  'workOrders',
  'jobCards',
  'qualityInspections',
  'produceLogs',
  'productionCosts',
])

export function normalizeViewContextType(value) {
  switch (String(value || '').trim()) {
    case CUSTOMER_VIEW_CONTEXT:
      return CUSTOMER_VIEW_CONTEXT
    case ORDER_VIEW_CONTEXT:
      return ORDER_VIEW_CONTEXT
    case EXTERNAL_CUSTOMER_VIEW_CONTEXT:
      return EXTERNAL_CUSTOMER_VIEW_CONTEXT
    default:
      return FACTORY_VIEW_CONTEXT
  }
}

function numberValue(...values) {
  for (const value of values) {
    const next = Number(value || 0)
    if (Number.isFinite(next) && next > 0) return next
  }
  return 0
}

function stringValue(...values) {
  for (const value of values) {
    const next = String(value || '').trim()
    if (next) return next
  }
  return ''
}

export function normalizeViewContext(value = {}) {
  if (typeof value === 'string') {
    return { type: normalizeViewContextType(value) }
  }
  const rawType = value.type || value.context_type || value.view_context || value.workspace
  const type = normalizeViewContextType(rawType)
  if (type === ORDER_VIEW_CONTEXT) {
    const orderID = numberValue(value.orderID, value.order_id, value.id)
    const customerID = numberValue(value.customerID, value.customer_id)
    return {
      type,
      ...(orderID ? { orderID } : {}),
      ...(stringValue(value.orderNo, value.order_no) ? { orderNo: stringValue(value.orderNo, value.order_no) } : {}),
      ...(customerID ? { customerID } : {}),
      ...(stringValue(value.customerName, value.customer_name) ? { customerName: stringValue(value.customerName, value.customer_name) } : {}),
    }
  }
  if (type === CUSTOMER_VIEW_CONTEXT || type === EXTERNAL_CUSTOMER_VIEW_CONTEXT) {
    const customerID = numberValue(value.customerID, value.customer_id, value.id)
    return {
      type,
      ...(customerID ? { customerID } : {}),
      ...(stringValue(value.customerName, value.customer_name, value.name) ? { customerName: stringValue(value.customerName, value.customer_name, value.name) } : {}),
    }
  }
  return { type: FACTORY_VIEW_CONTEXT }
}

function contextFromSearchParams(params, fallback = null) {
  const canonicalType = stringValue(params.get('view_context'), params.get('context'), params.get('viewContext'))
  const legacyWorkspace = stringValue(params.get('workspace'))
  const orderID = numberValue(params.get('order_id'))
  const orderNo = stringValue(params.get('order_no'))
  const customerID = numberValue(params.get('customer_id'))
  const customerName = stringValue(params.get('customer_name'))
  if (canonicalType === ORDER_VIEW_CONTEXT || orderID || orderNo) {
    return normalizeViewContext({
      type: ORDER_VIEW_CONTEXT,
      orderID,
      orderNo,
      customerID,
      customerName,
    })
  }
  if (canonicalType === CUSTOMER_VIEW_CONTEXT || legacyWorkspace === CUSTOMER_WORKSPACE_MODE) {
    return normalizeViewContext({
      type: CUSTOMER_VIEW_CONTEXT,
      customerID,
      customerName,
    })
  }
  if (canonicalType === EXTERNAL_CUSTOMER_VIEW_CONTEXT) {
    return normalizeViewContext({
      type: EXTERNAL_CUSTOMER_VIEW_CONTEXT,
      customerID,
      customerName,
    })
  }
  return normalizeViewContext(fallback || { type: FACTORY_VIEW_CONTEXT })
}

export function viewContextFromURL(urlLike, fallback = null) {
  try {
    const url = typeof urlLike === 'string' ? new URL(urlLike, 'https://kferp.local') : urlLike
    return contextFromSearchParams(url.searchParams, fallback)
  } catch {
    return normalizeViewContext(fallback || { type: FACTORY_VIEW_CONTEXT })
  }
}

export function viewContextToURLParams(context) {
  const ctx = normalizeViewContext(context)
  if (ctx.type === CUSTOMER_VIEW_CONTEXT || ctx.type === EXTERNAL_CUSTOMER_VIEW_CONTEXT) {
    const out = {
      view_context: ctx.type === EXTERNAL_CUSTOMER_VIEW_CONTEXT ? EXTERNAL_CUSTOMER_VIEW_CONTEXT : CUSTOMER_VIEW_CONTEXT,
      workspace: CUSTOMER_WORKSPACE_MODE,
    }
    if (ctx.customerID) out.customer_id = String(ctx.customerID)
    return out
  }
  if (ctx.type === ORDER_VIEW_CONTEXT) {
    const out = { view_context: ORDER_VIEW_CONTEXT }
    if (ctx.orderID) out.order_id = String(ctx.orderID)
    if (ctx.orderNo) out.order_no = ctx.orderNo
    if (ctx.customerID) out.customer_id = String(ctx.customerID)
    return out
  }
  return {}
}

export function legacyWorkspaceModeForViewContext(context) {
  const type = normalizeViewContext(context).type
  return type === CUSTOMER_VIEW_CONTEXT || type === ORDER_VIEW_CONTEXT || type === EXTERNAL_CUSTOMER_VIEW_CONTEXT
    ? CUSTOMER_WORKSPACE_MODE
    : FACTORY_WORKSPACE_MODE
}

export function customerIDForViewContext(context) {
  const ctx = normalizeViewContext(context)
  return numberValue(ctx.customerID)
}

export function orderIDForViewContext(context) {
  const ctx = normalizeViewContext(context)
  return ctx.type === ORDER_VIEW_CONTEXT ? numberValue(ctx.orderID) : 0
}

export function isCustomerLikeViewContext(context) {
  return legacyWorkspaceModeForViewContext(context) === CUSTOMER_WORKSPACE_MODE
}

export function viewContextViewParams(params = {}, context = { type: FACTORY_VIEW_CONTEXT }) {
  const out = { ...(params || {}) }
  const ctx = normalizeViewContext(context)
  const customerID = customerIDForViewContext(ctx)
  if (customerID > 0) out.customer_id = String(customerID)
  if (ctx.type === ORDER_VIEW_CONTEXT) {
    if (ctx.orderID) out.order_id = String(ctx.orderID)
    if (ctx.orderNo) out.order_no = ctx.orderNo
  }
  return out
}

function itemByKey(groups, key) {
  for (const group of groups || []) {
    const item = (group.items || []).find((row) => row.key === key)
    if (item) return { ...item }
  }
  return null
}

export function menuGroupsForViewContext(groups, context = { type: FACTORY_VIEW_CONTEXT }) {
  if (!isCustomerLikeViewContext(context)) {
    return (groups || [])
      .map((group) => ({
        ...group,
        items: (group.items || []).filter((item) => !factoryHiddenKeys.has(item.key)),
      }))
      .filter((group) => group.items.length > 0)
  }
  return customerWorkspaceSpec
    .map((group) => ({
      id: group.id,
      name: group.name,
      items: group.keys
        .filter((key) => !customerWorkspaceHiddenKeys.has(key))
        .map((key) => itemByKey(groups, key))
        .filter(Boolean),
    }))
    .filter((group) => group.items.length > 0)
}

export function currentViewLabel(context) {
  const ctx = normalizeViewContext(context)
  if (ctx.type === ORDER_VIEW_CONTEXT) {
    const order = ctx.orderNo || (ctx.orderID ? `#${ctx.orderID}` : '未选择订单')
    const customer = ctx.customerName || (ctx.customerID ? `客户 #${ctx.customerID}` : '')
    return customer ? `订单：${order} / ${customer}` : `订单：${order}`
  }
  if (ctx.type === CUSTOMER_VIEW_CONTEXT || ctx.type === EXTERNAL_CUSTOMER_VIEW_CONTEXT) {
    return `客户：${ctx.customerName || (ctx.customerID ? `#${ctx.customerID}` : '未选择')}`
  }
  return '工厂总览'
}

export function customerViewContextFromOption(option) {
  return normalizeViewContext({
    type: CUSTOMER_VIEW_CONTEXT,
    customerID: option?.customer_id || option?.id,
    customerName: option?.customer_name || option?.name || option?.label,
  })
}

export function orderViewContextFromOption(option) {
  return normalizeViewContext({
    type: ORDER_VIEW_CONTEXT,
    orderID: option?.order_id || option?.id,
    orderNo: option?.order_no,
    customerID: option?.customer_id,
    customerName: option?.customer_name,
  })
}

export function externalCustomerViewContext(context) {
  return normalizeViewContext({
    type: EXTERNAL_CUSTOMER_VIEW_CONTEXT,
    customerID: context?.customer_id || context?.customerID || context?.id,
    customerName: context?.customer_name || context?.customerName || context?.name,
  })
}
