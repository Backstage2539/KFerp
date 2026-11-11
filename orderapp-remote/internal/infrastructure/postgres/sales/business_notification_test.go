package sales

import (
	"context"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	messagepg "orderapp/internal/infrastructure/postgres/messagecenter"
	"testing"
)

func TestFirstAcceptanceNotice(t *testing.T) {
	for _, tc := range []struct {
		decision string
		previous int64
		want     bool
	}{
		{"accepted", 0, true}, {"accepted", 1, false}, {"rejected", 0, false}, {"rejected", 1, false},
	} {
		if got := firstAcceptanceNotice(tc.decision, tc.previous); got != tc.want {
			t.Fatalf("decision %q previous %d: got %v", tc.decision, tc.previous, got)
		}
	}
}

func TestShipmentParcelIdentityAcrossActualEntryPoints(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE %[1]s.customers(id bigint PRIMARY KEY,name text,responsible_employee_id bigint);
 CREATE TABLE %[1]s.ship_statuses(id bigint PRIMARY KEY,name text);
 CREATE TABLE %[1]s.orders(id bigint PRIMARY KEY,customer_id bigint,order_no text,ship_status_id bigint,ship_method text,ship_tracking_no text,is_void boolean,responsible_party_type text,responsible_party_id bigint,responsible_party_name text);
 CREATE TABLE %[1]s.order_items(id bigint,order_id bigint,item_name text,spec text,qty numeric,unit text,product_id bigint,bom_spec_id bigint,bom_variant_id bigint);
 CREATE TABLE %[1]s.order_stock_batch_allocations(order_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,spec_g bigint,allocated_g bigint,allocated_units bigint);
 CREATE TABLE %[1]s.order_stock_deductions(order_id bigint);
 CREATE TABLE %[1]s.order_shipping_trackings(id bigserial PRIMARY KEY,order_id bigint,tracking_no text,source text,created_by text,UNIQUE(order_id,tracking_no));
 CREATE TABLE %[1]s.order_shipments(id bigint PRIMARY KEY,status text);
 CREATE TABLE %[1]s.order_shipment_orders(id bigint PRIMARY KEY,shipment_id bigint,order_id bigint,tracking_no text,shipped_at timestamptz);
 CREATE TABLE %[1]s.order_audit_logs(order_id bigint,actor text,field text,old_value text,new_value text);
 CREATE TABLE %[1]s.audit_logs(actor text,entity_type text,entity_id bigint,action text,field text,old_value text,new_value text,meta jsonb);
 INSERT INTO %[1]s.customers VALUES(7,'测试客户',2);
 INSERT INTO %[1]s.ship_statuses VALUES(1,'未发货'),(2,'已发货');
 INSERT INTO %[1]s.orders VALUES(42,7,'SO-42',1,'express','',false,'employee',2,'负责人');
 INSERT INTO %[1]s.order_items VALUES(1,42,'咖啡','250g',2,'袋',0,0,0);
 INSERT INTO %[1]s.order_stock_deductions VALUES(42);
 INSERT INTO %[1]s.order_shipments VALUES(101,'excel_generated'),(102,'excel_generated');
 `, schema))
	if err != nil {
		t.Fatal(err)
	}
	if err = messagepg.EnsureBusinessEventSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	if _, err = repo.FillOrderTracking(ctx, salesapp.FillOrderTrackingCommand{OrderID: 42, TrackingNo: "SF001", Actor: "测试"}); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.order_shipment_orders VALUES(1,101,42,'',NULL),(2,102,42,'',NULL);
 INSERT INTO %[1]s.orders VALUES(43,7,'SO-43',2,'express','SF003',false,'employee',2,'负责人');
 INSERT INTO %[1]s.order_items VALUES(2,43,'咖啡','250g',2,'袋',0,0,0);
 INSERT INTO %[1]s.order_stock_deductions VALUES(43);
 INSERT INTO %[1]s.order_shipments VALUES(103,'excel_generated');`, schema))
	if err != nil {
		t.Fatal(err)
	}
	for _, batch := range []struct {
		id       int64
		tracking string
	}{{101, "SF001"}, {101, "SF001"}, {102, "SF002"}} {
		if _, err = repo.FillShipmentTracking(ctx, salesapp.FillShipmentTrackingCommand{ShipmentID: batch.id, Actor: "测试", Items: []salesapp.ShipmentTrackingItemCommand{{OrderID: 42, TrackingNo: batch.tracking}}}); err != nil {
			t.Fatalf("shipment %d: %v", batch.id, err)
		}
	}
	// Exercise the transactional shipment operation used by SaveOrder before
	// a later shipment batch associates the already-recorded parcel.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = repo.applySavedOrderShipmentTx(ctx, tx, 43, 1, 2, "测试"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_shipment_orders VALUES(3,103,43,'',NULL)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.FillShipmentTracking(ctx, salesapp.FillShipmentTrackingCommand{ShipmentID: 103, Actor: "测试", Items: []salesapp.ShipmentTrackingItemCommand{{OrderID: 43, TrackingNo: "SF003"}}}); err != nil {
		t.Fatal(err)
	}
	var count int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM "+schema+".message_business_events WHERE event_type='shipment_created'").Scan(&count)
	if err != nil || count != 3 {
		t.Fatalf("expected one event per parcel, got %d: %v", count, err)
	}
}

func TestSavedShipmentNoticeOnlyForActualNewShipment(t *testing.T) {
	for _, tc := range []struct {
		before, after string
		want          bool
	}{
		{"待发货", "已发货", true}, {"待发货", "已出库", true},
		{"已发货", "已发货", false}, {"已发货", "已签收", false}, {"待发货", "已签收", false},
	} {
		if got := savedShipmentNotice(tc.before, tc.after); got != tc.want {
			t.Fatalf("%s -> %s: got %v", tc.before, tc.after, got)
		}
	}
}

func TestOrderBusinessEventsTransactionAndIdempotency(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE %[1]s.customers(id bigint, name text, responsible_employee_id bigint);
 CREATE TABLE %[1]s.ship_statuses(id bigint, name text);
 CREATE TABLE %[1]s.orders(id bigint, customer_id bigint, order_no text, ship_status_id bigint, responsible_party_type text, responsible_party_id bigint, responsible_party_name text);
 CREATE TABLE %[1]s.order_items(id bigint, order_id bigint, item_name text, spec text, qty numeric, unit text);
 INSERT INTO %[1]s.customers VALUES(7,'测试客户',2);
 INSERT INTO %[1]s.ship_statuses VALUES(1,'部分发货');
 INSERT INTO %[1]s.orders VALUES(42,7,'SO-42',1,'employee',2,'负责人');
 INSERT INTO %[1]s.order_items VALUES(1,42,'咖啡','250g',2,'袋');`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if err = messagepg.EnsureBusinessEventSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = recordOrderBusinessNotice(ctx, tx, schema, "order_accepted", "order_accepted:42", 42, ""); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM "+schema+".message_business_events").Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback count=%d error=%v", count, err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for i := 0; i < 2; i++ {
		if err = recordShipmentNotices(ctx, tx, schema, 42, 101, "SF001"); err != nil {
			t.Fatal(err)
		}
	}
	if err = recordShipmentNotices(ctx, tx, schema, 42, 102, "SF002"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM "+schema+".message_business_events WHERE event_type='shipment_created'").Scan(&count); err != nil || count != 2 {
		t.Fatalf("shipments count=%d error=%v", count, err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders VALUES(43,NULL,'INTERNAL',1,'',0,'')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = recordOrderBusinessNotice(ctx, tx, schema, "order_accepted", "order_accepted:43", 43, ""); err != nil {
		t.Fatalf("internal order without customer must not fail: %v", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestShipmentNoticeUsesEachTrackingOnce(t *testing.T) {
	got := shipmentNoticeKeys(42, "SF123\nSF456\nSF123")
	if len(got) != 2 || got[0].tracking != "SF123" || got[1].tracking != "SF456" || got[0].key == got[1].key {
		t.Fatalf("unexpected shipment keys: %#v", got)
	}
	if len(shipmentNoticeKeys(42, "")) != 0 {
		t.Fatal("empty tracking must not create a shipment notice")
	}
}

func TestShipmentNoticeSourceKeyIdentifiesBatch(t *testing.T) {
	first := shipmentNoticeSourceKey(42, 101, "SF001")
	if first != shipmentNoticeSourceKey(42, 101, "SF001") {
		t.Fatal("same shipment should use one identity")
	}
	if first != shipmentNoticeSourceKey(42, 102, "SF001") {
		t.Fatal("same parcel must not send again after attaching another shipment batch")
	}
	if first == shipmentNoticeSourceKey(42, 102, "SF002") {
		t.Fatal("a different parcel must have a different identity")
	}
}
