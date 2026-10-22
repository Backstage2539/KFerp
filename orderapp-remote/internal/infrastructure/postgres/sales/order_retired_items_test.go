package sales

import (
	"context"
	"fmt"
	"testing"

	salesapp "orderapp/internal/application/sales"
)

func TestRetiredFulfillmentItemsExcludedFromCurrentSalesReads(t *testing.T) {
	ctx := context.Background()
	pool, schema := newSalesPostgresTestDB(t)
	t.Cleanup(func() { pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() })
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	prepareDeliveryNoteGenerationOrderColumns(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS is_void boolean NOT NULL DEFAULT false, ADD COLUMN IF NOT EXISTS portal_service_code text NOT NULL DEFAULT ''`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	seedDeliveryNoteGenerationOrder(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
 ALTER TABLE %[1]s.products ADD COLUMN product_kind text NOT NULL DEFAULT 'roasted_bean';
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS is_void boolean NOT NULL DEFAULT false;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS order_type_id bigint;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS process_status_id bigint;
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS receiver_company text NOT NULL DEFAULT '';
 ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS sender_id bigint;
 ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS price_tier_id bigint;
 ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS price_overridden boolean NOT NULL DEFAULT false;
 ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS discount_value numeric NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS bean_list_publication_id bigint;
 ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS bean_list_version_no text NOT NULL DEFAULT '';
 CREATE TABLE IF NOT EXISTS %[1]s.order_types(id bigint,name text);
 CREATE TABLE IF NOT EXISTS %[1]s.order_audit_logs(id bigserial,order_id bigint,actor text,field text,old_value text,new_value text,changed_at timestamptz DEFAULT now());
 CREATE TABLE IF NOT EXISTS %[1]s.product_categories(id bigint,parent_id bigint,name text);
 INSERT INTO %[1]s.products(id,name,product_kind) VALUES(2,'退役生豆','green_bean');
 INSERT INTO %[1]s.order_items(order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total,product_kind,discount_type,discount_amount)
 VALUES(1,2,2,'退役生豆','1000g',0,'袋',50,0,'green_bean','amount',0);
 `, schema)); err != nil {
		t.Fatal(err)
	}
	r := NewRepository(pool, schema)
	t.Run("form and inline detail", func(t *testing.T) {
		rows, err := pool.Query(ctx, orderEditItemsQuery(schema), int64(1))
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			count++
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("current edit/detail returned %d items including retired row", count)
		}
	})
	t.Run("copy details", func(t *testing.T) {
		copy, err := r.LoadOrderCopySnapshot(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if copy == nil || len(copy.Order.Items) != 1 {
			t.Fatalf("copy includes retired items: %+v", copy)
		}
	})
	t.Run("shipping excel", func(t *testing.T) {
		data, err := r.LoadOrderShippingExportData(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(data.Items) != 1 || data.Items[0].Name == "退役生豆" {
			t.Fatalf("shipping export includes retired items: %+v", data.Items)
		}
	})
	t.Run("sales PDF and PNG source", func(t *testing.T) {
		preview, err := r.PreviewSalesOrderDocument(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(preview.Snapshot.Items) != 1 {
			t.Fatalf("sales document includes retired items: %+v", preview.Snapshot.Items)
		}
	})
	t.Run("delivery document", func(t *testing.T) {
		preview, err := r.PreviewDeliveryNoteDocument(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(preview.Snapshot.Items) != 1 {
			t.Fatalf("delivery document includes retired items: %+v", preview.Snapshot.Items)
		}
	})
	t.Run("list product types and common products", func(t *testing.T) {
		orders := []salesapp.OrderRow{{ID: 1}}
		if err := populateOrderTypeSummaries(ctx, pool, schema, orders); err != nil {
			t.Fatal(err)
		}
		if len(orders[0].ProductTypeLabels) != 1 || orders[0].ProductTypeLabels[0] != "熟豆" {
			t.Fatalf("retired kind leaked: %+v", orders[0].ProductTypeLabels)
		}
		usage, err := r.fetchOrderCustomerProductUsages(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(usage) != 1 || usage[0].ProductID != 1 {
			t.Fatalf("retired product leaked to common products: %+v", usage)
		}
	})
	var physical int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.order_items WHERE order_id=1`, schema)).Scan(&physical); err != nil || physical != 2 {
		t.Fatalf("history/tombstone was deleted: %d %v", physical, err)
	}
}
