export function customerBillingTotals({ unbilled = [], bills = [] } = {}) {
  const unbilledCents = unbilled.reduce((sum, row) => sum + Math.max(0, Number(row.due_cents || row.due_amount_cents || 0)), 0)
  const issuedCents = bills.reduce((sum, row) => sum + Math.max(0, Number(row.due_amount_cents || 0)), 0)
  return {
    total_unpaid_cents: unbilledCents + issuedCents,
    unbilled_cents: unbilledCents,
    issued_cents: issuedCents,
  }
}

export function isCustomerOrderFullyPaid(value) {
  const status = String(value || '').trim().toLowerCase()
  return ['已付款', '已收款', '已支付', '已收清', '全额付款', '付款完成', 'paid', 'fully_paid', 'fully-paid'].includes(status)
}

export function customerBillingRowClass(row, shipped = false) {
  if (!isCustomerOrderFullyPaid(row?.pay_status)) return 'order-row-unpaid'
  return shipped ? 'order-row-complete' : ''
}

export function pageSelectionState(rows = [], selected = new Set()) {
  const ids = rows.filter(isSelectable).map((row) => String(row.id))
  const count = ids.filter((id) => selected.has(id)).length
  return { checked: ids.length > 0 && count === ids.length, indeterminate: count > 0 && count < ids.length }
}

export function togglePageSelection(rows = [], selected = new Set(), checked = false) {
  const next = new Set(selected)
  for (const row of rows.filter(isSelectable)) {
    const id = String(row.id)
    if (checked) next.add(id)
    else next.delete(id)
  }
  return next
}

export function visibleChargeSelection(rows = [], selected = new Set()) {
  return new Set(rows.filter(isSelectable).map((row) => String(row.id)).filter((id) => selected.has(id)))
}

function isSelectable(row) {
  const amount = row?.due_cents ?? row?.amount_cents
  return row?.confirmation_status !== 'pending_billing' && (amount === undefined || Number(amount) > 0)
}

// A recorded payment must be reversed before another can be made. A legacy
// paid bill has no safe reversal record and must be reconciled separately.
export function billPaymentAction(bill) {
  if (bill?.payment_reversible) return 'reverse'
  if (bill?.status === 'confirmed' && Number(bill?.current_due_amount_cents || 0) > 0) return 'pay'
  return ''
}
