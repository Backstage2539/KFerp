// Each draft owns its payment data: no amount or voucher is shared between orders.
export function makeBatchStatusDraft(order) {
  const draft = {
    ...order,
    payment_method: order.payment_method || '',
    prepayment_amount: String(order.prepayment_amount ?? ''),
    payment_goods_amount: String(order.payment_goods_amount ?? ''),
    payment_shipping_amount: String(order.payment_shipping_amount ?? ''),
    payment_voucher_asset_id: Number(order.payment_voucher_asset_id || 0),
    payment_voucher_filename: order.payment_voucher_filename || '',
    result: order.error ? 'failed' : '',
    resultError: order.error || '',
    uploading: false,
    uploadError: false,
    requiresReload: false,
  }
  draft.saved = {
    payment_method: draft.payment_method,
    prepayment_amount: draft.prepayment_amount,
    payment_goods_amount: draft.payment_goods_amount,
    payment_shipping_amount: draft.payment_shipping_amount,
    payment_voucher_asset_id: draft.payment_voucher_asset_id,
    payment_voucher_filename: draft.payment_voucher_filename,
  }
  return draft
}

// Refresh the saved snapshot and revision while retaining only deliberate edits.
export function refreshBatchStatusDraft(previous, order) {
  const next = makeBatchStatusDraft(order)
  for (const field of ['payment_method', 'prepayment_amount', 'payment_goods_amount', 'payment_shipping_amount', 'payment_voucher_asset_id']) {
    if (previous[field] !== previous.saved[field]) next[field] = previous[field]
  }
  if (next.payment_voucher_asset_id !== next.saved.payment_voucher_asset_id) next.payment_voucher_filename = previous.payment_voucher_filename
  return next
}

export function batchStatusRequest(draft, payStatusID, shipStatusID) {
  const result = { order_id: Number(draft.order_id), edit_revision: String(draft.edit_revision || '') }
  if (payStatusID) {
    result.pay_status_id = Number(payStatusID)
    result.payment_method = draft.payment_method || ''
    for (const field of ['prepayment_amount', 'payment_goods_amount', 'payment_shipping_amount']) {
      const value = String(draft[field] ?? '').trim()
      if (!value) continue
      const amount = Number(value)
      if (!Number.isFinite(amount) || amount < 0 || Math.abs(amount * 100 - Math.round(amount * 100)) > 0.00001) {
        throw new Error('金额须为非负数，最多两位小数')
      }
      result[field] = amount
    }
    if (draft.payment_voucher_asset_id) result.payment_voucher_asset_id = Number(draft.payment_voucher_asset_id)
  }
  if (shipStatusID) result.ship_status_id = Number(shipStatusID)
  return result
}

export function batchStatusResultLabel(status) {
  return ({ updated: '已修改', unchanged: '无需修改', failed: '未成功' })[status] || '待提交'
}
