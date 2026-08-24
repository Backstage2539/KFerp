package bom

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	bomapp "orderapp/internal/application/bom"
)

func TestUpdateProductionBomToMaterialOutputClearsDraftSpecGroupPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_definitions(code) VALUES('袋'),('kg') ON CONFLICT(code) DO NOTHING;
		INSERT INTO %s.products(id,name,active) VALUES(760,'PR603 历史商品',true);
	`, schema, schema)); err != nil {
		t.Fatal(err)
	}
	var outputMaterialID, componentMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit,purchase_price)
		VALUES('PR603-OUT','PR603 产出熟豆','roasted','kg','kg',0)
		RETURNING id
	`, schema)).Scan(&outputMaterialID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit,purchase_price)
		VALUES('PR603-COMP','PR603 生豆','bean','kg','kg',100)
		RETURNING id
	`, schema)).Scan(&componentMaterialID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	created, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "PR603 商品产出 BOM", OutputType: "product", OutputID: 760, OutputProductID: 760,
		OutputQty: 1, OutputUnit: "袋", Actor: "pr603-owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	draftVersionID := created.LatestVersionID
	if _, err := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{
		VersionID: draftVersionID,
		Variants: []bomapp.ProductionBomDraftVariant{
			{SpecKey: "spec-1", Name: "227g", InventoryUnit: "袋", IsDefault: true, SortOrder: 1, Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: componentMaterialID, ConsumeUnit: "kg", QtyPerUnit: 0.227}}},
			{SpecKey: "spec-2", Name: "454g", InventoryUnit: "袋", SortOrder: 2, Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: componentMaterialID, ConsumeUnit: "kg", QtyPerUnit: 0.454}}},
		},
		Actor: "pr603-owner",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.UpdateProductionBom(ctx, bomapp.UpdateProductionBomCommand{
		ID: created.ID, Name: "PR603 转物料产出 BOM",
		OutputType: "material", OutputID: outputMaterialID, OutputMaterialID: outputMaterialID,
		UpdateOutputBinding: true, OutputUnit: "kg", Actor: "pr603-owner",
	}); err != nil {
		t.Fatalf("switch output to material: %v", err)
	}

	var variantCount, variantItemCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_version_variants WHERE version_id=$1`, schema), draftVersionID).Scan(&variantCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_version_items WHERE version_id=$1 AND variant_id<>0`, schema), draftVersionID).Scan(&variantItemCount); err != nil {
		t.Fatal(err)
	}
	if variantCount != 0 || variantItemCount != 0 {
		t.Fatalf("material output conversion must clear draft spec group, got variants=%d variant_items=%d", variantCount, variantItemCount)
	}

	if _, err := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{
		VersionID: draftVersionID,
		OutputQty: 1, OutputUnit: "kg",
		Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: componentMaterialID, ConsumeUnit: "kg", QtyPerUnit: 1}},
		Actor: "pr603-owner",
	}); err != nil {
		t.Fatalf("flat recipe save after conversion: %v", err)
	}
	if err := repo.PublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: draftVersionID, Actor: "pr603-owner"}); err != nil {
		t.Fatalf("publish after conversion must succeed, got %v", err)
	}
}

func TestProductionBomDraftWorkspaceReplacesMaterialItemsIncludingEmptyListPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	var outputMaterialID, firstComponentID, secondComponentID int64
	for _, row := range []struct {
		code   string
		name   string
		target *int64
	}{
		{code: "PR604-OUT", name: "PR604 物料产出", target: &outputMaterialID},
		{code: "PR604-COMP-1", name: "PR604 组件一", target: &firstComponentID},
		{code: "PR604-COMP-2", name: "PR604 组件二", target: &secondComponentID},
	} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.materials(code,name,kind,unit,cost_unit,purchase_price)
			VALUES($1,$2,'bean','kg','kg',10) RETURNING id
		`, schema), row.code, row.name).Scan(row.target); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewRepository(pool, schema)
	created, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "PR604 物料产出 BOM", OutputType: "material", OutputID: outputMaterialID, OutputMaterialID: outputMaterialID,
		OutputQty: 1, OutputUnit: "kg", Actor: "pr604-owner",
	})
	if err != nil {
		t.Fatal(err)
	}

	workspace := func(items []bomapp.ProductionBomDraftItem, variants []bomapp.ProductionBomDraftVariant) {
		t.Helper()
		_, saveErr := repo.UpdateProductionBomDraftWorkspace(ctx, bomapp.ProductionBomDraftWorkspaceCommand{
			Bom:     bomapp.UpdateProductionBomCommand{ID: created.ID, Name: "PR604 物料产出 BOM", OutputType: "material", OutputID: outputMaterialID, OutputMaterialID: outputMaterialID, OutputUnit: "kg", Status: "active", UpdateOutputBinding: true, Actor: "pr604-owner"},
			Version: bomapp.UpdateProductionBomVersionDraftCommand{VersionID: created.LatestVersionID, OutputQty: 1, OutputUnit: "kg", Items: items, Variants: variants, Actor: "pr604-owner"},
		})
		if saveErr != nil {
			t.Fatal(saveErr)
		}
	}

	workspace([]bomapp.ProductionBomDraftItem{
		{ComponentType: "material", MaterialID: firstComponentID, ConsumeUnit: "kg", QtyPerUnit: 0.4},
		{ComponentType: "material", MaterialID: secondComponentID, ConsumeUnit: "kg", QtyPerUnit: 0.6},
	}, []bomapp.ProductionBomDraftVariant{{SpecKey: "legacy-ignored", Name: "不应写入", InventoryUnit: "袋", IsDefault: true}})
	var itemCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_version_items WHERE version_id=$1 AND variant_id=0`, schema), created.LatestVersionID).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if itemCount != 2 {
		t.Fatalf("workspace material items after replacement = %d, want 2", itemCount)
	}

	// An explicit empty JSON array is a real replacement, not an omitted field.
	workspace([]bomapp.ProductionBomDraftItem{}, []bomapp.ProductionBomDraftVariant{{SpecKey: "legacy-ignored-again", Name: "不应写入", InventoryUnit: "袋", IsDefault: true}})
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_version_items WHERE version_id=$1`, schema), created.LatestVersionID).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if itemCount != 0 {
		t.Fatalf("workspace empty material items must clear persisted rows, got %d", itemCount)
	}
}

func TestReplacementDraftKeepsPublishedSourceAndRollsBackAtomicallyPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.products(id,name,active) VALUES(9901,'PR605 源商品',true)`, schema)); err != nil {
		t.Fatal(err)
	}
	var componentID, outputMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,is_semi_finished,unit,cost_unit,purchase_price) VALUES('PR605-COMP','PR605 组件','bean',false,'kg','kg',20) RETURNING id`, schema)).Scan(&componentID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,is_semi_finished,unit,cost_unit,purchase_price) VALUES('PR605-OUT','PR605 半成品', 'other',true,'kg','kg',0) RETURNING id`, schema)).Scan(&outputMaterialID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	source, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: "PR605 源 BOM", OutputType: "product", OutputID: 9901, OutputProductID: 9901, OutputQty: 1, OutputUnit: "kg", Actor: "pr605-test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_versions SET status='published',published_at=now() WHERE id=$1;
		INSERT INTO %s.production_bom_version_items(version_id,material_id,component_type,consume_unit,ratio_pct,material_loss_rate) VALUES($1,$2,'material','ratio_pct',100,0.195)
	`, schema, schema), source.LatestVersionID, componentID); err != nil {
		t.Fatal(err)
	}
	loss := 0.195
	command := bomapp.CreateProductionBomReplacementDraftCommand{
		SourceBomID: source.ID, SourceVersionID: source.LatestVersionID,
		Workspace: bomapp.ProductionBomDraftWorkspaceCommand{
			Bom:     bomapp.UpdateProductionBomCommand{Name: "PR605 替代 BOM", OutputType: "material", OutputID: outputMaterialID, OutputMaterialID: outputMaterialID, OutputUnit: "kg", Status: "active", Actor: "pr605-test", UpdateOutputBinding: true},
			Version: bomapp.UpdateProductionBomVersionDraftCommand{OutputQty: 1, OutputUnit: "kg", MaterialLossRate: &loss, Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: componentID, ConsumeUnit: "ratio_pct", RatioPct: 100, MaterialLossRate: loss}}, Actor: "pr605-test"},
		},
	}
	replacement, err := repo.CreateProductionBomReplacementDraft(ctx, command)
	if err != nil {
		t.Fatalf("CreateProductionBomReplacementDraft: %v", err)
	}
	var sourceOutputType, sourceVersionStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT pb.output_type,v.status FROM %s.production_boms pb JOIN %s.production_bom_versions v ON v.id=$2 WHERE pb.id=$1`, schema, schema), source.ID, source.LatestVersionID).Scan(&sourceOutputType, &sourceVersionStatus); err != nil {
		t.Fatal(err)
	}
	if sourceOutputType != "product" || sourceVersionStatus != "published" {
		t.Fatalf("source mutated to output=%s version=%s", sourceOutputType, sourceVersionStatus)
	}
	if _, err := repo.UpdateProductionBom(ctx, bomapp.UpdateProductionBomCommand{
		ID: source.ID, Name: "PR605 源 BOM", OutputType: "material", OutputID: 999999, OutputMaterialID: 999999,
		UpdateOutputBinding: true, OutputUnit: "kg", Actor: "pr605-test",
	}); !errors.Is(err, bomapp.ErrPublishedOutputIdentityImmutable) {
		t.Fatalf("published identity guard must run before target validation, got %v", err)
	}
	var sourceBomID, sourceVersionID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT source_bom_id,source_bom_version_id FROM %s.production_boms WHERE id=$1`, schema), replacement.ID).Scan(&sourceBomID, &sourceVersionID); err != nil {
		t.Fatal(err)
	}
	if sourceBomID != source.ID || sourceVersionID != source.LatestVersionID {
		t.Fatalf("replacement provenance=%d/%d", sourceBomID, sourceVersionID)
	}
	var storedLoss float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT material_loss_rate::float8 FROM %s.production_bom_version_items WHERE version_id=$1`, schema), replacement.LatestVersionID).Scan(&storedLoss); err != nil {
		t.Fatal(err)
	}
	if storedLoss != loss {
		t.Fatalf("replacement item loss=%v want %v", storedLoss, loss)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='production_bom' AND entity_id=$1 AND action='create_replacement_draft'`, schema), replacement.ID).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("replacement audit count=%d err=%v", auditCount, err)
	}

	command.Workspace.Bom.GroupID = 999999
	command.Workspace.Bom.GroupCategoryID = 999998
	command.Workspace.Bom.UpdateGroupAssignment = true
	if _, err := repo.CreateProductionBomReplacementDraft(ctx, command); err == nil || !strings.Contains(err.Error(), "business group item mismatch") {
		t.Fatalf("invalid category replacement error=%v", err)
	}
	var replacementCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms WHERE source_bom_id=$1`, schema), source.ID).Scan(&replacementCount); err != nil {
		t.Fatal(err)
	}
	if replacementCount != 1 {
		t.Fatalf("failed replacement left partial rows, count=%d", replacementCount)
	}
}
