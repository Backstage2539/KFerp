package customerfulfillment

import (
	"context"
	"fmt"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
)

func TestFreightBillingDisplaysWaybillAndPreservesHistoricalSnapshot(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	var customer, order, fee int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('运费客户') RETURNING id`, schema)).Scan(&customer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled) VALUES($1,'settlement',true)`, schema), customer); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,order_date,grand_total) VALUES('SO-20260922-000190',$1,'2026-09-22',0) RETURNING id`, schema), customer).Scan(&order); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_shipments(id,carrier_id,order_id,customer_id,tracking_no) VALUES(111,1,$1,$2,'SF1224903286779')`, schema), order, customer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_entries(id,shipment_id,request_key,actual_cents,target_cents,delta_cents,previous_actual_cents,previous_billed_cents,reason,created_by) VALUES(121,111,'test',1810,1810,1810,0,0,'test','test')`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,occurred_at,status,note) VALUES($1,'freight_actual',121,'shipping',18.10,'2026-10-10','unsettled','寄件实际运费调整 · SF1224903286779') RETURNING id`, schema), customer).Scan(&fee); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.ListUnbilledCharges(ctx, app.BillingQuery{CustomerID: customer})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.Rows) != 1 || rows.Rows[0].Description != "运费" || rows.Rows[0].SourceNo != "SF1224903286779" {
		t.Fatalf("unbilled: %+v", rows.Rows)
	}
	cmd := app.BillingPreviewCommand{CustomerID: customer, Sources: []app.BillingSource{{ChargeType: "shipping", SourceType: "customer_fee_item", SourceID: fee}}}
	preview, err := repo.PreviewBilling(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Lines) != 1 || preview.Lines[0].Description != "运费" || preview.Lines[0].SourceNo != "SF1224903286779" {
		t.Fatalf("preview: %+v", preview.Lines)
	}
	bill, err := repo.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: cmd, PreviewVersion: preview.PreviewVersion, IdempotencyKey: "freight-display"})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate old immutable settlement data with its actual tracking snapshot.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_lines SET source_no='',description='寄件实际运费调整 · SF1224903286779',detail_snapshot='{"order_id":1,"order_no":"SO-20260922-000190","charge_type":"shipping","amount_cents":1810,"information_source":"出账快照","shipments":[{"tracking_no":"SF1224903286779"}]}'::jsonb WHERE settlement_batch_id=$1`, schema), bill.BatchID); err != nil {
		t.Fatal(err)
	}
	again, err := repo.GetIssuedBill(ctx, customer, bill.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Lines) != 1 || again.Lines[0].SourceNo != "SF1224903286779" || again.Lines[0].Description != "运费" || again.Lines[0].AmountCents != 1810 {
		t.Fatalf("historic: %+v", again.Lines)
	}
	var source, desc string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT source_no,description FROM %s.customer_settlement_lines WHERE settlement_batch_id=$1`, schema), bill.BatchID).Scan(&source, &desc); err != nil {
		t.Fatal(err)
	}
	if source != "" || desc != "寄件实际运费调整 · SF1224903286779" {
		t.Fatalf("persisted snapshot changed: %q %q", source, desc)
	}
}

func TestFreightBillingDoesNotExposeAnotherCustomersWaybillOrRenameOtherFees(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	var customer, other, order int64
	for _, item := range []struct {
		name string
		id   *int64
	}{{"客户甲", &customer}, {"客户乙", &other}} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES($1) RETURNING id`, schema), item.name).Scan(item.id); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,order_date,grand_total) VALUES('SO-SERVICE',$1,'2026-10-10',0) RETURNING id`, schema), customer).Scan(&order); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_shipments(id,carrier_id,order_id,customer_id,tracking_no) VALUES(211,1,$1,$2,'OTHER-CUSTOMER')`, schema), order, other); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_entries(id,shipment_id,request_key,actual_cents,target_cents,delta_cents,previous_actual_cents,previous_billed_cents,reason,created_by) VALUES(221,211,'isolated',100,100,100,0,0,'test','test')`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,occurred_at,status,note) VALUES($1,'freight_actual',221,'shipping',1,'2026-10-10','unsettled','运费旧说明'),($1,'order',$2,'direct_ship_service',2,'2026-10-10','unsettled','代发服务费')`, schema), customer, order); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.ListUnbilledCharges(ctx, app.BillingQuery{CustomerID: customer})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.Rows) != 2 {
		t.Fatalf("rows=%+v", rows.Rows)
	}
	for _, row := range rows.Rows {
		switch row.ChargeType {
		case "shipping":
			if row.SourceNo != "" || row.Description != "运费" {
				t.Fatalf("cross-customer waybill exposed: %+v", row)
			}
		case "direct_ship_service":
			if row.SourceNo != "SO-SERVICE" || row.Description != "代发服务费" {
				t.Fatalf("other fee changed: %+v", row)
			}
		default:
			t.Fatalf("unexpected charge: %+v", row)
		}
	}
}
