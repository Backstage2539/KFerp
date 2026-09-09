package bom

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	bomapp "orderapp/internal/application/bom"
	bomhttp "orderapp/internal/interfaces/http/bom"
)

func TestDraftCopyResponseAndRollbackPostgres(t *testing.T) {
	for _, component := range []string{"material", "product", "broken-read"} {
		t.Run(component, func(t *testing.T) {
			ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
			_, err := pool.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id) VALUES(1,'COPY','Copy','product',801);
				INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,source_spec_template_version_id,main_input_component_type,main_input_material_id,main_input_product_id,main_input_bom_spec_id)
				VALUES(101,1,'V001','published',123,'%[2]s',7,8,9);
			`, schema, component))
			if err != nil {
				t.Fatal(err)
			}
			if component == "broken-read" {
				if _, err = pool.Exec(ctx, "DROP TABLE "+schema+".process_routes"); err != nil {
					t.Fatal(err)
				}
			}
			e := echo.New()
			bomhttp.RegisterRoutes(e, bomhttp.Dependencies{Bom: bomapp.NewService(NewRepository(pool, schema))})
			req := httptest.NewRequest(http.MethodPost, "/api/production-boms/1/versions", strings.NewReader(`{"source_version_id":101}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			var count int
			if err = pool.QueryRow(ctx, "SELECT count(*) FROM "+schema+".production_bom_versions").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if component == "broken-read" {
				if rec.Code == http.StatusOK || count != 1 {
					t.Fatalf("failed response must roll back: status=%d count=%d body=%s", rec.Code, count, rec.Body.String())
				}
				return
			}
			if rec.Code != http.StatusOK || count != 2 {
				t.Fatalf("copy must commit once and return success: status=%d count=%d body=%s", rec.Code, count, rec.Body.String())
			}
			rows, err := NewRepository(pool, schema).listProductionBomVersions(ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			copied := rows[0]
			if copied.Status != "draft" || copied.MainInputComponent.ComponentType != component || copied.MainInputComponent.MaterialID != 7 || copied.MainInputComponent.ComponentProductID != 8 || copied.MainInputComponent.ComponentBomSpecID != 9 {
				t.Fatalf("lost copy provenance: %+v", copied)
			}
			var templateID int64
			if err = pool.QueryRow(ctx, "SELECT source_spec_template_version_id FROM "+schema+".production_bom_versions WHERE id=$1", copied.ID).Scan(&templateID); err != nil || templateID != 123 {
				t.Fatalf("template=%d err=%v", templateID, err)
			}
		})
	}
}

func TestTemplateReapplyReplacesNineBagSpecsWithTwoKgSpecsPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.product_unit_definitions(code) VALUES('袋'),('kg');
		INSERT INTO %[1]s.products(id,name) VALUES(801,'Template target');
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit) VALUES(701,'MAIN','Main','bean','kg','kg');
		INSERT INTO %[1]s.production_bom_spec_templates(id,code,name) VALUES(11,'ROAST','Roasted'),(12,'GREEN','Green');
		INSERT INTO %[1]s.production_bom_spec_template_versions(id,template_id,version_no,status) VALUES(111,11,'V001','published'),(112,12,'V003','published');
		INSERT INTO %[1]s.production_bom_spec_template_variants(version_id,spec_key,name,inventory_unit,is_default,sort_order)
		SELECT 112,'spec-'||n,CASE WHEN n=1 THEN '1KG' ELSE '500g' END,'kg',n=1,n FROM generate_series(1,2) n;
		INSERT INTO %[1]s.production_bom_spec_template_variant_items(variant_id,is_main_input,component_type,consume_unit,qty_per_unit)
		SELECT id,true,'material','main_input_unit',CASE WHEN sort_order=1 THEN 1 ELSE 0.5 END FROM %[1]s.production_bom_spec_template_variants WHERE version_id=112;
		INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,specification_mode) VALUES(1,'NINE','Nine','product',801,'spec_group');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,source_spec_template_version_id,main_input_material_id)
		VALUES(101,1,'V001','published',111,701),(102,1,'V002','draft',111,701);
		INSERT INTO %[1]s.production_bom_specs(bom_id,code,spec_key,name,inventory_unit)
		SELECT 1,'OLD-'||n,'spec-'||n,'Bag '||n,'袋' FROM generate_series(1,9) n;
		INSERT INTO %[1]s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
		SELECT v,s.id,s.name,'袋',s.spec_key='spec-1',s.id FROM %[1]s.production_bom_specs s CROSS JOIN unnest(ARRAY[101,102]) v;
	`, schema))
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	repo := NewRepository(pool, schema)
	bomhttp.RegisterRoutes(e, bomhttp.Dependencies{Bom: bomapp.NewService(repo)})
	apply := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/production-bom-versions/102/spec-template", strings.NewReader(`{"spec_template_version_id":112,"main_input_material_id":701}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	// Both validation after identity allocation and response decoding after all
	// writes must leave the original nine-spec draft completely untouched.
	for _, failure := range []string{"invalid-unit", "broken-read"} {
		if failure == "invalid-unit" {
			_, err = pool.Exec(ctx, "UPDATE "+schema+".production_bom_spec_template_variants SET inventory_unit='unknown' WHERE version_id=112 AND sort_order=2")
		} else {
			_, err = pool.Exec(ctx, "DROP TABLE "+schema+".process_routes")
		}
		if err != nil {
			t.Fatal(err)
		}
		if rec := apply(); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", failure, rec.Code, rec.Body.String())
		}
		var specs, variants, audits int
		var origin int64
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT count(*) FROM %[1]s.production_bom_specs),(SELECT count(*) FROM %[1]s.production_bom_version_variants WHERE version_id=102),(SELECT count(*) FROM %[1]s.audit_logs),source_spec_template_version_id FROM %[1]s.production_bom_versions WHERE id=102`, schema)).Scan(&specs, &variants, &audits, &origin); err != nil || specs != 9 || variants != 9 || audits != 0 || origin != 111 {
			t.Fatalf("%s failed to roll back specs/variants/audits/origin=%d/%d/%d/%d err=%v", failure, specs, variants, audits, origin, err)
		}
		if failure == "invalid-unit" {
			_, err = pool.Exec(ctx, "UPDATE "+schema+".production_bom_spec_template_variants SET inventory_unit='kg' WHERE version_id=112")
		} else {
			_, err = pool.Exec(ctx, "CREATE TABLE "+schema+".process_routes(id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '')")
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	var firstIDs string
	for attempt := 0; attempt < 2; attempt++ {
		rec := apply()
		if rec.Code != http.StatusOK {
			t.Fatalf("reapply attempt %d: %d %s", attempt, rec.Code, rec.Body.String())
		}
		var ids string
		var count, defaults, materialCount, oldCount int
		err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*),count(*) FILTER(WHERE is_default),string_agg(bom_spec_id::text,',' ORDER BY sort_order) FROM %s.production_bom_version_variants WHERE version_id=102`, schema)).Scan(&count, &defaults, &ids)
		if err != nil || count != 2 || defaults != 1 {
			t.Fatalf("draft count/default=%d/%d err=%v", count, defaults, err)
		}
		if attempt == 0 {
			firstIDs = ids
		} else if ids != firstIDs {
			t.Fatalf("reapply duplicated identities: %s -> %s", firstIDs, ids)
		}
		pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.production_bom_version_items WHERE version_id=102 AND material_id=701 AND consume_unit='kg'`, schema)).Scan(&materialCount)
		pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.production_bom_version_variants WHERE version_id=101 AND inventory_unit='袋'`, schema)).Scan(&oldCount)
		if materialCount != 2 || oldCount != 9 {
			t.Fatalf("material/old snapshot=%d/%d", materialCount, oldCount)
		}
	}
	var mapped int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.production_bom_specs WHERE source_spec_template_id=12 AND inventory_unit='kg'`, schema)).Scan(&mapped); err != nil || mapped != 2 {
		t.Fatalf("persistent mapping=%d err=%v", mapped, err)
	}
	var audited bool
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.audit_logs WHERE action='reapply_spec_template' AND jsonb_array_length(old_value::jsonb)=9 AND jsonb_array_length(new_value::jsonb)=2 AND (new_value::jsonb->0->>'source_spec_template_id')::bigint=12 AND new_value::jsonb->0->>'source_spec_template_key'='spec-1')`, schema)).Scan(&audited); err != nil || !audited {
		t.Fatalf("missing old/new identity audit err=%v", err)
	}
	// The UI uses its selected draft as the replacement source after changing
	// output product. Keep both original versions and generate a new BOM.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.products(id,name) VALUES(802,'Green target'); SELECT setval('%[1]s.production_boms_id_seq',10); SELECT setval('%[1]s.production_bom_versions_id_seq',200);`, schema)); err != nil {
		t.Fatal(err)
	}
	for _, badGroup := range []bool{false, true} {
		group := ""
		if badGroup {
			group = `,"group_id":999,"group_category_id":999`
		}
		req := httptest.NewRequest(http.MethodPost, "/api/production-boms/1/replacement-draft", strings.NewReader(`{"source_version_id":102,"name":"Green replacement","output_type":"product","output_product_id":802,"specification_mode":"spec_group","output_qty":1,"output_unit":"kg","spec_template_version_id":112,"main_input_material_id":701`+group+`}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		want := http.StatusOK
		if badGroup {
			want = http.StatusBadRequest
		}
		if rec.Code != want {
			t.Fatalf("replacement badGroup=%v status=%d body=%s", badGroup, rec.Code, rec.Body.String())
		}
		var boms, targetSpecs, originals int
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT count(*) FROM %[1]s.production_boms),(SELECT count(*) FROM %[1]s.production_bom_specs s JOIN %[1]s.production_boms b ON b.id=s.bom_id WHERE b.output_product_id=802 AND b.source_bom_version_id=102 AND s.source_spec_template_id=12 AND s.inventory_unit='kg'),(SELECT count(*) FROM %[1]s.production_bom_version_variants WHERE version_id=101 AND inventory_unit='袋')`, schema)).Scan(&boms, &targetSpecs, &originals); err != nil || boms != 2 || targetSpecs != 2 || originals != 9 {
			t.Fatalf("replacement/rollback boms/specs/originals=%d/%d/%d err=%v", boms, targetSpecs, originals, err)
		}
	}
	// Validation failure must retain the completed draft and its template provenance.
	if _, err = pool.Exec(ctx, "UPDATE "+schema+".production_bom_spec_template_versions SET status='archived' WHERE id=112"); err != nil {
		t.Fatal(err)
	}
	if rec := apply(); rec.Code != http.StatusBadRequest {
		t.Fatalf("archived template status=%d", rec.Code)
	}
	var finalIDs string
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT string_agg(bom_spec_id::text,',' ORDER BY sort_order) FROM %s.production_bom_version_variants WHERE version_id=102`, schema)).Scan(&finalIDs)
	if finalIDs != firstIDs {
		t.Fatalf("failed reapply changed identities: %s", finalIDs)
	}
}

func TestTemplateSpecIdentityLegacyOriginAndCopyPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.production_bom_spec_templates(id,code,name) VALUES(11,'A','A'),(12,'B','B');
		INSERT INTO %[1]s.production_bom_spec_template_versions(id,template_id,version_no,status) VALUES(111,11,'V001','published');
		INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id) VALUES(1,'A','A','product',801),(2,'B','B','product',802);
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,source_spec_template_version_id) VALUES(101,1,'V001','published',111),(102,2,'V001','draft',111);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,barcode,spec_key,name,inventory_unit) VALUES(100,1,'OLD','BAR','spec-1','Old','袋');
		INSERT INTO %[1]s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default) VALUES(101,100,'Old','袋',true);
	`, schema))
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		if err := ensureProductionBomSpecGroupTables(ctx, pool, schema); err != nil {
			t.Fatal(err)
		}
	}
	var unmapped bool
	if err := pool.QueryRow(ctx, "SELECT source_spec_template_id=0 AND source_spec_template_key='' FROM "+schema+".production_bom_specs WHERE id=100").Scan(&unmapped); err != nil || !unmapped {
		t.Fatalf("idempotent schema initialization must not rewrite history: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	legacy, key, barcode, err := resolveTemplateBomSpecTx(ctx, tx, schema, 1, 11, "spec-1", "Old", "袋", "test")
	if err != nil || legacy != 100 || key != "spec-1" || barcode != "BAR" {
		t.Fatalf("legacy=%d/%s/%s err=%v", legacy, key, barcode, err)
	}
	other, _, _, err := resolveTemplateBomSpecTx(ctx, tx, schema, 1, 12, "spec-1", "Other", "袋", "test")
	if err != nil || other == 100 {
		t.Fatalf("other template reused legacy=%d err=%v", other, err)
	}
	changed, _, _, err := resolveTemplateBomSpecTx(ctx, tx, schema, 1, 11, "spec-1", "New unit", "kg", "test")
	if err != nil || changed == 100 || changed == other {
		t.Fatalf("changed unit reused identity=%d err=%v", changed, err)
	}
	again, _, _, err := resolveTemplateBomSpecTx(ctx, tx, schema, 1, 11, "spec-1", "New unit", "kg", "test")
	if err != nil || again != changed {
		t.Fatalf("repeated identity=%d want=%d err=%v", again, changed, err)
	}
	if err = copyProductionBomVariantsToNewBomTx(ctx, tx, schema, 101, 2, 102, "test"); err != nil {
		t.Fatal(err)
	}
	var origin int64
	var originKey, copiedBarcode string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT source_spec_template_id,source_spec_template_key,barcode FROM %s.production_bom_specs WHERE bom_id=2`, schema)).Scan(&origin, &originKey, &copiedBarcode); err != nil || origin != 11 || originKey != "spec-1" || copiedBarcode != "" {
		t.Fatalf("copy origin=%d/%s barcode=%s err=%v", origin, originKey, copiedBarcode, err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	// Template replacement does not relax direct editing of historical units.
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = updateProductionBomSpecTx(ctx, tx, schema, 100, "Old", "kg", "BAR", "test"); err == nil || !strings.Contains(err.Error(), "cannot be changed") {
		t.Fatalf("manual historical unit guard: %v", err)
	}
}
