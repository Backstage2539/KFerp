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

export function customerEntryRoute(context: { miniapp_entry_mode?: string; capabilities?: Capability[] }): string {
  const canOpenMall = context.miniapp_entry_mode === 'mall' && (context.capabilities || []).some((item) => item.code === 'mall' && item.enabled)
  return canOpenMall ? '/pages/mall/mall' : '/pages/home/home'
}
