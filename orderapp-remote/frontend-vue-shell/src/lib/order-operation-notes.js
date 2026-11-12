import { buildProductSpecWriteIdentity } from './product-spec-cutover.js'

export function orderOperationNotesContext(row, customerId = 0) {
  const identity = buildProductSpecWriteIdentity(row)
  return [customerId || 0, identity.product_id || 0, identity.bom_spec_id || 0, identity.bom_variant_id || 0].join(':')
}

export function orderOperationOptionsURL(row, { customerPortal = false, customerId = 0, portalService = 'direct_ship' } = {}) {
  const identity = buildProductSpecWriteIdentity(row)
  if (!identity.product_id) return ''
  const params = new URLSearchParams({
    product_id: String(identity.product_id),
    bom_spec_id: String(identity.bom_spec_id || 0),
    bom_variant_id: String(identity.bom_variant_id || 0),
  })
  if (customerPortal) {
    params.set('service', portalService)
    params.set('customer_id', String(customerId || 0))
    return `/api/customer-processing/portal/order/operation-options?${params}`
  }
  return `/api/order-operation-options?${params}`
}
