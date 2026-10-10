export const beanCenterPath = '/pages/bean-list-center/bean-list-center'
export function beanCards<T extends {name:string;version:string}>(rows:T[],q:string):T[]{const term=q.trim().toLowerCase();return rows.filter(row=>!term||`${row.name} ${row.version}`.toLowerCase().includes(term))}
export type CustomerTabKey='home'|'beans'|'orders'|'billing'|'mine'
export function customerMainTabs(customer:boolean):Array<{key:CustomerTabKey;label:string;url:string}>{
 const tabs:Array<{key:CustomerTabKey;label:string;url:string}>=[{key:'home',label:'首页',url:'/pages/index/index'},{key:'beans',label:'豆单',url:beanCenterPath}]
 if(customer)tabs.push({key:'orders',label:'我的订单',url:'/pages/service/service?key=orders&source=official'},{key:'billing',label:'费用',url:'/pages/service/service?key=settlement'})
 tabs.push({key:'mine',label:'我的',url:'/pages/profile/profile'});return tabs
}
