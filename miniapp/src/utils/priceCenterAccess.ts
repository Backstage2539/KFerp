export function canUseEmployeePriceCenter(session: { accountType: string; roles: string[] }): boolean {
  return session.accountType === 'employee' && session.roles.some(role => role === 'sales' || role === 'admin')
}
export function hasAuthenticatedCustomerSession(session: { token?: string; miniUserID?: number; accountType: string; bindings?: Array<{ status: string }> }): boolean {
  return Boolean(session.token && Number(session.miniUserID) > 0 && ['', 'customer', 'channel_customer'].includes(session.accountType) && session.bindings?.some(binding => binding.status === 'approved'))
}
export function pageEntryAccessAction(cause: unknown): 'login' | 'choose_customer' | 'denied' | 'retry' {
  const error = cause as { statusCode?: number; code?: string } | null
  if (error?.statusCode === 401) return 'login'
  if (error?.statusCode === 403) return error.code === 'mini_customer_selection_required' ? 'choose_customer' : 'denied'
  return 'retry'
}
export function canOpenPageEntry(session: {accountType:string;roles:string[]}, page: {document:{kind:string;bean_center?:boolean};sample_enabled?:boolean}): boolean {
 const centerPrice=page.document.kind==='price' && (page.sample_enabled===true || page.document.bean_center===true)
 return !centerPrice || session.accountType !== 'employee' || canUseEmployeePriceCenter(session)
}
