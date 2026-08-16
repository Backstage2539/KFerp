package bom

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	bomapp "orderapp/internal/application/bom"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPublishProductionBomSpecTemplateVersionValidatesEveryConcreteComponentPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.product_unit_definitions(code) VALUES('袋'),('箱'),('kg') ON CONFLICT(code) DO NOTHING;
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,deprecated_at) VALUES
			(5001,'PR600-TPL-MAT-ACTIVE','有效包材','packaging','kg','kg',NULL),
			(5002,'PR600-TPL-MAT-INACTIVE','失效包材','packaging','kg','kg',now());
		INSERT INTO %[1]s.products(id,name,active) VALUES
			(6001,'有效组件商品',true),
			(6002,'其他有效商品',true),
			(6003,'失效组件商品',false);

		INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,status) VALUES
			(6101,'PR600-COMP-BOM-1','有效组件 BOM','product',6001,'active'),
			(6102,'PR600-COMP-BOM-2','其他商品 BOM','product',6002,'active'),
			(6103,'PR600-COMP-BOM-3','失效商品 BOM','product',6003,'active'),
			(6104,'PR600-COMP-BOM-4','未发布组件 BOM','product',6001,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,published_at) VALUES
			(6201,6101,'V001','published',now()),
			(6202,6102,'V001','published',now()),
			(6203,6103,'V001','published',now()),
			(6204,6104,'V001','draft',NULL);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit) VALUES
			(6301,6101,'BOM-SPEC-006301','default','有效规格','袋'),
			(6302,6102,'BOM-SPEC-006302','default','其他商品规格','袋'),
			(6303,6103,'BOM-SPEC-006303','default','失效商品规格','袋'),
			(6304,6104,'BOM-SPEC-006304','default','未发布规格','袋');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order
		) VALUES
			(6401,6201,6301,'有效规格','袋',true,1),
			(6402,6202,6302,'其他商品规格','袋',true,1),
			(6403,6203,6303,'失效商品规格','袋',true,1),
			(6404,6204,6304,'未发布规格','袋',true,1);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	mainInput := bomapp.ProductionBomSpecTemplateVariantDraftItem{
		IsMainInput: true,
		ProductionBomDraftItem: bomapp.ProductionBomDraftItem{
			ComponentType: "material",
			ConsumeUnit:   "main_input_unit",
			QtyPerUnit:    0.227,
		},
	}
	cases := []struct {
		name      string
		item      bomapp.ProductionBomSpecTemplateVariantDraftItem
		wantError string
	}{
		{
			name: "missing material",
			item: bomapp.ProductionBomSpecTemplateVariantDraftItem{ProductionBomDraftItem: bomapp.ProductionBomDraftItem{
				MaterialID: 5999, ComponentType: "material", ConsumeUnit: "kg", QtyPerUnit: 0.01,
			}},
			wantError: "component material not found",
		},
		{
			name: "inactive material",
			item: bomapp.ProductionBomSpecTemplateVariantDraftItem{ProductionBomDraftItem: bomapp.ProductionBomDraftItem{
				MaterialID: 5002, ComponentType: "material", ConsumeUnit: "kg", QtyPerUnit: 0.01,
			}},
			wantError: "component material is inactive",
		},
		{
			name: "zero fixed material quantity",
			item: bomapp.ProductionBomSpecTemplateVariantDraftItem{ProductionBomDraftItem: bomapp.ProductionBomDraftItem{
				MaterialID: 5001, ComponentType: "material", ConsumeUnit: "kg",
			}},
			wantError: "qty_per_unit required",
		},
		{
			name: "material inventory unit mismatch",
			item: bomapp.ProductionBomSpecTemplateVariantDraftItem{ProductionBomDraftItem: bomapp.ProductionBomDraftItem{
				MaterialID: 5001, ComponentType: "material", ConsumeUnit: "箱", QtyPerUnit: 1,
			}},
			wantError: "consume_unit must match component inventory_unit",
		},
		{
			name: "product without BOM specification identity",
			item: bomapp.ProductionBomSpecTemplateVariantDraftItem{ProductionBomDraftItem: bomapp.ProductionBomDraftItem{
				ComponentType: "product", ComponentProductID: 6001, ConsumeUnit: "unit_per_box", QtyPerUnit: 1,
			}},
			wantError: "product component requires component_bom_spec_id",
		},
		{
			name: "inactive product",
			item: bomapp.ProductionBomSpecTemplateVariantDraftItem{ProductionBomDraftItem: bomapp.ProductionBomDraftItem{
				ComponentType: "product", ComponentProductID: 6003, ComponentBomSpecID: 6303, ConsumeUnit: "袋", QtyPerUnit: 1,
			}},
			wantError: "component product is inactive",
		},
		{
			name: "specification belongs to another product",
			item: bomapp.ProductionBomSpecTemplateVariantDraftItem{ProductionBomDraftItem: bomapp.ProductionBomDraftItem{
				ComponentType: "product", ComponentProductID: 6001, ComponentBomSpecID: 6302, ConsumeUnit: "袋", QtyPerUnit: 1,
			}},
			wantError: "does not belong to component product",
		},
		{
			name: "specification has no published authority",
			item: bomapp.ProductionBomSpecTemplateVariantDraftItem{ProductionBomDraftItem: bomapp.ProductionBomDraftItem{
				ComponentType: "product", ComponentProductID: 6001, ComponentBomSpecID: 6304, ConsumeUnit: "袋", QtyPerUnit: 1,
			}},
			wantError: "has no published version",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			template, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{
				Name: "非法组件-" + testCase.name, Actor: "template-component-guard",
			})
			if err != nil {
				t.Fatal(err)
			}
			versionID := template.Versions[0].ID
			_, err = repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{
				VersionID: versionID,
				Actor:     "template-component-guard",
				Variants: []bomapp.ProductionBomSpecTemplateVariant{{
					SpecKey: "default", Name: "默认规格", InventoryUnit: "袋", IsDefault: true,
					Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{mainInput, testCase.item},
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			err = repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{
				VersionID: versionID, Actor: "template-component-guard",
			})
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("publish error=%v, want %q", err, testCase.wantError)
			}
			assertRejectedPR600TemplatePublishIsAtomic(t, ctx, pool, schema, versionID)
		})
	}

	validTemplate, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{
		Name: "合法完整组件模板", Actor: "template-component-guard",
	})
	if err != nil {
		t.Fatal(err)
	}
	validVersionID := validTemplate.Versions[0].ID
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{
		VersionID: validVersionID,
		Actor:     "template-component-guard",
		Variants: []bomapp.ProductionBomSpecTemplateVariant{{
			SpecKey: "default", Name: "默认规格", InventoryUnit: "袋", IsDefault: true,
			Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{
				mainInput,
				{ProductionBomDraftItem: bomapp.ProductionBomDraftItem{MaterialID: 5001, ComponentType: "material", ConsumeUnit: "kg", QtyPerUnit: 0.01}},
				{ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "product", ComponentProductID: 6001, ComponentBomSpecID: 6301, ConsumeUnit: "袋", QtyPerUnit: 1}},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{
		VersionID: validVersionID, Actor: "template-component-guard",
	}); err != nil {
		t.Fatalf("valid concrete components rejected: %v", err)
	}
}

func TestRejectedProductionBomSpecTemplateVersionKeepsCurrentPublishedVersionPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.product_unit_definitions(code) VALUES('袋') ON CONFLICT(code) DO NOTHING;
	`, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	template, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{
		Name: "原子发布模板", Actor: "template-atomic-guard",
	})
	if err != nil {
		t.Fatal(err)
	}
	v1 := template.Versions[0].ID
	validVariants := []bomapp.ProductionBomSpecTemplateVariant{{
		SpecKey: "default", Name: "默认规格", InventoryUnit: "袋", IsDefault: true,
		Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{
			IsMainInput: true,
			ProductionBomDraftItem: bomapp.ProductionBomDraftItem{
				ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 0.227,
			},
		}},
	}}
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{
		VersionID: v1, Variants: validVariants, Actor: "template-atomic-guard",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{
		VersionID: v1, Actor: "template-atomic-guard",
	}); err != nil {
		t.Fatal(err)
	}
	v2, err := repo.CreateProductionBomSpecTemplateVersion(ctx, bomapp.CreateProductionBomSpecTemplateVersionCommand{
		TemplateID: template.ID, SourceVersionID: v1, Actor: "template-atomic-guard",
	})
	if err != nil {
		t.Fatal(err)
	}
	invalidVariants := append([]bomapp.ProductionBomSpecTemplateVariant(nil), validVariants...)
	invalidVariants[0].Items = append([]bomapp.ProductionBomSpecTemplateVariantDraftItem(nil), validVariants[0].Items...)
	invalidVariants[0].Items = append(invalidVariants[0].Items, bomapp.ProductionBomSpecTemplateVariantDraftItem{
		ProductionBomDraftItem: bomapp.ProductionBomDraftItem{
			MaterialID: 999999, ComponentType: "material", ConsumeUnit: "kg", QtyPerUnit: 1,
		},
	})
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{
		VersionID: v2.ID, Variants: invalidVariants, Actor: "template-atomic-guard",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{
		VersionID: v2.ID, Actor: "template-atomic-guard",
	}); err == nil || !strings.Contains(err.Error(), "component material not found") {
		t.Fatalf("invalid replacement publish error=%v", err)
	}
	assertRejectedPR600TemplatePublishIsAtomic(t, ctx, pool, schema, v2.ID)
	var v1Status string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.production_bom_spec_template_versions WHERE id=$1`, schema), v1).Scan(&v1Status); err != nil {
		t.Fatal(err)
	}
	if v1Status != "published" {
		t.Fatalf("current template version status=%q, want published", v1Status)
	}
}

func TestProductBOMRequiresSpecGroupFailsClosedWhenMigrationTableMissingPostgres(t *testing.T) {
	ctx, pool, schema := newPR600SpecTemplatePublishTestDB(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = productBOMRequiresSpecGroupTx(ctx, tx, schema, 7001)
	_ = tx.Rollback(ctx)
	if err == nil {
		t.Fatal("missing product_bom_spec_migrations table must fail closed")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42P01" {
		t.Fatalf("missing migration table error=%T %v, want SQLSTATE 42P01", err, err)
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.product_bom_spec_migrations(
			product_id BIGINT PRIMARY KEY,
			state TEXT NOT NULL,
			legacy_catalog_product BOOLEAN NOT NULL
		)
	`, schema)); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requiresSpecGroup, err := productBOMRequiresSpecGroupTx(ctx, tx, schema, 7001)
	_ = tx.Rollback(ctx)
	if err != nil {
		t.Fatalf("existing migration table with no product row must keep legacy compatibility: %v", err)
	}
	if requiresSpecGroup {
		t.Fatal("missing product row must not require a specification group")
	}
}

func assertRejectedPR600TemplatePublishIsAtomic(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, versionID int64) {
	t.Helper()
	var status string
	var publishAudits int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT status FROM %s.production_bom_spec_template_versions WHERE id=$1
	`, schema), versionID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.audit_logs
		WHERE entity_type='production_bom_spec_template_version'
		  AND entity_id=$1 AND action='publish'
	`, schema), versionID).Scan(&publishAudits); err != nil {
		t.Fatal(err)
	}
	if status != "draft" || publishAudits != 0 {
		t.Fatalf("rejected version status/audits=%s/%d, want draft/0", status, publishAudits)
	}
}
