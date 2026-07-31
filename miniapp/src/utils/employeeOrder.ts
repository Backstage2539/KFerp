import type {
  EmployeeOrderCustomer,
  EmployeeOrderProductFamily,
  EmployeeOrderProductSpec,
} from '../api/customerPortal'

export function customerShippingDefaults(customer?: EmployeeOrderCustomer) {
  return {
    receiver_name: String(customer?.receiver_name || customer?.name || '').trim(),
    receiver_phone: String(customer?.receiver_phone || '').trim(),
    receiver_address: String(customer?.receiver_address || '').trim(),
    receiver_company: String(customer?.receiver_company || customer?.name || '').trim(),
  }
}

export function customerProductFamilies(
  families: EmployeeOrderProductFamily[] = [],
  customerID = 0,
) {
  const selected = Number(customerID || 0)
  return families.filter((family) => {
    const owner = Number(family.customer_id || 0)
    return owner === 0 || selected === 0 || owner === selected
  })
}

export function defaultProductSpec(
  family?: EmployeeOrderProductFamily,
): EmployeeOrderProductSpec | undefined {
  const specs = family?.specs || []
  return specs.find((spec) => spec.is_default_sku)
    || specs.find((spec) => Number(spec.product_id || spec.sku_id || 0) === Number(family?.default_sku_id || 0))
    || specs[0]
}

export function productSpecLabel(spec?: EmployeeOrderProductSpec) {
  const explicit = String(spec?.spec_label || spec?.sku_name || '').trim()
  if (explicit) return explicit
  const qty = Number(spec?.net_content_qty || 0)
  const unit = String(spec?.net_content_unit || '').trim()
  return qty > 0 && unit ? `${qty}${unit}` : '默认规格'
}

export function productSpecWeightG(spec?: EmployeeOrderProductSpec) {
  const qty = Number(spec?.net_content_qty || 0)
  const unit = String(spec?.net_content_unit || '').trim().toLowerCase()
  if (qty > 0 && (unit === 'kg' || unit === '千克' || unit === '公斤')) return Math.round(qty * 1000)
  if (qty > 0 && (unit === 'g' || unit === '克')) return Math.round(qty)
  const label = productSpecLabel(spec)
  const kg = label.match(/([0-9]+(?:\.[0-9]+)?)\s*(?:kg|千克|公斤)/i)
  if (kg) return Math.round(Number(kg[1]) * 1000)
  const gram = label.match(/([0-9]+(?:\.[0-9]+)?)\s*(?:g|克)/i)
  if (gram) return Math.round(Number(gram[1]))
  return 0
}

export function firstSpecUnitPrice(spec?: EmployeeOrderProductSpec) {
  return Number(spec?.tiers?.[0]?.unit_price || spec?.tiers?.[0]?.price || 0)
}
