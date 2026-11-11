package sales

import (
	"context"
	"fmt"
	"strings"

	salesapp "orderapp/internal/application/sales"
	messagepg "orderapp/internal/infrastructure/postgres/messagecenter"

	"github.com/jackc/pgx/v5"
)

func firstAcceptanceNotice(decision string, previous int64) bool {
	return decision == "accepted" && previous == 0
}

func savedShipmentNotice(before, after string) bool {
	before, after = strings.TrimSpace(before), strings.TrimSpace(after)
	if after != "已发货" && after != "已出库" {
		return false
	}
	switch before {
	case "已发货", "已出库", "已签收", "已收货", "已完成":
		return false
	}
	return true
}

type shipmentNoticeKey struct{ key, tracking string }

func shipmentNoticeKeys(orderID int64, raw string) []shipmentNoticeKey {
	var result []shipmentNoticeKey
	for _, number := range salesapp.NormalizeTrackingNumbers(raw) {
		result = append(result, shipmentNoticeKey{key: fmt.Sprintf("shipment_created:%d:%s", orderID, number), tracking: number})
	}
	return result
}

func recordOrderBusinessNotice(ctx context.Context, tx pgx.Tx, schema, eventType, sourceKey string, orderID int64, tracking string) error {
	var customerID int64
	var payload []byte
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT o.customer_id, jsonb_build_object(
 'order_id',o.id,'order_no',COALESCE(o.order_no,''),'customer_id',o.customer_id,
 'customer_name',COALESCE(c.name,''),'items',COALESCE((SELECT string_agg(i.item_name||' '||COALESCE(i.spec,'')||' × '||i.qty::text||COALESCE(i.unit,''),'；' ORDER BY i.id) FROM %[1]s.order_items i WHERE i.order_id=o.id),''),
 'responsible_id',CASE WHEN o.responsible_party_type='employee' THEN COALESCE(o.responsible_party_id,0) ELSE COALESCE(c.responsible_employee_id,0) END,
 'responsible_name',COALESCE(o.responsible_party_name,''),'tracking_no',$2::text,
 'shipping_status',COALESCE(s.name,''),'occurred_at',to_char(clock_timestamp() AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI:SS'))
 FROM %[1]s.orders o JOIN %[1]s.customers c ON c.id=o.customer_id LEFT JOIN %[1]s.ship_statuses s ON s.id=o.ship_status_id WHERE o.id=$1`, schema), orderID, tracking).Scan(&customerID, &payload)
	// Internal orders can have no customer and therefore have no eligible
	// service-account recipient. This must not prevent an existing order or
	// shipment transaction from completing.
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if customerID <= 0 {
		return nil
	}
	return messagepg.RecordBusinessEvent(ctx, tx, schema, eventType, sourceKey, orderID, customerID, jsonPayload(payload))
}

// MarshalJSON preserves the database's JSON object without encoding it as base64.
type jsonPayload []byte

func (p jsonPayload) MarshalJSON() ([]byte, error) { return p, nil }

func shipmentNoticeSourceKey(orderID, shipmentID int64, tracking string) string {
	// The same parcel may later be linked to an export batch. The existing
	// order_shipping_trackings unique key identifies it by order and tracking.
	// Distinct parcels must carry distinct tracking numbers.
	_ = shipmentID
	return fmt.Sprintf("shipment_created:%d:%s", orderID, tracking)
}

func recordShipmentNotices(ctx context.Context, tx pgx.Tx, schema string, orderID, shipmentID int64, raw string) error {
	for _, item := range shipmentNoticeKeys(orderID, raw) {
		key := shipmentNoticeSourceKey(orderID, shipmentID, item.tracking)
		if err := recordOrderBusinessNotice(ctx, tx, schema, "shipment_created", key, orderID, item.tracking); err != nil {
			return err
		}
	}
	return nil
}
