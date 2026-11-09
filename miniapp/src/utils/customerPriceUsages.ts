export type CustomerPriceUsage = 'product_order' | 'direct_ship'

export function customerPriceUsages(capabilities: string[], modes: string[]): CustomerPriceUsage[] {
  const result: CustomerPriceUsage[] = []
  if (capabilities.includes('product_order') || (capabilities.includes('direct_ship') && modes.includes('stock'))) result.push('product_order')
  if (capabilities.includes('direct_ship') && modes.includes('on_demand')) result.push('direct_ship')
  return result
}
