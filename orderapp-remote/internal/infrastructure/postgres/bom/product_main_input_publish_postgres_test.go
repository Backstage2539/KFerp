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

func TestProductSpecTemplateBOMProcessRouteOverrideCopiesAllVariantsAndKeepsTemplateUnchanged(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	ensurePR600ProductPublishTestTables(t, ctx, pool, schema)
	preparePR600ComponentSpecCatalogFixture(t, ctx, pool, schema)
	seedPR600ComponentSpecUnitAuthority(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		ALTER TABLE %s.process_routes ADD COLUMN status TEXT NOT NULL DEFAULT 'active';
		INSERT INTO %s.process_routes(id,name,status) VALUES(31,'模板路线A','active'),(32,'模板路线B','active'),(39,'本次覆盖路线','active');
		INSERT INTO %s.products(id,name,active) VALUES(2102,'工艺覆盖商品',true),(2103,'沿用模板工艺商品',true);
	`, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	var mainInputMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit,is_semi_finished)
		VALUES('PC-V5-ROUTE-MAIN-INPUT','工艺覆盖主体物料','bean','kg','kg',true) RETURNING id
	`, schema)).Scan(&mainInputMaterialID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	template, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "工艺覆盖规格模板", Actor: "route-override-test"})
	if err != nil {
		t.Fatal(err)
	}
	templateVersionID := template.Versions[0].ID
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{
		VersionID: templateVersionID, Actor: "route-override-test",
		Variants: []bomapp.ProductionBomSpecTemplateVariant{
			{SpecKey: "200g", Name: "200g", InventoryUnit: "袋", IsDefault: true, SortOrder: 1, ProcessRouteID: 31, Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 200}}}},
			{SpecKey: "500g", Name: "500g", InventoryUnit: "袋", SortOrder: 2, ProcessRouteID: 32, Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 500}}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateVersionID, Actor: "route-override-test"}); err != nil {
		t.Fatal(err)
	}
	create := func(productID int64, override int64, source string) int64 {
		t.Helper()
		created, createErr := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
			Name: "规格模板商品 BOM", OutputType: "product", OutputID: productID, OutputProductID: productID,
			SpecificationMode: bomapp.ProductionBomSpecificationModeSpecGroup, OutputQty: 1,
			SpecTemplateVersionID: templateVersionID, MainInputComponent: bomapp.ProductionBomMainInputComponent{ComponentType: "material", MaterialID: mainInputMaterialID},
			ProcessRouteOverrideID: override, ProcessRouteSource: source, ProcessRouteNodeID: "route-node", Actor: "route-override-test",
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		return created.LatestVersionID
	}
	readRoutes := func(versionID int64) []int64 {
		t.Helper()
		rows, queryErr := pool.Query(ctx, fmt.Sprintf(`SELECT process_route_id FROM %s.production_bom_version_variants WHERE version_id=$1 ORDER BY sort_order`, schema), versionID)
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		defer rows.Close()
		var routes []int64
		for rows.Next() {
			var routeID int64
			if scanErr := rows.Scan(&routeID); scanErr != nil {
				t.Fatal(scanErr)
			}
			routes = append(routes, routeID)
		}
		if rows.Err() != nil {
			t.Fatal(rows.Err())
		}
		return routes
	}
	overriddenVersionID := create(2102, 39, "connected_node")
	if got := readRoutes(overriddenVersionID); len(got) != 2 || got[0] != 39 || got[1] != 39 {
		t.Fatalf("overridden generated variants routes=%v, want both 39", got)
	}
	var routeAuditSource, routeAuditNode, routeAuditID string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT meta->>'process_route_source',meta->>'process_route_node_id',meta->>'process_route_override_id'
		FROM %s.audit_logs
		WHERE entity_type='production_bom' AND action='create'
		  AND entity_id=(SELECT bom_id FROM %s.production_bom_versions WHERE id=$1)
	`, schema, schema), overriddenVersionID).Scan(&routeAuditSource, &routeAuditNode, &routeAuditID); err != nil {
		t.Fatal(err)
	}
	if routeAuditSource != "connected_node" || routeAuditNode != "route-node" || routeAuditID != "39" {
		t.Fatalf("BOM create audit route provenance=%s/%s/%s, want connected_node/route-node/39", routeAuditSource, routeAuditNode, routeAuditID)
	}
	templateDefaultVersionID := create(2103, 0, "specification_template")
	if got := readRoutes(templateDefaultVersionID); len(got) != 2 || got[0] != 31 || got[1] != 32 {
		t.Fatalf("template-generated variants routes=%v, want 31 and 32", got)
	}
	var defaultRouteSource string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT meta->>'process_route_source' FROM %s.audit_logs
		WHERE entity_type='production_bom' AND action='create'
		  AND entity_id=(SELECT bom_id FROM %s.production_bom_versions WHERE id=$1)
	`, schema, schema), templateDefaultVersionID).Scan(&defaultRouteSource); err != nil {
		t.Fatal(err)
	}
	if defaultRouteSource != "specification_template" {
		t.Fatalf("template-default BOM audit route source=%q, want specification_template", defaultRouteSource)
	}
	detail, err := repo.GetProductionBomSpecTemplate(ctx, template.ID, templateVersionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Variants) != 2 || detail.Variants[0].ProcessRouteID != 31 || detail.Variants[1].ProcessRouteID != 32 {
		t.Fatalf("source specification template routes changed: %+v", detail.Variants)
	}
}
