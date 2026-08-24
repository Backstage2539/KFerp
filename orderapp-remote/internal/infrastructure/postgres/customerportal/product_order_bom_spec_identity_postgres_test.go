package customerportal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	customerportalapp "orderapp/internal/application/customerportal"
)

func TestProductOrderCutoverBOMSpecIdentityPersistsWithoutLegacyMapping(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalCostingSchema(t, ctx, pool, schema)
	fixture := seedPortalProcessingBOMSpecFixture(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	content := fmt.Sprintf(`{
		"groups":[{"items":[{
			"product_id":%d,"parent_product_id":%d,"bom_spec_id":%d,"bom_variant_id":%d,
			"name":"PR600袋装商品 · 227g袋",
			"commercial_wholesale_tiers":[{"bom_spec_id":%d,"bom_variant_id":%d,"min_qty":1,"price_per_unit":68,"final_unit_price":68,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count"}]
		}]}],
		"price_rows":[{
			"product_id":%d,"parent_product_id":%d,"bom_spec_id":%d,"bom_variant_id":%d,
			"min_qty":1,"price_per_unit":68,"final_unit_price":68,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count"
		}]
	}`, fixture.ProductID, fixture.ProductID, fixture.BomSpecID, fixture.BomVariantID,
		fixture.BomSpecID, fixture.BomVariantID,
		fixture.ProductID, fixture.ProductID, fixture.BomSpecID, fixture.BomVariantID)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.bean_list_publications(list_type,version_no,status,owner_type,owner_key,config_json,content_json,changelog,actor)
		VALUES('commercial','PR600-V1','published','official','','{}'::jsonb,$1::jsonb,'BOM规格价格','test')
	`, schema), content); err != nil {
		t.Fatal(err)
	}
	archivedVariantID := fixture.BomVariantID
	fixture.BomVariantID = switchPortalProcessingBOMSpecFixtureToV2(t, ctx, pool, schema, fixture)

	page, err := repo.LoadServicePage(ctx, customerportalapp.ServicePageQuery{CustomerID: fixture.CustomerID, Key: customerportalapp.ServiceKeyProductOrder, Limit: 20})
	if err != nil {
		t.Fatalf("LoadServicePage: %v", err)
	}
	if len(page.Products) != 1 {
		t.Fatalf("product-order catalog=%+v", page.Products)
	}
	option := page.Products[0]
	if option.ID != fixture.ProductID || option.BomSpecID != fixture.BomSpecID || option.BomVariantID != fixture.BomVariantID || option.SpecName != "227g袋" || option.InventoryUnit != "袋" || option.MigrationState != "cutover" {
		t.Fatalf("canonical product option=%+v", option)
	}

	created, err := repo.CreateFulfillmentOrder(ctx, customerportalapp.CreateFulfillmentOrderCommand{
		CustomerID: fixture.CustomerID, CreatedByMiniUserID: 600,
		PortalServiceCode: customerportalapp.PortalServiceProductOrder,
		RecipientName:     "张三", RecipientPhone: "13800138000", RecipientAddress: "上海市测试路",
		ProductID: fixture.ProductID, BomSpecID: fixture.BomSpecID,
		InventoryUnit: "袋", Qty: 2,
	})
	if err != nil {
		t.Fatalf("CreateFulfillmentOrder: %v", err)
	}
	if created.OrderID <= 0 {
		t.Fatalf("created order=%+v", created)
	}

	var productID, bomSpecID, bomVariantID int64
	var qty, unitPrice, lineTotal float64
	var unit, spec, salesUnit, source string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT product_id,bom_spec_id,bom_variant_id,qty::float8,unit,spec,
		       unit_price::float8,line_total::float8,sales_unit,price_source_json::text
		FROM %s.order_items WHERE order_id=$1
	`, schema), created.OrderID).Scan(&productID, &bomSpecID, &bomVariantID, &qty, &unit, &spec, &unitPrice, &lineTotal, &salesUnit, &source); err != nil {
		t.Fatal(err)
	}
	if productID != fixture.ProductID || bomSpecID != fixture.BomSpecID || bomVariantID != fixture.BomVariantID || qty != 2 || unit != "袋" || spec != "227g袋" || salesUnit != "袋" || unitPrice != 68 || lineTotal != 136 {
		t.Fatalf("order identity/price=%d/%d/%d %.2f %q/%q %.2f/%.2f %q", productID, bomSpecID, bomVariantID, qty, unit, spec, unitPrice, lineTotal, salesUnit)
	}
	var sourceSnapshot struct {
		BomSpecID     int64  `json:"bom_spec_id"`
		QuantityBasis string `json:"quantity_basis"`
	}
	if json.Unmarshal([]byte(source), &sourceSnapshot) != nil || sourceSnapshot.BomSpecID != fixture.BomSpecID || sourceSnapshot.QuantityBasis != "sales_spec_count" {
		t.Fatalf("price source=%s", source)
	}
	var audits int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.audit_logs
		WHERE entity_type='customer_portal_order' AND entity_id=$1 AND action='miniapp fulfillment BOM spec submit'
	`, schema), created.OrderID).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 1 {
		t.Fatalf("canonical product order audit=%d, want 1", audits)
	}
	_, err = repo.CreateFulfillmentOrder(ctx, customerportalapp.CreateFulfillmentOrderCommand{
		CustomerID: fixture.CustomerID, CreatedByMiniUserID: 600,
		PortalServiceCode: customerportalapp.PortalServiceProductOrder,
		RecipientName:     "张三", RecipientPhone: "13800138000", RecipientAddress: "上海市测试路",
		ProductID: fixture.ProductID, BomSpecID: fixture.BomSpecID, BomVariantID: archivedVariantID,
		InventoryUnit: "袋", Qty: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "current default published BOM") {
		t.Fatalf("stale product-order variant err=%v", err)
	}

	legacyChildID := seedPortalProcessingLegacyChildMapping(t, ctx, pool, schema, fixture)
	_, err = repo.CreateFulfillmentOrder(ctx, customerportalapp.CreateFulfillmentOrderCommand{
		CustomerID: fixture.CustomerID, CreatedByMiniUserID: 600,
		PortalServiceCode: customerportalapp.PortalServiceProductOrder,
		RecipientName:     "张三", RecipientPhone: "13800138000", RecipientAddress: "上海市测试路",
		ProductID: legacyChildID, SpecG: 227, Qty: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "legacy child SKU") {
		t.Fatalf("legacy child write err=%v", err)
	}
}

func TestProductOrderSerializesCurrentBOMSpecWithDefaultSwitch(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalCostingSchema(t, ctx, pool, schema)
	fixture := seedPortalProcessingBOMSpecFixture(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)
	content := fmt.Sprintf(`{"price_rows":[{"product_id":%d,"bom_spec_id":%d,"bom_variant_id":%d,"min_qty":1,"final_unit_price":68,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count"}]}`,
		fixture.ProductID, fixture.BomSpecID, fixture.BomVariantID)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.bean_list_publications(list_type,version_no,status,owner_type,owner_key,config_json,content_json,changelog,actor)
		VALUES('commercial','PR600-LOCK','published','official','','{}'::jsonb,$1::jsonb,'BOM规格锁','test')
	`, schema), content); err != nil {
		t.Fatal(err)
	}
	const pauseLock int64 = 600600
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE FUNCTION %[1]s.pr600_pause_product_order() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			PERFORM pg_advisory_xact_lock(%[2]d);
			RETURN NEW;
		END $$;
		CREATE TRIGGER aaa_pr600_pause_product_order BEFORE INSERT ON %[1]s.order_items
		FOR EACH ROW EXECUTE FUNCTION %[1]s.pr600_pause_product_order();
	`, schema, pauseLock)); err != nil {
		t.Fatal(err)
	}

	blocker, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Release()
	if _, err := blocker.Exec(ctx, `SELECT pg_advisory_lock($1)`, pauseLock); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = blocker.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, pauseLock) }()

	orderDone := make(chan error, 1)
	go func() {
		_, err := repo.CreateFulfillmentOrder(ctx, customerportalapp.CreateFulfillmentOrderCommand{
			CustomerID: fixture.CustomerID, CreatedByMiniUserID: 600,
			PortalServiceCode: customerportalapp.PortalServiceProductOrder,
			RecipientName:     "张三", RecipientPhone: "13800138000", RecipientAddress: "上海市测试路",
			ProductID: fixture.ProductID, BomSpecID: fixture.BomSpecID, BomVariantID: fixture.BomVariantID,
			InventoryUnit: "袋", Qty: 1,
		})
		orderDone <- err
	}()

	deadline := time.Now().Add(3 * time.Second)
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_locks
				WHERE locktype='advisory' AND granted=false AND classid=0 AND objid=$1
			)
		`, pauseLock).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("product order did not reach the pre-insert pause")
		}
		time.Sleep(10 * time.Millisecond)
	}

	switchDone := make(chan error, 1)
	go func() {
		_, err := pool.Exec(ctx, fmt.Sprintf(`SELECT state FROM %s.product_bom_spec_migrations WHERE product_id=$1 FOR UPDATE`, schema), fixture.ProductID)
		switchDone <- err
	}()
	select {
	case err := <-switchDone:
		t.Fatalf("default switch lock bypassed in-flight product order: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	if _, err := blocker.Exec(ctx, `SELECT pg_advisory_unlock($1)`, pauseLock); err != nil {
		t.Fatal(err)
	}
	if err := <-orderDone; err != nil {
		t.Fatalf("product order after lock release: %v", err)
	}
	if err := <-switchDone; err != nil {
		t.Fatalf("default switch lock after order commit: %v", err)
	}
}

func TestProductOrderCatalogReturnsAllCurrentBOMSpecsInConfiguredOrder(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	fixture := seedPortalProcessingBOMSpecFixture(t, ctx, pool, schema)
	for i := 2; i <= 30; i++ {
		var specID int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.production_bom_specs(bom_id,code,barcode,spec_key,name,inventory_unit)
			VALUES($1,$2,$3,$4,$5,'袋') RETURNING id
		`, schema), fixture.BomID, fmt.Sprintf("BSP-PR600-%02d", i), fmt.Sprintf("BAR-PR600-%02d", i), fmt.Sprintf("bag-%02d", i), fmt.Sprintf("规格%02d", i)).Scan(&specID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
			VALUES($1,$2,$3,'袋',false,$4)
		`, schema), fixture.VersionID, specID, fmt.Sprintf("规格%02d", i), i); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewRepository(pool, schema)
	page, err := repo.LoadServicePage(ctx, customerportalapp.ServicePageQuery{
		CustomerID: fixture.CustomerID,
		Key:        customerportalapp.ServiceKeyProductOrder,
		Limit:      500,
	})
	if err != nil {
		t.Fatalf("LoadServicePage: %v", err)
	}
	if len(page.Products) != 30 {
		t.Fatalf("product-order catalog count=%d, want all 30 current BOM specs", len(page.Products))
	}
	for i, product := range page.Products {
		wantOrder := i + 1
		if product.ID != fixture.ProductID || product.BomSpecID <= 0 || product.BomVariantID <= 0 || product.SortOrder != wantOrder {
			t.Fatalf("catalog[%d]=%+v, want parent %d sort_order %d", i, product, fixture.ProductID, wantOrder)
		}
	}
}
