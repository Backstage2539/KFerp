export const fulfillmentModuleTabs = () => [
 { key: 'overview', label: '概览', description: '先处理客户待办，再进入业务' },
 { key: 'inventory', label: '库存', description: '客户成品、原料与包材' },
 { key: 'orders', label: '订单', description: '同一客户的订单与履约记录' },
 { key: 'dispatch', label: '代发', description: '备货、收件信息与发货进度' },
 { key: 'fees', label: '费用', description: '核对费用、运费与结算进度' },
]
export function fulfillmentStockQuantities(row) {
 return { physical: Math.max(0, Number(row.total_qty || 0) - Number(row.pending_inbound_qty || 0)), reserved: Math.max(0, Number(row.occupied_qty || 0) - Number(row.pending_reserved_qty || 0)), shippable: Number(row.physical_available_qty ?? row.available_qty ?? 0), pending: Number(row.pending_inbound_qty || 0), pendingReserved: Number(row.pending_reserved_qty || 0) }
}
export function fulfillmentTodoCards(totals = {}) {
 return [
  { key: 'stock_waiting', label: '待备货', value: Number(totals.stock_waiting || 0), unit: '单', hint: '库存未齐 · 待生产或入库', tab: 'dispatch', queue: 'stock_waiting', tone: 'amber' },
  { key: 'pending_shipment', label: '待发货', value: Number(totals.pending_shipment || 0), unit: '单', hint: '已确认、尚未完成发货', tab: 'dispatch', queue: 'pending_shipment', tone: 'green' },
  { key: 'settlement', label: '待结算费用', value: Number(totals.unsettled_fee_count || 0), unit: '笔', hint: '待归入结算单的服务费用', tab: 'fees', queue: '', tone: 'slate' },
 ]
}
export function fulfillmentBusinessLabel(value) {
 return {stock:'库存代发',on_demand:'现做现发',direct_ship:'一件代发',processing_ship:'加工代发',product_order:'商品下单',customer_stock:'留厂补购',shipping:'运费',direct_ship_service:'代发服务费',storage:'仓储费',processing:'加工费',product:'商品',roasting:'烘焙费',labor:'人工费',material:'物料费',packaging:'包装费',adjustment:'调整',draft:'待确认',confirmed:'已确认',paid:'已结清',unsettled:'待结算',settled:'已归入结算',reversed:'已冲销',order:'订单',available:'可用',pending_receipt:'待入库',released:'已释放',submitted:'已提交',completed:'已完成',pending:'待处理',reserved:'已预订',cancelled:'已取消'}[value] || value || '—'
}
