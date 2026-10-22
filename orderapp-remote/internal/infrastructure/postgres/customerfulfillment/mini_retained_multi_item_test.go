package customerfulfillment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	portalhttp "orderapp/internal/interfaces/http/customerportal"
)

func TestMiniRequestItemIdentityMigrationPreservesCanonicalAndLegacyUniqueness(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	// PostgreSQL chooses the historical constraint name itself, including its
	// middle truncation. A migration cannot assume a hand-written long name.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.customer_direct_ship_request_items ADD UNIQUE(request_id,product_id,spec_g)`, schema)); err != nil {
		t.Fatal(err)
	}
	var requestID, historicalID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_requests(customer_id,idempotency_key)VALUES(501,'MULTI-MIGRATION-HISTORY')RETURNING id`, schema)).Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_request_items(request_id,line_no,product_id,spec_g,qty)VALUES($1,1,911,1000,3)RETURNING id`, schema), requestID).Scan(&historicalID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := EnsureSchema(ctx, pool, schema); err != nil {
			t.Fatal(err)
		}
	}
	var obsolete int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint c WHERE c.conrelid=$1::regclass AND c.contype='u' AND (SELECT array_agg(a.attname::text ORDER BY k.ord) FROM unnest(c.conkey) WITH ORDINALITY k(attnum,ord) JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.attnum)=ARRAY['request_id','product_id','spec_g']::text[]`, schema+".customer_direct_ship_request_items").Scan(&obsolete); err != nil || obsolete != 0 {
		t.Fatalf("obsolete legacy-only constraint remains: count=%d err=%v", obsolete, err)
	}
	insert := func(line, specID, grams int64, wantDuplicate bool) {
		t.Helper()
		_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_request_items(request_id,line_no,product_id,bom_spec_id,spec_g,qty)VALUES($1,$2,911,$3,$4,1)`, schema), requestID, line, specID, grams)
		if wantDuplicate {
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
				t.Fatalf("duplicate identity was not rejected: %v", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
	insert(2, 0, 500, false)
	insert(3, 7031, 0, false)
	insert(4, 7033, 0, false)
	insert(5, 7031, 0, true)
	insert(5, 0, 500, true)
	insert(1, 7035, 0, true) // The independent request/line constraint remains.
	var primaryKeys, foreignKeys int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE contype='p'),count(*) FILTER(WHERE contype='f') FROM pg_constraint WHERE conrelid=$1::regclass`, schema+".customer_direct_ship_request_items").Scan(&primaryKeys, &foreignKeys); err != nil || primaryKeys != 1 || foreignKeys != 1 {
		t.Fatalf("migration changed primary/foreign keys: primary=%d foreign=%d err=%v", primaryKeys, foreignKeys, err)
	}
	var qty int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty FROM %s.customer_direct_ship_request_items WHERE id=$1`, schema), historicalID).Scan(&qty); err != nil || qty != 3 {
		t.Fatalf("migration changed historical item: qty=%d err=%v", qty, err)
	}
}

func TestMiniRetainedMultiItemAPISavesEachBOMSpecAndRollsBackWholePurchase(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
ALTER TABLE %[1]s.customer_direct_ship_request_items ADD UNIQUE(request_id,product_id,spec_g);
INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)VALUES(7033,7030,'MULTI-227','227g','227g袋','袋');
INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default)VALUES(7034,7030,7033,'227g袋','袋',false);
INSERT INTO %[1]s.products(id,name,product_kind,active,unit_rule_override_json)VALUES(912,'另一商品','roasted_bean',true,'{"default_sales_unit":"bag","inventory_unit":"bag","unit_conversion_json":{"bag":{"bag":1}}}');
INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,status)VALUES(8030,'MULTI-BOM','另一商品BOM','product',912,'active');
INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit)VALUES(8030,8030,'V1','published',1,'袋');
INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)VALUES(8031,8030,'MULTI-OTHER-227','227g','227g袋','袋');
INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default)VALUES(8032,8030,8031,'227g袋','袋',true);
INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)VALUES('product',912,8030,8030,true);
INSERT INTO %[1]s.customer_service_capabilities(customer_id,capability_code,enabled)VALUES(501,'product_order',false);
`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	items := []app.MiniDirectShipItemCommand{
		{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 1, SalesUnit: "袋", InventoryUnit: "袋"},
		{ProductID: 912, BomSpecID: 8031, BomVariantID: 8032, Qty: 4, SalesUnit: "袋", InventoryUnit: "袋"},
		{ProductID: 910, BomSpecID: 7033, BomVariantID: 7034, Qty: 4, SalesUnit: "袋", InventoryUnit: "袋"},
	}
	prices := []int64{30, 20, 12}
	rows := []map[string]any{}
	for i, item := range items {
		rows = append(rows, map[string]any{"product_id": item.ProductID, "parent_product_id": item.ProductID, "bom_spec_id": item.BomSpecID, "bom_variant_id": item.BomVariantID, "source_price_record_id": 8000 + i, "min_qty": 1, "final_unit_price": prices[i], "price_unit": "袋", "inventory_unit": "袋", "quantity_basis": "sales_spec_count", "effective_sales_spec": map[string]any{"product_id": item.ProductID, "parent_product_id": item.ProductID, "bom_spec_id": item.BomSpecID, "bom_variant_id": item.BomVariantID, "net_content_qty": 227, "net_content_unit": "g", "sales_unit": "袋", "inventory_unit": "袋"}})
	}
	rows[0]["effective_sales_spec"].(map[string]any)["net_content_qty"] = 454
	content, _ := json.Marshal(map[string]any{"price_rows": rows})
	var publicationID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publications(list_type,version_no,status,owner_type,publication_purpose,content_json)VALUES('commercial','MULTI-RETAINED','published','official','factory_supply',$1::jsonb)RETURNING id`, schema), content).Scan(&publicationID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_order_price_table_bindings(customer_id,usage_code,product_type_key,publication_id)VALUES(501,'product_order','commercial',$1)`, schema), publicationID); err != nil {
		t.Fatal(err)
	}
	svc := app.NewService(NewRepository(pool, schema))
	svc.UseSalesOrderService(salesapp.NewService(salespg.NewRepository(pool, schema)))
	e := echo.New()
	portalhttp.RegisterRoutes(e, portalhttp.Dependencies{CustomerPortal: &priceCatalogPrincipal{customerID: 501, productOrder: true}, CustomerFulfillment: svc})
	post := func(path string, cmd app.MiniDirectShipCommand, want int) []byte {
		t.Helper()
		raw, _ := json.Marshal(cmd)
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(raw)))
		req.Header.Set(echo.HeaderAuthorization, "Bearer multi-retained")
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("POST %s status=%d want=%d body=%s", path, rec.Code, want, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	cmd := app.MiniDirectShipCommand{IdempotencyKey: "MULTI-RETAINED-THREE", DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-A1", Items: items}
	var preview app.MiniDirectShipPreview
	if err := json.Unmarshal(post("/api/mini/product-orders/preview", cmd, 200), &preview); err != nil || !preview.CanSubmit || preview.TotalAmount != 158 || len(preview.Items) != 3 {
		t.Fatalf("multi preview=%+v err=%v", preview, err)
	}
	cmd.PriceQuoteToken = preview.PriceQuoteToken
	counts := func() string {
		t.Helper()
		var state string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_array((SELECT count(*) FROM %[1]s.orders),(SELECT count(*) FROM %[1]s.order_items),(SELECT count(*) FROM %[1]s.customer_direct_ship_requests),(SELECT count(*) FROM %[1]s.customer_stock_entitlements),(SELECT count(*) FROM %[1]s.audit_logs))::text`, schema)).Scan(&state); err != nil {
			t.Fatal(err)
		}
		return state
	}
	before := counts()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_multi_purchase_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.entity_type='customer_direct_ship_request' AND NEW.action='submit' THEN RAISE EXCEPTION 'forced multi purchase audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_multi_purchase_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_multi_purchase_audit()`, schema)); err != nil {
		t.Fatal(err)
	}
	post("/api/mini/product-orders", cmd, 500)
	if after := counts(); after != before {
		t.Fatalf("failed purchase left partial order/lines/rights/audit: before=%s after=%s", before, after)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER reject_multi_purchase_audit ON %s.audit_logs`, schema)); err != nil {
		t.Fatal(err)
	}
	var created app.MiniDirectShipRequest
	if err := json.Unmarshal(post("/api/mini/product-orders", cmd, 201), &created); err != nil || len(created.Items) != 3 || created.TotalAmount != 158 {
		t.Fatalf("multi purchase=%+v err=%v", created, err)
	}
	for i, item := range items {
		var quantity, promised int64
		var unitPrice, lineTotal float64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT i.qty::bigint,i.unit_price::float8,i.line_total::float8,e.promised_qty FROM %[1]s.order_items i JOIN %[1]s.customer_stock_entitlements e ON e.source_order_item_id=i.id AND e.product_id=i.product_id AND e.bom_spec_id=i.bom_spec_id AND e.bom_variant_id=i.bom_variant_id JOIN %[1]s.customer_direct_ship_request_items ri ON ri.request_id=$5 AND ri.product_id=i.product_id AND ri.bom_spec_id=i.bom_spec_id AND (ri.snapshot->>'bom_variant_id')::bigint=i.bom_variant_id AND (ri.snapshot->>'qty')::bigint=i.qty AND (ri.snapshot->>'unit_price')::numeric=i.unit_price AND (ri.snapshot->>'line_amount')::numeric=i.line_total WHERE i.order_id=$1 AND i.product_id=$2 AND i.bom_spec_id=$3 AND i.bom_variant_id=$4 AND e.customer_id=501 AND e.warehouse_code='DEV-E2E-A1'`, schema), created.OrderID, item.ProductID, item.BomSpecID, item.BomVariantID, created.ID).Scan(&quantity, &unitPrice, &lineTotal, &promised); err != nil || quantity != item.Qty || promised != item.Qty || unitPrice != float64(prices[i]) || lineTotal != float64(prices[i]*item.Qty) {
			t.Fatalf("saved identity %+v qty=%d promised=%d price=%v line=%v err=%v", item, quantity, promised, unitPrice, lineTotal, err)
		}
	}
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='customer_direct_ship_request' AND entity_id=%d AND action='submit'", created.ID), 1)
	var retry app.MiniDirectShipRequest
	if err := json.Unmarshal(post("/api/mini/product-orders", cmd, 201), &retry); err != nil || retry.OrderID != created.OrderID {
		t.Fatalf("retry duplicated purchase: %+v err=%v", retry, err)
	}
	assertMiniDirectShipCount(t, pool, schema, "order_items", fmt.Sprintf("order_id=%d", created.OrderID), 3)
	assertMiniDirectShipCount(t, pool, schema, "customer_stock_entitlements", fmt.Sprintf("source_order_id=%d", created.OrderID), 3)
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='customer_direct_ship_request' AND entity_id=%d AND action='submit'", created.ID), 1)
	cmd.IdempotencyKey = "MULTI-RETAINED-SAME-SPEC-MERGES"
	cmd.Items = []app.MiniDirectShipItemCommand{items[0], items[0]}
	cmd.Items[1].Qty = 4
	if err := json.Unmarshal(post("/api/mini/product-orders/preview", cmd, 200), &preview); err != nil || len(preview.Items) != 1 || preview.Items[0].Qty != 5 {
		t.Fatalf("same spec was not combined: %+v err=%v", preview, err)
	}
	cmd.PriceQuoteToken = preview.PriceQuoteToken
	if err := json.Unmarshal(post("/api/mini/product-orders", cmd, 201), &created); err != nil || len(created.Items) != 1 || created.Items[0].Qty != 5 || created.TotalAmount != 150 {
		t.Fatalf("combined same spec purchase=%+v err=%v", created, err)
	}
}
