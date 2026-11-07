package customerfulfillment

import (
	"context"
	"fmt"
	domain "orderapp/internal/domain/orderresolution"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	"testing"
)

func TestOrderResolutionFeesTraceActualShipmentAndExcludeOrderCopies(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.orders(id,order_no,customer_id,order_date,total_amount,shipping_amount,grand_total)VALUES(78411,'FEE-LINKS',501,'2026-10-01',90,10,100),(78412,'OTHER-FEE',501,'2026-10-01',90,0,90);
 INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78411,78411,1,911,'coffee','1000g',3,'袋',30,90);
 CREATE TABLE IF NOT EXISTS %[1]s.freight_actual_shipments(id bigint PRIMARY KEY,order_id bigint,customer_id bigint,tracking_no text);
 CREATE TABLE IF NOT EXISTS %[1]s.freight_actual_entries(id bigint PRIMARY KEY,shipment_id bigint);
 INSERT INTO %[1]s.freight_actual_shipments(id,carrier_id,order_id,customer_id,tracking_no)VALUES(78411,1,78411,501,'isolated-78411'),(78412,1,78412,501,'isolated-other');
 INSERT INTO %[1]s.freight_actual_entries(id,shipment_id,request_key,actual_cents,target_cents,delta_cents,previous_actual_cents,previous_billed_cents,reason,created_by)VALUES(78411,78411,'fee-trace-1',500,500,500,0,0,'test','test'),(78412,78412,'fee-trace-2',900,900,900,0,0,'test','test');
 INSERT INTO %[1]s.customer_fee_items(id,customer_id,source_type,source_id,fee_type,amount,occurred_at)VALUES(78411,501,'order',78411,'direct_ship_service',3,'2026-10-01'),(78412,501,'order',78411,'shipping',10,'2026-10-01'),(78413,501,'order',78411,'product',90,'2026-10-01'),(78414,501,'freight_actual',78411,'shipping',5,'2026-10-01'),(78415,501,'freight_actual',78412,'shipping',9,'2026-10-01');
 `, schema))
	if err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78411)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	for _, d := range c.Dependencies {
		if d.Kind == "fee" {
			seen[d.ID] = true
		}
	}
	if len(seen) != 3 || !seen[-1] || !seen[78411] || !seen[78414] {
		t.Fatalf("wrong fee sources: %+v", c.Dependencies)
	}
	cmd := domain.Command{Action: "credit", Reason: "entry_error", Scope: "fees_only", Timing: "settle_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "fee-source-correction", Dispositions: []domain.Disposition{{Kind: "fee", ID: -1, Choice: "waive"}, {Kind: "fee", ID: 78411, Choice: "keep"}, {Kind: "fee", ID: 78414, Choice: "waive"}}}
	result, err := r.Submit(ctx, 78411, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		if _, err = r.CompleteStep(ctx, 78411, result.ID, step.ID, resolutionpg.StepCommand{Choice: step.Choice, Note: "核对运费来源", Reference: "freight-credit", IdempotencyKey: fmt.Sprint("fee-source-step-", step.ID)}, "finance"); err != nil {
			t.Fatal(err)
		}
	}
	var credit, actual, duplicate int64
	var state string
	var paused bool
	err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT resolution_credit_cents,resolution_status,fulfillment_paused,(SELECT resolution_credit_cents FROM %[1]s.customer_fee_items WHERE id=78414),(SELECT sum(resolution_credit_cents)FROM %[1]s.customer_fee_items WHERE id IN(78412,78413,78415))FROM %[1]s.orders WHERE id=78411`, schema)).Scan(&credit, &state, &paused, &actual, &duplicate)
	if err != nil || credit != 1000 || state != "open" || paused || actual != 500 || duplicate != 0 {
		t.Fatalf("fee source effects credit=%d state=%s paused=%v actual=%d other=%d err=%v", credit, state, paused, actual, duplicate, err)
	}
}
