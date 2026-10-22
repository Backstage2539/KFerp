package customerfulfillment

import (
	"context"
	"fmt"
	fd "orderapp/internal/domain/freight"
	d "orderapp/internal/domain/shippingreturn"
	fp "orderapp/internal/infrastructure/postgres/freight"
	sp "orderapp/internal/infrastructure/postgres/sales"
	"testing"
)

func TestShippingReturnAtomicCandidatesAndReceipts(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	fr := fp.NewRepository(pool, schema)
	if e := fr.EnsureSchema(ctx); e != nil {
		t.Fatal(e)
	}
	r := sp.NewRepository(pool, schema)
	var cid int64
	if e := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name)VALUES('return isolated')RETURNING id`, schema)).Scan(&cid); e != nil {
		t.Fatal(e)
	}
	seed := func(no string, shipping, paid int64) int64 {
		t.Helper()
		var oid int64
		e := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(order_no,customer_id,total_amount,grand_total,shipping_amount,payment_shipping_amount,receiver_name,receiver_phone,process_status_id,logistics_company_id)VALUES($1,$2,100,100+$3,$3,$4,'张三','13812341234',(SELECT id FROM %[1]s.order_process_statuses WHERE name='无需生产' LIMIT 1),1)RETURNING id`, schema), no, cid, float64(shipping)/100, float64(paid)/100).Scan(&oid)
		if e != nil {
			t.Fatal(e)
		}
		q := fd.Snapshot{Protocol: fd.OrderProtocol, ChargeMode: "manual_current", ManualAmountCents: &shipping, CurrentReceivableCents: shipping, SurchargeCents: 200, Policy: fd.Policy{CustomerID: cid, Scene: "sale"}}
		q.Token = fd.Token(q)
		tx, e := pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if e = fr.SaveSnapshotTx(ctx, tx, oid, q, "test"); e != nil {
			t.Fatal(e)
		}
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
		return oid
	}
	cfg := d.DefaultTemplate()
	cfg.CarrierID = 1
	if err := r.SaveShippingReturnTemplates(ctx, "tester", []d.Template{cfg}); err != nil {
		t.Fatal(err)
	}
	templates, err := r.ShippingReturnTemplates(ctx)
	if err != nil || len(templates) != 1 || templates[0].Revision != 2 {
		t.Fatalf("template persistence %+v %v", templates, err)
	}
	if err = r.SaveShippingReturnTemplates(ctx, "tester", []d.Template{cfg}); err == nil {
		t.Fatal("stale template accepted")
	}
	cfg = templates[0]
	row := func(no string) d.Row {
		return d.Row{RowNo: 2, TrackingNo: no, ActualCents: 1200, Receiver: "张*", Phone: "138****1234", Status: "pending"}
	}
	oid := seed("RETURN-1", 2000, 0)
	b, e := r.ImportShippingReturn(ctx, "tester", "test.xlsx", "hash1", cfg, []d.Row{row("SF-RETURN-1")})
	if e != nil || b.Updated != 1 {
		t.Fatalf("import %+v %v", b, e)
	}
	view, e := fr.OrderViews(ctx, pool, []int64{oid})
	if e != nil || view[oid] == nil || view[oid].FinalReceivableCents == nil || *view[oid].FinalReceivableCents != 1400 {
		t.Fatalf("view %+v %v", view, e)
	}
	b, e = r.ImportShippingReturn(ctx, "tester", "test.xlsx", "hash1", cfg, []d.Row{row("SF-RETURN-1")})
	if e != nil || len(b.OrderIDs) != 0 {
		t.Fatalf("replay %+v %v", b, e)
	}
	var count int
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.freight_actual_entries`, schema)).Scan(&count); e != nil || count != 1 {
		t.Fatalf("entries %d %v", count, e)
	}
	paidID := seed("RETURN-PAID", 2000, 2000)
	b, e = r.ImportShippingReturn(ctx, "tester", "test.xlsx", "hash2", cfg, []d.Row{row("SF-RETURN-2")})
	if e != nil || b.Updated != 1 {
		t.Fatalf("paid %+v %v", b, e)
	}
	view, e = fr.OrderViews(ctx, pool, []int64{paidID})
	if e != nil || *view[paidID].FinalReceivableCents != 2000 || view[paidID].ReceivedFreightCents != 2000 {
		t.Fatalf("paid view %+v %v", view, e)
	}
	// Both import entry points share the ledger; the second one must not charge again.
	oldImport, err := fr.PreviewImport(ctx, 1, "cross-import", "cost.xlsx", []fp.ImportRow{{RowNo: 2, TrackingNo: "SF-RETURN-1", ActualCents: 1200}}, "tester")
	if err != nil || oldImport.Rows[0].DeltaCents != 0 {
		t.Fatalf("cross preview %+v %v", oldImport, err)
	}
	if _, err = fr.ConfirmImport(ctx, oldImport.ID, "tester"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name                         string
		shipping, paid, actual, want int64
		goods, unknown               bool
	}{
		{name: "PARTIAL", shipping: 2000, paid: 500, actual: 1200, want: 1400},
		{name: "GOODS-PAID", shipping: 2000, actual: 1200, want: 1400, goods: true},
		{name: "FREE", actual: 1200, want: 0},
		{name: "BELOW-PAID", shipping: 2000, paid: 500, actual: 100, want: 2000},
		{name: "UNKNOWN", shipping: 2000, actual: 1200, want: 2000, unknown: true},
	} {
		id := seed("RETURN-"+c.name, c.shipping, c.paid)
		if c.goods || c.unknown {
			goods := 0
			if c.goods {
				goods = 100
			}
			if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET prepayment_amount=100,payment_goods_amount=$2 WHERE id=$1`, schema), id, goods); err != nil {
				t.Fatal(err)
			}
		}
		item := row("SF-" + c.name)
		item.ActualCents = c.actual
		b, err = r.ImportShippingReturn(ctx, "tester", "test.xlsx", c.name, cfg, []d.Row{item})
		if err != nil || b.Updated != 1 {
			t.Fatalf("%s %+v %v", c.name, b, err)
		}
		view, err = fr.OrderViews(ctx, pool, []int64{id})
		if err != nil || view[id] == nil || *view[id].FinalReceivableCents != c.want || view[id].ReceivedFreightCents != c.paid || *view[id].ActualCostCents != c.actual {
			t.Fatalf("%s %+v %v", c.name, view[id], err)
		}
	}
	// Legacy freight already represented by a separate order fee is not free,
	// and its principal must be counted once when the actual cost arrives.
	legacy := seed("RETURN-LEGACY-SEPARATE", 0, 500)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.order_freight_snapshots WHERE order_id=$1`, schema), legacy); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount)VALUES($1,'order',$2,'shipping',10)`, schema), cid, legacy); err != nil {
		t.Fatal(err)
	}
	b, err = r.ImportShippingReturn(ctx, "tester", "test.xlsx", "legacy-separate", cfg, []d.Row{row("SF-LEGACY-SEPARATE")})
	if err != nil || b.Updated != 1 {
		t.Fatalf("legacy %+v %v", b, err)
	}
	view, err = fr.OrderViews(ctx, pool, []int64{legacy})
	if err != nil || *view[legacy].FinalReceivableCents != 1200 || view[legacy].ReceivedFreightCents != 500 {
		t.Fatalf("legacy separate %+v %v", view[legacy], err)
	}
	a := seed("RETURN-A", 2000, 0)
	z := seed("RETURN-B", 2000, 0)
	b, e = r.ImportShippingReturn(ctx, "tester", "test.xlsx", "hash3", cfg, []d.Row{row("SF-RETURN-A"), row("SF-RETURN-B")})
	if e != nil || b.Pending != 2 || b.Updated != 0 {
		t.Fatalf("ambiguous %+v %v", b, e)
	}
	choices, e := r.ShippingReturnCandidates(ctx, a)
	if e != nil || len(choices) != 2 {
		t.Fatalf("choices %+v %v", choices, e)
	}
	res, e := r.SelectShippingReturn(ctx, "tester", a, choices[0].ID, choices[0].Version)
	if e != nil || res.Updated != 1 {
		t.Fatalf("select %+v %v", res, e)
	}
	if res, e = r.SelectShippingReturn(ctx, "tester", a, choices[0].ID, choices[0].Version); e != nil || res.Updated != 0 {
		t.Fatalf("replay select %+v %v", res, e)
	}
	remaining, e := r.ShippingReturnCandidates(ctx, z)
	if e != nil || len(remaining) != 1 {
		t.Fatalf("remaining %+v %v", remaining, e)
	}
	// A financial validation failure must roll back tracking and shipment state.
	if _, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.shipping_return_rows SET payload=jsonb_set(payload,'{actual_cents}','-1') WHERE id=$1`, schema), remaining[0].ID); e != nil {
		t.Fatal(e)
	}
	if _, e = r.SelectShippingReturn(ctx, "tester", z, remaining[0].ID, remaining[0].Version); e == nil {
		t.Fatal("invalid cost accepted")
	}
	var tracking string
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT coalesce(ship_tracking_no,'') FROM %s.orders WHERE id=$1`, schema), z).Scan(&tracking); e != nil || tracking != "" {
		t.Fatalf("partial write %s %v", tracking, e)
	}
}
