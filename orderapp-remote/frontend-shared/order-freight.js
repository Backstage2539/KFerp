export const EMPTY_DESTINATION = {province:'',city:'',district:'',province_code:'',city_code:'',district_code:''}
export function emptyFreightState(){return {manual:'',free:null,destination:{...EMPTY_DESTINATION},address:'',source:'auto'}}
export function manualFreightCents(value){
 const text=String(value??'').trim();if(!text)return null
 if(!/^\d+(\.\d{1,2})?$/.test(text))throw new Error('运费需为非负金额，最多两位小数')
 const [whole,fraction='']=text.split('.');const cents=Number(whole)*100+Number(fraction.padEnd(2,'0'))
 if(!Number.isSafeInteger(cents)||cents>999999999999)throw new Error('运费超出可处理范围')
 return cents
}
export function freightAmount(state){return state?.free?0:(manualFreightCents(state?.manual)??0)/100}
export function freightOverride(state){let cents;try{cents=manualFreightCents(state?.manual)}catch(e){if(!state?.free)throw e;cents=null}return {...(cents===null?{}:{manual_amount_cents:cents}),...(state?.free==null?(cents===null?{}:{free_shipping:false}):{free_shipping:state.free})}}
export function freightStateFromOrder(order={}){
 const state=emptyFreightState(),f=order.freight
 state.address=order.receiver_address||''
 if(f){state.destination={...EMPTY_DESTINATION,...f.destination};state.manual=f.manual_amount_cents!=null?(f.manual_amount_cents/100).toFixed(2):'';state.free=!!f.policy?.free_shipping}
 else{state.manual=(Number(order.shipping_amount)||0).toFixed(2);state.free=false}
 state.source='saved';return state
}
export function searchOrderRegions(rows,query){const terms=String(query||'').trim().toLowerCase().split(/\s+/).filter(Boolean);return rows.filter(r=>terms.every(t=>[r.name,r.alias,r.path,r.pinyin,r.initials,r.code].join(' ').toLowerCase().includes(t)))}
export function regionDestination(rows,code){const map=new Map(rows.map(r=>[r.code,r]));let r=map.get(code);const d={...EMPTY_DESTINATION};while(r){if(r.level===3){d.district=r.name;d.district_code=r.code}else if(r.level===2){d.city=r.name;d.city_code=r.code}else{d.province=r.name;d.province_code=r.code}r=map.get(r.parent_code)}return d}
