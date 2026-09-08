export function priceTableOrderabilityBlockedReason(selections = [], families = []) {
  const invalid = selections.find(row => !(Number(row.bom_spec_id) > 0 && Number(row.bom_variant_id) > 0))
  if (!invalid) return ''
  const parentID = Number(invalid.parent_product_id || invalid.product_id || 0)
  const family = families.find(row => Number(row.parent_product_id || row.product_id || row.id || 0) === parentID)
  const name = family?.parent_product_name || family?.name || `#${parentID}`
  return `商品「${name}」未选择可用于录单的已发布 BOM 规格。请配置并发布该商品的默认 BOM 后重新选择规格，再生成或发布价格表。`
}
