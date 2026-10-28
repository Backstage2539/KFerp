import { apiGet, apiSend } from './client.js'

export function fetchCustomerFulfillmentOverview(customerId) {
  return apiGet(`/api/customer-fulfillment/${Number(customerId)}/overview`)
}

export function fetchCustomerFulfillmentOptions(customerId) {
  return apiGet(`/api/customer-fulfillment/${Number(customerId)}/options`)
}

export function fetchCustomerFulfillmentCustomers(query = '', limit = 200) {
  const params = new URLSearchParams()
  const q = String(query || '').trim()
  if (q) params.set('q', q)
  params.set('limit', String(Number(limit) || 200))
  return apiGet(`/api/customer-fulfillment/customers?${params.toString()}`)
}

export function createCustomerFulfillmentSettlement(customerId, payload) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/settlements`, { body: payload })
}

export function adjustCustomerFulfillmentCustodyInventory(customerId, payload) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/custody-adjustments`, { body: payload })
}

export function fetchCustomerFulfillmentERPBindings(customerId) {
  return apiGet(`/api/customer-fulfillment/${Number(customerId)}/erp-bindings`)
}

export function upsertCustomerFulfillmentERPBinding(customerId, payload) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/erp-bindings`, { body: payload })
}

export function fetchCustomerFulfillmentExternalUsers(customerId) {
  return apiGet(`/api/customer-fulfillment/${Number(customerId)}/external-users`)
}

export function createCustomerFulfillmentExternalUser(customerId, payload) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/external-users`, { body: payload })
}

export function resetCustomerFulfillmentExternalUserPassword(customerId, employeeId, password) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/external-users/${Number(employeeId)}/password/reset`, {
    body: { password },
  })
}

export function setCustomerFulfillmentExternalUserLoginEnabled(customerId, employeeId, loginEnabled) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/external-users/${Number(employeeId)}/login-enabled`, {
    body: { login_enabled: !!loginEnabled },
  })
}

export function submitCustomerFulfillmentProcessingWorkOrder(customerId, payload) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/work-orders`, { body: payload })
}

export function submitCustomerFulfillmentDirectShipOrder(customerId, payload) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/direct-ship-orders`, { body: payload })
}

export function fetchCustomerFulfillmentOrders(customerId, options = {}) {
  const params = new URLSearchParams()
  params.set('scope', 'fulfillment')
  if (options.queue) params.set('fulfillment_queue', options.queue)
  params.set('customer_id', String(Number(customerId) || 0))
  const page = Number(options.page || 0)
  if (page > 0) params.set('page', String(page))
  const limit = Number(options.limit || 0)
  if (limit > 0) params.set('limit', String(limit))
  const query = String(options.q || '').trim()
  if (query) params.set('q', query)
  return apiGet(`/api/orders?${params.toString()}`)
}

export function fetchCustomerFulfillmentOrderDetail(orderId) {
  return apiGet(`/api/orders/${Number(orderId)}/detail`)
}

export function fetchCustomerProcessingPortalOverview() {
  return apiGet('/api/customer-processing/portal/overview')
}

export function fetchCustomerProcessingPortalOptions() {
  return apiGet('/api/customer-processing/portal/options')
}

export function fetchInternalCustomerProcessingPortalOverview(customerID) {
  return apiGet(`/api/customer-processing/internal/${Number(customerID)}/overview`)
}

export function fetchInternalCustomerProcessingPortalOptions(customerID) {
  return apiGet(`/api/customer-processing/internal/${Number(customerID)}/options`)
}

export function submitCustomerProcessingWorkOrder(payload) {
  return apiSend('/api/customer-processing/portal/work-orders', { body: payload })
}

export function submitCustomerDirectShipOrder(payload) {
  return apiSend('/api/customer-processing/portal/direct-ship-orders', { body: payload })
}
