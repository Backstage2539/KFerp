package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	authzapp "orderapp/internal/application/authz"
	messagecenterapp "orderapp/internal/application/messagecenter"
	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	support "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
)

func TestOrderEntryRedirectsToVueShell(t *testing.T) {
	e := echo.New()
	registerOrderRoutes(e, nil)

	req := httptest.NewRequest(http.MethodGet, "/order?edit_id=9", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("GET /order status = %d, want %d", rec.Code, http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != "vue-shell?view=order&edit_id=9" {
		t.Fatalf("GET /order Location = %q, want Vue order shell with edit_id", got)
	}
}

func TestOrderAPIRoutesExposeIrreversibleVoidJSONEndpoints(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "sales", "order_api.go"))
	if err != nil {
		t.Fatalf("read order_api.go: %v", err)
	}
	source := string(body)
	for _, want := range []string{
		`e.POST("/api/orders/:id/void", h.void)`,
		`e.POST("/api/orders/void", h.voidMany)`,
		"func (h orderAPIHandler) void",
		"func (h orderAPIHandler) voidMany",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("order API missing JSON void endpoint wiring %q", want)
		}
	}
	for _, forbidden := range []string{
		"/api/orders/:id/unvoid",
		"func (h orderAPIHandler) unvoid",
		"sales.Unvoid",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("order API must not expose restore marker %q", forbidden)
		}
	}
}

func TestOrderAPIFormReturnsRetailSpecs(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"products"`, `"retail_specs":[227,250]`, `"retail_price_227g":50`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/order/form missing %s: %s", needle, body)
		}
	}
}

func TestOrderAPIFormReturnsCustomerDefaultsForOrderEntry(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"default_source_id":1`, `"default_order_type_id":2`, `"py"`, `"pyi"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/order/form missing %s: %s", needle, body)
		}
	}
}

func TestOrderAPIVoidIsIrreversibleAndBulkVoidUsesSoftDeleteAndListFilters(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES
			(7001, 'VOID-API-NORMAL', '2026-05-18', 3, 1, 2, 1, 1, 100, false),
			(7002, 'VOID-API-TARGET', '2026-05-18', 3, 1, 2, 1, 1, 200, false),
			(7003, 'VOID-API-BULK', '2026-05-18', 3, 1, 2, 1, 1, 300, false);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodPost, "/api/orders/7002/void", strings.NewReader(`{"reason":"客户取消"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/:id/void status=%d body=%s, want 200", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"is_void":true`) || !strings.Contains(body, `"order_id":7002`) {
		t.Fatalf("void response missing state: %s", body)
	}

	var isVoid bool
	var reason string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT is_void, COALESCE(void_reason,'') FROM %s.orders WHERE id=7002`, schema)).Scan(&isVoid, &reason); err != nil {
		t.Fatalf("query voided order: %v", err)
	}
	if !isVoid || reason != "客户取消" {
		t.Fatalf("voided order state=%v reason=%q, want true/客户取消", isVoid, reason)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/orders?q=VOID-API&limit=20", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders default status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "VOID-API-NORMAL") || strings.Contains(body, "VOID-API-TARGET") {
		t.Fatalf("default order list should hide voided order, body=%s", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/orders?q=VOID-API&void=void&limit=20", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders void status=%d body=%s", rec.Code, rec.Body.String())
	}
	body = rec.Body.String()
	if !strings.Contains(body, "VOID-API-TARGET") || strings.Contains(body, "VOID-API-NORMAL") {
		t.Fatalf("void order list should include only voided order, body=%s", body)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/orders/7002/unvoid", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST /api/orders/:id/unvoid status=%d body=%s, want 404", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/orders/void", strings.NewReader(`{"order_ids":[7001,7003],"reason":"批量失效"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/void status=%d body=%s, want 200", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"voided":2`) || !strings.Contains(body, `"order_ids":[7001,7003]`) {
		t.Fatalf("bulk void response missing count and ids: %s", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/orders?q=VOID-API&limit=20", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders after bulk void status=%d body=%s", rec.Code, rec.Body.String())
	}
	body = rec.Body.String()
	if strings.Contains(body, "VOID-API-NORMAL") || strings.Contains(body, "VOID-API-TARGET") || strings.Contains(body, "VOID-API-BULK") {
		t.Fatalf("default order list should hide all voided orders, body=%s", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/orders?q=VOID-API&void=void&limit=20", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders void after bulk status=%d body=%s", rec.Code, rec.Body.String())
	}
	body = rec.Body.String()
	for _, want := range []string{"VOID-API-NORMAL", "VOID-API-TARGET", "VOID-API-BULK"} {
		if !strings.Contains(body, want) {
			t.Fatalf("void order list missing %s after bulk void, body=%s", want, body)
		}
	}
}

func TestOrderAPIFormReturnsResponsiblePersonOptions(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIResponsibleData(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{
		`"employees"`,
		`"name":"销售小王"`,
		`"department":"销售"`,
		`"contact":"测试收件人"`,
		`"phone":"13800000000"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/order/form missing responsible option %s: %s", needle, body)
		}
	}
}

func TestOrderAPIFormFiltersCustomerSpecificProducts(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.customers(id,name,active) VALUES (4,'其他客户',true);
		INSERT INTO %s.products(id,name,default_price,active,retail_price_227g,customer_id,base_product_id,visibility,custom_type)
		VALUES
			(8,'测试客户专属深烘',58,true,58,3,7,'customer_only','custom_roast'),
			(9,'其他客户专属深烘',59,true,59,4,7,'customer_only','custom_roast');
	`, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form?customer_id=3", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"name":"橘皮乌龙"`, `"name":"测试客户专属深烘"`, `"customer_id":3`, `"base_product_id":7`, `"visibility":"customer_only"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/order/form missing %s: %s", needle, body)
		}
	}
	if strings.Contains(body, "其他客户专属深烘") {
		t.Fatalf("GET /api/order/form leaked another customer's product: %s", body)
	}
}

func TestOrderAPISmallBatchDirectShipCustomerUsesDefaultPriceTier(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.product_price_tiers(id, product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, min_qty_lb, max_qty_lb, price_per_lb, active)
		VALUES
			(101,7,454,1,14,120,1,14,120,true),
			(102,7,454,15,28,90,15,28,90,true),
			(103,7,454,29,NULL,80,29,NULL,80,true);
		INSERT INTO %[1]s.customer_service_capabilities(customer_id, capability_code, enabled, config_json)
		VALUES(3,'direct_ship',true,'{"small_batch_price_rule":{"enabled":true,"threshold_lb":14,"tier_min_lb":15,"tier_max_lb":28}}'::jsonb);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-05-06",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  2,
		"payment_method": "微信支付",
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{""},
		"unit_price":     []string{""},
		"item_name":      []string{"客户A自定义货品名"},
		"qty":            []string{"10"},
		"unit":           []string{"件"},
		"spec":           []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var tierID int64
	var lineTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(price_tier_id,0), COALESCE(line_total,0)::float8
		FROM %s.order_items
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&tierID, &lineTotal); err != nil {
		t.Fatalf("query order item: %v", err)
	}
	if tierID != 102 || lineTotal != 900 {
		t.Fatalf("tier/line_total=%d/%.2f, want 102/900.00", tierID, lineTotal)
	}
}

func TestOrderAPISavesSelectedBeanListPublicationVersion(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.bean_list_publications(id, list_type, version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor)
		VALUES(88,'commercial','V3.0.8','published','customer','3','{}'::jsonb,'{"title":"客户豆单 V3.0.8"}'::jsonb,'新版','tester');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":               "2026-05-18",
		"customer_id":              3,
		"source_id":                1,
		"order_type_id":            1,
		"pay_status_id":            2,
		"payment_method":           "微信支付",
		"ship_status_id":           1,
		"bean_list_publication_id": 88,
		"product_id":               []string{"7"},
		"tier_id":                  []string{""},
		"unit_price":               []string{"99"},
		"item_name":                []string{"客户A自定义货品名"},
		"qty":                      []string{"1"},
		"unit":                     []string{"件"},
		"spec":                     []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var publicationID int64
	var version string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT bean_list_publication_id, bean_list_version_no
		FROM %s.orders
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&publicationID, &version); err != nil {
		t.Fatalf("query order bean list version: %v", err)
	}
	if publicationID != 88 || version != "V3.0.8" {
		t.Fatalf("bean list publication/version=%d/%q, want 88/V3.0.8", publicationID, version)
	}
}

func TestOrderAPIWholesaleExactSpecTierUsesKilogramQuantityForKgProducts(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,default_price,active,retail_price_227g)
		VALUES (77,'曲奇拼配',0,true,0);
		INSERT INTO %[1]s.product_price_tiers(id, product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, min_qty_lb, max_qty_lb, price_per_lb, active)
		VALUES
			(901,77,2500,1,23,550,1,23,0,true),
			(902,77,2500,24,49,512.50,24,49,0,true),
			(903,77,2500,50,NULL,475,50,NULL,0,true);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-05-17",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  2,
		"payment_method": "微信支付",
		"ship_status_id": 1,
		"product_id":     []string{"77"},
		"tier_id":        []string{""},
		"unit_price":     []string{""},
		"item_name":      []string{"曲奇拼配"},
		"qty":            []string{"10"},
		"unit":           []string{"件"},
		"spec":           []string{"2500"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var tierID int64
	var unitPrice, lineTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(price_tier_id,0), COALESCE(unit_price,0)::float8, COALESCE(line_total,0)::float8
		FROM %s.order_items
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&tierID, &unitPrice, &lineTotal); err != nil {
		t.Fatalf("query order item: %v", err)
	}
	if tierID != 902 || unitPrice != 205 || lineTotal != 5125 {
		t.Fatalf("tier/unit_price/line_total=%d/%.2f/%.2f, want 902/205.00/5125.00", tierID, unitPrice, lineTotal)
	}
}

func TestOrderAPICreatesDripBagOrderSavesUnitMetadataPriceAndAudit(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.products
		SET product_kind='drip_bag', drip_bag_grams=10, drip_box_bag_count=10
		WHERE id=7;
		INSERT INTO %[1]s.product_price_tiers(id, product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, active, product_kind, price_basis, sales_unit, unit_bag_count, price_source_json)
		VALUES (9101,7,10,100,NULL,2.15,true,'drip_bag','unit','bag',1,'{"version":"V3.0.6","source":"published"}'::jsonb);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-05-18",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  2,
		"payment_method": "微信支付",
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{"auto"},
		"item_name":      []string{"耶加雪菲挂耳"},
		"qty":            []string{"120"},
		"unit":           []string{"袋"},
		"product_kind":   []string{"drip_bag"},
		"sales_unit":     []string{"bag"},
		"unit_bag_count": []string{"1"},
		"unit_bean_g":    []string{"10"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var orderID int64
	var productKind, salesUnit, priceSource string
	var unitBagCount int
	var unitBeanG, matchedQty, unitPrice, lineTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT order_id, product_kind, sales_unit, unit_bag_count, unit_bean_g::float8, matched_price_qty::float8, price_source_json::text, unit_price::float8, line_total::float8
		FROM %s.order_items
		WHERE product_id=7
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&orderID, &productKind, &salesUnit, &unitBagCount, &unitBeanG, &matchedQty, &priceSource, &unitPrice, &lineTotal); err != nil {
		t.Fatalf("query drip order item: %v", err)
	}
	if productKind != "drip_bag" || salesUnit != "bag" || unitBagCount != 1 || unitBeanG != 10 || matchedQty != 120 {
		t.Fatalf("saved metadata kind=%q unit=%q bags=%d bean=%.3f matched=%.3f", productKind, salesUnit, unitBagCount, unitBeanG, matchedQty)
	}
	if unitPrice != 2.15 || lineTotal != 258 {
		t.Fatalf("unit_price/line_total=%.2f/%.2f, want 2.15/258.00", unitPrice, lineTotal)
	}
	if !strings.Contains(priceSource, `"version": "V3.0.6"`) && !strings.Contains(priceSource, `"version":"V3.0.6"`) {
		t.Fatalf("price_source_json missing version: %s", priceSource)
	}
	var auditNewValue string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(new_value,'')
		FROM %s.order_audit_logs
		WHERE order_id=$1
		ORDER BY id DESC
		LIMIT 1
	`, schema), orderID).Scan(&auditNewValue); err != nil {
		t.Fatalf("query order audit: %v", err)
	}
	for _, want := range []string{"drip_bag", "bag", "120", "source"} {
		if !strings.Contains(auditNewValue, want) {
			t.Fatalf("audit new_value missing %q: %s", want, auditNewValue)
		}
	}

	editReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/order/form?edit_id=%d", orderID), nil)
	editRec := httptest.NewRecorder()
	e.ServeHTTP(editRec, editReq)
	if editRec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form edit status = %d, want 200, body=%s", editRec.Code, editRec.Body.String())
	}
	editBody := editRec.Body.String()
	for _, want := range []string{`"product_kind":"drip_bag"`, `"sales_unit":"bag"`, `"unit_bag_count":1`, `"unit_bean_g":"10"`, `"matched_price_qty":"120"`, `"unit_conversion_label":"10g/袋"`, `"price_source_json":`} {
		if !strings.Contains(editBody, want) {
			t.Fatalf("edit data missing %s: %s", want, editBody)
		}
	}
}

func TestOrderAPICreatesDripBoxOrderFallsBackToBagTier(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.products
		SET product_kind='drip_bag', drip_bag_grams=10, drip_box_bag_count=10
		WHERE id=7;
		INSERT INTO %[1]s.product_price_tiers(id, product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, active, product_kind, price_basis, sales_unit, unit_bag_count, price_source_json)
		VALUES (9201,7,10,100,NULL,2.15,true,'drip_bag','unit','bag',1,'{"version":"V3.0.6","source":"published"}'::jsonb);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-05-18",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  2,
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{"auto"},
		"item_name":      []string{"耶加雪菲挂耳"},
		"qty":            []string{"12"},
		"unit":           []string{"盒"},
		"product_kind":   []string{"drip_bag"},
		"sales_unit":     []string{"box"},
		"unit_bag_count": []string{"10"},
		"unit_bean_g":    []string{"10"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var tierID int64
	var productKind, salesUnit string
	var unitBagCount int
	var matchedQty, unitPrice, lineTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(price_tier_id,0), product_kind, sales_unit, unit_bag_count, matched_price_qty::float8, unit_price::float8, line_total::float8
		FROM %s.order_items
		WHERE product_id=7
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&tierID, &productKind, &salesUnit, &unitBagCount, &matchedQty, &unitPrice, &lineTotal); err != nil {
		t.Fatalf("query drip box order item: %v", err)
	}
	if tierID != 9201 || productKind != "drip_bag" || salesUnit != "box" || unitBagCount != 10 || matchedQty != 120 {
		t.Fatalf("saved tier/metadata=%d %q %q bags=%d matched=%.3f", tierID, productKind, salesUnit, unitBagCount, matchedQty)
	}
	if unitPrice != 21.50 || lineTotal != 258 {
		t.Fatalf("unit_price/line_total=%.2f/%.2f, want 21.50/258.00", unitPrice, lineTotal)
	}
}

func TestOrderAPISavesAndListsEmployeeResponsiblePerson(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIResponsibleData(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":       "2026-05-06",
		"customer_id":      3,
		"source_id":        1,
		"order_type_id":    1,
		"pay_status_id":    2,
		"payment_method":   "微信支付",
		"ship_status_id":   1,
		"responsible_type": "employee",
		"responsible_id":   5,
		"product_id":       []string{"7"},
		"tier_id":          []string{"manual"},
		"unit_price":       []string{"88"},
		"item_name":        []string{"橘皮乌龙"},
		"qty":              []string{"1"},
		"unit":             []string{"件"},
		"spec":             []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var orderID int64
	var responsibleType, responsibleName string
	var responsibleID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(responsible_party_type,''), COALESCE(responsible_party_id,0), COALESCE(responsible_party_name,'')
		FROM %s.orders
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&orderID, &responsibleType, &responsibleID, &responsibleName); err != nil {
		t.Fatalf("query order responsible person: %v", err)
	}
	if responsibleType != "employee" || responsibleID != 5 || responsibleName != "销售小王" {
		t.Fatalf("responsible party = %s/%d/%s, want employee/5/销售小王", responsibleType, responsibleID, responsibleName)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/orders?limit=1", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	for _, needle := range []string{`"responsible_type":"employee"`, `"responsible_id":5`, `"responsible_name":"销售小王"`} {
		if !strings.Contains(rec.Body.String(), needle) {
			t.Fatalf("GET /api/orders missing %s: %s", needle, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/order/form?edit_id=%d", orderID), nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form edit status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	for _, needle := range []string{`"responsible_type":"employee"`, `"responsible_id":5`, `"responsible_name":"销售小王"`} {
		if !strings.Contains(rec.Body.String(), needle) {
			t.Fatalf("GET /api/order/form edit missing %s: %s", needle, rec.Body.String())
		}
	}
}

func TestOrderAPIEditsPaidOrderRequirePaymentMethodAndExposeToList(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, source_id, order_type_id, pay_status_id, ship_status_id, total_amount, grand_total)
		VALUES(91, 'SO-PAYMENT-METHOD', '2026-05-15', 3, 1, 1, 1, 1, 88, 88);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES(91, 1, 7, '橘皮乌龙', 1, '件', '454g', 88, 88);
	`, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"edit_id":        91,
		"order_date":     "2026-05-15",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  2,
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{"manual"},
		"unit_price":     []string{"88"},
		"item_name":      []string{"橘皮乌龙"},
		"qty":            []string{"1"},
		"unit":           []string{"件"},
		"spec":           []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "payment_method required") {
		t.Fatalf("paid edit without payment_method status=%d body=%s", rec.Code, rec.Body.String())
	}

	payload["payment_method"] = "银行转账"
	body, _ = json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("paid edit with payment_method status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/orders?q=SO-PAYMENT-METHOD", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"payment_method":"银行转账"`) {
		t.Fatalf("GET /api/orders payment_method status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOrderAPISavesCustomerResponsiblePersonForPartnerCommission(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIResponsibleData(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":       "2026-05-06",
		"customer_id":      3,
		"source_id":        1,
		"order_type_id":    1,
		"pay_status_id":    2,
		"payment_method":   "微信支付",
		"ship_status_id":   1,
		"responsible_type": "customer",
		"responsible_id":   4,
		"product_id":       []string{"7"},
		"tier_id":          []string{"manual"},
		"unit_price":       []string{"88"},
		"item_name":        []string{"橘皮乌龙"},
		"qty":              []string{"1"},
		"unit":             []string{"件"},
		"spec":             []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var responsibleType, responsibleName string
	var responsibleID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(responsible_party_type,''), COALESCE(responsible_party_id,0), COALESCE(responsible_party_name,'')
		FROM %s.orders
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&responsibleType, &responsibleID, &responsibleName); err != nil {
		t.Fatalf("query customer responsible person: %v", err)
	}
	if responsibleType != "customer" || responsibleID != 4 || responsibleName != "渠道伙伴A" {
		t.Fatalf("responsible party = %s/%d/%s, want customer/4/渠道伙伴A", responsibleType, responsibleID, responsibleName)
	}
}

func TestFilterOrderProductsForCustomerKeepsPublicAndOwnProducts(t *testing.T) {
	products := []ProductOption{
		{ID: 1, Name: "公共拼配", CustomerID: 0, Visibility: "public"},
		{ID: 2, Name: "测试客户专属深烘", CustomerID: 3, Visibility: "customer_only"},
		{ID: 3, Name: "其他客户专属深烘", CustomerID: 4, Visibility: "customer_only"},
	}

	got := filterOrderProductsForCustomer(products, 3)
	names := make([]string, 0, len(got))
	for _, product := range got {
		names = append(names, product.Name)
	}
	if strings.Join(names, ",") != "公共拼配,测试客户专属深烘" {
		t.Fatalf("filtered names = %q", strings.Join(names, ","))
	}
}

func TestOrderAPIListUsesSalesReadModel(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (9, 'SO-API-LIST', '2026-04-26', 3, 2, 1, 1, 1, 123.45, false);
		INSERT INTO %s.order_audit_logs(order_id, actor, field, old_value, new_value)
		VALUES (9, '测试员', 'create', '', 'SO-API-LIST');
	`, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/orders?limit=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"rows"`, `"order_no":"SO-API-LIST"`, `"summary"`, `"order_types"`, `"process_statuses"`, `"total":`, `"total_pages":`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/orders missing %s: %s", needle, body)
		}
	}
}

func TestOrderAPIListSearchMatchesResponsibleName(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, responsible_party_type, responsible_party_id, responsible_party_name, is_void)
		VALUES
			(31, 'SO-RESP-001', '2026-05-06', 3, 1, 2, 1, 1, 88, 'employee', 5, '销售小王', false),
			(32, 'SO-RESP-002', '2026-05-06', 3, 1, 2, 1, 1, 99, 'employee', 6, '售后小李', false);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/orders?q=销售小王&limit=10", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"order_no":"SO-RESP-001"`, `"responsible_name":"销售小王"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/orders responsible search missing %s: %s", needle, body)
		}
	}
	if strings.Contains(body, "SO-RESP-002") {
		t.Fatalf("GET /api/orders responsible search returned non-matching order: %s", body)
	}
}

func TestOrderAPIListUsesOrderRecipientSnapshot(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(
			id, order_no, order_date, customer_id, source_id, order_type_id, pay_status_id,
			ship_status_id, process_status_id, grand_total, is_void,
			receiver_name, receiver_phone, receiver_address, receiver_company,
			portal_service_code, source_warehouse
		)
		VALUES (
			19, 'SO-RECIPIENT-SNAPSHOT', '2026-05-04', 3, 1, 1, 2,
			1, 1, 88.00, false,
			'张三', '13800000001', '上海市测试路1号', '上海测试公司',
			'processing_ship', 'cust_3_processing'
		);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/orders?limit=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{
		`"receiver_name":"张三"`,
		`"receiver_phone":"13800000001"`,
		`"receiver_address":"上海市测试路1号"`,
		`"receiver_company":"上海测试公司"`,
		`"portal_service_code":"processing_ship"`,
		`"source_warehouse":"cust_3_processing"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/orders missing %s: %s", needle, body)
		}
	}
}

func TestOrderAPIFormEditReturnsRecipientSnapshot(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(
			id, order_no, order_date, customer_id, source_id, order_type_id, pay_status_id,
			ship_status_id, process_status_id, grand_total, is_void,
			receiver_name, receiver_phone, receiver_address, receiver_company,
			portal_service_code, source_warehouse
		)
		VALUES (
			29, 'SO-FORM-RECIPIENT', '2026-05-04', 3, 1, 1, 2,
			1, 1, 88.00, false,
			'李四', '13800000002', '杭州市测试路2号', '杭州测试公司',
			'direct_ship', 'cust_3_direct'
		);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES(29, 1, 7, '橘皮乌龙', 1, '件', '454g', 88, 88);
	`, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form?edit_id=29", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form edit status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{
		`"receiver_name":"李四"`,
		`"receiver_phone":"13800000002"`,
		`"receiver_address":"杭州市测试路2号"`,
		`"receiver_company":"杭州测试公司"`,
		`"portal_service_code":"direct_ship"`,
		`"source_warehouse":"cust_3_direct"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/order/form edit missing %s: %s", needle, body)
		}
	}
}

func TestOrderAPIListSupportsCustomerFilterAndFeeBreakdown(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(
			id, order_no, order_date, customer_id, source_id, order_type_id, pay_status_id,
			ship_status_id, process_status_id, total_amount, shipping_amount, discount_amount, grand_total,
			express_fee, outsource_material_fee, outsource_roast_fee, outsource_packaging_fee,
			outsource_manual_fee, outsource_tax_fee, outsource_other_fee, outsource_total_fee, is_void
		)
		VALUES
			(43, 'SO-FEE-CUSTOMER-3', '2026-05-07', 3, 1, 1, 2, 1, 1, 128.00, 12.00, 5.00, 135.00,
			 '顺丰18元', 30.00, 40.00, 8.00, 6.00, 2.00, 4.00, 90.00, false),
			(44, 'SO-FEE-CUSTOMER-4', '2026-05-07', 4, 1, 1, 2, 1, 1, 200.00, 18.00, 10.00, 208.00,
			 '', 0, 0, 0, 0, 0, 0, 0, false);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/orders?customer_id=3&limit=20", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders customer filter status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{
		`"order_no":"SO-FEE-CUSTOMER-3"`,
		`"total_amount":"128.00"`,
		`"shipping_amount":"12.00"`,
		`"discount_amount":"5.00"`,
		`"grand_total":"135.00"`,
		`"express_fee":"顺丰18元"`,
		`"outsource_material_fee":"30.00"`,
		`"outsource_roast_fee":"40.00"`,
		`"outsource_packaging_fee":"8.00"`,
		`"outsource_manual_fee":"6.00"`,
		`"outsource_tax_fee":"2.00"`,
		`"outsource_other_fee":"4.00"`,
		`"outsource_total_fee":"90.00"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/orders customer filter missing %s: %s", needle, body)
		}
	}
	if strings.Contains(body, "SO-FEE-CUSTOMER-4") {
		t.Fatalf("GET /api/orders customer filter leaked other customer order: %s", body)
	}
}

func TestOrderAPIListKeepsSearchKeywordForResponsibleLookup(t *testing.T) {
	repo := &capturingOrderListRepo{}
	e := echo.New()
	registerOrderAPI(e, salesapp.NewService(repo), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/orders?q="+url.QueryEscape("销售小王")+"&limit=20", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if repo.query.Q != "销售小王" {
		t.Fatalf("orders API q = %q, want responsible search keyword", repo.query.Q)
	}
	if repo.query.Limit != 20 {
		t.Fatalf("orders API limit = %d, want 20", repo.query.Limit)
	}
}

func TestOrderAPIListCarriesOrderScopeAndCurrentEmployee(t *testing.T) {
	repo := &capturingOrderListRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	registerOrderAPI(e, salesapp.NewService(repo), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/orders?scope=mine&limit=20", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if repo.query.Scope != "mine" {
		t.Fatalf("orders API scope = %q, want mine", repo.query.Scope)
	}
	if repo.query.EmployeeID != 7 {
		t.Fatalf("orders API employee id = %d, want 7", repo.query.EmployeeID)
	}
}

func TestOrderAPIListFulfillmentScopeAllowsCustomerWorkbenchPermission(t *testing.T) {
	repo := &capturingOrderListRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	e.Use(support.AuthorizationMiddleware(&orderAPIAuthzService{actor: authzapp.Actor{
		Permissions: []string{"customer_processing.read"},
	}}))
	registerOrderAPI(e, salesapp.NewService(repo), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/orders?scope=fulfillment&customer_id=152&limit=20", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders fulfillment customer workbench status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("GET /api/orders fulfillment customer workbench response must be valid JSON, got %q: %v", rec.Body.String(), err)
	}
	if !repo.called {
		t.Fatal("orders API was not called for customer workbench fulfillment scope")
	}
	if repo.query.Scope != "fulfillment" {
		t.Fatalf("orders API scope = %q, want fulfillment", repo.query.Scope)
	}
	if repo.query.CustomerID != 152 {
		t.Fatalf("orders API customer id = %d, want 152", repo.query.CustomerID)
	}
	if repo.query.EmployeeID != 7 {
		t.Fatalf("orders API employee id = %d, want 7", repo.query.EmployeeID)
	}
	if repo.query.FulfillmentEmployeeID != 7 {
		t.Fatalf("orders API fulfillment employee id = %d, want 7", repo.query.FulfillmentEmployeeID)
	}
}

func TestOrderAPIDetailAllowsCustomerWorkbenchBoundOrder(t *testing.T) {
	repo := &capturingOrderDetailRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	e.Use(support.AuthorizationMiddleware(&orderAPIAuthzService{actor: authzapp.Actor{
		Permissions: []string{"customer_processing.read"},
	}}))
	registerOrderAPI(e, salesapp.NewService(repo), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/orders/88/detail", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders/88/detail customer workbench status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if repo.formEditID != 88 {
		t.Fatalf("order detail form edit id = %d, want 88", repo.formEditID)
	}
	if !repo.accessCalled {
		t.Fatal("order detail must verify customer workbench fulfillment binding")
	}
	if repo.accessQuery.OrderID != 88 || repo.accessQuery.Scope != "fulfillment" || repo.accessQuery.CustomerID != 152 || repo.accessQuery.FulfillmentEmployeeID != 7 {
		t.Fatalf("order detail access query = %#v, want order 88 customer 152 employee 7 fulfillment scope", repo.accessQuery)
	}
	body := rec.Body.String()
	for _, needle := range []string{
		`"edit_mode":true`,
		`"receiver_name":"李四"`,
		`"receiver_phone":"13800000002"`,
		`"bean_list_publication_id":7`,
		`"bean_list_version_no":"V3.0.8"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/orders/88/detail missing %s: %s", needle, body)
		}
	}
}

func TestOrderAPIListFulfillmentScopeSkipsLegacyNonWorkbenchBinding(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		ALTER TABLE %[1]s.customers ADD COLUMN IF NOT EXISTS customer_type TEXT NOT NULL DEFAULT 'retail';
		CREATE TABLE %[1]s.customer_portal_profiles (
			customer_id BIGINT PRIMARY KEY,
			capability_template_key TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %[1]s.customer_capability_templates (
			template_key TEXT PRIMARY KEY,
			active BOOLEAN NOT NULL DEFAULT true,
			erp_permissions JSONB NOT NULL DEFAULT '[]'::jsonb,
			erp_view_keys JSONB NOT NULL DEFAULT '[]'::jsonb
		);
		CREATE TABLE %[1]s.customer_erp_user_bindings (
			id BIGSERIAL PRIMARY KEY,
			customer_id BIGINT NOT NULL,
			employee_id BIGINT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active'
		);
		ALTER TABLE %[1]s.company_employees ADD COLUMN IF NOT EXISTS account_type TEXT NOT NULL DEFAULT 'channel_customer';
		CREATE TABLE IF NOT EXISTS %[1]s.employee_login_passwords (
			employee_id BIGINT PRIMARY KEY,
			password_hash TEXT NOT NULL DEFAULT '',
			login_disabled BOOLEAN NOT NULL DEFAULT false
		);
		INSERT INTO %[1]s.customers(id,name,customer_type,active) VALUES
			(31,'历史零售模板批发客户','wholesale',true),
			(32,'有效代加工履约客户','wholesale',true);
		INSERT INTO %[1]s.company_departments(id,name,active) VALUES (1,'渠道部',true);
		INSERT INTO %[1]s.company_employees(id,name,phone,department_id,account_type,active) VALUES
			(501,'历史零售模板账号','13900000501',1,'channel_customer',true),
			(502,'有效代加工账号','13900000502',1,'channel_customer',true);
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, capability_template_key) VALUES
			(31,'retail_mall'),
			(32,'processing_fulfillment');
		INSERT INTO %[1]s.customer_erp_user_bindings(customer_id, employee_id, status) VALUES
			(31,501,'active'),
			(32,502,'active');
		INSERT INTO %[1]s.orders(order_no, order_date, customer_id, source_id, order_type_id, pay_status_id, ship_status_id, process_status_id, portal_service_code, grand_total)
		VALUES
			('SO-LEGACY-RETAIL-WORKBENCH', '2026-05-13', 31, 1, 1, 1, 1, 1, 'direct_ship', 11),
			('SO-VALID-PROCESSING-WORKBENCH', '2026-05-13', 32, 1, 1, 1, 1, 1, 'direct_ship', 22);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/orders?scope=fulfillment&limit=20", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders fulfillment status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "SO-VALID-PROCESSING-WORKBENCH") {
		t.Fatalf("fulfillment scope should include valid workbench binding order: %s", body)
	}
	if strings.Contains(body, "SO-LEGACY-RETAIL-WORKBENCH") {
		t.Fatalf("fulfillment scope leaked legacy non-workbench binding order: %s", body)
	}
}

func TestOrderAPIListFulfillmentScopeSkipsDisabledLoginBinding(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		ALTER TABLE %[1]s.customers ADD COLUMN IF NOT EXISTS customer_type TEXT NOT NULL DEFAULT 'retail';
		CREATE TABLE %[1]s.customer_portal_profiles (
			customer_id BIGINT PRIMARY KEY,
			capability_template_key TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %[1]s.customer_capability_templates (
			template_key TEXT PRIMARY KEY,
			active BOOLEAN NOT NULL DEFAULT true,
			erp_permissions JSONB NOT NULL DEFAULT '[]'::jsonb,
			erp_view_keys JSONB NOT NULL DEFAULT '[]'::jsonb
		);
		CREATE TABLE %[1]s.customer_erp_user_bindings (
			id BIGSERIAL PRIMARY KEY,
			customer_id BIGINT NOT NULL,
			employee_id BIGINT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active'
		);
		ALTER TABLE %[1]s.company_employees ADD COLUMN IF NOT EXISTS account_type TEXT NOT NULL DEFAULT 'channel_customer';
		CREATE TABLE IF NOT EXISTS %[1]s.employee_login_passwords (
			employee_id BIGINT PRIMARY KEY,
			password_hash TEXT NOT NULL DEFAULT '',
			login_disabled BOOLEAN NOT NULL DEFAULT false
		);
		INSERT INTO %[1]s.customers(id,name,customer_type,active) VALUES
			(33,'禁用账号履约客户','wholesale',true),
			(34,'启用账号履约客户','wholesale',true);
		INSERT INTO %[1]s.company_departments(id,name,active) VALUES (1,'渠道部',true);
		INSERT INTO %[1]s.company_employees(id,name,phone,department_id,account_type,active) VALUES
			(503,'禁用渠道账号','13900000503',1,'channel_customer',true),
			(504,'启用渠道账号','13900000504',1,'channel_customer',true);
		INSERT INTO %[1]s.employee_login_passwords(employee_id, password_hash, login_disabled) VALUES
			(503,'disabled-hash',true),
			(504,'enabled-hash',false);
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, capability_template_key) VALUES
			(33,'processing_fulfillment'),
			(34,'processing_fulfillment');
		INSERT INTO %[1]s.customer_erp_user_bindings(customer_id, employee_id, status) VALUES
			(33,503,'active'),
			(34,504,'active');
		INSERT INTO %[1]s.orders(order_no, order_date, customer_id, source_id, order_type_id, pay_status_id, ship_status_id, process_status_id, portal_service_code, grand_total)
		VALUES
			('SO-DISABLED-LOGIN-BINDING', '2026-05-13', 33, 1, 1, 1, 1, 1, 'direct_ship', 33),
			('SO-ENABLED-LOGIN-BINDING', '2026-05-13', 34, 1, 1, 1, 1, 1, 'direct_ship', 44);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/orders?scope=fulfillment&limit=20", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders fulfillment status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "SO-ENABLED-LOGIN-BINDING") {
		t.Fatalf("fulfillment scope should include enabled login binding order: %s", body)
	}
	if strings.Contains(body, "SO-DISABLED-LOGIN-BINDING") {
		t.Fatalf("fulfillment scope leaked disabled login binding order: %s", body)
	}
}

func TestOrderAPIListRejectsInvalidScope(t *testing.T) {
	repo := &capturingOrderListRepo{}
	e := echo.New()
	registerOrderAPI(e, salesapp.NewService(repo), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/orders?scope=fulfillment_typo&limit=20", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /api/orders invalid scope status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if repo.called {
		t.Fatal("orders API must reject invalid scope before querying repository")
	}
	if !strings.Contains(rec.Body.String(), "invalid scope") {
		t.Fatalf("GET /api/orders invalid scope body should explain invalid scope, got %s", rec.Body.String())
	}
}

type capturingOrderListRepo struct {
	salesapp.Repository
	called bool
	query  salesapp.OrderListQuery
}

type capturingOrderDetailRepo struct {
	salesapp.Repository
	formEditID   int64
	accessCalled bool
	accessQuery  salesapp.OrderListQuery
}

type orderAPIAuthzService struct {
	actor authzapp.Actor
}

func (s *orderAPIAuthzService) ActorByEmployeeID(ctx context.Context, employeeID int64) (authzapp.Actor, error) {
	actor := s.actor
	actor.EmployeeID = employeeID
	return actor, nil
}

func (s *orderAPIAuthzService) ListRoles(ctx context.Context) ([]authzapp.Role, error) {
	return nil, nil
}

func (s *orderAPIAuthzService) ListEmployeeRoles(ctx context.Context) (map[int64][]string, error) {
	return nil, nil
}

func (s *orderAPIAuthzService) AssignEmployeeRoles(ctx context.Context, cmd authzapp.AssignmentCommand) error {
	return nil
}

func (r *capturingOrderListRepo) ListOrders(ctx context.Context, query salesapp.OrderListQuery) (salesapp.OrderListResult, error) {
	r.called = true
	r.query = query
	return salesapp.OrderListResult{
		Rows: []salesapp.OrderRow{{
			ID:              31,
			OrderNo:         "SO-RESP-001",
			ResponsibleName: "销售小王",
		}},
		Summary: salesapp.OrdersSummary{Orders: 1, Customers: 1},
	}, nil
}

func (r *capturingOrderDetailRepo) OrderForm(ctx context.Context, editID int64) (salesapp.OrderFormData, error) {
	r.formEditID = editID
	return salesapp.OrderFormData{
		EditData: &salesapp.OrderEditData{
			ID:              editID,
			OrderNo:         "SO-DETAIL-88",
			OrderDate:       "2026-05-17",
			CustomerID:      152,
			ReceiverName:    "李四",
			ReceiverPhone:   "13800000002",
			ReceiverAddress: "杭州市测试路2号",
			GrandTotal:      "88.00",
			Items: []salesapp.OrderEditItem{{
				ItemID:                901,
				ProductID:             12,
				Product:               "兰卡拼配",
				Spec:                  "1000g",
				Qty:                   "2",
				Unit:                  "件",
				UnitPrice:             "82.00",
				LineTotal:             "164.00",
				BeanListPublicationID: 7,
				BeanListVersionNo:     "V3.0.8",
			}},
		},
	}, nil
}

func (r *capturingOrderDetailRepo) ListOrders(ctx context.Context, query salesapp.OrderListQuery) (salesapp.OrderListResult, error) {
	r.accessCalled = true
	r.accessQuery = query
	if query.OrderID == 88 && query.CustomerID == 152 && query.FulfillmentEmployeeID == 7 {
		return salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 88}}}, nil
	}
	return salesapp.OrderListResult{}, nil
}

type capturingSaveOrderRepo struct {
	salesapp.Repository
	cmd salesapp.SaveOrderCommand
}

func (r *capturingSaveOrderRepo) SaveOrder(ctx context.Context, cmd salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error) {
	r.cmd = cmd
	return salesapp.SaveOrderResult{OrderID: 71, OrderNo: "SO-NOTE-001"}, nil
}

type capturingMessagePublisher struct {
	cmd messagecenterapp.PublishCommand
}

func (p *capturingMessagePublisher) Publish(ctx context.Context, cmd messagecenterapp.PublishCommand) (int64, error) {
	p.cmd = cmd
	return 99, nil
}

func TestOrderAPISavePublishesNewOrderNotification(t *testing.T) {
	repo := &capturingSaveOrderRepo{}
	messages := &capturingMessagePublisher{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			return next(c)
		}
	})
	registerOrderAPI(e, salesapp.NewService(repo), messages)

	payload := map[string]any{
		"order_date":     "2026-05-09",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  2,
		"payment_method": "微信支付",
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{"manual"},
		"unit_price":     []string{"88"},
		"item_name":      []string{"花魁"},
		"qty":            []string{"1"},
		"unit":           []string{"件"},
		"spec":           []string{"100"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status=%d body=%s", rec.Code, rec.Body.String())
	}
	if messages.cmd.EventType != "order.created" || messages.cmd.SourceID != 71 {
		t.Fatalf("publish command = %#v", messages.cmd)
	}
	if got := messages.cmd.Payload["highlight_order_id"]; got != int64(71) {
		t.Fatalf("highlight payload = %#v, want order id 71", got)
	}
}

func TestOrderAPISaveCarriesItemNotes(t *testing.T) {
	repo := &capturingSaveOrderRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			return next(c)
		}
	})
	registerOrderAPI(e, salesapp.NewService(repo), nil)

	payload := map[string]any{
		"order_date":     "2026-05-09",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  2,
		"payment_method": "微信支付",
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{"manual"},
		"unit_price":     []string{"88"},
		"item_name":      []string{"橘皮乌龙"},
		"item_note":      []string{"贴标：A店"},
		"qty":            []string{"2"},
		"unit":           []string{"件"},
		"spec":           []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.cmd.Items) != 1 {
		t.Fatalf("captured items len = %d, want 1", len(repo.cmd.Items))
	}
	if repo.cmd.Items[0].Note != "贴标：A店" {
		t.Fatalf("captured item note = %q, want per-item note", repo.cmd.Items[0].Note)
	}
}

func TestOrderAPISaveCarriesDripBagMetadata(t *testing.T) {
	repo := &capturingSaveOrderRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			return next(c)
		}
	})
	registerOrderAPI(e, salesapp.NewService(repo), nil)

	payload := map[string]any{
		"order_date":     "2026-05-18",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  2,
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{"auto"},
		"item_name":      []string{"耶加雪菲挂耳"},
		"qty":            []string{"12"},
		"unit":           []string{"盒"},
		"product_kind":   []string{"drip_bag"},
		"sales_unit":     []string{"box"},
		"unit_bag_count": []string{"10"},
		"unit_bean_g":    []string{"10"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.cmd.Items) != 1 {
		t.Fatalf("captured items len = %d, want 1", len(repo.cmd.Items))
	}
	got := repo.cmd.Items[0]
	if got.ProductKind != "drip_bag" || got.SalesUnit != "box" || got.UnitBagCount != 10 || got.UnitBeanG != 10 {
		t.Fatalf("drip metadata = kind %q unit %q bags %d bean %.3f", got.ProductKind, got.SalesUnit, got.UnitBagCount, got.UnitBeanG)
	}
}

func TestOrderAPISaveCarriesItemDiscounts(t *testing.T) {
	repo := &capturingSaveOrderRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			return next(c)
		}
	})
	registerOrderAPI(e, salesapp.NewService(repo), nil)

	payload := map[string]any{
		"order_date":     "2026-05-15",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  2,
		"payment_method": "微信支付",
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{"manual"},
		"unit_price":     []string{"88"},
		"item_name":      []string{"橘皮乌龙"},
		"qty":            []string{"2"},
		"unit":           []string{"件"},
		"spec":           []string{"454"},
		"discount_type":  []string{"percent"},
		"discount_value": []string{"50"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.cmd.Items) != 1 {
		t.Fatalf("captured items len = %d, want 1", len(repo.cmd.Items))
	}
	if repo.cmd.Items[0].DiscountType != "percent" || repo.cmd.Items[0].DiscountValue != 50 {
		t.Fatalf("captured item discount = %q/%v, want percent/50", repo.cmd.Items[0].DiscountType, repo.cmd.Items[0].DiscountValue)
	}
}

func TestOrderAPISavesRetailCustomSpecPrice(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-04-25",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  2,
		"pay_status_id":  1,
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{"auto"},
		"unit_price":     []string{""},
		"item_name":      []string{"橘皮乌龙"},
		"qty":            []string{"2"},
		"unit":           []string{"件"},
		"spec":           []string{"300"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var spec string
	var lineTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(spec,''), COALESCE(line_total,0)
		FROM %s.order_items
		WHERE product_id=7
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&spec, &lineTotal); err != nil {
		t.Fatalf("query order item: %v", err)
	}
	if spec != "300g" {
		t.Fatalf("saved spec = %q, want 300g", spec)
	}
	if lineTotal != 134 {
		t.Fatalf("line_total = %.2f, want 134.00", lineTotal)
	}
}

func TestOrderAPISavesWholesale1000gByBeanListWeightTier(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.product_price_tiers(product_id,spec_g,min_qty_units,max_qty_units,price_per_unit,min_qty_lb,max_qty_lb,price_per_lb,active)
		VALUES
			(7,454,2,13,63,2,13,63,true),
			(7,454,14,23,57,14,23,57,true),
			(7,454,24,48,51,24,48,51,true),
			(7,454,49,NULL,48,49,NULL,48,true);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-05-09",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  1,
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{"auto"},
		"unit_price":     []string{""},
		"item_name":      []string{"榛巧拼配"},
		"qty":            []string{"30"},
		"unit":           []string{"袋"},
		"spec":           []string{"1000"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var tierID int64
	var spec string
	var unitPrice, lineTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(price_tier_id,0), COALESCE(spec,''), COALESCE(unit_price,0)::float8, COALESCE(line_total,0)::float8
		FROM %s.order_items
		WHERE product_id=7
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&tierID, &spec, &unitPrice, &lineTotal); err != nil {
		t.Fatalf("query order item: %v", err)
	}
	if tierID == 0 {
		t.Fatalf("price_tier_id = 0, want matched bean-list tier")
	}
	if spec != "1000g" {
		t.Fatalf("saved spec = %q, want 1000g", spec)
	}
	if unitPrice != 106 {
		t.Fatalf("unit_price = %.2f, want 106.00", unitPrice)
	}
	wantLineTotal := 106 * 30.0
	if diff := lineTotal - wantLineTotal; diff > 0.0001 || diff < -0.0001 {
		t.Fatalf("line_total = %.6f, want %.6f", lineTotal, wantLineTotal)
	}
}

func TestOrderAPISavesManualWholesale1000gPriceAsKgUnit(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-05-09",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  1,
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{"manual"},
		"unit_price":     []string{"106"},
		"item_name":      []string{"榛巧拼配"},
		"qty":            []string{"30"},
		"unit":           []string{"袋"},
		"spec":           []string{"1000"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var unitPrice, lineTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(unit_price,0)::float8, COALESCE(line_total,0)::float8
		FROM %s.order_items
		WHERE product_id=7
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&unitPrice, &lineTotal); err != nil {
		t.Fatalf("query order item: %v", err)
	}
	if unitPrice != 106 {
		t.Fatalf("unit_price = %.2f, want 106.00", unitPrice)
	}
	if lineTotal != 3180 {
		t.Fatalf("line_total = %.2f, want 3180.00", lineTotal)
	}
}

func TestOrderAPIDefaultsNewOrderToPaidAndUnshipped(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-04-27",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  0,
		"payment_method": "微信支付",
		"ship_status_id": 0,
		"product_id":     []string{"7"},
		"tier_id":        []string{"manual"},
		"unit_price":     []string{"88"},
		"item_name":      []string{"橘皮乌龙"},
		"qty":            []string{"1"},
		"unit":           []string{"件"},
		"spec":           []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var payStatusID, shipStatusID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(pay_status_id,0), COALESCE(ship_status_id,0)
		FROM %s.orders
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&payStatusID, &shipStatusID); err != nil {
		t.Fatalf("query order statuses: %v", err)
	}
	if payStatusID != 2 || shipStatusID != 1 {
		t.Fatalf("saved statuses pay=%d ship=%d, want pay=2 ship=1", payStatusID, shipStatusID)
	}
}

func TestOrderStockBatchPreviewAPIShowsFIFOBatchChoice(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIFinishedBatches(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"product_id": []string{"7"},
		"item_name":  []string{"橘皮乌龙"},
		"qty":        []string{"3"},
		"unit":       []string{"件"},
		"spec":       []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order/stock-batch-preview", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order/stock-batch-preview status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	bodyText := rec.Body.String()
	for _, needle := range []string{`"sufficient":true`, `"product_name":"橘皮乌龙"`, `"batch_code":"FP-OLD-454"`, `"batch_code":"FP-NEW-454"`, `"total_need_g":1362`, `"allocated_g":908`, `"allocated_g":454`} {
		if !strings.Contains(bodyText, needle) {
			t.Fatalf("stock batch preview missing %s: %s", needle, bodyText)
		}
	}
}

func TestOrderStockBatchPreviewAPIUsesLegacyFinishedInventoryWhenNoBatchRows(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPILegacyFinishedInventory(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"product_id": []string{"7"},
		"item_name":  []string{"橘皮乌龙"},
		"qty":        []string{"2"},
		"unit":       []string{"件"},
		"spec":       []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order/stock-batch-preview", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order/stock-batch-preview status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	bodyText := rec.Body.String()
	for _, needle := range []string{`"sufficient":true`, `"has_batch_choices":true`, `"batch_id":0`, `"batch_code":"LEGACY-FP-7-454"`, `"total_available_g":1816`, `"allocated_g":908`} {
		if !strings.Contains(bodyText, needle) {
			t.Fatalf("legacy stock preview missing %s: %s", needle, bodyText)
		}
	}
}

func TestOrderAPISaveWithStockBatchDecisionMarksInventoryReadyAndStoresBatchChoice(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIFinishedBatches(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":           "2026-05-03",
		"customer_id":          3,
		"source_id":            1,
		"order_type_id":        1,
		"pay_status_id":        2,
		"payment_method":       "微信支付",
		"ship_status_id":       1,
		"stock_batch_decision": "use_batch",
		"product_id":           []string{"7"},
		"tier_id":              []string{"manual"},
		"unit_price":           []string{"88"},
		"item_name":            []string{"橘皮乌龙"},
		"qty":                  []string{"2"},
		"unit":                 []string{"件"},
		"spec":                 []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"stock_batch_used":true`) {
		t.Fatalf("save response should report stock batch used: %s", rec.Body.String())
	}

	var processStatus, decision, batchCode string
	var allocatedG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(ops.name,''), COALESCE(d.decision,''), COALESCE(a.batch_code,''), COALESCE(a.allocated_g,0)
		FROM %s.orders o
		LEFT JOIN %s.order_process_statuses ops ON ops.id=o.process_status_id
		LEFT JOIN %s.order_stock_decisions d ON d.order_id=o.id
		LEFT JOIN %s.order_stock_batch_allocations a ON a.order_id=o.id
		WHERE o.order_no = (
			SELECT order_no FROM %s.orders ORDER BY id DESC LIMIT 1
		)
		ORDER BY a.id
		LIMIT 1
	`, schema, schema, schema, schema, schema)).Scan(&processStatus, &decision, &batchCode, &allocatedG); err != nil {
		t.Fatalf("query stock decision: %v", err)
	}
	if processStatus != "库存待发货" || decision != "use_batch" || batchCode != "FP-OLD-454" || allocatedG != 908 {
		t.Fatalf("stock decision process=%q decision=%q batch=%q allocated=%d, want 库存待发货/use_batch/FP-OLD-454/908", processStatus, decision, batchCode, allocatedG)
	}
}

func TestOrderAPISaveWithLegacyFinishedInventoryDecisionMarksReadyAndShipReady(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPILegacyFinishedInventory(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":           "2026-05-03",
		"customer_id":          3,
		"source_id":            1,
		"order_type_id":        1,
		"pay_status_id":        2,
		"payment_method":       "微信支付",
		"ship_status_id":       1,
		"stock_batch_decision": "use_batch",
		"product_id":           []string{"7"},
		"tier_id":              []string{"manual"},
		"unit_price":           []string{"88"},
		"item_name":            []string{"橘皮乌龙"},
		"qty":                  []string{"2"},
		"unit":                 []string{"件"},
		"spec":                 []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"stock_batch_used":true`) {
		t.Fatalf("save response should report legacy stock used: %s", rec.Body.String())
	}

	var orderNo, processStatus, decision, batchCode string
	var batchID, allocatedG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(o.order_no,''), COALESCE(ops.name,''), COALESCE(d.decision,''), COALESCE(a.batch_id,0), COALESCE(a.batch_code,''), COALESCE(a.allocated_g,0)
		FROM %s.orders o
		LEFT JOIN %s.order_process_statuses ops ON ops.id=o.process_status_id
		LEFT JOIN %s.order_stock_decisions d ON d.order_id=o.id
		LEFT JOIN %s.order_stock_batch_allocations a ON a.order_id=o.id
		ORDER BY o.id DESC, a.id
		LIMIT 1
	`, schema, schema, schema, schema)).Scan(&orderNo, &processStatus, &decision, &batchID, &batchCode, &allocatedG); err != nil {
		t.Fatalf("query legacy stock decision: %v", err)
	}
	if processStatus != "库存待发货" || decision != "use_batch" || batchID != 0 || batchCode != "LEGACY-FP-7-454" || allocatedG != 908 {
		t.Fatalf("legacy stock decision process=%q decision=%q batch_id=%d batch=%q allocated=%d, want 库存待发货/use_batch/0/LEGACY-FP-7-454/908", processStatus, decision, batchID, batchCode, allocatedG)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/orders?ship_ready=1&limit=50", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders ship_ready status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), orderNo) {
		t.Fatalf("ship_ready list should include legacy stock-ready order %s: %s", orderNo, rec.Body.String())
	}
}

func TestOrderAPISaveWithProduceDecisionKeepsOrderInProductionGap(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIFinishedBatches(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":           "2026-05-03",
		"customer_id":          3,
		"source_id":            1,
		"order_type_id":        1,
		"pay_status_id":        2,
		"payment_method":       "微信支付",
		"ship_status_id":       1,
		"stock_batch_decision": "produce",
		"product_id":           []string{"7"},
		"tier_id":              []string{"manual"},
		"unit_price":           []string{"88"},
		"item_name":            []string{"橘皮乌龙"},
		"qty":                  []string{"1"},
		"unit":                 []string{"件"},
		"spec":                 []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var processStatus *string
	var decision string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT ops.name, COALESCE(d.decision,'')
		FROM %s.orders o
		LEFT JOIN %s.order_process_statuses ops ON ops.id=o.process_status_id
		LEFT JOIN %s.order_stock_decisions d ON d.order_id=o.id
		ORDER BY o.id DESC
		LIMIT 1
	`, schema, schema, schema)).Scan(&processStatus, &decision); err != nil {
		t.Fatalf("query produce decision: %v", err)
	}
	if processStatus != nil || decision != "produce" {
		t.Fatalf("declined stock process=%v decision=%q, want nil/produce", processStatus, decision)
	}
}

func TestOrderAPISaveDoesNotGenerateShippingExcel(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	exportDir := filepath.Join(dir, "exports")
	writeOrderShippingTemplateForTest(t, templatePath)
	t.Setenv("ORDER_SHIP_TEMPLATE", templatePath)
	t.Setenv("ORDER_SHIP_EXPORT_DIR", exportDir)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.sender_settings
		SET sender_name='寄件人', sender_phone='13900000000', sender_addr='上海市测试路', sender_company='寄件公司', sender_goods='', sf_biz_type='标快'
		WHERE id=1;
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-04-27",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  2,
		"payment_method": "微信支付",
		"ship_status_id": 1,
		"product_id":     []string{"7"},
		"tier_id":        []string{"manual"},
		"unit_price":     []string{"88"},
		"item_name":      []string{"橘皮乌龙"},
		"qty":            []string{"2"},
		"unit":           []string{"件"},
		"spec":           []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		OrderNo          string `json:"order_no"`
		ShippingExcelURL string `json:"shipping_excel_url"`
		Error            string `json:"shipping_excel_error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != "" || resp.ShippingExcelURL != "" {
		t.Fatalf("order save should not process shipping excel, response = %+v body=%s", resp, rec.Body.String())
	}
	files, err := os.ReadDir(exportDir)
	if err == nil && len(files) > 0 {
		t.Fatalf("order save generated shipping exports = %d, want 0", len(files))
	}
}

func TestOrdersShippingExcelAPIGeneratesFromSelectedOrders(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	exportDir := filepath.Join(dir, "exports")
	writeOrderShippingTemplateForTest(t, templatePath)
	t.Setenv("ORDER_SHIP_TEMPLATE", templatePath)
	t.Setenv("ORDER_SHIP_EXPORT_DIR", exportDir)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.sender_settings
		SET sender_name='寄件人', sender_phone='13900000000', sender_addr='上海市测试路', sender_company='寄件公司', sender_goods='', sf_biz_type='标快'
		WHERE id=1;
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('生产完成',20,true)
		ON CONFLICT(name) DO UPDATE SET sort=excluded.sort, active=excluded.active;
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (20, 'SO-SHIP-LIST', '2026-04-27', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='生产完成' LIMIT 1), 176, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES (20, 1, 7, '橘皮乌龙', 2, '件', '454g', 88, 176);
	`, schema, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{"order_ids": []int64{20}})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-excel", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/shipping-excel status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ShippingExcelURL string `json:"shipping_excel_url"`
		Error            string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != "" || resp.ShippingExcelURL == "" {
		t.Fatalf("shipping excel response = %+v body=%s", resp, rec.Body.String())
	}
	files, err := os.ReadDir(exportDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("export files = %d, want 1", len(files))
	}
	wb, err := excelize.OpenFile(filepath.Join(exportDir, files[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	defer wb.Close()
	sheet := wb.GetSheetName(0)
	cell := func(name string) string {
		v, err := wb.GetCellValue(sheet, name)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if cell("A2") != "测试收件人" || cell("B2") != "13800000000" || cell("D2") != "寄件人" || cell("H2") != "1" || cell("I2") != "茶叶" || cell("J2") != "0.1" {
		t.Fatalf("shipping excel cells A2=%q B2=%q D2=%q H2=%q I2=%q J2=%q", cell("A2"), cell("B2"), cell("D2"), cell("H2"), cell("I2"), cell("J2"))
	}
	if !strings.Contains(cell("N2"), "SO-SHIP-LIST") || !strings.Contains(cell("N2"), "橘皮乌龙 454g x2件") {
		t.Fatalf("shipping excel remark N2=%q", cell("N2"))
	}
	if strings.Contains(cell("N2"), "单价") || strings.Contains(cell("N2"), "小计") || strings.Contains(cell("N2"), "88") || strings.Contains(cell("N2"), "176") {
		t.Fatalf("shipping excel remark should not include price or subtotal N2=%q", cell("N2"))
	}
}

func TestOrdersShippingExcelAPICleansFileWhenShipmentSaveFails(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	exportDir := filepath.Join(dir, "exports")
	writeOrderShippingTemplateForTest(t, templatePath)
	t.Setenv("ORDER_SHIP_TEMPLATE", templatePath)
	t.Setenv("ORDER_SHIP_EXPORT_DIR", exportDir)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.sender_settings
		SET sender_name='寄件人', sender_phone='13900000000', sender_addr='上海市测试路', sender_company='寄件公司', sender_goods='', sf_biz_type='标快'
		WHERE id=1;
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('生产完成',20,true)
		ON CONFLICT(name) DO UPDATE SET sort=excluded.sort, active=excluded.active;
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (22, 'SO-SHIP-SAVE-FAIL', '2026-04-27', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='生产完成' LIMIT 1), 176, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES (22, 1, 7, '橘皮乌龙', 2, '件', '454g', 88, 176);
		ALTER TABLE %s.order_shipments ADD CONSTRAINT order_shipment_test_reject_generated CHECK (status <> 'excel_generated');
	`, schema, schema, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{"order_ids": []int64{22}})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-excel", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("POST /api/orders/shipping-excel status = %d, want 500, body=%s", rec.Code, rec.Body.String())
	}
	files, err := os.ReadDir(exportDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("failed shipment save left shipping export files=%v", files)
	}
}

func TestOrdersShippingExcelAPIAcceptsNoProductionShipReadyOrders(t *testing.T) {
	if !orderShippingReady(salesapp.OrderShippingExportData{ProcessStatus: "无需生产"}) {
		t.Fatal("无需生产 status should be treated as ready for shipping")
	}
	if !orderShippingReady(salesapp.OrderShippingExportData{ProcessStatus: "库存待发货"}) {
		t.Fatal("库存待发货 status should be treated as ready for shipping")
	}

	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	exportDir := filepath.Join(dir, "exports")
	writeOrderShippingTemplateForTest(t, templatePath)
	t.Setenv("ORDER_SHIP_TEMPLATE", templatePath)
	t.Setenv("ORDER_SHIP_EXPORT_DIR", exportDir)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.sender_settings
		SET sender_name='寄件人', sender_phone='13900000000', sender_addr='上海市测试路', sender_company='寄件公司', sender_goods='', sf_biz_type='标快'
		WHERE id=1;
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('无需生产',34,true)
		ON CONFLICT(name) DO NOTHING;
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES
			(30, 'SO-STOCK-READY', '2026-05-03', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='无需生产' LIMIT 1), 176, false),
			(32, 'SO-BATCH-READY', '2026-05-03', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='库存待发货' LIMIT 1), 88, false),
			(31, 'SO-STILL-PENDING', '2026-05-03', 3, 1, 2, 1, 1, 88, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES
			(30, 1, 7, '库存熟豆', 2, '件', '454g', 88, 176),
			(32, 1, 7, '批次熟豆', 1, '件', '454g', 88, 88),
			(31, 1, 7, '待生产熟豆', 1, '件', '454g', 88, 88);
	`, schema, schema, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/orders?ship_ready=1&limit=50", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders ship_ready status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "SO-STOCK-READY") {
		t.Fatalf("ship_ready list should include no-production order: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "SO-BATCH-READY") {
		t.Fatalf("ship_ready list should include stock-batch-ready order: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SO-STILL-PENDING") {
		t.Fatalf("ship_ready list should exclude pending order: %s", rec.Body.String())
	}

	body, _ := json.Marshal(map[string]any{"order_ids": []int64{30, 32}})
	req = httptest.NewRequest(http.MethodPost, "/api/orders/shipping-excel", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/shipping-excel no-production status=%d body=%s", rec.Code, rec.Body.String())
	}
	if files, err := os.ReadDir(exportDir); err != nil || len(files) != 1 {
		t.Fatalf("export files err=%v count=%d, want 1", err, len(files))
	}
}

func TestOrdersShippingExcelAPIUsesSelectedSenderProfile(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	exportDir := filepath.Join(dir, "exports")
	writeOrderShippingTemplateForTest(t, templatePath)
	t.Setenv("ORDER_SHIP_TEMPLATE", templatePath)
	t.Setenv("ORDER_SHIP_EXPORT_DIR", exportDir)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.sender_settings(id, sender_label, sender_name, sender_phone, sender_addr, sender_company, sender_goods, sf_biz_type, is_default, active)
		VALUES
			(2, '默认仓库', '默认寄件人', '13900000000', '默认地址', '默认公司', '茶叶', '标快', true, true),
			(3, '门店', '门店寄件人', '13900000003', '门店地址', '门店公司', '茶叶', '特快', false, true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('生产完成',20,true)
		ON CONFLICT(name) DO UPDATE SET sort=excluded.sort, active=excluded.active;
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (22, 'SO-SENDER-SELECTED', '2026-04-27', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='生产完成' LIMIT 1), 88, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES (22, 1, 7, '橘皮乌龙', 1, '件', '454g', 88, 88);
	`, schema, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{"order_ids": []int64{22}, "sender_id": int64(3)})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-excel", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/shipping-excel status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	files, err := os.ReadDir(exportDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("export files = %d, want 1", len(files))
	}
	wb, err := excelize.OpenFile(filepath.Join(exportDir, files[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	defer wb.Close()
	sheet := wb.GetSheetName(0)
	cell := func(name string) string {
		v, err := wb.GetCellValue(sheet, name)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if cell("D2") != "门店寄件人" || cell("E2") != "13900000003" || cell("F2") != "门店地址" || cell("O2") != "门店公司" || cell("P2") != "特快" {
		t.Fatalf("selected sender cells D2=%q E2=%q F2=%q O2=%q P2=%q", cell("D2"), cell("E2"), cell("F2"), cell("O2"), cell("P2"))
	}
}

func TestOrdersShippingExcelAPIUsesPerOrderSenderOverrides(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	exportDir := filepath.Join(dir, "exports")
	writeOrderShippingTemplateForTest(t, templatePath)
	t.Setenv("ORDER_SHIP_TEMPLATE", templatePath)
	t.Setenv("ORDER_SHIP_EXPORT_DIR", exportDir)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.sender_settings(id, sender_label, sender_name, sender_phone, sender_addr, sender_company, sender_goods, sf_biz_type, is_default, active)
		VALUES
			(2, '默认仓库', '默认寄件人', '13900000000', '默认地址', '默认公司', '茶叶', '标快', true, true),
			(3, '门店', '门店寄件人', '13900000003', '门店地址', '门店公司', '茶叶', '特快', false, true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('生产完成',20,true)
		ON CONFLICT(name) DO UPDATE SET sort=excluded.sort, active=excluded.active;
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES
			(23, 'SO-SENDER-DEFAULT', '2026-04-27', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='生产完成' LIMIT 1), 88, false),
			(24, 'SO-SENDER-OVERRIDE', '2026-04-27', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='生产完成' LIMIT 1), 88, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES
			(23, 1, 7, '橘皮乌龙', 1, '件', '454g', 88, 88),
			(24, 1, 7, '橘皮乌龙', 1, '件', '454g', 88, 88);
	`, schema, schema, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{
		"order_ids": []int64{23, 24},
		"sender_id": int64(2),
		"order_senders": []map[string]any{
			{"order_id": int64(24), "sender_id": int64(3)},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-excel", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/shipping-excel status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	files, err := os.ReadDir(exportDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("export files = %d, want 1", len(files))
	}
	wb, err := excelize.OpenFile(filepath.Join(exportDir, files[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	defer wb.Close()
	sheet := wb.GetSheetName(0)
	cell := func(name string) string {
		v, err := wb.GetCellValue(sheet, name)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if cell("D2") != "默认寄件人" || cell("D3") != "门店寄件人" || cell("P2") != "标快" || cell("P3") != "特快" {
		t.Fatalf("sender rows D2=%q D3=%q P2=%q P3=%q", cell("D2"), cell("D3"), cell("P2"), cell("P3"))
	}
}

func TestOrdersShippingExcelAPICreatesShipmentRecord(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	exportDir := filepath.Join(dir, "exports")
	writeOrderShippingTemplateForTest(t, templatePath)
	t.Setenv("ORDER_SHIP_TEMPLATE", templatePath)
	t.Setenv("ORDER_SHIP_EXPORT_DIR", exportDir)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.sender_settings SET is_default=false WHERE is_default=true;
		INSERT INTO %s.sender_settings(id, sender_label, sender_name, sender_phone, sender_addr, sender_company, sender_goods, sf_biz_type, is_default, active)
		VALUES (4, '仓库', '仓库寄件人', '13900000004', '仓库地址', '仓库公司', '茶叶', '标快', true, true);
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (25, 'SO-SHIPMENT-CREATE', '2026-04-28', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='生产完成' LIMIT 1), 88, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES (25, 1, 7, '橘皮乌龙', 1, '件', '454g', 88, 88);
	`, schema, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{"order_ids": []int64{25}, "sender_id": int64(4)})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-excel", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/shipping-excel status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ShippingExcelURL string `json:"shipping_excel_url"`
		ShipmentID       int64  `json:"shipment_id"`
		ShipmentNo       string `json:"shipment_no"`
		Error            string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != "" || resp.ShippingExcelURL == "" || resp.ShipmentID <= 0 || resp.ShipmentNo == "" {
		t.Fatalf("shipment response = %+v body=%s", resp, rec.Body.String())
	}

	var shipmentNo, fileURL, status string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT shipment_no, file_url, status
		FROM %s.order_shipments
		WHERE id=$1
	`, schema), resp.ShipmentID).Scan(&shipmentNo, &fileURL, &status); err != nil {
		t.Fatalf("query shipment: %v", err)
	}
	if shipmentNo != resp.ShipmentNo || fileURL != resp.ShippingExcelURL || status != "excel_generated" {
		t.Fatalf("shipment row no=%q file=%q status=%q resp=%+v", shipmentNo, fileURL, status, resp)
	}
	var linkedSender int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT sender_id
		FROM %s.order_shipment_orders
		WHERE shipment_id=$1 AND order_id=25
	`, schema), resp.ShipmentID).Scan(&linkedSender); err != nil {
		t.Fatalf("query shipment order: %v", err)
	}
	if linkedSender != 4 {
		t.Fatalf("shipment order sender_id=%d, want 4", linkedSender)
	}
}

func TestOrdersShippingTrackingAPIMarksOrdersShipped(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (26, 'SO-SHIPMENT-TRACK', '2026-04-28', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='生产完成' LIMIT 1), 88, false);
		INSERT INTO %s.order_shipments(id, shipment_no, created_by, sender_id, file_url, status)
		VALUES (11, 'SHIP-20260428-0001', '测试员', 1, '/ship/order_exports/test.xlsx', 'excel_generated');
		INSERT INTO %s.order_shipment_orders(shipment_id, order_id, sender_id)
		VALUES (11, 26, 1);
	`, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{
		"shipment_id": int64(11),
		"items": []map[string]any{
			{"order_id": int64(26), "tracking_no": "SF123456789CN"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-tracking", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/shipping-tracking status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"updated":1`) {
		t.Fatalf("tracking response should include updated=1: %s", rec.Body.String())
	}

	var trackingNo, shipStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(o.ship_tracking_no,''), COALESCE(ss.name,'')
		FROM %s.orders o
		LEFT JOIN %s.ship_statuses ss ON ss.id=o.ship_status_id
		WHERE o.id=26
	`, schema, schema)).Scan(&trackingNo, &shipStatus); err != nil {
		t.Fatalf("query order tracking: %v", err)
	}
	if trackingNo != "SF123456789CN" || shipStatus != "已发货" {
		t.Fatalf("order tracking=%q ship_status=%q, want shipped", trackingNo, shipStatus)
	}

	var rowTracking, shipmentStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(so.tracking_no,''), s.status
		FROM %s.order_shipment_orders so
		JOIN %s.order_shipments s ON s.id=so.shipment_id
		WHERE so.shipment_id=11 AND so.order_id=26
	`, schema, schema)).Scan(&rowTracking, &shipmentStatus); err != nil {
		t.Fatalf("query shipment tracking: %v", err)
	}
	if rowTracking != "SF123456789CN" || shipmentStatus != "shipped" {
		t.Fatalf("shipment tracking=%q status=%q, want shipped", rowTracking, shipmentStatus)
	}
}

func TestOrdersShippingTrackingAPIDeductsReservedLegacyFinishedInventoryOnce(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedOrderAPILegacyFinishedInventory(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.ship_statuses(id,name) VALUES (2,'已发货') ON CONFLICT DO NOTHING;
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (30, 'SO-LEGACY-STOCK-SHIP', '2026-05-03', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='库存待发货' LIMIT 1), 100, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES (30,1,7,'橘皮乌龙',2,'袋','454g',50,100);
		INSERT INTO %s.order_stock_decisions(order_id,decision,operator) VALUES (30,'use_batch','录单员');
		INSERT INTO %s.order_stock_batch_allocations(order_id,product_id,spec_g,need_g,batch_id,batch_code,allocated_g,operator)
		VALUES (30,7,454,908,0,'LEGACY-FP-7-454',908,'录单员');
		INSERT INTO %s.order_shipments(id, shipment_no, created_by, sender_id, file_url, status)
		VALUES (15, 'SHIP-20260503-LEGACY', '测试员', 1, '/ship/order_exports/test.xlsx', 'excel_generated');
		INSERT INTO %s.order_shipment_orders(shipment_id, order_id, sender_id)
		VALUES (15, 30, 1);
	`, schema, schema, schema, schema, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{
		"shipment_id": int64(15),
		"items": []map[string]any{
			{"order_id": int64(30), "tracking_no": "SF-LEGACY-001"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-tracking", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/shipping-tracking status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var units, looseG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g
		FROM %s.finished_inventory
		WHERE product_id=7 AND spec_g=454 AND warehouse='finished_goods'
	`, schema)).Scan(&units, &looseG); err != nil {
		t.Fatalf("query finished inventory: %v", err)
	}
	if units != 2 || looseG != 0 {
		t.Fatalf("finished inventory after shipment = %d units + %dg, want 2 + 0g", units, looseG)
	}
	var deductionCount, ledgerCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.order_stock_deductions WHERE order_id=30 AND batch_code='LEGACY-FP-7-454' AND deducted_g=908`, schema)).Scan(&deductionCount); err != nil {
		t.Fatalf("query order stock deductions: %v", err)
	}
	if deductionCount != 1 {
		t.Fatalf("deduction rows = %d, want 1", deductionCount)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s.stock_ledger_entries
		WHERE item_type='finished_product'
		  AND item_id=7
		  AND spec_g=454
		  AND source_doc_type='sales_order_shipment'
		  AND source_doc_id=30
		  AND source_batch_code='LEGACY-FP-7-454'
		  AND qty_change_g=-908
	`, schema)).Scan(&ledgerCount); err != nil {
		t.Fatalf("query stock ledger entries: %v", err)
	}
	if ledgerCount != 1 {
		t.Fatalf("stock ledger rows = %d, want 1", ledgerCount)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/orders/shipping-tracking", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("second POST /api/orders/shipping-tracking status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g
		FROM %s.finished_inventory
		WHERE product_id=7 AND spec_g=454 AND warehouse='finished_goods'
	`, schema)).Scan(&units, &looseG); err != nil {
		t.Fatalf("query finished inventory after second post: %v", err)
	}
	if units != 2 || looseG != 0 {
		t.Fatalf("finished inventory after duplicate shipment update = %d units + %dg, want 2 + 0g", units, looseG)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.order_stock_deductions WHERE order_id=30`, schema)).Scan(&deductionCount); err != nil {
		t.Fatalf("query order stock deductions after second post: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='sales_order_shipment' AND source_doc_id=30`, schema)).Scan(&ledgerCount); err != nil {
		t.Fatalf("query stock ledger entries after second post: %v", err)
	}
	if deductionCount != 1 || ledgerCount != 1 {
		t.Fatalf("duplicate shipment update should not double deduct: deductions=%d ledger=%d", deductionCount, ledgerCount)
	}
}

func TestOrdersShippingTrackingAPIDeductsOrderSourceWarehouseWithoutAllocation(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.ship_statuses(id,name) VALUES (2,'已发货') ON CONFLICT DO NOTHING;
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void, source_warehouse, portal_service_code)
		VALUES (32, 'SO-CUSTOMER-WH-SHIP', '2026-05-04', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='无需生产' LIMIT 1), 100, false, 'cust_147_processing', 'processing_ship');
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES (32,1,7,'橘皮乌龙',2,'袋','454g',50,100);
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES (7,454,'cust_147_processing',3,0), (7,454,'finished_goods',9,0);
		INSERT INTO %s.order_shipments(id, shipment_no, created_by, sender_id, file_url, status)
		VALUES (16, 'SHIP-20260504-CUSTOMER', '测试员', 1, '/ship/order_exports/test.xlsx', 'excel_generated');
		INSERT INTO %s.order_shipment_orders(shipment_id, order_id, sender_id)
		VALUES (16, 32, 1);
	`, schema, schema, schema, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{
		"shipment_id": int64(16),
		"items": []map[string]any{
			{"order_id": int64(32), "tracking_no": "SF-CUSTOMER-001"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-tracking", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/shipping-tracking status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var customerUnits, publicUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=7 AND spec_g=454 AND warehouse='cust_147_processing'`, schema)).Scan(&customerUnits); err != nil {
		t.Fatalf("query customer warehouse inventory: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=7 AND spec_g=454 AND warehouse='finished_goods'`, schema)).Scan(&publicUnits); err != nil {
		t.Fatalf("query public warehouse inventory: %v", err)
	}
	if customerUnits != 1 || publicUnits != 9 {
		t.Fatalf("inventory after shipment customer=%d public=%d, want customer=1 public=9", customerUnits, publicUnits)
	}
	var deductionCount, ledgerCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.order_stock_deductions WHERE order_id=32 AND batch_code='SOURCE-WH:cust_147_processing' AND deducted_g=908`, schema)).Scan(&deductionCount); err != nil {
		t.Fatalf("query source warehouse deductions: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='sales_order_shipment' AND source_doc_id=32 AND warehouse='cust_147_processing' AND qty_change_g=-908`, schema)).Scan(&ledgerCount); err != nil {
		t.Fatalf("query source warehouse ledger: %v", err)
	}
	if deductionCount != 1 || ledgerCount != 1 {
		t.Fatalf("source warehouse deduction=%d ledger=%d, want 1/1", deductionCount, ledgerCount)
	}
}

func TestOrdersSingleShippingTrackingAPIDeductsReservedFinishedBatch(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedOrderAPIFinishedBatches(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.ship_statuses(id,name) VALUES (2,'已发货') ON CONFLICT DO NOTHING;
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (31, 'SO-FP-STOCK-SHIP', '2026-05-03', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='库存待发货' LIMIT 1), 100, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES (31,1,7,'橘皮乌龙',2,'袋','454g',50,100);
		INSERT INTO %s.order_stock_decisions(order_id,decision,operator) VALUES (31,'use_batch','录单员');
		INSERT INTO %s.order_stock_batch_allocations(order_id,product_id,spec_g,need_g,batch_id,batch_code,allocated_g,operator)
		VALUES (31,7,454,908,101,'FP-OLD-454',908,'录单员');
		INSERT INTO %s.order_shipments(id, shipment_no, created_by, sender_id, file_url, status)
		VALUES (16, 'SHIP-20260503-FP', '测试员', 1, '/ship/order_exports/test.xlsx', 'excel_generated');
		INSERT INTO %s.order_shipment_orders(shipment_id, order_id, sender_id)
		VALUES (16, 31, 1);
	`, schema, schema, schema, schema, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{"tracking_no": "SF-FP-001"})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/31/shipping-tracking", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/31/shipping-tracking status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var remainingG, remainingUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g,remaining_units FROM %s.stock_batches WHERE id=101`, schema)).Scan(&remainingG, &remainingUnits); err != nil {
		t.Fatalf("query finished stock batch: %v", err)
	}
	if remainingG != 0 || remainingUnits != 0 {
		t.Fatalf("finished batch remaining = %dg/%d units, want 0/0", remainingG, remainingUnits)
	}
	var ledgerCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s.stock_ledger_entries
		WHERE item_type='finished_product'
		  AND item_id=7
		  AND spec_g=454
		  AND source_doc_type='sales_order_shipment'
		  AND source_doc_id=31
		  AND source_batch_code='FP-OLD-454'
		  AND qty_change_g=-908
	`, schema)).Scan(&ledgerCount); err != nil {
		t.Fatalf("query stock ledger entries: %v", err)
	}
	if ledgerCount != 1 {
		t.Fatalf("stock ledger rows = %d, want 1", ledgerCount)
	}
}

func TestOrdersSingleShippingTrackingAPIMarksOrderShipped(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (28, 'SO-DRAWER-TRACK', '2026-05-03', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='生产完成' LIMIT 1), 88, false);
		INSERT INTO %s.order_shipments(id, shipment_no, created_by, sender_id, file_url, status)
		VALUES (13, 'SHIP-20260503-0001', '测试员', 1, '/ship/order_exports/test.xlsx', 'excel_generated');
		INSERT INTO %s.order_shipment_orders(shipment_id, order_id, sender_id)
		VALUES (13, 28, 1);
	`, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{"tracking_no": "SF-DRAWER-001"})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/28/shipping-tracking", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/28/shipping-tracking status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"updated":1`) || !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Fatalf("single tracking response should include updated=1 total=1: %s", rec.Body.String())
	}

	var trackingNo, shipStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(o.ship_tracking_no,''), COALESCE(ss.name,'')
		FROM %s.orders o
		LEFT JOIN %s.ship_statuses ss ON ss.id=o.ship_status_id
		WHERE o.id=28
	`, schema, schema)).Scan(&trackingNo, &shipStatus); err != nil {
		t.Fatalf("query order tracking: %v", err)
	}
	if trackingNo != "SF-DRAWER-001" || shipStatus != "已发货" {
		t.Fatalf("order tracking=%q ship_status=%q, want shipped", trackingNo, shipStatus)
	}

	var rowTracking, shipmentStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(so.tracking_no,''), s.status
		FROM %s.order_shipment_orders so
		JOIN %s.order_shipments s ON s.id=so.shipment_id
		WHERE so.shipment_id=13 AND so.order_id=28
	`, schema, schema)).Scan(&rowTracking, &shipmentStatus); err != nil {
		t.Fatalf("query shipment tracking: %v", err)
	}
	if rowTracking != "SF-DRAWER-001" || shipmentStatus != "shipped" {
		t.Fatalf("shipment tracking=%q status=%q, want shipped", rowTracking, shipmentStatus)
	}
}

func TestOrdersSingleShippingTrackingAPIPreservesMultipleNumbers(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, ship_tracking_no, is_void)
		VALUES (34, 'SO-MULTI-TRACK', '2026-05-09', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='生产完成' LIMIT 1), 88, 'SF-OLD-001', false);
		INSERT INTO %s.order_shipments(id, shipment_no, created_by, sender_id, file_url, status)
		VALUES (15, 'SHIP-20260509-0001', '测试员', 1, '/ship/order_exports/test.xlsx', 'excel_generated');
		INSERT INTO %s.order_shipment_orders(shipment_id, order_id, sender_id, tracking_no)
		VALUES (15, 34, 1, 'SF-OLD-001');
	`, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{"tracking_no": "SF-NEW-001，SF-NEW-002"})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/34/shipping-tracking", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/34/shipping-tracking status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var trackingNo, shipStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(o.ship_tracking_no,''), COALESCE(ss.name,'')
		FROM %s.orders o
		LEFT JOIN %s.ship_statuses ss ON ss.id=o.ship_status_id
		WHERE o.id=34
	`, schema, schema)).Scan(&trackingNo, &shipStatus); err != nil {
		t.Fatalf("query order tracking: %v", err)
	}
	want := "SF-OLD-001\nSF-NEW-001\nSF-NEW-002"
	if trackingNo != want || shipStatus != "已发货" {
		t.Fatalf("order tracking=%q ship_status=%q, want %q/已发货", trackingNo, shipStatus, want)
	}
	var count int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.order_shipping_trackings WHERE order_id=34`, schema)).Scan(&count); err != nil {
		t.Fatalf("query normalized tracking rows: %v", err)
	}
	if count != 3 {
		t.Fatalf("normalized tracking rows=%d want 3", count)
	}
}

func TestOrdersListIncludesLatestShipmentSender(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.sender_settings(id, sender_label, sender_name, sender_phone, sender_addr, sender_company, sender_goods, sf_biz_type, is_default, active)
		VALUES (4, '仓库', '王小二', '13900000000', '普洱仓库', '棵凡咖啡', '茶叶', '顺丰标快', false, true);
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, ship_tracking_no, is_void)
		VALUES (29, 'SO-SENDER-LIST', '2026-05-03', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='生产完成' LIMIT 1), 88, 'SF-SENDER-001', false);
		INSERT INTO %s.order_shipments(id, shipment_no, created_by, sender_id, file_url, status)
		VALUES (14, 'SHIP-20260503-0002', '测试员', 1, '/ship/order_exports/test.xlsx', 'excel_generated');
		INSERT INTO %s.order_shipment_orders(shipment_id, order_id, sender_id, tracking_no)
		VALUES (14, 29, 4, 'SF-SENDER-001');
	`, schema, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/orders?limit=10", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"order_no":"SO-SENDER-LIST"`, `"sender_id":4`, `"sender_label":"仓库"`, `"ship_tracking_no":"SF-SENDER-001"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("orders list missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestOrdersShippingTrackingExcelAPIMarksOrdersByRemarkOrderNo(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (27, 'SO-20260428-0001', '2026-04-28', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='生产完成' LIMIT 1), 88, false);
		INSERT INTO %s.order_shipments(id, shipment_no, created_by, sender_id, file_url, status)
		VALUES (12, 'SHIP-20260428-0002', '测试员', 1, '/ship/order_exports/test.xlsx', 'excel_generated');
		INSERT INTO %s.order_shipment_orders(shipment_id, order_id, sender_id)
		VALUES (12, 27, 1);
	`, schema, schema, schema, schema))

	wb := excelize.NewFile()
	sheet := wb.GetSheetName(0)
	for i, header := range []string{"运单号", "备注"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := wb.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	if err := wb.SetCellValue(sheet, "A2", "SF5199040648127"); err != nil {
		t.Fatal(err)
	}
	if err := wb.SetCellValue(sheet, "B2", "SO-20260428-0001；橘皮乌龙 227g x1件"); err != nil {
		t.Fatal(err)
	}
	var fileBytes bytes.Buffer
	if err := wb.Write(&fileBytes); err != nil {
		t.Fatal(err)
	}
	if err := wb.Close(); err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "tracking.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(fileBytes.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-tracking-excel", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/shipping-tracking-excel status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"updated":1`) || !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Fatalf("tracking excel response should include updated=1 total=1: %s", rec.Body.String())
	}

	var trackingNo, shipStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(o.ship_tracking_no,''), COALESCE(ss.name,'')
		FROM %s.orders o
		LEFT JOIN %s.ship_statuses ss ON ss.id=o.ship_status_id
		WHERE o.id=27
	`, schema, schema)).Scan(&trackingNo, &shipStatus); err != nil {
		t.Fatalf("query order tracking: %v", err)
	}
	if trackingNo != "SF5199040648127" || shipStatus != "已发货" {
		t.Fatalf("order tracking=%q ship_status=%q, want shipped", trackingNo, shipStatus)
	}
}

func TestOrdersShippingTrackingExcelAPIRejectsOversizedUpload(t *testing.T) {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			c.Set("actor", "测试员")
			return next(c)
		}
	})
	registerOrderShippingExcelRoutes(e, salesapp.NewService(nil), nil)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "tracking.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), (20<<20)+1)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-tracking-excel", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "file too large") {
		t.Fatalf("oversized tracking excel status=%d body=%s, want file too large", rec.Code, rec.Body.String())
	}
}

func TestLegacyShippingTrackingFillRejectsOversizedUpload(t *testing.T) {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			c.Set("actor", "测试员")
			return next(c)
		}
	})
	registerShipExportRoutes(e, salesapp.NewService(nil))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "tracking.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), (20<<20)+1)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/ship/tracking_fill", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "file too large") {
		t.Fatalf("legacy oversized tracking upload status=%d body=%s, want file too large", rec.Code, rec.Body.String())
	}
}

func TestOrdersShippingExcelAPIRejectsUnfinishedOrders(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	exportDir := filepath.Join(dir, "exports")
	writeOrderShippingTemplateForTest(t, templatePath)
	t.Setenv("ORDER_SHIP_TEMPLATE", templatePath)
	t.Setenv("ORDER_SHIP_EXPORT_DIR", exportDir)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (21, 'SO-NOT-FINISHED', '2026-04-27', 3, 1, 2, 1, 1, 88, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES (21, 1, 7, '橘皮乌龙', 1, '件', '454g', 88, 88);
	`, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{"order_ids": []int64{21}})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/shipping-excel", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/orders/shipping-excel status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "尚不可发货") {
		t.Fatalf("unfinished order error body=%s", rec.Body.String())
	}
	files, err := os.ReadDir(exportDir)
	if err == nil && len(files) > 0 {
		t.Fatalf("unfinished order generated shipping exports = %d, want 0", len(files))
	}
}

func TestSenderSettingsAPIListsProfilesWithDefault(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.sender_settings(id, sender_label, sender_name, sender_phone, sender_addr, sender_company, sender_goods, sf_biz_type, is_default, active)
		VALUES (2, '仓库', '仓库寄件人', '13900000002', '仓库地址', '仓库公司', '茶叶', '标快', true, true);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/settings/sender", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/settings/sender status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"profiles"`, `"sender_label":"仓库"`, `"is_default":true`, `"profile"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("sender settings response missing %s: %s", needle, body)
		}
	}
}

func newOrderAPITestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for order API tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_order_api_%d", time.Now().UnixNano())
	mustExecOrderAPITestSQL(t, ctx, pool, "CREATE SCHEMA "+schema)
	mustExecOrderAPITestSQL(t, ctx, pool, orderAPITestDDL(schema))
	mustExecOrderAPITestSQL(t, ctx, pool, orderAPITrackingDDL(schema))
	if err := support.EnsureAuditTables(ctx, pool, schema); err != nil {
		t.Fatalf("ensureAuditTables: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	return pool, schema
}

func newOrderAPITestEcho(pool *pgxpool.Pool, schema string) *echo.Echo {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			c.Set("actor", "测试员")
			return next(c)
		}
	})
	svc := salesapp.NewService(postgressales.NewRepository(pool, schema))
	registerOrderAPI(e, svc, nil)
	registerOrderShippingExcelRoutes(e, svc, nil)
	registerSenderSettingsPage(e, svc)
	return e
}

func seedOrderAPITestData(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.customers(id,name,contact,phone,address,active,default_source_id,default_order_type_id) VALUES (3,'测试客户','测试收件人','13800000000','杭州市测试路',true,1,2);
		INSERT INTO %s.sources(id,name) VALUES (1,'小程序');
		INSERT INTO %s.order_types(id,name) VALUES (1,'批发订单'),(2,'零售订单');
		INSERT INTO %s.pay_statuses(id,name) VALUES (1,'未付款'),(2,'已付款');
		INSERT INTO %s.ship_statuses(id,name) VALUES (1,'未发货');
		INSERT INTO %s.order_process_statuses(id,name,sort,active) VALUES (1,'待处理',10,true),(2,'库存待发货',33,true);
		SELECT setval(pg_get_serial_sequence('%s.order_process_statuses','id'), (SELECT COALESCE(MAX(id),1) FROM %s.order_process_statuses));
		INSERT INTO %s.products(id,name,default_price,active,retail_price_227g,retail_price_250g)
		VALUES (7,'橘皮乌龙',50,true,50,56);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema))
}

func seedOrderAPIResponsibleData(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.company_departments(id,name,active) VALUES (1,'销售',true);
		INSERT INTO %s.company_employees(id,name,phone,department_id,active) VALUES (5,'销售小王','13800000005',1,true);
		INSERT INTO %s.customers(id,name,contact,phone,address,active) VALUES (4,'渠道伙伴A','代理老张','13800000004','上海市渠道路',true);
	`, schema, schema, schema))
}

func seedOrderAPIFinishedBatches(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(id,batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,quality_status,operator,created_at)
		VALUES
			(101,'FP-OLD-454','finished_product',7,'橘皮乌龙',454,'production_run',501,'PB-OLD',908,2,908,2,'pass','烘焙员','2026-05-01 08:00:00+08'),
			(102,'FP-NEW-454','finished_product',7,'橘皮乌龙',454,'production_run',502,'PB-NEW',908,2,908,2,'pass','烘焙员','2026-05-02 08:00:00+08');
		INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,source_batch_id,qty_before_g,qty_change_g,qty_after_g,qty_before_units,qty_change_units,qty_after_units,operator,created_at)
		VALUES
			('finished_product',7,'橘皮乌龙',454,'finished_goods','production_run',501,'FP-OLD-454','PB-OLD',0,908,908,0,2,2,'烘焙员','2026-05-01 08:00:00+08'),
			('finished_product',7,'橘皮乌龙',454,'finished_goods','production_run',502,'FP-NEW-454','PB-NEW',908,908,1816,2,2,4,'烘焙员','2026-05-02 08:00:00+08');
	`, schema, schema))
}

func seedOrderAPILegacyFinishedInventory(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES (7,454,'finished_goods',4,0);
	`, schema))
}

func mustExecOrderAPITestSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec sql: %v\n%s", err, sql)
	}
}

func orderAPITestDDL(schema string) string {
	return fmt.Sprintf(`
CREATE TABLE %s.customers (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	company_name TEXT NOT NULL DEFAULT '',
	company_address TEXT NOT NULL DEFAULT '',
	company_phone TEXT NOT NULL DEFAULT '',
	contact TEXT NOT NULL DEFAULT '',
	phone TEXT NOT NULL DEFAULT '',
	address TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	default_source_id BIGINT,
	default_order_type_id BIGINT
);
CREATE TABLE %s.company_profile (
	id INTEGER PRIMARY KEY DEFAULT 1,
	company_name TEXT NOT NULL DEFAULT '',
	company_address TEXT NOT NULL DEFAULT '',
	company_phone TEXT NOT NULL DEFAULT '',
	taxpayer_id TEXT NOT NULL DEFAULT '',
	bank_account_name TEXT NOT NULL DEFAULT '',
	bank_name TEXT NOT NULL DEFAULT '',
	bank_account_no TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT '',
	CONSTRAINT company_profile_singleton CHECK (id = 1)
);
CREATE TABLE %s.company_departments (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.company_employees (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	phone TEXT NOT NULL DEFAULT '',
	department_id BIGINT NOT NULL REFERENCES %s.company_departments(id),
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.sources (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.order_types (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.pay_statuses (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.ship_statuses (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.order_process_statuses (
	id SERIAL PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	sort INTEGER NOT NULL DEFAULT 0,
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %s.stock_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	item_type TEXT NOT NULL,
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_id TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	remaining_g BIGINT NOT NULL DEFAULT 0,
	remaining_units BIGINT NOT NULL DEFAULT 0,
	unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	quality_status TEXT NOT NULL DEFAULT 'unchecked',
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.stock_ledger_entries (
	id BIGSERIAL PRIMARY KEY,
	item_type TEXT NOT NULL,
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	warehouse TEXT NOT NULL DEFAULT 'finished_goods',
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_code TEXT NOT NULL DEFAULT '',
	source_batch_id TEXT NOT NULL DEFAULT '',
	qty_before_g BIGINT NOT NULL DEFAULT 0,
	qty_change_g BIGINT NOT NULL DEFAULT 0,
	qty_after_g BIGINT NOT NULL DEFAULT 0,
	qty_before_units BIGINT NOT NULL DEFAULT 0,
	qty_change_units BIGINT NOT NULL DEFAULT 0,
	qty_after_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	warehouse TEXT NOT NULL DEFAULT 'finished_goods',
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g, warehouse)
);
CREATE TABLE %s.products (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	roast_level TEXT NOT NULL DEFAULT '',
	default_price NUMERIC NOT NULL DEFAULT 0,
	product_kind TEXT NOT NULL DEFAULT 'roasted',
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	retail_price_100g NUMERIC NOT NULL DEFAULT 0,
	retail_price_200g NUMERIC NOT NULL DEFAULT 0,
	retail_price_227g NUMERIC NOT NULL DEFAULT 0,
	retail_price_250g NUMERIC NOT NULL DEFAULT 0,
	customer_id BIGINT NOT NULL DEFAULT 0,
	base_product_id BIGINT NOT NULL DEFAULT 0,
	visibility TEXT NOT NULL DEFAULT 'public',
	custom_type TEXT NOT NULL DEFAULT '',
	product_kind TEXT NOT NULL DEFAULT 'roasted_bean',
	drip_bag_grams NUMERIC(12,3) NOT NULL DEFAULT 10,
	drip_box_bag_count INT NOT NULL DEFAULT 10
);
CREATE TABLE %s.product_price_tiers (
	id BIGSERIAL PRIMARY KEY,
	product_id BIGINT,
	spec_g INTEGER NOT NULL DEFAULT 454,
	min_qty_units NUMERIC,
	max_qty_units NUMERIC,
	price_per_unit NUMERIC,
	min_qty_lb NUMERIC,
	max_qty_lb NUMERIC,
	price_per_lb NUMERIC,
	active BOOLEAN NOT NULL DEFAULT true,
	product_kind TEXT NOT NULL DEFAULT 'roasted_bean',
	price_basis TEXT NOT NULL DEFAULT 'weight',
	sales_unit TEXT NOT NULL DEFAULT '',
	unit_bag_count INT NOT NULL DEFAULT 0,
	price_source_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE TABLE %s.customer_service_capabilities (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL,
	capability_code TEXT NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT true,
	config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(customer_id, capability_code)
);
CREATE TABLE %s.bean_list_publications (
	id BIGSERIAL PRIMARY KEY,
	list_type TEXT NOT NULL,
	version_no TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'published',
	owner_type TEXT NOT NULL DEFAULT 'official',
	owner_key TEXT NOT NULL DEFAULT '',
	config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	content_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	changelog TEXT NOT NULL DEFAULT '',
	actor TEXT NOT NULL DEFAULT '',
	published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.orders (
	id BIGSERIAL PRIMARY KEY,
	order_date DATE,
	customer_id BIGINT,
	source_id BIGINT,
	order_type_id BIGINT,
	pay_status_id BIGINT,
	payment_method TEXT NOT NULL DEFAULT '',
	ship_status_id BIGINT,
	ship_method TEXT,
	ship_tracking_no TEXT,
	receiver_name TEXT NOT NULL DEFAULT '',
	receiver_phone TEXT NOT NULL DEFAULT '',
	receiver_address TEXT NOT NULL DEFAULT '',
	receiver_company TEXT NOT NULL DEFAULT '',
	portal_service_code TEXT NOT NULL DEFAULT '',
	source_warehouse TEXT NOT NULL DEFAULT '',
	sender_id BIGINT NOT NULL DEFAULT 0,
	notes TEXT,
	total_amount NUMERIC NOT NULL DEFAULT 0,
	shipping_amount NUMERIC NOT NULL DEFAULT 0,
	discount_amount NUMERIC NOT NULL DEFAULT 0,
	round_to_int BOOLEAN NOT NULL DEFAULT false,
	rounding_amount NUMERIC NOT NULL DEFAULT 0,
	grand_total NUMERIC NOT NULL DEFAULT 0,
	express_fee TEXT,
	outsource_material_fee NUMERIC NOT NULL DEFAULT 0,
	outsource_roast_fee NUMERIC NOT NULL DEFAULT 0,
	outsource_packaging_fee NUMERIC NOT NULL DEFAULT 0,
	outsource_manual_fee NUMERIC NOT NULL DEFAULT 0,
	outsource_tax_fee NUMERIC NOT NULL DEFAULT 0,
	outsource_other_fee NUMERIC NOT NULL DEFAULT 0,
	outsource_total_fee NUMERIC NOT NULL DEFAULT 0,
	order_no TEXT,
	is_void BOOLEAN NOT NULL DEFAULT false,
	voided_at TIMESTAMPTZ,
	void_reason TEXT,
	process_status_id INTEGER,
	responsible_party_type TEXT NOT NULL DEFAULT '',
	responsible_party_id BIGINT NOT NULL DEFAULT 0,
	responsible_party_name TEXT NOT NULL DEFAULT '',
	bean_list_publication_id BIGINT NOT NULL DEFAULT 0,
	bean_list_version_no TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.order_items (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT,
	line_no INTEGER,
	product_id BIGINT,
	price_tier_id BIGINT,
	product_kind TEXT NOT NULL DEFAULT 'roasted',
	price_overridden BOOLEAN NOT NULL DEFAULT false,
	item_name TEXT,
	item_note TEXT NOT NULL DEFAULT '',
	qty NUMERIC,
	unit TEXT,
	spec TEXT,
	unit_price NUMERIC NOT NULL DEFAULT 0,
	line_total_before_discount NUMERIC NOT NULL DEFAULT 0,
	discount_type TEXT NOT NULL DEFAULT '',
	discount_value NUMERIC NOT NULL DEFAULT 0,
	discount_amount NUMERIC NOT NULL DEFAULT 0,
	product_kind TEXT NOT NULL DEFAULT 'roasted_bean',
	sales_unit TEXT NOT NULL DEFAULT '',
	unit_bag_count INT NOT NULL DEFAULT 0,
	unit_bean_g NUMERIC(12,3) NOT NULL DEFAULT 0,
	matched_price_qty NUMERIC(14,3) NOT NULL DEFAULT 0,
	price_source_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	line_total NUMERIC NOT NULL DEFAULT 0
);
CREATE TABLE %s.order_stock_decisions (
	order_id BIGINT PRIMARY KEY REFERENCES %s.orders(id) ON DELETE CASCADE,
	decision TEXT NOT NULL DEFAULT '',
	operator TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.order_stock_batch_allocations (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT NOT NULL REFERENCES %s.orders(id) ON DELETE CASCADE,
	product_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,
	need_g BIGINT NOT NULL DEFAULT 0,
	batch_id BIGINT NOT NULL DEFAULT 0,
	batch_code TEXT NOT NULL DEFAULT '',
	allocated_g BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.order_stock_deductions (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT NOT NULL REFERENCES %s.orders(id) ON DELETE CASCADE,
	product_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,
	batch_id BIGINT NOT NULL DEFAULT 0,
	batch_code TEXT NOT NULL DEFAULT '',
	deducted_g BIGINT NOT NULL DEFAULT 0,
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(order_id, product_id, spec_g, batch_code)
);
CREATE TABLE %s.sales_order_assets (
	id BIGSERIAL PRIMARY KEY,
	kind TEXT NOT NULL,
	filename TEXT NOT NULL DEFAULT '',
	content_type TEXT NOT NULL DEFAULT '',
	bytes BIGINT NOT NULL DEFAULT 0,
	sha256 TEXT NOT NULL DEFAULT '',
	object_key TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %s.order_invoices (
	order_id BIGINT PRIMARY KEY REFERENCES %s.orders(id) ON DELETE CASCADE,
	order_no TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'requested',
	requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	requested_by TEXT NOT NULL DEFAULT '',
	invoice_asset_id BIGINT REFERENCES %s.sales_order_assets(id),
	uploaded_at TIMESTAMPTZ,
	uploaded_by TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %s.sender_settings (
	id SMALLINT PRIMARY KEY DEFAULT 1,
	sender_label TEXT NOT NULL DEFAULT '',
	sender_name TEXT NOT NULL DEFAULT '',
	sender_phone TEXT NOT NULL DEFAULT '',
	sender_addr TEXT NOT NULL DEFAULT '',
	sender_company TEXT NOT NULL DEFAULT '',
	sender_goods TEXT NOT NULL DEFAULT '茶叶',
	sf_biz_type TEXT NOT NULL DEFAULT '',
	is_default BOOLEAN NOT NULL DEFAULT false,
	active BOOLEAN NOT NULL DEFAULT true,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.order_shipments (
	id BIGSERIAL PRIMARY KEY,
	shipment_no TEXT NOT NULL UNIQUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	sender_id BIGINT,
	file_url TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'excel_generated'
);
CREATE TABLE %s.order_shipment_orders (
	id BIGSERIAL PRIMARY KEY,
	shipment_id BIGINT NOT NULL REFERENCES %s.order_shipments(id) ON DELETE CASCADE,
	order_id BIGINT NOT NULL REFERENCES %s.orders(id) ON DELETE CASCADE,
	sender_id BIGINT,
	tracking_no TEXT NOT NULL DEFAULT '',
	shipped_at TIMESTAMPTZ,
	UNIQUE(shipment_id, order_id)
);
INSERT INTO %s.sender_settings(id, sender_label, is_default, active) VALUES(1, '默认寄件人', true, true);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema,
		schema, schema, schema, schema, schema, schema, schema, schema, schema, schema,
		schema, schema, schema, schema, schema, schema, schema, schema, schema, schema,
		schema, schema, schema, schema, schema)
}

func orderAPITrackingDDL(schema string) string {
	return fmt.Sprintf(`
CREATE TABLE %s.order_shipping_trackings (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT NOT NULL REFERENCES %s.orders(id) ON DELETE CASCADE,
	tracking_no TEXT NOT NULL,
	source TEXT NOT NULL DEFAULT '',
	created_by TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(order_id, tracking_no)
);
CREATE INDEX order_shipping_trackings_order_idx ON %s.order_shipping_trackings(order_id, id);
CREATE INDEX order_shipping_trackings_no_idx ON %s.order_shipping_trackings(tracking_no);
	`, schema, schema, schema, schema)
}

func writeOrderShippingTemplateForTest(t *testing.T, path string) {
	t.Helper()
	wb := excelize.NewFile()
	sheet := wb.GetSheetName(0)
	headers := []string{"收件人", "收件人手机/电话", "收件地址", "寄件人", "寄件人手机/电话", "寄件地址", "收件公司", "包裹件数", "托寄物", "重量", "长", "宽", "高", "备注(选填)", "寄件公司", "业务类型", "包装服务费"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := wb.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	if err := wb.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	if err := wb.Close(); err != nil {
		t.Fatal(err)
	}
}
