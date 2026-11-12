function productID(row = {}) {
  return Number(row.parent_product_id || row.effective_parent_product_id || row.product_id || row.productId || 0)
}
export function publicationProductPickerError(publication = {}) {
  if (publication.status !== 'published') return '请选择已发布价格表'
  if (publication.owner_type !== 'official' || (publication.publication_purpose || 'factory_supply') !== 'factory_supply') return '请选择公共供货价格表'
  const rows = publication.content?.price_rows
  if (!Array.isArray(rows) || !rows.length) return '来源价格表没有可选商品'
  if (rows.some((row) => !(productID(row) > 0))) return '来源价格表存在无法识别商品身份的价格行，请先处理来源表'
  return ''
}
export function publicationProducts(publication = {}) {
  const seen = new Map()
  for (const row of publication.content?.price_rows || []) {
    const id = productID(row)
    if (id > 0 && !seen.has(id)) seen.set(id, { id, name: String(row.parent_product_name || row.product_name || row.name || `商品 #${id}`) })
  }
  return [...seen.values()]
}
export function publicPublicationURL(row = {}) {
  const params = new URLSearchParams({scope:'official',list_type:String(row.list_type || 'commercial'),publication_purpose:'factory_supply'})
  if (Number(row.product_type_category_id) > 0) params.set('product_type_category_id', String(row.product_type_category_id))
  if (Number(row.classification_template_id) > 0) params.set('classification_template_id', String(row.classification_template_id))
  return `/api/costing/bean-list/publications/${Number(row.id || 0)}?${params}`
}
