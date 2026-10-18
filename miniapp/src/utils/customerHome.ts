import type { CustomerOrderSummary } from '../api/customerPortal'

export function customerOrderNeedsHandling(order: Pick<CustomerOrderSummary, 'delivery_mode' | 'delivery_status' | 'ship_status'>): boolean {
  if (order.delivery_mode === 'customer_stock') return order.delivery_status !== 'received'
  return !['已发货', '已出库', '已签收', '已收货', '已完成'].includes(order.ship_status)
}
