import { describe, expect, it } from 'vitest'
import type { BeanListSummary } from '../api/customerPortal'
import { filterPriceList, priceListFacets, emptyPriceListSearch, reconcilePriceListSearch, priceListBadgeCategory, priceListSearchScope, PriceListSearchMemory } from './priceListSearch'
const list: BeanListSummary = {
 id: 8, cache_key:'bean-list:8:v1', list_type:'commercial', version_no:'v1', status:'published', published_at:'2026-10-14', changelog:'新版报价', title:'棵凡价格表', subtitle:'不含运费', brand_intro:'现烘现发',
 groups:[{category:'云南',show_category:true,items:[
  {name:'初晓 Sunrise',code:'A01',badge:'new',badge_label:'NEW 上新',flavor:'柑橘',description:'干净明亮',recommended_use:'手冲',product_attributes:[{key:'roast',label:'烘焙度',value:'浅烘',template_id:11},{key:'use',label:'适用出品',value:'手冲',template_id:11}],attribute_lines:['烘焙度：浅烘；适用出品：手冲'],prices:[{label:'227g / 1件起',value:'82/袋'}]},
  {name:'芒霜',code:'A02',badge:'thumb',badge_label:'推荐',product_attributes:[{key:'roast',label:'烘焙度',value:'中烘',template_id:11},{key:'use',label:'适用出品',value:'奶咖',template_id:11}],prices:[{label:'1KG',value:'120/包'}]},
  {name:'深夜',badge:'medal',badge_label:'推荐',product_attributes:[{key:'roast',label:'烘焙度',value:'深烘',template_id:11},{key:'use',label:'适用出品',value:'奶咖',template_id:11}]},
  {name:'无标签豆',attribute_lines:['历史工艺：蜜处理']}
 ]},{category:'隐去分类',show_category:false,items:[{name:'孤品',description:'独立项'}]}]
}
const names=(state=emptyPriceListSearch())=>filterPriceList(list,state).groups.flatMap(g=>g.items.map(i=>i.name))
describe('current price list search and facets',()=>{
 it('searches visible text only with case-insensitive partial AND terms',()=>{
  for(const q of ['sunr','初晓 227g','a01 柑橘','82/袋 浅烘','适用出品 手冲','干净','蜜处理']) expect(names({...emptyPriceListSearch(),query:q})).toEqual([q==='蜜处理'?'无标签豆':'初晓 Sunrise'])
  expect(names({...emptyPriceListSearch(),query:'现烘'})).toHaveLength(5)
  expect(names({...emptyPriceListSearch(),query:'隐去分类'})).toEqual([])
  expect(names({...emptyPriceListSearch(),query:'not-found'})).toEqual([])
 })
 it('does not index private identifiers or hidden properties',()=>{
  const privateList={...list,groups:[{category:'普通',items:[{name:'公开名',product_id:999123,price_source_json:{password:'秘密内部'},highlight_terms:['未展示']}]}]}
  for(const query of ['999123','秘密内部','未展示']) expect(filterPriceList(privateList as BeanListSummary,{...emptyPriceListSearch(),query}).count).toBe(0)
 })
 it('generates dynamic facets from structured frozen attributes without parsing legacy lines',()=>{
  expect(priceListFacets(list).map(f=>({key:f.key,label:f.label,values:f.values}))).toEqual([
   {key:'roast',label:'烘焙度',values:['浅烘','中烘','深烘']},{key:'use',label:'适用出品',values:['手冲','奶咖']}
  ])
 })
 it('combines OR within each facet and AND across fields and tags',()=>{
  expect(names({query:'',attributes:{roast:['浅烘','中烘'],use:['奶咖']},badges:['recommended']})).toEqual(['芒霜'])
  expect(names({query:'sunrise',attributes:{roast:['浅烘']},badges:['new']})).toEqual(['初晓 Sunrise'])
  const result=filterPriceList(list,{query:'孤品',attributes:{},badges:[]})
  expect(result.groups.map(g=>g.category)).toEqual(['隐去分类']);expect(result.count).toBe(1)
 })
 it('merges recommendation styles and recognizes no badge',()=>{
  expect(priceListBadgeCategory({name:'x',badge:'thumb'})).toBe('recommended')
  expect(priceListBadgeCategory({name:'x',badge:'medal'})).toBe('recommended')
  expect(names({query:'',attributes:{},badges:['recommended']})).toEqual(['芒霜','深夜'])
  expect(names({query:'',attributes:{},badges:['none']})).toEqual(['无标签豆','孤品'])
  expect(names({query:'',attributes:{},badges:['new','recommended']})).toEqual(['初晓 Sunrise','芒霜','深夜'])
 })
 it('clears invalid facet values on a changed version but keeps valid selections',()=>{
  expect(reconcilePriceListSearch(list,{query:'  sunrise ',attributes:{roast:['浅烘','不存在'],deleted:['旧值']},badges:['recommended']})).toEqual({query:'  sunrise ',attributes:{roast:['浅烘']},badges:['recommended']})
  expect(names(emptyPriceListSearch())).toHaveLength(5)
 })
 it('keeps same-table state in memory and isolates accounts, environment and target entries',()=>{
  const memory=new PriceListSearchMemory();const scope=priceListSearchScope('production',4,0,'entry-a')
  const state={query:'coffee',attributes:{roast:['浅烘']},badges:['new'] as const}
  memory.save(scope,{...state,badges:['new']})
  expect(memory.load(scope).query).toBe('coffee')
  for(const other of [priceListSearchScope('development',4,0,'entry-a'),priceListSearchScope('production',5,0,'entry-a'),priceListSearchScope('production',4,1,'entry-a'),priceListSearchScope('production',4,0,'entry-b')]) expect(memory.load(other)).toEqual(emptyPriceListSearch())
  const loaded=memory.load(scope);loaded.attributes.roast.push('中烘');expect(memory.load(scope).attributes.roast).toEqual(['浅烘'])
 })
})
