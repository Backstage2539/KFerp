export function shippingReturnUploadMessage(data) {
  if (data.rematched) {
    return `已重新匹配；待选择 ${Number(data.pending || 0)} 条；补录寄件重量 ${Number(data.weight_backfilled || 0)} 条；重量冲突或异常跳过 ${Number(data.weight_skipped || 0)} 条。本次未执行发货。`
  }
  return `已回填 ${Number(data.updated || 0)} 条；待选择 ${Number(data.pending || 0)} 条；跳过 ${Number(data.skipped || 0)} 条`
}

export function shippingReturnBatchMatches(rows, batchID) {
  const orderIDs = new Set()
  const candidatesByOrder = new Map()
  let pendingCount = 0
  for (const row of rows) {
    if (row.status === 'applied' && row.order_id) orderIDs.add(row.order_id)
    if (row.status !== 'pending' || !row.candidates?.length) continue
    pendingCount++
    for (const candidate of row.candidates) {
      const orderID = Number(candidate.order_id)
      orderIDs.add(orderID)
      if (!candidatesByOrder.has(orderID)) candidatesByOrder.set(orderID, [])
      candidatesByOrder.get(orderID).push({ ...row, ...candidate, id: row.id, batch_id: batchID })
    }
  }
  return { orderCount: orderIDs.size, pendingCount, candidatesByOrder }
}
