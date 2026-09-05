import {
  CUSTOMER_VIEW_CONTEXT,
  FACTORY_VIEW_CONTEXT,
  CUSTOMER_WORKSPACE_MODE,
  FACTORY_WORKSPACE_MODE,
  menuGroupsForViewContext,
  viewContextViewParams,
} from './view-context.js'

export { FACTORY_WORKSPACE_MODE, CUSTOMER_WORKSPACE_MODE }
export const WORKSPACE_CUSTOMERS_REFRESH_EVENT = 'kferp:workspace-customers-refresh'

const customerActorRoleCodes = new Set(['customer_processing_customer', 'customer_direct_ship_customer'])

export function normalizeWorkspaceMode(value) {
  return value === CUSTOMER_WORKSPACE_MODE ? CUSTOMER_WORKSPACE_MODE : FACTORY_WORKSPACE_MODE
}

export function menuGroupsForWorkspaceMode(groups, mode = FACTORY_WORKSPACE_MODE) {
  return menuGroupsForViewContext(groups, {
    type: normalizeWorkspaceMode(mode) === CUSTOMER_WORKSPACE_MODE ? CUSTOMER_VIEW_CONTEXT : FACTORY_VIEW_CONTEXT,
  })
}

export function defaultWorkspaceEntryKey(groups) {
  return groups?.[0]?.items?.[0]?.key || ''
}

export function workspaceViewParams(params = {}, { mode = FACTORY_WORKSPACE_MODE, customerID = 0 } = {}) {
  return viewContextViewParams(params, {
    type: normalizeWorkspaceMode(mode) === CUSTOMER_WORKSPACE_MODE ? CUSTOMER_VIEW_CONTEXT : FACTORY_VIEW_CONTEXT,
    customerID,
  })
}

export function customerOptionLabel(option) {
  return option?.name || option?.company_name || `客户 #${option?.id || ''}`
}

export function customerWorkspaceDisplayName(customerID, customers = [], contextLabel = '') {
  const normalizedCustomerID = Number(customerID || 0)
  const customer = (Array.isArray(customers) ? customers : [])
    .find((option) => Number(option?.id || 0) === normalizedCustomerID)
  if (customer) return customerOptionLabel(customer)
  const normalizedContextLabel = String(contextLabel || '').trim()
  if (normalizedContextLabel) return normalizedContextLabel
  return normalizedCustomerID > 0 ? `客户 #${normalizedCustomerID}` : ''
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

export function workspaceCustomersRefreshEvent() {
  return new CustomEvent(WORKSPACE_CUSTOMERS_REFRESH_EVENT)
}

export function isCustomerAccountActor(actor = {}) {
  if (!actor || actor.basic_auth_admin) return false
  const roles = Array.isArray(actor.roles) ? actor.roles : []
  if (roles.some((role) => String(role?.code || '').trim().toLowerCase() === 'admin')) return false
  if (roles.some((role) => customerActorRoleCodes.has(String(role?.code || '').trim().toLowerCase()))) return true
  const allowedViews = Array.isArray(actor.allowed_views) ? actor.allowed_views : []
  return allowedViews.length === 1 && allowedViews[0] === 'customerProcessingPortal'
}
