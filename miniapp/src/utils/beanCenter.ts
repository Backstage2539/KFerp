import type { Capability } from './capabilities'
import { customerDestinations } from './customerNavigation'

export const beanCenterPath = customerDestinations.beans.url
export function beanCards<T extends {name:string;version:string}>(rows:T[],q:string):T[]{const term=q.trim().toLowerCase();return rows.filter(row=>!term||`${row.name} ${row.version}`.toLowerCase().includes(term))}
export type CustomerTabKey='home'|'beans'|'orders'|'billing'|'mine'
export function customerMainTabs(customer:boolean, capabilities:Capability[]=[], accountType=''):Array<{key:CustomerTabKey;label:string;url:string}>{
 if(accountType==='employee')return []
 const tabs:Array<{key:CustomerTabKey;label:string;url:string}>=[{key:'home',...customerDestinations.home},{key:'beans',...customerDestinations.beans}]
 if(customer){
  tabs.push({key:'orders',...customerDestinations.orders})
  if(capabilities.some(item=>item.code==='settlement'&&item.enabled))tabs.push({key:'billing',...customerDestinations.billing})
 }
 tabs.push({key:'mine',...customerDestinations.mine});return tabs
}
