package materials

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNormalizeMaterialInputRejectsInvalidValues(t *testing.T) {
	_, err := normalizeMaterialInput(materialInput{
		Code:          "bean-a",
		Name:          "豆子A",
		Kind:          "bean",
		Unit:          "kg",
		PurchasePrice: -1,
	})
	if err == nil || !strings.Contains(err.Error(), "negative price") {
		t.Fatalf("normalizeMaterialInput() error = %v, want negative price", err)
	}
}

func TestNormalizeMaterialInputDefaultsKindAndUnit(t *testing.T) {
	got, err := normalizeMaterialInput(materialInput{Code: " m-1 ", Name: " 物料1 "})
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "m-1" || got.Name != "物料1" || got.Kind != "other" || got.Unit != "kg" || got.CostUnit != "kg" {
		t.Fatalf("normalizeMaterialInput() = %+v", got)
	}
}

func TestNormalizeMaterialInputDerivesCostUnitFromInventoryUnit(t *testing.T) {
	weight, err := normalizeMaterialInput(materialInput{Code: "bean-cost", Name: "重量物料", Unit: "kg"})
	if err != nil {
		t.Fatal(err)
	}
	if weight.CostUnit != "kg" {
		t.Fatalf("weight cost unit = %q, want inventory unit kg", weight.CostUnit)
	}
	discrete, err := normalizeMaterialInput(materialInput{Code: "pack-cost", Name: "计件物料", Unit: "个"})
	if err != nil {
		t.Fatal(err)
	}
	if discrete.CostUnit != "个" {
		t.Fatalf("discrete cost unit = %q, want 个", discrete.CostUnit)
	}
	for _, unit := range []string{"g", "gram", "lb", "pound", "oz", "ounce", "克", "磅", "盎司"} {
		if _, err := normalizeMaterialInput(materialInput{Code: "bad-weight", Name: "非标准重量主档", Unit: unit}); err == nil || !strings.Contains(err.Error(), "重量物料库存单位统一使用 kg；BOM 用量可使用 g 并自动换算") {
			t.Fatalf("weight unit %q error = %v", unit, err)
		}
	}
	if _, err := normalizeMaterialInput(materialInput{Code: "bad-cost", Name: "错误计价", Unit: "kg", CostUnit: "g"}); err == nil || !strings.Contains(err.Error(), "采购价与成本单价单位必须与库存单位一致") {
		t.Fatalf("invalid cost unit error = %v", err)
	}
}

func TestMaterialInventoryUnitIsLockedAfterCreate(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"assertMaterialInventoryUnitReadOnly",
		"requestedInventoryUnit",
		"库存单位保存后不能修改",
		"old.Unit",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("material inventory unit lock missing marker %q", want)
		}
	}
}

func TestMaterialCostUnitIsDerivedFromInventoryUnit(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"assertMaterialCostUnitMatchesInventoryUnit",
		"requestedCostUnit",
		"采购价与成本单价单位必须与库存单位一致",
		"next.CostUnit = next.Unit",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("material cost unit derivation missing marker %q", want)
		}
	}
}

func TestMaterialCostUnitMustMatchEffectiveInventoryUnit(t *testing.T) {
	if err := assertMaterialCostUnitMatchesInventoryUnit("kg", "g"); err == nil || !strings.Contains(err.Error(), "库存单位一致") {
		t.Fatalf("mismatched unit error = %v", err)
	}
	if err := assertMaterialCostUnitMatchesInventoryUnit("g", "g"); err != nil {
		t.Fatalf("matching unit rejected: %v", err)
	}
	if err := assertMaterialCostUnitMatchesInventoryUnit("", "kg"); err != nil {
		t.Fatalf("omitted compatibility field rejected: %v", err)
	}
}

func TestMaterialWriteRejectsMismatchedCostUnitPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr599_material_unit_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.audit_logs(
			id BIGSERIAL PRIMARY KEY, ts TIMESTAMPTZ NOT NULL DEFAULT now(), actor TEXT NOT NULL DEFAULT '',
			entity_type TEXT NOT NULL DEFAULT '', entity_id BIGINT, action TEXT NOT NULL DEFAULT '', field TEXT,
			old_value TEXT, new_value TEXT, meta JSONB
		)
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	created, err := createMaterialInline(ctx, pool, schema, "pr599-test", materialInput{
		Code: "UNIT-LOCK-1", Name: "单位保护物料", Kind: "bean", Unit: "kg", PurchasePrice: 288,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Unit != "kg" || created.CostUnit != "kg" {
		t.Fatalf("created unit/cost_unit = %s/%s, want kg/kg", created.Unit, created.CostUnit)
	}
	bad := materialInput{Code: "UNIT-LOCK-1", Name: "单位保护物料", Kind: "bean", Unit: "kg", CostUnit: "g", BatchNo: created.BatchNo, PurchasePrice: 288}
	if _, err := updateMaterialInline(ctx, pool, schema, "pr599-test", created.ID, bad); err == nil || !strings.Contains(err.Error(), "库存单位一致") {
		t.Fatalf("mismatched update error = %v", err)
	}
	var unit, costUnit string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit,cost_unit FROM %s.materials WHERE id=$1`, schema), created.ID).Scan(&unit, &costUnit); err != nil {
		t.Fatal(err)
	}
	if unit != "kg" || costUnit != "kg" {
		t.Fatalf("failed update changed unit/cost_unit to %s/%s, want kg/kg", unit, costUnit)
	}
}

func TestMaterialWriteValidatesActiveUnitDefinitionAndRejectsCustomWeightPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr599_material_unit_dictionary_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.audit_logs(
			id BIGSERIAL PRIMARY KEY, ts TIMESTAMPTZ NOT NULL DEFAULT now(), actor TEXT NOT NULL DEFAULT '',
			entity_type TEXT NOT NULL DEFAULT '', entity_id BIGINT, action TEXT NOT NULL DEFAULT '', field TEXT,
			old_value TEXT, new_value TEXT, meta JSONB
		)
	`, schema)); err != nil {
		t.Fatal(err)
	}
	// A clean material schema is still supported before the global unit dictionary exists.
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.product_unit_definitions(
			code TEXT PRIMARY KEY, name TEXT NOT NULL, unit_type TEXT NOT NULL,
			allow_decimal BOOLEAN NOT NULL DEFAULT true, active BOOLEAN NOT NULL DEFAULT true
		);
		INSERT INTO %[1]s.product_unit_definitions(code,name,unit_type,active) VALUES
			('kg','kg','weight',true),
			('t','吨','weight',true),
			('吨','吨','重量',true),
			('袋','袋','package',true),
			('停用袋','停用袋','package',false);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	if _, err := createMaterialInline(ctx, pool, schema, "pr599-test", materialInput{
		Code: "CUSTOM-TON", Name: "自定义吨物料", Kind: "bean", Unit: "t", CostUnit: "t",
	}); err == nil || !strings.Contains(err.Error(), "重量物料库存单位统一使用 kg") {
		t.Fatalf("custom weight create error = %v", err)
	}
	if _, err := createMaterialInline(ctx, pool, schema, "pr599-test", materialInput{
		Code: "CUSTOM-CHINESE-TON", Name: "自定义中文吨物料", Kind: "bean", Unit: "吨", CostUnit: "吨",
	}); err == nil || !strings.Contains(err.Error(), "重量物料库存单位统一使用 kg") {
		t.Fatalf("custom Chinese weight create error = %v", err)
	}
	if _, err := createMaterialInline(ctx, pool, schema, "pr599-test", materialInput{
		Code: "INACTIVE-BAG", Name: "停用袋物料", Kind: "pack", Unit: "停用袋", CostUnit: "停用袋",
	}); err == nil || !strings.Contains(err.Error(), "已启用的全局单位字典") {
		t.Fatalf("inactive unit create error = %v", err)
	}
	if _, err := createMaterialInline(ctx, pool, schema, "pr599-test", materialInput{
		Code: "MISSING-BAG", Name: "缺失字典袋物料", Kind: "pack", Unit: "缺失袋", CostUnit: "缺失袋",
	}); err == nil || !strings.Contains(err.Error(), "已启用的全局单位字典") {
		t.Fatalf("missing unit create error = %v", err)
	}
	var legacyInactiveID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit,batch_no)
		VALUES('LEGACY-INACTIVE-BAG','历史停用袋物料','pack','停用袋','停用袋','legacy')
		RETURNING id
	`, schema)).Scan(&legacyInactiveID); err != nil {
		t.Fatal(err)
	}
	if _, err := updateMaterialInline(ctx, pool, schema, "pr599-test", legacyInactiveID, materialInput{
		Code: "LEGACY-INACTIVE-BAG", Name: "历史停用袋物料改名", Kind: "pack", Unit: "停用袋", CostUnit: "停用袋", BatchNo: "legacy",
	}); err != nil {
		t.Fatalf("unchanged inactive legacy unit should allow non-unit edit: %v", err)
	}
	created, err := createMaterialInline(ctx, pool, schema, "pr599-test", materialInput{
		Code: "ACTIVE-BAG", Name: "有效袋物料", Kind: "pack", Unit: "袋", CostUnit: "袋",
	})
	if err != nil {
		t.Fatalf("custom package create: %v", err)
	}
	if created.Unit != "袋" || created.CostUnit != "袋" {
		t.Fatalf("custom package units = %s/%s, want 袋/袋", created.Unit, created.CostUnit)
	}
	if _, err := updateMaterialInline(ctx, pool, schema, "pr599-test", created.ID, materialInput{
		Code: created.Code, Name: created.Name, Kind: created.Kind, Unit: "t", CostUnit: "t", BatchNo: created.BatchNo,
	}); err == nil || !strings.Contains(err.Error(), "重量物料库存单位统一使用 kg") {
		t.Fatalf("custom weight update error = %v", err)
	}
	var unit, costUnit string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit,cost_unit FROM %s.materials WHERE id=$1`, schema), created.ID).Scan(&unit, &costUnit); err != nil {
		t.Fatal(err)
	}
	if unit != "袋" || costUnit != "袋" {
		t.Fatalf("failed custom-weight update changed units to %s/%s", unit, costUnit)
	}
}

func TestNormalizeMaterialInputAcceptsLegacyKindAliases(t *testing.T) {
	bean, err := normalizeMaterialInput(materialInput{Code: "raw-a", Name: "旧生豆", Kind: " raw_bean ", Unit: "kg"})
	if err != nil {
		t.Fatalf("normalize raw_bean alias: %v", err)
	}
	if bean.Kind != "bean" {
		t.Fatalf("raw_bean kind = %q, want bean", bean.Kind)
	}

	pack, err := normalizeMaterialInput(materialInput{Code: "bag-a", Name: "旧包材", Kind: "packaging", Unit: "个"})
	if err != nil {
		t.Fatalf("normalize packaging alias: %v", err)
	}
	if pack.Kind != "pack" {
		t.Fatalf("packaging kind = %q, want pack", pack.Kind)
	}
}

func TestNormalizeMaterialInputDefaultsBatchNoToToday(t *testing.T) {
	got, err := normalizeMaterialInput(materialInput{Code: "bean-a", Name: "豆子A", Kind: "bean", Unit: "kg"})
	if err != nil {
		t.Fatal(err)
	}
	if got.BatchNo != time.Now().Format("20060102") {
		t.Fatalf("batch no = %q, want today", got.BatchNo)
	}
}

func TestNormalizeMaterialInputKeepsBeanProfileOnlyForBeans(t *testing.T) {
	got, err := normalizeMaterialInput(materialInput{
		Code:        "bean-a",
		Name:        "豆子A",
		Kind:        "bean",
		Unit:        "kg",
		Profile:     &beanProfileInput{Origin: " 云南 ", Flavor: " 柑橘 "},
		PackProfile: &packProfileInput{SizeSpec: "不应保留"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile == nil || got.Profile.Origin != "云南" || got.Profile.Flavor != "柑橘" {
		t.Fatalf("bean profile = %+v", got.Profile)
	}
	if got.PackProfile != nil {
		t.Fatalf("bean pack profile = %+v, want nil", got.PackProfile)
	}

	pack, err := normalizeMaterialInput(materialInput{
		Code:        "pack-a",
		Name:        "袋子A",
		Kind:        "pack",
		Unit:        "个",
		Profile:     &beanProfileInput{Flavor: "不应保留"},
		PackProfile: &packProfileInput{SizeSpec: " 227g ", Dimensions: " 12x20cm "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pack.Profile != nil {
		t.Fatalf("pack profile = %+v, want nil", pack.Profile)
	}
	if pack.PackProfile == nil || pack.PackProfile.SizeSpec != "227g" || pack.PackProfile.Dimensions != "12x20cm" {
		t.Fatalf("pack profile = %+v", pack.PackProfile)
	}
}

func TestAssertMaterialStockFieldsReadOnlyAllowsBaseFieldEdits(t *testing.T) {
	old := materialRow{
		Code:          "bean-a",
		Name:          "豆子A",
		Kind:          "bean",
		Unit:          "g",
		BatchNo:       "20260427",
		PurchasePrice: 88,
		SalePrice:     99,
	}
	next := materialInput{
		Code:          "bean-a",
		Name:          "豆子A新",
		Kind:          "bean",
		Unit:          "kg",
		BatchNo:       "20260427",
		PurchasePrice: 90,
		SalePrice:     99,
	}
	err := assertMaterialStockFieldsReadOnly(old, next)
	if err != nil {
		t.Fatalf("assertMaterialStockFieldsReadOnly() error = %v, want base fields editable", err)
	}
}

func TestAssertImmutableMaterialFieldsRejectsInlineStockChange(t *testing.T) {
	old := materialRow{
		Code:          "bean-a",
		Name:          "豆子A",
		Kind:          "bean",
		Unit:          "g",
		BatchNo:       "20260427",
		PurchasePrice: 88,
		SalePrice:     99,
		OnhandG:       1000,
		OnhandUnits:   2,
	}
	next := materialInput{
		Code:          "bean-a",
		Name:          "豆子A",
		Kind:          "bean",
		Unit:          "g",
		BatchNo:       "20260427",
		PurchasePrice: 88,
		SalePrice:     99,
		OnhandG:       1200,
		OnhandUnits:   2,
	}
	err := assertMaterialStockFieldsReadOnly(old, next)
	if err == nil || !strings.Contains(err.Error(), "stock adjustment") {
		t.Fatalf("assertMaterialStockFieldsReadOnly() error = %v, want stock adjustment", err)
	}
}

func TestNormalizeMaterialInputCarriesIndustryTemplateAndFields(t *testing.T) {
	got, err := normalizeMaterialInput(materialInput{
		Code:                    "bean-industry",
		Name:                    "行业字段豆",
		Unit:                    "kg",
		IndustryFieldTemplateID: 7,
		IndustryFields: []materialIndustryFieldInput{
			{FieldKey: " 产地 ", ValueText: " 云南 "},
			{FieldKey: "", ValueText: "忽略"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.IndustryFieldTemplateID != 7 || len(got.IndustryFields) != 1 || got.IndustryFields[0].FieldKey != "产地" || got.IndustryFields[0].ValueText != "云南" {
		t.Fatalf("industry fields = %+v template=%d", got.IndustryFields, got.IndustryFieldTemplateID)
	}
}
