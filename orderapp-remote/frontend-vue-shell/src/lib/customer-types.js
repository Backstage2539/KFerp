export const customerTypeOptions = [
  { value: 'retail', label: '零售客户' },
  { value: 'ecommerce', label: '电商客户' },
  { value: 'wholesale', label: '批发客户' },
  { value: 'channel', label: '渠道客户' },
]

const validTypes = new Set(customerTypeOptions.map((item) => item.value))

export function normalizeCustomerType(value) {
  const raw = String(value || '').trim()
  return validTypes.has(raw) ? raw : 'retail'
}

export function validCustomerType(value) {
  return validTypes.has(String(value || '').trim())
}

export function customerTypeLabel(value) {
  const normalized = normalizeCustomerType(value)
  return customerTypeOptions.find((item) => item.value === normalized)?.label || ''
}

export function defaultCapabilityTemplateForCustomerType(value) {
  switch (normalizeCustomerType(value)) {
    case 'channel':
      return 'channel_direct_ship'
    case 'wholesale':
      return 'processing_fulfillment'
    case 'retail':
    case 'ecommerce':
      return 'retail_mall'
    default:
      return ''
  }
}
