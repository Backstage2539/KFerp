export type MallTemplateKey = 'hero' | 'compact' | 'wide'

export type MallProduct = {
  id: number
  product_id: number
  title: string
  subtitle: string
  description: string
  image_url: string
  spec_g: number
  unit_price: number
  template_key: MallTemplateKey
  status: string
  sort_order: number
}

export type MallCartItem = {
  mall_product_id: number
  title: string
  unit_price: number
  qty: number
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
  items: Array<{ mall_product_id: number; qty: number }>
}

export function normalizeMallTemplateKey(value: unknown): MallTemplateKey {
  return value === 'compact' || value === 'wide' ? value : 'hero'
}

export function normalizeMallProduct(row: Partial<Record<keyof MallProduct, unknown>> = {}): MallProduct {
  return {
    id: Number(row.id || 0),
    product_id: Number(row.product_id || 0),
    title: String(row.title || '').trim() || '商品',
    subtitle: String(row.subtitle || '').trim(),
    description: String(row.description || '').trim(),
    image_url: String(row.image_url || '').trim(),
    spec_g: Number(row.spec_g || 0) > 0 ? Number(row.spec_g) : 454,
    unit_price: Number(row.unit_price || 0),
    template_key: normalizeMallTemplateKey(row.template_key),
    status: String(row.status || '').trim() || 'published',
    sort_order: Number(row.sort_order || 0),
  }
}

export function addMallCartItem(cart: MallCartItem[], product: MallProduct, qty = 1): MallCartItem[] {
  const nextQty = Math.max(1, Number(qty || 1))
  const existing = cart.find((item) => item.mall_product_id === product.id)
  if (!existing) {
    return [...cart, { mall_product_id: product.id, title: product.title, unit_price: product.unit_price, qty: nextQty }]
  }
  return cart.map((item) => (
    item.mall_product_id === product.id
      ? { ...item, qty: item.qty + nextQty }
      : item
  ))
}

export function updateMallCartQty(cart: MallCartItem[], mallProductID: number, qty: number): MallCartItem[] {
  const nextQty = Math.floor(Number(qty || 0))
  if (nextQty <= 0) return cart.filter((item) => item.mall_product_id !== mallProductID)
  return cart.map((item) => (item.mall_product_id === mallProductID ? { ...item, qty: nextQty } : item))
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
      .map((item) => ({ mall_product_id: item.mall_product_id, qty: Math.floor(item.qty) })),
  }
  const note = String(form.note || '').trim()
  if (note) payload.note = note
  return payload
}

export function formatMallMoney(value: number): string {
  return `¥${Number(value || 0).toFixed(2)}`
}
