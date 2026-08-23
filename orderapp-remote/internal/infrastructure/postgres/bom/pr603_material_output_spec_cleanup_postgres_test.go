package bom

import (
	"fmt"
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
