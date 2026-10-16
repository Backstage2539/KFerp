import type { EmployeeOrderDraftItem, SpecConversionCatalog } from '../api/customerPortal'
type Spec = SpecConversionCatalog['specs'][number]
export function orderedBOMSpecs(specs:Spec[]=[]):Spec[]{return [...specs].sort((a,b)=>Number(Boolean(b.is_default))-Number(Boolean(a.is_default))||Number(a.sort_order||0)-Number(b.sort_order||0)||a.bom_spec_id-b.bom_spec_id)}
export function selectBOMSpec(item:EmployeeOrderDraftItem,target:Spec,keepTier=true):EmployeeOrderDraftItem{
 const tier=keepTier?item.bom_catalog?.source_tiers?.find(t=>t.price_row_key===item.selected_price_row_key):undefined
 return {...item,bom_spec_id:target.bom_spec_id,bom_variant_id:target.bom_variant_id,spec_label:target.spec_name,sales_unit:target.inventory_unit,spec_g:0,unit_bag_count:0,unit_bean_g:0,migration_state:'cutover',unit_price:0,price_override:false,price_source_json:'',price_selection_mode:'bom_tier',selected_price_row_key:tier?.price_row_key||'',spec_conversion:tier?{pricing_method:'bom_rule',group_id:0,revision:0,source_bom_spec_id:tier.bom_spec_id,source_bom_variant_id:tier.bom_variant_id,reprice:true}:undefined,validation_error:'请选择价格档位'}
}
