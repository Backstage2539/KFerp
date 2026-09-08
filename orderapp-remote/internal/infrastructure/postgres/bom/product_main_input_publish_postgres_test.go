package bom

import (
	"fmt"
	"strings"
	"testing"

	bomapp "orderapp/internal/application/bom"
)

func TestProductMainInputTemplateBOMPublishPostgres(t *testing.T) {
	for _, tc := range []struct {
		name      string
		mutation  string
		wantError string
	}{
		{name: "published template and product specification"},
		{name: "historically published template", mutation: "UPDATE %[1]s.production_bom_spec_template_versions SET status='archived' WHERE id=$1"},
		{name: "inactive product", mutation: "UPDATE %[1]s.products SET active=false WHERE id=1101", wantError: "inactive"},
		{name: "unpublished product specification", mutation: "UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=1120", wantError: "published"},
		{name: "missing provenance specification", mutation: "UPDATE %[1]s.production_bom_versions SET main_input_bom_spec_id=0 WHERE id=$2", wantError: "规格主体组件"},
		{name: "mismatched provenance product", mutation: "UPDATE %[1]s.production_bom_versions SET main_input_product_id=2101 WHERE id=$2", wantError: "规格主体商品"},
		{name: "missing template provenance", mutation: "UPDATE %[1]s.production_bom_versions SET source_spec_template_version_id=0 WHERE id=$2", wantError: "必须同时配置"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
			ensurePR600ProductPublishTestTables(t, ctx, pool, schema)
			preparePR600ComponentSpecCatalogFixture(t, ctx, pool, schema)
			seedPR600ComponentSpecUnitAuthority(t, ctx, pool, schema)
			if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.products(id,name,active) VALUES(2101,'盒装商品',true)`, schema)); err != nil {
				t.Fatal(err)
			}
			repo := NewRepository(pool, schema)
			template, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "盒装模板", Actor: "main-input-test"})
			if err != nil {
				t.Fatal(err)
			}
			templateVersionID := template.Versions[0].ID
			if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{
				VersionID: templateVersionID, Actor: "main-input-test",
				Variants: []bomapp.ProductionBomSpecTemplateVariant{{
					SpecKey: "box", Name: "盒装", InventoryUnit: "盒", IsDefault: true,
					Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{
						IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 10},
					}},
				}},
			}); err != nil {
				t.Fatal(err)
			}
			if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateVersionID, Actor: "main-input-test"}); err != nil {
				t.Fatal(err)
			}
			created, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
				Name: "盒装 BOM", OutputType: "product", OutputID: 2101, OutputProductID: 2101,
				SpecificationMode: bomapp.ProductionBomSpecificationModeSpecGroup, OutputQty: 1, OutputUnit: "盒",
				SpecTemplateVersionID: templateVersionID, Actor: "main-input-test",
				MainInputComponent: bomapp.ProductionBomMainInputComponent{ComponentType: "product", ComponentProductID: 1101, ComponentBomSpecID: 1202},
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.mutation != "" {
				// Use a parameterized CTE so every mutation binds the same fixture IDs.
				query := "WITH fixture AS (SELECT $1::bigint AS template_id,$2::bigint AS version_id) " + fmt.Sprintf(tc.mutation, schema)
				if _, err := pool.Exec(ctx, query, templateVersionID, created.LatestVersionID); err != nil {
					t.Fatal(err)
				}
			}
			err = repo.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: created.LatestVersionID, Actor: "main-input-test"})
			var status string
			var materialID, productID, specID, auditCount int64
			if scanErr := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status,main_input_material_id,main_input_product_id,main_input_bom_spec_id FROM %s.production_bom_versions WHERE id=$1`, schema), created.LatestVersionID).Scan(&status, &materialID, &productID, &specID); scanErr != nil {
				t.Fatal(scanErr)
			}
			if scanErr := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='production_bom_version' AND entity_id=$1 AND action='publish_production_bom_version'`, schema), created.LatestVersionID).Scan(&auditCount); scanErr != nil {
				t.Fatal(scanErr)
			}
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("publish error=%v want %q", err, tc.wantError)
				}
				if status != "draft" || auditCount != 0 {
					t.Fatalf("rejected publication changed status/audit=%s/%d", status, auditCount)
				}
				return
			}
			if err != nil {
				t.Fatalf("publish product main input: %v", err)
			}
			if status != "published" || materialID != 0 || productID != 1101 || specID != 1202 || auditCount != 1 {
				t.Fatalf("published status/material/product/spec/audit=%s/%d/%d/%d/%d", status, materialID, productID, specID, auditCount)
			}
			if _, err := repo.BindProductProductionBom(ctx, bomapp.BindProductProductionBomCommand{ProductID: 2101, BomID: created.ID, Actor: "main-input-test"}); err != nil {
				t.Fatalf("bind product main input BOM: %v", err)
			}
			if _, err := repo.BindProductionBomOutput(ctx, bomapp.BindProductionBomOutputCommand{OutputType: "product", OutputID: 2101, BomID: created.ID, Actor: "main-input-test"}); err != nil {
				t.Fatalf("bind output for product main input BOM: %v", err)
			}
		})
	}
}
