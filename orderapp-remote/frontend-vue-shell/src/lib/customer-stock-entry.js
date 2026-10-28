export function customerStockWarehouses(rows = [], customerID = 0) {
  return rows.filter(row => row.active !== false && Number(row.customer_id) === Number(customerID) && ['finished', 'customer_finished', 'customer_processing', 'customer'].includes(row.kind))
}

export function recipientAddressSnapshot(address) {
  return {
    receiver_name: String(address?.recipient_name || '').trim(),
    receiver_phone: String(address?.phone || '').trim(),
    receiver_company: String(address?.company || '').trim(),
    receiver_address: ['province', 'city', 'district', 'detail_address'].map(key => String(address?.[key] || '').trim()).join(''),
  }
}

export function stockSupplySummary(spec = {}, requested = 0) {
  const physical = Math.max(0, Number(spec.physical_available_qty ?? spec.stock_available_qty ?? 0))
  const pending = Math.max(0, Number(spec.pending_inbound_qty || 0))
  const reserved = Math.max(0, Number(spec.pending_reserved_qty || 0))
  return { physical, pending, reserved, gap: Math.max(0, Number(requested || 0) - physical - Math.max(0, pending - reserved)) }
}

export function stockPurchaseSummary(spec = {}) {
  return { purchased: spec.purchased_qty == null ? null : Number(spec.purchased_qty), received: spec.received_qty == null ? null : Number(spec.received_qty) }
}

export function stockGroupShortage(spec = {}, quantities = []) {
  const requested = quantities.reduce((sum, qty) => sum + Math.max(0, Number(qty) || 0), 0)
  return stockSupplySummary(spec, requested).gap
}

export function enabledFulfillmentModes(settings) {
  return (settings?.modes || []).filter(row => row.enabled && ['stock', 'on_demand'].includes(row.mode))
}

export function customerStockDeliveryLabel(status = '') {
  return ({ pending_receipt: '待入客户仓', partially_received: '部分入客户仓', received: '已入客户仓' })[status] || '待入客户仓'
}

export function isCustomerStockOrder(order = {}) {
  return order?.delivery_mode === 'customer_stock'
}

export function customerStockProgressLabel(order = {}) {
  if (order.fulfillment_progress?.label) return order.fulfillment_progress.label
  if (['received', 'partially_received'].includes(order.delivery_status)) return customerStockDeliveryLabel(order.delivery_status)
  const production = String(order.process_status || '').trim()
  if (['生产完成', '已生产完成', '已生产', '无需生产', '库存待发货'].includes(production)) return '待入客户仓'
  return production || '待生产'
}

export function ordinaryOrderVoidable(row = {}) { return !row.is_void && !['stock', 'on_demand'].includes(row.fulfillment_mode) }

export function orderProgressLabel(order = {}) {
 return order.fulfillment_progress?.label || (isCustomerStockOrder(order) ? customerStockProgressLabel(order) : [order.process_status,order.ship_status].filter(Boolean).join(' · ')) || '待处理'
}
