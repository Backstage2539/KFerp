package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestOrderAPIGreenBOMSpecPriceOverridesLegacyProductKind(t *testing.T) {
	for _, tc := range []struct {
		name       string
		orderType  int
		listType   string
		qty        string
		wantStatus int
	}{
		{"wholesale 3kg", 1, "green", "3", http.StatusOK},
		{"retail 3kg selected green table", 2, "green", "3", http.StatusOK},
		{"mismatched publication type", 1, "commercial", "3", http.StatusBadRequest},
		{"outside published tier", 1, "green", "60", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := newOrderAPITestDB(t)
			ctx := context.Background()
			seedOrderAPITestData(t, ctx, pool, schema)
			seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
			// Older product masters retain roasted even though the selected,
			// published BOM-spec price belongs to the green-bean price list.
			mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
				CREATE TABLE %[1]s.customer_order_production_demands(order_id BIGINT);
				ALTER TABLE %[1]s.order_items ADD COLUMN customer_product_reference_id BIGINT, ADD COLUMN material_source_mode TEXT;
				UPDATE %[1]s.products SET name='测试生豆',product_kind='roasted' WHERE id=7;
				UPDATE %[1]s.production_bom_specs SET name='1KG',inventory_unit='kg' WHERE id=9001;
				UPDATE %[1]s.production_bom_version_variants SET spec_name_snapshot='1KG',inventory_unit='kg' WHERE id=9101;
				INSERT INTO %[1]s.bean_list_publications(
					id,list_type,publication_purpose,version_no,status,owner_type,owner_key,
					config_json,content_json,changelog,actor,published_at
				) VALUES (9301,'green','factory_supply','GREEN-SPEC-V1','published','official','','{}'::jsonb,
					'{"price_rows":[{"product_id":7,"parent_product_id":7,"bom_spec_id":9001,"bom_variant_id":9101,"tier_label":"1-59kg","min_qty":1,"max_qty":59,"final_unit_price":98,"price_unit":"kg","inventory_unit":"kg","quantity_basis":"sales_spec_count"}]}'::jsonb,
					'测试生豆规格发布价','测试员','2026-09-09 09:00:00+08');
			`, schema))
			payload := map[string]any{
				"order_date": "2026-09-09", "customer_id": 3, "source_id": 1,
				"order_type_id": tc.orderType, "pay_status_id": 1, "ship_status_id": 1,
				"green_bean_list_publication_id": 9301,
				"item_bean_list_publication_id":  []string{"9301"},
				"product_id":                     []string{"7"}, "parent_product_id": []string{"7"},
				"bom_spec_id": []string{"9001"}, "bom_variant_id": []string{"9101"},
				"product_kind": []string{"green_bean"}, "item_name": []string{"测试生豆"},
				"qty": []string{tc.qty}, "unit": []string{"kg"},
				"price_source_json": []string{fmt.Sprintf(`{"list_type":%q,"publication_id":9301}`, tc.listType)},
			}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			newOrderAPITestEcho(pool, schema).ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("POST /api/order status=%d body=%s, want %d", rec.Code, rec.Body.String(), tc.wantStatus)
			}
			if tc.wantStatus != http.StatusOK {
				var count int
				if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s.orders", schema)).Scan(&count); err != nil || count != 0 {
					t.Fatalf("rejected order count=%d err=%v, want no order", count, err)
				}
				return
			}
			var productKind, unit, spec, listType, version string
			var publicationID, bomSpecID, bomVariantID int64
			var qty, price, total float64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`
				SELECT product_kind,unit,spec,qty::float8,unit_price::float8,line_total::float8,
				       bean_list_publication_id,bean_list_version_no,bom_spec_id,bom_variant_id,
				       price_source_json->>'list_type'
				FROM %s.order_items ORDER BY id DESC LIMIT 1
			`, schema)).Scan(&productKind, &unit, &spec, &qty, &price, &total, &publicationID, &version, &bomSpecID, &bomVariantID, &listType); err != nil {
				t.Fatal(err)
			}
			if productKind != "green_bean" || unit != "kg" || spec != "1KG" || qty != 3 || price != 98 || total != 294 || publicationID != 9301 || version != "GREEN-SPEC-V1" || bomSpecID != 9001 || bomVariantID != 9101 || listType != "green" {
				t.Fatalf("saved kind=%s unit/spec=%s/%s qty/price/total=%v/%v/%v publication=%d/%s bom=%d/%d list=%s", productKind, unit, spec, qty, price, total, publicationID, version, bomSpecID, bomVariantID, listType)
			}
			var catalogKind string
			if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT product_kind FROM %s.products WHERE id=7", schema)).Scan(&catalogKind); err != nil || catalogKind != "roasted" {
				t.Fatalf("product master was changed: kind=%s err=%v", catalogKind, err)
			}
		})
	}
}
