export function canUseEmployeePriceCenter(session: { accountType: string; roles: string[] }): boolean {
  return session.accountType === 'employee' && session.roles.some(role => role === 'sales' || role === 'admin')
}
export function canOpenPageEntry(session: {accountType:string;roles:string[]}, page: {document:{kind:string;bean_center?:boolean};sample_enabled?:boolean}): boolean {
 const centerPrice=page.document.kind==='price' && (page.sample_enabled===true || page.document.bean_center===true)
 return !centerPrice || session.accountType !== 'employee' || canUseEmployeePriceCenter(session)
}
