export type FulfillmentProgress = {label: string; production?: string; inbound?: string; stock?: string; can_ship?: boolean; block_reason?: string}
type OrderLike = {fulfillment_progress?: FulfillmentProgress; delivery_mode?: string; delivery_status?: string; process_status?: string; ship_status?: string}
export function orderProgressLabel(order: OrderLike = {}): string {
 if (order.fulfillment_progress?.label) return order.fulfillment_progress.label
 if (order.delivery_mode === 'customer_stock') return [order.process_status || '待生产', ({received:'已入库',partially_received:'部分入库',pending_receipt:'待入库'} as Record<string,string>)[order.delivery_status || ''] || '待入库'].join(' · ')
 return [order.process_status, order.ship_status].filter(Boolean).join(' · ') || '待处理'
}

export function directShipProgressLabel(request: {status?: string; packages?: OrderLike[]}): string {
 if (['shipped','delivered','cancelled','partially_shipped'].includes(request.status || '')) return ({shipped:'已发货',delivered:'已签收',cancelled:'已取消',partially_shipped:'部分发货'} as Record<string,string>)[request.status!]
 const labels=(request.packages||[]).map(p=>p.fulfillment_progress?.label).filter(Boolean)
 if (labels.length && labels.length===request.packages?.length && new Set(labels).size===1) return labels[0]!
 return ({reserved:'待发货',pending:'待处理',shipped:'已发货',partially_shipped:'部分发货',delivered:'已签收',cancelled:'已取消'} as Record<string,string>)[request.status||''] || request.status || '待处理'
}
