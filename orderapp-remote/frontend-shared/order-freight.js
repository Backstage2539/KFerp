export const EMPTY_DESTINATION = {province:'',city:'',district:'',province_code:'',city_code:'',district_code:''}
export function emptyFreightState(){return {manual:'',free:null,chargeMode:'auto_current',templateId:0,templateOverride:false,parcelWeightG:'',autoAmountCents:null,preserveExisting:false,preservedAmount:null,destination:{...EMPTY_DESTINATION},address:'',source:'auto'}}
export function manualFreightCents(value){
 const text=String(value??'').trim();if(!text)return null
 if(!/^\d+(\.\d{1,2})?$/.test(text))throw new Error('运费需为非负金额，最多两位小数')
 const [whole,fraction='']=text.split('.');const cents=Number(whole)*100+Number(fraction.padEnd(2,'0'))
 if(!Number.isSafeInteger(cents)||cents>999999999999)throw new Error('运费超出可处理范围')
 return cents
}
export function freightAmount(state){if(state?.preserveExisting)return Number(state?.preservedAmount||0);if(state?.free)return 0;const manual=manualFreightCents(state?.manual);if(manual!==null)return manual/100;return state?.autoAmountCents==null?0:Number(state.autoAmountCents)/100}
export function freightOverride(state){if(state?.preserveExisting)return {};let cents;try{cents=manualFreightCents(state?.manual)}catch(e){if(!state?.free)throw e;cents=null}const parcel=String(state?.parcelWeightG??'').trim();if(parcel&&(!/^\d+$/.test(parcel)||Number(parcel)<=0||Number(parcel)>1e9))throw new Error('本单包裹总重量需为1至1000000000克');const mode=cents!==null?'manual_current':state?.free?'auto_current':state?.chargeMode==='deferred'?'':(state?.chargeMode||'auto_current');return {...(mode?{charge_mode:mode}:{}),...(cents===null?{}:{manual_amount_cents:cents}),...(parcel&&cents===null?{parcel_weight_g:Number(parcel)}:{}),...(state?.templateOverride&&Number(state.templateId)>0?{template_id:Number(state.templateId)}:{}),...(state?.free==null?(cents===null?{}:{free_shipping:false}):{free_shipping:state.free})}}
export function freightStateFromOrder(order={}){
 const state=emptyFreightState(),f=order.freight
 state.address=order.receiver_address||''
 if(f){state.destination={...EMPTY_DESTINATION,...f.destination};state.manual=f.manual_amount_cents!=null?(f.manual_amount_cents/100).toFixed(2):'';state.free=!!f.policy?.free_shipping;state.chargeMode=f.charge_mode||'deferred';state.templateId=Number(f.template_id||f.policy?.template_id||0);state.parcelWeightG=f.parcel_weight_g?String(f.parcel_weight_g):'';state.autoAmountCents=f.charge_mode==='auto_current'?Number(f.current_receivable_cents||0):null}
 else{state.manual=(Number(order.shipping_amount)||0).toFixed(2);state.free=false}
 if(Number(order.id||order.edit_id||0)>0){state.preserveExisting=true;state.preservedAmount=Number(order.shipping_amount||0)}
 state.source='saved';return state
}
export function missingWeightItemNames(items=[],quote=null){
 if(!quote||quote.estimate_known)return[]
 return (quote.item_weights||[]).filter(x=>x.logistics_unit_weight_g==null).map(x=>{
  const row=items.find(i=>Number(i.product_id)===Number(x.product_id)&&(!x.bom_spec_id||Number(i.bom_spec_id||0)===Number(x.bom_spec_id)))
  return row?.name||`商品${x.product_id}${x.bom_spec_id?`（规格${x.bom_spec_id}）`:''}`
 })
}
export function searchOrderRegions(rows,query){const terms=String(query||'').trim().toLowerCase().split(/\s+/).filter(Boolean);return rows.filter(r=>terms.every(t=>[r.name,r.alias,r.path,r.pinyin,r.initials,r.code].join(' ').toLowerCase().includes(t)))}
export function regionDestination(rows,code){const map=new Map(rows.map(r=>[r.code,r]));let r=map.get(code);const d={...EMPTY_DESTINATION};while(r){if(r.level===3){d.district=r.name;d.district_code=r.code}else if(r.level===2){d.city=r.name;d.city_code=r.code}else{d.province=r.name;d.province_code=r.code}r=map.get(r.parent_code)}return d}
