package appmain

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	bomapp "orderapp/internal/application/bom"
	catalogapp "orderapp/internal/application/catalog"
	costingapp "orderapp/internal/application/costing"
	materialsapp "orderapp/internal/application/materials"
	creatorapp "orderapp/internal/application/productcreator"
	purchaseapp "orderapp/internal/application/purchase"
	stockapp "orderapp/internal/application/stock"
	postgresbom "orderapp/internal/infrastructure/postgres/bom"
	postgrescatalog "orderapp/internal/infrastructure/postgres/catalog"
	postgrescosting "orderapp/internal/infrastructure/postgres/costing"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"
	postgrescreator "orderapp/internal/infrastructure/postgres/productcreator"
	postgrespurchase "orderapp/internal/infrastructure/postgres/purchase"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
)

func TestProductCreatorCommitsMultiLevelGenericProductConfigurationAtomically(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for product creator business integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("test_product_creator_business_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if err := ensureAppSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_unit_definitions(code,name,unit_type,allow_decimal,active) VALUES ('件','件','other',true,true),('个','个','other',true,true),('张','张','other',true,true),('台','台','other',true,true) ON CONFLICT (code) DO NOTHING`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.purchase_suppliers(name,active) VALUES('测试供应商',true)`, schema)); err != nil {
		t.Fatal(err)
	}
	var sourcePriceListID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publications(publication_purpose,list_type,product_type_name,version_no,status,owner_type,owner_key,config_json,content_json,actor)
		VALUES('factory_supply','commercial','装配商品','V3.0.1','published','official','',
		'{"publication_batch":{"release_id":"integration-source","table_key":"assembly-main","table_name":"装配商品价目表","is_default_table":true,"direct_ship_enabled":false}}'::jsonb,
		'{"title":"装配商品价目表","groups":[],"price_rows":[]}'::jsonb,'integration-test') RETURNING id`, schema)).Scan(&sourcePriceListID); err != nil {
		t.Fatal(err)
	}

	workflow := creatorapp.Workflow{Nodes: []creatorapp.Node{
		{ID: "product", Kind: creatorapp.ModuleProduct},
		{ID: "materials", Kind: creatorapp.ModuleMaterial},
		{ID: "semi-bom", Kind: creatorapp.ModuleBOM},
		{ID: "semi-publish", Kind: creatorapp.ModulePublish},
		{ID: "assembly-bom", Kind: creatorapp.ModuleBOM},
		{ID: "assembly-publish", Kind: creatorapp.ModulePublish},
		{ID: "finished-bom", Kind: creatorapp.ModuleBOM},
		{ID: "publish", Kind: creatorapp.ModulePublish},
		{ID: "purchase", Kind: creatorapp.ModulePurchase},
		{ID: "pricing", Kind: creatorapp.ModulePricing},
	}, Edges: []creatorapp.Edge{
		productCreatorEdge("raw-to-semi-component", "materials", "material", "semi-bom", "components"),
		productCreatorEdge("semi-to-semi-output", "materials", "material", "semi-bom", "output"),
		productCreatorEdge("semi-bom-to-publish", "semi-bom", "bom", "semi-publish", "bom"),
		productCreatorEdge("housing-to-assembly-component", "semi-publish", "output", "assembly-bom", "components"),
		productCreatorEdge("parts-to-assembly-output", "materials", "material", "assembly-bom", "output"),
		productCreatorEdge("board-to-assembly-component", "materials", "material", "assembly-bom", "components"),
		productCreatorEdge("assembly-bom-to-publish", "assembly-bom", "bom", "assembly-publish", "bom"),
		productCreatorEdge("product-to-finished-output", "product", "product", "finished-bom", "output"),
		productCreatorEdge("assembly-output-to-finished-component", "assembly-publish", "output", "finished-bom", "components"),
		productCreatorEdge("materials-to-finished-component", "materials", "material", "finished-bom", "components"),
		productCreatorEdge("finished-bom-to-publish", "finished-bom", "bom", "publish", "bom"),
		productCreatorEdge("resin-to-purchase", "materials", "material", "purchase", "material"),
		productCreatorEdge("published-specs-to-pricing", "publish", "specs", "pricing", "product"),
		productCreatorEdge("receipt-to-pricing", "purchase", "receipt", "pricing", "receipt"),
	}}
	catalogSvc := catalogapp.NewService(postgrescatalog.NewRepository(pool, schema))
	materialsSvc := materialsapp.NewService(postgresmaterials.NewRepository(pool, schema))
	bomSvc := bomapp.NewService(postgresbom.NewRepository(pool, schema))
	stockSvc := stockapp.NewService(postgresstock.NewRepository(pool, schema))
	purchaseSvc := purchaseapp.NewService(postgrespurchase.NewRepository(pool, schema), stockSvc)
	costingSvc := costingapp.NewService(postgrescosting.NewRepository(pool, schema))
	creatorRepo := postgrescreator.NewRepository(pool, schema)
	creatorSvc := creatorapp.NewService(creatorRepo)
	executor := postgrescreator.NewBusinessExecutor(schema, pool, catalogSvc, materialsSvc, bomSvc, purchaseSvc, costingSvc)
	creatorSvc.UseConfigurationExecutor(executor)
	creatorSvc.UseRunStepExecutor(executor)
	template, err := creatorSvc.SaveTemplate(ctx, creatorapp.TemplateSave{Name: "非咖啡多级装配", Workflow: workflow, Actor: "integration-test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := creatorSvc.PublishTemplate(ctx, template.ID, template.Revision, "integration-test"); err != nil {
		t.Fatal(err)
	}
	run, err := creatorSvc.StartRun(ctx, template.ID, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	inputs := map[string]map[string]any{
		"product": {"name": "电控组件", "action": "create", "product_kind": "generic", "owner": "factory", "industry_fields": ""},
		"materials": {"rows": []any{
			map[string]any{"row_id": "resin", "name": "工程树脂", "action": "create", "kind": "other", "supply_mode": "purchase", "unit": "kg", "owner_type": "factory", "owner_customer_id": 0},
			map[string]any{"row_id": "housing", "name": "控制器外壳", "action": "create", "kind": "other", "supply_mode": "manufacture", "unit": "件", "owner_type": "factory", "owner_customer_id": 0},
			map[string]any{"row_id": "driver", "name": "控制器驱动模块", "action": "create", "kind": "other", "supply_mode": "manufacture", "unit": "件", "owner_type": "factory", "owner_customer_id": 0},
			map[string]any{"row_id": "board", "name": "控制板", "action": "create", "kind": "other", "supply_mode": "purchase", "unit": "个", "owner_type": "factory", "owner_customer_id": 0},
			map[string]any{"row_id": "label", "name": "铭牌", "action": "create", "kind": "pack", "supply_mode": "purchase", "unit": "张", "owner_type": "factory", "owner_customer_id": 0},
		}},
		"semi-bom":     {"action": "create", "output_source_row_id": "housing", "variants": []any{productCreatorVariant("housing-spec", "外壳", "件", true)}, "components": []any{productCreatorComponent("resin-use", "materials", "resin", "", 0.15, "kg")}},
		"semi-publish": {"set_default": true},
		"assembly-bom": {"action": "create", "output_source_row_id": "driver", "variants": []any{productCreatorVariant("driver-spec", "驱动模块", "件", true)}, "components": []any{
			productCreatorComponent("housing-use", "semi-publish", "output", "", 1, "件"),
			productCreatorComponent("board-use", "materials", "board", "", 1, "个"),
		}},
		"assembly-publish": {"set_default": true},
		"finished-bom": {"action": "create", "output_source_row_id": "product", "variants": []any{productCreatorVariant("controller", "控制器", "台", true), productCreatorVariant("controller-set", "控制器套装", "个", false)}, "components": []any{
			productCreatorComponent("driver-use", "assembly-publish", "output", "", 1, "件"),
			productCreatorComponent("label-use", "materials", "label", "", 1, "张"),
		}},
		"publish":  {"set_default": true},
		"purchase": {"material_source_row_id": "resin", "supplier_id": 1, "quantity": 10, "warehouse": "raw_materials", "unit_price": 12.5},
		"pricing": {"price_list_id": sourcePriceListID, "prices": []any{
			map[string]any{"row_id": "controller-price", "spec_row_id": "controller", "pricing_mode": "fixed", "price": 99},
			map[string]any{"row_id": "controller-set-price", "spec_row_id": "controller-set", "pricing_mode": "fixed", "price": 149},
		}},
	}
	run, err = creatorSvc.SaveRunInputs(ctx, run.ID, run.Revision, inputs, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	previewCountsBefore, err := productCreatorBusinessCounts(ctx, pool, schema)
	if err != nil {
		t.Fatal(err)
	}
	run, err = creatorSvc.PreviewRun(ctx, run.ID, run.Revision, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	if run.Preview == nil || !run.Preview.Valid {
		t.Fatalf("generic multi-level assembly preview failed: %+v", run.Preview)
	}
	previewCountsAfter, err := productCreatorBusinessCounts(ctx, pool, schema)
	if err != nil {
		t.Fatal(err)
	}
	if previewCountsAfter != previewCountsBefore {
		t.Fatalf("business preview must not create products/materials/BOMs/purchase records/price publications: before=%v after=%v", previewCountsBefore, previewCountsAfter)
	}
	run, err = creatorSvc.CommitConfiguration(ctx, run.ID, run.Revision, "integration-product-creator-run", "integration-test")
	if err != nil {
		if executionErr, ok := err.(creatorapp.ExecutionError); ok {
			t.Fatalf("configuration commit issues: %+v", executionErr.Issues)
		}
		t.Fatal(err)
	}
	if run.Status != "in_progress" {
		t.Fatalf("run status=%q, want in_progress while purchase receipt is pending", run.Status)
	}
	var resinID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.materials WHERE name='工程树脂'`, schema)).Scan(&resinID); err != nil {
		t.Fatal(err)
	}
	var beforeReceiptPrice float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT purchase_price FROM %s.materials WHERE id=$1`, schema), resinID).Scan(&beforeReceiptPrice); err != nil {
		t.Fatal(err)
	}
	if beforeReceiptPrice != 0 {
		t.Fatalf("purchase order should not change actual material cost before receipt, got %.2f", beforeReceiptPrice)
	}
	createOrderRevision := run.Revision
	run, err = creatorSvc.ExecuteRunStep(ctx, run.ID, run.Revision, "purchase", "create_purchase_order", "integration-run-purchase-order", "integration-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "in_progress" {
		t.Fatalf("run status=%q after purchase order, want in_progress until receipt", run.Status)
	}
	var purchaseOrderCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.purchase_orders WHERE material_id=$1 AND status='ordered'`, schema), resinID).Scan(&purchaseOrderCount); err != nil {
		t.Fatal(err)
	}
	if purchaseOrderCount != 1 {
		t.Fatalf("purchase order count=%d, want 1", purchaseOrderCount)
	}
	if _, err := creatorSvc.ExecuteRunStep(ctx, run.ID, createOrderRevision, "purchase", "create_purchase_order", "integration-run-purchase-order", "integration-test", nil); err != nil {
		t.Fatalf("retrying lost purchase order response should return original run: %v", err)
	}
	receiptRevision := run.Revision
	run, err = creatorSvc.ExecuteRunStep(ctx, run.ID, run.Revision, "purchase", "confirm_receipt", "integration-run-purchase-receipt", "integration-test", map[string]any{"quantity": 9.5, "unit_price": 13.25})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "in_progress" {
		t.Fatalf("run status=%q after receipt, want in_progress until pricing publication", run.Status)
	}
	if _, err := creatorSvc.ExecuteRunStep(ctx, run.ID, receiptRevision, "purchase", "confirm_receipt", "integration-run-purchase-receipt", "integration-test", map[string]any{"quantity": 9.5, "unit_price": 13.25}); err != nil {
		t.Fatalf("retrying lost receipt response should return original run: %v", err)
	}
	var actualPrice float64
	var receiptCount int
	var batchCode string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT purchase_price FROM %s.materials WHERE id=$1`, schema), resinID).Scan(&actualPrice); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*),MAX(stock_batch_code) FROM %s.purchase_receipts WHERE material_id=$1`, schema), resinID).Scan(&receiptCount, &batchCode); err != nil {
		t.Fatal(err)
	}
	if actualPrice != 13.25 || receiptCount != 1 || strings.TrimSpace(batchCode) == "" {
		t.Fatalf("receipt result price/count/batch=%v/%d/%q, want 13.25/1/assigned batch", actualPrice, receiptCount, batchCode)
	}
	pricingRevision := run.Revision
	run, err = creatorSvc.ExecuteRunStep(ctx, run.ID, run.Revision, "pricing", "preview_pricing", "integration-run-price-preview", "integration-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	steps, _ := run.BusinessResults["steps"].(map[string]any)
	pricingStep, _ := steps["pricing"].(map[string]any)
	pricingPreview, _ := pricingStep["pricing_preview"].(map[string]any)
	if pricingPreview["valid"] != true {
		t.Fatalf("pricing preview should be valid after purchase receipt: %#v", pricingPreview)
	}
	draftRevision := run.Revision
	run, err = creatorSvc.ExecuteRunStep(ctx, run.ID, run.Revision, "pricing", "save_price_draft", "integration-run-price-draft", "integration-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "in_progress" {
		t.Fatalf("run status=%q after price draft, want in_progress until publication", run.Status)
	}
	if _, err := creatorSvc.ExecuteRunStep(ctx, run.ID, pricingRevision, "pricing", "preview_pricing", "integration-run-price-preview", "integration-test", nil); err != nil {
		t.Fatalf("retrying lost price preview response should return original run: %v", err)
	}
	if _, err := creatorSvc.ExecuteRunStep(ctx, run.ID, draftRevision, "pricing", "save_price_draft", "integration-run-price-draft", "integration-test", nil); err != nil {
		t.Fatalf("retrying lost price draft response should return original run: %v", err)
	}
	publishRevision := run.Revision
	run, err = creatorSvc.ExecuteRunStep(ctx, run.ID, run.Revision, "pricing", "publish_price", "integration-run-price-publish", "integration-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "completed" {
		t.Fatalf("run status=%q after price publication, want completed", run.Status)
	}
	if _, err := creatorSvc.ExecuteRunStep(ctx, run.ID, publishRevision, "pricing", "publish_price", "integration-run-price-publish", "integration-test", nil); err != nil {
		t.Fatalf("retrying lost price publication response should return original run: %v", err)
	}
	var priceDrafts, publishedPriceLists int
	var publishedPriceVersion, publishedTableName, publishedTableKey string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FILTER (WHERE status='draft'),COUNT(*) FILTER (WHERE status='published'),MAX(version_no) FILTER (WHERE status='published'),MAX(config_json->'publication_batch'->>'table_name') FILTER (WHERE status='published'),MAX(config_json->'publication_batch'->>'table_key') FILTER (WHERE status='published') FROM %s.bean_list_publications WHERE price_source_publication_id=$1 AND actor='integration-test'`, schema), sourcePriceListID).Scan(&priceDrafts, &publishedPriceLists, &publishedPriceVersion, &publishedTableName, &publishedTableKey); err != nil {
		t.Fatal(err)
	}
	if priceDrafts != 1 || publishedPriceLists != 1 || publishedPriceVersion == "" || publishedTableName != "装配商品价目表" || publishedTableKey != "assembly-main" {
		t.Fatalf("price draft/publication/table/version=%d/%d/%q/%q/%q", priceDrafts, publishedPriceLists, publishedPriceVersion, publishedTableName, publishedTableKey)
	}
	var publishedPriceRows int
	var lowestPublishedPrice, highestPublishedPrice float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*),MIN((price_row->>'final_unit_price')::float8),MAX((price_row->>'final_unit_price')::float8) FROM %s.bean_list_publications publication CROSS JOIN LATERAL jsonb_array_elements(publication.content_json->'price_rows') price_row WHERE publication.price_source_publication_id=$1 AND publication.status='published' AND price_row->>'product_name'='电控组件'`, schema), sourcePriceListID).Scan(&publishedPriceRows, &lowestPublishedPrice, &highestPublishedPrice); err != nil {
		t.Fatal(err)
	}
	if publishedPriceRows != 2 || lowestPublishedPrice != 99 || highestPublishedPrice != 149 {
		t.Fatalf("published specs and fixed price rows=%d/%v/%v, want 2/99/149", publishedPriceRows, lowestPublishedPrice, highestPublishedPrice)
	}
	var productID, productBomID, materialBomID, assemblyBomID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.products WHERE name='电控组件'`, schema)).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_boms WHERE output_type='product' AND output_product_id=$1`, schema), productID).Scan(&productBomID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_boms WHERE output_type='material' AND output_material_id=(SELECT id FROM %s.materials WHERE name='控制器外壳')`, schema, schema)).Scan(&materialBomID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_boms WHERE output_type='material' AND output_material_id=(SELECT id FROM %s.materials WHERE name='控制器驱动模块')`, schema, schema)).Scan(&assemblyBomID); err != nil {
		t.Fatal(err)
	}
	var publishedSpecs, publishedVersions, defaultBindings int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_specs WHERE bom_id=$1`, schema), productBomID).Scan(&publishedSpecs); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_versions WHERE bom_id IN ($1,$2,$3) AND status='published'`, schema), productBomID, materialBomID, assemblyBomID).Scan(&publishedVersions); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_output_bindings WHERE is_default=true AND ((output_type='product' AND output_id=$1 AND bom_id=$2) OR (output_type='material' AND output_id=(SELECT id FROM %s.materials WHERE name='控制器外壳') AND bom_id=$3) OR (output_type='material' AND output_id=(SELECT id FROM %s.materials WHERE name='控制器驱动模块') AND bom_id=$4))`, schema, schema, schema), productID, productBomID, materialBomID, assemblyBomID).Scan(&defaultBindings); err != nil {
		t.Fatal(err)
	}
	if publishedSpecs != 2 || publishedVersions != 3 || defaultBindings != 3 {
		t.Fatalf("published spec/version/default binding counts=%d/%d/%d, want 2/3/3", publishedSpecs, publishedVersions, defaultBindings)
	}
	var runAuditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='product_creator_run' AND entity_id=$1 AND action='commit_configuration'`, schema), run.ID).Scan(&runAuditCount); err != nil {
		t.Fatal(err)
	}
	if runAuditCount != 1 {
		t.Fatalf("configuration commit audit count=%d, want 1", runAuditCount)
	}

	// A second published template references the semi-finished material and
	// packaging from the first run. It must create only the new product BOM.
	var housingID, labelID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.materials WHERE name='控制器外壳'`, schema)).Scan(&housingID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.materials WHERE name='铭牌'`, schema)).Scan(&labelID); err != nil {
		t.Fatal(err)
	}
	var materialsBefore, bomsBefore, batchesBefore, bindingsBefore int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.materials`, schema)).Scan(&materialsBefore); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms`, schema)).Scan(&bomsBefore); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.material_batches`, schema)).Scan(&batchesBefore); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_output_bindings WHERE output_type='material' AND output_id=$1 AND is_default=true`, schema), housingID).Scan(&bindingsBefore); err != nil {
		t.Fatal(err)
	}
	reuseWorkflow := creatorapp.Workflow{Nodes: []creatorapp.Node{
		{ID: "product", Kind: creatorapp.ModuleProduct}, {ID: "parts", Kind: creatorapp.ModuleMaterial},
		{ID: "bom", Kind: creatorapp.ModuleBOM}, {ID: "publish", Kind: creatorapp.ModulePublish},
	}, Edges: []creatorapp.Edge{
		productCreatorEdge("product-output", "product", "product", "bom", "output"),
		productCreatorEdge("reused-parts", "parts", "material", "bom", "components"),
		productCreatorEdge("bom-publish", "bom", "bom", "publish", "bom"),
	}}
	reuseTemplate, err := creatorSvc.SaveTemplate(ctx, creatorapp.TemplateSave{Name: "复用半成品与包材", Workflow: reuseWorkflow, Actor: "integration-test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := creatorSvc.PublishTemplate(ctx, reuseTemplate.ID, reuseTemplate.Revision, "integration-test"); err != nil {
		t.Fatal(err)
	}
	reuseRun, err := creatorSvc.StartRun(ctx, reuseTemplate.ID, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	reuseInputs := map[string]map[string]any{
		"product": {"name": "控制器备件包", "action": "create", "product_kind": "generic", "owner": "factory"},
		"parts": {"rows": []any{
			map[string]any{"row_id": "existing-housing", "name": "控制器外壳", "action": "reuse", "material_id": housingID, "kind": "other", "supply_mode": "manufacture", "unit": "件", "owner_type": "factory", "owner_customer_id": 0},
			map[string]any{"row_id": "existing-label", "name": "铭牌", "action": "reuse", "material_id": labelID, "kind": "pack", "supply_mode": "purchase", "unit": "张", "owner_type": "factory", "owner_customer_id": 0},
		}},
		"bom": {"action": "create", "output_source_row_id": "product", "variants": []any{productCreatorVariant("spare-kit", "备件套装", "台", true)}, "components": []any{
			productCreatorComponent("housing-component", "parts", "existing-housing", "", 1, "件"),
			productCreatorComponent("label-component", "parts", "existing-label", "", 1, "张"),
		}},
		"publish": {"set_default": true},
	}
	reuseRun, err = creatorSvc.SaveRunInputs(ctx, reuseRun.ID, reuseRun.Revision, reuseInputs, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	reuseRun, err = creatorSvc.PreviewRun(ctx, reuseRun.ID, reuseRun.Revision, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	if reuseRun.Preview == nil || !reuseRun.Preview.Valid {
		t.Fatalf("reuse preview failed: %+v", reuseRun.Preview)
	}
	reuseRun, err = creatorSvc.CommitConfiguration(ctx, reuseRun.ID, reuseRun.Revision, "integration-reuse-semi-and-package", "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	if reuseRun.Status != "config_committed" {
		t.Fatalf("reuse run status=%q, want config_committed", reuseRun.Status)
	}
	var materialsAfter, bomsAfter, batchesAfter, bindingsAfter int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.materials`, schema)).Scan(&materialsAfter); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms`, schema)).Scan(&bomsAfter); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.material_batches`, schema)).Scan(&batchesAfter); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_output_bindings WHERE output_type='material' AND output_id=$1 AND is_default=true`, schema), housingID).Scan(&bindingsAfter); err != nil {
		t.Fatal(err)
	}
	var reusedComponentCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_version_items item JOIN %s.production_bom_versions version ON version.id=item.version_id JOIN %s.production_boms bom ON bom.id=version.bom_id WHERE bom.output_type='product' AND bom.output_product_id=(SELECT id FROM %s.products WHERE name='控制器备件包') AND item.material_id IN ($1,$2)`, schema, schema, schema, schema), housingID, labelID).Scan(&reusedComponentCount); err != nil {
		t.Fatal(err)
	}
	if materialsAfter != materialsBefore || bomsAfter != bomsBefore+1 || batchesAfter != batchesBefore || bindingsAfter != bindingsBefore || reusedComponentCount != 2 {
		t.Fatalf("reuse counts materials=%d/%d boms=%d/%d batches=%d/%d bindings=%d/%d components=%d; expected only one new product BOM with both existing materials", materialsBefore, materialsAfter, bomsBefore, bomsAfter, batchesBefore, batchesAfter, bindingsBefore, bindingsAfter, reusedComponentCount)
	}
}

func TestProductCreatorBOMCentricCommitsMaterialThenMultiSpecProduct(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for product creator business integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("test_product_creator_bom_v2_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if err := ensureAppSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_definitions(code,name,unit_type,allow_decimal,active)
		VALUES ('kg','千克','weight',true,true),('个','个','package',false,true),('袋','袋','package',false,true)
		ON CONFLICT (code) DO UPDATE SET name=excluded.name,unit_type=excluded.unit_type,allow_decimal=excluded.allow_decimal,active=true,deleted_at=NULL
	`, schema)); err != nil {
		t.Fatal(err)
	}
	var roastRouteID, packRouteID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.process_routes(name,status) VALUES('半成品烘焙','active') RETURNING id`, schema)).Scan(&roastRouteID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.process_routes(name,status) VALUES('成品包装','active') RETURNING id`, schema)).Scan(&packRouteID); err != nil {
		t.Fatal(err)
	}

	workflow := creatorapp.Workflow{Version: 3, Variables: []creatorapp.WorkflowVariable{{ID: "product-name", Name: "商品名称", DefaultValue: "坚果风味拼配咖啡"}}, Nodes: []creatorapp.Node{
		{ID: "raw", Kind: creatorapp.ModuleMaterial, Config: map[string]any{"data_role": "input"}},
		{ID: "semi-bom", Kind: creatorapp.ModuleBOM, Config: map[string]any{
			"output_type": "material", "output_qty": 10.0, "output_unit": "kg", "route_id": float64(0), "material_loss_rate": 0.04,
			"components": []any{map[string]any{"row_id": "raw-default", "source_node_id": "raw", "quantity": 100.0, "unit": "ratio_pct"}},
		}},
		{ID: "semi", Kind: creatorapp.ModuleMaterial, Config: map[string]any{"data_role": "output", "object_action": "create", "name_parts": []any{map[string]any{"type": "variable", "variable_id": "product-name"}, map[string]any{"type": "text", "value": " 半成品"}}, "supply_mode": "manufacture"}},
		{ID: "roast-route", Kind: creatorapp.ModuleProcess, Config: map[string]any{"route_id": roastRouteID}},
		{ID: "pack", Kind: creatorapp.ModuleMaterial, Config: map[string]any{"data_role": "input"}},
		{ID: "finished-bom", Kind: creatorapp.ModuleBOM, Config: map[string]any{
			"output_type": "product", "output_qty": 1.0, "output_unit": "", "route_id": float64(0), "material_loss_rate": 0.0,
			"variants": []any{productCreatorVariant("bag-200", "200g", "袋", true), productCreatorVariant("bag-500", "500g", "袋", false)},
			"components": []any{
				map[string]any{"row_id": "semi-default", "source_node_id": "semi", "quantity": 0.2, "unit": "kg"},
				map[string]any{"row_id": "pack-default", "source_node_id": "pack", "quantity": 1.0, "unit": "个"},
			},
		}},
		{ID: "product", Kind: creatorapp.ModuleProduct, Config: map[string]any{"data_role": "output", "object_action": "create", "name_parts": []any{map[string]any{"type": "variable", "variable_id": "product-name"}}, "owner": "factory"}},
		{ID: "pack-route", Kind: creatorapp.ModuleProcess, Config: map[string]any{"route_id": packRouteID}},
	}, Edges: []creatorapp.Edge{
		productCreatorEdge("raw-to-semi", "raw", "material", "semi-bom", "components"),
		productCreatorEdge("roast-to-semi", "roast-route", "route", "semi-bom", "route"),
		productCreatorEdge("semi-to-output", "semi-bom", "assembly", "semi", "from_bom"),
		productCreatorEdge("semi-to-finished", "semi", "material", "finished-bom", "components"),
		productCreatorEdge("pack-to-finished", "pack", "material", "finished-bom", "components"),
		productCreatorEdge("pack-route-to-finished", "pack-route", "route", "finished-bom", "route"),
		productCreatorEdge("finished-to-product", "finished-bom", "assembly", "product", "from_bom"),
	}}
	catalogSvc := catalogapp.NewService(postgrescatalog.NewRepository(pool, schema))
	materialsSvc := materialsapp.NewService(postgresmaterials.NewRepository(pool, schema))
	bomSvc := bomapp.NewService(postgresbom.NewRepository(pool, schema))
	creatorSvc := creatorapp.NewService(postgrescreator.NewRepository(pool, schema))
	creatorSvc.UseConfigurationExecutor(postgrescreator.NewBusinessExecutor(schema, pool, catalogSvc, materialsSvc, bomSvc, nil, nil))
	template, err := creatorSvc.SaveTemplate(ctx, creatorapp.TemplateSave{Name: "BOM中心物料到商品", Workflow: workflow, Actor: "integration-test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := creatorSvc.PublishTemplate(ctx, template.ID, template.Revision, "integration-test"); err != nil {
		t.Fatal(err)
	}
	run, err := creatorSvc.StartRun(ctx, template.ID, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	inputs := map[string]map[string]any{
		"raw":         {"rows": []any{map[string]any{"row_id": "raw-row", "name": "浅焙拼配原料", "action": "create", "kind": "bean", "supply_mode": "purchase", "unit": "kg", "owner_type": "factory"}}},
		"semi-bom":    {"output_qty": 12.0, "output_unit": "kg", "material_loss_rate": 0.04, "components": []any{map[string]any{"row_id": "raw-run", "source_node_id": "raw", "source_row_id": "raw-row", "quantity": 100.0, "unit": "ratio_pct"}}},
		"semi":        {"action": "create", "name": "", "name_mode": "automatic", "kind": "bean", "supply_mode": "manufacture", "owner_type": "factory"},
		"roast-route": {"route_id": roastRouteID},
		"pack":        {"rows": []any{map[string]any{"row_id": "pack-row", "name": "复合铝箔袋", "action": "create", "kind": "pack", "supply_mode": "purchase", "unit": "个", "owner_type": "factory"}}},
		"finished-bom": {"output_qty": 1.0, "variants": []any{productCreatorVariant("bag-200", "200g", "袋", true), productCreatorVariant("bag-500", "500g", "袋", false)}, "components": []any{
			productCreatorComponent("semi-200", "semi", "output", "bag-200", 0.2, "kg"), productCreatorComponent("pack-200", "pack", "pack-row", "bag-200", 1, "个"),
			productCreatorComponent("semi-500", "semi", "output", "bag-500", 0.5, "kg"), productCreatorComponent("pack-500", "pack", "pack-row", "bag-500", 1, "个"),
		}},
		"product":    {"name": "", "name_mode": "automatic", "action": "create", "product_kind": "roasted", "owner": "factory"},
		"pack-route": {"route_id": packRouteID},
	}
	run, err = creatorSvc.SaveRunDraft(ctx, run.ID, run.Revision, inputs, map[string]string{"product-name": "坚果风味拼配咖啡"}, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	if run.VariableValues["product-name"] != "坚果风味拼配咖啡" || run.Inputs["semi"]["name"] != "坚果风味拼配咖啡半成品" || run.Inputs["product"]["name"] != "坚果风味拼配咖啡" {
		t.Fatalf("V3 draft must persist one shared variable and resolve both output names: version=%d semi_parts=%#v product_parts=%#v vars=%v semi=%v product=%v", run.Workflow.Version, run.Workflow.Nodes[2].Config["name_parts"], run.Workflow.Nodes[6].Config["name_parts"], run.VariableValues, run.Inputs["semi"], run.Inputs["product"])
	}
	countsBefore, err := productCreatorBusinessCounts(ctx, pool, schema)
	if err != nil {
		t.Fatal(err)
	}
	run, err = creatorSvc.PreviewRun(ctx, run.ID, run.Revision, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	if run.Preview == nil || !run.Preview.Valid {
		t.Fatalf("BOM-centric preview failed: %+v", run.Preview)
	}
	countsAfterPreview, err := productCreatorBusinessCounts(ctx, pool, schema)
	if err != nil {
		t.Fatal(err)
	}
	if countsAfterPreview != countsBefore {
		t.Fatalf("preview created business data: before=%v after=%v", countsBefore, countsAfterPreview)
	}
	run, err = creatorSvc.CommitConfiguration(ctx, run.ID, run.Revision, "bom-centric-integration-run", "integration-test")
	if err != nil {
		if executionErr, ok := err.(creatorapp.ExecutionError); ok {
			t.Fatalf("BOM-centric commit issues: %+v", executionErr.Issues)
		}
		t.Fatal(err)
	}
	if run.Status != "config_committed" {
		t.Fatalf("status=%q, want config_committed", run.Status)
	}
	var materialCount, productCount, bomCount, semiID, productID int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.materials`, schema)).Scan(&materialCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.products`, schema)).Scan(&productCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms`, schema)).Scan(&bomCount); err != nil {
		t.Fatal(err)
	}
	if materialCount != 3 || productCount != 1 || bomCount != 2 {
		t.Fatalf("created material/product/BOM counts=%d/%d/%d, want 3/1/2", materialCount, productCount, bomCount)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.materials WHERE name='坚果风味拼配咖啡半成品'`, schema)).Scan(&semiID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.products WHERE name='坚果风味拼配咖啡'`, schema)).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	var semiQty, semiLoss float64
	var semiUnit, semiStatus string
	var semiRoute int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT version.output_qty,version.output_unit,version.material_loss_rate,version.process_route_id,version.status FROM %s.production_boms bom JOIN %s.production_bom_versions version ON version.bom_id=bom.id WHERE bom.output_type='material' AND bom.output_material_id=$1`, schema, schema), semiID).Scan(&semiQty, &semiUnit, &semiLoss, &semiRoute, &semiStatus); err != nil {
		t.Fatal(err)
	}
	if semiQty != 12 || semiUnit != "kg" || semiLoss != 0.04 || semiRoute != roastRouteID || semiStatus != "published" {
		t.Fatalf("semi BOM qty/unit/loss/route/status=%v/%s/%v/%d/%s", semiQty, semiUnit, semiLoss, semiRoute, semiStatus)
	}
	var materialKinds int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.materials WHERE kind <> 'other'`, schema)).Scan(&materialKinds); err != nil {
		t.Fatal(err)
	}
	var productKind string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT product_kind FROM %s.products WHERE id=$1`, schema), productID).Scan(&productKind); err != nil {
		t.Fatal(err)
	}
	if materialKinds != 0 || productKind != "generic" {
		t.Fatalf("V3 forged category values should resolve to neutral defaults: nonneutral_materials=%d product_kind=%q", materialKinds, productKind)
	}
	var variantCount, defaultVariantCount, finishedRouteCount, productDefaultBindingCount, semiDefaultBindingCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*),COUNT(*) FILTER (WHERE variant.is_default) FROM %s.production_boms bom JOIN %s.production_bom_versions version ON version.bom_id=bom.id AND version.status='published' JOIN %s.production_bom_version_variants variant ON variant.version_id=version.id WHERE bom.output_type='product' AND bom.output_product_id=$1`, schema, schema, schema), productID).Scan(&variantCount, &defaultVariantCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT process_route_id FROM %s.production_boms bom JOIN %s.production_bom_versions version ON version.bom_id=bom.id AND version.status='published' WHERE bom.output_type='product' AND bom.output_product_id=$1`, schema, schema), productID).Scan(&finishedRouteCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_output_bindings WHERE output_type='product' AND output_id=$1 AND is_default=true`, schema), productID).Scan(&productDefaultBindingCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_output_bindings WHERE output_type='material' AND output_id=$1 AND is_default=true`, schema), semiID).Scan(&semiDefaultBindingCount); err != nil {
		t.Fatal(err)
	}
	if variantCount != 2 || defaultVariantCount != 1 || finishedRouteCount != int(packRouteID) || productDefaultBindingCount != 1 || semiDefaultBindingCount != 1 {
		t.Fatalf("variants/default/route/product binding/semi binding=%d/%d/%d/%d/%d", variantCount, defaultVariantCount, finishedRouteCount, productDefaultBindingCount, semiDefaultBindingCount)
	}
	verifyV8ReuseDownstream(t, ctx, pool, schema, creatorSvc, bomSvc, semiID, productID, packRouteID)
}

func TestProductCreatorConfigurationRollsBackEarlierObjectsAfterCommitValidationFailure(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for product creator business integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("test_product_creator_rollback_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if err := ensureAppSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_unit_definitions(code,name,unit_type,allow_decimal,active) VALUES ('件','件','other',true,true) ON CONFLICT (code) DO UPDATE SET active=true,deleted_at=NULL`, schema)); err != nil {
		t.Fatal(err)
	}
	workflow := creatorapp.Workflow{Nodes: []creatorapp.Node{
		{ID: "product", Kind: creatorapp.ModuleProduct}, {ID: "materials", Kind: creatorapp.ModuleMaterial},
		{ID: "bom", Kind: creatorapp.ModuleBOM}, {ID: "publish", Kind: creatorapp.ModulePublish},
	}, Edges: []creatorapp.Edge{
		productCreatorEdge("product-output", "product", "product", "bom", "output"),
		productCreatorEdge("material-component", "materials", "material", "bom", "components"),
		productCreatorEdge("bom-publish", "bom", "bom", "publish", "bom"),
	}}
	catalogSvc := catalogapp.NewService(postgrescatalog.NewRepository(pool, schema))
	materialsSvc := materialsapp.NewService(postgresmaterials.NewRepository(pool, schema))
	bomSvc := bomapp.NewService(postgresbom.NewRepository(pool, schema))
	creatorSvc := creatorapp.NewService(postgrescreator.NewRepository(pool, schema))
	creatorSvc.UseConfigurationExecutor(postgrescreator.NewBusinessExecutor(schema, pool, catalogSvc, materialsSvc, bomSvc, nil, nil))
	template, err := creatorSvc.SaveTemplate(ctx, creatorapp.TemplateSave{Name: "提交时单位失效回滚", Workflow: workflow, Actor: "integration-test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := creatorSvc.PublishTemplate(ctx, template.ID, template.Revision, "integration-test"); err != nil {
		t.Fatal(err)
	}
	run, err := creatorSvc.StartRun(ctx, template.ID, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	inputs := map[string]map[string]any{
		"product": {"name": "必须整体回滚商品", "action": "create", "product_kind": "generic", "owner": "factory"},
		"materials": {"rows": []any{
			map[string]any{"row_id": "raw", "name": "必须整体回滚原料", "action": "create", "kind": "other", "supply_mode": "purchase", "unit": "kg", "owner_type": "factory", "owner_customer_id": 0},
			map[string]any{"row_id": "invalid-unit", "name": "失效单位物料", "action": "create", "kind": "other", "supply_mode": "purchase", "unit": "件", "owner_type": "factory", "owner_customer_id": 0},
		}},
		"bom":     {"action": "create", "output_source_row_id": "product", "variants": []any{productCreatorVariant("rollback-spec", "测试规格", "盒", true)}, "components": []any{productCreatorComponent("raw-component", "materials", "raw", "", 1, "kg")}},
		"publish": {"set_default": true},
	}
	run, err = creatorSvc.SaveRunInputs(ctx, run.ID, run.Revision, inputs, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	run, err = creatorSvc.PreviewRun(ctx, run.ID, run.Revision, "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	if run.Preview == nil || !run.Preview.Valid {
		t.Fatalf("pre-change preview should be valid: %+v", run.Preview)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_unit_definitions SET active=false,deleted_at=now() WHERE code='件'`, schema)); err != nil {
		t.Fatal(err)
	}
	_, err = creatorSvc.CommitConfiguration(ctx, run.ID, run.Revision, "integration-rollback-run", "integration-test")
	executionErr, ok := err.(creatorapp.ExecutionError)
	if !ok || len(executionErr.Issues) == 0 || executionErr.Issues[0].NodeID != "materials" {
		t.Fatalf("commit error=%v, want material validation failure after product/first material creation", err)
	}
	var productCount, materialCount, commitAuditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.products WHERE name='必须整体回滚商品'`, schema)).Scan(&productCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.materials WHERE name IN ('必须整体回滚原料','失效单位物料')`, schema)).Scan(&materialCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='product_creator_run' AND entity_id=$1 AND action='commit_configuration'`, schema), run.ID).Scan(&commitAuditCount); err != nil {
		t.Fatal(err)
	}
	if productCount != 0 || materialCount != 0 || commitAuditCount != 0 {
		t.Fatalf("failed config transaction left product/material/audit=%d/%d/%d; all must roll back", productCount, materialCount, commitAuditCount)
	}
}

func productCreatorEdge(id, source, sourceHandle, target, targetHandle string) creatorapp.Edge {
	return creatorapp.Edge{ID: id, Source: source, SourceHandle: sourceHandle, Target: target, TargetHandle: targetHandle, Kind: creatorapp.EdgeData}
}

func productCreatorVariant(id, name, unit string, isDefault bool) map[string]any {
	return map[string]any{"row_id": id, "name": name, "unit": unit, "is_default": isDefault}
}

func productCreatorBusinessCounts(ctx context.Context, pool *pgxpool.Pool, schema string) ([6]int, error) {
	var counts [6]int
	query := fmt.Sprintf(`SELECT
		(SELECT COUNT(*) FROM %s.products),
		(SELECT COUNT(*) FROM %s.materials),
		(SELECT COUNT(*) FROM %s.production_boms),
		(SELECT COUNT(*) FROM %s.purchase_orders),
		(SELECT COUNT(*) FROM %s.purchase_receipts),
		(SELECT COUNT(*) FROM %s.bean_list_publications)`, schema, schema, schema, schema, schema, schema)
	if err := pool.QueryRow(ctx, query).Scan(&counts[0], &counts[1], &counts[2], &counts[3], &counts[4], &counts[5]); err != nil {
		return counts, err
	}
	return counts, nil
}

func productCreatorComponent(id, source, sourceRow, variant string, quantity float64, unit string) map[string]any {
	return map[string]any{"row_id": id, "source_node_id": source, "source_row_id": sourceRow, "variant_row_id": variant, "quantity": quantity, "unit": unit, "loss_rate": 0}
}
