export function conversionSnapshot(row) {
  try { return JSON.parse(row?.price_source_json || '{}').spec_conversion || null } catch { return null }
}
export function conversionTargets(group, specs) {
  const same = (a, b) => String(a || '').toLowerCase() === String(b || '').toLowerCase()
  return (specs || []).filter(spec => (group?.members || []).filter(member =>
    same(member.inventory_unit, spec.inventory_unit) && (Number(member.bom_spec_id) > 0
      ? Number(member.bom_spec_id) === Number(spec.bom_spec_id)
      : Number(member.template_id) > 0 && Number(member.template_id) === Number(spec.template_id) && member.template_key === spec.template_key),
  ).length === 1)
}
export function convertedOrderRow(row, quote) {
  return {
    ...row, product_id: Number(quote.product_id), parent_product_id: Number(quote.product_id),
    bom_spec_id: Number(quote.target_bom_spec_id), bom_variant_id: Number(quote.target_bom_variant_id),
    migration_state: 'cutover', spec_identity_mode: 'bom_spec', bom_spec_authoritative: true,
    spec_mode: String(quote.target_bom_spec_id), spec_source: 'price_list_sku', spec_label: quote.target_spec_name,
    inventory_unit: quote.target_unit, unit: quote.target_unit, sales_unit: quote.target_unit, spec_g: 0, unit_bean_g: 0, unit_bag_count: 0,
    quantity_basis: 'sales_spec_count', price_selection_mode: quote.pricing_method === 'bom_rule' ? 'bom_tier' : 'converted_tier', selected_price_row_key: quote.price_row_key,
    spec_conversion: { pricing_method: quote.pricing_method, quote_revision: quote.quote_revision, current_rule_revision: quote.current_rule_revision, group_id: quote.group_id, revision: quote.revision, source_bom_spec_id: quote.source_bom_spec_id, source_bom_variant_id: quote.source_bom_variant_id, reprice: true },
    unit_price: String(quote.unit_price), price_source_json: quote.price_source_json,
    bean_list_publication_id: Number(quote.publication_id), bean_list_version_no: quote.version_no,
    price_unit: `元/${quote.target_unit}`, price_unit_suffix: `/${quote.target_unit}`, price_unit_g: 1,
    manual_price: false, tier_id: 'auto', price_missing: false, spec_invalid_message: '', historical_spec_readonly: false,
  }
}
export function conversionDescription(q) {
  if (!q) return ''
  if (q.pricing_method === 'bom_rule') return `${q.source_spec_name} · ${q.tier_label || '所选档位'} → ${q.target_spec_name}；${q.pricing_rule_name}${q.pricing_rule_basis === 'confirmed_current_template' ? '（已确认当前模板）' : q.pricing_rule_basis === 'verified_legacy_template' ? '（已核实旧表模板未变化）' : ''}；${q.unit_price}元/${q.target_unit}`
  return `${q.group_name}：1${q.source_unit} ${q.source_spec_name} = ${q.source_quantity}${q.base_unit}；1${q.target_unit} ${q.target_spec_name} = ${q.target_quantity}${q.base_unit}。${q.source_unit_price} × ${q.target_quantity} ÷ ${q.source_quantity} = ${q.unit_price}元/${q.target_unit}`
}
export function latestConversionRequest() {
  let revision = 0
  return { next: () => ++revision, current: value => value === revision, cancel: () => ++revision }
}
