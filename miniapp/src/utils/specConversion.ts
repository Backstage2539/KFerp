import type { EmployeeOrderDraftItem, SpecConversionCatalog, SpecConversionGroup, SpecConversionQuote } from '../api/customerPortal'
export function conversionSnapshot(row: { price_source_json?: string }): SpecConversionQuote | null {
  try { return JSON.parse(row.price_source_json || '{}').spec_conversion || null } catch { return null }
}
export function conversionTargets(group: SpecConversionGroup | undefined, specs: SpecConversionCatalog['specs']) {
  return specs.filter(s => (group?.members || []).filter(m => String(m.inventory_unit).toLowerCase() === String(s.inventory_unit).toLowerCase() && (m.bom_spec_id ? m.bom_spec_id === s.bom_spec_id : Boolean(m.template_id) && m.template_id === s.template_id && m.template_key === s.template_key)).length === 1)
}
export function applyConversionQuote(row: EmployeeOrderDraftItem, q: SpecConversionQuote): EmployeeOrderDraftItem {
  return { ...row, product_id:q.product_id,product_family_id:q.product_id,migration_state:'cutover',bom_spec_id:q.target_bom_spec_id,bom_variant_id:q.target_bom_variant_id,spec_label:q.target_spec_name,sales_unit:q.target_unit,spec_g:0,unit_bean_g:0,unit_bag_count:0,unit_price:Number(q.unit_price),bean_list_publication_id:q.publication_id,bean_list_version_no:q.version_no,price_override:false,price_selection_mode:q.pricing_method==='bom_rule'?'bom_tier':'converted_tier',selected_price_row_key:q.price_row_key,price_source_json:q.price_source_json,validation_error:'',spec_conversion:{pricing_method:q.pricing_method,quote_revision:q.quote_revision,group_id:q.group_id,revision:q.revision,source_bom_spec_id:q.source_bom_spec_id,source_bom_variant_id:q.source_bom_variant_id,reprice:true} }
}
export function conversionDescription(q: SpecConversionQuote | null): string {
  if(q?.pricing_method==='bom_rule')return `${q.source_spec_name} · ${q.tier_label || '所选档位'} → ${q.target_spec_name}；${q.pricing_rule_name}；${q.unit_price}元/${q.target_unit}`
  return q ? `${q.group_name}：1${q.source_unit} ${q.source_spec_name} = ${q.source_quantity}${q.base_unit}；1${q.target_unit} ${q.target_spec_name} = ${q.target_quantity}${q.base_unit}。${q.source_unit_price} × ${q.target_quantity} ÷ ${q.source_quantity} = ${q.unit_price}元/${q.target_unit}` : ''
}
