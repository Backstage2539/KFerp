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

export function normalizedProductKind(productOrKind) {
  const raw = typeof productOrKind === 'object'
    ? productOrKind?.product_kind
    : productOrKind
  return String(raw || '').trim() === 'green_bean' ? 'green_bean' : 'roasted'
}

export function productKindLabel(productOrKind) {
  return normalizedProductKind(productOrKind) === 'green_bean' ? '生豆' : '熟豆'
}

export function productKindBadgeClass(productOrKind) {
  return normalizedProductKind(productOrKind) === 'green_bean' ? 'kind-green' : 'kind-roasted'
}

export function wholesaleSpecOptions(product) {
  const specs = new Set(COMMON_SPEC_GRAMS)
  for (const tier of product?.tiers || []) {
    const spec = toInt(tier.spec_g)
    if (spec > 0) specs.add(spec)
  }
  return [
    ...[...specs].sort((a, b) => a - b).map((spec) => ({ label: formatSpecLabel(spec), value: String(spec) })),
    { label: '自定义克数', value: CUSTOM_SPEC_VALUE },
  ]
}

export function defaultWholesaleSpec(product) {
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
  return tierUsesKgQuantity(tier) ? 'kg' : '件'
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
  const value = String(unit || '').trim().toLowerCase()
  if (['kg', 'lb', 'g100', 'g227', 'g250'].includes(value)) return value
  return ''
}

function priceUnitForDisplayUnit(unit) {
  switch (normalizeTierDisplayUnit(unit)) {
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
      return null
  }
}

function priceUnitForStoredFields(label, suffix, unitG) {
  const normalizedUnitG = toNumber(unitG)
  if (normalizedUnitG === 1000 || label === '元/kg' || suffix === '/kg') return { label: '元/kg', suffix: '/kg', unitG: 1000 }
  if (normalizedUnitG === 454 || label === '元/磅' || suffix === '/磅') return { label: '元/磅', suffix: '/磅', unitG: 454 }
  if (normalizedUnitG === 100 || label === '元/100g' || suffix === '/100g') return { label: '元/100g', suffix: '/100g', unitG: 100 }
  if (normalizedUnitG === 227 || label === '元/227g' || suffix === '/227g') return { label: '元/227g', suffix: '/227g', unitG: 227 }
  if (normalizedUnitG === 250 || label === '元/250g' || suffix === '/250g') return { label: '元/250g', suffix: '/250g', unitG: 250 }
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
  const sourceUnit = priceUnitForDisplayUnit(tier?.display_unit)
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
  const lower = sorted.find((item) => minValue(item) <= quantity)
  if (lower) return { tier: lower, belowMin: false }
  const lowest = [...tiers].sort((a, b) => minValue(a) - minValue(b))[0] || null
  return { tier: lowest, belowMin: Boolean(lowest && quantity < minValue(lowest)) }
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
  return (product?.tiers || [])
    .filter((tier) => toInt(tier.spec_g) > 0)
    .map((tier) => {
      const priceUnit = priceUnitForDisplayUnit(tier?.display_unit) || wholesalePriceUnit(toInt(tier.spec_g))
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

function findWholesaleTierMatch(product, row) {
  const specG = normalizeSpecG(row)
  const qty = Math.max(1, toInt(row?.qty))
  const tiers = (product?.tiers || []).filter((item) => toInt(item.spec_g) > 0)
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
    return {
      tierID: 'auto',
      unitPrice: '',
      priceUnit: orderRowPriceUnit(row),
      tierPriceLabel: '',
      beanListPublicationID: 0,
      beanListVersionNo: '',
      belowMinTier: false,
    }
  }
  const priceUnit = priceUnitForDisplayUnit(tier?.display_unit) || orderRowPriceUnit(row)
  const unitPrice = wholesaleTierDisplayUnitPrice(tier, priceUnit) || 0
  const source = tierPriceSource(tier) || {}
  return {
    tierID: String(tier.id),
    unitPrice: String(unitPrice),
    priceUnit,
    tierPriceLabel: `${formatTierUnitPrice(unitPrice)}${priceUnit.suffix}`,
    beanListPublicationID: toInt(source.publication_id || source.bean_list_publication_id),
    beanListVersionNo: String(source.version_no || source.bean_list_version_no || source.version || '').trim(),
    belowMinTier: matched.belowMin,
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

export function dripSalesUnitSpec(product, row = {}) {
  const salesUnit = normalizeDripSalesUnit(row?.sales_unit)
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
  return toNumber(tier?.min ?? tier?.min_qty_units)
}

function dripTierMax(tier) {
  const max = tier?.max ?? tier?.max_qty_units
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

function normalizeBeanListType(value) {
  const type = String(value || '').trim()
  if (type === 'green') return 'green'
  if (type === 'drip') return 'drip'
  return 'commercial'
}

export function productBeanListType(product) {
  if (isDripProduct(product)) return 'drip'
  if (String(product?.product_kind || '').trim() === 'green_bean') return 'green'
  return 'commercial'
}

export function latestBeanListVersionOption(options, listType) {
  const normalized = normalizeBeanListType(listType)
  const rows = (options || []).filter((item) => normalizeBeanListType(item?.list_type) === normalized)
  if (!rows.length) return null
  return rows.find((item) => item?.is_default) || rows[0]
}

export function rowUsesStaleBeanListPublication(row, options, listType = productBeanListType(row)) {
  if (toInt(row?.product_id) <= 0) return false
  const publicationID = toInt(row?.bean_list_publication_id)
  if (publicationID <= 0) return false
  const latest = latestBeanListVersionOption(options, listType)
  const latestID = toInt(latest?.id)
  return latestID > 0 && latestID !== publicationID
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
  const listType = productBeanListType(product)
  const publicationIDs = publicationIDsByType[listType]
  if (!publicationIDs?.size) return true
  return productMatchesExplicitPublicationScope(product, publicationIDsByType)
}

function productMatchesExplicitPublicationScope(product, publicationIDsByType) {
  const listType = productBeanListType(product)
  const publicationIDs = publicationIDsByType[listType]
  if (!publicationIDs?.size) return false
  return (product?.tiers || []).some((tier) => {
    const source = tierPriceSource(tier)
    if (!source) return false
    const publicationID = toInt(source.publication_id)
    return publicationID > 0
      && publicationIDs.has(publicationID)
      && normalizeBeanListType(source.list_type) === listType
  })
}

function customerAllowsPublicSKU(customerID, publicUsages = []) {
  const selectedCustomerID = toInt(customerID)
  if (selectedCustomerID <= 0) return true
  const usage = (publicUsages || []).find((item) => toInt(item?.customer_id) === selectedCustomerID)
  return usage ? Boolean(usage.use_public_sku) : true
}

export function filterProductsForCustomer(products, customerID, publicationIDsByType = {}, publicUsages = []) {
  const selectedCustomerID = toInt(customerID)
  const scopedPublicationIDs = normalizePublicationIDsByType(publicationIDsByType)
  const allowsPublicSKU = customerAllowsPublicSKU(selectedCustomerID, publicUsages)
  return (products || []).filter((product) => {
    const productCustomerID = toInt(product?.customer_id)
    const visibility = String(product?.visibility || (productCustomerID > 0 ? 'customer_only' : 'public')).trim()
    if (visibility === 'public' || productCustomerID === 0) {
      if (
        selectedCustomerID > 0
        && !allowsPublicSKU
        && !productMatchesExplicitPublicationScope(product, scopedPublicationIDs)
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
  return Math.max(base - lineDiscountAmount(base, row, retailOrder), 0)
}

export function lineTotalBeforeDiscount(product, row, retailOrder) {
  const units = Math.max(0, toInt(row?.qty))
  if (isDripProduct(product) || row?.product_kind === 'drip_bag') {
    if (units <= 0) return 0
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

export function rowUnitDiscountUnits(row, retailOrder = false) {
  const units = Math.max(0, toInt(row?.qty))
  if (units <= 0) return 0
  if (row?.product_kind === 'drip_bag' || row?.sales_unit === 'bag' || row?.sales_unit === 'box') return units
  if (retailOrder) return units
  const specG = normalizeSpecG(row)
  if (specG <= 0) return units
  return (specG * units) / orderRowPriceUnit(row).unitG
}

export function lineDiscountAmount(baseLineTotal, row, retailOrder = false) {
  const base = Math.max(0, toNumber(baseLineTotal))
  if (base <= 0) return 0
  const type = String(row?.discount_type || '').trim()
  const value = Math.max(0, toNumber(row?.discount_value))
  if (type === 'free') return base
  if (type === 'amount') return Math.min(value, base)
  if (type === 'unit_amount') return Math.min(value * rowUnitDiscountUnits(row, retailOrder), base)
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
    const productKind = row.product_kind === 'drip_bag' ? 'drip_bag' : row.product_kind === 'green_bean' ? 'green_bean' : 'roasted_bean'
    const dripSpec = dripSalesUnitSpec(null, row)
    const specG = productKind === 'drip_bag' ? dripSpec.specG : normalizeSpecG(row)
    const qty = toInt(row.qty)
    if (productID <= 0 || specG <= 0 || qty <= 0) continue
    payload.product_id.push(String(productID))
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
