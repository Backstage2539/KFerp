package customerfulfillment

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	fd "orderapp/internal/domain/freight"
	d "orderapp/internal/domain/shippingreturn"
	fp "orderapp/internal/infrastructure/postgres/freight"
)

func TestShippingReturnGroupFreight(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	r := fp.NewRepository(pool, schema)
	if err := r.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf("INSERT INTO %s.customers(name) VALUES('multi parcel') RETURNING id", schema)).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	quote := int64(1000)
	var orderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf("INSERT INTO %s.orders(order_no,customer_id,shipping_amount,payment_shipping_amount,total_amount,grand_total) VALUES('MULTI-PARCEL',$1,10,5,100,110) RETURNING id", schema), customerID).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	save := func(fn func(pgx.Tx) error) error {
		return pgx.BeginFunc(ctx, pool, fn)
	}
	q := fd.Snapshot{Protocol: fd.OrderProtocol, ChargeMode: "manual_current", ManualAmountCents: &quote, CurrentReceivableCents: quote, SurchargeCents: 200, Policy: fd.Policy{CustomerID: customerID, Scene: "sale"}}
	q.Token = fd.Token(q)
	if err := save(func(tx pgx.Tx) error { return r.SaveSnapshotTx(ctx, tx, orderID, q, "test") }); err != nil {
		t.Fatal(err)
	}
	parcels := []d.Row{{ID: 71, TrackingNo: "PARCEL-A", ActualCents: 440}, {ID: 72, TrackingNo: "PARCEL-B", ActualCents: 760}}
	post := func(rows []d.Row) error {
		return save(func(tx pgx.Tx) error {
			_, err := r.ConfirmShippingReturnCostsTx(ctx, tx, orderID, 1, rows, "test")
			return err
		})
	}
	if err := post(parcels); err != nil {
		t.Fatal(err)
	}
	if err := post(parcels); err != nil {
		t.Fatalf("repeat identical posting: %v", err)
	}
	views, err := r.OrderViews(ctx, pool, []int64{orderID})
	if err != nil {
		t.Fatal(err)
	}
	view := views[orderID]
	if view == nil || view.ActualCostCents == nil || view.FinalReceivableCents == nil {
		t.Fatalf("missing freight totals: %+v", view)
	}
	if *view.ActualCostCents != 1200 || *view.FinalReceivableCents != 1400 || view.ReceivedFreightCents != 500 {
		t.Fatalf("two parcels: want cost 1200, receivable 1400, received 500; got cost %d, receivable %d, received %d", *view.ActualCostCents, *view.FinalReceivableCents, view.ReceivedFreightCents)
	}
	var entries, fees, groupMembers, feeCents int64
	var first, second int64
	err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*),count(f.id),count(*) FILTER(WHERE e.shipping_return_group_id=$2),round(sum(f.amount)*100)::bigint,min(e.id),max(e.id)
	 FROM %[1]s.freight_actual_entries e JOIN %[1]s.freight_actual_shipments s ON s.id=e.shipment_id LEFT JOIN %[1]s.customer_fee_items f ON f.source_type='freight_actual' AND f.source_id=e.id WHERE s.order_id=$1`, schema), orderID, parcels[0].ID).Scan(&entries, &fees, &groupMembers, &feeCents, &first, &second)
	if err != nil || entries != 2 || fees != 1 || groupMembers != 2 || feeCents != 400 {
		t.Fatalf("entries=%d fees=%d members=%d fee=%d err=%v", entries, fees, groupMembers, feeCents, err)
	}
	for _, row := range parcels {
		var amount int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT actual_cents FROM %s.freight_actual_shipments WHERE order_id=$1 AND tracking_no=$2`, schema), orderID, row.TrackingNo).Scan(&amount); err != nil || amount != row.ActualCents {
			t.Fatalf("tracking %s: actual=%d err=%v", row.TrackingNo, amount, err)
		}
	}
	if err := post(parcels[:1]); err == nil {
		t.Fatal("single parcel replay of a grouped financial posting accepted")
	}
	for _, id := range []int64{first, second} {
		if err := r.ReverseEntry(ctx, id, fmt.Sprintf("invalid-reverse-%d", id), "单票撤销", "test"); err == nil || !strings.Contains(err.Error(), "多票") {
			t.Fatalf("reverse entry %d: %v", id, err)
		}
	}

	// Independent carrier bills still require reconciliation per order, even if
	// someone later reduces the visible tracking list to one parcel.
	if _, err := pool.Exec(ctx, fmt.Sprintf("UPDATE %s.orders SET ship_tracking_no=$2,logistics_company_id=1 WHERE id=$1", schema), orderID, parcels[0].TrackingNo); err != nil {
		t.Fatal(err)
	}
	bill, err := r.PreviewImport(ctx, 1, "group-bill", "carrier.xlsx", []fp.ImportRow{{RowNo: 2, TrackingNo: parcels[0].TrackingNo, ActualCents: 500}}, "test")
	if err != nil || len(bill.Rows) != 1 || !strings.Contains(bill.Rows[0].Error, "多票") {
		t.Fatalf("single-waybill bill would overwrite grouped costs: %+v err=%v", bill, err)
	}
}

func TestShippingReturnGroupFreightPoliciesAndRollback(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	repo := fp.NewRepository(pool, schema)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	var customer int64
	if err := pool.QueryRow(ctx, fmt.Sprintf("INSERT INTO %s.customers(name) VALUES('group policies') RETURNING id", schema)).Scan(&customer); err != nil {
		t.Fatal(err)
	}
	create := func(no string, quoted, paid, prepay int64) int64 {
		t.Helper()
		var oid int64
		if err := pool.QueryRow(ctx, fmt.Sprintf("INSERT INTO %s.orders(order_no,customer_id,shipping_amount,payment_shipping_amount,prepayment_amount) VALUES($1,$2,$3,$4,$5) RETURNING id", schema), no, customer, quoted, paid, prepay).Scan(&oid); err != nil {
			t.Fatal(err)
		}
		return oid
	}
	post := func(oid int64, rows []d.Row) error {
		return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			_, err := repo.ConfirmShippingReturnCostsTx(ctx, tx, oid, 1, rows, "test")
			return err
		})
	}
	for i, tc := range []struct {
		name                string
		quote, paid, prepay int64
		reason              string
	}{
		{"free", 0, 0, 0, "包邮"},
		{"paid", 20, 20, 20, "已收清"},
		{"below-paid", 20, 18, 18, "低于已收"},
		{"unknown", 20, 0, 5, "待核对"},
	} {
		oid := create(tc.name, tc.quote, tc.paid, tc.prepay)
		id := int64(101 + i*2)
		items := []d.Row{{ID: id, TrackingNo: tc.name + "-A", ActualCents: 200}, {ID: id + 1, TrackingNo: tc.name + "-B", ActualCents: 300}}
		if err := post(oid, items); err != nil {
			t.Fatal(err)
		}
		views, err := repo.OrderViews(ctx, pool, []int64{oid})
		if err != nil {
			t.Fatal(err)
		}
		v := views[oid]
		if v == nil || v.ActualCostCents == nil || *v.ActualCostCents != 500 || v.FinalReceivableCents == nil || *v.FinalReceivableCents != tc.quote*100 || v.ReceivedFreightCents != tc.paid*100 || !strings.Contains(v.ActualConfirmationReason, tc.reason) {
			t.Fatalf("%s: %+v", tc.name, v)
		}
	}
	occupied := create("occupied", 20, 0, 0)
	if err := post(occupied, []d.Row{{ID: 300, TrackingNo: "OCCUPIED", ActualCents: 300}}); err != nil {
		t.Fatal(err)
	}
	target := create("atomic", 10, 0, 0)
	err := post(target, []d.Row{{ID: 301, TrackingNo: "FIRST", ActualCents: 200}, {ID: 302, TrackingNo: "OCCUPIED", ActualCents: 300}})
	if err == nil || !strings.Contains(err.Error(), "关联其他订单") {
		t.Fatalf("occupied second parcel: %v", err)
	}
	var posted int
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s.freight_actual_shipments WHERE order_id=$1", schema), target).Scan(&posted); err != nil || posted != 0 {
		t.Fatalf("partial financial posting persisted after failure: count=%d err=%v", posted, err)
	}
}
