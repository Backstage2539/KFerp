package customerportal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	customerportalapp "orderapp/internal/application/customerportal"
	manufacturingpg "orderapp/internal/infrastructure/postgres/manufacturing"
	productspecpg "orderapp/internal/infrastructure/postgres/productspecmigration"
	saleshttp "orderapp/internal/interfaces/http/sales"
)

type mallTestScope struct{ customerID int64 }

func (s mallTestScope) BoundCustomerID(context.Context, int64) (int64, error) {
	return s.customerID, nil
}
func (s mallTestScope) CustomerWorkspace(_ context.Context, _ int64, id int64, _ string, _ int64) (map[string]any, error) {
	if id != s.customerID {
		return nil, fmt.Errorf("客户不匹配")
	}
	return map[string]any{"customer_id": s.customerID}, nil
}
func TestMallHTTPPersistsCanonicalIdentityAndOperationNotes(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	f := seedPortalProcessingBOMSpecFixture(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.order_resolution_stock_returns(id bigint,request_id bigint,step_id bigint,order_id bigint,order_item_id bigint,source_kind text,source_id bigint,batch_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,source_warehouse text,target_warehouse text,source_owner_customer_id bigint,target_owner_customer_id bigint,quantity bigint,unit_cost numeric,quality text,target_batch_id bigint,target_batch_code text,status text)`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := productspecpg.EnsureAuthorityView(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET customer_id=0,visibility='public' WHERE id=$1`, schema), f.ProductID); err != nil {
		t.Fatal(err)
	}
	if err := manufacturingpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	var routeID, operationID, stepID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.process_routes(name,status) VALUES('包装路线','active') RETURNING id`, schema)).Scan(&routeID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.manufacturing_operations(name) VALUES('包装') RETURNING id`, schema)).Scan(&operationID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.process_route_operations(route_id,seq,operation,operation_id) VALUES($1,1,'包装',$2) RETURNING id`, schema), routeID, operationID).Scan(&stepID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET process_route_id=$2 WHERE id=$1`, schema), f.BomVariantID, routeID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	var mallID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.mall_products(product_id,bom_spec_id,bom_variant_id,spec_name,inventory_unit,title,unit_price,status) VALUES($1,$2,$3,'227g袋','袋','袋装',68,'published') RETURNING id`, schema), f.ProductID, f.BomSpecID, f.BomVariantID).Scan(&mallID); err != nil {
		t.Fatal(err)
	}
	var err error
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("portal_internal_allowed", true)
			c.Set("actor", "测试员")
			return next(c)
		}
	})
	saleshttp.RegisterRoutes(e, saleshttp.Dependencies{CustomerScope: mallTestScope{f.CustomerID}, CustomerMall: customerportalapp.NewService(repo, nil)})
	payload := map[string]any{"recipient_name": "客户", "recipient_phone": "13800138000", "recipient_address": "上海测试路", "items": []any{map[string]any{"mall_product_id": mallID, "product_id": f.ProductID, "bom_spec_id": f.BomSpecID, "bom_variant_id": f.BomVariantID, "qty": 2, "sales_unit": "袋", "operation_notes": []any{map[string]any{"output_type": "product", "output_id": f.ProductID, "bom_spec_id": f.BomSpecID, "bom_variant_id": f.BomVariantID, "route_id": routeID, "step_id": stepID, "operation_id": operationID, "operation_name": "包装", "note": "贴标签"}}}}}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/customer-processing/portal/mall/orders?customer_id=%d", f.CustomerID), bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("order HTTP %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		OrderID int64 `json:"order_id"`
	}
	if err = json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	var product, spec, variant int64
	var raw string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT product_id,bom_spec_id,bom_variant_id,operation_notes_json::text FROM %s.order_items WHERE order_id=$1`, schema), got.OrderID).Scan(&product, &spec, &variant, &raw); err != nil {
		t.Fatal(err)
	}
	if product != f.ProductID || spec != f.BomSpecID || variant != f.BomVariantID || !strings.Contains(raw, "贴标签") {
		t.Fatalf("wrong saved identity %d/%d/%d notes=%s", product, spec, variant, raw)
	}
	item := payload["items"].([]any)[0].(map[string]any)
	item["bom_variant_id"] = f.BomVariantID + 100000
	body, _ = json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/customer-processing/portal/mall/orders?customer_id=%d", f.CustomerID), bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("stale variant accepted: %d %s", rec.Code, rec.Body.String())
	}
	var count int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.orders WHERE id<>$1`, schema), got.OrderID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed request left an order: %d %v", count, err)
	}
}
