export function orderedBOMSpecs(specs = []) {
  return [...specs].sort((a,b) => Number(Boolean(b.is_default))-Number(Boolean(a.is_default)) || Number(a.sort_order || 0)-Number(b.sort_order || 0) || Number(a.bom_spec_id)-Number(b.bom_spec_id))
}
export function bomTierSelection(tier) {
  return { price_selection_mode:'bom_tier',selected_price_row_key:tier?.price_row_key || '',spec_conversion:tier ? {pricing_method:'bom_rule',group_id:0,revision:0,source_bom_spec_id:tier.bom_spec_id,source_bom_variant_id:tier.bom_variant_id,reprice:true} : null }
}
export function bomQuoteIsCurrent(a,b) {
  return ['customer','product','spec','variant','publication','key'].every(key=>String(a[key] ?? '')===String(b[key] ?? ''))
}
