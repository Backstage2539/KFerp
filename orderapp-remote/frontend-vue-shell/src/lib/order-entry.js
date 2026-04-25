export const CUSTOM_SPEC_VALUE = 'custom'

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
  const specs = [...new Set((product?.retail_specs || []).map(toInt).filter((spec) => spec > 0))]
    .sort((a, b) => a - b)
  return [
    ...specs.map((spec) => ({ label: `${spec}g`, value: String(spec) })),
    { label: '自定义克数', value: CUSTOM_SPEC_VALUE },
  ]
}

export function normalizeSpecG(row) {
  if (row?.spec_mode === CUSTOM_SPEC_VALUE) {
    return Math.max(0, toInt(row.custom_spec_g))
  }
  return Math.max(0, toInt(row?.spec_g || row?.spec_mode))
}

export function lineTotal(product, row, retailOrder) {
  const units = Math.max(0, toInt(row?.qty))
  const specG = normalizeSpecG(row)
  if (units <= 0 || specG <= 0) return 0
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
