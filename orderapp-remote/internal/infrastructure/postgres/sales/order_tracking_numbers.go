package sales

import (
	"context"
	"fmt"
	"strings"

	salesapp "orderapp/internal/application/sales"

	"github.com/jackc/pgx/v5"
)

func orderTrackingSummaryExpr(schema, orderAlias string) string {
	return fmt.Sprintf(`COALESCE(NULLIF((
		SELECT string_agg(ost.tracking_no, E'\n' ORDER BY ost.id)
		FROM %s.order_shipping_trackings ost
		WHERE ost.order_id=%s.id
	), ''), COALESCE(%s.ship_tracking_no,''))`, schema, orderAlias, orderAlias)
}

func appendOrderTrackingNumbersTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64, raw, source, actor string) (string, error) {
	if err := seedLegacyOrderTrackingNumbersTx(ctx, tx, schema, orderID); err != nil {
		return "", err
	}
	if err := insertOrderTrackingNumbersTx(ctx, tx, schema, orderID, salesapp.NormalizeTrackingNumbers(raw), source, actor); err != nil {
		return "", err
	}
	return syncOrderTrackingSummaryTx(ctx, tx, schema, orderID)
}

func replaceOrderTrackingNumbersTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64, raw, source, actor string) (string, error) {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.order_shipping_trackings WHERE order_id=$1`, schema), orderID); err != nil {
		return "", err
	}
	if err := insertOrderTrackingNumbersTx(ctx, tx, schema, orderID, salesapp.NormalizeTrackingNumbers(raw), source, actor); err != nil {
		return "", err
	}
	return syncOrderTrackingSummaryTx(ctx, tx, schema, orderID)
}

func seedLegacyOrderTrackingNumbersTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64) error {
	var legacy string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(ship_tracking_no,'') FROM %s.orders WHERE id=$1`, schema), orderID).Scan(&legacy)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	return insertOrderTrackingNumbersTx(ctx, tx, schema, orderID, salesapp.NormalizeTrackingNumbers(legacy), "legacy_order_field", "migration")
}

func insertOrderTrackingNumbersTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64, numbers []string, source, actor string) error {
	source = strings.TrimSpace(source)
	actor = strings.TrimSpace(actor)
	for _, no := range numbers {
		no = strings.TrimSpace(no)
		if no == "" {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_shipping_trackings(order_id, tracking_no, source, created_by)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (order_id, tracking_no) DO NOTHING
		`, schema), orderID, no, source, actor); err != nil {
			return err
		}
	}
	return nil
}

func syncOrderTrackingSummaryTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64) (string, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT tracking_no
		FROM %s.order_shipping_trackings
		WHERE order_id=$1
		ORDER BY id
	`, schema), orderID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	numbers := make([]string, 0)
	for rows.Next() {
		var no string
		if err := rows.Scan(&no); err != nil {
			return "", err
		}
		numbers = append(numbers, no)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	summary := salesapp.TrackingNumbersSummary(numbers)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET ship_tracking_no=$2 WHERE id=$1`, schema), orderID, summary); err != nil {
		return "", err
	}
	return summary, nil
}
