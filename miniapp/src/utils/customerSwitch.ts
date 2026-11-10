import type { CustomerBinding } from '../api/customerPortal'
import type { Capability } from './capabilities'

export function shouldShowCustomerSwitcher(bindings: CustomerBinding[] = []): boolean {
  return bindings.length > 1
}

export function customerPickerLabels(bindings: CustomerBinding[] = [], currentCustomerID = 0): string[] {
  return bindings.map((binding) => {
    const label = binding.customer_name || `客户 ${binding.customer_id}`
    return binding.customer_id === currentCustomerID ? `${label}（当前）` : label
  })
}

export function customerPickerIndex(bindings: CustomerBinding[] = [], currentCustomerID = 0): number {
  const index = bindings.findIndex((binding) => binding.customer_id === currentCustomerID)
  return index >= 0 ? index : 0
}

export function selectedCustomerID(bindings: CustomerBinding[] = [], index: number): number {
  return bindings[index]?.customer_id || 0
}

export function customerEntryRoute(context: { miniapp_entry_mode?: string; capabilities?: Capability[]; registration_complete?: boolean; current_customer_id?:number }): string {
  if ((context as { account_type?: string }).account_type === 'employee') return '/pages/home/home'
  if(context.registration_complete && !context.current_customer_id)return '/pages/index/index'
 return '/pages/home/home'
}

// Phone authorization identifies an account before returning to visitor browsing.
export function phoneVerifiedDestination(context: { account_type?: string; current_customer_id?: number; bindings?: Array<{ customer_id: number; status?: string }> }, visitorReturn = ''): string {
  if (context.account_type === 'employee') return '/pages/home/home'
  const approved = (context.bindings || []).filter(binding => binding.status === 'approved')
  if (!approved.length) return visitorReturn || '/pages/index/index'
  if (context.current_customer_id && approved.some(binding => binding.customer_id === context.current_customer_id)) return '/pages/home/home'
  return '/pages/customer-select/customer-select?return_to=%2Fpages%2Fhome%2Fhome'
}
