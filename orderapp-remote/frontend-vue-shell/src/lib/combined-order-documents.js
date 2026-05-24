export function buildCombinedDocumentQuery(orderIDs = []) {
  const params = new URLSearchParams()
  const ids = orderIDs
    .map((id) => Number(id))
    .filter((id) => Number.isFinite(id) && id > 0)
  if (!ids.length) return ''
  params.set('order_ids', ids.join(','))
  return params.toString()
}

export function selectedOrdersShareSameCustomer(orderIDs = [], rows = []) {
  const selectedRows = selectedOrderRows(orderIDs, rows)
  if (selectedRows.length < 2 || selectedRows.length !== uniquePositiveIDs(orderIDs).length) return false
  const firstCustomerID = Number(selectedRows[0]?.customer_id || 0)
  return firstCustomerID > 0 && selectedRows.every((row) => Number(row?.customer_id || 0) === firstCustomerID)
}

export function combinedDocumentSelectionSummary(orderIDs = [], rows = []) {
  const selectedRows = selectedOrderRows(orderIDs, rows)
  const first = selectedRows[0] || {}
  return {
    count: selectedRows.length,
    customerId: Number(first.customer_id || 0),
    customer: first.customer || '',
    valid: selectedOrdersShareSameCustomer(orderIDs, rows),
  }
}

export function selectedOrderRows(orderIDs = [], rows = []) {
  const ids = uniquePositiveIDs(orderIDs)
  return ids
    .map((id) => rows.find((row) => Number(row?.id || 0) === id))
    .filter(Boolean)
}

function uniquePositiveIDs(orderIDs = []) {
  const seen = new Set()
  const ids = []
  for (const raw of orderIDs) {
    const id = Number(raw)
    if (!Number.isFinite(id) || id <= 0 || seen.has(id)) continue
    seen.add(id)
    ids.push(id)
  }
  return ids
}
