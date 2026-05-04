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

export function wholesaleTierPriceRows(product) {
  return (product?.tiers || [])
    .filter((tier) => toInt(tier.spec_g) > 0)
    .map((tier) => ({
      id: String(tier.id || ''),
      specG: toInt(tier.spec_g),
      specLabel: formatSpecLabel(tier.spec_g),
      rangeLabel: formatTierRange(tier),
      unitPrice: toNumber(tier.unit_price || tier.price),
    }))
}

export function findWholesaleTier(product, row) {
  const specG = normalizeSpecG(row)
  const qty = Math.max(1, toInt(row?.qty))
  const tiers = (product?.tiers || [])
    .filter((item) => toInt(item.spec_g) === specG)
    .sort((a, b) => toNumber(b.min) - toNumber(a.min))
  const exact = tiers.find((item) => toNumber(item.min) <= qty && (!item.max || toNumber(item.max) >= qty))
  if (exact) return exact
  return tiers
    .filter((item) => toNumber(item.min) <= qty)
    .sort((a, b) => toNumber(b.min) - toNumber(a.min))[0] || tiers.sort((a, b) => toNumber(a.min) - toNumber(b.min))[0] || null
}

export function syncWholesaleTierPrice(product, row) {
  const tier = findWholesaleTier(product, row)
  if (!tier) return { tierID: 'auto', unitPrice: '' }
  return { tierID: String(tier.id), unitPrice: String(tier.unit_price || tier.price || 0) }
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
  if (row?.tier_id === 'manual') return toNumber(row?.unit_price) * units
  if (retailOrder) return retailPackagePrice(product, specG) * units
  const price = toNumber(row?.unit_price)
  return price * units
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
    payload.qty.push(String(qty))
    payload.unit.push(row.unit || '件')
    payload.spec.push(String(specG))
  }

  return payload
}
