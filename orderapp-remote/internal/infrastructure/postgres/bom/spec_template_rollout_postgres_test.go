package bom

import (
	"fmt"
	bomapp "orderapp/internal/application/bom"
	"sync"
	"testing"
)

func newRolloutFixture(t *testing.T) (pr600DefaultSwitchFixture, int64) {
	t.Helper()
	f := newPR600DefaultSwitchFixture(t)
	ctx, repo, pool, schema := f.ctx, f.repo, f.pool, f.schema
	preparePR600ComponentSpecCatalogFixture(t, ctx, pool, schema)
	if err := repo.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: f.first.LatestVersionID, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.BindProductionBomOutput(ctx, bomapp.BindProductionBomOutputCommand{OutputType: "product", OutputID: f.productID, BomID: f.first.ID, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	var source, tid int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT v.source_spec_template_version_id,t.template_id FROM %s.production_bom_versions v JOIN %s.production_bom_spec_template_versions t ON t.id=v.source_spec_template_version_id WHERE v.id=$1`, schema, schema), f.first.LatestVersionID).Scan(&source, &tid); err != nil {
		t.Fatal(err)
	}
	next, err := repo.CreateProductionBomSpecTemplateVersion(ctx, bomapp.CreateProductionBomSpecTemplateVersionCommand{TemplateID: tid, SourceVersionID: source, Actor: "test"})
	if err != nil {
		t.Fatal(err)
	}
	template, err := repo.GetProductionBomSpecTemplate(ctx, tid, next.ID)
	if err != nil {
		t.Fatal(err)
	}
	template.Variants[0].LogisticsWeightG = logisticsTestWeight(250)
	if _, err = repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: next.ID, Variants: template.Variants, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	if err = repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: next.ID, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	return f, next.ID
}

func TestSpecTemplateRolloutAtomicPublishPostgres(t *testing.T) {
	f, nextID := newRolloutFixture(t)
	ctx, repo, pool, schema := f.ctx, f.repo, f.pool, f.schema
	svc := bomapp.NewService(repo)
	preview, err := svc.PreviewSpecTemplateRollout(ctx, nextID)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Rows) != 1 || preview.Rows[0].Status != "ready" {
		t.Fatalf("preview=%+v", preview)
	}
	cmd := bomapp.TemplateRolloutCommand{VersionID: nextID, Targets: []bomapp.TemplateRolloutTarget{{BomID: f.first.ID, ExpectedVersionID: f.first.LatestVersionID}}, Actor: "test"}
	var wg sync.WaitGroup
	results := make([]bomapp.TemplateRolloutResult, 2)
	errs := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i], errs[i] = svc.ApplySpecTemplateRollout(ctx, cmd) }(i)
	}
	wg.Wait()
	updated := 0
	for i, r := range results {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if len(r.Rows) != 1 {
			t.Fatal(r)
		}
		if r.Rows[0].Status == "updated" {
			updated++
		} else if r.Rows[0].Status != "already_updated" {
			t.Fatal(r)
		}
	}
	if updated != 1 {
		t.Fatal(results)
	}
	var count int
	var weight float64
	var specID int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.production_bom_versions WHERE bom_id=$1`, schema), f.first.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT x.bom_spec_id,x.logistics_weight_g::float8 FROM %s.production_bom_version_variants x JOIN %s.production_bom_versions v ON v.id=x.version_id WHERE v.bom_id=$1 AND v.status='published'`, schema, schema), f.first.ID).Scan(&specID, &weight); err != nil || specID != f.firstSpec || weight != 250 {
		t.Fatalf("identity/weight %d %f %v", specID, weight, err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE action='rollout_spec_template'`, schema)).Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit %d %v", count, err)
	}
}

func TestSpecTemplateRolloutGuardsAndRollbackPostgres(t *testing.T) {
	for _, scenario := range []string{"draft", "conflict", "local-name", "local-spec", "stale-bom", "stale-template", "publish-rollback", "unit-change", "new-spec"} {
		t.Run(scenario, func(t *testing.T) {
			f, nextID := newRolloutFixture(t)
			ctx, repo, pool, schema := f.ctx, f.repo, f.pool, f.schema
			exec := func(query string, args ...any) {
				t.Helper()
				if _, err := pool.Exec(ctx, fmt.Sprintf(query, schema), args...); err != nil {
					t.Fatal(err)
				}
			}
			want := "updated"
			switch scenario {
			case "draft":
				if _, err := repo.CreateProductionBomVersion(ctx, bomapp.CreateProductionBomVersionCommand{BomID: f.first.ID, Actor: "test"}); err != nil {
					t.Fatal(err)
				}
				want = "blocked"
			case "conflict":
				exec(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=999 WHERE version_id=$1`, f.first.LatestVersionID)
				want = "blocked"
			case "local-name":
				exec(`UPDATE %s.production_bom_version_variants SET spec_name_snapshot='本地名称' WHERE version_id=$1`, f.first.LatestVersionID)
			case "local-spec":
				exec(`INSERT INTO %s.production_bom_specs(bom_id,code,spec_key,name,inventory_unit) VALUES($1,'LOCAL-TEST','local-extra','本地规格','袋')`, f.first.ID)
				exec(`INSERT INTO %[1]s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,logistics_weight_g) SELECT $1,id,name,inventory_unit,false,5,227 FROM %[1]s.production_bom_specs WHERE code='LOCAL-TEST'`, f.first.LatestVersionID)
				exec(`INSERT INTO %[1]s.production_bom_version_items(version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit) SELECT $1,v.id,i.material_id,i.component_type,i.consume_unit,i.qty_per_unit FROM %[1]s.production_bom_version_variants v JOIN %[1]s.production_bom_specs s ON s.id=v.bom_spec_id JOIN %[1]s.production_bom_version_items i ON i.version_id=$1 AND i.variant_id<>v.id WHERE s.code='LOCAL-TEST' AND v.version_id=$1`, f.first.LatestVersionID)
			case "stale-bom":
				v, err := repo.CreateProductionBomVersion(ctx, bomapp.CreateProductionBomVersionCommand{BomID: f.first.ID, Actor: "test"})
				if err != nil {
					t.Fatal(err)
				}
				if err = repo.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: v.ID, Actor: "test"}); err != nil {
					t.Fatal(err)
				}
				want = "blocked"
			case "stale-template":
				var tid int64
				if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT template_id FROM %s.production_bom_spec_template_versions WHERE id=$1`, schema), nextID).Scan(&tid); err != nil {
					t.Fatal(err)
				}
				v, err := repo.CreateProductionBomSpecTemplateVersion(ctx, bomapp.CreateProductionBomSpecTemplateVersionCommand{TemplateID: tid, Actor: "test"})
				if err != nil {
					t.Fatal(err)
				}
				if err = repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: v.ID, Actor: "test"}); err != nil {
					t.Fatal(err)
				}
				want = "failed"
			case "publish-rollback":
				exec(`UPDATE %s.production_bom_spec_template_variants SET logistics_weight_g=NULL WHERE version_id=$1`, nextID)
				want = "failed"
			case "unit-change":
				exec(`INSERT INTO %s.product_unit_definitions(code) VALUES('盒') ON CONFLICT DO NOTHING`)
				exec(`UPDATE %s.production_bom_spec_template_variants SET inventory_unit='盒' WHERE version_id=$1`, nextID)
			case "new-spec":
				exec(`INSERT INTO %s.production_bom_spec_template_variants(version_id,spec_key,name,inventory_unit,is_default,sort_order,logistics_weight_g) VALUES($1,'new-spec','新增规格','袋',false,3,500)`, nextID)
				exec(`INSERT INTO %[1]s.production_bom_spec_template_variant_items(variant_id,is_main_input,component_type,consume_unit,qty_per_unit) SELECT id,true,'material','main_input_unit',1 FROM %[1]s.production_bom_spec_template_variants WHERE version_id=$1 AND spec_key='new-spec'`, nextID)
			}
			var before int
			pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.production_bom_versions WHERE bom_id=$1`, schema), f.first.ID).Scan(&before)
			result, err := bomapp.NewService(repo).ApplySpecTemplateRollout(ctx, bomapp.TemplateRolloutCommand{VersionID: nextID, Targets: []bomapp.TemplateRolloutTarget{{BomID: f.first.ID, ExpectedVersionID: f.first.LatestVersionID}}, Actor: "test"})
			if err != nil || len(result.Rows) != 1 || result.Rows[0].Status != want {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			var after int
			pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.production_bom_versions WHERE bom_id=$1`, schema), f.first.ID).Scan(&after)
			if want != "updated" {
				if before != after {
					t.Fatalf("failure left draft: %d -> %d", before, after)
				}
				return
			}
			if scenario == "local-name" {
				var route string
				pool.QueryRow(ctx, fmt.Sprintf(`SELECT spec_name_snapshot FROM %s.production_bom_version_variants WHERE version_id=$1 AND is_default`, schema), result.Rows[0].NewVersionID).Scan(&route)
				if route != "本地名称" {
					t.Fatalf("local name lost %s", route)
				}
			}
			if scenario == "local-spec" {
				var n int
				pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %[1]s.production_bom_version_variants v JOIN %[1]s.production_bom_specs s ON s.id=v.bom_spec_id WHERE v.version_id=$1 AND s.code='LOCAL-TEST' AND s.source_spec_template_id=0`, schema), result.Rows[0].NewVersionID).Scan(&n)
				if n != 1 {
					t.Fatal("local spec lost or provenance changed")
				}
			}
			if scenario == "unit-change" {
				var spec int64
				pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_spec_id FROM %s.production_bom_version_variants WHERE version_id=$1 AND is_default`, schema), result.Rows[0].NewVersionID).Scan(&spec)
				if spec == f.firstSpec {
					t.Fatal("unit changed without new stable identity")
				}
			}
		})
	}
}

func TestSpecTemplateRolloutMultipleOldVersionsAndPartialSuccessPostgres(t *testing.T) {
	f, v2 := newRolloutFixture(t)
	ctx, repo, pool, schema := f.ctx, f.repo, f.pool, f.schema
	svc := bomapp.NewService(repo)
	first, err := svc.ApplySpecTemplateRollout(ctx, bomapp.TemplateRolloutCommand{VersionID: v2, Targets: []bomapp.TemplateRolloutTarget{{BomID: f.first.ID, ExpectedVersionID: f.first.LatestVersionID}}, Actor: "test"})
	if err != nil || first.Rows[0].Status != "updated" {
		t.Fatalf("%+v %v", first, err)
	}
	if err = repo.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: f.second.LatestVersionID, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	var tid int64
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT template_id FROM %s.production_bom_spec_template_versions WHERE id=$1`, schema), v2).Scan(&tid)
	v3, err := repo.CreateProductionBomSpecTemplateVersion(ctx, bomapp.CreateProductionBomSpecTemplateVersionCommand{TemplateID: tid, Actor: "test"})
	if err != nil {
		t.Fatal(err)
	}
	template, err := repo.GetProductionBomSpecTemplate(ctx, tid, v3.ID)
	if err != nil {
		t.Fatal(err)
	}
	template.Variants[0].LogisticsWeightG = logisticsTestWeight(275)
	if _, err = repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: v3.ID, Variants: template.Variants, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	if err = repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: v3.ID, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=999 WHERE version_id=$1`, schema), f.second.LatestVersionID); err != nil {
		t.Fatal(err)
	}
	preview, err := svc.PreviewSpecTemplateRollout(ctx, v3.ID)
	if err != nil || len(preview.Rows) != 2 {
		t.Fatalf("%+v %v", preview, err)
	}
	if preview.Rows[0].SourceTemplateVersionID == preview.Rows[1].SourceTemplateVersionID {
		t.Fatal("fixture did not cover multiple source versions")
	}
	result, err := svc.ApplySpecTemplateRollout(ctx, bomapp.TemplateRolloutCommand{VersionID: v3.ID, Targets: []bomapp.TemplateRolloutTarget{{BomID: f.first.ID, ExpectedVersionID: first.Rows[0].NewVersionID}, {BomID: f.second.ID, ExpectedVersionID: f.second.LatestVersionID}}, Actor: "test"})
	if err != nil || result.Rows[0].Status != "updated" || result.Rows[1].Status != "blocked" {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestSpecTemplateRolloutRemovalChecksPostgres(t *testing.T) {
	for _, mode := range []string{"remove", "local-edit", "stock"} {
		t.Run(mode, func(t *testing.T) {
			f, v2 := newRolloutFixture(t)
			ctx, repo, pool, schema := f.ctx, f.repo, f.pool, f.schema
			svc := bomapp.NewService(repo)
			exec := func(q string, args ...any) {
				t.Helper()
				if _, err := pool.Exec(ctx, fmt.Sprintf(q, schema), args...); err != nil {
					t.Fatal(err)
				}
			}
			exec(`INSERT INTO %s.production_bom_spec_template_variants(version_id,spec_key,name,inventory_unit,is_default,sort_order,logistics_weight_g) VALUES($1,'extra','附加规格','袋',false,3,500)`, v2)
			exec(`INSERT INTO %[1]s.production_bom_spec_template_variant_items(variant_id,is_main_input,component_type,consume_unit,qty_per_unit) SELECT id,true,'material','main_input_unit',1 FROM %[1]s.production_bom_spec_template_variants WHERE version_id=$1 AND spec_key='extra'`, v2)
			first, err := svc.ApplySpecTemplateRollout(ctx, bomapp.TemplateRolloutCommand{VersionID: v2, Targets: []bomapp.TemplateRolloutTarget{{BomID: f.first.ID, ExpectedVersionID: f.first.LatestVersionID}}, Actor: "test"})
			if err != nil || first.Rows[0].Status != "updated" {
				t.Fatalf("%+v %v", first, err)
			}
			current := first.Rows[0].NewVersionID
			var tid, extra int64
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT template_id FROM %s.production_bom_spec_template_versions WHERE id=$1`, schema), v2).Scan(&tid); err != nil {
				t.Fatal(err)
			}
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_spec_id FROM %[1]s.production_bom_version_variants v JOIN %[1]s.production_bom_specs s ON s.id=v.bom_spec_id WHERE v.version_id=$1 AND s.source_spec_template_key='extra'`, schema), current).Scan(&extra); err != nil {
				t.Fatal(err)
			}
			v3, err := repo.CreateProductionBomSpecTemplateVersion(ctx, bomapp.CreateProductionBomSpecTemplateVersionCommand{TemplateID: tid, Actor: "test"})
			if err != nil {
				t.Fatal(err)
			}
			tpl, err := repo.GetProductionBomSpecTemplate(ctx, tid, v3.ID)
			if err != nil {
				t.Fatal(err)
			}
			keep := []bomapp.ProductionBomSpecTemplateVariant{}
			for _, v := range tpl.Variants {
				if v.SpecKey != "extra" {
					keep = append(keep, v)
				}
			}
			if _, err = repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: v3.ID, Variants: keep, Actor: "test"}); err != nil {
				t.Fatal(err)
			}
			if err = repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: v3.ID, Actor: "test"}); err != nil {
				t.Fatal(err)
			}
			want := "updated"
			if mode == "local-edit" {
				exec(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=501 WHERE version_id=$1 AND bom_spec_id=$2`, current, extra)
				want = "blocked"
			}
			if mode == "stock" {
				exec(`CREATE TABLE %s.finished_inventory(product_id bigint,bom_spec_id bigint,onhand_units bigint,onhand_loose_g bigint)`)
				exec(`INSERT INTO %s.finished_inventory VALUES($1,$2,1,0)`, f.productID, extra)
				want = "failed"
			}
			result, err := svc.ApplySpecTemplateRollout(ctx, bomapp.TemplateRolloutCommand{VersionID: v3.ID, Targets: []bomapp.TemplateRolloutTarget{{BomID: f.first.ID, ExpectedVersionID: current}}, Actor: "test"})
			if err != nil || result.Rows[0].Status != want {
				t.Fatalf("%+v %v", result, err)
			}
			var versions, oldSpecs int
			pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.production_bom_versions WHERE bom_id=$1`, schema), f.first.ID).Scan(&versions)
			expected := 2
			if mode == "remove" {
				expected = 3
			}
			if versions != expected {
				t.Fatalf("versions=%d", versions)
			}
			pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.production_bom_version_variants WHERE version_id=$1`, schema), current).Scan(&oldSpecs)
			if oldSpecs != 2 {
				t.Fatal("historical variants modified")
			}
		})
	}
}

func TestSpecTemplateRolloutExpandsProductMainInput(t *testing.T) {
	template := []bomapp.ProductionBomSpecTemplateVariant{{SpecKey: "bag", Name: "bag", InventoryUnit: "袋", LogisticsWeightG: logisticsTestWeight(250), Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 2}}}}}
	main := bomapp.ProductionBomMainInputComponent{ComponentType: "product", ComponentProductID: 22, ComponentBomSpecID: 33}
	old := expandRolloutTemplate(template, main, "袋")
	current := expandRolloutTemplate(template, main, "袋")
	template[0].LogisticsWeightG = logisticsTestWeight(270)
	result := bomapp.MergeTemplateRollout(old, expandRolloutTemplate(template, main, "袋"), current)
	if len(result.Conflicts) > 0 || len(result.Changes) != 1 || result.Specs[0].Variant.Items[0].ComponentBomSpecID != 33 {
		t.Fatalf("main product produced false conflict: %+v", result)
	}
}
