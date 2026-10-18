import { apiFetch } from './client.js'

export async function fetchOrderDetailsText(id, customerPortal = false, customerID = 0) {
  const base = customerPortal ? '/api/customer-processing/portal/orders' : '/api/orders'
  const query = customerPortal && Number(customerID) > 0 ? `?customer_id=${Number(customerID)}` : ''
  // Do not deduplicate across session/customer changes using apiGet's URL cache.
  const response = await apiFetch(`${base}/${id}/copy-details${query}`, { cache: 'no-store', headers: { Accept: 'application/json' } })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(data.error || data.message || '读取订单详情失败，请重试')
  return data
}
