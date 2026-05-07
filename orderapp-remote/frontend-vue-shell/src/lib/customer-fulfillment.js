export function importTypeOptions() {
  return [
    { value: 'processing_workbook', label: '代加工工单' },
    { value: 'direct_ship_workbook', label: '代发清单' },
    { value: 'settlement_workbook', label: '结算单' },
  ]
}

export function importSummaryCards(summary = {}) {
  const cards = []
  addPositiveCard(cards, '有效行', summary.valid_rows)
  addPositiveCard(cards, '错误行', summary.invalid_rows)
  addPositiveCard(cards, '代发订单', summary.direct_ship_orders)
  addPositiveCard(cards, '加工工单', summary.processing_orders)
  addPositiveCard(cards, '费用明细', summary.fee_items)
  return cards
}

export function rowStatusLabel(status) {
  return {
    valid: '有效',
    invalid: '错误',
    applied: '已应用',
    parsed: '已解析',
  }[status] || status || '-'
}

export function activeCustomerFulfillmentCustomers(payload = {}) {
  const rows = Array.isArray(payload.customers) ? payload.customers : (Array.isArray(payload.rows) ? payload.rows : [])
  return rows.filter((row) => row?.active !== false)
}

export function customerFulfillmentCustomerOptionLabel(customer) {
  const fallback = Number(customer?.id || 0) > 0 ? `客户 ${customer.id}` : ''
  return String(customer?.name || customer?.company_name || fallback).trim()
}

export function customerFulfillmentCustomerOptionMeta(customer) {
  const label = customerFulfillmentCustomerOptionLabel(customer)
  const parts = []
  if (customer?.company_name && customer.company_name !== label) parts.push(customer.company_name)
  if (customer?.contact) parts.push(customer.contact)
  if (customer?.phone || customer?.company_phone) parts.push(customer.phone || customer.company_phone)
  return parts.join(' / ')
}

function addPositiveCard(cards, label, value) {
  const n = Number(value || 0)
  if (n > 0) cards.push({ label, value: n })
}
