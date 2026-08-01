import type {
  EmployeeOrderCustomer,
  EmployeeOrderDraftItem,
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
let employeeOrderItemSequence = 0

export type EmployeeOrderShippingSnapshot = {
  receiver_name: string
  receiver_phone: string
  receiver_address: string
  receiver_company: string
}

export function createEmployeeOrderItem(key = ''): EmployeeOrderDraftItem {
  employeeOrderItemSequence += 1
  return {
    key: key || `line-${Date.now()}-${employeeOrderItemSequence}`,
    product_family_key: '',
    product_family_id: 0,
    customer_product_alias_id: 0,
    product_id: 0,
    product_name: '',
    product_kind: 'roasted_bean',
    spec_label: '',
    spec_g: 0,
    sales_unit: '袋',
    unit_bag_count: 0,
    unit_bean_g: 0,
    qty: 1,
    unit_price: 0,
    validation_error: '',
  }
}

export function clearEmployeeOrderItem(item: EmployeeOrderDraftItem): EmployeeOrderDraftItem {
  return createEmployeeOrderItem(item.key)
}

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

export function employeeOrderShippingChanged(
  current: EmployeeOrderShippingSnapshot,
  baseline: EmployeeOrderShippingSnapshot,
): boolean {
  return (Object.keys(baseline) as Array<keyof EmployeeOrderShippingSnapshot>)
    .some((key) => String(current[key] || '').trim() !== String(baseline[key] || '').trim())
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

export function employeeOrderItemFromSpec(
  item: EmployeeOrderDraftItem,
  family: EmployeeOrderProductFamily,
  spec: EmployeeOrderProductSpec,
): EmployeeOrderDraftItem {
  const tier = spec.tiers?.[0]
  return {
    ...item,
    product_family_key: employeeOrderProductFamilyKey(family),
    product_family_id: Number(family.parent_product_id || 0),
    customer_product_alias_id: Number(family.customer_product_alias_id || 0),
    product_id: Number(spec.product_id || spec.sku_id || 0),
    product_name: family.name,
    product_kind: spec.product_kind || family.product_kind || 'roasted_bean',
    spec_label: productSpecLabel(spec),
    spec_g: productSpecWeightG(spec),
    sales_unit: spec.sales_unit || tier?.sales_unit || '袋',
    unit_bag_count: Number(spec.unit_bag_count || tier?.unit_bag_count || 0),
    unit_bean_g: Number(spec.unit_bean_g || 0),
    unit_price: firstSpecUnitPrice(spec),
    validation_error: '',
  }
}

export function revalidateEmployeeOrderItems(
  items: EmployeeOrderDraftItem[],
  families: EmployeeOrderProductFamily[],
  customerID: number,
  options: { preserveUnitPrice?: boolean; preserveUnavailable?: boolean } = {},
): EmployeeOrderDraftItem[] {
  const available = customerProductFamilies(families, customerID)
  return items.map((item) => {
    if (Number(item.product_id || 0) <= 0) return { ...item }
    const exactFamily = item.product_family_key
      ? available.find((family) => employeeOrderProductFamilyKey(family) === item.product_family_key)
      : undefined
    const candidates = item.product_family_key ? (exactFamily ? [exactFamily] : []) : available
    for (const family of candidates) {
      const spec = (family.specs || []).find(
        (row) => Number(row.product_id || row.sku_id || 0) === Number(item.product_id),
      )
      if (spec) {
        const validated = employeeOrderItemFromSpec(item, family, spec)
        return options.preserveUnitPrice
          ? { ...validated, unit_price: Number(item.unit_price || 0) }
          : validated
      }
    }
    return options.preserveUnavailable
      ? { ...item, validation_error: '商品已失效或不适用于当前客户，请重新选择' }
      : clearEmployeeOrderItem(item)
  })
}

export function preserveEmployeeOrderDraftItemsForMissingCustomer(
  items: EmployeeOrderDraftItem[],
): EmployeeOrderDraftItem[] {
  return items.map((item) => Number(item.product_id || 0) > 0
    ? { ...item, validation_error: '草稿客户已失效或不可用，请重新选择客户和商品' }
    : { ...item })
}

export function buildEmployeeOrderItemsPayload(items: EmployeeOrderDraftItem[]) {
  return items
    .filter((item) => Number(item.product_id || 0) > 0)
    .map((item) => ({
      product_id: Number(item.product_id),
      customer_product_alias_id: Number(item.customer_product_alias_id || 0),
      name: item.product_name,
      product_kind: item.product_kind,
      qty: Number(item.qty),
      spec_g: Number(item.spec_g),
      unit: item.sales_unit,
      sales_unit: item.sales_unit,
      unit_bag_count: Number(item.unit_bag_count || 0),
      unit_bean_g: Number(item.unit_bean_g || 0),
      unit_price: Number(item.unit_price || 0),
    }))
}

export function employeeOrderItemsTotal(items: EmployeeOrderDraftItem[]): number {
  return items.reduce((total, item) => {
    if (Number(item.product_id || 0) <= 0) return total
    return total + (Number(item.qty || 0) * Number(item.unit_price || 0))
  }, 0)
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
