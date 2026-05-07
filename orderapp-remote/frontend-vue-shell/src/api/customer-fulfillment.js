import { apiGet, apiSend } from './client.js'

export function buildCustomerFulfillmentImportForm(importType, file) {
  const form = new FormData()
  form.append('import_type', importType)
  form.append('file', file, file?.name || 'import.xlsx')
  return form
}

export function parseCustomerFulfillmentImport(customerId, importType, file) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/imports/parse`, {
    method: 'POST',
    body: buildCustomerFulfillmentImportForm(importType, file),
  })
}

export function applyCustomerFulfillmentImport(batchId) {
  return apiSend(`/api/customer-fulfillment/imports/${Number(batchId)}/apply`, { body: {} })
}

export function fetchCustomerFulfillmentOverview(customerId) {
  return apiGet(`/api/customer-fulfillment/${Number(customerId)}/overview`)
}

export function fetchCustomerFulfillmentImports(customerId) {
  return apiGet(`/api/customer-fulfillment/${Number(customerId)}/imports`)
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
