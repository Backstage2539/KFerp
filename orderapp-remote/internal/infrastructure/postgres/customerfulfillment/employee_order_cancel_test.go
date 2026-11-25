package customerfulfillment

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
)

func TestEmployeeOrderCancellationIsScopedAndAuditedOnce(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount,responsible_party_type,responsible_party_id,prepayment_amount)
 VALUES(78241,'MINI-CANCEL',501,90,'employee',7,0),(78242,'MINI-PAID',501,90,'employee',7,20),(78243,'MINI-PARTIAL',501,90,'employee',7,0),(78244,'MINI-FEE',501,90,'employee',7,0);
 UPDATE %[1]s.orders SET ship_status_id=(SELECT id FROM %[1]s.ship_statuses WHERE name='部分发货' LIMIT 1) WHERE id=78243;
 UPDATE %[1]s.orders SET shipping_amount=10,grand_total=100 WHERE id=78244;
 INSERT INTO %[1]s.order_items(order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)
 SELECT id,1,911,'coffee','1000g',3,'袋',30,90 FROM %[1]s.orders WHERE id BETWEEN 78241 AND 78244`, schema))
	if err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	for _, tc := range []struct {
		id, owner int64
		reason    string
		want      error
	}{
		{78241, 8, "duplicate", pgx.ErrNoRows},
		{78241, 7, "other", resolutionpg.ErrEmployeeCancelReason},
		{78242, 7, "customer_cancel", resolutionpg.ErrEmployeeCancelNeedsERP},
		{78243, 7, "entry_error", resolutionpg.ErrEmployeeCancelNeedsERP},
		{78244, 7, "duplicate", resolutionpg.ErrEmployeeCancelNeedsERP},
	} {
		if err := r.CancelEmployeeOrder(ctx, tc.id, tc.owner, false, tc.reason, "test"); !errors.Is(err, tc.want) {
			t.Fatalf("order %d: got %v, want %v", tc.id, err, tc.want)
		}
	}
	for i := 0; i < 2; i++ {
		if err := r.CancelEmployeeOrder(ctx, 78241, 7, false, "entry_error", "mini-employee:7"); err != nil {
			t.Fatal(err)
		}
	}
	var cancelled, paidVoided bool
	var logs, requests int
	var reason, actor string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT is_void FROM %[1]s.orders WHERE id=78241),(SELECT bool_or(is_void) FROM %[1]s.orders WHERE id BETWEEN 78242 AND 78244),
 (SELECT count(*) FROM %[1]s.audit_logs WHERE action='resolution_submit' AND entity_id BETWEEN 78241 AND 78244),
 (SELECT count(*) FROM %[1]s.order_resolution_requests WHERE order_id BETWEEN 78241 AND 78244),command_json->>'reason',actor
 FROM %[1]s.order_resolution_requests WHERE order_id=78241`, schema)).Scan(&cancelled, &paidVoided, &logs, &requests, &reason, &actor); err != nil {
		t.Fatal(err)
	}
	if !cancelled || paidVoided || logs != 1 || requests != 1 || reason != "entry_error" || actor != "mini-employee:7" {
		t.Fatalf("cancel=%t paid=%t logs=%d requests=%d reason=%s actor=%s", cancelled, paidVoided, logs, requests, reason, actor)
	}
}
