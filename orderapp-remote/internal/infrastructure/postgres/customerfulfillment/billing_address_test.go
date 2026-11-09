package customerfulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
)

func TestBillingAddressSnapshotHistoricFallbackAndOwnership(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	var customer, other, order, otherOrder, sameCustomerOtherOrder int64
	for _, p := range []struct {
		name string
		id   *int64
	}{{"客户", &customer}, {"其他", &other}} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES($1) RETURNING id`, schema), p.name).Scan(p.id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled) VALUES($1,'settlement',true)`, schema), customer); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct {
		no, address string
		owner       int64
		id          *int64
	}{{"SO-ADDRESS", "云南省测试市初始地址", customer, &order}, {"SO-OTHER", "其他客户私密地址", other, &otherOrder}, {"SO-SAME-CUSTOMER", "同客户另一单地址", customer, &sameCustomerOtherOrder}} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,order_date,grand_total,receiver_address) VALUES($1,$2,'2026-10-22',80,$3) RETURNING id`, schema), p.no, p.owner, p.address).Scan(p.id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_items(order_id,line_no,item_name,spec,qty,unit,unit_price,line_total) VALUES($1,1,'商品','454g',2,'袋',40,80)`, schema), order); err != nil {
		t.Fatal(err)
	}
	cmd := app.BillingPreviewCommand{CustomerID: customer, Sources: []app.BillingSource{{ChargeType: "order_goods", SourceType: "order", SourceID: order}}}
	p, err := repo.PreviewBilling(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	bill, err := repo.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: cmd, PreviewVersion: p.PreviewVersion, IdempotencyKey: "address-snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	read := func() (map[string]any, int64) {
		t.Helper()
		b, e := repo.GetIssuedBill(ctx, customer, bill.BatchID)
		if e != nil {
			t.Fatal(e)
		}
		if len(b.Lines) != 1 || b.Lines[0].Detail == nil {
			t.Fatalf("missing detail %+v", b)
		}
		raw, e := json.Marshal(b.Lines[0].Detail)
		if e != nil {
			t.Fatal(e)
		}
		var d map[string]any
		if e = json.Unmarshal(raw, &d); e != nil {
			t.Fatal(e)
		}
		return d, b.TotalAmountCents
	}
	check := func(d map[string]any, wantAddr, wantSource string) {
		t.Helper()
		a, _ := d["receiver_address"].(string)
		s, _ := d["receiver_address_source"].(string)
		if a != wantAddr || s != wantSource {
			t.Fatalf("address=%q source=%q want=%q %q", a, s, wantAddr, wantSource)
		}
	}
	d, total := read()
	check(d, "云南省测试市初始地址", "出账时订单收货地址")
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET receiver_address='云南省测试市当前地址' WHERE id=$1`, schema), order); err != nil {
		t.Fatal(err)
	}
	d, again := read()
	check(d, "云南省测试市初始地址", "出账时订单收货地址")
	if again != total {
		t.Fatal("bill amount changed")
	}
	// Old bills have no address in the immutable detail snapshot.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_lines SET detail_snapshot=detail_snapshot-'receiver_address'-'receiver_address_source' WHERE settlement_batch_id=$1`, schema), bill.BatchID); err != nil {
		t.Fatal(err)
	}
	d, again = read()
	check(d, "云南省测试市当前地址", "订单当前收货地址")
	if again != total {
		t.Fatal("historic amount changed")
	}
	var stored []byte
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT detail_snapshot FROM %s.customer_settlement_lines WHERE settlement_batch_id=$1`, schema), bill.BatchID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err = json.Unmarshal(stored, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["receiver_address"]; ok {
		t.Fatal("historic snapshot changed")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET receiver_address='' WHERE id=$1`, schema), order); err != nil {
		t.Fatal(err)
	}
	d, _ = read()
	check(d, "", "")
	// Even another order of the same customer cannot replace the bill source.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_lines SET detail_snapshot=jsonb_set(detail_snapshot,'{order_id}',to_jsonb($2::bigint),true) WHERE settlement_batch_id=$1`, schema), bill.BatchID, sameCustomerOtherOrder); err != nil {
		t.Fatal(err)
	}
	d, _ = read()
	check(d, "", "")
	// Corrupt/foreign order IDs cannot reveal another customer's address.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_lines SET detail_snapshot=jsonb_set(detail_snapshot,'{order_id}',to_jsonb($2::bigint),true) WHERE settlement_batch_id=$1`, schema), bill.BatchID, otherOrder); err != nil {
		t.Fatal(err)
	}
	d, _ = read()
	check(d, "", "")
}

func TestFreightUnbilledAndPreviewShowOnlyOwnedOrderAddressAndShipmentWeight(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	var customer, other, order, otherOrder, fee, foreignFee, serviceFee int64
	for _, p := range []struct {
		name string
		id   *int64
	}{{"客户", &customer}, {"其他", &other}} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES($1) RETURNING id`, schema), p.name).Scan(p.id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled) VALUES($1,'settlement',true)`, schema), customer); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct {
		no, address string
		owner       int64
		id          *int64
	}{{"SO-FREIGHT", "云南省测试市收货地址", customer, &order}, {"SO-OTHER", "其他客户私密地址", other, &otherOrder}} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,order_date,grand_total,receiver_address) VALUES($1,$2,'2026-10-22',0,$3) RETURNING id`, schema), p.no, p.owner, p.address).Scan(p.id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.shipping_return_rows(id bigint PRIMARY KEY,status text,selected_order_id bigint,payload jsonb)`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.shipping_return_rows(id,status,selected_order_id,payload) VALUES(1,'applied',$1,'{"tracking_no":"SF-OWN","settlement_weight_kg":"7.000"}'),(2,'applied',$2,'{"tracking_no":"SF-OTHER","settlement_weight_kg":"8.000"}')`, schema), order, otherOrder); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_shipments(id,carrier_id,order_id,customer_id,tracking_no) VALUES(111,1,$1,$2,'SF-OWN'),(112,1,$3,$4,'SF-OTHER')`, schema), order, customer, otherOrder, other); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_entries(id,shipment_id,request_key,actual_cents,target_cents,delta_cents,previous_actual_cents,previous_billed_cents,reason,created_by) VALUES(121,111,'owned',1810,1810,1810,0,0,'test','test'),(122,112,'foreign',1900,1900,1900,0,0,'test','test')`, schema)); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct {
		source int64
		id     *int64
	}{{121, &fee}, {122, &foreignFee}} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,occurred_at,status,note) VALUES($1,'freight_actual',$2,'shipping',18.10,'2026-10-26','unsettled','运费') RETURNING id`, schema), customer, p.source).Scan(p.id); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,occurred_at,status,note) VALUES($1,'order',$2,'direct_ship_service',3,'2026-10-26','unsettled','代发服务费') RETURNING id`, schema), customer, order).Scan(&serviceFee); err != nil {
		t.Fatal(err)
	}
	page, err := repo.ListUnbilledCharges(ctx, app.BillingQuery{CustomerID: customer})
	if err != nil {
		t.Fatal(err)
	}
	find := func(v any, id int64) map[string]any {
		t.Helper()
		raw, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		var out struct {
			Rows  []map[string]any `json:"rows"`
			Lines []map[string]any `json:"lines"`
		}
		if e = json.Unmarshal(raw, &out); e != nil {
			t.Fatal(e)
		}
		for _, row := range append(out.Rows, out.Lines...) {
			if row["source_id"] == float64(id) {
				return row
			}
		}
		t.Fatalf("missing fee %d in %s", id, raw)
		return nil
	}
	check := func(row map[string]any, address, weight string) {
		t.Helper()
		d, ok := row["detail"].(map[string]any)
		if !ok {
			if address != "" {
				t.Fatalf("missing detail: %+v", row)
			}
			return
		}
		a, _ := d["receiver_address"].(string)
		if a != address {
			t.Fatalf("address=%q want=%q", a, address)
		}
		wantSource := "订单当前收货地址"
		if d["information_source"] == "出账快照" {
			wantSource = "出账时订单收货地址"
		}
		if address != "" && d["receiver_address_source"] != wantSource {
			t.Fatalf("wrong source %+v", d)
		}
		shipments, _ := d["shipments"].([]any)
		if weight != "" {
			if len(shipments) != 1 || shipments[0].(map[string]any)["settlement_weight_kg"] != weight {
				t.Fatalf("wrong shipment %+v", d)
			}
		}
	}
	check(find(page, fee), "云南省测试市收货地址", "7.000")
	check(find(page, foreignFee), "", "")
	check(find(page, serviceFee), "云南省测试市收货地址", "7.000")
	p, err := repo.PreviewBilling(ctx, app.BillingPreviewCommand{CustomerID: customer, Sources: []app.BillingSource{{ChargeType: "shipping", SourceType: "customer_fee_item", SourceID: fee}, {ChargeType: "direct_ship_service", SourceType: "customer_fee_item", SourceID: serviceFee}}})
	if err != nil {
		t.Fatal(err)
	}
	check(find(p, fee), "云南省测试市收货地址", "7.000")
	check(find(p, serviceFee), "云南省测试市收货地址", "7.000")
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET receiver_address='云南省测试市修改后地址' WHERE id=$1`, schema), order); err != nil {
		t.Fatal(err)
	}
	cmd := app.IssueBillingCommand{BillingPreviewCommand: app.BillingPreviewCommand{CustomerID: customer, Sources: []app.BillingSource{{ChargeType: "shipping", SourceType: "customer_fee_item", SourceID: fee}, {ChargeType: "direct_ship_service", SourceType: "customer_fee_item", SourceID: serviceFee}}}, PreviewVersion: p.PreviewVersion, IdempotencyKey: "address-changed"}
	if _, err = repo.IssueBilling(ctx, cmd); err == nil || !strings.Contains(err.Error(), "preview changed") {
		t.Fatalf("changed address must reject stale preview, got %v", err)
	}
	// The displayed weight is also part of the confirmed preview version.
	p, err = repo.PreviewBilling(ctx, cmd.BillingPreviewCommand)
	if err != nil {
		t.Fatal(err)
	}
	cmd.PreviewVersion = p.PreviewVersion
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.shipping_return_rows SET payload=jsonb_set(payload,'{settlement_weight_kg}','"8.000"'::jsonb) WHERE id=1`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.IssueBilling(ctx, cmd); err == nil || !strings.Contains(err.Error(), "preview changed") {
		t.Fatalf("changed weight must reject stale preview, got %v", err)
	}
	p, err = repo.PreviewBilling(ctx, cmd.BillingPreviewCommand)
	if err != nil {
		t.Fatal(err)
	}
	// Reproduce the version used before address/weight were part of the preview.
	oldHash := sha256.New()
	fmt.Fprintf(oldHash, "%d|%s|%s|%d\n", p.CustomerID, p.PeriodFrom, p.PeriodTo, p.TotalAmountCents)
	for _, l := range p.Lines {
		fmt.Fprintf(oldHash, "%s|%s|%d|%s|%s|%s|%d|%d|%d\n", l.ChargeType, l.SourceType, l.SourceID, l.OccurredAt, l.SourceNo, l.Description, l.ReceivableCents, l.ReceivedCents, l.DueCents)
	}
	cmd.PreviewVersion = hex.EncodeToString(oldHash.Sum(nil))
	if _, err = repo.IssueBilling(ctx, cmd); err == nil || !strings.Contains(err.Error(), "preview changed") {
		t.Fatalf("old preview must require refresh, got %v", err)
	}
	cmd.PreviewVersion = p.PreviewVersion
	bill, err := repo.IssueBilling(ctx, cmd)
	if err != nil {
		t.Fatalf("fresh preview: %v", err)
	}
	if bill.TotalAmountCents != p.TotalAmountCents {
		t.Fatalf("amount changed: %d / %d", bill.TotalAmountCents, p.TotalAmountCents)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET receiver_address='云南省测试市后来地址' WHERE id=$1`, schema), order); err != nil {
		t.Fatal(err)
	}
	replayed, err := repo.IssueBilling(ctx, cmd)
	if err != nil || replayed.BatchID != bill.BatchID {
		t.Fatalf("idempotent retry: %+v %v", replayed, err)
	}
	check(find(replayed, fee), "云南省测试市修改后地址", "8.000")
}
