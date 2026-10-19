package bom

import (
	"fmt"
	"strings"
	"testing"

	bomapp "orderapp/internal/application/bom"
)

func logisticsTestWeight(value float64) *float64 { return &value }

func TestPR706LogisticsWeightTemplateAndProductPublishLifecyclePostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	preparePR600ComponentSpecCatalogFixture(t, ctx, pool, schema)
	ensurePR600ProductPublishTestTables(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.product_unit_definitions(code) VALUES('袋');
		INSERT INTO %[1]s.products(id,name,active) VALUES(706,'手填物流重量商品',true);
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit) VALUES(706,'WEIGHT-MAIN','主体物料','bean','kg','kg');
	`, schema)); err != nil {
		t.Fatal(err)
	}
	r := NewRepository(pool, schema)
	template, err := r.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "物流重量模板", Actor: "pr706"})
	if err != nil {
		t.Fatal(err)
	}
	versionID := template.Versions[0].ID
	variant := bomapp.ProductionBomSpecTemplateVariant{SpecKey: "bag", Name: "任意名称", InventoryUnit: "袋", IsDefault: true, Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 0.227}}}}
	saveTemplate := func() {
		t.Helper()
		if _, err := r.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: versionID, Variants: []bomapp.ProductionBomSpecTemplateVariant{variant}, Actor: "pr706"}); err != nil {
			t.Fatal(err)
		}
	}
	saveTemplate()
	if err := r.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: versionID}); err == nil || !strings.Contains(err.Error(), "logistics_weight_g") {
		t.Fatalf("missing template weight publish=%v", err)
	}
	variant.LogisticsWeightG = logisticsTestWeight(231.125)
	saveTemplate()
	if err := r.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: versionID, Actor: "pr706"}); err != nil {
		t.Fatal(err)
	}
	created, err := r.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: "带重商品", OutputType: "product", OutputID: 706, OutputProductID: 706, SpecificationMode: "spec_group", OutputQty: 1, SpecTemplateVersionID: versionID, MainInputMaterialID: 706, Actor: "pr706"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: created.LatestVersionID, Actor: "pr706"}); err != nil {
		t.Fatal(err)
	}
	specs, err := r.ListProductionBomPublishedSpecs(ctx, 706)
	if err != nil || len(specs) != 1 || specs[0].LogisticsWeightG == nil || *specs[0].LogisticsWeightG != 231.125 {
		t.Fatalf("published specs=%+v err=%v", specs, err)
	}
	cloned, err := r.CreateProductionBomVersion(ctx, bomapp.CreateProductionBomVersionCommand{BomID: created.ID, Actor: "pr706"})
	if err != nil {
		t.Fatal(err)
	}
	variants, err := r.listProductionBomVersionVariants(ctx, cloned.ID)
	if err != nil || len(variants) != 1 || variants[0].LogisticsWeightG == nil || *variants[0].LogisticsWeightG != 231.125 {
		t.Fatalf("cloned variants=%+v err=%v", variants, err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=NULL WHERE version_id=$1`, schema), cloned.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: cloned.ID}); err == nil || !strings.Contains(err.Error(), "logistics_weight_g") {
		t.Fatalf("missing BOM weight publish=%v", err)
	}
	// Old published variants remain readable and unchanged when a draft is incomplete.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=NULL WHERE version_id=$1`, schema), created.LatestVersionID); err != nil {
		t.Fatal(err)
	}
	specs, err = r.ListProductionBomPublishedSpecs(ctx, 706)
	if err != nil || len(specs) != 1 || specs[0].LogisticsWeightG != nil {
		t.Fatalf("legacy published null specs=%+v err=%v", specs, err)
	}
	newTemplate, err := r.CreateProductionBomSpecTemplateVersion(ctx, bomapp.CreateProductionBomSpecTemplateVersionCommand{TemplateID: template.ID, Actor: "pr706"})
	if err != nil {
		t.Fatal(err)
	}
	gotTemplate, err := r.GetProductionBomSpecTemplate(ctx, template.ID, newTemplate.ID)
	if err != nil || len(gotTemplate.Variants) != 1 || gotTemplate.Variants[0].LogisticsWeightG == nil || *gotTemplate.Variants[0].LogisticsWeightG != 231.125 {
		t.Fatalf("cloned template=%+v err=%v", gotTemplate.Variants, err)
	}
	gotTemplate.Variants[0].LogisticsWeightG = logisticsTestWeight(462.25)
	if _, err := r.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: newTemplate.ID, Variants: gotTemplate.Variants, Actor: "pr706"}); err != nil {
		t.Fatal(err)
	}
	if err := r.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: newTemplate.ID, Actor: "pr706"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReapplyProductionBomSpecTemplateVersion(ctx, bomapp.ReapplyProductionBomSpecTemplateVersionCommand{VersionID: cloned.ID, SpecTemplateVersionID: newTemplate.ID, MainInputMaterialID: 706, Actor: "pr706"}); err != nil {
		t.Fatal(err)
	}
	variants, err = r.listProductionBomVersionVariants(ctx, cloned.ID)
	if err != nil || len(variants) != 1 || variants[0].LogisticsWeightG == nil || *variants[0].LogisticsWeightG != 462.25 {
		t.Fatalf("reapplied variants=%+v err=%v", variants, err)
	}
	if err := r.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: cloned.ID, Actor: "pr706"}); err != nil {
		t.Fatal(err)
	}
	copied, err := r.CopyProductionBom(ctx, bomapp.CopyProductionBomCommand{ID: created.ID, Name: "物流重量复制", OutputType: "product", OutputID: 706, OutputProductID: 706, SpecificationMode: "spec_group", Actor: "pr706"})
	if err != nil {
		t.Fatal(err)
	}
	variants, err = r.listProductionBomVersionVariants(ctx, copied.LatestVersionID)
	if err != nil || len(variants) != 1 || variants[0].LogisticsWeightG == nil || *variants[0].LogisticsWeightG != 462.25 {
		t.Fatalf("copied BOM variants=%+v err=%v", variants, err)
	}
	var weightAudits int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE actor='pr706' AND (field='logistics_weight_g' OR field='variants' AND new_value LIKE '%%logistics_weight_g%%')`, schema)).Scan(&weightAudits); err != nil || weightAudits == 0 {
		t.Fatalf("weight audits=%d err=%v", weightAudits, err)
	}
}

func TestPR706MaterialOutputDoesNotRequireLogisticsWeightAndLegacyProductRepublishDoes(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	preparePR600ComponentSpecCatalogFixture(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,is_semi_finished) VALUES(706,'W-RAW','原料','bean','kg','kg',false),(707,'W-SEMI','半成品','bean','kg','kg',true); INSERT INTO %[1]s.products(id,name,active) VALUES(706,'历史单产出商品',true);`, schema)); err != nil {
		t.Fatal(err)
	}
	r := NewRepository(pool, schema)
	material, err := r.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: "物料免填物流重", OutputType: "material", OutputID: 707, OutputMaterialID: 707, SpecificationMode: "single", OutputQty: 1, OutputUnit: "kg", Actor: "pr706"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: material.LatestVersionID, Items: []bomapp.ProductionBomDraftItem{{MaterialID: 706, ComponentType: "material", ConsumeUnit: "kg", QtyPerUnit: 1}}, Actor: "pr706"}); err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: material.LatestVersionID, Actor: "pr706"}); err != nil {
		t.Fatalf("material publish requires product weight: %v", err)
	}
	legacy, err := r.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: "历史商品新草稿", OutputType: "product", OutputID: 706, OutputProductID: 706, SpecificationMode: "single", OutputQty: 1, OutputUnit: "kg", Actor: "pr706"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: legacy.LatestVersionID, Items: []bomapp.ProductionBomDraftItem{{MaterialID: 706, ComponentType: "material", ConsumeUnit: "kg", QtyPerUnit: 1}}, Actor: "pr706"}); err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: legacy.LatestVersionID, Actor: "pr706"}); err == nil || !strings.Contains(err.Error(), "默认规格及物流重量") {
		t.Fatalf("legacy product publish=%v", err)
	}
}
