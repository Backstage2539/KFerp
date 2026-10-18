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

export function enabledFulfillmentModes(settings) {
  return (settings?.modes || []).filter(row => row.enabled && ['stock', 'on_demand'].includes(row.mode))
}

export function customerStockDeliveryLabel(status = '') {
  return ({ pending_receipt: '待入客户仓', partially_received: '部分入客户仓', received: '已入客户仓' })[status] || '待入客户仓'
}

export function ordinaryOrderVoidable(row = {}) { return !row.is_void && !['stock', 'on_demand'].includes(row.fulfillment_mode) }
