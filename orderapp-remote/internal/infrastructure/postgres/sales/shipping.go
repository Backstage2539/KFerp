package sales

import (
	"context"
	"fmt"
	"strings"

	salesapp "orderapp/internal/application/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

func (r Repository) FillTrackingPairs(ctx context.Context, cmd salesapp.FillTrackingPairsCommand) (salesapp.FillTrackingResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return salesapp.FillTrackingResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	group := map[string][]string{}
	for _, pair := range cmd.Pairs {
		group[pair.Phone] = append(group[pair.Phone], pair.Tracking)
	}

	updated := 0
	for phone, tracks := range group {
		rows, err := tx.Query(ctx, fmt.Sprintf(`
			SELECT o.id
			FROM %s.orders o
			JOIN %s.customers c ON c.id=o.customer_id
			WHERE o.is_void=false
			  AND COALESCE(o.ship_tracking_no,'')=''
			  AND NOT EXISTS (SELECT 1 FROM %s.order_shipping_trackings ost WHERE ost.order_id=o.id)
			  AND regexp_replace(COALESCE(c.phone,''),'\D','','g') = $1
			ORDER BY o.order_date, o.id
		`, r.schema, r.schema, r.schema), phone)
		if err != nil {
			return salesapp.FillTrackingResult{}, err
		}
		ids := make([]int64, 0)
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return salesapp.FillTrackingResult{}, err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return salesapp.FillTrackingResult{}, err
		}
		rows.Close()

		if len(ids) == 1 {
			summary, err := appendOrderTrackingNumbersTx(ctx, tx, r.schema, ids[0], salesapp.TrackingNumbersSummary(salesapp.NormalizeTrackingNumbers(strings.Join(tracks, "\n"))), "phone_tracking_fill", cmd.Actor)
			if err != nil {
				return salesapp.FillTrackingResult{}, err
			}
			if err := r.insertShippingAuditTx(ctx, tx, cmd.Actor, ids[0], "ship_tracking_no", summary); err != nil {
				return salesapp.FillTrackingResult{}, err
			}
			updated++
			continue
		}
		n := len(ids)
		if len(tracks) < n {
			n = len(tracks)
		}
		for i := 0; i < n; i++ {
			summary, err := appendOrderTrackingNumbersTx(ctx, tx, r.schema, ids[i], tracks[i], "phone_tracking_fill", cmd.Actor)
			if err != nil {
				return salesapp.FillTrackingResult{}, err
			}
			if err := r.insertShippingAuditTx(ctx, tx, cmd.Actor, ids[i], "ship_tracking_no", summary); err != nil {
				return salesapp.FillTrackingResult{}, err
			}
			updated++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return salesapp.FillTrackingResult{}, err
	}
	return salesapp.FillTrackingResult{Updated: updated, Total: len(cmd.Pairs)}, nil
}

func (r Repository) SetShipMethod(ctx context.Context, cmd salesapp.SetShipMethodCommand) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, id := range cmd.OrderIDs {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET ship_method=$2 WHERE id=$1`, r.schema), id, cmd.Method); err != nil {
			return err
		}
		if err := r.insertShippingAuditTx(ctx, tx, cmd.Actor, id, "ship_method", cmd.Method); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r Repository) insertShippingAuditTx(ctx context.Context, tx pgx.Tx, actor string, orderID int64, field, newValue string) error {
	_, _ = tx.Exec(ctx,
		fmt.Sprintf(`INSERT INTO %s.order_audit_logs(order_id, actor, field, old_value, new_value) VALUES ($1,$2,$3,NULL,$4)`, r.schema),
		orderID,
		actor,
		field,
		newValue,
	)
	return postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &orderID, "update", postgresinfra.StrPtr(field), nil, postgresinfra.StrPtr(newValue), postgresinfra.AuditMeta{"order_id": orderID})
}
