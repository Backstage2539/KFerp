package bom

import (
	"fmt"
	"strings"
	"testing"

	bomapp "orderapp/internal/application/bom"
)

func TestProductionBomMaterialOptionsExcludeDeprecatedRowsPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	var activeID, deprecatedID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,is_semi_finished,unit,cost_unit,purchase_price)
		VALUES('PR617-ACTIVE','PR617 有效咖啡粉','other',true,'kg','kg',0)
		RETURNING id
	`, schema)).Scan(&activeID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,is_semi_finished,unit,cost_unit,purchase_price,deprecated_at)
		VALUES('PR617-DEPRECATED','PR617 已失效咖啡粉','other',true,'kg','kg',0,now())
		RETURNING id
	`, schema)).Scan(&deprecatedID); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	allOptions, err := repo.Materials(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertPR617MaterialOptionIDs(t, allOptions, activeID, deprecatedID)

	scopedOptions, err := listProductionBomMaterialOptions(ctx, pool, schema, []int64{activeID, deprecatedID})
	if err != nil {
		t.Fatal(err)
	}
	assertPR617MaterialOptionIDs(t, scopedOptions, activeID, deprecatedID)
}

func TestCreateProductionBomRejectsDeprecatedOutputWithStableErrorPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	var activeID, deprecatedID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,is_semi_finished,unit,cost_unit,purchase_price)
		VALUES('PR617-EDIT-SOURCE','PR617 编辑来源物料','other',true,'kg','kg',0)
		RETURNING id
	`, schema)).Scan(&activeID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,is_semi_finished,unit,cost_unit,purchase_price,deprecated_at)
		VALUES('PR617-STALE-OUTPUT','PR617 过期产出物料','other',true,'kg','kg',0,now())
		RETURNING id
	`, schema)).Scan(&deprecatedID); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	editable, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "PR617 可编辑 BOM", OutputType: "material", OutputID: activeID,
		OutputMaterialID: activeID, OutputQty: 1, OutputUnit: "kg", Actor: "pr617-setup",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "PR617 不应落库的 BOM", OutputType: "material", OutputID: deprecatedID,
		OutputMaterialID: deprecatedID, OutputQty: 1, OutputUnit: "kg", Actor: "pr617-test",
	})
	if err == nil || !strings.Contains(err.Error(), "产出物料不存在或已失效") {
		t.Fatalf("deprecated output material error = %v", err)
	}
	_, err = repo.UpdateProductionBom(ctx, bomapp.UpdateProductionBomCommand{
		ID: editable.ID, Name: editable.Name, OutputType: "material", OutputID: deprecatedID,
		OutputMaterialID: deprecatedID, UpdateOutputBinding: true, Status: "active", Actor: "pr617-test",
	})
	if err == nil || !strings.Contains(err.Error(), "产出物料不存在或已失效") {
		t.Fatalf("deprecated output material update error = %v", err)
	}

	var bomCount, auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_boms WHERE name='PR617 不应落库的 BOM'`, schema)).Scan(&bomCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE actor='pr617-test' AND entity_type='production_bom'`, schema)).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if bomCount != 0 || auditCount != 0 {
		t.Fatalf("rejected stale output persisted bom/audit = %d/%d", bomCount, auditCount)
	}
	var persistedOutputID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT output_material_id FROM %s.production_boms WHERE id=$1`, schema), editable.ID).Scan(&persistedOutputID); err != nil {
		t.Fatal(err)
	}
	if persistedOutputID != activeID {
		t.Fatalf("rejected stale output update changed material = %d, want %d", persistedOutputID, activeID)
	}
}

func assertPR617MaterialOptionIDs(t *testing.T, options []bomapp.Option, activeID, deprecatedID int64) {
	t.Helper()
	var activeFound, deprecatedFound bool
	for _, option := range options {
		activeFound = activeFound || option.ID == activeID
		deprecatedFound = deprecatedFound || option.ID == deprecatedID
	}
	if !activeFound || deprecatedFound {
		t.Fatalf("material options active/deprecated = %v/%v, options=%+v", activeFound, deprecatedFound, options)
	}
}
