import type { CustomerFulfillmentSettings } from '../api/customerPortal'
import { customerDestinations, customerDestinationURL } from './customerNavigation'
export type Capability = {
  code: string
  enabled: boolean
}

export type HomeEntry = {
  key: string
  label: string
  capability?: string
  capabilities?: string[]
  url: string
}

const entries: HomeEntry[] = [
  { key: 'mall', ...customerDestinations.mall, capability: 'mall' },
  { key: 'productOrder', ...customerDestinations.productOrder, capability: 'product_order' },
  { key: 'directShip', ...customerDestinations.directShip, capability: 'direct_ship' },
  { key: 'processing', ...customerDestinations.processing, capability: 'processing' },
  { key: 'inventory', ...customerDestinations.inventory, capabilities: ['inventory_custody', 'processing'] },
]

export function visibleHomeEntries(capabilities: Capability[] = [], settings?: CustomerFulfillmentSettings): HomeEntry[] {
  const enabled = new Set(capabilities.filter((item) => item.enabled).map((item) => item.code))
  const dropship = enabled.has('direct_ship') && settings?.modes.some(mode => mode.enabled && ['stock','on_demand'].includes(mode.mode))
  return entries.filter((entry) => !(dropship && entry.key==='productOrder') && entryCapabilities(entry).some((code) => enabled.has(code))).flatMap(entry => {
    if (entry.key !== 'directShip' || !settings) return [entry]
    return settings.modes.filter(mode => mode.enabled && ['stock', 'on_demand'].includes(mode.mode)).map(mode => ({ ...entry, key: mode.mode === 'stock' ? 'directShip' : 'onDemandShip', label: mode.mode === 'stock' ? customerDestinations.directShip.label : '无库存代发', url: customerDestinationURL('directShip', { fulfillment_mode: mode.mode }) }))
  })
}

function entryCapabilities(entry: HomeEntry): string[] {
  if (entry.capabilities?.length) return entry.capabilities
  return entry.capability ? [entry.capability] : []
}
