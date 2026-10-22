export const customerDestinations = {
  home: { label: '首页', url: '/pages/index/index' },
  beans: { label: '价格表', url: '/pages/bean-list-center/bean-list-center' },
  orders: { label: '我的订单', url: '/pages/service/service?key=orders&source=official' },
  billing: { label: '费用', url: '/pages/service/service?key=settlement' },
  mine: { label: '我的', url: '/pages/profile/profile' },
  mall: { label: '商城下单', url: '/pages/mall/mall' },
  productOrder: { label: '商品下单', url: '/pages/service/service?key=productOrder' },
  directShip: { label: '一件代发', url: '/pages/service/service?key=directShip' },
  processing: { label: '生产工单', url: '/pages/service/service?key=processing' },
  inventory: { label: '我的库存', url: '/pages/service/service?key=inventory' },
  processingShipments: { label: '代加工发货', url: '/pages/service/service?key=orders' },
} as const

type DestinationOptions = { q?: string; fulfillment_mode?: 'stock' | 'on_demand'; stock_purchase?: boolean }

export function customerDestinationURL(key: keyof typeof customerDestinations, options: DestinationOptions = {}): string {
  let url: string = customerDestinations[key].url
  if (key === 'directShip' && options.fulfillment_mode) url += `&fulfillment_mode=${options.fulfillment_mode}`
  if (key === 'productOrder' && options.stock_purchase) url += '&stock_purchase=1'
  if ((key === 'orders' || key === 'processingShipments') && options.q) url += `&q=${encodeURIComponent(options.q)}`
  return url
}
