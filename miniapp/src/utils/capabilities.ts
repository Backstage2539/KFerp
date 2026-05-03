export type Capability = {
  code: string
  enabled: boolean
}

export type HomeEntry = {
  key: string
  label: string
  capability: string
  url: string
}

const entries: HomeEntry[] = [
  { key: 'beanList', label: '我的豆单', capability: 'bean_list', url: '/pages/service/service?key=beanList' },
  { key: 'productOrder', label: '现货下单', capability: 'product_order', url: '/pages/service/service?key=productOrder' },
  { key: 'directShip', label: '一件代发', capability: 'direct_ship', url: '/pages/service/service?key=directShip' },
  { key: 'processing', label: '代加工', capability: 'processing', url: '/pages/service/service?key=processing' },
  { key: 'inventory', label: '我的库存', capability: 'inventory_custody', url: '/pages/service/service?key=inventory' },
  { key: 'shipping', label: '物流查询', capability: 'shipping_query', url: '/pages/service/service?key=shipping' },
  { key: 'settlement', label: '结算中心', capability: 'settlement', url: '/pages/service/service?key=settlement' },
]

export function visibleHomeEntries(capabilities: Capability[] = []): HomeEntry[] {
  const enabled = new Set(capabilities.filter((item) => item.enabled).map((item) => item.code))
  return entries.filter((entry) => enabled.has(entry.capability))
}
