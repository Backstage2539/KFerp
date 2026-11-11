import { parseCombinedMoneyCents } from './combined-order-payment.js'

export function receiptActorIdentity(actor) {
  const id = Number(actor?.employee_id)
  if (Number.isSafeInteger(id) && id > 0) return `employee-${id}`
  // Basic authentication uses the server's single configured administrator.
  if (actor?.basic_auth_admin === true && String(actor?.name || '').trim()) return 'basic-admin'
  return ''
}
export function receiptPendingKey(identity, orderId) {
  if (!/^(employee-[1-9]\d*|basic-admin)$/.test(String(identity || '')) || !Number.isSafeInteger(Number(orderId)) || Number(orderId) <= 0) throw new Error('无法确认当前账号身份，不能安全登记收款')
  return `kferp.order-receipt.pending.${identity}.order-${Number(orderId)}`
}

// Only the original response may prove that this request did not commit.
export function receiptKeepPending(status, notCommitted, firstAttempt) {
  return !(firstAttempt === true && notCommitted === true && (status === 400 || status === 409))
}

export function orderReceiptRequest({ orderId, revision, dueCents, amount, date, method, key, voucherID = 0 }) {
  const actual = parseCombinedMoneyCents(amount)
  if (!Number.isSafeInteger(Number(orderId)) || Number(orderId) <= 0 || !String(revision || '').trim()) throw new Error('订单版本无效，请重新读取')
  if (!Number.isSafeInteger(dueCents) || dueCents <= 0 || actual <= 0 || actual > dueCents) throw new Error('本次收款须大于零且不超过待收金额')
  if (!/^\d{4}-\d{2}-\d{2}$/.test(String(date || ''))) throw new Error('请选择实际到账日期')
  if (!String(method || '').trim() || !String(key || '').trim()) throw new Error('请选择收款方式并生成操作标识')
  return { edit_revision: String(revision), expected_due_cents: dueCents, actual_cents: actual, receipt_date: String(date), payment_method: String(method).trim(), payment_voucher_asset_id: Number(voucherID || 0), idempotency_key: String(key) }
}
