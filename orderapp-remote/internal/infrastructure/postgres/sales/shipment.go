package sales

import (
	"context"
	"fmt"
	"time"

	salesapp "orderapp/internal/application/sales"

	"github.com/jackc/pgx/v5"
)

func (r Repository) CreateOrderShipment(ctx context.Context, cmd salesapp.CreateOrderShipmentCommand) (salesapp.OrderShipmentResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return salesapp.OrderShipmentResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.order_shipments IN SHARE ROW EXCLUSIVE MODE", r.schema)); err != nil {
		return salesapp.OrderShipmentResult{}, err
	}
	shipmentNo, err := nextShipmentNo(ctx, tx, r.schema)
	if err != nil {
		return salesapp.OrderShipmentResult{}, err
	}

	var shipmentID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_shipments(shipment_no, created_by, sender_id, file_url, status)
		VALUES ($1,$2,$3,$4,'excel_generated')
		RETURNING id
	`, r.schema), shipmentNo, cmd.Actor, nullInt(cmd.SenderID), cmd.FileURL).Scan(&shipmentID); err != nil {
		return salesapp.OrderShipmentResult{}, err
	}

	readyStatusID := lookupDefaultStatusID(ctx, tx, r.schema, "ship_statuses", "待发货")
	for _, order := range cmd.Orders {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_shipment_orders(shipment_id, order_id, sender_id)
			VALUES ($1,$2,$3)
			ON CONFLICT (shipment_id, order_id) DO UPDATE SET sender_id=EXCLUDED.sender_id
		`, r.schema), shipmentID, order.OrderID, nullInt(order.SenderID)); err != nil {
			return salesapp.OrderShipmentResult{}, err
		}
		if readyStatusID > 0 {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET ship_status_id=$2 WHERE id=$1`, r.schema), order.OrderID, readyStatusID); err != nil {
				return salesapp.OrderShipmentResult{}, err
			}
			if err := r.insertShippingAuditTx(ctx, tx, cmd.Actor, order.OrderID, "ship_status_id", fmt.Sprintf("%d", readyStatusID)); err != nil {
				return salesapp.OrderShipmentResult{}, err
			}
		}
		if err := r.insertShippingAuditTx(ctx, tx, cmd.Actor, order.OrderID, "shipment_no", shipmentNo); err != nil {
			return salesapp.OrderShipmentResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return salesapp.OrderShipmentResult{}, err
	}
	return salesapp.OrderShipmentResult{ShipmentID: shipmentID, ShipmentNo: shipmentNo}, nil
}

func (r Repository) FillShipmentTracking(ctx context.Context, cmd salesapp.FillShipmentTrackingCommand) (salesapp.FillShipmentTrackingResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return salesapp.FillShipmentTrackingResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	shippedStatusID := lookupDefaultStatusID(ctx, tx, r.schema, "ship_statuses", "已发货")
	if shippedStatusID <= 0 {
		return salesapp.FillShipmentTrackingResult{}, fmt.Errorf("ship status 已发货 not found")
	}

	updated := 0
	for _, item := range cmd.Items {
		tag, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.order_shipment_orders
			SET tracking_no=$3, shipped_at=now()
			WHERE shipment_id=$1 AND order_id=$2
		`, r.schema), cmd.ShipmentID, item.OrderID, item.TrackingNo)
		if err != nil {
			return salesapp.FillShipmentTrackingResult{}, err
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.orders
			SET ship_tracking_no=$2, ship_status_id=$3
			WHERE id=$1
		`, r.schema), item.OrderID, item.TrackingNo, shippedStatusID); err != nil {
			return salesapp.FillShipmentTrackingResult{}, err
		}
		if err := r.insertShippingAuditTx(ctx, tx, cmd.Actor, item.OrderID, "ship_tracking_no", item.TrackingNo); err != nil {
			return salesapp.FillShipmentTrackingResult{}, err
		}
		if err := r.insertShippingAuditTx(ctx, tx, cmd.Actor, item.OrderID, "ship_status_id", fmt.Sprintf("%d", shippedStatusID)); err != nil {
			return salesapp.FillShipmentTrackingResult{}, err
		}
		updated++
	}
	if updated > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_shipments SET status='shipped' WHERE id=$1`, r.schema), cmd.ShipmentID); err != nil {
			return salesapp.FillShipmentTrackingResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.FillShipmentTrackingResult{}, err
	}
	return salesapp.FillShipmentTrackingResult{Updated: updated}, nil
}

func nextShipmentNo(ctx context.Context, tx pgx.Tx, schema string) (string, error) {
	prefix := "SHIP-" + time.Now().Format("20060102") + "-"
	var maxNo int
	q := fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(right(shipment_no,4) AS INT)), 0)
		FROM %s.order_shipments
		WHERE shipment_no LIKE $1
	`, schema)
	if err := tx.QueryRow(ctx, q, prefix+"%").Scan(&maxNo); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%04d", prefix, maxNo+1), nil
}
