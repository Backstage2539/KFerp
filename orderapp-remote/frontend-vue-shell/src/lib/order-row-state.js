export function isShipped(row) {
  return String(row?.ship_status || '').includes('已发货')
}

export function isUnpaid(row) {
  const status = String(row?.pay_status || '').trim()
  return status === '' || status.includes('未')
}

function isPaid(row) {
  return /已付款|已收款|已支付/.test(String(row?.pay_status || '').trim())
}

function isUnproduced(row) {
  const status = String(row?.process_status || '').trim()
  if (!status) return true
  return !(status.includes('生产完成') || status === '无需生产' || status === '库存待发货')
}

export function orderRowClass(row, highlighted = false) {
  const unpaid = isUnpaid(row)
  const shipped = isShipped(row)
  const shippedPaid = !row?.is_void && !unpaid && shipped && isPaid(row)
  return {
    voided: row?.is_void,
    'highlight-new': highlighted,
    'state-unpaid': unpaid,
    'state-shipped-paid': shippedPaid,
    'state-unshipped': !highlighted && !unpaid && !shipped,
    'state-unproduced': !highlighted && !unpaid && shipped && !shippedPaid && isUnproduced(row),
  }
}
