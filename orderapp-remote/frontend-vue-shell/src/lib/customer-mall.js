export const mallTemplateOptions = [
  { key: 'hero', label: '大图' },
  { key: 'compact', label: '紧凑' },
  { key: 'wide', label: '横幅' },
]

export const mallStatusOptions = [
  { key: 'draft', label: '草稿' },
  { key: 'published', label: '上架' },
]

export function normalizeMallTemplateKey(value) {
  const key = String(value || '').trim()
  return mallTemplateOptions.some((item) => item.key === key) ? key : 'hero'
}

export function normalizeMallProductStatus(value) {
  const key = String(value || '').trim()
  return mallStatusOptions.some((item) => item.key === key) ? key : 'draft'
}

export function optionForProduct(productID, productOptions = []) {
  const id = Number(productID || 0)
  return (productOptions || []).find((item) => Number(item.id || 0) === id) || null
}

export function createBlankMallProduct(productOptions = []) {
  const first = productOptions?.[0] || {}
  return normalizeMallProduct({
    product_id: Number(first.id || 0),
    title: first.name || '',
    unit_price: Number(first.default_price || 0),
    spec_g: 454,
    template_key: 'hero',
    status: 'draft',
    sort_order: 100,
  }, productOptions)
}

export function normalizeMallProduct(row = {}, productOptions = []) {
  const option = optionForProduct(row.product_id, productOptions)
  const title = String(row.title || '').trim() || option?.name || ''
  return {
    id: Number(row.id || 0),
    product_id: Number(row.product_id || 0),
    title,
    subtitle: String(row.subtitle || '').trim(),
    description: String(row.description || '').trim(),
    image_url: String(row.image_url || '').trim(),
    spec_g: Number(row.spec_g || 0) > 0 ? Number(row.spec_g) : 454,
    unit_price: Number(row.unit_price || 0),
    template_key: normalizeMallTemplateKey(row.template_key),
    status: normalizeMallProductStatus(row.status),
    sort_order: Number(row.sort_order || 0) > 0 ? Number(row.sort_order) : 100,
  }
}

export function formatMallPrice(value) {
  return `¥${Number(value || 0).toFixed(2)}`
}
