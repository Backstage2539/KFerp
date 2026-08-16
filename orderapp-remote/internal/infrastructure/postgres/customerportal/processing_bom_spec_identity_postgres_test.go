package customerportal

import (
	"context"
	"fmt"
	"strings"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	postgresproductspecmigration "orderapp/internal/infrastructure/postgres/productspecmigration"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"

	"github.com/jackc/pgx/v5/pgxpool"
)

type portalProcessingBOMSpecFixture struct {
	CustomerID   int64
	ProductID    int64
	BomID        int64
	VersionID    int64
	BomSpecID    int64
	BomVariantID int64
	BeanID       int64
	BagID        int64
}

func TestProcessingRequestCutoverBOMSpecIdentityPersistsWithoutLegacyMapping(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	fixture := seedPortalProcessingBOMSpecFixture(t, ctx, pool, schema)
	archivedVariantID := fixture.BomVariantID
	fixture.BomVariantID = switchPortalProcessingBOMSpecFixtureToV2(t, ctx, pool, schema, fixture)
	repo := NewRepository(pool, schema)

	targets, err := repo.ListProcessingCatalogTargets(ctx, fixture.CustomerID, []int64{fixture.ProductID})
	if err != nil {
		t.Fatalf("ListProcessingCatalogTargets: %v", err)
	}
	if len(targets) != 1 || targets[0].ProductID != fixture.ProductID || targets[0].BomSpecID != fixture.BomSpecID || targets[0].BomVariantID != fixture.BomVariantID || targets[0].SpecName != "227g袋" || targets[0].InventoryUnit != "袋" {
		t.Fatalf("catalog targets=%+v", targets)
	}

	cmd := customerportalapp.CreateProcessingRequestCommand{
		CustomerID:          fixture.CustomerID,
		CreatedByMiniUserID: 600,
		Items: []customerportalapp.ProcessingRequestItemCommand{{
			ProductID: fixture.ProductID, BomSpecID: fixture.BomSpecID, Qty: 2,
		}},
		Note: "PR-600 BOM规格代加工",
	}
	preview, err := repo.PreviewProcessingRequest(ctx, cmd)
	if err != nil {
		t.Fatalf("PreviewProcessingRequest: %v", err)
	}
	if !preview.CanSubmit || len(preview.Items) != 1 {
		t.Fatalf("preview=%+v", preview)
	}
	item := preview.Items[0]
	if item.ProductID != fixture.ProductID || item.ParentProductID != fixture.ProductID || item.BomSpecID != fixture.BomSpecID || item.BomVariantID != fixture.BomVariantID {
		t.Fatalf("preview identity=%d/%d/%d/%d", item.ProductID, item.ParentProductID, item.BomSpecID, item.BomVariantID)
	}
	if item.SpecG != 0 || item.Qty != 2 || item.SpecName != "227g袋" || item.InventoryUnit != "袋" {
		t.Fatalf("preview quantity=%+v", item)
	}
	assertPortalProcessingMaterialNeed(t, preview.Materials, fixture.BeanID, 454, 0)
	assertPortalProcessingMaterialNeed(t, preview.Materials, fixture.BagID, 0, 2)

	created, err := repo.CreateProcessingRequest(ctx, cmd)
	if err != nil {
		t.Fatalf("CreateProcessingRequest: %v", err)
	}
	if len(created.Items) != 1 || created.Items[0].BomSpecID != fixture.BomSpecID || created.Items[0].BomVariantID != fixture.BomVariantID || created.Items[0].InventoryUnit != "袋" {
		t.Fatalf("created=%+v", created)
	}
	listed, err := repo.ListProcessingRequests(ctx, fixture.CustomerID, 10)
	if err != nil {
		t.Fatalf("ListProcessingRequests: %v", err)
	}
	if len(listed) != 1 || len(listed[0].Items) != 1 || listed[0].Items[0].ProductID != fixture.ProductID || listed[0].Items[0].BomSpecID != fixture.BomSpecID || listed[0].Items[0].BomVariantID != fixture.BomVariantID || listed[0].Items[0].InventoryUnit != "袋" {
		t.Fatalf("listed=%+v", listed)
	}
	var itemProductID, itemSpecID, itemVariantID, demandProductID, demandSpecID, demandVariantID int64
	var itemSpecName, itemUnit, demandSpecName, demandUnit string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT product_id,bom_spec_id,bom_variant_id,spec_name,inventory_unit
		FROM %s.processing_job_request_items WHERE request_id=$1
	`, schema), created.ID).Scan(&itemProductID, &itemSpecID, &itemVariantID, &itemSpecName, &itemUnit); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT product_id,bom_spec_id,bom_variant_id,spec_name,inventory_unit
		FROM %s.customer_processing_production_demands WHERE request_id=$1
	`, schema), created.ID).Scan(&demandProductID, &demandSpecID, &demandVariantID, &demandSpecName, &demandUnit); err != nil {
		t.Fatal(err)
	}
	if itemProductID != fixture.ProductID || itemSpecID != fixture.BomSpecID || itemVariantID != fixture.BomVariantID || itemSpecName != "227g袋" || itemUnit != "袋" {
		t.Fatalf("request item identity=%d/%d/%d %q %q", itemProductID, itemSpecID, itemVariantID, itemSpecName, itemUnit)
	}
	if demandProductID != fixture.ProductID || demandSpecID != fixture.BomSpecID || demandVariantID != fixture.BomVariantID || demandSpecName != "227g袋" || demandUnit != "袋" {
		t.Fatalf("demand identity=%d/%d/%d %q %q", demandProductID, demandSpecID, demandVariantID, demandSpecName, demandUnit)
	}
	assertPortalProcessingCount(t, pool, schema, "audit_logs", "entity_type='processing_job_request' AND entity_id=$1 AND action='mini_submit'", created.ID, 1)

	_, err = repo.CreateProcessingRequest(ctx, customerportalapp.CreateProcessingRequestCommand{
		CustomerID: fixture.CustomerID, CreatedByMiniUserID: 600,
		Items: []customerportalapp.ProcessingRequestItemCommand{{ProductID: fixture.ProductID, Qty: 1}},
	})
	if err == nil || !strings.Contains(err.Error(), "bom_spec_id") {
		t.Fatalf("missing spec error=%v", err)
	}
	_, err = repo.CreateProcessingRequest(ctx, customerportalapp.CreateProcessingRequestCommand{
		CustomerID: fixture.CustomerID, CreatedByMiniUserID: 600,
		Items: []customerportalapp.ProcessingRequestItemCommand{{ProductID: fixture.ProductID, BomSpecID: fixture.BomSpecID, BomVariantID: archivedVariantID, Qty: 1}},
	})
	if err == nil || !strings.Contains(err.Error(), "not published for product") {
		t.Fatalf("stale variant error=%v", err)
	}

	nonDefaultSpecID, nonDefaultVariantID := seedPortalProcessingNonDefaultPublishedSpec(t, ctx, pool, schema, fixture)
	_, err = repo.CreateProcessingRequest(ctx, customerportalapp.CreateProcessingRequestCommand{
		CustomerID: fixture.CustomerID, CreatedByMiniUserID: 600,
		Items: []customerportalapp.ProcessingRequestItemCommand{{ProductID: fixture.ProductID, BomSpecID: nonDefaultSpecID, BomVariantID: nonDefaultVariantID, Qty: 1}},
	})
	if err == nil || !strings.Contains(err.Error(), "not published for product") {
		t.Fatalf("non-default published spec error=%v", err)
	}

	legacyChildID := seedPortalProcessingLegacyChildMapping(t, ctx, pool, schema, fixture)
	_, err = repo.CreateProcessingRequest(ctx, customerportalapp.CreateProcessingRequestCommand{
		CustomerID: fixture.CustomerID, CreatedByMiniUserID: 600,
		Items: []customerportalapp.ProcessingRequestItemCommand{{ProductID: legacyChildID, SpecG: 227, Qty: 1}},
	})
	if err == nil || !strings.Contains(err.Error(), "legacy") {
		t.Fatalf("legacy child error=%v", err)
	}
	targets, err = repo.ListProcessingCatalogTargets(ctx, fixture.CustomerID, []int64{fixture.ProductID, legacyChildID})
	if err != nil {
		t.Fatalf("ListProcessingCatalogTargets after legacy mapping: %v", err)
	}
	if len(targets) != 1 || targets[0].ProductID != fixture.ProductID || targets[0].BomSpecID != fixture.BomSpecID {
		t.Fatalf("cutover catalog leaked legacy child: %+v", targets)
	}
	assertPortalProcessingCount(t, pool, schema, "processing_job_requests", "id>0 AND $1>0", created.ID, 1)
	assertPortalProcessingCount(t, pool, schema, "audit_logs", "entity_type='processing_job_request' AND entity_id=$1 AND action='mini_submit'", created.ID, 1)
}

func switchPortalProcessingBOMSpecFixtureToV2(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, fixture portalProcessingBOMSpecFixture) int64 {
	t.Helper()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_versions SET status='archived' WHERE id=$1`, schema), fixture.VersionID); err != nil {
		t.Fatal(err)
	}
	var versionID, variantID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit)
		VALUES($1,'v2','published',1,'袋') RETURNING id
	`, schema), fixture.BomID).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
		VALUES($1,$2,'227g袋','袋',true,1) RETURNING id
	`, schema), versionID, fixture.BomSpecID).Scan(&variantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_items(version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct)
		SELECT $1,$2,material_id,component_type,consume_unit,qty_per_unit,ratio_pct
		FROM %s.production_bom_version_items WHERE version_id=$3 AND variant_id=$4
	`, schema, schema), versionID, variantID, fixture.VersionID, fixture.BomVariantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_output_bindings SET bom_version_id=$1
		WHERE output_type='product' AND output_id=$2 AND is_default=true;
	`, schema), versionID, fixture.ProductID); err != nil {
		t.Fatal(err)
	}
	return variantID
}

func assertPortalProcessingMaterialNeed(t *testing.T, rows []customerportalapp.ProcessingMaterialPreview, materialID, wantG, wantUnits int64) {
	t.Helper()
	for _, row := range rows {
		if row.MaterialID == materialID {
			if row.RequiredG != wantG || row.RequiredUnits != wantUnits {
				t.Fatalf("material %d need=%d/%d want=%d/%d", materialID, row.RequiredG, row.RequiredUnits, wantG, wantUnits)
			}
			return
		}
	}
	t.Fatalf("material %d missing from %+v", materialID, rows)
}

func assertPortalProcessingCount(t *testing.T, pool *pgxpool.Pool, schema, table, where string, id int64, want int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE `+where, schema, table), id).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("count %s=%d want=%d", table, count, want)
	}
}

func seedPortalProcessingBOMSpecFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) portalProcessingBOMSpecFixture {
	t.Helper()
	if err := postgresstock.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("stock.EnsureSchema: %v", err)
	}
	if err := postgresproduction.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("production.EnsureSchema: %v", err)
	}
	if err := postgresproductspecmigration.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("productspecmigration.EnsureSchema: %v", err)
	}
	fixture := portalProcessingBOMSpecFixture{}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('PR600代加工客户') RETURNING id`, schema)).Scan(&fixture.CustomerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.warehouses(code,name,kind,is_default,active,customer_id)
		VALUES('PR600-CUSTOMER-FINISHED','PR600客户成品仓','customer_processing',true,true,$1)
	`, schema), fixture.CustomerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id,display_name,processing_warehouse_code)
		VALUES($1,'PR600代加工客户','PR600-CUSTOMER-FINISHED')
	`, schema), fixture.CustomerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name,sku_name,sku_code,product_kind,active,customer_id,visibility,custom_type)
		VALUES('PR600袋装商品','PR600袋装商品','PR600-PARENT','roasted_bean',true,0,'public','') RETURNING id
	`, schema)).Scan(&fixture.ProductID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,unit,cost_unit) VALUES('PR600-BEAN','PR600熟豆','kg','kg') RETURNING id`, schema)).Scan(&fixture.BeanID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,unit,cost_unit) VALUES('PR600-BAG','PR600袋子','个','个') RETURNING id`, schema)).Scan(&fixture.BagID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_boms(code,name,output_type,output_product_id,status)
		VALUES('BOM-PR600-PORTAL','PR600客户门户商品BOM','product',$1,'active') RETURNING id
	`, schema), fixture.ProductID).Scan(&fixture.BomID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit)
		VALUES($1,'v1','published',1,'袋') RETURNING id
	`, schema), fixture.BomID).Scan(&fixture.VersionID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_specs(bom_id,code,barcode,spec_key,name,inventory_unit)
		VALUES($1,'BSP-PR600-227','BAR-PR600-227','bag-227','227g袋','袋') RETURNING id
	`, schema), fixture.BomID).Scan(&fixture.BomSpecID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
		VALUES($1,$2,'227g袋','袋',true,1) RETURNING id
	`, schema), fixture.VersionID, fixture.BomSpecID).Scan(&fixture.BomVariantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_items(version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct)
		VALUES($1,$2,$3,'material','fixed_qty',0.227,0),($1,$2,$4,'material','fixed_qty',1,0)
	`, schema), fixture.VersionID, fixture.BomVariantID, fixture.BeanID, fixture.BagID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by)
		VALUES('product',$1,$2,$3,true,'test')
	`, schema), fixture.ProductID, fixture.BomID, fixture.VersionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,cutover_by,cutover_at)
		VALUES($1,'cutover','test',now())
	`, schema), fixture.ProductID); err != nil {
		t.Fatal(err)
	}
	seedPortalProcessingMaterialStock(t, ctx, pool, schema, fixture.BeanID, "PR600-BEAN-BATCH", 100000, 0)
	seedPortalProcessingMaterialStock(t, ctx, pool, schema, fixture.BagID, "PR600-BAG-BATCH", 0, 1000)
	return fixture
}

func seedPortalProcessingMaterialStock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, materialID int64, batchCode string, qtyG, qtyUnits int64) {
	t.Helper()
	var batchID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batches(batch_code,material_id,qty_g,qty_units,remaining_g,remaining_units,status,quality_status)
		VALUES($1,$2,$3,$4,$3,$4,'active','unchecked') RETURNING id
	`, schema), batchCode, materialID, qtyG, qtyUnits).Scan(&batchID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units)
		VALUES($1,$2,$3,'raw_materials',$4,$5)
	`, schema), batchID, batchCode, materialID, qtyG, qtyUnits); err != nil {
		t.Fatal(err)
	}
}

func seedPortalProcessingLegacyChildMapping(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, fixture portalProcessingBOMSpecFixture) int64 {
	t.Helper()
	// Legacy children existed before cutover. Temporarily model that historical
	// state because the production guard correctly rejects creating a child once
	// the parent is already cut over.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_bom_spec_migrations SET state='preparing' WHERE product_id=$1`, schema), fixture.ProductID); err != nil {
		t.Fatal(err)
	}
	var childID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name,sku_name,sku_code,parent_product_id,base_product_id,product_kind,active,customer_id,visibility,custom_type,spec_label,net_content_qty,net_content_unit)
		VALUES('PR600旧227g子SKU','PR600旧227g子SKU','OLD-PR600-227',$1,$1,'roasted_bean',true,0,'public','derived_sku','227g',227,'g') RETURNING id
	`, schema), fixture.ProductID).Scan(&childID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.legacy_child_sku_bom_spec_mappings(
			parent_product_id,legacy_child_product_id,bom_id,bom_spec_id,bom_variant_id,
			legacy_spec_key,legacy_spec_name,legacy_sales_unit,legacy_spec_g,created_by
		) VALUES($1,$2,$3,$4,$5,'bag-227','227g袋','袋',227,'test')
	`, schema), fixture.ProductID, childID, fixture.BomID, fixture.BomSpecID, fixture.BomVariantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_bom_spec_migrations SET state='cutover' WHERE product_id=$1`, schema), fixture.ProductID); err != nil {
		t.Fatal(err)
	}
	return childID
}

func seedPortalProcessingNonDefaultPublishedSpec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, fixture portalProcessingBOMSpecFixture) (int64, int64) {
	t.Helper()
	var bomID, versionID, specID, variantID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_boms(code,name,output_type,output_product_id,status)
		VALUES('BOM-PR600-PORTAL-NONDEFAULT','PR600非默认商品BOM','product',$1,'active') RETURNING id
	`, schema), fixture.ProductID).Scan(&bomID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit)
		VALUES($1,'v1','published',1,'盒') RETURNING id
	`, schema), bomID).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_specs(bom_id,code,barcode,spec_key,name,inventory_unit)
		VALUES($1,'BSP-PR600-NONDEFAULT','BAR-PR600-NONDEFAULT','gift','非默认礼盒','盒') RETURNING id
	`, schema), bomID).Scan(&specID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
		VALUES($1,$2,'非默认礼盒','盒',true,1) RETURNING id
	`, schema), versionID, specID).Scan(&variantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_items(version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct)
		VALUES($1,$2,$3,'material','fixed_qty',1,0)
	`, schema), versionID, variantID, fixture.BagID); err != nil {
		t.Fatal(err)
	}
	return specID, variantID
}
