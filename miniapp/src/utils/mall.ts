import type { ProductKind, SalesUnit } from '../api/customerPortal'

export type MallTemplateKey = 'hero' | 'compact' | 'wide'

export type MallProduct = {
  id: number
  product_id: number
  product_kind?: string
  title: string
  subtitle: string
  description: string
  image_url: string
  spec_g: number
  unit_price: number
  mall_price?: number
  template_key: MallTemplateKey
  status: string
  sort_order: number
  product_kind: ProductKind
  sales_units: SalesUnit[]
  sales_unit?: SalesUnit
  unit_label?: string
  unit_bag_count?: number
  unit_bean_g?: number
  base_unit_price?: number
  price_sales_unit?: SalesUnit
  drip_bag_grams?: number
  drip_box_bag_count?: number
}

export type MallCartItem = {
  mall_product_id: number
  title: string
  product_kind?: string
  unit_price: number
  qty: number
  sales_unit?: SalesUnit
  unit_label?: string
  unit_bag_count?: number
  unit_bean_g?: number
}

export type MallRecipientForm = {
  name: string
  phone: string
  address: string
  note?: string
}

export type MallOrderPayload = {
  recipient_name: string
  recipient_phone: string
  recipient_address: string
  note?: string
  items: Array<{
    mall_product_id: number
    qty: number
    sales_unit?: SalesUnit
    unit_bag_count?: number
    unit_bean_g?: number
  }>
}

export function normalizeMallTemplateKey(value: unknown): MallTemplateKey {
  return value === 'compact' || value === 'wide' ? value : 'hero'
}

export function normalizeMallProduct(row: Record<string, unknown> = {}): MallProduct {
  const productKind = normalizeProductKind(row.product_kind)
  const salesUnits = normalizeSalesUnits(row.sales_units, productKind)
  const dripBagGrams = positiveNumber(row.drip_bag_grams) || 10
  const dripBoxBagCount = positiveInteger(row.drip_box_bag_count) || 10
  const requestedSalesUnit = normalizeSalesUnit(row.sales_unit)
  const salesUnit = productKind === 'drip_bag' ? (requestedSalesUnit && salesUnits.includes(requestedSalesUnit) ? requestedSalesUnit : salesUnits[0]) : undefined
  const priceSalesUnit = productKind === 'drip_bag' ? normalizePriceSalesUnit(row, salesUnits) : undefined
  const baseUnitPrice = productKind === 'drip_bag' ? dripMallPrice(row) : Number(row.unit_price || 0)
  const unitBagCount = salesUnit === 'box' ? dripBoxBagCount : salesUnit === 'bag' ? 1 : undefined
  const unitBeanG = salesUnit ? dripBagGrams : undefined
  const unitPrice = productKind === 'drip_bag' ? mallUnitPriceForSalesUnit(baseUnitPrice, priceSalesUnit, salesUnit, dripBoxBagCount) : baseUnitPrice
  return {
    id: Number(row.id || 0),
    product_id: Number(row.product_id || 0),
    product_kind: String(row.product_kind || '').trim() === 'green_bean' ? 'green_bean' : 'roasted',
    title: String(row.title || '').trim() || '商品',
    subtitle: String(row.subtitle || '').trim(),
    description: String(row.description || '').trim(),
    image_url: String(row.image_url || '').trim(),
    spec_g: Number(row.spec_g || 0) > 0 ? Number(row.spec_g) : 454,
    unit_price: unitPrice,
    mall_price: unitPrice || undefined,
    template_key: normalizeMallTemplateKey(row.template_key),
    status: String(row.status || '').trim() || 'published',
    sort_order: Number(row.sort_order || 0),
    product_kind: productKind,
    sales_units: salesUnits,
    sales_unit: salesUnit,
    unit_label: salesUnit ? salesUnitLabel(salesUnit, dripBagGrams, dripBoxBagCount) : undefined,
    unit_bag_count: unitBagCount,
    unit_bean_g: unitBeanG,
    base_unit_price: productKind === 'drip_bag' ? baseUnitPrice : undefined,
    price_sales_unit: priceSalesUnit,
    drip_bag_grams: productKind === 'drip_bag' ? dripBagGrams : undefined,
    drip_box_bag_count: productKind === 'drip_bag' ? dripBoxBagCount : undefined,
  }
}

export function addMallCartItem(cart: MallCartItem[], product: MallProduct, qty = 1): MallCartItem[] {
  const nextQty = Math.max(1, Number(qty || 1))
  const existing = cart.find((item) => item.mall_product_id === product.id && item.sales_unit === product.sales_unit)
  const nextItem = mallCartItemFromProduct(product, nextQty)
  if (!existing) {
    return [...cart, nextItem]
  }
  return cart.map((item) => (
    item.mall_product_id === product.id && item.sales_unit === product.sales_unit
      ? { ...item, qty: item.qty + nextQty }
      : item
  ))
}

export function updateMallCartQty(cart: MallCartItem[], mallProductID: number, qty: number, salesUnit?: SalesUnit | string): MallCartItem[] {
  const nextQty = Math.floor(Number(qty || 0))
  const unit = normalizeSalesUnit(salesUnit)
  const matches = (item: MallCartItem) => item.mall_product_id === mallProductID && (!unit || item.sales_unit === unit)
  if (nextQty <= 0) return cart.filter((item) => !matches(item))
  return cart.map((item) => (matches(item) ? { ...item, qty: nextQty } : item))
}

export function mallCartTotal(cart: MallCartItem[]): number {
  return cart.reduce((sum, item) => sum + Number(item.unit_price || 0) * Number(item.qty || 0), 0)
}

export function mallCartCount(cart: MallCartItem[]): number {
  return cart.reduce((sum, item) => sum + Number(item.qty || 0), 0)
}

export function buildMallOrderPayload(form: MallRecipientForm, cart: MallCartItem[]): MallOrderPayload {
  const payload: MallOrderPayload = {
    recipient_name: String(form.name || '').trim(),
    recipient_phone: String(form.phone || '').trim(),
    recipient_address: String(form.address || '').trim(),
    items: cart
      .filter((item) => item.mall_product_id > 0 && item.qty > 0)
      .map((item) => {
        const row: MallOrderPayload['items'][number] = {
          mall_product_id: item.mall_product_id,
          qty: Math.floor(item.qty),
        }
        if (item.sales_unit) {
          row.sales_unit = item.sales_unit
          row.unit_bag_count = positiveInteger(item.unit_bag_count) || (item.sales_unit === 'box' ? 10 : 1)
          row.unit_bean_g = positiveNumber(item.unit_bean_g) || 10
        }
        return row
      }),
  }
  const note = String(form.note || '').trim()
  if (note) payload.note = note
  return payload
}

export function visibleMallProducts(rows: unknown[] = []): MallProduct[] {
  return rows.map((row) => normalizeMallProduct(asRecord(row))).filter((product) => product.unit_price > 0)
}

export function mallProductUnitLabel(product: MallProduct): string {
  if (product.product_kind !== 'drip_bag') return `${product.spec_g}g`
  const bagGrams = positiveNumber(product.drip_bag_grams) || positiveNumber(product.unit_bean_g) || 10
  const boxBagCount = positiveInteger(product.drip_box_bag_count) || positiveInteger(product.unit_bag_count) || 10
  return product.sales_units.map((unit) => salesUnitLabel(unit, bagGrams, boxBagCount)).join(' / ')
}

export function mallProductForSalesUnit(product: MallProduct, salesUnit?: SalesUnit | string): MallProduct {
  if (product.product_kind !== 'drip_bag') return product
  const unit = normalizeSalesUnit(salesUnit)
  if (!unit || !product.sales_units.includes(unit)) return product
  const bagGrams = positiveNumber(product.drip_bag_grams) || positiveNumber(product.unit_bean_g) || 10
  const boxBagCount = positiveInteger(product.drip_box_bag_count) || positiveInteger(product.unit_bag_count) || 10
  const priceSalesUnit = product.price_sales_unit || (product.sales_units.length === 1 ? product.sales_units[0] : 'bag')
  const baseUnitPrice = positiveNumber(product.base_unit_price) || positiveNumber(product.mall_price) || positiveNumber(product.unit_price)
  return {
    ...product,
    unit_price: mallUnitPriceForSalesUnit(baseUnitPrice, priceSalesUnit, unit, boxBagCount),
    sales_unit: unit,
    unit_label: salesUnitLabel(unit, bagGrams, boxBagCount),
    unit_bag_count: unit === 'box' ? boxBagCount : 1,
    unit_bean_g: bagGrams,
    base_unit_price: baseUnitPrice,
    price_sales_unit: priceSalesUnit,
  }
}

export function formatMallMoney(value: number): string {
  return `¥${Number(value || 0).toFixed(2)}`
}

function mallCartItemFromProduct(product: MallProduct, qty: number): MallCartItem {
  const item: MallCartItem = {
    mall_product_id: product.id,
    title: product.title,
    unit_price: product.unit_price,
    qty,
  }
  if (product.product_kind === 'drip_bag' && product.sales_unit) {
    item.sales_unit = product.sales_unit
    item.unit_label = product.unit_label || mallProductUnitLabel(product)
    item.unit_bag_count = positiveInteger(product.unit_bag_count) || (product.sales_unit === 'box' ? 10 : 1)
    item.unit_bean_g = positiveNumber(product.unit_bean_g) || positiveNumber(product.drip_bag_grams) || 10
  }
  return item
}

function normalizeProductKind(value: unknown): ProductKind {
  return value === 'drip_bag' ? 'drip_bag' : 'roasted_bean'
}

function normalizeSalesUnit(value: unknown): SalesUnit | '' {
  return value === 'box' || value === 'bag' ? value : ''
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? value as Record<string, unknown> : {}
}

function normalizeSalesUnits(value: unknown, productKind: ProductKind): SalesUnit[] {
  if (productKind !== 'drip_bag') return []
  const source = Array.isArray(value) ? value : ['bag', 'box']
  const out: SalesUnit[] = []
  for (const item of source) {
    const unit = normalizeSalesUnit(item)
    if (unit && !out.includes(unit)) out.push(unit)
  }
  return out.length ? out : ['bag', 'box']
}

function normalizePriceSalesUnit(row: Record<string, unknown>, salesUnits: SalesUnit[]): SalesUnit {
  const explicit = normalizeSalesUnit(row.price_sales_unit || row.mall_price_sales_unit || row.unit_price_sales_unit)
  if (explicit) return explicit
  return salesUnits.length === 1 ? salesUnits[0] : 'bag'
}

function dripMallPrice(row: Record<string, unknown>): number {
  const explicit = firstPositiveNumber(row.mall_price, row.mall_unit_price, row.mall_price_per_unit, row.customer_price, row.price_per_unit)
  if (explicit > 0) return explicit
  if (row.has_mall_price === true || row.price_source === 'mall') return positiveNumber(row.unit_price)
  return 0
}

function mallUnitPriceForSalesUnit(baseUnitPrice: number, priceSalesUnit: SalesUnit | undefined, salesUnit: SalesUnit | undefined, boxBagCount: number): number {
  const price = positiveNumber(baseUnitPrice)
  if (!price || !salesUnit) return price
  const sourceUnit = priceSalesUnit || 'bag'
  if (sourceUnit === salesUnit) return price
  if (sourceUnit === 'bag' && salesUnit === 'box') return price * Math.max(1, boxBagCount)
  if (sourceUnit === 'box' && salesUnit === 'bag') return price / Math.max(1, boxBagCount)
  return price
}

function firstPositiveNumber(...values: unknown[]): number {
  for (const value of values) {
    const n = positiveNumber(value)
    if (n > 0) return n
  }
  return 0
}

function salesUnitLabel(unit: SalesUnit, bagGrams: number, boxBagCount: number): string {
  return unit === 'box' ? `盒(${boxBagCount}袋)` : `袋(${formatNumber(bagGrams)}g)`
}

function positiveNumber(value: unknown): number {
  const n = Number(value || 0)
  return Number.isFinite(n) && n > 0 ? n : 0
}

function positiveInteger(value: unknown): number {
  const n = Number(value || 0)
  return Number.isFinite(n) && n > 0 ? Math.trunc(n) : 0
}

function formatNumber(value: number): string {
  return Number.isInteger(value) ? String(value) : value.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
}
