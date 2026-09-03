package bom

import (
	"fmt"
	"strings"
	"testing"

	bomapp "orderapp/internal/application/bom"
)

func TestManualProductBOMSpecGroupPublishesWithoutTemplateProvenancePostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	ensurePR600ProductPublishTestTables(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.product_unit_definitions(code) VALUES('盒'),('条') ON CONFLICT(code) DO NOTHING;
		INSERT INTO %[1]s.products(id,name,active) VALUES(1001,'手工规格速溶商品',true);
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,purchase_price)
		VALUES(1101,'PR625-INSTANT','速溶咖啡条','finished','条','条',1);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	created, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "商品-速溶-黑咖", OutputType: "product", SpecificationMode: bomapp.ProductionBomSpecificationModeSpecGroup,
		OutputID: 1001, OutputProductID: 1001, OutputQty: 1, OutputUnit: "盒", Actor: "pr625-auditor",
		Variants: []bomapp.ProductionBomDraftVariant{
			{
				SpecKey: "box", Name: "盒装", InventoryUnit: "盒", IsDefault: true, SortOrder: 1,
				Items: []bomapp.ProductionBomDraftItem{{MaterialID: 1101, ComponentType: "material", ConsumeUnit: "条", QtyPerUnit: 10}},
			},
			{
				SpecKey: "stick", Name: "条装", InventoryUnit: "条", SortOrder: 2,
				Items: []bomapp.ProductionBomDraftItem{{MaterialID: 1101, ComponentType: "material", ConsumeUnit: "条", QtyPerUnit: 1}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := publishProductionBOMVersionForMaintenanceTest(ctx, pool, repo, created.LatestVersionID, "pr625-auditor"); err != nil {
		t.Fatalf("publish manual specification group: %v", err)
	}
	if _, err := repo.BindProductProductionBom(ctx, bomapp.BindProductProductionBomCommand{
		ProductID: 1001, BomID: created.ID, Actor: "pr625-auditor",
	}); err != nil {
		t.Fatalf("bind manual specification group as product default BOM: %v", err)
	}

	var status string
	var templateVersionID, mainInputMaterialID, bindingCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT status,COALESCE(source_spec_template_version_id,0),COALESCE(main_input_material_id,0)
		FROM %s.production_bom_versions WHERE id=$1
	`, schema), created.LatestVersionID).Scan(&status, &templateVersionID, &mainInputMaterialID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.production_bom_output_bindings
		WHERE output_type='product' AND output_id=1001 AND bom_id=$1 AND is_default=true
	`, schema), created.ID).Scan(&bindingCount); err != nil {
		t.Fatal(err)
	}
	if status != "published" || templateVersionID != 0 || mainInputMaterialID != 0 || bindingCount != 1 {
		t.Fatalf("manual BOM status/provenance/default=%s/%d/%d/%d", status, templateVersionID, mainInputMaterialID, bindingCount)
	}
}

func TestProductBOMSpecGroupRejectsPartialTemplateProvenancePostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	ensurePR600ProductPublishTestTables(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.product_unit_definitions(code) VALUES('条') ON CONFLICT(code) DO NOTHING;
		INSERT INTO %[1]s.products(id,name,active) VALUES(1002,'来源不完整商品',true);
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,purchase_price)
		VALUES(1102,'PR625-PARTIAL','来源不完整物料','finished','条','条',1);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	created, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "来源不完整 BOM", OutputType: "product", SpecificationMode: bomapp.ProductionBomSpecificationModeSpecGroup,
		OutputID: 1002, OutputProductID: 1002, OutputQty: 1, OutputUnit: "条", Actor: "pr625-auditor",
		Variants: []bomapp.ProductionBomDraftVariant{{
			SpecKey: "stick", Name: "条装", InventoryUnit: "条", IsDefault: true,
			Items: []bomapp.ProductionBomDraftItem{{MaterialID: 1102, ComponentType: "material", ConsumeUnit: "条", QtyPerUnit: 1}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_versions SET source_spec_template_version_id=999999 WHERE id=$1
	`, schema), created.LatestVersionID); err != nil {
		t.Fatal(err)
	}
	err = publishProductionBOMVersionForMaintenanceTest(ctx, pool, repo, created.LatestVersionID, "pr625-auditor")
	if err == nil || !strings.Contains(err.Error(), "规格模板来源和规格主体物料必须同时配置") {
		t.Fatalf("partial specification-template provenance error=%v", err)
	}
}
