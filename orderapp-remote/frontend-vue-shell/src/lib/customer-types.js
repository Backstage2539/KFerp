export const customerTypeOptions = [
  { value: 'retail', label: '零售客户' },
  { value: 'ecommerce', label: '电商客户' },
  { value: 'wholesale', label: '批发客户' },
  { value: 'channel', label: '渠道客户' },
]

function normalizeOption(item) {
  const value = String(item?.value || '').trim()
  const label = String(item?.label || item?.name || value).trim()
  return value && label ? { value, label } : null
}

export function mergeCustomerTypeOptions(options = []) {
  const out = []
  const seen = new Set()
  for (const item of [...customerTypeOptions, ...(options || [])]) {
    const normalized = normalizeOption(item)
    if (!normalized || seen.has(normalized.value)) continue
    seen.add(normalized.value)
    out.push(normalized)
  }
  return out
}

export function normalizeCustomerType(value, options = customerTypeOptions) {
  const raw = String(value || '').trim()
  const validTypes = new Set(mergeCustomerTypeOptions(options).map((item) => item.value))
  return validTypes.has(raw) ? raw : 'retail'
}

export function validCustomerType(value, options = customerTypeOptions) {
  const validTypes = new Set(mergeCustomerTypeOptions(options).map((item) => item.value))
  return validTypes.has(String(value || '').trim())
}

export function customerTypeLabel(value, options = customerTypeOptions) {
  const normalized = normalizeCustomerType(value, options)
  return mergeCustomerTypeOptions(options).find((item) => item.value === normalized)?.label || normalized || ''
}
