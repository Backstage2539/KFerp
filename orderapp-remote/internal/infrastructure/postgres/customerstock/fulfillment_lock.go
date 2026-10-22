package customerstock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
)

// LockFulfillmentCustomersTx must be called before physical inventory changes
// that can convert retained-sale holds. All dropship submit/edit/cancel writers
// use these same customer locks; sorted ids make combined receipts deterministic.
func LockFulfillmentCustomersTx(ctx context.Context, tx pgx.Tx, schema, orderNos string) error {
	if strings.TrimSpace(orderNos) == "" {
		return nil
	}
	rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT DISTINCT customer_id FROM %s.orders WHERE order_no=ANY(string_to_array(replace($1,' ',''),',')) AND customer_id>0 ORDER BY customer_id`, schema), orderNos)
	if e != nil {
		return e
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("mini-direct-ship:%d", id)); e != nil {
			return e
		}
	}
	return nil
}

// The production entry point calls this before locking the running row or any
// finished stock. Read all output references, including combined-spec receipts.
func LockRunningFulfillmentCustomersTx(ctx context.Context, tx pgx.Tx, schema string, runningID int64) error {
	var nos string
	if e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(order_nos,'') FROM %s.produce_running_items WHERE id=$1`, schema), runningID).Scan(&nos); e != nil {
		return e
	}
	var hasOutputs bool
	if e := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".produce_running_outputs").Scan(&hasOutputs); e != nil {
		return e
	}
	if hasOutputs {
		var other string
		if e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(string_agg(order_nos,','),'') FROM %s.produce_running_outputs WHERE running_item_id=$1`, schema), runningID).Scan(&other); e != nil {
			return e
		}
		nos += "," + other
	}
	return LockFulfillmentCustomersTx(ctx, tx, schema, nos)
}
