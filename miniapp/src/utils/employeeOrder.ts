import type {
  EmployeeOrderCustomer,
  EmployeeOrderDetailItem,
  EmployeeOrderDraftItem,
  EmployeeOrderProductFamily,
  EmployeeOrderProductSpec,
} from '../api/customerPortal'
import {
  buildMiniappProductSpecIdentity,
  miniappProductMigrationState,
  visibleMiniappProductFamilies,
} from './productSpecIdentity'

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
    item_id: 0,
    key: key || `line-${Date.now()}-${employeeOrderItemSequence}`,
    product_family_key: '',
    product_family_id: 0,
    customer_product_alias_id: 0,
    product_id: 0,
    migration_state: 'legacy',
    bom_spec_id: 0,
    bom_variant_id: 0,
    product_name: '',
    product_kind: 'roasted_bean',
    spec_label: '',
    spec_g: 0,
    sales_unit: '袋',
    unit_bag_count: 0,
    unit_bean_g: 0,
    qty: 1,
    unit_price: 0,
    bean_list_publication_id: 0,
    bean_list_version_no: '',
    price_override: false,
    price_source_json: '',
    discount_type: '',
    discount_value: 0,
    discount_amount: 0,
    retail_order: false,
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

  const visibleFamilies = visibleMiniappProductFamilies(families)
  const customerFamilies = visibleFamilies.filter((family) => Number(family.customer_id || 0) === selected)
  const overriddenParentIDs = new Set(customerFamilies
    .map((family) => Number(family.parent_product_id || 0))
    .filter((parentID) => parentID > 0))
  const overriddenSKUIDs = new Set(customerFamilies
    .flatMap((family) => family.specs || [])
    .map((spec) => Number(spec.sku_id || spec.product_id || 0))
    .filter((skuID) => skuID > 0))

  return visibleFamilies.filter((family) => {
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
      spec.spec_code,
      spec.barcode,
      spec.sku_name,
      spec.spec_key,
      spec.spec_name,
      spec.py,
      spec.pyi,
      spec.spec_label,
      spec.inventory_unit,
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
  if (miniappProductMigrationState(family as unknown as Record<string, unknown>) === 'cutover') {
    return specs.find((spec) => Number(spec.bom_spec_id || 0) === Number(family?.default_bom_spec_id || 0))
      || specs.find((spec) => spec.is_default_sku)
      || specs[0]
  }
  return specs.find((spec) => Number(spec.product_id || spec.sku_id || 0) === Number(family?.default_sku_id || 0))
    || specs.find((spec) => spec.is_default_sku)
    || specs[0]
}

export function productSpecLabel(spec?: EmployeeOrderProductSpec) {
  const explicit = String(spec?.spec_label || spec?.spec_name || spec?.sku_name || '').trim()
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

function roundEmployeeOrderMoney(value: number): number {
  return Math.round((Number.isFinite(value) ? value : 0) * 100) / 100
}

function employeeOrderItemBaseTotal(item: EmployeeOrderDraftItem): number {
  const qty = Math.max(Number(item.qty || 0), 0)
  const unitPrice = Math.max(Number(item.unit_price || 0), 0)
  const productKind = String(item.product_kind || '').trim().toLowerCase()
  if (item.migration_state === 'cutover' || Number(item.bom_spec_id || 0) > 0 || item.retail_order || employeeOrderItemQuantityBasis(item) === 'sales_spec_count' || productKind === 'drip_bag') {
    return qty * unitPrice
  }
  const specG = Math.max(Number(item.spec_g || 0), 0)
  return unitPrice * specG * qty / (specG >= 1000 ? 1000 : 454)
}

function normalizedEmployeeOrderDiscountType(value: unknown): string {
  switch (String(value || '').trim().toLowerCase()) {
    case 'amount':
    case 'fixed':
    case 'minus':
      return 'amount'
    case 'unit_amount':
    case 'unit':
    case 'unit_discount':
    case 'per_unit':
    case 'unit_price':
      return 'unit_amount'
    case 'percent':
    case 'discount':
      return 'percent'
    case 'free':
      return 'free'
    default:
      return ''
  }
}

function employeeOrderItemQuantityBasis(item: EmployeeOrderDraftItem): string {
  const raw = String(item.price_source_json || '').trim()
  if (!raw) return ''
  try {
    const source = JSON.parse(raw) as { quantity_basis?: unknown }
    return String(source.quantity_basis || '').trim()
  } catch {
    return ''
  }
}

function employeeOrderItemDiscountUnits(item: EmployeeOrderDraftItem): number {
  const qty = Math.max(Number(item.qty || 0), 0)
  if (item.retail_order) return qty
  if (employeeOrderItemQuantityBasis(item) === 'sales_spec_count') return qty
  const productKind = String(item.product_kind || '').trim().toLowerCase()
  const salesUnit = String(item.sales_unit || '').trim().toLowerCase()
  if (productKind === 'drip_bag' || ['bag', '袋', 'box', '盒'].includes(salesUnit)) return qty
  const specG = Math.max(Number(item.spec_g || 0), 0)
  if (specG <= 0) return qty
  return specG * qty / (specG >= 1000 ? 1000 : 454)
}

export function employeeOrderItemDiscountAmount(item: EmployeeOrderDraftItem): number {
  const baseLineTotal = employeeOrderItemBaseTotal(item)
  if (baseLineTotal <= 0) return 0
  const discountValue = Math.max(Number(item.discount_value || 0), 0)
  let discountAmount = 0
  switch (normalizedEmployeeOrderDiscountType(item.discount_type)) {
    case 'free':
      discountAmount = baseLineTotal
      break
    case 'amount':
      discountAmount = Math.min(discountValue, baseLineTotal)
      break
    case 'unit_amount':
      discountAmount = Math.min(discountValue * employeeOrderItemDiscountUnits(item), baseLineTotal)
      break
    case 'percent': {
      const payableRate = Math.min(discountValue, 100)
      discountAmount = Math.max(baseLineTotal - (baseLineTotal * payableRate / 100), 0)
      break
    }
    default:
      discountAmount = 0
  }
  return roundEmployeeOrderMoney(discountAmount)
}

function withEmployeeOrderItemDiscount(item: EmployeeOrderDraftItem): EmployeeOrderDraftItem {
  return {
    ...item,
    discount_amount: employeeOrderItemDiscountAmount(item),
  }
}

export function employeeOrderTierForQuantity(spec?: EmployeeOrderProductSpec, quantity = 0) {
  const tiers = [...(spec?.tiers || [])]
  if (!tiers.length) return undefined
  const qty = Number(quantity)
  if (!Number.isFinite(qty) || qty <= 0) return undefined
  const ranked = tiers.map((tier, index) => ({
    tier,
    index,
    min: Number(tier.min_qty ?? tier.min ?? 0),
    max: Number(tier.max_qty ?? tier.max ?? 0),
  }))
  const matching = ranked
    .filter((row) => qty >= row.min && (row.max <= 0 || qty <= row.max))
    .sort((left, right) => right.min - left.min || left.index - right.index)
  return matching[0]?.tier
}

export function firstSpecUnitPrice(spec?: EmployeeOrderProductSpec, quantity = 0) {
  const tier = employeeOrderTierForQuantity(spec, quantity)
  return Number(tier?.unit_price || tier?.price || 0)
}

function employeeOrderSpecUsesTierTemplate(spec?: EmployeeOrderProductSpec): boolean {
  const tiers = spec?.tiers || []
  if (tiers.length > 1) return true
  return tiers.some((tier) => {
    const raw = String(tier.price_source_json || '').trim()
    if (!raw) return false
    try {
      const source = JSON.parse(raw) as {
        pricing_mode?: unknown
        tier_template_id?: unknown
        template_id?: unknown
        template_tier_id?: unknown
      }
      return String(source.pricing_mode || '').trim() === 'tier_template'
        || Number(source.tier_template_id || 0) > 0
        || Number(source.template_id || 0) > 0
        || Number(source.template_tier_id || 0) > 0
    } catch {
      return false
    }
  })
}

export function employeeOrderItemFromSpec(
  item: EmployeeOrderDraftItem,
  family: EmployeeOrderProductFamily,
  spec: EmployeeOrderProductSpec,
): EmployeeOrderDraftItem {
  const tier = employeeOrderTierForQuantity(spec, Number(item.qty || 0))
  const publicationID = Number(
    tier?.publication_id
    || tier?.bean_list_publication_id
    || spec.default_publication_id
    || family.default_publication_id
    || 0,
  )
  const publicationVersion = String(
    tier?.publication_version_no
    || tier?.bean_list_version_no
    || spec.default_publication_version_no
    || family.default_publication_version_no
    || '',
  )
  const cutover = miniappProductMigrationState({ ...family, ...spec } as unknown as Record<string, unknown>) === 'cutover'
  return withEmployeeOrderItemDiscount({
    ...item,
    product_family_key: employeeOrderProductFamilyKey(family),
    product_family_id: Number(family.parent_product_id || 0),
    customer_product_alias_id: Number(family.customer_product_alias_id || 0),
    product_id: cutover ? Number(family.parent_product_id || 0) : Number(spec.product_id || spec.sku_id || 0),
    migration_state: cutover ? 'cutover' : 'legacy',
    bom_spec_id: cutover ? Number(spec.bom_spec_id || 0) : 0,
    bom_variant_id: cutover ? Number(spec.bom_variant_id || 0) : 0,
    product_name: family.name,
    product_kind: spec.product_kind || family.product_kind || 'roasted_bean',
    spec_label: productSpecLabel(spec),
    spec_g: cutover ? 0 : productSpecWeightG(spec),
    sales_unit: spec.inventory_unit || spec.sales_unit || tier?.sales_unit || '袋',
    unit_bag_count: Number(spec.unit_bag_count || tier?.unit_bag_count || 0),
    unit_bean_g: Number(spec.unit_bean_g || 0),
    unit_price: firstSpecUnitPrice(spec, Number(item.qty || 0)),
    bean_list_publication_id: publicationID,
    bean_list_version_no: publicationVersion,
    price_override: false,
    price_source_json: String(tier?.price_source_json || ''),
    validation_error: tier || Number(item.qty || 0) <= 0 ? '' : '当前数量没有匹配的价格档，请调整数量',
  })
}

export function employeeOrderItemForSpecSelection(
  item: EmployeeOrderDraftItem,
  family: EmployeeOrderProductFamily,
  spec: EmployeeOrderProductSpec,
): EmployeeOrderDraftItem {
  if (!employeeOrderSpecUsesTierTemplate(spec)) {
    return employeeOrderItemFromSpec(item, family, spec)
  }
  const selected = employeeOrderItemFromSpec({ ...item, qty: 0 }, family, spec)
  return withEmployeeOrderItemDiscount({
    ...selected,
    qty: 0,
    unit_price: 0,
    price_override: false,
    price_source_json: '',
    validation_error: '',
  })
}

export function repriceEmployeeOrderItemForQuantity(
  item: EmployeeOrderDraftItem,
  family?: EmployeeOrderProductFamily,
): EmployeeOrderDraftItem {
  if (!family) return withEmployeeOrderItemDiscount({ ...item })
  const spec = (family.specs || []).find(
    (candidate) => item.migration_state === 'cutover'
      ? Number(candidate.bom_spec_id || 0) === Number(item.bom_spec_id || 0)
      : Number(candidate.product_id || candidate.sku_id || 0) === Number(item.product_id || 0),
  )
  if (!spec) return withEmployeeOrderItemDiscount({ ...item })
  const repriced = employeeOrderItemFromSpec(item, family, spec)
  return item.price_override
    ? withEmployeeOrderItemDiscount({
        ...repriced,
        unit_price: Number(item.unit_price || 0),
        price_override: true,
      })
    : repriced
}

export function applyEmployeeOrderQuantityChange(
  item: EmployeeOrderDraftItem,
  family: EmployeeOrderProductFamily | undefined,
  quantity: unknown,
): { accepted: boolean; item: EmployeeOrderDraftItem; error: string } {
  const nextQuantity = Number(quantity)
  const quantityLabel = String(quantity ?? '').trim() || '空值'
  if (!Number.isFinite(nextQuantity) || nextQuantity <= 0) {
    return {
      accepted: false,
      item: { ...item },
      error: `数量 ${quantityLabel} 不正确，原数量和单价已保留`,
    }
  }
  if (!family) {
    return {
      accepted: false,
      item: { ...item },
      error: '当前商品规格无法刷新价格，原数量和单价已保留',
    }
  }
  const repriced = repriceEmployeeOrderItemForQuantity({ ...item, qty: nextQuantity }, family)
  if (repriced.validation_error || !Number.isFinite(Number(repriced.unit_price)) || Number(repriced.unit_price) <= 0) {
    return {
      accepted: false,
      item: { ...item },
      error: `数量 ${quantityLabel} 没有匹配的阶梯价格，原数量和单价已保留`,
    }
  }
  return { accepted: true, item: repriced, error: '' }
}

function historicalOrderSpecLabel(item: EmployeeOrderDetailItem): string {
  const explicit = String(item.spec || '').trim()
  if (!explicit) return '默认规格'
  if (/^\d+(?:\.\d+)?$/.test(explicit)) return `${explicit}g`
  return explicit
}

function historicalOrderSpecWeightG(item: EmployeeOrderDetailItem): number {
  const explicit = Number(item.spec_g || 0)
  if (Number.isFinite(explicit) && explicit > 0) return explicit
  const match = String(item.spec || '').match(/([0-9]+(?:\.[0-9]+)?)/)
  return match ? Number(match[1]) : 0
}

export function hydrateEmployeeOrderEditItems(
  detailItems: EmployeeOrderDetailItem[] = [],
  families: EmployeeOrderProductFamily[] = [],
  customerID = 0,
  retailOrder = false,
): EmployeeOrderDraftItem[] {
  if (!detailItems.length) return [createEmployeeOrderItem()]
  const available = customerProductFamilies(families, customerID)

  return detailItems.map((detail, index) => {
    const aliasID = Number(detail.customer_product_alias_id || 0)
    const productID = Number(detail.product_id || 0)
    const bomSpecID = Number(detail.bom_spec_id || 0)
    const cutover = bomSpecID > 0 || detail.migration_state === 'cutover'
    const matchingFamilies = available.filter((candidate) => (candidate.specs || []).some(
      (spec) => cutover
        ? Number(candidate.parent_product_id || 0) === productID && Number(spec.bom_spec_id || 0) === bomSpecID
        : Number(spec.product_id || spec.sku_id || 0) === productID,
    ))
    const aliasCandidates = aliasID > 0
      ? matchingFamilies.filter((family) => Number(family.customer_product_alias_id || 0) === aliasID)
      : matchingFamilies
    const publicCandidates = aliasCandidates.filter((family) => Number(family.customer_id || 0) === 0
      && Number(family.customer_product_alias_id || 0) === 0)
    const ambiguousAlias = aliasID <= 0 && publicCandidates.length !== 1 && aliasCandidates.length > 1
    const family = aliasID > 0
      ? aliasCandidates[0]
      : (publicCandidates.length === 1 ? publicCandidates[0] : (aliasCandidates.length === 1 ? aliasCandidates[0] : undefined))
    const spec = family?.specs.find(
      (candidate) => cutover
        ? Number(candidate.bom_spec_id || 0) === bomSpecID
        : Number(candidate.product_id || candidate.sku_id || 0) === productID,
    )
    const historical = withEmployeeOrderItemDiscount({
      ...createEmployeeOrderItem(`edit-${detail.item_id || detail.id || index + 1}`),
      item_id: Number(detail.item_id || detail.id || 0),
      customer_product_alias_id: aliasID,
      product_id: productID,
      product_family_id: Number(detail.parent_product_id || (cutover ? productID : 0)),
      migration_state: cutover ? 'cutover' : 'legacy',
      bom_spec_id: bomSpecID,
      bom_variant_id: Number(detail.bom_variant_id || 0),
      product_name: String(
        detail.customer_product_display_name_snapshot
        || detail.product_name_snapshot
        || detail.product_name
        || '',
      ),
      product_kind: String(detail.product_kind || 'roasted_bean'),
      spec_label: historicalOrderSpecLabel(detail),
      spec_g: historicalOrderSpecWeightG(detail),
      sales_unit: String(detail.sales_unit || detail.unit || '袋'),
      unit_bag_count: Number(detail.unit_bag_count || 0),
      unit_bean_g: Number(detail.unit_bean_g || 0),
      qty: Number(detail.qty || 0),
      unit_price: Number(detail.unit_price || 0),
      bean_list_publication_id: Number(detail.bean_list_publication_id || 0),
      bean_list_version_no: String(detail.bean_list_version_no || ''),
      price_override: Boolean(detail.price_override),
      price_source_json: String(detail.price_source_json || ''),
      discount_type: String(detail.discount_type || ''),
      discount_value: Number(detail.discount_value || 0),
      discount_amount: Number(detail.discount_amount || 0),
      retail_order: retailOrder,
    })

    if (!family || !spec) {
      return {
        ...historical,
        validation_error: ambiguousAlias
          ? '该历史商品对应多个客户别名，请重新选择当前可售商品'
          : '该历史商品或规格已不在当前价格表，请重新选择当前可售规格',
      }
    }

    const current = employeeOrderItemFromSpec(historical, family, spec)
    const historicalUnitPrice = Number(detail.unit_price || 0)
    const currentUnitPrice = Number(current.unit_price || 0)
    const hasOverrideSemantic = typeof detail.price_override === 'boolean'
    const priceOverride = hasOverrideSemantic
      ? detail.price_override === true
      : Math.abs(historicalUnitPrice - currentUnitPrice) > 0.000001
    return withEmployeeOrderItemDiscount({
      ...current,
      qty: Number(detail.qty || 0),
      unit_price: priceOverride ? historicalUnitPrice : currentUnitPrice,
      price_override: priceOverride,
    })
  })
}

export function revalidateEmployeeOrderItems(
  items: EmployeeOrderDraftItem[],
  families: EmployeeOrderProductFamily[],
  customerID: number,
  options: { preserveUnitPrice?: boolean; preserveManualPrice?: boolean; preserveUnavailable?: boolean } = {},
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
        (row) => item.migration_state === 'cutover'
          ? Number(row.bom_spec_id || 0) === Number(item.bom_spec_id || 0)
          : Number(row.product_id || row.sku_id || 0) === Number(item.product_id),
      )
      if (spec) {
        const validated = employeeOrderItemFromSpec(item, family, spec)
        const preservePrice = options.preserveUnitPrice || (options.preserveManualPrice && item.price_override)
        return preservePrice
          ? withEmployeeOrderItemDiscount({
              ...validated,
              unit_price: Number(item.unit_price || 0),
              price_override: Boolean(item.price_override)
                || Math.abs(Number(item.unit_price || 0) - Number(validated.unit_price || 0)) > 0.000001,
            })
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
    .map((item) => {
      const identity = buildMiniappProductSpecIdentity(item)
      return {
        product_id: identity.product_id,
        item_id: Number(item.item_id || 0),
        parent_product_id: item.migration_state === 'cutover' ? identity.product_id : Number(item.product_family_id || 0),
        ...(item.migration_state === 'cutover' ? {
          bom_spec_id: identity.bom_spec_id,
          bom_variant_id: identity.bom_variant_id,
        } : {}),
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
        bean_list_publication_id: Number(item.bean_list_publication_id || 0),
        bean_list_version_no: String(item.bean_list_version_no || ''),
        price_override: Boolean(item.price_override),
        price_source_json: String(item.price_source_json || ''),
      }
    })
}

export function employeeOrderItemsTotal(items: EmployeeOrderDraftItem[]): number {
  const total = items.reduce((sum, item) => {
    if (Number(item.product_id || 0) <= 0) return sum
    return sum + employeeOrderItemBaseTotal(item)
  }, 0)
  return roundEmployeeOrderMoney(total)
}

function employeeOrderLineDiscountTotal(items: EmployeeOrderDraftItem[]): number {
  return roundEmployeeOrderMoney(items.reduce((total, item) => {
    if (Number(item.product_id || 0) <= 0) return total
    return total + employeeOrderItemDiscountAmount(item)
  }, 0))
}

export function employeeOrderGrandTotal(
  items: EmployeeOrderDraftItem[],
  shippingAmount = 0,
  discountAmount = 0,
  outsourceAmount = 0,
  roundToInt = false,
): number {
  const total = (
    employeeOrderItemsTotal(items)
    + Number(shippingAmount || 0)
    - employeeOrderLineDiscountTotal(items)
    - Number(discountAmount || 0)
    + Number(outsourceAmount || 0)
  )
  return roundEmployeeOrderMoney(roundToInt ? Math.trunc(total) : total)
}

function nonNegativeMoney(value: unknown): number {
  const parsed = Number(value || 0)
  return Number.isFinite(parsed) ? Math.max(parsed, 0) : 0
}

export function employeeOrderOutsourceTotal(order: {
  outsource_material_fee?: unknown
  outsource_roast_fee?: unknown
  outsource_packaging_fee?: unknown
  outsource_manual_fee?: unknown
  outsource_tax_fee?: unknown
  outsource_other_fee?: unknown
} = {}): number {
  return roundEmployeeOrderMoney([
    order.outsource_material_fee,
    order.outsource_roast_fee,
    order.outsource_packaging_fee,
    order.outsource_manual_fee,
    order.outsource_tax_fee,
    order.outsource_other_fee,
  ].reduce<number>((total, value) => total + nonNegativeMoney(value), 0))
}

export function employeeOrderEditableOrderDiscount(
  totalDiscount: unknown,
  items: Array<{ discount_amount?: unknown }> = [],
  explicitOrderDiscount?: unknown,
): number {
  const hasExplicitOrderDiscount = explicitOrderDiscount !== undefined
    && explicitOrderDiscount !== null
    && String(explicitOrderDiscount).trim() !== ''
  if (hasExplicitOrderDiscount) return nonNegativeMoney(explicitOrderDiscount)
  const preservedLineDiscount = items.reduce(
    (total, item) => total + nonNegativeMoney(item.discount_amount),
    0,
  )
  return Math.round(Math.max(nonNegativeMoney(totalDiscount) - preservedLineDiscount, 0) * 100) / 100
}

export function isEmployeeOrderNonNegativeMoney(value: unknown): boolean {
  const amount = Number(value)
  return String(value ?? '').trim() !== '' && Number.isFinite(amount) && amount >= 0
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
