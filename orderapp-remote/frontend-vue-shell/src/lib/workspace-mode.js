export const FACTORY_WORKSPACE_MODE = 'factory'
export const CUSTOMER_WORKSPACE_MODE = 'customer'

const customerWorkspaceSpec = [
  {
    id: 'customerAccount',
    name: '客户账户',
    keys: ['customerFulfillment', 'order', 'orders', 'warehouseInventory', 'producePlan', 'workspaceModeManual'],
  },
  {
    id: 'customerGoods',
    name: '客户商品与配方',
    keys: ['productSettings', 'costing', 'bom', 'mallSettings'],
  },
  {
    id: 'customerFinance',
    name: '客户财务',
    keys: ['financeExpenses'],
  },
]

const factoryHiddenKeys = new Set(['customerFulfillment'])
const customerActorRoleCodes = new Set(['customer_processing_customer', 'customer_direct_ship_customer'])

export function normalizeWorkspaceMode(value) {
  return value === CUSTOMER_WORKSPACE_MODE ? CUSTOMER_WORKSPACE_MODE : FACTORY_WORKSPACE_MODE
}

function itemByKey(groups, key) {
  for (const group of groups || []) {
    const item = (group.items || []).find((row) => row.key === key)
    if (item) return { ...item }
  }
  return null
}

export function menuGroupsForWorkspaceMode(groups, mode = FACTORY_WORKSPACE_MODE) {
  if (normalizeWorkspaceMode(mode) !== CUSTOMER_WORKSPACE_MODE) {
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
      items: group.keys.map((key) => itemByKey(groups, key)).filter(Boolean),
    }))
    .filter((group) => group.items.length > 0)
}

export function defaultWorkspaceEntryKey(groups) {
  return groups?.[0]?.items?.[0]?.key || ''
}

export function workspaceViewParams(params = {}, { mode = FACTORY_WORKSPACE_MODE, customerID = 0 } = {}) {
  const out = { ...(params || {}) }
  if (normalizeWorkspaceMode(mode) === CUSTOMER_WORKSPACE_MODE && Number(customerID || 0) > 0) {
    out.customer_id = String(Number(customerID || 0))
  }
  return out
}

export function customerOptionLabel(option) {
  return option?.name || option?.company_name || `客户 #${option?.id || ''}`
}

export function customerOptionMeta(option) {
  const parts = []
  if (option?.company_name && option.company_name !== option.name) parts.push(option.company_name)
  if (option?.contact) parts.push(option.contact)
  if (option?.phone || option?.company_phone) parts.push(option.phone || option.company_phone)
  return parts.join(' / ')
}

export function workspaceCustomerChangeEvent(customerID) {
  return new CustomEvent('kferp:workspace-customer-change', {
    detail: { customerID: Number(customerID || 0) },
  })
}

export function isCustomerAccountActor(actor = {}) {
  if (!actor || actor.basic_auth_admin) return false
  const roles = Array.isArray(actor.roles) ? actor.roles : []
  if (roles.some((role) => String(role?.code || '').trim().toLowerCase() === 'admin')) return false
  if (roles.some((role) => customerActorRoleCodes.has(String(role?.code || '').trim().toLowerCase()))) return true
  const allowedViews = Array.isArray(actor.allowed_views) ? actor.allowed_views : []
  return allowedViews.length === 1 && allowedViews[0] === 'customerProcessingPortal'
}
