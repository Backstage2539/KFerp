package bom

import (
	"context"
	"fmt"
	"strings"
	"testing"

	bomapp "orderapp/internal/application/bom"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductComponentSelectedSpecUnitGovernsTemplateSaveAndPublishPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	preparePR600ComponentSpecCatalogFixture(t, ctx, pool, schema)
	seedPR600ComponentSpecUnitAuthority(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	for _, testCase := range []struct {
		name      string
		specID    int64
		consume   string
		wantError bool
	}{
		{name: "box specification accepts box", specID: 1201, consume: "盒"},
		{name: "box specification rejects parent kg", specID: 1201, consume: "kg", wantError: true},
		{name: "bag specification accepts bag", specID: 1202, consume: "袋"},
		{name: "different specification units stay isolated", specID: 1202, consume: "盒", wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			template, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "规格单位模板 " + testCase.name, Actor: "component-spec-unit-auditor"})
			if err != nil {
				t.Fatal(err)
			}
			versionID := template.Versions[0].ID
			variants := pr600ComponentSpecUnitTemplateVariants(testCase.specID, testCase.consume)
			_, saveErr := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{
				VersionID: versionID, Variants: variants, Actor: "component-spec-unit-auditor",
			})
			if testCase.wantError {
				if saveErr == nil || !strings.Contains(saveErr.Error(), "component BOM specification inventory_unit") {
					t.Errorf("template save consume_unit=%q spec=%d error=%v", testCase.consume, testCase.specID, saveErr)
				}
				if saveErr != nil {
					seedPR600InvalidTemplateUnitDraft(t, ctx, pool, schema, versionID, testCase.specID, testCase.consume)
				}
				publishErr := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: versionID, Actor: "component-spec-unit-auditor"})
				if publishErr == nil || !strings.Contains(publishErr.Error(), "component BOM specification inventory_unit") {
					t.Errorf("template publish consume_unit=%q spec=%d error=%v", testCase.consume, testCase.specID, publishErr)
				}
				return
			}
			if saveErr != nil {
				t.Fatalf("valid template save consume_unit=%q spec=%d: %v", testCase.consume, testCase.specID, saveErr)
			}
			if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: versionID, Actor: "component-spec-unit-auditor"}); err != nil {
				t.Fatalf("valid template publish consume_unit=%q spec=%d: %v", testCase.consume, testCase.specID, err)
			}
		})
	}
}

func TestProductComponentSelectedSpecUnitGovernsBOMDraftAndPublishPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	ensurePR600ProductPublishTestTables(t, ctx, pool, schema)
	preparePR600ComponentSpecCatalogFixture(t, ctx, pool, schema)
	seedPR600ComponentSpecUnitAuthority(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	for index, testCase := range []struct {
		name      string
		specID    int64
		consume   string
		wantError bool
	}{
		{name: "box specification accepts box", specID: 1201, consume: "盒"},
		{name: "box specification rejects parent kg", specID: 1201, consume: "kg", wantError: true},
		{name: "bag specification accepts bag", specID: 1202, consume: "袋"},
		{name: "different specification units stay isolated", specID: 1202, consume: "盒", wantError: true},
		{name: "legacy product component without specification keeps parent unit", consume: "kg"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var outputMaterialID int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`
				INSERT INTO %s.materials(code,name,kind,unit,cost_unit)
				VALUES($1,$2,'bean','kg','kg') RETURNING id
			`, schema), fmt.Sprintf("PR600-SPEC-UNIT-OUTPUT-%d", index), "规格单位产出 "+testCase.name).Scan(&outputMaterialID); err != nil {
				t.Fatal(err)
			}
			created, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
				Name: "规格单位 BOM " + testCase.name, OutputType: "material", OutputID: outputMaterialID, OutputMaterialID: outputMaterialID,
				OutputQty: 1, OutputUnit: "kg", Actor: "component-spec-unit-auditor",
			})
			if err != nil {
				t.Fatal(err)
			}
			item := bomapp.ProductionBomDraftItem{
				ComponentType: "product", ComponentProductID: 1101, ComponentBomSpecID: testCase.specID,
				ConsumeUnit: testCase.consume, QtyPerUnit: 1,
			}
			_, saveErr := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{
				VersionID: created.LatestVersionID, OutputQty: 1, OutputUnit: "kg", Items: []bomapp.ProductionBomDraftItem{item}, Actor: "component-spec-unit-auditor",
			})
			if testCase.wantError {
				if saveErr == nil || !strings.Contains(saveErr.Error(), "component BOM specification inventory_unit") {
					t.Errorf("BOM draft consume_unit=%q spec=%d error=%v", testCase.consume, testCase.specID, saveErr)
				}
				if saveErr != nil {
					seedPR600InvalidBOMUnitDraft(t, ctx, pool, schema, created.LatestVersionID, item)
				}
				publishErr := repo.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: created.LatestVersionID, Actor: "component-spec-unit-auditor"})
				if publishErr == nil || !strings.Contains(publishErr.Error(), "component BOM specification inventory_unit") {
					t.Errorf("BOM publish consume_unit=%q spec=%d error=%v", testCase.consume, testCase.specID, publishErr)
				}
				return
			}
			if saveErr != nil {
				t.Fatalf("valid BOM draft consume_unit=%q spec=%d: %v", testCase.consume, testCase.specID, saveErr)
			}
			if err := repo.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: created.LatestVersionID, Actor: "component-spec-unit-auditor"}); err != nil {
				t.Fatalf("valid BOM publish consume_unit=%q spec=%d: %v", testCase.consume, testCase.specID, err)
			}
		})
	}
}

func preparePR600ComponentSpecCatalogFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS customer_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS parent_product_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS unit_rule_override_json JSONB NOT NULL DEFAULT '{}'::jsonb;
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS unit_template_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS product_config_template_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS product_category_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS roast_level TEXT NOT NULL DEFAULT '';
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS product_kind TEXT NOT NULL DEFAULT 'roasted_bean';
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS drip_bag_grams NUMERIC(12,3) NOT NULL DEFAULT 10;
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS drip_box_bag_count INT NOT NULL DEFAULT 10;
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS auto_derived_sku BOOLEAN NOT NULL DEFAULT false;
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS derived_spec_status TEXT NOT NULL DEFAULT '';
		CREATE TABLE IF NOT EXISTS %[1]s.product_unit_templates(id BIGINT PRIMARY KEY);
		ALTER TABLE %[1]s.product_unit_templates ADD COLUMN IF NOT EXISTS inventory_unit TEXT NOT NULL DEFAULT 'kg';
		ALTER TABLE %[1]s.product_unit_templates ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;
		ALTER TABLE %[1]s.product_unit_templates ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
		CREATE TABLE IF NOT EXISTS %[1]s.product_config_templates(id BIGINT PRIMARY KEY);
		ALTER TABLE %[1]s.product_config_templates ADD COLUMN IF NOT EXISTS inventory_unit TEXT NOT NULL DEFAULT 'kg';
		ALTER TABLE %[1]s.product_config_templates ADD COLUMN IF NOT EXISTS unit_template_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.product_config_templates ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
		CREATE TABLE IF NOT EXISTS %[1]s.product_categories(id BIGINT PRIMARY KEY);
		ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS inventory_unit TEXT NOT NULL DEFAULT 'kg';
		ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS unit_template_id BIGINT NOT NULL DEFAULT 0;
		CREATE TABLE IF NOT EXISTS %[1]s.orders(id BIGINT PRIMARY KEY,is_void BOOLEAN NOT NULL DEFAULT false);
		CREATE TABLE IF NOT EXISTS %[1]s.order_items(id BIGSERIAL PRIMARY KEY,order_id BIGINT NOT NULL,product_id BIGINT NOT NULL);
	`, schema)); err != nil {
		t.Fatal(err)
	}
}

func seedPR600ComponentSpecUnitAuthority(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.product_unit_definitions(code) VALUES('个'),('盒'),('袋') ON CONFLICT(code) DO NOTHING;
		INSERT INTO %[1]s.products(id,name,active) VALUES(1101,'父库存单位为 kg 的商品',true);
		INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,output_material_id,status)
		VALUES(1110,'BOM-COMPONENT-SPECS','组件商品规格 BOM','product',1101,0,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit,published_at,created_at)
		VALUES(1120,1110,'V001','published',1,'盒',now(),now());
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)
		VALUES
			(1201,1110,'BOM-SPEC-BOX','box','盒装','盒'),
			(1202,1110,'BOM-SPEC-BAG','bag','袋装','袋');
		INSERT INTO %[1]s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
		VALUES
			(1120,1201,'盒装','盒',true,1),
			(1120,1202,'袋装','袋',false,2);
	`, schema)); err != nil {
		t.Fatal(err)
	}
}

func pr600ComponentSpecUnitTemplateVariants(specID int64, consumeUnit string) []bomapp.ProductionBomSpecTemplateVariant {
	return []bomapp.ProductionBomSpecTemplateVariant{{
		SpecKey: "unit", Name: "单位规格", InventoryUnit: "个", IsDefault: true,
		Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{
			{IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 1}},
			{ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "product", ComponentProductID: 1101, ComponentBomSpecID: specID, ConsumeUnit: consumeUnit, QtyPerUnit: 1}},
		},
	}}
}

func seedPR600InvalidTemplateUnitDraft(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, versionID, specID int64, consumeUnit string) {
	t.Helper()
	var variantID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_spec_template_variants(version_id,spec_key,name,inventory_unit,is_default)
		VALUES($1,'unit','单位规格','个',true) RETURNING id
	`, schema), versionID).Scan(&variantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_spec_template_variant_items(
			variant_id,is_main_input,material_id,component_type,component_product_id,component_bom_spec_id,consume_unit,qty_per_unit,sort_order
		) VALUES
			($1,true,0,'material',0,0,'main_input_unit',1,1),
			($1,false,0,'product',1101,$2,$3,1,2)
	`, schema), variantID, specID, consumeUnit); err != nil {
		t.Fatal(err)
	}
}

func seedPR600InvalidBOMUnitDraft(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, versionID int64, item bomapp.ProductionBomDraftItem) {
	t.Helper()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_version_items WHERE version_id=$1`, schema), versionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_items(
			version_id,material_id,component_type,component_product_id,component_bom_spec_id,consume_unit,qty_per_unit
		) VALUES($1,0,'product',$2,$3,$4,$5)
	`, schema), versionID, item.ComponentProductID, item.ComponentBomSpecID, item.ConsumeUnit, item.QtyPerUnit); err != nil {
		t.Fatal(err)
	}
}
