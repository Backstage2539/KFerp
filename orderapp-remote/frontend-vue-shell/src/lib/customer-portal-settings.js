export function customerDossierNavigationDetail(row = {}) {
  const customerID = Number(row?.customer?.id || row?.id || 0)
  return {
    key: 'customers',
    params: customerID > 0 ? { edit_id: customerID } : {},
  }
}

export function effectiveCustomerCapabilities(row = {}, template = {}) {
  const sameTemplate = String(row.form?.capability_template_key || '') === String(row.customer?.capability_template_key || '')
  const capabilities = sameTemplate && row.capabilities?.length ? row.capabilities : template.capabilities || []
  const result = capabilities.map(item => ({ ...item, config: { ...(item.config || {}) } }))
  const dependency = row.capabilities?.find(item => item.code === 'product_order' && item.config?.required_by_fulfillment === true)
  if (dependency) {
    let productOrder = result.find(item => item.code === 'product_order')
    if (!productOrder) { productOrder = { code: 'product_order', config: {} }; result.push(productOrder) }
    productOrder.enabled = true
    productOrder.config.required_by_fulfillment = true
  }
  return result
}
