package appmain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	bomapp "orderapp/internal/application/bom"
	postgresbom "orderapp/internal/infrastructure/postgres/bom"
	bomhttp "orderapp/internal/interfaces/http/bom"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestPR600SpecificationTemplateCopyAndStableBomSpecIdentityPostgresAPI(t *testing.T) {
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
	defer pool.Close()
	schema := fmt.Sprintf("pr600_spec_group_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if err := ensureAppSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_definitions(code,name,unit_type,allow_decimal,active)
		VALUES('袋','袋','package',false,true),('个','个','package',false,true)
		ON CONFLICT(code) DO UPDATE SET active=true,deleted_at=NULL
	`, schema)); err != nil {
		t.Fatal(err)
	}
	var productID, mainMaterialID, bagMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.products(name,active,product_kind,unit_rule_override_json) VALUES('规格组商品',true,'roasted_bean','{"inventory_unit":"袋"}'::jsonb) RETURNING id`, schema)).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,unit,cost_unit,purchase_price) VALUES('MAT-PR600-MAIN','熟豆','bean','kg','kg',80) RETURNING id`, schema)).Scan(&mainMaterialID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,unit,cost_unit,purchase_price) VALUES('MAT-PR600-BAG','包装袋','pack','个','个',1) RETURNING id`, schema)).Scan(&bagMaterialID); err != nil {
		t.Fatal(err)
	}

	repo := postgresbom.NewRepository(pool, schema)
	e := echo.New()
	bomhttp.RegisterRoutes(e, bomhttp.Dependencies{Bom: bomapp.NewService(repo)})

	createdTemplate := pr600APIJSON(t, e, http.MethodPost, "/api/production-bom-spec-templates", map[string]any{"name": "熟豆包装规格"}, http.StatusOK)
	templateID := pr600JSONInt64(t, createdTemplate, "id")
	versions := createdTemplate["versions"].([]any)
	templateVersionID := pr600JSONInt64(t, versions[0].(map[string]any), "id")
	variantsPayload := []map[string]any{
		{
			"spec_key": "227g", "name": "227g 袋装", "inventory_unit": "袋", "is_default": true, "sort_order": 10,
			"items": []map[string]any{
				{"is_main_input": true, "component_type": "material", "material_id": 0, "consume_unit": "main_input_unit", "qty_per_unit": 0.227, "sort_order": 10},
				{"component_type": "material", "material_id": bagMaterialID, "consume_unit": "个", "qty_per_unit": 1, "sort_order": 20},
			},
		},
		{
			"spec_key": "454g", "name": "454g 袋装", "inventory_unit": "袋", "is_default": false, "sort_order": 20,
			"items": []map[string]any{
				{"is_main_input": true, "component_type": "material", "material_id": 0, "consume_unit": "main_input_unit", "qty_per_unit": 0.454, "sort_order": 10},
				{"component_type": "material", "material_id": bagMaterialID, "consume_unit": "个", "qty_per_unit": 1, "sort_order": 20},
			},
		},
	}
	for index := 3; index <= 10; index++ {
		variantsPayload = append(variantsPayload, map[string]any{
			"spec_key": fmt.Sprintf("bag-%02d", index), "name": fmt.Sprintf("规格袋 %02d", index),
			"inventory_unit": "袋", "is_default": false, "sort_order": index * 10,
			"items": []map[string]any{
				{"is_main_input": true, "component_type": "material", "material_id": 0, "consume_unit": "main_input_unit", "qty_per_unit": 0.1 * float64(index), "sort_order": 10},
				{"component_type": "material", "material_id": bagMaterialID, "consume_unit": "个", "qty_per_unit": 1, "sort_order": 20},
			},
		})
	}
	pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/production-bom-spec-template-versions/%d/draft", templateVersionID), map[string]any{"variants": variantsPayload}, http.StatusOK)
	pr600APIJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-bom-spec-template-versions/%d/publish", templateVersionID), nil, http.StatusOK)
	listedRows := pr600APIJSONArray(t, e, http.MethodGet, "/api/production-bom-spec-templates", http.StatusOK)
	if len(listedRows) != 1 {
		t.Fatalf("listed specification templates = %v, want one published template", listedRows)
	}
	listedVersions, _ := listedRows[0].(map[string]any)["versions"].([]any)
	if len(listedVersions) != 1 || pr600JSONInt64(t, listedVersions[0].(map[string]any), "id") != templateVersionID {
		t.Fatalf("list endpoint omitted published template version needed by BOM create: %v", listedRows[0])
	}

	firstBom := pr600APIJSON(t, e, http.MethodPost, "/api/production-boms", map[string]any{
		"name": "规格组 BOM A", "output_type": "product", "output_product_id": productID,
		"output_qty": 1, "spec_template_version_id": templateVersionID, "main_input_material_id": mainMaterialID,
	}, http.StatusOK)
	firstBomID := pr600JSONInt64(t, firstBom, "id")
	firstVersionID := pr600JSONInt64(t, firstBom, "latest_version_id")
	firstDetail := pr600APIJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-boms/%d", firstBomID), nil, http.StatusOK)
	firstVariants := firstDetail["variants"].([]any)
	if len(firstVariants) != 10 {
		t.Fatalf("copied variants = %d, want 10: %v", len(firstVariants), firstDetail)
	}
	firstSpecIDs := make([]int64, 0, len(firstVariants))
	for _, raw := range firstVariants {
		variant := raw.(map[string]any)
		firstSpecIDs = append(firstSpecIDs, pr600JSONInt64(t, variant, "bom_spec_id"))
		items := variant["items"].([]any)
		if len(items) != 2 || pr600JSONInt64(t, items[0].(map[string]any), "material_id") != mainMaterialID {
			t.Fatalf("main input was not copied into variant: %v", variant)
		}
	}
	pr600APIJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-bom-versions/%d/publish", firstVersionID), nil, http.StatusOK)

	templateV2 := pr600APIJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-bom-spec-templates/%d/versions", templateID), map[string]any{"source_version_id": templateVersionID}, http.StatusOK)
	templateV2ID := pr600JSONInt64(t, templateV2, "id")
	templateV2Detail := pr600APIJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-bom-spec-templates/%d?version_id=%d", templateID, templateV2ID), nil, http.StatusOK)
	templateV2Variants := templateV2Detail["variants"].([]any)
	templateV2Default := templateV2Variants[0].(map[string]any)
	templateV2Default["name"] = "227g 袋装新版"
	templateV2DefaultItems := templateV2Default["items"].([]any)
	templateV2DefaultItems[0].(map[string]any)["qty_per_unit"] = 0.23
	pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/production-bom-spec-template-versions/%d/draft", templateV2ID), map[string]any{"variants": templateV2Variants}, http.StatusOK)
	pr600APIJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-bom-spec-template-versions/%d/publish", templateV2ID), nil, http.StatusOK)
	firstAfterTemplateChange := pr600APIJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-boms/%d?version_id=%d", firstBomID, firstVersionID), nil, http.StatusOK)
	frozenDefault := firstAfterTemplateChange["variants"].([]any)[0].(map[string]any)
	if frozenDefault["name"] != "227g 袋装" || frozenDefault["items"].([]any)[0].(map[string]any)["qty_per_unit"] != 0.227 {
		t.Fatalf("template v2 mutated copied BOM version: %v", frozenDefault)
	}

	newVersion := pr600APIJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-boms/%d/versions", firstBomID), map[string]any{"source_version_id": firstVersionID}, http.StatusOK)
	newVersionID := pr600JSONInt64(t, newVersion, "id")
	newVersionDetail := pr600APIJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-boms/%d?version_id=%d", firstBomID, newVersionID), nil, http.StatusOK)
	newVariants := newVersionDetail["variants"].([]any)
	for i := range newVariants {
		if got := pr600JSONInt64(t, newVariants[i].(map[string]any), "bom_spec_id"); got != firstSpecIDs[i] {
			t.Fatalf("same BOM version changed stable spec identity at %d: %d -> %d", i, firstSpecIDs[i], got)
		}
	}
	newVariants[0].(map[string]any)["barcode"] = "PR600-BAG-227"
	pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/production-bom-versions/%d/draft", newVersionID), map[string]any{"variants": newVariants}, http.StatusOK)
	barcodeDetail := pr600APIJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-boms/%d?version_id=%d", firstBomID, newVersionID), nil, http.StatusOK)
	barcodeVariants := barcodeDetail["variants"].([]any)
	if code := fmt.Sprint(barcodeVariants[0].(map[string]any)["code"]); !strings.HasPrefix(code, "BOM-SPEC-") {
		t.Fatalf("generated immutable specification code = %q", code)
	}
	if got := barcodeVariants[0].(map[string]any)["barcode"]; got != "PR600-BAG-227" {
		t.Fatalf("BOM specification barcode = %v", got)
	}
	newVariants[1].(map[string]any)["barcode"] = "PR600-BAG-227"
	duplicateBarcode := pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/production-bom-versions/%d/draft", newVersionID), map[string]any{"variants": newVariants}, http.StatusBadRequest)
	if !strings.Contains(strings.ToLower(fmt.Sprint(duplicateBarcode["error"])), "barcode") && !strings.Contains(strings.ToLower(fmt.Sprint(duplicateBarcode["error"])), "duplicate") {
		t.Fatalf("duplicate global barcode error = %v", duplicateBarcode)
	}
	newVariants[1].(map[string]any)["barcode"] = ""
	publishedUnit := newVariants[0].(map[string]any)["inventory_unit"]
	newVariants[0].(map[string]any)["inventory_unit"] = "个"
	unitChangeError := pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/production-bom-versions/%d/draft", newVersionID), map[string]any{"variants": newVariants}, http.StatusBadRequest)
	if !strings.Contains(fmt.Sprint(unitChangeError["error"]), "inventory_unit cannot be changed") {
		t.Fatalf("published specification unit change error = %v", unitChangeError)
	}
	newVariants[0].(map[string]any)["inventory_unit"] = publishedUnit
	unitFrozenDetail := pr600APIJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-boms/%d?version_id=%d", firstBomID, newVersionID), nil, http.StatusOK)
	if got := unitFrozenDetail["variants"].([]any)[0].(map[string]any)["inventory_unit"]; got != publishedUnit {
		t.Fatalf("failed unit change mutated stable specification: got=%v want=%v", got, publishedUnit)
	}

	secondBom := pr600APIJSON(t, e, http.MethodPost, "/api/production-boms", map[string]any{
		"name": "规格组 BOM B", "output_type": "product", "output_product_id": productID,
		"output_qty": 1, "spec_template_version_id": templateV2ID, "main_input_material_id": mainMaterialID,
	}, http.StatusOK)
	secondBomID := pr600JSONInt64(t, secondBom, "id")
	secondDetail := pr600APIJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-boms/%d", secondBomID), nil, http.StatusOK)
	secondVariants := secondDetail["variants"].([]any)
	for i := range secondVariants {
		if got := pr600JSONInt64(t, secondVariants[i].(map[string]any), "bom_spec_id"); got == firstSpecIDs[i] {
			t.Fatalf("different BOMs shared specification identity at %d: %d", i, got)
		}
	}
	secondVersionID := pr600JSONInt64(t, secondBom, "latest_version_id")
	pr600APIJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-bom-versions/%d/publish", secondVersionID), nil, http.StatusOK)

	firstDraftVariants := newVersionDetail["variants"].([]any)
	firstDefault := firstDraftVariants[0].(map[string]any)
	firstDefaultItems := firstDefault["items"].([]any)
	firstDefault["items"] = append(firstDefaultItems, map[string]any{
		"component_type": "product", "component_product_id": productID,
		"component_bom_spec_id": pr600JSONInt64(t, secondVariants[0].(map[string]any), "bom_spec_id"),
		"consume_unit":          "袋", "qty_per_unit": 1,
	})
	pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/production-bom-versions/%d/draft", newVersionID), map[string]any{"variants": firstDraftVariants}, http.StatusOK)
	pr600APIJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-bom-versions/%d/publish", newVersionID), nil, http.StatusOK)

	secondNewVersion := pr600APIJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-boms/%d/versions", secondBomID), map[string]any{"source_version_id": secondVersionID}, http.StatusOK)
	secondNewVersionID := pr600JSONInt64(t, secondNewVersion, "id")
	secondDraftDetail := pr600APIJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-boms/%d?version_id=%d", secondBomID, secondNewVersionID), nil, http.StatusOK)
	secondDraftVariants := secondDraftDetail["variants"].([]any)
	secondDefault := secondDraftVariants[0].(map[string]any)
	secondDefaultItems := secondDefault["items"].([]any)
	secondDefault["items"] = append(secondDefaultItems, map[string]any{
		"component_type": "product", "component_product_id": productID,
		"component_bom_spec_id": firstSpecIDs[0], "consume_unit": "袋", "qty_per_unit": 1,
	})
	pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/production-bom-versions/%d/draft", secondNewVersionID), map[string]any{"variants": secondDraftVariants}, http.StatusOK)
	cycleError := pr600APIJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-bom-versions/%d/publish", secondNewVersionID), nil, http.StatusBadRequest)
	if !strings.Contains(fmt.Sprint(cycleError["error"]), "cycle detected") {
		t.Fatalf("typed product_spec cycle error = %v", cycleError)
	}

	atomicTemplate := pr600APIJSON(t, e, http.MethodPost, "/api/production-bom-spec-templates", map[string]any{"name": "原子发布模板"}, http.StatusOK)
	atomicTemplateID := pr600JSONInt64(t, atomicTemplate, "id")
	atomicVersions := atomicTemplate["versions"].([]any)
	atomicVersionID := pr600JSONInt64(t, atomicVersions[0].(map[string]any), "id")
	pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/production-bom-spec-template-versions/%d/draft", atomicVersionID), map[string]any{"variants": variantsPayload}, http.StatusOK)
	invalidUnitVariants := make([]map[string]any, len(variantsPayload))
	for index, variant := range variantsPayload {
		copyVariant := make(map[string]any, len(variant))
		for key, value := range variant {
			copyVariant[key] = value
		}
		invalidUnitVariants[index] = copyVariant
	}
	invalidUnitVariants[9]["inventory_unit"] = "不存在单位"
	invalidUnitError := pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/production-bom-spec-template-versions/%d/draft", atomicVersionID), map[string]any{"variants": invalidUnitVariants}, http.StatusBadRequest)
	if !strings.Contains(fmt.Sprint(invalidUnitError["error"]), "not an active unit") {
		t.Fatalf("invalid unit error = %v", invalidUnitError)
	}
	atomicAfterFailure := pr600APIJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-bom-spec-templates/%d", atomicTemplateID), nil, http.StatusOK)
	if got := len(atomicAfterFailure["variants"].([]any)); got != 10 {
		t.Fatalf("failed group update partially replaced variants: %v", atomicAfterFailure)
	}
	noDefaultVariants := make([]map[string]any, len(variantsPayload))
	for i, variant := range variantsPayload {
		copyVariant := make(map[string]any, len(variant))
		for key, value := range variant {
			copyVariant[key] = value
		}
		copyVariant["is_default"] = false
		noDefaultVariants[i] = copyVariant
	}
	pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/production-bom-spec-template-versions/%d/draft", atomicVersionID), map[string]any{"variants": noDefaultVariants}, http.StatusOK)
	defaultError := pr600APIJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-bom-spec-template-versions/%d/publish", atomicVersionID), nil, http.StatusBadRequest)
	if !strings.Contains(fmt.Sprint(defaultError["error"]), "exactly one default") {
		t.Fatalf("default specification error = %v", defaultError)
	}
	var atomicStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.production_bom_spec_template_versions WHERE id=$1`, schema), atomicVersionID).Scan(&atomicStatus); err != nil || atomicStatus != "draft" {
		t.Fatalf("failed group publish status=%q err=%v", atomicStatus, err)
	}

	pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/products/%d/default-production-bom", productID), map[string]any{"default_production_bom_id": firstBomID}, http.StatusOK)
	var firstCurrentVariantID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_bom_version_variants WHERE version_id=$1 AND bom_spec_id=$2`, schema), newVersionID, firstSpecIDs[0]).Scan(&firstCurrentVariantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,cutover_at,cutover_by)
		VALUES($1,'cutover',now(),'tester')
		ON CONFLICT(product_id) DO UPDATE SET state='cutover',cutover_at=now(),cutover_by='tester'
	`, schema), productID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES($1,$2,$3,0,'pr600_default_switch',1,0)
		ON CONFLICT(product_id,bom_spec_id,spec_g,warehouse) DO UPDATE SET bom_variant_id=excluded.bom_variant_id,onhand_units=1
	`, schema), productID, firstSpecIDs[0], firstCurrentVariantID); err != nil {
		t.Fatal(err)
	}
	blockedSwitch := pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/products/%d/default-production-bom", productID), map[string]any{"default_production_bom_id": secondBomID}, http.StatusBadRequest)
	if !strings.Contains(fmt.Sprint(blockedSwitch["error"]), "default BOM switch blocked") {
		t.Fatalf("default BOM switch blocker = %v", blockedSwitch)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=0,onhand_loose_g=0 WHERE product_id=$1 AND bom_spec_id=$2`, schema), productID, firstSpecIDs[0]); err != nil {
		t.Fatal(err)
	}
	pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/products/%d/default-production-bom", productID), map[string]any{"default_production_bom_id": secondBomID}, http.StatusOK)
	var boundBOMID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_id FROM %s.production_bom_output_bindings WHERE output_type='product' AND output_id=$1`, schema), productID).Scan(&boundBOMID); err != nil || boundBOMID != secondBomID {
		t.Fatalf("default BOM after cleared switch = %d, want %d, err=%v", boundBOMID, secondBomID, err)
	}

	badVariants := append([]map[string]any(nil), variantsPayload...)
	badVariants[9] = map[string]any{"spec_key": "bag-10", "name": "规格袋 10", "inventory_unit": "不存在单位", "items": variantsPayload[9]["items"]}
	rejected := pr600APIJSON(t, e, http.MethodPut, fmt.Sprintf("/api/production-bom-spec-template-versions/%d/draft", templateVersionID), map[string]any{"variants": badVariants}, http.StatusBadRequest)
	if !strings.Contains(fmt.Sprint(rejected["error"]), "read-only") {
		t.Fatalf("published template mutation error = %v", rejected)
	}

	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type IN ('production_bom_spec_template','production_bom_spec_template_version','production_bom')`, schema)).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount < 5 {
		t.Fatalf("audit count = %d, want at least 5", auditCount)
	}
}

func pr600APIJSON(t *testing.T, e *echo.Echo, method, path string, body any, wantStatus int) map[string]any {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, rec.Code, wantStatus, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s %s: %v body=%s", method, path, err, rec.Body.String())
	}
	return out
}

func pr600APIJSONArray(t *testing.T, e *echo.Echo, method, path string, wantStatus int) []any {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, rec.Code, wantStatus, rec.Body.String())
	}
	var out []any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s %s: %v body=%s", method, path, err, rec.Body.String())
	}
	return out
}

func pr600JSONInt64(t *testing.T, row map[string]any, key string) int64 {
	t.Helper()
	value, ok := row[key].(float64)
	if !ok || value <= 0 {
		t.Fatalf("%s is not a positive id in %v", key, row)
	}
	return int64(value)
}
