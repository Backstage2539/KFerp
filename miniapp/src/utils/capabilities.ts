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
  { key: 'mall', label: '商城下单', capability: 'mall', url: '/pages/mall/mall' },
  { key: 'beanList', label: '我的豆单', capability: 'bean_list', url: '/pages/service/service?key=beanList' },
  { key: 'productOrder', label: '现货下单', capability: 'product_order', url: '/pages/service/service?key=productOrder' },
  { key: 'directShip', label: '一件代发', capability: 'direct_ship', url: '/pages/service/service?key=directShip' },
  { key: 'processing', label: '代加工', capability: 'processing', url: '/pages/service/service?key=processing' },
  { key: 'inventory', label: '我的库存', capability: 'inventory_custody', url: '/pages/service/service?key=inventory' },
]

export function visibleHomeEntries(capabilities: Capability[] = []): HomeEntry[] {
  const enabled = new Set(capabilities.filter((item) => item.enabled).map((item) => item.code))
  return entries.filter((entry) => entryCapabilities(entry).some((code) => enabled.has(code)))
}

function entryCapabilities(entry: HomeEntry): string[] {
  if (entry.capabilities?.length) return entry.capabilities
  return entry.capability ? [entry.capability] : []
}
