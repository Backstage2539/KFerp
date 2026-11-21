const legacyChoices: [string, string][] = [['created','下单成功'],['paid','付款成功'],['production_started','开始生产'],['production_finished','生产完成'],['shipped','发货进度'],['cancelled','订单取消']]
export const defaultOrderNotificationEvents = legacyChoices.map(([key]) => key)
export const orderNotificationChoices: [string, string][] = [
  ['order_accepted', '订单已接单'],
  ['shipment_created', '订单发货'],
  ['receipt_received', '货款到账'],
  ['bill_issued', '对账单生成'],
]

export function visibleOrderNotificationChoices(identity: { accountType: string; capabilities: Array<{ code: string; enabled: boolean }> }): [string, string][] {
  if (identity.accountType === 'employee') return orderNotificationChoices.slice(0, 2)
  const enabled = new Set((identity.capabilities || []).filter(item => item.enabled).map(item => item.code))
  const canViewOrders = ['product_order', 'direct_ship', 'shipping_query', 'mall'].some(code => enabled.has(code))
  return orderNotificationChoices.filter(([key]) => key === 'order_accepted' || key === 'shipment_created' ? canViewOrders : enabled.has('settlement'))
}

export function toggleOrderNotificationChoice(events: string[], key: string): string[] {
  return events.includes(key) ? events.filter(item => item !== key) : [...events, key]
}
