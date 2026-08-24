package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"orderapp/internal/infrastructure/postgres/orderbeans"
	postgresmigration "orderapp/internal/infrastructure/postgres/productspecmigration"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestOrderAPICutoverProductUsesBOMSpecBusinessIdentity(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	formReq := httptest.NewRequest(http.MethodGet, "/api/order/form", nil)
	formRec := httptest.NewRecorder()
	e.ServeHTTP(formRec, formReq)
	if formRec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status=%d body=%s", formRec.Code, formRec.Body.String())
	}
	for _, want := range []string{
		`"product_bom_spec_options"`,
		`"parent_product_id":7`,
		`"legacy_child_product_id":701`,
		`"bom_spec_id":9001`,
		`"bom_variant_id":9101`,
		`"spec_code":"BOM-SPEC-009001"`,
		`"barcode":"NEW-BOM-BAR-9001"`,
		`"spec_key":"227g-bag"`,
		`"inventory_unit":"袋"`,
		`"write_product_id":7`,
		`"bom_spec_id":9002`,
		`"spec_key":"454g-bag"`,
	} {
		if !strings.Contains(formRec.Body.String(), want) {
			t.Fatalf("order form missing %s: %s", want, formRec.Body.String())
		}
	}
	if strings.Contains(formRec.Body.String(), "LEGACY-BAR-701") {
		t.Fatalf("cutover order options must not expose the legacy child SKU barcode: %s", formRec.Body.String())
	}

	payload := map[string]any{
		"order_date":        "2026-08-17",
		"customer_id":       3,
		"pay_status_id":     1,
		"ship_status_id":    1,
		"product_id":        []string{"7"},
		"parent_product_id": []string{"7"},
		"bom_spec_id":       []string{"9001"},
		"bom_variant_id":    []string{"9101"},
		"item_name":         []string{"橘皮乌龙 227g 袋装"},
		"tier_id":           []string{"manual"},
		"unit_price":        []string{"88"},
		"qty":               []string{"2"},
		// Both legacy fields are intentionally wrong. The server must freeze the
		// published BOM spec name/unit and must not derive identity through grams.
		"unit": []string{"kg"},
		"spec": []string{"999"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status=%d body=%s", rec.Code, rec.Body.String())
	}
	var saved struct {
		OrderID int64 `json:"order_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}

	var productID, bomSpecID, bomVariantID int64
	var unit, spec, priceSource string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT product_id,bom_spec_id,bom_variant_id,COALESCE(unit,''),COALESCE(spec,''),price_source_json::text
		FROM %s.order_items WHERE order_id=$1
	`, schema), saved.OrderID).Scan(&productID, &bomSpecID, &bomVariantID, &unit, &spec, &priceSource); err != nil {
		t.Fatal(err)
	}
	if productID != 7 || bomSpecID != 9001 || bomVariantID != 9101 {
		t.Fatalf("saved identity product/spec/variant=%d/%d/%d, want 7/9001/9101", productID, bomSpecID, bomVariantID)
	}
	if unit != "袋" || spec != "227g 袋装" {
		t.Fatalf("saved direct BOM spec unit/name=%q/%q, want 袋/227g 袋装", unit, spec)
	}
	for _, want := range []string{`"product_id": 7`, `"bom_spec_id": 9001`, `"bom_variant_id": 9101`, `"quantity_basis": "sales_spec_count"`} {
		if !strings.Contains(priceSource, want) {
			t.Fatalf("price source missing %s: %s", want, priceSource)
		}
	}
	if strings.Contains(priceSource, `"spec_g"`) {
		t.Fatalf("canonical price source must not reintroduce spec_g conversion: %s", priceSource)
	}

	editReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/order/form?edit_id=%d", saved.OrderID), nil)
	editRec := httptest.NewRecorder()
	e.ServeHTTP(editRec, editReq)
	if editRec.Code != http.StatusOK {
		t.Fatalf("GET edit form status=%d body=%s", editRec.Code, editRec.Body.String())
	}
	for _, want := range []string{`"bom_spec_id":9001`, `"bom_variant_id":9101`, `"bom_spec_key":"227g-bag"`, `"bom_spec_name":"227g 袋装"`, `"unit":"袋"`} {
		if !strings.Contains(editRec.Body.String(), want) {
			t.Fatalf("edit form missing %s: %s", want, editRec.Body.String())
		}
	}
}

func TestOrderAPICutoverProductRejectsLegacyChildAndMissingBOMSpec(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
	e := newOrderAPITestEcho(pool, schema)

	for name, identity := range map[string]map[string]any{
		"legacy child": {
			"product_id": []string{"701"},
		},
		"parent without spec": {
			"product_id":        []string{"7"},
			"parent_product_id": []string{"7"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			payload := map[string]any{
				"order_date":     "2026-08-17",
				"customer_id":    3,
				"pay_status_id":  1,
				"ship_status_id": 1,
				"item_name":      []string{"旧规格写入"},
				"tier_id":        []string{"manual"},
				"unit_price":     []string{"88"},
				"qty":            []string{"1"},
				"unit":           []string{"袋"},
				"spec":           []string{"227"},
			}
			for key, value := range identity {
				payload[key] = value
			}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("POST /api/order status=%d body=%s, want 400", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "BOM") && !strings.Contains(rec.Body.String(), "legacy") {
				t.Fatalf("rejection should explain BOM spec/legacy identity: %s", rec.Body.String())
			}
		})
	}
}

func TestOrderAPICutoverProductResolvesLegacyPublishedPriceIntoCurrentBOMSpecIdentity(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.customers SET default_order_type_id=1 WHERE id=3;
		INSERT INTO %[1]s.bean_list_publications(
			id,list_type,publication_purpose,version_no,status,owner_type,owner_key,
			config_json,content_json,changelog,actor,published_at
		) VALUES (9201,'commercial','factory_supply','SPEC-PRICE-V1','published','official','','{}'::jsonb,
			'{"price_rows":[{"product_id":701,"sku_id":701,"parent_product_id":7,"tier_label":"1袋+","min_qty":1,"final_unit_price":75,"price_unit":"袋","quantity_basis":"sales_spec_count","tier_quantity_unit":"227g 袋装","effective_sales_spec":{"sku_id":701,"spec_key":"227g-bag","spec_name":"227g 袋装","spec_label":"227g 袋装","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}}]}'::jsonb,
			'旧子 SKU 发布价兼容','测试员','2026-08-17 09:00:00+08');
	`, schema))
	legacyPrice, err := orderbeans.ResolvePublishedPricingForPublicationWithUnit(ctx, pool, schema, 3, 701, orderbeans.ListTypeCommercial, 9201, 227, 2, "袋", 0)
	if err != nil || legacyPrice.UnitPrice != 75 {
		t.Fatalf("legacy published price compatibility setup price=%+v err=%v", legacyPrice, err)
	}

	e := newOrderAPITestEcho(pool, schema)
	formReq := httptest.NewRequest(http.MethodGet, "/api/order/form", nil)
	formRec := httptest.NewRecorder()
	e.ServeHTTP(formRec, formReq)
	if formRec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status=%d body=%s", formRec.Code, formRec.Body.String())
	}
	for _, want := range []string{`"unit_price":75`, `"bom_spec_id":9001`, `"bom_variant_id":9101`, `legacy_pricing_product_id\":701`} {
		if !strings.Contains(formRec.Body.String(), want) {
			t.Fatalf("current spec pricing option missing %s: %s", want, formRec.Body.String())
		}
	}

	payload := map[string]any{
		"order_date":                          "2026-08-17",
		"customer_id":                         3,
		"source_id":                           1,
		"order_type_id":                       1,
		"pay_status_id":                       1,
		"ship_status_id":                      1,
		"commercial_bean_list_publication_id": 9201,
		"product_id":                          []string{"7"},
		"parent_product_id":                   []string{"7"},
		"bom_spec_id":                         []string{"9001"},
		"bom_variant_id":                      []string{"9101"},
		"item_name":                           []string{"橘皮乌龙 227g 袋装"},
		"qty":                                 []string{"2"},
		"unit":                                []string{"袋"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status=%d body=%s", rec.Code, rec.Body.String())
	}
	var productID, bomSpecID, bomVariantID int64
	var unitPrice, lineTotal float64
	var priceSource string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT product_id,bom_spec_id,bom_variant_id,unit_price::float8,line_total::float8,price_source_json::text
		FROM %s.order_items ORDER BY id DESC LIMIT 1
	`, schema)).Scan(&productID, &bomSpecID, &bomVariantID, &unitPrice, &lineTotal, &priceSource); err != nil {
		t.Fatal(err)
	}
	if productID != 7 || bomSpecID != 9001 || bomVariantID != 9101 || unitPrice != 75 || lineTotal != 150 {
		t.Fatalf("saved current price identity=%d/%d/%d price=%.2f total=%.2f", productID, bomSpecID, bomVariantID, unitPrice, lineTotal)
	}
	for _, want := range []string{`"product_id": 7`, `"bom_spec_id": 9001`, `"bom_variant_id": 9101`, `"legacy_pricing_product_id": 701`, `"publication_id": 9201`} {
		if !strings.Contains(priceSource, want) {
			t.Fatalf("canonical published price source missing %s: %s", want, priceSource)
		}
	}
}

func TestOrderAPICutoverProductUsesCanonicalPublishedPriceAcrossBOMVersionsWithoutLegacyMapping(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		DELETE FROM %[1]s.legacy_child_sku_bom_spec_mappings WHERE parent_product_id=7;
		UPDATE %[1]s.customers SET default_order_type_id=1 WHERE id=3;
		INSERT INTO %[1]s.bean_list_publications(
			id,list_type,publication_purpose,version_no,status,owner_type,owner_key,
			config_json,content_json,changelog,actor,published_at
		) VALUES (9301,'commercial','factory_supply','SPEC-PRICE-V1','published','official','','{}'::jsonb,
			'{"price_rows":[{"product_id":7,"parent_product_id":7,"bom_spec_id":9001,"bom_variant_id":9101,"tier_label":"1袋+","min_qty":1,"final_unit_price":79,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count"}]}'::jsonb,
			'新BOM规格发布价','测试员','2026-08-17 09:00:00+08');
		UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=8101;
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status) VALUES (8102,8001,'V002','published');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order
		) VALUES (9201,8102,9001,'227g 袋装','袋',true,1),
		         (9202,8102,9002,'454g 袋装','袋',false,2);
		UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=8102 WHERE output_type='product' AND output_id=7 AND is_default=true;
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	formReq := httptest.NewRequest(http.MethodGet, "/api/order/form", nil)
	formRec := httptest.NewRecorder()
	e.ServeHTTP(formRec, formReq)
	if formRec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status=%d body=%s", formRec.Code, formRec.Body.String())
	}
	for _, want := range []string{`"bom_spec_id":9001`, `"bom_variant_id":9201`, `"unit_price":79`} {
		if !strings.Contains(formRec.Body.String(), want) {
			t.Fatalf("canonical current spec pricing option missing %s: %s", want, formRec.Body.String())
		}
	}

	payload := map[string]any{
		"order_date":                          "2026-08-17",
		"customer_id":                         3,
		"source_id":                           1,
		"order_type_id":                       1,
		"pay_status_id":                       1,
		"ship_status_id":                      1,
		"commercial_bean_list_publication_id": 9301,
		"product_id":                          []string{"7"},
		"parent_product_id":                   []string{"7"},
		"bom_spec_id":                         []string{"9001"},
		"item_name":                           []string{"橘皮乌龙 227g 袋装"},
		"qty":                                 []string{"2"},
		"unit":                                []string{"袋"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status=%d body=%s", rec.Code, rec.Body.String())
	}
	var productID, bomSpecID, bomVariantID int64
	var unitPrice, lineTotal float64
	var priceSource string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT product_id,bom_spec_id,bom_variant_id,unit_price::float8,line_total::float8,price_source_json::text
		FROM %s.order_items ORDER BY id DESC LIMIT 1
	`, schema)).Scan(&productID, &bomSpecID, &bomVariantID, &unitPrice, &lineTotal, &priceSource); err != nil {
		t.Fatal(err)
	}
	if productID != 7 || bomSpecID != 9001 || bomVariantID != 9201 || unitPrice != 79 || lineTotal != 158 {
		t.Fatalf("saved canonical current identity=%d/%d/%d price=%.2f total=%.2f", productID, bomSpecID, bomVariantID, unitPrice, lineTotal)
	}
	for _, want := range []string{`"product_id": 7`, `"bom_spec_id": 9001`, `"bom_variant_id": 9201`, `"publication_id": 9301`} {
		if !strings.Contains(priceSource, want) {
			t.Fatalf("canonical published price source missing %s: %s", want, priceSource)
		}
	}
	payload["bom_variant_id"] = []string{"9101"}
	body, _ = json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "BOM spec") {
		t.Fatalf("stale explicit variant status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func seedOrderAPIBOMSpecIdentity(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.production_bom_versions (
			id BIGINT PRIMARY KEY,
			bom_id BIGINT NOT NULL,
			version_no TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft'
		);
		CREATE TABLE %[1]s.production_bom_output_bindings (
			bom_id BIGINT NOT NULL,
			bom_version_id BIGINT NOT NULL,
			output_type TEXT NOT NULL,
			output_id BIGINT NOT NULL,
			is_default BOOLEAN NOT NULL DEFAULT false
		);
		CREATE TABLE %[1]s.production_bom_specs (
			id BIGINT PRIMARY KEY,
			bom_id BIGINT NOT NULL,
			code TEXT NOT NULL,
			barcode TEXT NOT NULL DEFAULT '',
			spec_key TEXT NOT NULL,
			name TEXT NOT NULL,
			inventory_unit TEXT NOT NULL
		);
		CREATE TABLE %[1]s.production_bom_version_variants (
			id BIGINT PRIMARY KEY,
			version_id BIGINT NOT NULL,
			bom_spec_id BIGINT NOT NULL,
			spec_name_snapshot TEXT NOT NULL,
			inventory_unit TEXT NOT NULL,
			is_default BOOLEAN NOT NULL DEFAULT false,
			sort_order INTEGER NOT NULL DEFAULT 0
		);
		INSERT INTO %[1]s.products(
			id,name,active,parent_product_id,auto_derived_sku,derived_spec_status,
			sku_name,sku_code,spec_label,net_content_qty,net_content_unit,derived_sales_unit
		) VALUES (701,'橘皮乌龙 227g 袋装',false,7,true,'bom_spec_cutover',
			'227g 袋装','SKU-701','227g 袋装',227,'g','袋');
		UPDATE %[1]s.products SET barcode='LEGACY-BAR-701' WHERE id=701;
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status) VALUES (8101,8001,'V001','published');
		INSERT INTO %[1]s.production_bom_output_bindings(bom_id,bom_version_id,output_type,output_id,is_default)
		VALUES (8001,8101,'product',7,true);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,barcode,spec_key,name,inventory_unit)
		VALUES (9001,8001,'BOM-SPEC-009001','NEW-BOM-BAR-9001','227g-bag','227g 袋装','袋'),
		       (9002,8001,'BOM-SPEC-009002','','454g-bag','454g 袋装','袋');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order
		) VALUES (9101,8101,9001,'227g 袋装','袋',true,1),
		         (9102,8101,9002,'454g 袋装','袋',false,2);
	`, schema))
	if err := postgresmigration.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("productspecmigration.EnsureSchema: %v", err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.product_bom_spec_migrations(product_id,state,cutover_at,cutover_by)
		VALUES (7,'cutover',now(),'测试员');
		INSERT INTO %[1]s.legacy_child_sku_bom_spec_mappings(
			parent_product_id,legacy_child_product_id,bom_id,bom_spec_id,bom_variant_id,
			legacy_spec_key,legacy_spec_name,legacy_sales_unit,legacy_spec_g,metadata_snapshot,
			created_by,updated_by,tombstoned_at
		) VALUES (7,701,8001,9001,9101,'227g-bag','227g 袋装','袋',227,
			'{"spec_key":"227g-bag"}'::jsonb,'测试员','测试员',now());
	`, schema))
}
