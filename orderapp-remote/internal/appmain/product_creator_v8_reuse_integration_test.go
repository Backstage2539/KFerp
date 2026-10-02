package appmain

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	bomapp "orderapp/internal/application/bom"
	creatorapp "orderapp/internal/application/productcreator"
	"testing"
)

// Uses archives produced through the unchanged V3 path, then exercises V8 with
// real domain services and the creator's outer transaction.
func verifyV8ReuseDownstream(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, svc *creatorapp.Service, bomSvc *bomapp.Service, semiID, productID int, routeID int64) {
	for _, kind := range []creatorapp.ModuleKind{creatorapp.ModuleMaterial, creatorapp.ModuleProduct} {
		t.Run("V8 reuse "+string(kind), func(t *testing.T) {
			template, err := bomSvc.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "V8下游规格 " + string(kind), Actor: "integration-test"})
			if err != nil {
				t.Fatal(err)
			}
			versionID := template.Versions[0].ID
			qty := 0.01
			if kind == creatorapp.ModuleProduct {
				qty = 10
			}
			variants := []bomapp.ProductionBomSpecTemplateVariant{{SpecKey: "box", Name: "10件盒装", InventoryUnit: "盒", IsDefault: true, ProcessRouteID: routeID, Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: qty}}}}}
			if _, err = bomSvc.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: versionID, Actor: "integration-test", Variants: variants}); err != nil {
				t.Fatal(err)
			}
			if err = bomSvc.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: versionID, Actor: "integration-test"}); err != nil {
				t.Fatal(err)
			}
			key, id, port := "material_id", semiID, "material"
			upConfig := map[string]any{"output_type": "material"}
			if kind == creatorapp.ModuleProduct {
				key, id, port = "product_id", productID, "specs"
				upConfig = map[string]any{"output_type": "product", "spec_template_version_id": 99999999}
			}
			w := creatorapp.Workflow{Version: 8, Nodes: []creatorapp.Node{
				{ID: "raw", Kind: creatorapp.ModuleMaterial, Name: "不执行生豆", Config: map[string]any{"data_role": "input", "supply_mode": "purchase"}},
				{ID: "route", Kind: creatorapp.ModuleProcess, Name: "不执行烘焙工艺", Config: map[string]any{"route_id": 99999999}},
				{ID: "up", Kind: creatorapp.ModuleBOM, Name: "不执行上游BOM", Config: upConfig},
				{ID: "source", Kind: kind, Name: "已有产出", Config: map[string]any{"data_role": "output", "supply_mode": "manufacture"}},
				{ID: "down", Kind: creatorapp.ModuleBOM, Name: "V8下游盒装", Config: map[string]any{"output_type": "product", "spec_template_version_id": versionID}},
				{ID: "out", Kind: creatorapp.ModuleProduct, Name: "新盒装商品", Config: map[string]any{"data_role": "output"}},
			}, Edges: []creatorapp.Edge{
				productCreatorEdge("1", "raw", "material", "up", "components"), productCreatorEdge("2", "route", "route", "up", "route"), productCreatorEdge("3", "up", "assembly", "source", "from_bom"),
				productCreatorEdge("4", "source", port, "down", "components"), productCreatorEdge("5", "down", "assembly", "out", "from_bom"),
			}}
			saved, err := svc.SaveTemplate(ctx, creatorapp.TemplateSave{Name: "V8复用 " + string(kind), Workflow: w, Actor: "integration-test"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = svc.PublishTemplate(ctx, saved.ID, saved.Revision, "integration-test"); err != nil {
				t.Fatal(err)
			}
			run, err := svc.StartRun(ctx, saved.ID, "integration-test")
			if err != nil {
				t.Fatal(err)
			}
			rowID := "output"
			var specID int64
			if kind == creatorapp.ModuleProduct {
				if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT s.id FROM %s.production_bom_specs s JOIN %s.production_boms b ON b.id=s.bom_id WHERE b.output_product_id=$1 ORDER BY s.id LIMIT 1`, schema, schema), productID).Scan(&specID); err != nil {
					t.Fatal(err)
				}
				rowID = fmt.Sprint(specID)
			}
			inputs := map[string]map[string]any{"source": {"action": "reuse", key: id}, "down": {"main_input_source_node_id": "source", "main_input_source_row_id": rowID}, "out": {"action": "create", "name": "V8新盒装 " + string(kind), "owner": "factory"}}
			run, err = svc.SaveRunDraft(ctx, run.ID, run.Revision, inputs, nil, "integration-test")
			if err != nil {
				t.Fatal(err)
			}
			before, err := productCreatorBusinessCounts(ctx, pool, schema)
			if err != nil {
				t.Fatal(err)
			}
			run, err = svc.PreviewRun(ctx, run.ID, run.Revision, "integration-test")
			if err != nil {
				t.Fatal(err)
			}
			if !run.Preview.Valid {
				t.Fatal(run.Preview.Issues)
			}
			for _, s := range run.Preview.Steps {
				if s.NodeID == "raw" || s.NodeID == "up" || s.NodeID == "route" {
					if s.Status != "skipped" {
						t.Fatal(s)
					}
				}
			}
			afterPreview, _ := productCreatorBusinessCounts(ctx, pool, schema)
			if before != afterPreview {
				t.Fatal("preview wrote business data")
			}
			// A late domain error must roll back the downstream product as well.
			if kind == creatorapp.ModuleProduct {
				if _, err = pool.Exec(ctx, fmt.Sprintf("UPDATE %s.product_unit_definitions SET active=false WHERE code='盒'", schema)); err != nil {
					t.Fatal(err)
				}
				if _, err = svc.CommitConfiguration(ctx, run.ID, run.Revision, "v8-failed", "integration-test"); err == nil {
					t.Fatal("invalid unit accepted")
				}
				afterFailure, _ := productCreatorBusinessCounts(ctx, pool, schema)
				if before != afterFailure {
					t.Fatalf("partial write: %v %v", before, afterFailure)
				}
				if _, err = pool.Exec(ctx, fmt.Sprintf("UPDATE %s.product_unit_definitions SET active=true WHERE code='盒'", schema)); err != nil {
					t.Fatal(err)
				}
			}
			committed, err := svc.CommitConfiguration(ctx, run.ID, run.Revision, "v8-reuse-"+string(kind), "integration-test")
			if err != nil {
				t.Fatal(err)
			}
			after, _ := productCreatorBusinessCounts(ctx, pool, schema)
			want := before
			want[0]++
			want[2]++
			if after != want {
				t.Fatalf("only downstream product and BOM may be created: %v want %v", after, want)
			}
			steps := committed.BusinessResults["steps"].(map[string]any)
			for _, id := range []string{"raw", "up", "route"} {
				if steps[id].(map[string]any)["status"] != "skipped" {
					t.Fatal(steps)
				}
			}
			if _, err = svc.CommitConfiguration(ctx, run.ID, run.Revision, "v8-reuse-"+string(kind), "integration-test"); err == nil {
				t.Fatal("stale duplicate accepted")
			}
			afterRetry, _ := productCreatorBusinessCounts(ctx, pool, schema)
			if afterRetry != after {
				t.Fatal("duplicate writes")
			}
			var materialRef, productRef, specRef int64
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT v.main_input_material_id,v.main_input_product_id,v.main_input_bom_spec_id FROM %s.production_bom_versions v JOIN %s.production_boms b ON b.id=v.bom_id JOIN %s.products p ON p.id=b.output_product_id WHERE p.name=$1 AND v.status='published'`, schema, schema, schema), "V8新盒装 "+string(kind)).Scan(&materialRef, &productRef, &specRef); err != nil {
				t.Fatal(err)
			}
			if kind == creatorapp.ModuleMaterial && materialRef != int64(semiID) {
				t.Fatal("material identity changed")
			}
			if kind == creatorapp.ModuleProduct && (productRef != int64(productID) || specRef != specID) {
				t.Fatal("product/spec identity changed")
			}
		})
	}
}
