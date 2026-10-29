import { apiFetch, apiGet, apiSend } from './client.js'

function queryString(options = {}) {
  const params = new URLSearchParams()
  for (const key of ['fee_type', 'status', 'date_from', 'date_to']) {
    const value = String(options[key] || '').trim()
    if (value) params.set(key, value)
  }
  for (const key of ['page', 'limit']) {
    const value = Number(options[key] || 0)
    if (value > 0) params.set(key, String(value))
  }
  const query = params.toString()
  return query ? `?${query}` : ''
}

const root = (customerId, customerPortal = false) => customerPortal ? '/api/customer-processing/portal/fulfillment/billing' : `/api/customer-fulfillment/${Number(customerId)}/billing`

export function fetchCustomerBillingSummary(customerId, customerPortal = false) {
  return apiGet(`${root(customerId, customerPortal)}/summary`)
}

export function fetchCustomerUnbilledCharges(customerId, options = {}) {
  return apiGet(`${root(customerId, options.customerPortal)}/unbilled${queryString(options)}`)
}

export function fetchCustomerIssuedBills(customerId, options = {}) {
  return apiGet(`${root(customerId, options.customerPortal)}/bills${queryString(options)}`)
}

export function previewCustomerBill(customerId, payload) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/settlements/preview`, { body: payload })
}

export async function previewCustomerBillExcel(customerId, payload, previewVersion = '') {
  const response = await apiFetch(`/api/customer-fulfillment/${Number(customerId)}/settlements/preview.xlsx`, {
    method: 'POST', headers: { 'Content-Type': 'application/json', ...(previewVersion ? { 'If-Match': previewVersion } : {}) }, body: JSON.stringify(payload),
  })
  if (!response.ok) {
    const data = await response.json().catch(() => ({}))
    throw new Error(data.error || data.message || '预览 Excel 导出失败')
  }
  return response.blob()
}

export async function downloadCustomerBill(customerId, batchId, format = 'xlsx', customerPortal = false) {
	const extension = ['pdf', 'png', 'xlsx'].includes(format) ? format : 'xlsx'
	const response = await apiFetch(`${root(customerId, customerPortal)}/bills/${Number(batchId)}.${extension}`)
  if (!response.ok) {
    const data = await response.json().catch(() => ({}))
    throw new Error(data.error || data.message || '账单下载失败')
  }
  return response.blob()
}

export function issueCustomerBill(customerId, payload) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/settlements/issue`, { body: payload })
}

export function fetchCustomerBill(customerId, batchId, customerPortal = false) {
  return apiGet(`${root(customerId, customerPortal)}/bills/${Number(batchId)}`)
}

export function updateCustomerBillPayment(customerId, batchId, action, expectedDueAmountCents, idempotencyKey, expectedPaymentRevision, paymentMethod = '') {
  return apiSend(`${root(customerId)}/bills/${Number(batchId)}/payment`, { body: { action, expected_due_amount_cents: expectedDueAmountCents, idempotency_key: idempotencyKey, expected_payment_revision: expectedPaymentRevision, ...(paymentMethod ? { payment_method: paymentMethod } : {}) } })
}

export function fetchCustomerBillPDFSettings(customerId, batchId) {
  return apiGet(`${root(customerId)}/bills/${Number(batchId)}/pdf-settings`)
}

export function saveCustomerBillPDFSettings(customerId, batchId, payload) {
  return apiSend(`${root(customerId)}/bills/${Number(batchId)}/pdf-settings`, { method: 'PUT', body: payload })
}

export function customerBillExportURL(customerId, batchId, format) {
  return `${root(customerId)}/bills/${Number(batchId)}.${format === 'pdf' ? 'pdf' : 'xlsx'}`
}
