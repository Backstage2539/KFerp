package sales

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	salesdomain "orderapp/internal/domain/sales"
	"strings"
)

func validateStoredPrepaymentTx(ctx context.Context, tx pgx.Tx, schema string, id, statusID int64, total *float64) error {
	var paid, grand float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((to_jsonb(o)->>'prepayment_amount')::numeric,0)::float8,COALESCE(grand_total,0)::float8 FROM %s.orders o WHERE id=$1 FOR UPDATE`, schema), id).Scan(&paid, &grand); err != nil {
		return err
	}
	if total != nil {
		grand = *total
	}
	status, err := lookupStatusName(ctx, tx, schema, "pay_statuses", statusID)
	if err != nil {
		return err
	}
	return salesdomain.ValidatePrepayment(status, paid, grand)
}

// Payment updates invalidate only current exports; historical document versions stay intact.
func invalidatePrepaymentDocumentsTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64) error {
	var amount float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((to_jsonb(o)->>'prepayment_amount')::numeric,0)::float8 FROM %s.orders o WHERE id=$1`, schema), orderID).Scan(&amount); err != nil {
		return err
	}
	if amount <= 0 {
		return nil
	}
	for _, table := range []string{"sales_order_documents", "sales_order_images", "combined_sales_order_documents", "combined_sales_order_images"} {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+"."+table).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			continue
		}
		condition := "order_id=$1"
		if strings.HasPrefix(table, "combined_") {
			condition = "order_ids @> jsonb_build_array($1::bigint)"
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.%s SET is_latest=false WHERE %s AND is_latest`, schema, table, condition), orderID); err != nil {
			return err
		}
	}
	return nil
}
