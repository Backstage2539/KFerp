import { miniRequest } from './client'

export type PreviewCustomer = { id: number; name: string }
export type PreviewCustomers = { rows: PreviewCustomer[]; has_next: boolean }
export type PreviewSession = { token: string; expires_at: string }

export function fetchPreviewCustomers(token: string, query = '', page = 1): Promise<PreviewCustomers> {
  const q = encodeURIComponent(query.trim())
  return miniRequest(`/api/mini/employee/customer-preview/customers?q=${q}&page=${page}&limit=50`, { token })
}

export function beginPreview(token: string, customerID: number): Promise<PreviewSession> {
  return miniRequest('/api/mini/employee/customer-preview', { method: 'POST', token, data: { customer_id: customerID } })
}

export function exitPreview(token: string): Promise<unknown> {
  return miniRequest('/api/mini/customer-preview/exit', { method: 'POST', token })
}
