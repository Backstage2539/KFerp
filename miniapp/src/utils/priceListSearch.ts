import type { BeanListSummary, BeanListProductSummary, BeanListGroupSummary } from '../api/customerPortal'
import { beanListQualityLines } from './beanListDisplay'
export type PriceListSearch = { query: string; attributes: Record<string,string[]>; badges: string[] }
export type PriceListFacet = { key:string; label:string; values:string[] }
export function emptyPriceListSearch():PriceListSearch { return {query:'',attributes:{},badges:[]} }
const normalize=(value:unknown)=>String(value??'').normalize('NFKC').toLowerCase().trim()
export function priceListBadgeCategory(item:BeanListProductSummary):string {
 const badge=normalize(item.badge), label=normalize(item.badge_label)
 if(badge==='new'||label==='上新'||label==='new'||label==='new 上新')return 'new'
 if(['thumb','medal'].includes(badge)||label==='推荐')return 'recommended'
 return badge||label ? 'other' : 'none'
}
export function priceListFacets(list:BeanListSummary):PriceListFacet[] {
 const facets=new Map<string,PriceListFacet>()
 for(const group of list.groups||[]) for(const item of group.items) for(const attr of item.product_attributes||[]) {
  const key=attr.key.trim(), value=attr.value.trim()
  if(!key||!value)continue
  let facet=facets.get(key)
  if(!facet){facet={key,label:attr.label||key,values:[]};facets.set(key,facet)}
  if(!facet.values.includes(value))facet.values.push(value)
 }
 return [...facets.values()]
}
export function filterPriceList(list:BeanListSummary,state:PriceListSearch):{groups:BeanListGroupSummary[];count:number;total:number} {
 const terms=normalize(state.query).split(/\s+/).filter(Boolean)
 const header=[list.title,list.subtitle,list.brand_intro,list.brand_name||'棵凡咖啡',list.show_version!==false?`${list.version_no} ${list.published_at}`:'',list.show_changelog!==false?list.changelog:''].filter(Boolean).join(' ')
 let count=0,total=0
 const groups=(list.groups||[]).flatMap(group=>{
  total+=group.items.length
  const category=list.show_category_numbers!==false&&group.show_category!==false?group.category:''
  const items=group.items.filter(item=>{
   if(state.badges.length&&!state.badges.includes(priceListBadgeCategory(item)))return false
   for(const [key,values] of Object.entries(state.attributes)) {
    if(values.length&&!(item.product_attributes||[]).some(attr=>attr.key===key&&values.includes(attr.value.trim())))return false
   }
   const visible=[header,category,item.name,item.code,item.badge_label,item.badge_label?'':({new:'上新',recommended:'推荐'}[priceListBadgeCategory(item)]||''),item.recommended_use,item.flavor,item.description,...(item.attribute_lines||[]),...(item.product_attributes||[]).flatMap(a=>[a.label,a.value]),...(item.prices||[]).flatMap(p=>[p.label,p.value]),...beanListQualityLines(item).flatMap(q=>[q.label,q.value])].filter(Boolean).join(' ')
   const text=normalize(visible)
   return terms.every(term=>text.includes(term))
  })
  count+=items.length
  return items.length?[{...group,items}]:[]
 })
 return {groups,count,total}
}
export function reconcilePriceListSearch(list:BeanListSummary,state:PriceListSearch):PriceListSearch {
 const attributes:Record<string,string[]>={}
 for(const facet of priceListFacets(list)) {
  const values=(state.attributes[facet.key]||[]).filter(v=>facet.values.includes(v))
  if(values.length)attributes[facet.key]=[...new Set(values)]
 }
 return {query:state.query,attributes,badges:[...new Set(state.badges.filter(b=>['new','recommended','none','other'].includes(b)))]}
}
export function priceListSearchScope(environment:string,userID:number,customerID:number,target:string):string {return JSON.stringify([environment,userID,customerID,target])}
function clone(state:PriceListSearch):PriceListSearch{return {query:state.query,attributes:Object.fromEntries(Object.entries(state.attributes).map(([key,value])=>[key,[...value]])),badges:[...state.badges]}}
export class PriceListSearchMemory {
 private states=new Map<string,PriceListSearch>()
 save(scope:string,state:PriceListSearch){this.states.delete(scope);this.states.set(scope,clone(state));if(this.states.size>40)this.states.delete(this.states.keys().next().value as string)}
 load(scope:string){return clone(this.states.get(scope)||emptyPriceListSearch())}
}
export const priceListSearchMemory=new PriceListSearchMemory()
