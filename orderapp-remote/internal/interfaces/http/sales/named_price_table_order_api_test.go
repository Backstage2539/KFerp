package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	"orderapp/internal/infrastructure/postgres/orderbeans"
	"strings"
	"testing"
)

func TestNamedPriceTableOrderAPIUsesSelectedSnapshotAndBlocksBypass(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %[1]s.orders ADD COLUMN prepayment_amount NUMERIC(12,2) NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.order_items ADD COLUMN customer_product_reference_id BIGINT, ADD COLUMN material_source_mode TEXT;
 CREATE TABLE %[1]s.customer_order_production_demands(order_id BIGINT);
 UPDATE %[1]s.customers SET default_order_type_id=1 WHERE id=3;`, schema))
	for _, table := range []struct {
		id, customer int
		name         string
		price        int
	}{{88, 3, "227g常规表", 30}, {89, 3, "227g会员表", 90}, {90, 4, "其他客户", 70}} {
		content := fmt.Sprintf(`{"price_rows":[{"product_id":7,"bom_spec_id":9001,"bom_variant_id":9101,"min_qty":1,"final_unit_price":%d,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count"}]}`, table.price)
		config := fmt.Sprintf(`{"publication_batch":{"release_id":"batch-%d","table_key":"table-%d","table_name":"%s","is_default_table":%t}}`, table.customer, table.id, table.name, table.id == 88)
		_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publications(id,list_type,version_no,status,owner_type,owner_key,config_json,content_json) VALUES($1,'commercial','V3.0.6','published','customer',$2,$3::jsonb,$4::jsonb)`, schema), table.id, fmt.Sprint(table.customer), config, content)
		if err != nil {
			t.Fatal(err)
		}
	}
	price, priceErr := orderbeans.ResolvePublishedPricingForPublicationWithBOMSpec(ctx, pool, schema, 3, 7, "commercial", 88, 9001, 9101, 2)
	if priceErr != nil || price.UnitPrice != 30 {
		t.Fatalf("fixture price=%+v err=%v", price, priceErr)
	}
	e := newOrderAPITestEcho(pool, schema)
	for _, tc := range []struct {
		selected, item, spec int
		manual               string
		status               int
		price                float64
		name                 string
	}{
		{88, 88, 9001, "", 200, 30, "227g常规表"}, {89, 89, 9001, "", 200, 90, "227g会员表"},
		{88, 89, 9001, "", 400, 0, ""}, {90, 90, 9001, "", 400, 0, ""}, {89, 89, 9002, "1", 400, 0, ""},
	} {
		tier := ""
		if tc.manual != "" {
			tier = "manual"
		}
		variant := 9101
		if tc.spec == 9002 {
			variant = 9102
		}
		payload := fmt.Sprintf(`{"order_date":"2026-09-07","customer_id":3,"order_type_id":1,"source_id":1,"pay_status_id":1,"ship_status_id":1,"selected_price_table_ids":[%d],"bean_list_publication_id":%d,"item_bean_list_publication_id":["%d"],"product_id":["7"],"bom_spec_id":["%d"],"bom_variant_id":["%d"],"item_name":["测试豆"],"tier_id":["%s"],"unit_price":["%s"],"qty":["2"],"unit":["袋"],"spec":["227"]}`, tc.selected, tc.item, tc.item, tc.spec, variant, tier, tc.manual)
		req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(payload))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("selected=%d item=%d spec=%d: %d %s", tc.selected, tc.item, tc.spec, rec.Code, rec.Body.String())
		}
		if tc.status != 200 {
			continue
		}
		var result struct {
			OrderID int64 `json:"order_id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		var id int64
		var price float64
		var version, source string
		err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bean_list_publication_id,bean_list_version_no,unit_price::float8,price_source_json::text FROM %s.order_items WHERE order_id=$1`, schema), result.OrderID).Scan(&id, &version, &price, &source)
		if err != nil || id != int64(tc.item) || version != "V3.0.6" || price != tc.price || !strings.Contains(source, tc.name) {
			t.Fatalf("frozen id=%d version=%s price=%v source=%s err=%v", id, version, price, source, err)
		}
	}
}
