import { isDripProduct } from './drip-product.js'

export const CUSTOM_SPEC_VALUE = 'custom'
export const COMMON_SPEC_GRAMS = [36, 80, 100, 227, 454, 500, 1000, 2500]
export const orderReceiptMethodOptions = [
  '微信支付',
  '支付宝',
  '银行转账',
  '对公银行',
  '现金',
  'POS刷卡',
  '其他',
]

export function toNumber(value) {
  const n = Number.parseFloat(String(value ?? '').trim())
  return Number.isFinite(n) ? n : 0
}

export function toInt(value) {
  const n = Number.parseInt(String(value ?? '').trim(), 10)
  return Number.isFinite(n) ? n : 0
}

export function exactRetailPrice(product, specG) {
  const prices = {
    100: toNumber(product?.retail_price_100g),
    200: toNumber(product?.retail_price_200g),
    227: toNumber(product?.retail_price_227g),
    250: toNumber(product?.retail_price_250g),
  }
  return prices[toInt(specG)] || 0
}

export function retailPackagePrice(product, specG) {
  const spec = toInt(specG)
  const exact = exactRetailPrice(product, spec)
  if (exact > 0) return exact
  const base = toNumber(product?.retail_price_227g)
  if (base <= 0 || spec <= 0) return 0
  return Math.ceil((base * spec) / 227)
}

export function retailSpecOptions(product, retailOrder) {
  if (!retailOrder) return []
  const specs = [...new Set([
    ...COMMON_SPEC_GRAMS,
    ...(product?.retail_specs || []).map(toInt).filter((spec) => spec > 0),
  ])]
    .sort((a, b) => a - b)
  return [
    ...specs.map((spec) => ({ label: formatSpecLabel(spec), value: String(spec) })),
    { label: '自定义克数', value: CUSTOM_SPEC_VALUE },
  ]
}

export function formatSpecLabel(specG) {
  const spec = toInt(specG)
  if (spec === 2500) return '2.5kg'
  return `${spec}g`
}

function parseUnitConversion(value) {
  if (!value) return {}
  if (typeof value === 'object') return value
  try {
    const parsed = JSON.parse(String(value))
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

export function productOrderUnit(product) {
  return String(product?.order_unit || '').trim()
}

export function productOrderUnitSpecG(product) {
  const orderUnit = productOrderUnit(product)
  if (!orderUnit || orderUnit === 'kg') return 0
  const conversion = parseUnitConversion(product?.unit_conversion_json)
  const rule = conversion?.[orderUnit] || conversion?.[orderUnit.toLowerCase?.()]
  const kg = toNumber(rule?.kg ?? rule?.KG)
  if (kg > 0) return Math.round(kg * 1000)
  const g = toNumber(rule?.g ?? rule?.G)
  if (g > 0) return Math.round(g)
  return 0
}

function productOrderUnitSpecOption(product) {
  const unit = productOrderUnit(product)
  const specG = productOrderUnitSpecG(product)
  if (!unit || specG <= 0) return null
  return { label: `${unit}（${formatSpecLabel(specG)}）`, value: String(specG), orderUnit: unit }
}

export function normalizedProductKind(productOrKind) {
  const raw = typeof productOrKind === 'object'
    ? productOrKind?.product_kind
    : productOrKind
  const kind = String(raw || '').trim()
  if (kind === 'green_bean') return 'green_bean'
  if (kind === 'drip_bag') return 'drip_bag'
  if (kind === 'instant_coffee' || kind === 'instant') return 'instant_coffee'
  return 'roasted'
}

export function productKindLabel(productOrKind) {
  const kind = normalizedProductKind(productOrKind)
  if (kind === 'green_bean') return '生豆'
  if (kind === 'drip_bag') return '挂耳'
  if (kind === 'instant_coffee') return '速溶咖啡'
  return '熟豆'
}

export function productKindBadgeClass(productOrKind) {
  const kind = normalizedProductKind(productOrKind)
  if (kind === 'green_bean') return 'kind-green'
  if (kind === 'drip_bag') return 'kind-drip'
  if (kind === 'instant_coffee') return 'kind-instant'
  return 'kind-roasted'
}

function orderFamilyText(value) {
  return String(value ?? '').trim()
}

function orderFamilyObject(value) {
  if (value && typeof value === 'object' && !Array.isArray(value)) return value
  if (!value) return {}
  try {
    const parsed = JSON.parse(String(value))
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

function orderFamilyID(...values) {
  for (const value of values) {
    const id = toInt(value)
    if (id > 0) return id
  }
  return 0
}

function orderSpecLabel(spec = {}) {
  const effective = orderFamilyObject(spec.effective_sales_spec ?? spec.effectiveSalesSpec)
  const explicit = orderFamilyText(
    spec.spec_label
      ?? spec.specLabel
      ?? effective.spec_label
      ?? effective.specLabel
      ?? effective.spec_name
      ?? effective.specName
      ?? spec.sales_spec
      ?? spec.salesSpec,
  )
  if (explicit) return explicit
  const qty = toNumber(spec.net_content_qty ?? spec.netContentQty)
  const unit = orderFamilyText(spec.net_content_unit ?? spec.netContentUnit)
  if (qty > 0 && unit) return `${trimNumber(qty)}${unit}`
  const tierSpec = (Array.isArray(spec.tiers) ? spec.tiers : []).find((tier) => toInt(tier?.spec_g) > 0)
  if (tierSpec) return formatSpecLabel(tierSpec.spec_g)
  return orderFamilyText(spec.sku_name ?? spec.skuName ?? spec.name)
}

export function orderSpecWeightG(spec = {}) {
  const explicit = toNumber(spec.spec_g ?? spec.specG)
  if (explicit > 0) return explicit
  const qty = toNumber(spec.net_content_qty ?? spec.netContentQty)
  const unit = orderFamilyText(spec.net_content_unit ?? spec.netContentUnit).toLowerCase()
  if (qty > 0) {
    if (unit === 'g' || unit === '克') return qty
    if (unit === 'kg' || unit === '千克' || unit === '公斤') return qty * 1000
    if (unit === 'lb' || unit === 'lbs' || unit === '磅') return qty * 454
    if (unit === 'mg' || unit === '毫克') return qty / 1000
  }
  const label = orderSpecLabel(spec)
  const kg = label.match(/([0-9]+(?:\.[0-9]+)?)\s*(?:kg|千克|公斤)/i)
  if (kg) return toNumber(kg[1]) * 1000
  const gram = label.match(/([0-9]+(?:\.[0-9]+)?)\s*(?:g|克)/i)
  if (gram) return toNumber(gram[1])
  if (/^(?:lb|lbs|磅)$/i.test(label)) return 454
  const unitBeanG = toNumber(spec.unit_bean_g ?? spec.unitBeanG)
  const bagCount = Math.max(1, toInt(spec.unit_bag_count ?? spec.unitBagCount))
  if (unitBeanG > 0) return unitBeanG * bagCount
  return 0
}

function normalizeOrderFamilySpec(raw = {}, sourceProduct = null) {
  const source = sourceProduct || raw.source_product || raw.sourceProduct || {}
  const tiers = Array.isArray(raw.tiers)
    ? raw.tiers
    : (Array.isArray(source?.tiers) ? source.tiers : [])
  const skuID = orderFamilyID(raw.sku_id, raw.skuID, raw.product_id, raw.id, source?.id)
  return {
    ...source,
    ...raw,
    sku_id: skuID,
    sku_name: orderFamilyText(raw.sku_name ?? raw.skuName ?? raw.name ?? source?.name),
    product_code: orderFamilyText(raw.product_code ?? raw.productCode ?? raw.code ?? source?.product_code ?? source?.code),
    spec_label: orderSpecLabel({ ...source, ...raw, tiers }),
    is_default_sku: Boolean(raw.is_default_sku ?? raw.isDefaultSku ?? source?.is_default_sku),
    net_content_qty: toNumber(raw.net_content_qty ?? raw.netContentQty ?? source?.net_content_qty),
    net_content_unit: orderFamilyText(raw.net_content_unit ?? raw.netContentUnit ?? source?.net_content_unit),
    order_unit: orderFamilyText(raw.order_unit ?? raw.orderUnit ?? source?.order_unit),
    quote_unit: orderFamilyText(raw.quote_unit ?? raw.quoteUnit ?? source?.quote_unit),
    product_kind: orderFamilyText(raw.product_kind ?? raw.productKind ?? source?.product_kind),
    tiers,
    source_product: sourceProduct || raw.source_product || raw.sourceProduct || null,
  }
}

function normalizeExplicitOrderFamily(raw = {}) {
  const parentProductID = orderFamilyID(raw.parent_product_id, raw.parentProductID, raw.product_id, raw.id)
  const parentProductName = orderFamilyText(
    raw.parent_product_name
      ?? raw.parentProductName
      ?? raw.product_name_snapshot
      ?? raw.productNameSnapshot
      ?? raw.product_record_name
      ?? raw.name,
  )
  const displayName = orderFamilyText(
    raw.customer_product_display_name
      ?? raw.customerProductDisplayName
      ?? raw.display_name
      ?? raw.name
      ?? parentProductName,
  ) || parentProductName
  const specs = (Array.isArray(raw.specs) ? raw.specs : [])
    .map((spec) => normalizeOrderFamilySpec(spec))
    .filter((spec) => spec.sku_id > 0)
  const searchCode = [
    raw.code,
    raw.product_code,
    raw.alias_name,
    raw.customer_product_display_name,
    parentProductName,
    ...specs.flatMap((spec) => [spec.sku_name, spec.product_code, spec.spec_label]),
  ].map(orderFamilyText).filter(Boolean).join(' ')
  const familyKey = orderProductFamilyIdentity({
    ...raw,
    parent_product_id: parentProductID,
  })
  return {
    ...raw,
    id: parentProductID,
    parent_product_id: parentProductID,
    parent_product_name: parentProductName,
    product_name_snapshot: parentProductName,
    name: displayName,
    alias_name: orderFamilyText(raw.alias_name ?? raw.customer_product_display_name ?? raw.customerProductDisplayName),
    customer_product_display_name: orderFamilyText(raw.customer_product_display_name ?? raw.customerProductDisplayName ?? displayName),
    default_sku_id: orderFamilyID(raw.default_sku_id, raw.defaultSkuID),
    family_key: familyKey,
    specs,
    tiers: specs.flatMap((spec) => spec.tiers || []),
    code: searchCode,
    __order_product_family: true,
    __order_concrete_price_family: raw.__order_concrete_price_family === undefined
      ? true
      : Boolean(raw.__order_concrete_price_family),
  }
}

function fallbackOrderFamilies(products = []) {
  const groups = new Map()
  const legacyProducts = []
  for (const product of products || []) {
    const productID = orderFamilyID(product?.id, product?.product_id)
    if (productID <= 0) continue
    const tiers = Array.isArray(product?.tiers) ? product.tiers : []
    const concreteTiers = tiers.filter(isConcreteOrderPublicationTier)
    if (!concreteTiers.length) {
      legacyProducts.push({
        ...product,
        id: productID,
        parent_product_id: orderFamilyID(product?.parent_product_id, product?.parentProductID) || productID,
        tiers: tiers.filter((tier) => !isConcreteOrderPublicationTier(tier)),
        __order_product_family: false,
        __order_concrete_price_family: false,
        __order_legacy_price_product: true,
      })
      continue
    }
    const annotatedParentID = orderFamilyID(product?.parent_product_id, product?.parentProductID)
    const parentProductID = annotatedParentID || productID
    const key = String(parentProductID)
    if (!groups.has(key)) groups.set(key, [])
    groups.get(key).push({ ...product, tiers: concreteTiers })
  }
  const concreteFamilies = [...groups.values()].map((group) => {
    const first = group[0] || {}
    const parentProductID = orderFamilyID(first.parent_product_id, first.parentProductID, first.id)
    const parentProductName = orderFamilyText(
      first.parent_product_name
        ?? first.parentProductName
        ?? first.product_name_snapshot
        ?? first.productNameSnapshot
        ?? first.product_record_name
        ?? first.name,
    )
    const displayName = orderFamilyText(
      first.customer_product_display_name
        ?? first.customerProductDisplayName
        ?? first.parent_product_display_name
        ?? parentProductName,
    ) || parentProductName
    const specs = group.map((product) => normalizeOrderFamilySpec({
      ...product,
      sku_id: product.id,
      sku_name: product.sku_name || product.name,
      spec_label: product.spec_label || product.effective_sales_spec || product.sales_spec,
    }, product))
    const concrete = group.some((product) => (product?.tiers || []).some(isConcreteOrderPublicationTier))
    return normalizeExplicitOrderFamily({
      ...first,
      parent_product_id: parentProductID,
      parent_product_name: parentProductName,
      name: displayName,
      default_sku_id: orderFamilyID(first.default_sku_id, specs.find((spec) => spec.is_default_sku)?.sku_id),
      specs,
      __order_concrete_price_family: concrete,
    })
  }).map((family, index) => ({
    ...family,
    __order_concrete_price_family: Boolean(family.__order_concrete_price_family),
    __order_fallback_index: index,
  }))
  return [...concreteFamilies, ...legacyProducts]
}

export function normalizeOrderProductFamilies(productFamilies = [], products = []) {
  if (Array.isArray(productFamilies) && productFamilies.length) {
    const productsByFamilySKU = new Map()
    const productsBySKU = new Map()
    for (const product of products || []) {
      const skuID = orderFamilyID(product?.sku_id, product?.skuID, product?.id, product?.product_id)
      if (skuID <= 0) continue
      productsByFamilySKU.set(`${orderProductFamilyIdentity(product)}:${skuID}`, product)
      if (!productsBySKU.has(skuID)) productsBySKU.set(skuID, [])
      productsBySKU.get(skuID).push(product)
    }
    const concreteFamilies = productFamilies.map((raw) => {
      const familyIdentity = orderProductFamilyIdentity(raw)
      const enrichedSpecs = (raw?.specs || []).map((spec) => {
        const skuID = orderFamilyID(spec?.sku_id, spec?.skuID, spec?.product_id, spec?.id)
        const exact = productsByFamilySKU.get(`${familyIdentity}:${skuID}`)
        const candidates = productsBySKU.get(skuID) || []
        const source = exact || (candidates.length === 1 ? candidates[0] : null)
        return { ...(source || {}), ...spec }
      })
      const family = normalizeExplicitOrderFamily({ ...raw, specs: enrichedSpecs })
      family.py = orderFamilyText(raw?.py) || family.specs.map((spec) => orderFamilyText(spec?.py)).filter(Boolean).join(' ')
      family.pyi = orderFamilyText(raw?.pyi) || family.specs.map((spec) => orderFamilyText(spec?.pyi)).filter(Boolean).join(' ')
      return family
    }).filter((family) => family.parent_product_id > 0 && family.specs.length)
    const concreteParentIDs = new Set(concreteFamilies.map((family) => family.parent_product_id))
    const unrelatedLegacyProducts = (products || []).filter((product) => {
      const parentProductID = orderFamilyID(product?.parent_product_id, product?.parentProductID, product?.id)
      if (concreteParentIDs.has(parentProductID)) return false
      return (product?.tiers || []).some((tier) => tierPublicationID(tier) > 0 && !isConcreteOrderPublicationTier(tier))
    })
    return [...concreteFamilies, ...fallbackOrderFamilies(unrelatedLegacyProducts)]
  }
  return fallbackOrderFamilies(products)
}

export function isOrderProductFamily(family = {}) {
  return Boolean(family?.__order_product_family) && Array.isArray(family?.specs) && family.specs.length > 0
}

export function orderProductFamilyIdentity(family = {}) {
  return [
    orderFamilyID(family?.customer_id, family?.customerID),
    orderFamilyID(family?.parent_product_id, family?.parentProductID, family?.id),
    orderFamilyID(family?.customer_product_alias_id, family?.customerProductAliasID),
  ].join(':')
}

export function orderProductFamilyForContext(families = [], reference = {}) {
  const candidates = Array.isArray(families) ? families : []
  const familyKey = orderFamilyText(reference?.product_family_key ?? reference?.family_key)
  if (familyKey) {
    const exact = candidates.find((family) => orderProductFamilyIdentity(family) === familyKey)
    if (exact) return exact
  }
  const aliasID = orderFamilyID(reference?.customer_product_alias_id, reference?.customerProductAliasID)
  if (aliasID > 0) {
    const aliased = candidates.find((family) => orderFamilyID(family?.customer_product_alias_id, family?.customerProductAliasID) === aliasID)
    if (aliased) return aliased
  }
  const customerID = orderFamilyID(reference?.customer_id, reference?.customerID)
  if (customerID > 0) {
    const customerFamily = candidates.find((family) => orderFamilyID(family?.customer_id, family?.customerID) === customerID)
    if (customerFamily) return customerFamily
  }
  return candidates.find((family) => orderFamilyID(family?.customer_id, family?.customerID) === 0) || candidates[0] || null
}

const ORDER_PRODUCT_KIND_FILTERS = [
  { value: 'roasted', label: '熟豆' },
  { value: 'drip_bag', label: '挂耳' },
  { value: 'green_bean', label: '生豆' },
  { value: 'instant_coffee', label: '速溶咖啡' },
]

export function orderProductKindFilterOptions(families = []) {
  const availableKinds = new Set((families || []).map(normalizedProductKind))
  return [
    { value: '', label: '全部' },
    ...ORDER_PRODUCT_KIND_FILTERS.filter((option) => availableKinds.has(option.value)),
  ]
}

export function orderProductFamilyOptions(families = [], query = '', productKindFilter = '') {
  const q = orderFamilyText(query).toLowerCase()
  const kind = String(productKindFilter || '').trim()
  return (families || []).filter((family) => {
    if (kind && normalizedProductKind(family) !== kind) return false
    if (!q) return true
    const haystack = [
      family?.name,
      family?.parent_product_name,
      family?.alias_name,
      family?.customer_product_display_name,
      family?.py,
      family?.pyi,
      family?.code,
      ...(family?.specs || []).flatMap((spec) => [spec?.spec_label, spec?.sku_name, spec?.product_code]),
    ].map(orderFamilyText).join(' ').toLowerCase()
    return haystack.includes(q)
  })
}

export function closeOrderProductDropdowns(rows = [], keepKey = '') {
  const preservedKey = String(keepKey || '')
  for (const row of rows || []) {
    if (!preservedKey || String(row?.key || '') !== preservedKey) row.product_open = false
  }
  return rows
}

export function orderFamilyForSKU(families = [], skuID = 0) {
  const id = toInt(skuID)
  if (id <= 0) return null
  return (families || []).find((family) => (family?.specs || []).some((spec) => toInt(spec?.sku_id) === id)) || null
}

export function orderFamilySpecsForPublication(family = {}, publicationID = 0) {
  const selectedPublicationID = toInt(publicationID)
  return (family?.specs || []).flatMap((spec) => {
    const tiers = (spec?.tiers || []).filter((tier) => {
      if (selectedPublicationID <= 0) return true
      return tierPublicationID(tier) === selectedPublicationID
    })
    if (!tiers.length && selectedPublicationID > 0) return []
    const tier = tiers[0] || {}
    const source = tierPriceSource(tier) || {}
    const sourceSnapshot = orderFamilyObject(source.effective_sales_spec ?? source.effectiveSalesSpec)
    const directSnapshot = orderFamilyObject(tier.effective_sales_spec ?? tier.effectiveSalesSpec)
    const effectiveSalesSpec = { ...sourceSnapshot, ...directSnapshot }
    const snapshotName = orderFamilyText(effectiveSalesSpec.spec_name ?? effectiveSalesSpec.specName)
    const snapshotLabel = orderFamilyText(
      effectiveSalesSpec.spec_label
        ?? effectiveSalesSpec.specLabel
        ?? effectiveSalesSpec.spec_name
        ?? effectiveSalesSpec.specName,
    )
    const snapshotSalesUnit = orderFamilyText(effectiveSalesSpec.sales_unit ?? effectiveSalesSpec.salesUnit)
    const snapshotNetContentQty = toNumber(effectiveSalesSpec.net_content_qty ?? effectiveSalesSpec.netContentQty)
    const snapshotNetContentUnit = orderFamilyText(effectiveSalesSpec.net_content_unit ?? effectiveSalesSpec.netContentUnit)
    const snapshotProductKind = orderFamilyText(
      effectiveSalesSpec.product_kind
        ?? effectiveSalesSpec.productKind
        ?? tier.product_kind
        ?? tier.productKind,
    )
    const snapshotInventoryUnit = orderFamilyText(effectiveSalesSpec.inventory_unit ?? effectiveSalesSpec.inventoryUnit)
    const snapshotConversion = orderFamilyObject(
      effectiveSalesSpec.inventory_conversion_json
        ?? effectiveSalesSpec.inventoryConversionJSON
        ?? effectiveSalesSpec.unit_conversion_json
        ?? effectiveSalesSpec.unitConversionJSON,
    )
    return [{
      ...spec,
      sku_name: snapshotName || spec.sku_name,
      spec_name: snapshotName || orderFamilyText(spec.spec_name ?? spec.specName),
      spec_label: snapshotLabel || spec.spec_label,
      spec_key: orderFamilyText(effectiveSalesSpec.spec_key ?? effectiveSalesSpec.specKey) || orderFamilyText(spec.spec_key ?? spec.specKey),
      sales_unit: snapshotSalesUnit || orderFamilyText(spec.sales_unit ?? spec.salesUnit),
      net_content_qty: snapshotNetContentQty > 0 ? snapshotNetContentQty : toNumber(spec.net_content_qty ?? spec.netContentQty),
      net_content_unit: snapshotNetContentUnit || orderFamilyText(spec.net_content_unit ?? spec.netContentUnit),
      inventory_unit: snapshotInventoryUnit || orderFamilyText(spec.inventory_unit ?? spec.inventoryUnit),
      inventory_conversion_json: Object.keys(snapshotConversion).length
        ? snapshotConversion
        : orderFamilyObject(spec.inventory_conversion_json ?? spec.inventoryConversionJSON),
      unit_conversion_json: Object.keys(snapshotConversion).length
        ? snapshotConversion
        : (spec.unit_conversion_json ?? spec.unitConversionJSON),
      product_kind: snapshotProductKind || orderFamilyText(spec.product_kind ?? spec.productKind),
      effective_sales_spec: Object.keys(effectiveSalesSpec).length
        ? effectiveSalesSpec
        : orderFamilyObject(spec.effective_sales_spec ?? spec.effectiveSalesSpec),
      tiers,
    }]
  })
}

export function orderFamilyMaintainedSpecs(family = {}) {
  return (family?.specs || []).map((spec) => ({
    ...spec,
    tiers: Array.isArray(spec?.tiers) ? spec.tiers : [],
  }))
}

function orderFamilyMaintainedSpecsForPublication(family = {}, publicationID = 0) {
  const selectedPublicationID = toInt(publicationID)
  const specs = orderFamilyMaintainedSpecs(family)
  if (selectedPublicationID <= 0) return specs
  return specs.filter((spec) => spec.tiers.some((tier) => tierPublicationID(tier) === selectedPublicationID))
}

export function orderFamilySearchScopeForPublication(family = {}, publicationID = 0) {
  const selectedPublicationID = toInt(publicationID)
  if (selectedPublicationID <= 0) return family
  const specs = orderFamilyMaintainedSpecsForPublication(family, selectedPublicationID)
  const codeParts = [
    family?.parent_product_code,
    family?.customer_item_code,
    ...specs.flatMap((spec) => [spec?.sku_code, spec?.product_code]),
  ].map(orderFamilyText).filter(Boolean)
  const code = [...new Set(codeParts)].join(' ')
  return { ...family, specs, code }
}

export function orderFamilySpecOptions(family = {}, publicationID = 0) {
  const specs = orderFamilyMaintainedSpecsForPublication(family, publicationID)
  return specs.map((spec) => ({
    label: orderSpecLabel(spec),
    value: String(toInt(spec.sku_id)),
    skuID: toInt(spec.sku_id),
  }))
}

export function orderFamilyDefaultSpec(family = {}, publicationID = 0) {
  const specs = orderFamilyMaintainedSpecsForPublication(family, publicationID)
  const defaultSkuID = orderFamilyID(family?.default_sku_id, family?.defaultSkuID)
  return specs.find((spec) => toInt(spec.sku_id) === defaultSkuID)
    || specs.find((spec) => spec.is_default_sku)
    || specs[0]
    || null
}

export function orderSpecSelectionAfterPublicationChange(family = {}, currentSkuID = 0, publicationID = 0) {
  const skuID = toInt(currentSkuID)
  if (skuID <= 0) return null
  const maintained = orderFamilyMaintainedSpecs(family)
    .find((spec) => toInt(spec.sku_id) === skuID)
  if (!maintained) return null
  const selectedPublicationID = toInt(publicationID)
  if (selectedPublicationID <= 0) return maintained
  return orderFamilySpecsForPublication({ specs: [maintained] }, selectedPublicationID)[0] || null
}

function orderFamilyTierVersion(tier = {}) {
  const source = tierPriceSource(tier) || {}
  return orderFamilyText(tier.version_no ?? tier.versionNo ?? source.version_no ?? source.bean_list_version_no ?? source.version)
}

function orderSpecSalesUnit(spec = {}, tier = {}) {
  const raw = orderFamilyText(spec.sales_unit ?? spec.salesUnit ?? tier.sales_unit ?? tier.salesUnit).toLowerCase()
  if (raw === 'box' || raw.includes('盒')) return 'box'
  if (raw === 'bag' || raw.includes('袋')) return 'bag'
  return ''
}

function orderSpecQuantityUnit(spec = {}, tier = {}) {
  const salesUnit = orderSpecSalesUnit(spec, tier)
  if (salesUnit === 'box') return '盒'
  if (salesUnit === 'bag') return '袋'
  const orderUnit = orderFamilyText(spec.order_unit ?? spec.orderUnit)
  if (['盒', '袋', '条', '瓶', '罐'].some((unit) => orderUnit.includes(unit))) return orderUnit
  return '件'
}

export function orderFamilySpecProduct(family = {}, spec = {}, publicationID = 0) {
  const selected = orderFamilySpecsForPublication({ specs: [spec] }, publicationID)[0] || { ...spec, tiers: [] }
  const sourceProduct = selected.source_product || selected.sourceProduct || {}
  const skuID = toInt(selected.sku_id)
  return {
    ...family,
    ...sourceProduct,
    ...selected,
    id: skuID,
    product_id: skuID,
    parent_product_id: toInt(family.parent_product_id || family.id),
    parent_product_name: orderFamilyText(family.parent_product_name || family.product_name_snapshot || family.name),
    name: orderFamilyText(family.name || family.parent_product_name),
    product_name_snapshot: orderFamilyText(family.parent_product_name || family.product_name_snapshot || family.name),
    effective_sales_spec: orderFamilyObject(selected.effective_sales_spec ?? selected.effectiveSalesSpec),
    tiers: selected.tiers || [],
    __order_product_family: true,
    __order_concrete_price_family: true,
  }
}

export function orderFamilySpecRowPatch(family = {}, spec = {}, publicationID = 0) {
  const product = orderFamilySpecProduct(family, spec, publicationID)
  const tier = product.tiers[0] || {}
  const source = tierPriceSource(tier) || {}
  const salesUnit = orderSpecSalesUnit(spec, tier)
  const unitBeanG = toNumber(spec.unit_bean_g ?? spec.unitBeanG ?? tier.unit_bean_g ?? tier.unitBeanG)
  const unitBagCount = Math.max(0, toInt(spec.unit_bag_count ?? spec.unitBagCount ?? tier.unit_bag_count ?? tier.unitBagCount))
  const parentName = orderFamilyText(family.parent_product_name || family.product_name_snapshot || family.name)
  const displayName = orderFamilyText(family.name || family.customer_product_display_name || parentName)
  return {
    parent_product_id: toInt(family.parent_product_id || family.id),
    parent_product_name: parentName,
    product_id: toInt(spec.sku_id),
    product_name: displayName,
    product_query: displayName,
    product_code: orderFamilyText(spec.product_code || spec.sku_name || `SKU-${toInt(spec.sku_id)}`),
    product_record_name: parentName,
    customer_product_alias_id: toInt(family.customer_product_alias_id),
    customer_product_display_name: orderFamilyText(family.customer_product_display_name || displayName),
    customer_item_code: orderFamilyText(family.customer_item_code),
    brand_name: orderFamilyText(family.brand_name),
    product_kind: orderFamilyText(spec.product_kind || family.product_kind || 'roasted_bean'),
    product_type_category_id: toInt(family.product_type_category_id),
    product_type_name: orderFamilyText(family.product_type_name),
    spec_source: 'price_list_sku',
    spec_mode: String(toInt(spec.sku_id)),
    spec_label: orderSpecLabel(spec),
    spec_g: orderSpecWeightG(spec),
    custom_spec_g: '',
    sales_unit: salesUnit,
    unit_bag_count: unitBagCount,
    unit_bean_g: unitBeanG || '',
    unit: orderSpecQuantityUnit(spec, tier),
    bean_list_publication_id: toInt(publicationID || tier.publication_id || source.publication_id || source.bean_list_publication_id),
    bean_list_version_no: orderFamilyTierVersion(tier),
    historical_spec_readonly: false,
    spec_invalid_message: '',
  }
}

export function orderFamilyHydratedSpecRowPatch(
  family = {},
  spec = {},
  publicationID = 0,
  frozen = {},
  keepFrozenPublication = false,
) {
  return {
    ...orderFamilySpecRowPatch(family, spec, publicationID),
    ...frozen,
    historical_spec_readonly: Boolean(keepFrozenPublication),
  }
}

export function wholesaleSpecOptions(product) {
  const specs = new Set(COMMON_SPEC_GRAMS)
  for (const tier of product?.tiers || []) {
    const spec = toInt(tier.spec_g)
    if (spec > 0) specs.add(spec)
  }
  const orderUnitOption = productOrderUnitSpecOption(product)
  return [
    ...(orderUnitOption ? [orderUnitOption] : []),
    ...[...specs].sort((a, b) => a - b).map((spec) => ({ label: formatSpecLabel(spec), value: String(spec) })),
    { label: '自定义克数', value: CUSTOM_SPEC_VALUE },
  ]
}

export function defaultWholesaleSpec(product) {
  const orderUnitSpec = productOrderUnitSpecG(product)
  if (orderUnitSpec > 0) return String(orderUnitSpec)
  const tier = (product?.tiers || []).find((item) => toInt(item.spec_g) > 0)
  if (tier) return String(toInt(tier.spec_g))
  return wholesaleSpecOptions(product)[0]?.value || ''
}

export function formatTierRange(tier) {
  const min = toNumber(tier?.min)
  const max = tier?.max == null ? 0 : toNumber(tier.max)
  const unit = tierQuantityUnitLabel(tier)
  if (min > 0 && max > 0) return `${trimNumber(min)}-${trimNumber(max)}${unit}`
  if (min > 0) return `${trimNumber(min)}${unit}+`
  if (max > 0) return `≤${trimNumber(max)}${unit}`
  return '全部数量'
}

function tierSpecG(tier) {
  return Math.max(1, toInt(tier?.spec_g) || 454)
}

function tierMinLb(tier) {
  return toNumber(tier?.min) * tierSpecG(tier) / 454
}

function tierMaxLb(tier) {
  if (tier?.max == null) return null
  return toNumber(tier.max) * tierSpecG(tier) / 454
}

function rowQuantityLb(row) {
  return normalizeSpecG(row) * Math.max(1, toInt(row?.qty)) / 454
}

function rowQuantityKg(row) {
  return normalizeSpecG(row) * Math.max(1, toInt(row?.qty)) / 1000
}

function tierUsesKgQuantity(tier) {
  return tierSpecG(tier) >= 1000
}

function tierQuantityUnitLabel(tier) {
	const explicit = String(tier?.tier_quantity_unit || tierPriceSource(tier)?.tier_quantity_unit || '').trim()
	if (explicit) return explicit
  return tierUsesKgQuantity(tier) ? 'kg' : '件'
}

function tierQuantityBasis(tier) {
	return String(tier?.quantity_basis || tierPriceSource(tier)?.quantity_basis || '').trim()
}

function orderRowQuantityBasis(product, row) {
  const direct = tierQuantityBasis(row)
  if (direct) return direct
  const tierID = String(row?.tier_id || '').trim()
  if (!tierID || tierID === 'auto' || tierID === 'manual') return ''
  const tier = (Array.isArray(product?.tiers) ? product.tiers : [])
    .find((item) => String(item?.id || '') === tierID)
  return tierQuantityBasis(tier)
}

export function wholesalePriceUnit(rowOrSpec) {
  const specG = typeof rowOrSpec === 'object' ? normalizeSpecG(rowOrSpec) : toInt(rowOrSpec)
  if (specG >= 1000) return { label: '元/kg', suffix: '/kg', unitG: 1000 }
  return { label: '元/磅', suffix: '/磅', unitG: 454 }
}

function rowQuantityForWholesalePriceUnit(row) {
  const unit = orderRowPriceUnit(row)
  return normalizeSpecG(row) * Math.max(1, toInt(row?.qty)) / unit.unitG
}

function normalizeTierDisplayUnit(unit) {
  const raw = String(unit || '').trim()
  const value = raw.toLowerCase()
  if (['kg', 'lb', 'g100', 'g227', 'g250'].includes(value)) return value
  return raw
}

function priceUnitForDisplayUnit(unit, specG = 0) {
  const displayUnit = normalizeTierDisplayUnit(unit)
  switch (displayUnit) {
    case 'kg':
      return { label: '元/kg', suffix: '/kg', unitG: 1000 }
    case 'lb':
      return { label: '元/磅', suffix: '/磅', unitG: 454 }
    case 'g100':
      return { label: '元/100g', suffix: '/100g', unitG: 100 }
    case 'g227':
      return { label: '元/227g', suffix: '/227g', unitG: 227 }
    case 'g250':
      return { label: '元/250g', suffix: '/250g', unitG: 250 }
    default:
      if (!displayUnit) return null
      const unitG = Math.max(1, toInt(specG))
      return { label: `元/${displayUnit}`, suffix: `/${displayUnit}`, unitG }
  }
}

function priceUnitForStoredFields(label, suffix, unitG) {
  const rawLabel = String(label || '').trim()
  const rawSuffix = String(suffix || '').trim()
  const normalizedUnitG = toNumber(unitG)
  if (normalizedUnitG === 1000 || rawLabel === '元/kg' || rawSuffix === '/kg') return { label: '元/kg', suffix: '/kg', unitG: 1000 }
  if (normalizedUnitG === 454 || rawLabel === '元/磅' || rawSuffix === '/磅') return { label: '元/磅', suffix: '/磅', unitG: 454 }
  if (normalizedUnitG === 100 || rawLabel === '元/100g' || rawSuffix === '/100g') return { label: '元/100g', suffix: '/100g', unitG: 100 }
  if (normalizedUnitG === 227 || rawLabel === '元/227g' || rawSuffix === '/227g') return { label: '元/227g', suffix: '/227g', unitG: 227 }
  if (normalizedUnitG === 250 || rawLabel === '元/250g' || rawSuffix === '/250g') return { label: '元/250g', suffix: '/250g', unitG: 250 }
  const customUnit = rawLabel.startsWith('元/') ? rawLabel.slice(2) : rawSuffix.startsWith('/') ? rawSuffix.slice(1) : ''
  if (customUnit) {
    return { label: `元/${customUnit}`, suffix: `/${customUnit}`, unitG: Math.max(1, normalizedUnitG || 1) }
  }
  return null
}

export function orderRowPriceUnit(row) {
  return priceUnitForStoredFields(
    String(row?.price_unit || '').trim(),
    String(row?.price_unit_suffix || '').trim(),
    row?.price_unit_g,
  ) || wholesalePriceUnit(row)
}

function tierConfiguredUnitPrice(tier) {
  const configuredPrice = toNumber(tier?.unit_price)
  return configuredPrice > 0 ? configuredPrice : toNumber(tier?.price)
}

function roundToCents(value) {
  return Math.round((Number(value || 0) + Number.EPSILON) * 100) / 100
}

export function orderTotalPreview({ itemsTotal = 0, shippingAmount = 0, discountAmount = 0, roundToInt = false } = {}) {
  const goodsAmount = Math.max(0, roundToCents(toNumber(itemsTotal) - toNumber(discountAmount)))
  const logisticsAmount = Math.max(0, roundToCents(toNumber(shippingAmount)))
  const rawTotal = goodsAmount + logisticsAmount
  return {
    goodsAmount,
    logisticsAmount,
    grandTotal: roundToInt ? Math.round(rawTotal) : roundToCents(rawTotal),
  }
}

function wholesaleTierUnitPriceLb(tier) {
  const pricePerPackage = tierConfiguredUnitPrice(tier)
  if (pricePerPackage <= 0) return 0
  return pricePerPackage * 454 / tierSpecG(tier)
}

function wholesaleDisplayUnitPrice(pricePerLb, rowOrSpec) {
  const unit = wholesalePriceUnit(rowOrSpec)
  const price = toNumber(pricePerLb) * unit.unitG / 454
  if (unit.unitG === 1000) return Math.round(price)
  return price
}

function wholesaleTierDisplayUnitPrice(tier, targetUnit) {
  const sourceUnit = priceUnitForDisplayUnit(tier?.display_unit, tier?.spec_g)
  if (!sourceUnit) {
    const price = wholesaleTierUnitPriceLb(tier) * targetUnit.unitG / 454
    if (targetUnit.unitG === 1000) return Math.round(price)
    return roundToCents(price)
  }
  const price = tierConfiguredUnitPrice(tier) * targetUnit.unitG / sourceUnit.unitG
  if (sourceUnit.unitG === 454 && targetUnit.unitG === 1000) return Math.round(price)
  return roundToCents(price)
}

function matchTierByQuantityResult(tiers, quantity, minValue, maxValue) {
  const sorted = [...tiers].sort((a, b) => minValue(b) - minValue(a))
  const exact = sorted.find((item) => minValue(item) <= quantity && (maxValue(item) == null || maxValue(item) >= quantity))
  if (exact) return { tier: exact, belowMin: false }
  return { tier: null, belowMin: false }
}

function matchTierByQuantity(tiers, quantity, minValue, maxValue) {
  return matchTierByQuantityResult(tiers, quantity, minValue, maxValue).tier
}

function formatTierUnitPrice(value) {
  const n = Number(value || 0)
  if (!Number.isFinite(n)) return '0'
  if (Number.isInteger(n)) return String(n)
  return String(roundToCents(n)).replace(/\.?0+$/, '')
}

export function wholesaleTierPriceRows(product, row = null) {
  return tiersForSelectedPublication(product, row)
    .filter((tier) => tierQuantityBasis(tier) === 'sales_spec_count' || toInt(tier.spec_g) > 0)
    .map((tier) => {
      const priceUnit = priceUnitForDisplayUnit(tier?.display_unit, tier?.spec_g) || wholesalePriceUnit(toInt(tier.spec_g))
      return {
        id: String(tier.id || ''),
        specG: toInt(tier.spec_g),
        specLabel: formatSpecLabel(tier.spec_g),
        rangeLabel: formatTierRange(tier),
        unitPrice: wholesaleTierDisplayUnitPrice(tier, priceUnit),
        priceUnit,
      }
    })
}

function tierPublicationID(tier) {
  const source = tierPriceSource(tier)
  return toInt(
    tier?.publication_id
      || tier?.publicationID
      || tier?.bean_list_publication_id
      || tier?.beanListPublicationID
      || source?.publication_id
      || source?.bean_list_publication_id,
  )
}

export function isConcreteOrderPublicationTier(tier = {}) {
  const source = tierPriceSource(tier) || {}
  const effectiveSalesSpec = {
    ...orderFamilyObject(source.effective_sales_spec ?? source.effectiveSalesSpec),
    ...orderFamilyObject(tier.effective_sales_spec ?? tier.effectiveSalesSpec),
  }
  return tierQuantityBasis(tier) === 'sales_spec_count'
    && tierPublicationID(tier) > 0
    && orderFamilyID(effectiveSalesSpec.sku_id, effectiveSalesSpec.skuID, effectiveSalesSpec.skuId) > 0
}

export function orderProductPublicationMode(product = {}, publicationID = 0) {
  const selectedPublicationID = toInt(publicationID)
  if (selectedPublicationID <= 0) return ''
  const tiers = (Array.isArray(product?.tiers) ? product.tiers : [])
    .filter((tier) => tierPublicationID(tier) === selectedPublicationID)
  if (!tiers.length) return ''
  return tiers.some(isConcreteOrderPublicationTier) ? 'concrete' : 'legacy'
}

export function orderLegacyProductForPublication(product = {}, publicationID = 0) {
  const selectedPublicationID = toInt(publicationID)
  if (selectedPublicationID <= 0) return null
  const tiers = (Array.isArray(product?.tiers) ? product.tiers : [])
    .filter((tier) => tierPublicationID(tier) === selectedPublicationID)
  if (orderProductPublicationMode(product, selectedPublicationID) !== 'legacy') return null
  return {
    ...product,
    tiers,
    __order_legacy_price_product: true,
  }
}

function tiersForSelectedPublication(product, row = null) {
  const tiers = product?.tiers || []
  const publicationID = toInt(row?.bean_list_publication_id)
  if (publicationID <= 0) return tiers
  const selected = tiers.filter((tier) => tierPublicationID(tier) === publicationID)
  const hasPublishedTiers = tiers.some((tier) => tierPublicationID(tier) > 0)
  return selected.length || hasPublishedTiers ? selected : tiers
}

function findWholesaleTierMatch(product, row) {
  const specG = normalizeSpecG(row)
  const qty = Math.max(1, toInt(row?.qty))
	const selectedTiers = tiersForSelectedPublication(product, row)
	const countTiers = selectedTiers.filter((item) => tierQuantityBasis(item) === 'sales_spec_count')
	if (countTiers.length) {
		return matchTierByQuantityResult(countTiers, qty, (item) => toNumber(item.min), (item) => (item.max == null ? null : toNumber(item.max)))
	}
  const tiers = selectedTiers.filter((item) => toInt(item.spec_g) > 0)
  const exactSpecTiers = tiers
    .filter((item) => toInt(item.spec_g) === specG)
  if (exactSpecTiers.length) {
    const exactQuantity = tierUsesKgQuantity(exactSpecTiers[0]) ? rowQuantityKg(row) : qty
    return matchTierByQuantityResult(exactSpecTiers, exactQuantity, (item) => toNumber(item.min), (item) => (item.max == null ? null : toNumber(item.max)))
  }
  return matchTierByQuantityResult(tiers, rowQuantityLb(row), tierMinLb, tierMaxLb)
}

export function findWholesaleTier(product, row) {
  return findWholesaleTierMatch(product, row).tier
}

export function resolveWholesaleTierPrice(product, row) {
  const matched = findWholesaleTierMatch(product, row)
  const tier = matched.tier
  if (!tier) {
    const selectedTiers = tiersForSelectedPublication(product, row)
    const countTier = selectedTiers.find((item) => tierQuantityBasis(item) === 'sales_spec_count')
    const quantityBasis = countTier
      ? 'sales_spec_count'
      : ''
    return {
      tierID: 'auto',
      unitPrice: '',
      priceUnit: orderRowPriceUnit(row),
      tierPriceLabel: '',
      beanListPublicationID: 0,
      beanListVersionNo: '',
      quantityBasis,
      priceSourceJSON: String(countTier?.price_source_json || ''),
      belowMinTier: false,
      priceMissing: true,
    }
  }
  const priceUnit = priceUnitForDisplayUnit(tier?.display_unit, tier?.spec_g) || orderRowPriceUnit(row)
  const unitPrice = wholesaleTierDisplayUnitPrice(tier, priceUnit) || 0
  const source = tierPriceSource(tier) || {}
  return {
    tierID: String(tier.id),
    unitPrice: String(unitPrice),
    priceUnit,
    tierPriceLabel: `${formatTierUnitPrice(unitPrice)}${priceUnit.suffix}`,
    beanListPublicationID: tierPublicationID(tier),
    beanListVersionNo: String(tier?.version_no || tier?.versionNo || source.version_no || source.bean_list_version_no || source.version || '').trim(),
    quantityBasis: tierQuantityBasis(tier),
    priceSourceJSON: String(tier?.price_source_json || ''),
    belowMinTier: matched.belowMin,
    priceMissing: false,
  }
}

export function syncWholesaleTierPrice(product, row) {
  const price = resolveWholesaleTierPrice(product, row)
  return { tierID: price.tierID, unitPrice: price.unitPrice }
}

export function isOrderTierActive(row, tier) {
  const rowTierID = String(row?.tier_id || '')
  const tierID = String(tier?.id || '')
  if (!rowTierID || !tierID || rowTierID === 'auto' || rowTierID === 'manual') return false
  return rowTierID === tierID
}

function normalizeDripSalesUnit(unit) {
  return String(unit || '').trim() === 'box' ? 'box' : 'bag'
}

export function defaultDripSalesUnit(product) {
  const configuredUnits = [product?.order_unit, product?.quote_unit]
    .map((unit) => String(unit || '').trim().toLowerCase())
    .filter(Boolean)
  if (configuredUnits.some((unit) => unit === 'box' || unit.includes('盒'))) return 'box'
  if (configuredUnits.some((unit) => unit === 'bag' || unit.includes('袋'))) return 'bag'
  const tierUnits = [...new Set((product?.tiers || [])
    .filter((tier) => String(tier?.product_kind || product?.product_kind || '').trim() === 'drip_bag')
    .map((tier) => String(tier?.sales_unit || '').trim())
    .filter((unit) => unit === 'bag' || unit === 'box'))]
  if (tierUnits.length === 1) return tierUnits[0]
  return 'bag'
}

export function defaultDripSalesUnitSpec(product) {
  return dripSalesUnitSpec(product, { sales_unit: defaultDripSalesUnit(product) })
}

export function dripSalesUnitSpec(product, row = {}) {
  const salesUnit = normalizeDripSalesUnit(row?.sales_unit || defaultDripSalesUnit(product))
  const unitBeanG = toNumber(row?.unit_bean_g) || toNumber(product?.drip_bag_grams) || 10
  const productBoxBagCount = toInt(product?.drip_box_bag_count) || 10
  const unitBagCount = salesUnit === 'box' ? (toInt(row?.unit_bag_count) || productBoxBagCount) : 1
  return {
    salesUnit,
    unitBeanG,
    unitBagCount,
    unitLabel: salesUnit === 'box' ? '盒' : '袋',
    specG: salesUnit === 'box' ? unitBeanG * unitBagCount : unitBeanG,
    specLabel: salesUnit === 'box' ? `${unitBagCount}袋/盒` : `${trimNumber(unitBeanG)}g/袋`,
  }
}

function dripTierSalesUnit(tier) {
  return normalizeDripSalesUnit(tier?.sales_unit)
}

function dripTierMin(tier) {
  return toNumber(tier?.min ?? tier?.min_qty ?? tier?.min_qty_units)
}

function dripTierMax(tier) {
  const max = tier?.max ?? tier?.max_qty ?? tier?.max_qty_units
  if (max == null || max === '') return null
  return toNumber(max)
}

function dripTierPrice(tier) {
  return toNumber(tier?.unit_price ?? tier?.price_per_unit ?? tier?.price)
}

function isDripTier(tier) {
  return String(tier?.product_kind || 'drip_bag') === 'drip_bag' && ['bag', 'box'].includes(dripTierSalesUnit(tier))
}

function dripTiersForUnit(product, salesUnit) {
  const tiers = (product?.tiers || []).filter(isDripTier)
  if (salesUnit === 'box') return tiers.filter((tier) => dripTierSalesUnit(tier) === 'bag' || dripTierSalesUnit(tier) === 'box')
  return tiers.filter((tier) => dripTierSalesUnit(tier) === 'bag')
}

function dripTierRangeLabel(tier) {
  const min = dripTierMin(tier)
  const max = dripTierMax(tier)
  const unit = dripTierSalesUnit(tier) === 'box' ? '盒' : '袋'
  if (min > 0 && max > 0) return `${trimNumber(min)}-${trimNumber(max)}${unit}`
  if (min > 0) return `${trimNumber(min)}${unit}+`
  if (max > 0) return `≤${trimNumber(max)}${unit}`
  return '全部数量'
}

function matchDripTier(tiers, salesUnit, quantity) {
  const unitTiers = tiers.filter((tier) => dripTierSalesUnit(tier) === salesUnit)
  return matchTierByQuantity(unitTiers, quantity, dripTierMin, dripTierMax)
}

export function findDripTier(product, row) {
  const spec = dripSalesUnitSpec(product, row)
  const qty = Math.max(1, toInt(row?.qty))
  const tiers = dripTiersForUnit(product, spec.salesUnit)
  if (spec.salesUnit === 'box') {
    const boxTiers = tiers.filter((tier) => dripTierSalesUnit(tier) === 'box')
    if (boxTiers.length) {
      const boxTier = matchDripTier(boxTiers, 'box', qty)
      if (!boxTier) return null
      return {
        tier: boxTier,
        matchedQty: qty,
        unitPrice: dripTierPrice(boxTier),
      }
    }
    const bagQty = qty * spec.unitBagCount
    const bagTier = matchDripTier(tiers, 'bag', bagQty)
    if (bagTier) {
      return {
        tier: bagTier,
        matchedQty: bagQty,
        unitPrice: dripTierPrice(bagTier) * spec.unitBagCount,
      }
    }
  }
  const tier = matchDripTier(tiers, spec.salesUnit, qty)
  if (!tier) return null
  return {
    tier,
    matchedQty: qty,
    unitPrice: dripTierPrice(tier),
  }
}

export function syncDripTierPrice(product, row) {
  const matched = findDripTier(product, row)
  if (!matched) return { tierID: 'auto', unitPrice: '' }
  return { tierID: String(matched.tier.id || ''), unitPrice: String(matched.unitPrice || 0) }
}

export function dripTierPriceRows(product, row = {}) {
  const spec = dripSalesUnitSpec(product, row)
  return dripTiersForUnit(product, spec.salesUnit)
    .map((tier) => {
      const tierUnit = dripTierSalesUnit(tier)
      const unitPrice = tierUnit === 'bag' && spec.salesUnit === 'box'
        ? dripTierPrice(tier) * spec.unitBagCount
        : dripTierPrice(tier)
      const tierBagCount = toInt(tier?.unit_bag_count) || toInt(product?.drip_box_bag_count) || 10
      return {
        id: String(tier.id || ''),
        salesUnit: tierUnit,
        specLabel: tierUnit === 'box' ? `${tierBagCount}袋/盒` : `${trimNumber(spec.unitBeanG)}g/袋`,
        rangeLabel: dripTierRangeLabel(tier),
        unitPrice,
        priceUnit: {
          label: spec.salesUnit === 'box' ? '元/盒' : '元/袋',
          suffix: spec.salesUnit === 'box' ? '/盒' : '/袋',
        },
      }
    })
}

export function filterOptions(options, query) {
  const q = String(query || '').trim().toLowerCase()
  if (!q) return options || []
  return (options || []).filter((item) => {
    const haystack = `${item.name || ''} ${item.py || ''} ${item.pyi || ''} ${item.code || ''}`.toLowerCase()
    return haystack.includes(q)
  })
}

export function sortProductsByCustomerUsage(products, customerID, usages = []) {
  const selectedCustomerID = toInt(customerID)
  if (selectedCustomerID <= 0) return products || []
  const usageByProduct = new Map()
  for (const row of usages || []) {
    if (toInt(row?.customer_id) !== selectedCustomerID) continue
    const productID = toInt(row?.product_id)
    if (productID <= 0) continue
    usageByProduct.set(productID, {
      orderCount: toInt(row.order_count),
      itemCount: toInt(row.item_count),
      lastOrderDate: String(row.last_order_date || ''),
    })
  }
  if (!usageByProduct.size) return products || []
  return (products || [])
    .map((product, index) => {
      const productIDs = new Set([
        toInt(product?.id),
        toInt(product?.parent_product_id),
        ...(product?.specs || []).map((spec) => toInt(spec?.sku_id)),
      ].filter((id) => id > 0))
      const matching = [...productIDs].map((id) => usageByProduct.get(id)).filter(Boolean)
      const usage = matching.length
        ? matching.reduce((acc, item) => ({
          orderCount: acc.orderCount + item.orderCount,
          itemCount: acc.itemCount + item.itemCount,
          lastOrderDate: acc.lastOrderDate > item.lastOrderDate ? acc.lastOrderDate : item.lastOrderDate,
        }), { orderCount: 0, itemCount: 0, lastOrderDate: '' })
        : null
      return { product, index, usage }
    })
    .sort((a, b) => {
      if (a.usage && !b.usage) return -1
      if (!a.usage && b.usage) return 1
      if (a.usage && b.usage) {
        if (a.usage.orderCount !== b.usage.orderCount) return b.usage.orderCount - a.usage.orderCount
        if (a.usage.itemCount !== b.usage.itemCount) return b.usage.itemCount - a.usage.itemCount
        if (a.usage.lastOrderDate !== b.usage.lastOrderDate) return b.usage.lastOrderDate.localeCompare(a.usage.lastOrderDate)
      }
      return a.index - b.index
    })
    .map((row) => row.product)
}

function normalizeBeanListType(value) {
  const type = String(value || '').trim()
  if (!type) return 'commercial'
  if (type === 'commercial' || type === 'retail') return type
  if (type === 'green') return 'green'
  if (type === 'drip') return 'drip'
  return type
}

export function productBeanListType(product) {
  if (String(product?.product_kind || '').trim() === 'green_bean') return 'green'
  return 'commercial'
}

function compareVersionNumbers(a, b) {
  const left = String(a || '').match(/\d+/g)?.map(toInt) || []
  const right = String(b || '').match(/\d+/g)?.map(toInt) || []
  const length = Math.max(left.length, right.length)
  for (let i = 0; i < length; i += 1) {
    const diff = (left[i] || 0) - (right[i] || 0)
    if (diff !== 0) return diff
  }
  return 0
}

function compareBeanListVersionOption(a, b) {
  const leftPublished = String(a?.published_at || a?.created_at || '').trim()
  const rightPublished = String(b?.published_at || b?.created_at || '').trim()
  if (leftPublished && rightPublished && leftPublished !== rightPublished) {
    return leftPublished.localeCompare(rightPublished)
  }
  const versionDiff = compareVersionNumbers(a?.version_no || a?.label, b?.version_no || b?.label)
  if (versionDiff !== 0) return versionDiff
  return toInt(a?.id) - toInt(b?.id)
}

export function latestBeanListVersionOption(options, listType) {
  const normalized = normalizeBeanListType(listType)
  const rows = (options || []).filter((item) => normalizeBeanListType(item?.list_type) === normalized)
  if (!rows.length) return null
  return rows.reduce((latest, item) => (
    compareBeanListVersionOption(item, latest) > 0 ? item : latest
  ), rows[0])
}

export function latestProductPriceListVersionOption(options, productOrType, listType = '') {
  const productTypeCategoryID = toInt(
    productOrType?.classification_template_id
      || productOrType?.classificationTemplateID
      || productOrType?.product_type_category_id
      || productOrType?.productTypeCategoryID,
  )
  const productTypeName = String(productOrType?.product_type_name || productOrType?.productTypeName || '').trim()
  const requestedListType = String(
    listType
      || productOrType?.list_type
      || productOrType?.listType
      || '',
  ).trim()
  let rows = []
  if (productTypeCategoryID > 0) {
    rows = (options || []).filter((item) => toInt(
      item?.classification_template_id
        || item?.classificationTemplateID
        || item?.product_type_category_id
        || item?.productTypeCategoryID,
    ) === productTypeCategoryID)
  } else if (productTypeName) {
    rows = (options || []).filter((item) => String(item?.product_type_name || '').trim() === productTypeName)
  }
  if (rows.length && requestedListType) {
    const normalizedListType = normalizeBeanListType(requestedListType)
    rows = rows.filter((item) => normalizeBeanListType(item?.list_type) === normalizedListType)
  }
  if (!rows.length) {
    return latestBeanListVersionOption(options, requestedListType || productBeanListType(productOrType))
  }
  return rows.reduce((latest, item) => (
    compareBeanListVersionOption(item, latest) > 0 ? item : latest
  ), rows[0])
}

export function beanListVersionGroupIdentity(item = {}) {
  const listType = normalizeBeanListType(item?.list_type)
  const classificationTemplateID = toInt(
    item?.classification_template_id
      || item?.classificationTemplateID
      || item?.product_type_category_id
      || item?.productTypeCategoryID,
  )
  const classificationTemplateName = String(
    item?.classification_template_name
      || item?.classificationTemplateName
      || item?.product_type_name
      || item?.productTypeName
      || '',
  ).trim()
  return {
    key: classificationTemplateID > 0
      ? `classification:${classificationTemplateID}:${listType}`
      : `legacy:${listType}`,
    label: classificationTemplateName || beanListTypeLabel(listType),
    listType,
    classificationTemplateID,
    classified: classificationTemplateID > 0,
  }
}

export function beanListVersionOptionGroups(options) {
  const groups = new Map()
  for (const item of options || []) {
    const identity = beanListVersionGroupIdentity(item)
    if (!groups.has(identity.key)) {
      groups.set(identity.key, {
        ...identity,
        options: [],
      })
    }
    groups.get(identity.key).options.push(item)
  }
  const rows = [...groups.values()]
  const classifiedTypes = new Set(rows.filter((group) => group.classified).map((group) => group.listType))
  return rows.map((group) => ({
    ...group,
    autoSelect: group.classified || !classifiedTypes.has(group.listType),
    options: group.options,
  }))
}

export function filterBeanListVersionOptionsToCurrentTypes(options = [], currentTypes = []) {
  const types = Array.isArray(currentTypes) ? currentTypes : []
  if (!types.length) return options
  if (types.some((type) => Number(type?.id || 0) === 0)) return options
  const currentIDs = new Set(types.map((type) => Number(type?.id || 0)).filter((id) => id > 0))
  return (Array.isArray(options) ? options : []).filter((item) => {
    const tplID = Number(item?.classification_template_id ?? item?.classificationTemplateID ?? 0)
    return tplID > 0 && currentIDs.has(tplID)
  })
}

export function beanListVersionOptionForGroup(group = {}, selectedID = 0) {
  const rows = group?.options || []
  if (!rows.length) return null
  const selected = toInt(selectedID)
  if (selected > 0) {
    const explicit = rows.find((item) => toInt(item?.id) === selected)
    if (explicit) return explicit
  }
  if (!group?.autoSelect) return null
  return rows.find((item) => item?.is_default)
    || rows.reduce((latest, item) => (
      compareBeanListVersionOption(item, latest) > 0 ? item : latest
    ), rows[0])
}

export function activeBeanListPublicationIDsByType(groups = [], selections = {}) {
  const out = {}
  for (const group of groups || []) {
    const selected = beanListVersionOptionForGroup(group, selections?.[group.key])
    const id = toInt(selected?.id)
    if (id <= 0) continue
    const listType = normalizeBeanListType(selected?.list_type || group?.listType)
    if (!out[listType]) out[listType] = []
    if (!out[listType].includes(id)) out[listType].push(id)
  }
  return out
}

export function activeCustomerOwnedBeanListPublicationIDsByType(groups = [], selections = {}, customerID = 0) {
  const selectedCustomerID = toInt(customerID)
  const out = {}
  for (const group of groups || []) {
    const selected = beanListVersionOptionForGroup(group, selections?.[group.key])
    const id = toInt(selected?.id)
    if (id <= 0 || !selected?.is_customer_owned) continue
    const ownerID = toInt(selected?.customer_id)
    if (selectedCustomerID > 0 && ownerID > 0 && ownerID !== selectedCustomerID) continue
    const listType = normalizeBeanListType(selected?.list_type || group?.listType)
    if (!out[listType]) out[listType] = []
    if (!out[listType].includes(id)) out[listType].push(id)
  }
  return out
}

export function beanListVersionGroupForPublicationID(groups = [], publicationID = 0) {
  const id = toInt(publicationID)
  if (id <= 0) return null
  return (groups || []).find((group) => (
    (group?.options || []).some((item) => toInt(item?.id) === id)
  )) || null
}

export function shouldKeepFrozenOrderPublication(groups = [], publicationID = 0, copyMode = false) {
  const id = toInt(publicationID)
  return !copyMode && id > 0 && !beanListVersionGroupForPublicationID(groups, id)
}

export function beanListVersionOptionForProductGroups(
  groups = [],
  selections = {},
  product = {},
  preferredPublicationID = 0,
) {
  const preferredGroup = beanListVersionGroupForPublicationID(groups, preferredPublicationID)
  if (preferredGroup) {
    const selected = beanListVersionOptionForGroup(preferredGroup, selections?.[preferredGroup.key])
    if (selected) return selected
  }

  const publicationIDs = new Set()
  const productPublicationID = toInt(product?.bean_list_publication_id)
  if (productPublicationID > 0) publicationIDs.add(productPublicationID)
  for (const tier of product?.tiers || []) {
    const publicationID = toInt(tier?.publication_id)
    if (publicationID > 0) publicationIDs.add(publicationID)
  }
  for (const group of groups || []) {
    const selected = beanListVersionOptionForGroup(group, selections?.[group.key])
    if (selected && publicationIDs.has(toInt(selected?.id))) return selected
  }

  const identity = beanListVersionGroupIdentity(product)
  const identityGroup = (groups || []).find((group) => group?.key === identity.key)
  return identityGroup
    ? beanListVersionOptionForGroup(identityGroup, selections?.[identityGroup.key])
    : null
}

function beanListTypeLabel(listType) {
  switch (normalizeBeanListType(listType)) {
    case 'green':
      return '生豆豆单'
    case 'drip':
      return '挂耳豆单'
    case 'retail':
      return '零售价格表'
    default:
      return '熟豆豆单'
  }
}

export function rowUsesStaleBeanListPublication(row, options, listType = productBeanListType(row)) {
  if (toInt(row?.product_id) <= 0) return false
  const publicationID = toInt(row?.bean_list_publication_id)
  if (publicationID <= 0) return false
  const groups = beanListVersionOptionGroups(options)
  const publicationGroup = beanListVersionGroupForPublicationID(groups, publicationID)
  const latest = publicationGroup?.options?.length
    ? publicationGroup.options.reduce((current, item) => (
      compareBeanListVersionOption(item, current) > 0 ? item : current
    ), publicationGroup.options[0])
    : (latestProductPriceListVersionOption(options, row, listType) || latestBeanListVersionOption(options, listType))
  const latestID = toInt(latest?.id)
  return latestID > 0 && latestID !== publicationID
}

export function beanListVersionOptionsForCustomer(options, customerID) {
  const selectedCustomerID = toInt(customerID)
  const rows = (options || []).filter((item) => toInt(item?.customer_id) === selectedCustomerID)
  if (rows.length) return rows
  const seen = new Set()
  return (options || []).filter((item) => {
    if (item?.is_customer_owned) return false
    const id = toInt(item?.id)
    if (id <= 0) return false
    const key = `${normalizeBeanListType(item?.list_type)}:${id}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

export function isBlankOrderLine(row) {
  return toInt(row?.product_id) <= 0
    && !String(row?.product_query || '').trim()
    && !String(row?.item_note || '').trim()
    && !String(row?.unit_price || '').trim()
}

export function needsTrailingBlankOrderLine(rows) {
  return !(rows || []).some(isBlankOrderLine)
}

function normalizePublicationIDsByType(publicationIDsByType = {}) {
  const out = {}
  for (const [type, value] of Object.entries(publicationIDsByType || {})) {
    const ids = Array.isArray(value) ? value : [value]
    const normalizedIDs = ids.map(toInt).filter((id) => id > 0)
    if (!normalizedIDs.length) continue
    out[normalizeBeanListType(type)] = new Set(normalizedIDs)
  }
  return out
}

function tierPriceSource(tier) {
  const raw = tier?.price_source_json
  if (!raw) return null
  if (typeof raw === 'object') return raw
  try {
    return JSON.parse(String(raw))
  } catch {
    return null
  }
}

function productMatchesPublicationScope(product, publicationIDsByType) {
  const hasPublicationScope = Object.values(publicationIDsByType || {}).some((ids) => ids?.size)
  if (!hasPublicationScope) return true
  return productMatchesExplicitPublicationScope(product, publicationIDsByType)
}

function productMatchesExplicitPublicationScope(product, publicationIDsByType) {
  const activePublicationIDs = new Set(
    Object.values(publicationIDsByType || {}).flatMap((ids) => [...(ids || [])]),
  )
  if (!activePublicationIDs.size) return false
  return (product?.tiers || []).some((tier) => {
    const publicationID = tierPublicationID(tier)
    return publicationID > 0 && activePublicationIDs.has(publicationID)
  })
}

function customerAllowsPublicSKU(customerID, publicUsages = []) {
  const selectedCustomerID = toInt(customerID)
  if (selectedCustomerID <= 0) return true
  const usage = (publicUsages || []).find((item) => toInt(item?.customer_id) === selectedCustomerID)
  return usage ? Boolean(usage.use_public_sku) : true
}

export function filterProductsForCustomer(
  products,
  customerID,
  publicationIDsByType = {},
  publicUsages = [],
  customerOwnedPublicationIDsByType = {},
) {
  const selectedCustomerID = toInt(customerID)
  const scopedPublicationIDs = normalizePublicationIDsByType(publicationIDsByType)
  const customerOwnedPublicationIDs = normalizePublicationIDsByType(customerOwnedPublicationIDsByType)
  const allowsPublicSKU = customerAllowsPublicSKU(selectedCustomerID, publicUsages)
  const aliasProductIDs = new Set(
    (products || [])
      .filter((product) => selectedCustomerID > 0
        && toInt(product?.customer_product_alias_id) > 0
        && toInt(product?.customer_id) === selectedCustomerID)
      .map((product) => toInt(product?.id))
      .filter((id) => id > 0),
  )
  return (products || []).filter((product) => {
    const productCustomerID = toInt(product?.customer_id)
    const visibility = String(product?.visibility || (productCustomerID > 0 ? 'customer_only' : 'public')).trim()
    if (visibility === 'customer_alias' || toInt(product?.customer_product_alias_id) > 0) {
      return selectedCustomerID > 0
        && productCustomerID === selectedCustomerID
        && productMatchesPublicationScope(product, scopedPublicationIDs)
    }
    if (visibility === 'public' || productCustomerID === 0) {
      if (selectedCustomerID > 0 && aliasProductIDs.has(toInt(product?.id))) {
        return false
      }
      if (
        selectedCustomerID > 0
        && !allowsPublicSKU
        && !productMatchesExplicitPublicationScope(product, customerOwnedPublicationIDs)
      ) {
        return false
      }
      return productMatchesPublicationScope(product, scopedPublicationIDs)
    }
    const visible = visibility === 'public' || productCustomerID === 0
      ? true
      : selectedCustomerID > 0 && productCustomerID === selectedCustomerID
    return visible && productMatchesPublicationScope(product, scopedPublicationIDs)
  })
}

export function responsibleOptions({ employees = [] } = {}) {
  return (employees || [])
    .filter((item) => toInt(item?.id) > 0 && String(item?.name || '').trim())
    .map((item) => {
      const name = String(item.name || '').trim()
      const department = String(item.department || '').trim()
      const phone = String(item.phone || '').trim()
      const meta = [department, phone].filter(Boolean).join(' ')
      return {
        type: 'employee',
        id: toInt(item.id),
        name,
        label: `员工 - ${name}`,
        meta,
        search: ['员工', name, department, phone].filter(Boolean).join(' '),
      }
    })
}

export function defaultStatusID(options, names) {
  const wanted = (names || []).map((name) => String(name).trim()).filter(Boolean)
  for (const name of wanted) {
    const found = (options || []).find((item) => String(item.name || '').trim() === name)
    if (found) return toInt(found.id)
  }
  return 0
}

export function requiresOrderPaymentMethod(form, payStatuses) {
  const statusID = toInt(form?.pay_status_id)
  if (statusID <= 0) return false
  const status = (payStatuses || []).find((item) => toInt(item.id) === statusID)
  const name = String(status?.name || '').trim()
  return name.includes('已付款') || name.includes('已收款') || name.includes('已支付')
}

export function requiresOrderPaymentReceipt(form, payStatuses) {
  const statusID = toInt(form?.pay_status_id)
  if (statusID <= 0) return false
  const status = (payStatuses || []).find((item) => toInt(item.id) === statusID)
  const name = String(status?.name || '').trim()
  return name.includes('已收款')
}

export function normalizeSpecG(row) {
  if (row?.spec_mode === CUSTOM_SPEC_VALUE) {
    return Math.max(0, toInt(row.custom_spec_g))
  }
  return Math.max(0, toInt(row?.spec_g || row?.spec_mode))
}

function trimNumber(value) {
  return String(Number(value || 0)).replace(/\.0+$/, '')
}

export function lineTotal(product, row, retailOrder) {
  const base = lineTotalBeforeDiscount(product, row, retailOrder)
  return Math.max(base - lineDiscountAmount(base, row, retailOrder, product), 0)
}

export function lineTotalBeforeDiscount(product, row, retailOrder) {
  const units = Math.max(0, toInt(row?.qty))
  if (isDripProduct(product) || row?.product_kind === 'drip_bag') {
    if (units <= 0) return 0
    return toNumber(row?.unit_price) * units
  }
  if (orderRowQuantityBasis(product, row) === 'sales_spec_count') {
    return toNumber(row?.unit_price) * units
  }
  const specG = normalizeSpecG(row)
  if (units <= 0 || specG <= 0) return 0
  if (row?.tier_id === 'manual') {
    return retailOrder ? toNumber(row?.unit_price) * units : toNumber(row?.unit_price) * rowQuantityForWholesalePriceUnit(row)
  }
  if (retailOrder) return retailPackagePrice(product, specG) * units
  const price = toNumber(row?.unit_price)
  return price * rowQuantityForWholesalePriceUnit(row)
}

export function rowUnitDiscountUnits(row, retailOrder = false, product = null) {
  const units = Math.max(0, toInt(row?.qty))
  if (units <= 0) return 0
  if (row?.product_kind === 'drip_bag' || row?.sales_unit === 'bag' || row?.sales_unit === 'box') return units
  if (retailOrder) return units
  if (orderRowQuantityBasis(product, row) === 'sales_spec_count') return units
  const specG = normalizeSpecG(row)
  if (specG <= 0) return units
  return (specG * units) / orderRowPriceUnit(row).unitG
}

export function lineDiscountAmount(baseLineTotal, row, retailOrder = false, product = null) {
  const base = Math.max(0, toNumber(baseLineTotal))
  if (base <= 0) return 0
  const type = String(row?.discount_type || '').trim()
  const value = Math.max(0, toNumber(row?.discount_value))
  if (type === 'free') return base
  if (type === 'amount') return Math.min(value, base)
  if (type === 'unit_amount') return Math.min(value * rowUnitDiscountUnits(row, retailOrder, product), base)
  if (type === 'percent') {
    const rate = Math.max(0, Math.min(value, 100))
    return Math.max(base - (base * rate / 100), 0)
  }
  return 0
}

export function buildOrderPayload({ form, rows }) {
  const payload = {
    edit_id: Number(form.edit_id || 0),
    document_date: form.document_date || form.order_date || '',
    order_date: form.order_date || '',
    customer_id: Number(form.customer_id || 0),
    source_id: Number(form.source_id || 0),
    order_type_id: Number(form.order_type_id || 0),
    pay_status_id: Number(form.pay_status_id || 0),
    payment_method: String(form.payment_method || '').trim(),
    ship_status_id: Number(form.ship_status_id || 0),
    ship_method: form.ship_method || '',
    ship_tracking_no: form.ship_tracking_no || '',
    logistics_company_id: Number(form.logistics_company_id || 0),
    logistics_product_id: Number(form.logistics_product_id || 0),
    payment_goods_amount: String(form.payment_goods_amount || ''),
    payment_shipping_amount: String(form.payment_shipping_amount || ''),
    payment_voucher_asset_id: Number(form.payment_voucher_asset_id || 0),
    bean_list_publication_id: Number(form.bean_list_publication_id || 0),
    commercial_bean_list_publication_id: Number(form.commercial_bean_list_publication_id || 0),
    green_bean_list_publication_id: Number(form.green_bean_list_publication_id || 0),
    drip_bean_list_publication_id: Number(form.drip_bean_list_publication_id || 0),
    receiver_name: String(form.receiver_name || '').trim(),
    receiver_phone: String(form.receiver_phone || '').trim(),
    receiver_address: String(form.receiver_address || '').trim(),
    receiver_company: String(form.receiver_company || '').trim(),
    portal_service_code: 'direct_ship',
    orders_scope: 'fulfillment',
    notes: form.notes || '',
    shipping_amount: String(form.shipping_amount || ''),
    discount_amount: String(form.discount_amount || ''),
    round_to_int: form.round_to_int ? 'on' : '',
    express_fee: String(form.express_fee || ''),
    outsource_material_fee: String(form.outsource_material_fee || ''),
    outsource_roast_fee: String(form.outsource_roast_fee || ''),
    outsource_packaging_fee: String(form.outsource_packaging_fee || ''),
    outsource_manual_fee: String(form.outsource_manual_fee || ''),
    outsource_tax_fee: String(form.outsource_tax_fee || ''),
    outsource_other_fee: String(form.outsource_other_fee || ''),
    product_id: [],
    parent_product_id: [],
    customer_product_alias_id: [],
    customer_product_display_name_snapshot: [],
    customer_item_code_snapshot: [],
    brand_name_snapshot: [],
    product_code_snapshot: [],
    product_name_snapshot: [],
    item_bean_list_publication_id: [],
    item_bean_list_version_no: [],
    price_source_json: [],
    tier_id: [],
    unit_price: [],
    item_name: [],
    item_note: [],
    qty: [],
    unit: [],
    spec: [],
    product_kind: [],
    sales_unit: [],
    unit_bag_count: [],
    unit_bean_g: [],
    discount_type: [],
    discount_value: [],
  }

  for (const row of rows || []) {
    const productID = toInt(row.product_id)
    const normalizedKind = normalizedProductKind(row)
    const productKind = normalizedKind === 'drip_bag' ? 'drip_bag' : normalizedKind === 'green_bean' ? 'green_bean' : normalizedKind === 'instant_coffee' ? 'instant_coffee' : 'roasted_bean'
    const dripSpec = dripSalesUnitSpec(null, row)
    const specG = productKind === 'drip_bag' ? dripSpec.specG : normalizeSpecG(row)
    const qty = toInt(row.qty)
    if (productID <= 0 || specG <= 0 || qty <= 0) continue
    payload.product_id.push(String(productID))
    payload.parent_product_id.push(String(toInt(row.parent_product_id)))
    payload.customer_product_alias_id.push(String(toInt(row.customer_product_alias_id)))
    payload.customer_product_display_name_snapshot.push(String(row.customer_product_display_name || row.product_name || row.item_name || '').trim())
    payload.customer_item_code_snapshot.push(String(row.customer_item_code || '').trim())
    payload.brand_name_snapshot.push(String(row.brand_name || '').trim())
    payload.product_code_snapshot.push(String(row.product_code || `SKU-${productID}`).trim())
    payload.product_name_snapshot.push(String(row.product_record_name || row.product_name_snapshot || row.source_product_name || row.product_name || row.item_name || '').trim())
    payload.item_bean_list_publication_id.push(String(toInt(row.bean_list_publication_id)))
    payload.item_bean_list_version_no.push(String(row.bean_list_version_no || '').trim())
    payload.price_source_json.push(String(row.price_source_json || '').trim())
    payload.tier_id.push(row.tier_id || 'auto')
    payload.unit_price.push(String(row.unit_price || ''))
    payload.item_name.push(row.product_name || row.item_name || '')
    payload.item_note.push(String(row.item_note || '').trim())
    payload.qty.push(String(qty))
    payload.unit.push(productKind === 'drip_bag' ? dripSpec.unitLabel : (row.unit || '件'))
    payload.spec.push(String(trimNumber(specG)))
    payload.product_kind.push(productKind)
    payload.sales_unit.push(productKind === 'drip_bag' ? dripSpec.salesUnit : '')
    payload.unit_bag_count.push(productKind === 'drip_bag' ? String(dripSpec.unitBagCount) : '0')
    payload.unit_bean_g.push(productKind === 'drip_bag' ? String(trimNumber(dripSpec.unitBeanG)) : '0')
    payload.discount_type.push(String(row.discount_type || '').trim())
    payload.discount_value.push(['amount', 'unit_amount', 'percent'].includes(row.discount_type) ? String(row.discount_value || '') : '')
  }

  return payload
}
