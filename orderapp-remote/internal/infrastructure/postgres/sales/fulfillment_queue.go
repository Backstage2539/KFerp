package sales

import "fmt"

// These predicates are shared by the full server counts and the paginated
// destination list. They never depend on the number of rows loaded in a UI.
func FulfillmentOrderPredicate() string {
	return "(o.portal_service_code IN ('direct_ship','processing_ship','product_order') OR COALESCE(to_jsonb(o)->>'delivery_mode','')='customer_stock')"
}
func FulfillmentPendingAmount(schema string) string {
	return fmt.Sprintf(`CASE WHEN EXISTS(SELECT 1 FROM %s.pay_statuses fp WHERE fp.id=o.pay_status_id AND fp.name ~ '(已付款|已收款|已支付)') THEN 0 ELSE greatest(0,o.grand_total-COALESCE((to_jsonb(o)->>'prepayment_amount')::numeric,0)) END`, schema)
}
func FulfillmentQueuePredicate(schema, queue string) string {
	pending := fmt.Sprintf(`(NOT COALESCE(o.is_void,false) AND COALESCE(to_jsonb(o)->>'delivery_mode','ship')<>'customer_stock' AND COALESCE(to_jsonb(o)->>'confirmation_status','accepted')='accepted' AND NOT EXISTS(SELECT 1 FROM %s.ship_statuses fs WHERE fs.id=o.ship_status_id AND fs.name IN ('已发货','已出库','已签收','已收货','已完成')))`, schema)
	switch queue {
	case "dispatch":
		return "(COALESCE(to_jsonb(o)->>'delivery_mode','ship')<>'customer_stock' AND (o.portal_service_code IN ('direct_ship','processing_ship') OR COALESCE(to_jsonb(o)->>'fulfillment_mode','')<>''))"
	case "pending_shipment":
		return pending
	case "missing_recipient":
		return pending + " AND (btrim(coalesce(o.receiver_name,''))='' OR btrim(coalesce(o.receiver_phone,''))='' OR btrim(coalesce(o.receiver_address,''))='')"
	case "pending_payment":
		return "(" + FulfillmentPendingAmount(schema) + ")>0"
	case "pending_confirmation":
		return "COALESCE(to_jsonb(o)->>'confirmation_status','accepted')<>'accepted'"
	case "stock_waiting":
		return pending + fmt.Sprintf(` AND (EXISTS(SELECT 1 FROM %[1]s.order_process_statuses fp WHERE fp.id=o.process_status_id AND fp.name IN ('待生产','生产中','待排产','待计划','已排产','未生产')) OR EXISTS(SELECT 1 FROM %[1]s.customer_direct_ship_request_orders ro JOIN %[1]s.customer_direct_ship_requests rq ON rq.id=ro.request_id AND rq.status NOT IN ('cancelled','shipped') JOIN %[1]s.order_items oi ON oi.order_id=ro.order_id WHERE ro.order_id=o.id AND oi.qty>COALESCE((SELECT sum(a.allocated_qty) FROM %[1]s.customer_direct_ship_request_allocations a WHERE a.order_item_id=oi.id AND a.status='reserved'),0)))`, schema)
	case "":
		return "true"
	default:
		return "false"
	}
}
