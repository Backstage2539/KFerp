package customerportal

import (
	"context"
	"fmt"
	"strings"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMallCutoverBOMSpecIdentityPersistsWithoutLegacyMapping(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	fixture := seedPortalProcessingBOMSpecFixture(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	mallProduct, err := repo.SaveMallProduct(ctx, customerportalapp.SaveMallProductCommand{
		ProductID: fixture.ProductID, BomSpecID: fixture.BomSpecID,
		Title: "PR600 227g袋", UnitPrice: 68, Status: customerportalapp.MallProductStatusPublished, Actor: "admin:600",
	})
	if err != nil {
		t.Fatalf("SaveMallProduct cutover spec: %v", err)
	}
	assertMallBOMSpecIdentity(t, mallProduct, fixture, "save")
	archivedVariantID := fixture.BomVariantID
	fixture.BomVariantID = switchPortalProcessingBOMSpecFixtureToV2(t, ctx, pool, schema, fixture)

	rows, options, err := repo.ListMallProducts(ctx)
	if err != nil {
		t.Fatalf("ListMallProducts: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("mall rows=%+v", rows)
	}
	assertMallBOMSpecIdentity(t, rows[0], fixture, "admin list")
	var canonicalOptionFound bool
	for _, option := range options {
		if option.ProductID == fixture.ProductID && option.BomSpecID == fixture.BomSpecID {
			canonicalOptionFound = option.BomVariantID == fixture.BomVariantID && option.SpecName == "227g袋" && option.InventoryUnit == "袋"
		}
	}
	if !canonicalOptionFound {
		t.Fatalf("canonical option missing without legacy mapping: %+v", options)
	}

	page, err := repo.LoadMallPage(ctx, fixture.CustomerID)
	if err != nil {
		t.Fatalf("LoadMallPage: %v", err)
	}
	if len(page.Products) != 1 {
		t.Fatalf("mall page=%+v", page)
	}
	assertMallBOMSpecIdentity(t, page.Products[0], fixture, "catalog")

	order, err := repo.CreateMallOrder(ctx, customerportalapp.CreateMallOrderCommand{
		CustomerID: fixture.CustomerID, CreatedByMiniUserID: 600,
		RecipientName: "规格客户", RecipientPhone: "13800138000", RecipientAddress: "上海市测试路",
		Items: []customerportalapp.MallOrderItemCommand{{
			MallProductID: mallProduct.ID, ProductID: fixture.ProductID,
			BomSpecID: fixture.BomSpecID, Qty: 2,
		}},
	})
	if err != nil {
		t.Fatalf("CreateMallOrder cutover spec: %v", err)
	}
	var productID, bomSpecID, bomVariantID int64
	var qty, unitPrice, lineTotal float64
	var unit, spec, salesUnit string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT product_id,bom_spec_id,bom_variant_id,qty::float8,unit,spec,unit_price::float8,line_total::float8,sales_unit
		FROM %s.order_items WHERE order_id=$1
	`, schema), order.OrderID).Scan(&productID, &bomSpecID, &bomVariantID, &qty, &unit, &spec, &unitPrice, &lineTotal, &salesUnit); err != nil {
		t.Fatal(err)
	}
	if productID != fixture.ProductID || bomSpecID != fixture.BomSpecID || bomVariantID != fixture.BomVariantID || qty != 2 || unit != "袋" || spec != "227g袋" || unitPrice != 68 || lineTotal != 136 || salesUnit != "袋" {
		t.Fatalf("order item identity=%d/%d/%d qty=%.2f unit=%q spec=%q price=%.2f total=%.2f sales=%q", productID, bomSpecID, bomVariantID, qty, unit, spec, unitPrice, lineTotal, salesUnit)
	}
	assertPortalProcessingCount(t, pool, schema, "audit_logs", "entity_type='mall_product' AND entity_id=$1", mallProduct.ID, 1)
	assertPortalProcessingCount(t, pool, schema, "audit_logs", "entity_type='customer_portal_order' AND entity_id=$1 AND action='mall submit'", order.OrderID, 1)

	_, err = repo.CreateMallOrder(ctx, customerportalapp.CreateMallOrderCommand{
		CustomerID: fixture.CustomerID, CreatedByMiniUserID: 600,
		RecipientName: "规格客户", RecipientPhone: "13800138000", RecipientAddress: "上海市测试路",
		Items: []customerportalapp.MallOrderItemCommand{{MallProductID: mallProduct.ID, ProductID: fixture.ProductID, Qty: 1}},
	})
	if err == nil || !strings.Contains(err.Error(), "bom_spec_id") {
		t.Fatalf("missing order spec error=%v", err)
	}
	_, err = repo.CreateMallOrder(ctx, customerportalapp.CreateMallOrderCommand{
		CustomerID: fixture.CustomerID, CreatedByMiniUserID: 600,
		RecipientName: "规格客户", RecipientPhone: "13800138000", RecipientAddress: "上海市测试路",
		Items: []customerportalapp.MallOrderItemCommand{{MallProductID: mallProduct.ID, ProductID: fixture.ProductID, BomSpecID: fixture.BomSpecID, BomVariantID: archivedVariantID, Qty: 1}},
	})
	if err == nil || !strings.Contains(err.Error(), "no longer current") {
		t.Fatalf("stale mall variant error=%v", err)
	}

	historicalVariantID := seedPortalMallArchivedVariant(t, ctx, pool, schema, fixture)
	_, err = repo.SaveMallProduct(ctx, customerportalapp.SaveMallProductCommand{
		ProductID: fixture.ProductID, BomSpecID: fixture.BomSpecID, BomVariantID: historicalVariantID,
		Title: "过期规格", UnitPrice: 68, Status: customerportalapp.MallProductStatusPublished, Actor: "admin:600",
	})
	if err == nil || !strings.Contains(err.Error(), "current default") {
		t.Fatalf("archived variant save error=%v", err)
	}

	nonDefaultSpecID, nonDefaultVariantID := seedPortalProcessingNonDefaultPublishedSpec(t, ctx, pool, schema, fixture)
	_, err = repo.SaveMallProduct(ctx, customerportalapp.SaveMallProductCommand{
		ProductID: fixture.ProductID, BomSpecID: nonDefaultSpecID, BomVariantID: nonDefaultVariantID,
		Title: "非默认BOM规格", UnitPrice: 68, Status: customerportalapp.MallProductStatusPublished, Actor: "admin:600",
	})
	if err == nil || !strings.Contains(err.Error(), "current default") {
		t.Fatalf("non-default spec save error=%v", err)
	}

	_, err = repo.SaveMallProduct(ctx, customerportalapp.SaveMallProductCommand{
		ProductID: fixture.ProductID, Title: "漏规格", SpecG: 227, UnitPrice: 68,
		Status: customerportalapp.MallProductStatusPublished, Actor: "admin:600",
	})
	if err == nil || !strings.Contains(err.Error(), "bom_spec_id") {
		t.Fatalf("missing save spec error=%v", err)
	}

	legacyChildID := seedPortalProcessingLegacyChildMapping(t, ctx, pool, schema, fixture)
	_, err = repo.SaveMallProduct(ctx, customerportalapp.SaveMallProductCommand{
		ProductID: legacyChildID, SpecG: 227, Title: "旧子SKU", UnitPrice: 68,
		Status: customerportalapp.MallProductStatusPublished, Actor: "admin:600",
	})
	if err == nil || !strings.Contains(err.Error(), "legacy") {
		t.Fatalf("legacy child save error=%v", err)
	}

	assertPortalProcessingCount(t, pool, schema, "mall_products", "id>0 AND $1>0", mallProduct.ID, 1)
	assertPortalProcessingCount(t, pool, schema, "orders", "id=$1", order.OrderID, 1)
}

func assertMallBOMSpecIdentity(t *testing.T, row customerportalapp.MallProduct, fixture portalProcessingBOMSpecFixture, source string) {
	t.Helper()
	if row.ProductID != fixture.ProductID || row.BomSpecID != fixture.BomSpecID || row.BomVariantID != fixture.BomVariantID || row.SpecG != 0 || row.SpecName != "227g袋" || row.InventoryUnit != "袋" {
		t.Fatalf("%s mall identity=%+v", source, row)
	}
}

func seedPortalMallArchivedVariant(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, fixture portalProcessingBOMSpecFixture) int64 {
	t.Helper()
	var versionID, variantID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit)
		VALUES($1,'v0','archived',1,'袋') RETURNING id
	`, schema), fixture.BomID).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
		VALUES($1,$2,'历史227g袋','袋',true,1) RETURNING id
	`, schema), versionID, fixture.BomSpecID).Scan(&variantID); err != nil {
		t.Fatal(err)
	}
	return variantID
}
