const statusRanks = {
  awaiting_schedule: 0,
  planned: 0,
  released: 1,
  paused: 2,
  running: 2,
  partially_completed: 2,
  completed: 3,
}

export function customerProcessingSourceLabel(source) {
  return String(source || '').trim() === 'customer_processing' ? '客户工单' : String(source || '').trim() || '订单需求'
}

export function customerProcessingProgress(item = {}) {
  const requestedQty = Math.max(0, Number(item.target_qty || item.qty || 0))
  const inboundQty = Math.min(requestedQty, Math.max(0, Number(item.actual_inbound_qty || 0)))
  const reservedQty = Math.max(0, Number(item.output_reserved_qty || 0))
  const convertedQty = Math.max(0, Number(item.output_converted_qty || 0))
  const releasedQty = Math.max(0, Number(item.output_released_qty || 0))
  const orderOccupiedQty = Math.max(0, reservedQty - convertedQty - releasedQty)
  return {
    requestedQty,
    inboundQty,
    orderOccupiedQty,
    remainingReservableQty: Math.max(0, requestedQty - inboundQty - orderOccupiedQty),
  }
}

export function customerProcessingTimeline(request = {}) {
  const currentRank = statusRanks[String(request.status || '')] ?? 0
  return [
    { key: 'submitted', label: '待接单', time: request.created_at },
    { key: 'accepted', label: '已接单', time: request.accepted_at },
    { key: 'running', label: '开始生产', time: request.started_at },
    { key: 'completed', label: '生产完成', time: request.completed_at },
  ].map((step, index) => ({
    ...step,
    time: String(step.time || '').trim() || '暂无记录',
    state: index < currentRank ? 'done' : index === currentRank ? 'current' : 'pending',
  }))
}

export function customerProcessingReceiptResult({ document = {}, workOrder = {}, request = {}, processingRequestItemID = 0 } = {}) {
  const hub = workOrder.execution_hub || {}
  const header = hub.header || workOrder.work_order || {}
  const quality = hub.quality_status || {}
  const item = (request.items || []).find((row) => Number(row.id || 0) === Number(processingRequestItemID || header.processing_request_item_id || 0)) || {}
  const receiptItems = Array.isArray(document.items) ? document.items : []
  const receiptQty = receiptItems.reduce((sum, row) => sum + Number(row.qty_units || row.quantity || row.qty_g || 0), 0)
  const inboundQty = Math.max(0, Number(item.actual_inbound_qty || 0))
  const occupiedQty = Math.max(0, Number(item.output_converted_qty || 0) - Number(item.output_released_qty || 0))
  const qualityLabel = String(quality.result || '').trim() || ({ pass: '合格', reject: '不合格', hold: '待处理', blocked: '已冻结' }[quality.status] || '暂无记录')
  return {
    stockEntryID: Number(document.id || 0),
    stockEntryNo: String(document.entry_no || '').trim(),
    workOrderID: Number(header.work_order_id || document.work_order_id || 0),
    workOrderNo: String(header.work_order_no || document.work_order_no || '').trim(),
    requestNo: String(request.request_no || '').trim(),
    receiptQty,
    inboundQty,
    occupiedQty,
    availableQty: Math.max(0, inboundQty - occupiedQty),
    warehouse: String(receiptItems[0]?.to_warehouse || header.target_warehouse || item.target_warehouse || '').trim(),
    batchCode: String(receiptItems[0]?.batch_code || header.batch_id || '').trim(),
    qualityLabel,
    convertedOrders: (item.related_orders || []).filter((order) => Number(order.converted_qty || 0) > Number(order.released_qty || 0)),
  }
}
