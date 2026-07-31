import type {
  EmployeeOrderCustomer,
  EmployeeOrderProductFamily,
  EmployeeOrderProductSpec,
} from '../api/customerPortal'

export type EmployeeOrderProductCategory = 'all' | 'roasted' | 'drip_bag' | 'green_bean' | 'instant_coffee'

export const employeeOrderProductCategories: Array<{ key: EmployeeOrderProductCategory; label: string }> = [
  { key: 'all', label: '全部' },
  { key: 'roasted', label: '熟豆' },
  { key: 'drip_bag', label: '挂耳' },
  { key: 'green_bean', label: '生豆' },
  { key: 'instant_coffee', label: '速溶咖啡' },
]

const CUSTOMER_RESULT_LIMIT = 20
const PRODUCT_RESULT_LIMIT = 30

function normalizedSearchValue(value: unknown): string {
  return String(value || '').trim().toLowerCase().replace(/\s+/g, '')
}

function includesEmployeeOrderSearch(query: string, values: unknown[]): boolean {
  if (!query) return true
  return values.some((value) => normalizedSearchValue(value).includes(query))
}

export function shanghaiToday(now = new Date()): string {
  return new Date(now.getTime() + (8 * 60 * 60 * 1000)).toISOString().slice(0, 10)
}

export function customerShippingDefaults(customer?: EmployeeOrderCustomer) {
  return {
    receiver_name: String(customer?.receiver_name || customer?.contact || customer?.name || '').trim(),
    receiver_phone: String(customer?.receiver_phone || customer?.phone || customer?.company_phone || '').trim(),
    receiver_address: String(customer?.receiver_address || customer?.address || customer?.company_address || '').trim(),
    receiver_company: String(customer?.receiver_company || customer?.company_name || customer?.name || '').trim(),
  }
}

export function filterEmployeeOrderCustomers(
  customers: EmployeeOrderCustomer[] = [],
  query = '',
): EmployeeOrderCustomer[] {
  const normalizedQuery = normalizedSearchValue(query)
  return customers
    .filter((customer) => includesEmployeeOrderSearch(normalizedQuery, [customer.name, customer.py, customer.pyi]))
    .slice(0, CUSTOMER_RESULT_LIMIT)
}

export function customerProductFamilies(
  families: EmployeeOrderProductFamily[] = [],
  customerID = 0,
) {
  const selected = Number(customerID || 0)
  if (selected <= 0) return []

  const customerFamilies = families.filter((family) => Number(family.customer_id || 0) === selected)
  const overriddenParentIDs = new Set(customerFamilies
    .map((family) => Number(family.parent_product_id || 0))
    .filter((parentID) => parentID > 0))
  const overriddenSKUIDs = new Set(customerFamilies
    .flatMap((family) => family.specs || [])
    .map((spec) => Number(spec.sku_id || spec.product_id || 0))
    .filter((skuID) => skuID > 0))

  return families.filter((family) => {
    const owner = Number(family.customer_id || 0)
    if (owner === selected) return true
    if (owner !== 0) return false
    const parentID = Number(family.parent_product_id || 0)
    if (parentID > 0 && overriddenParentIDs.has(parentID)) return false
    return !(family.specs || []).some((spec) => overriddenSKUIDs.has(Number(spec.sku_id || spec.product_id || 0)))
  })
}

export function employeeOrderProductFamilyKey(family: EmployeeOrderProductFamily): string {
  return [
    Number(family.customer_id || 0),
    Number(family.parent_product_id || 0),
    Number(family.customer_product_alias_id || 0),
  ].join(':')
}

export function employeeOrderProductCategory(
  family?: EmployeeOrderProductFamily,
): Exclude<EmployeeOrderProductCategory, 'all'> | '' {
  const kind = normalizedSearchValue(family?.product_kind || family?.specs?.[0]?.product_kind)
  const typeName = normalizedSearchValue(family?.product_type_name)
  const combined = `${kind}|${typeName}`
  if (combined.includes('drip') || combined.includes('挂耳')) return 'drip_bag'
  if (combined.includes('green') || combined.includes('生豆')) return 'green_bean'
  if (combined.includes('instant') || combined.includes('速溶')) return 'instant_coffee'
  if (combined.includes('roasted') || combined.includes('熟豆')) return 'roasted'
  return ''
}

function employeeOrderProductSearchValues(family: EmployeeOrderProductFamily): unknown[] {
  const specValues: unknown[] = []
  for (const spec of family.specs || []) {
    specValues.push(
      spec.sku_code,
      spec.sku_name,
      spec.py,
      spec.pyi,
      spec.spec_label,
      spec.net_content_qty && spec.net_content_unit
        ? `${spec.net_content_qty}${spec.net_content_unit}`
        : '',
    )
  }
  return [
    family.name,
    family.parent_product_name,
    family.alias_name,
    family.customer_product_display_name,
    family.customer_item_code,
    family.code,
    family.py,
    family.pyi,
    family.product_code,
    family.product_type_name,
    ...specValues,
  ]
}

export function filterEmployeeOrderProductFamilies(
  families: EmployeeOrderProductFamily[] = [],
  customerID = 0,
  query = '',
  category: EmployeeOrderProductCategory = 'all',
): EmployeeOrderProductFamily[] {
  const normalizedQuery = normalizedSearchValue(query)
  const uniqueFamilies = new Map<string, EmployeeOrderProductFamily>()

  for (const family of customerProductFamilies(families, customerID)) {
    const key = employeeOrderProductFamilyKey(family)
    if (!uniqueFamilies.has(key)) uniqueFamilies.set(key, family)
  }

  return Array.from(uniqueFamilies.values())
    .filter((family) => category === 'all' || employeeOrderProductCategory(family) === category)
    .filter((family) => includesEmployeeOrderSearch(normalizedQuery, employeeOrderProductSearchValues(family)))
    .slice(0, PRODUCT_RESULT_LIMIT)
}

export function defaultProductSpec(
  family?: EmployeeOrderProductFamily,
): EmployeeOrderProductSpec | undefined {
  const specs = family?.specs || []
  return specs.find((spec) => Number(spec.product_id || spec.sku_id || 0) === Number(family?.default_sku_id || 0))
    || specs.find((spec) => spec.is_default_sku)
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

export function salesUnitLabel(unit?: string): string {
  const value = String(unit || '').trim()
  const lower = value.toLowerCase()
  if (lower === 'bag') return '袋'
  if (lower === 'box') return '盒'
  if (lower === 'kg') return '公斤'
  if (lower === 'g') return '克'
  return value || '件'
}
