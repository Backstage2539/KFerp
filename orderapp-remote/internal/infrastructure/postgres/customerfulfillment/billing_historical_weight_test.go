package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
)

func TestIssuedBillFillsOnlyUnambiguousMissingWeightWithoutChangingSnapshot(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	var customerID, otherCustomerID, orderID, otherOrderID, batchID int64
	for _, p := range []struct {
		name string
		dest *int64
	}{{"客户", &customerID}, {"其他客户", &otherCustomerID}} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES($1) RETURNING id`, schema), p.name).Scan(p.dest); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []struct {
		no    string
		owner int64
		dest  *int64
	}{{"SO-WEIGHT", customerID, &orderID}, {"SO-OTHER", otherCustomerID, &otherOrderID}} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,order_date,grand_total) VALUES($1,$2,'2026-10-22',0) RETURNING id`, schema), p.no, p.owner).Scan(p.dest); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_batches(customer_id,settlement_no,status,total_amount,created_by) VALUES($1,'WEIGHT-SNAPSHOT','confirmed',18.10,'test') RETURNING id`, schema), customerID).Scan(&batchID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.shipping_return_rows(id bigint PRIMARY KEY,status text,selected_order_id bigint,payload jsonb)`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.shipping_return_rows(id,status,selected_order_id,payload) VALUES
		(1,'applied',$1,'{"tracking_no":"SF-EXACT","settlement_weight_kg":"7.000"}'),
		(2,'applied',$1,'{"tracking_no":"SF-SNAPSHOT","settlement_weight_kg":"9.000"}'),
		(3,'applied',$2,'{"tracking_no":"SF-WRONG-ORDER","settlement_weight_kg":"4.000"}'),
		(4,'applied',$1,'{"tracking_no":"SF-DUPLICATE","settlement_weight_kg":"1.000"}'),
		(5,'applied',$1,'{"tracking_no":"SF-DUPLICATE","settlement_weight_kg":"2.000"}'),
		(6,'pending',$1,'{"tracking_no":"SF-PENDING","settlement_weight_kg":"5.000"}'),
		(7,'applied',$1,'{"tracking_no":"SF-INCONSISTENT","settlement_weight_kg":"6.000"}')`, schema), orderID, otherOrderID); err != nil {
		t.Fatal(err)
	}
	detail := app.BillingLineDetail{OrderID: orderID, OrderNo: "SO-WEIGHT", ChargeType: "shipping", AmountCents: 1810, InformationSource: "出账快照", Shipments: []app.AccountShipment{
		{TrackingNo: "SF-EXACT"}, {TrackingNo: "SF-SNAPSHOT", SettlementWeightKG: "8.000"}, {TrackingNo: "SF-WRONG-ORDER"}, {TrackingNo: "SF-DUPLICATE"}, {TrackingNo: "SF-PENDING"}, {TrackingNo: ""}, {TrackingNo: "SF-INCONSISTENT", OrderID: otherOrderID},
	}}
	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	var lineID int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_lines(customer_id,settlement_batch_id,charge_type,source_type,source_id,source_no,description,occurred_at,receivable_cents,amount_cents,due_cents,settlement_status,detail_snapshot) VALUES($1,$2,'shipping','customer_fee_item',1,'','运费','2026-10-22',1810,1810,1810,'confirmed',$3::jsonb) RETURNING id`, schema), customerID, batchID, raw).Scan(&lineID); err != nil {
		t.Fatal(err)
	}
	lines := []app.SettlementLineSummary{{ID: lineID, ChargeType: "shipping", AmountCents: 1810}}
	if err = repo.hydrateBillingLineDetails(ctx, pool, customerID, lines); err != nil {
		t.Fatal(err)
	}
	d := lines[0].Detail
	if d == nil || d.AmountCents != 1810 || d.InformationSource != "出账快照" {
		t.Fatalf("snapshot metadata changed: %+v", d)
	}
	for i, want := range []string{"7.000", "8.000", "", "", "", "", ""} {
		if d.Shipments[i].SettlementWeightKG != want {
			t.Errorf("shipment %d weight=%q want %q", i, d.Shipments[i].SettlementWeightKG, want)
		}
	}
	var stored []byte
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT detail_snapshot FROM %s.customer_settlement_lines WHERE id=$1`, schema), lineID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var after app.BillingLineDetail
	if err = json.Unmarshal(stored, &after); err != nil {
		t.Fatal(err)
	}
	if after.Shipments[0].SettlementWeightKG != "" || after.AmountCents != 1810 {
		t.Fatalf("persisted snapshot was mutated: %+v", after)
	}
	// A forged snapshot pointing at another customer's order must not read its weight.
	detail.OrderID = otherOrderID
	detail.Shipments = []app.AccountShipment{{TrackingNo: "SF-WRONG-ORDER"}}
	raw, _ = json.Marshal(detail)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_lines SET detail_snapshot=$1::jsonb WHERE id=$2`, schema), raw, lineID); err != nil {
		t.Fatal(err)
	}
	if err = repo.hydrateBillingLineDetails(ctx, pool, customerID, lines); err != nil {
		t.Fatal(err)
	}
	if got := lines[0].Detail.Shipments[0].SettlementWeightKG; got != "" {
		t.Fatalf("other customer's weight leaked: %q", got)
	}
}
