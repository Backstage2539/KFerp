package inventory

const ShipmentStockDeductionSource = "sales_order_shipment"

// TraceableShipmentAggregateSyncSource marks shipment deductions that update
// both the traceable batch and the finished_inventory compatibility aggregate.
// Older deduction rows use sales_order_shipment and therefore identify the
// historical batch-only behavior during compatibility reads.
const TraceableShipmentAggregateSyncSource = "sales_order_shipment_inventory_sync_v2"
