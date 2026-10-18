import type { EmployeeOrderDraftItem, SpecConversionCatalog } from '../api/customerPortal'
type Spec = SpecConversionCatalog['specs'][number]
export function orderedBOMSpecs(specs:Spec[]=[]):Spec[]{return [...specs].sort((a,b)=>Number(Boolean(b.is_default))-Number(Boolean(a.is_default))||Number(a.sort_order||0)-Number(b.sort_order||0)||a.bom_spec_id-b.bom_spec_id)}
export function selectBOMSpec(item:EmployeeOrderDraftItem,target:Spec,keepTier=false):EmployeeOrderDraftItem{
 const result=resolveBOMTier(item.bom_catalog,target.bom_spec_id,item.qty,keepTier&&(!target.is_default||item.bom_tier_origin==='manual')?item.selected_price_row_key:'', 'manual')
 const tier=result.tier
 return {...item,bom_current_rule_revision:'',bom_current_rule_context:'',bom_tier_origin:result.mode==='auto'?'auto':'manual',bom_spec_id:target.bom_spec_id,bom_variant_id:target.bom_variant_id,spec_label:target.spec_name,sales_unit:target.inventory_unit,spec_g:0,unit_bag_count:0,unit_bean_g:0,migration_state:'cutover',unit_price:0,price_override:false,price_source_json:'',price_selection_mode:'bom_tier',selected_price_row_key:tier?.price_row_key||'',spec_conversion:tier?{pricing_method:'bom_rule',group_id:0,revision:0,source_bom_spec_id:tier.bom_spec_id,source_bom_variant_id:tier.bom_variant_id,reprice:true}:undefined,validation_error:'请选择价格档位'}
}

// Published quantity metadata is authoritative; display labels never establish a range.
export function resolveBOMTier(catalog:SpecConversionCatalog|undefined, specID:number|undefined, quantity:number, selectedKey = '', origin = 'manual') {
  const tiers = catalog?.source_tiers || []
  const manual = origin === 'manual' ? tiers.find(t => t.price_row_key === selectedKey) : undefined
  const spec = catalog?.specs?.find(s => Number(s.bom_spec_id) === Number(specID))
  if (!spec?.is_default) return { mode:'manual', tier:manual, reason:'' }
  const source = tiers.filter(t => Number(t.bom_spec_id) === Number(specID))
  const fallback = (reason:string) => ({ mode:'fallback', tier:manual, reason })
  if (!source.length) return fallback('默认规格未直接定价，请手动选择来源档位')
  if (!Number.isSafeInteger(Number(quantity)) || Number(quantity) <= 0) return fallback('请填写有效数量后匹配档位')
  const valid = source.filter(t => t.quantity_basis === 'sales_spec_count' && typeof t.min_qty === 'number' && Number.isSafeInteger(t.min_qty) && t.min_qty >= 0 &&
    (t.max_qty === null || (typeof t.max_qty === 'number' && Number.isSafeInteger(t.max_qty) && t.max_qty >= t.min_qty)))
  if (valid.length !== source.length) return fallback('档位缺少有效数量区间或计价口径，请手动选择')
  const matching = valid.filter(t => Number(quantity) >= t.min_qty! && (t.max_qty === null || Number(quantity) <= t.max_qty!))
  if (matching.length !== 1) return fallback(matching.length ? '当前数量匹配多个档位，请手动选择' : '当前数量没有匹配档位，请手动选择')
  return { mode:'auto', tier:matching[0], reason:'' }
}
