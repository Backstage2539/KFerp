// Package orderconfirmation separates the current editable order version from
// the accepted version used by accounting and execution.
package orderconfirmation

import "strings"

// CurrentRead is only for order list/detail/edit queries. Accounting deliberately
// reads the accepted base tables. Snapshots are produced by SaveOrder, never by clients.
func CurrentRead(query, schema string) string {
	orders := `(SELECT (jsonb_populate_record(current_order, COALESCE(to_jsonb(current_order)->'confirmation_pending'->'order','{}'::jsonb))).* FROM ` + schema + `.orders current_order)`
	items := `(SELECT base_item.* FROM ` + schema + `.order_items base_item JOIN ` + schema + `.orders base_order ON base_order.id=base_item.order_id WHERE to_jsonb(base_order)->'confirmation_pending' IS NULL OR to_jsonb(base_order)->'confirmation_pending'='null'::jsonb UNION ALL SELECT (jsonb_populate_record(NULL::` + schema + `.order_items, pending_item)).* FROM ` + schema + `.orders pending_order CROSS JOIN LATERAL jsonb_array_elements(COALESCE(NULLIF(to_jsonb(pending_order)->'confirmation_pending','null'::jsonb)->'items','[]'::jsonb)) pending_item)`
	// Replace only relation tokens, not column names or functions containing them.
	return strings.NewReplacer("FROM "+schema+".orders ", "FROM "+orders+" ", "JOIN "+schema+".orders ", "JOIN "+orders+" ", "FROM "+schema+".order_items ", "FROM "+items+" ", "JOIN "+schema+".order_items ", "JOIN "+items+" ").Replace(query)
}
