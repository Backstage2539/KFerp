export const CUSTOM_SPEC_VALUE = 'custom'
export const COMMON_SPEC_GRAMS = [36, 80, 100, 227, 454, 500, 1000, 2500]

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

export function wholesaleSpecOptions(product) {
  const specs = new Set(COMMON_SPEC_GRAMS)
  for (const tier of product?.tiers || []) {
    const spec = toInt(tier.spec_g)
    if (spec > 0) specs.add(spec)
  }
  return [...specs].sort((a, b) => a - b).map((spec) => ({ label: formatSpecLabel(spec), value: String(spec) }))
}

export function defaultWholesaleSpec(product) {
  const tier = (product?.tiers || []).find((item) => toInt(item.spec_g) > 0)
  if (tier) return String(toInt(tier.spec_g))
  return wholesaleSpecOptions(product)[0]?.value || ''
}

export function formatTierRange(tier) {
  const min = toNumber(tier?.min)
  const max = tier?.max == null ? 0 : toNumber(tier.max)
  if (min > 0 && max > 0) return `${trimNumber(min)}-${trimNumber(max)}件`
  if (min > 0) return `${trimNumber(min)}件+`
  if (max > 0) return `≤${trimNumber(max)}件`
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

export function wholesalePriceUnit(rowOrSpec) {
  const specG = typeof rowOrSpec === 'object' ? normalizeSpecG(rowOrSpec) : toInt(rowOrSpec)
  if (specG >= 1000) return { label: '元/kg', suffix: '/kg', unitG: 1000 }
  return { label: '元/磅', suffix: '/磅', unitG: 454 }
}

function rowQuantityForWholesalePriceUnit(row) {
  const unit = wholesalePriceUnit(row)
  return normalizeSpecG(row) * Math.max(1, toInt(row?.qty)) / unit.unitG
}

function wholesaleTierUnitPriceLb(tier) {
  const configuredPrice = toNumber(tier?.unit_price)
  const pricePerPackage = configuredPrice > 0 ? configuredPrice : toNumber(tier?.price)
  if (pricePerPackage <= 0) return 0
  return pricePerPackage * 454 / tierSpecG(tier)
}

function wholesaleDisplayUnitPrice(pricePerLb, rowOrSpec) {
  const unit = wholesalePriceUnit(rowOrSpec)
  const price = toNumber(pricePerLb) * unit.unitG / 454
  if (unit.unitG === 1000) return Math.round(price)
  return price
}

function matchTierByQuantity(tiers, quantity, minValue, maxValue) {
  const sorted = [...tiers].sort((a, b) => minValue(b) - minValue(a))
  const exact = sorted.find((item) => minValue(item) <= quantity && (maxValue(item) == null || maxValue(item) >= quantity))
  if (exact) return exact
  return sorted.find((item) => minValue(item) <= quantity)
    || [...tiers].sort((a, b) => minValue(a) - minValue(b))[0]
    || null
}

export function wholesaleTierPriceRows(product, row = null) {
  return (product?.tiers || [])
    .filter((tier) => toInt(tier.spec_g) > 0)
    .map((tier) => {
      const priceUnitTarget = row || toInt(tier.spec_g)
      return {
        id: String(tier.id || ''),
        specG: toInt(tier.spec_g),
        specLabel: formatSpecLabel(tier.spec_g),
        rangeLabel: formatTierRange(tier),
        unitPrice: wholesaleDisplayUnitPrice(wholesaleTierUnitPriceLb(tier), priceUnitTarget),
        priceUnit: wholesalePriceUnit(priceUnitTarget),
      }
    })
}

export function findWholesaleTier(product, row) {
  const specG = normalizeSpecG(row)
  const qty = Math.max(1, toInt(row?.qty))
  const tiers = (product?.tiers || []).filter((item) => toInt(item.spec_g) > 0)
  const exactSpecTiers = tiers
    .filter((item) => toInt(item.spec_g) === specG)
  if (exactSpecTiers.length) {
    return matchTierByQuantity(exactSpecTiers, qty, (item) => toNumber(item.min), (item) => (item.max == null ? null : toNumber(item.max)))
  }
  return matchTierByQuantity(tiers, rowQuantityLb(row), tierMinLb, tierMaxLb)
}

export function syncWholesaleTierPrice(product, row) {
  const tier = findWholesaleTier(product, row)
  if (!tier) return { tierID: 'auto', unitPrice: '' }
  return { tierID: String(tier.id), unitPrice: String(wholesaleDisplayUnitPrice(wholesaleTierUnitPriceLb(tier), row) || 0) }
}

export function filterOptions(options, query) {
  const q = String(query || '').trim().toLowerCase()
  if (!q) return options || []
  return (options || []).filter((item) => {
    const haystack = `${item.name || ''} ${item.py || ''} ${item.pyi || ''} ${item.code || ''}`.toLowerCase()
    return haystack.includes(q)
  })
}

export function filterProductsForCustomer(products, customerID) {
  const selectedCustomerID = toInt(customerID)
  return (products || []).filter((product) => {
    const productCustomerID = toInt(product?.customer_id)
    const visibility = String(product?.visibility || (productCustomerID > 0 ? 'customer_only' : 'public')).trim()
    if (visibility === 'public' || productCustomerID === 0) return true
    return selectedCustomerID > 0 && productCustomerID === selectedCustomerID
  })
}

export function responsibleOptions({ employees = [], customers = [] } = {}) {
  const employeeOptions = (employees || [])
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
  const customerOptions = (customers || [])
    .filter((item) => toInt(item?.id) > 0 && String(item?.name || '').trim())
    .map((item) => {
      const name = String(item.name || '').trim()
      const contact = String(item.contact || '').trim()
      const phone = String(item.phone || '').trim()
      const meta = [contact, phone].filter(Boolean).join(' ')
      return {
        type: 'customer',
        id: toInt(item.id),
        name,
        label: `合作方/客户 - ${name}`,
        meta,
        search: ['合作方', '客户', name, contact, phone].filter(Boolean).join(' '),
      }
    })
  return [...employeeOptions, ...customerOptions]
}

export function defaultStatusID(options, names) {
  const wanted = (names || []).map((name) => String(name).trim()).filter(Boolean)
  for (const name of wanted) {
    const found = (options || []).find((item) => String(item.name || '').trim() === name)
    if (found) return toInt(found.id)
  }
  return 0
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
  const units = Math.max(0, toInt(row?.qty))
  const specG = normalizeSpecG(row)
  if (units <= 0 || specG <= 0) return 0
  if (row?.tier_id === 'manual') {
    return retailOrder ? toNumber(row?.unit_price) * units : toNumber(row?.unit_price) * rowQuantityForWholesalePriceUnit(row)
  }
  if (retailOrder) return retailPackagePrice(product, specG) * units
  const price = toNumber(row?.unit_price)
  return price * rowQuantityForWholesalePriceUnit(row)
}

export function buildOrderPayload({ form, rows }) {
  const payload = {
    edit_id: Number(form.edit_id || 0),
    order_date: form.order_date || '',
    customer_id: Number(form.customer_id || 0),
    source_id: Number(form.source_id || 0),
    order_type_id: Number(form.order_type_id || 0),
    pay_status_id: Number(form.pay_status_id || 0),
    ship_status_id: Number(form.ship_status_id || 0),
    ship_method: form.ship_method || '',
    ship_tracking_no: form.ship_tracking_no || '',
    responsible_type: form.responsible_type || '',
    responsible_id: Number(form.responsible_id || 0),
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
  }

  for (const row of rows || []) {
    const productID = toInt(row.product_id)
    const specG = normalizeSpecG(row)
    const qty = toInt(row.qty)
    if (productID <= 0 || specG <= 0 || qty <= 0) continue
    payload.product_id.push(String(productID))
    payload.tier_id.push(row.tier_id || 'auto')
    payload.unit_price.push(String(row.unit_price || ''))
    payload.item_name.push(row.product_name || row.item_name || '')
    payload.item_note.push(String(row.item_note || '').trim())
    payload.qty.push(String(qty))
    payload.unit.push(row.unit || '件')
    payload.spec.push(String(specG))
  }

  return payload
}
