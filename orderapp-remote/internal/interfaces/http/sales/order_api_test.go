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

func TestAPIProductsCarriesTierDisplayUnit(t *testing.T) {
	products := apiProducts([]ProductOption{{
		ID:          414,
		Name:        "兰卡拼配生豆",
		ProductKind: "green_bean",
		Tiers: []ProductTierOption{{
			ID:          50,
			SpecG:       1000,
			MinQty:      1,
			UnitPrice:   23.49,
			DisplayUnit: "kg",
			ProductKind: "green_bean",
		}},
	}})

	tiers, ok := products[0]["tiers"].([]map[string]any)
	if !ok || len(tiers) != 1 {
		t.Fatalf("tiers = %#v, want one tier map", products[0]["tiers"])
	}
	if got := tiers[0]["display_unit"]; got != "kg" {
		t.Fatalf("display_unit = %#v, want kg", got)
	}
}

func TestEditDataForAPIPreservesManualCountPriceSource(t *testing.T) {
	got := editDataForAPI(&OrderEditData{Items: []OrderEditItem{{
		ProductID:       558,
		Product:         "初晓",
		Spec:            "227g",
		Qty:             "2",
		UnitPrice:       "68.00",
		PriceOverride:   true,
		PriceSourceJSON: `{"source":"published_bean_list","publication_id":22,"quantity_basis":"sales_spec_count"}`,
	}}})
	encoded, err := json.Marshal(got["items"])
	if err != nil {
		t.Fatalf("marshal edit items: %v", err)
	}
	var items []struct {
		TierID          string `json:"tier_id"`
		PriceOverride   bool   `json:"price_override"`
		PriceSourceJSON string `json:"price_source_json"`
	}
	if err := json.Unmarshal(encoded, &items); err != nil {
		t.Fatalf("decode edit items: %v", err)
	}
	if len(items) != 1 || items[0].TierID != "manual" || !items[0].PriceOverride {
		t.Fatalf("manual edit item = %+v, want manual price override", items)
	}
	if !strings.Contains(items[0].PriceSourceJSON, `"quantity_basis":"sales_spec_count"`) {
		t.Fatalf("manual edit price source = %q, want sales_spec_count", items[0].PriceSourceJSON)
	}
}

func TestEditDataForAPIClearsAmbiguousCommercialHeaderAndKeepsLinePublications(t *testing.T) {
	got := editDataForAPI(&OrderEditData{
		BeanListPublicationID: 999,
		BeanListVersionNo:     "STALE-HEADER",
		Items: []OrderEditItem{
			{ProductID: 7, Product: "白月光瑰夏", ProductKind: "roasted_bean", BeanListPublicationID: 9951},
			{ProductID: 8, Product: "冻干咖啡", ProductKind: "instant_coffee", BeanListPublicationID: 9952},
		},
	})

	if got["bean_list_publication_id"] != int64(0) || got["commercial_bean_list_publication_id"] != int64(0) {
		t.Fatalf("ambiguous commercial headers = %#v/%#v, want 0/0", got["bean_list_publication_id"], got["commercial_bean_list_publication_id"])
	}
	encoded, err := json.Marshal(got["items"])
	if err != nil {
		t.Fatal(err)
	}
	var items []struct {
		BeanListPublicationID int64 `json:"bean_list_publication_id"`
	}
	if err := json.Unmarshal(encoded, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].BeanListPublicationID != 9951 || items[1].BeanListPublicationID != 9952 {
		t.Fatalf("edit line publications = %+v, want 9951 and 9952", items)
	}
}

func TestEditDataForAPISingleCommercialLineOverridesStaleHeader(t *testing.T) {
	got := editDataForAPI(&OrderEditData{
		BeanListPublicationID: 999,
		BeanListVersionNo:     "STALE-HEADER",
		Items: []OrderEditItem{{
			ProductID:             7,
			Product:               "白月光瑰夏",
			ProductKind:           "roasted_bean",
			BeanListPublicationID: 9951,
			BeanListVersionNo:     "COMM-ITEM",
			PriceSourceJSON:       `{"list_type":"commercial","publication_id":9951}`,
		}},
	})

	if got["bean_list_publication_id"] != int64(9951) || got["commercial_bean_list_publication_id"] != int64(9951) {
		t.Fatalf("single commercial headers = %#v/%#v, want authoritative line publication 9951/9951", got["bean_list_publication_id"], got["commercial_bean_list_publication_id"])
	}
	if got["bean_list_version_no"] != "COMM-ITEM" {
		t.Fatalf("single commercial version = %#v, want authoritative line version COMM-ITEM", got["bean_list_version_no"])
	}
}

func TestEditDataForAPIUsesPriceSourceListTypeBeforeProductKindForPublicationHeaders(t *testing.T) {
	got := editDataForAPI(&OrderEditData{
		BeanListPublicationID: 999,
		BeanListVersionNo:     "STALE-HEADER",
		Items: []OrderEditItem{
			{
				ProductID:             7,
				Product:               "白月光瑰夏",
				ProductKind:           "roasted_bean",
				BeanListPublicationID: 9951,
				PriceSourceJSON:       `{"list_type":"commercial","publication_id":9951}`,
			},
			{
				ProductID:             8,
				Product:               "曲奇挂耳",
				ProductKind:           "drip_bag",
				BeanListPublicationID: 9952,
				PriceSourceJSON:       `{"list_type":"commercial","publication_id":9952}`,
			},
		},
	})

	if got["bean_list_publication_id"] != int64(0) || got["commercial_bean_list_publication_id"] != int64(0) {
		t.Fatalf("ambiguous commercial headers = %#v/%#v, want 0/0", got["bean_list_publication_id"], got["commercial_bean_list_publication_id"])
	}
	encoded, err := json.Marshal(got["items"])
	if err != nil {
		t.Fatal(err)
	}
	var items []struct {
		BeanListPublicationID int64 `json:"bean_list_publication_id"`
	}
	if err := json.Unmarshal(encoded, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].BeanListPublicationID != 9951 || items[1].BeanListPublicationID != 9952 {
		t.Fatalf("edit line publications = %+v, want 9951 and 9952", items)
	}
}

func TestEditDataForAPIFallsBackToProductKindWhenPriceSourceListTypeIsMissing(t *testing.T) {
	got := editDataForAPI(&OrderEditData{Items: []OrderEditItem{
		{ProductID: 8, Product: "曲奇挂耳", ProductKind: "drip_bag", BeanListPublicationID: 9952},
	}})

	if got["drip_bean_list_publication_id"] != int64(9952) {
		t.Fatalf("drip header = %#v, want ProductKind fallback publication 9952", got["drip_bean_list_publication_id"])
	}
	if got["commercial_bean_list_publication_id"] != int64(0) {
		t.Fatalf("commercial header = %#v, want 0", got["commercial_bean_list_publication_id"])
	}
}

func TestAPIProductsCarriesProductTypeAndUnitRule(t *testing.T) {
	products := apiProducts([]ProductOption{{
		ID:                       515,
		Name:                     "冻干美式 20杯盒",
		ProductKind:              "instant_coffee",
		ProductTypeCategoryID:    12,
		ProductSubtypeCategoryID: 13,
		ProductTypeName:          "速溶咖啡",
		ProductSubtypeName:       "冻干速溶",
		InventoryUnit:            "kg",
		QuoteUnit:                "盒",
		OrderUnit:                "盒",
		UnitConversionJSON:       `{"盒":{"kg":0.2}}`,
		IntegerUnit:              true,
	}})

	got := products[0]
	for key, want := range map[string]any{
		"product_type_category_id":    int64(12),
		"product_subtype_category_id": int64(13),
		"product_type_name":           "速溶咖啡",
		"product_subtype_name":        "冻干速溶",
		"inventory_unit":              "kg",
		"quote_unit":                  "盒",
		"order_unit":                  "盒",
		"unit_conversion_json":        `{"盒":{"kg":0.2}}`,
		"integer_unit":                true,
	} {
		if got[key] != want {
			t.Fatalf("%s = %#v, want %#v", key, got[key], want)
		}
	}
}

func TestAPIProductsCarriesCustomerAliasSnapshots(t *testing.T) {
	products := apiProducts([]ProductOption{{
		ID:                               77,
		Name:                             "Karen 贴牌意式",
		ProductKind:                      "roasted_bean",
		CustomerID:                       42,
		Visibility:                       "customer_alias",
		CustomerProductAliasID:           910,
		CustomerProductDisplayName:       "Karen 贴牌意式",
		CustomerItemCode:                 "KAREN-ESP",
		BrandName:                        "",
		ProductCode:                      "SKU-77",
		ProductRecordName:                "精品意式拼配",
		CustomerAliasDisplayCategoryID:   5,
		CustomerAliasDisplayCategoryName: "定制熟豆",
	}})

	got := products[0]
	for key, want := range map[string]any{
		"id":                                   int64(77),
		"name":                                 "Karen 贴牌意式",
		"customer_product_alias_id":            int64(910),
		"customer_product_display_name":        "Karen 贴牌意式",
		"customer_item_code":                   "KAREN-ESP",
		"brand_name":                           "",
		"product_code":                         "SKU-77",
		"product_name_snapshot":                "精品意式拼配",
		"customer_alias_display_category_id":   int64(5),
		"customer_alias_display_category_name": "定制熟豆",
	} {
		if got[key] != want {
			t.Fatalf("%s = %#v, want %#v", key, got[key], want)
		}
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

func TestOrderAPIFormUsesCustomerProductAliasesForCustomerScope(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_product_aliases(
			id, customer_id, product_id, display_name, customer_item_code, brand_name,
			display_category_id, sort_order, include_in_price_list, active
		)
		VALUES
			(910, 3, 7, 'Karen 贴牌意式', 'KAREN-ESP', '', 0, 1, true, true),
			(911, 4, 7, '其他客户贴牌意式', 'OTHER-ESP', '', 0, 1, true, true);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form?customer_id=3", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form?customer_id=3 status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"name":"Karen 贴牌意式"`,
		`"customer_product_alias_id":910`,
		`"customer_item_code":"KAREN-ESP"`,
		`"product_name_snapshot":"橘皮乌龙"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("customer scoped order form missing %s: %s", want, body)
		}
	}
	for _, forbidden := range []string{
		`"name":"橘皮乌龙"`,
		`"name":"其他客户贴牌意式"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("customer scoped order form leaked %s: %s", forbidden, body)
		}
	}
}

func TestOrderAPISaveRejectsCustomerAliasWithoutPublishedPrice(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id, name, default_price, active, visibility, product_kind)
		VALUES (707, '无发布价商品档案', 0, true, 'public', 'roasted_bean');
		INSERT INTO %[1]s.customer_product_aliases(
			id, customer_id, product_id, display_name, customer_item_code, brand_name,
			display_category_id, sort_order, include_in_price_list, active
		)
		VALUES (9707, 3, 707, 'Karen 无发布价商品名', 'KAREN-NOPRICE', '', 0, 1, true, true);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := `{
		"order_date":"2026-05-31",
		"customer_id":3,
		"pay_status_id":1,
		"ship_status_id":1,
		"product_id":["707"],
		"customer_product_alias_id":["9707"],
		"item_name":["Karen 无发布价商品名"],
		"qty":["1"],
		"unit":["件"],
		"spec":["454"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(payload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/order status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "customer product price unpublished") {
		t.Fatalf("POST /api/order error = %s, want unpublished customer product price", rec.Body.String())
	}
}

func TestOrderAPIFormReturnsCustomerDefaultsForOrderEntry(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(id,name,phone,department_id,account_type,active)
		VALUES (44,'外部客户账号','13900000044',1,'channel_customer',true);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"customer_type":"wholesale"`, `"default_source_id":1`, `"default_order_type_id":2`, `"responsible_employee_id":5`, `"responsible_employee_name":"销售小王"`, `"py"`, `"pyi"`, `"销售小王"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/order/form missing %s: %s", needle, body)
		}
	}
	if strings.Contains(body, "外部客户账号") {
		t.Fatalf("GET /api/order/form exposed channel customer account as employee option: %s", body)
	}
}

func TestOrderAPISaveUsesCustomerProfileDefaultsForHiddenHeaderFields(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := `{
		"order_date":"2026-05-23",
		"customer_id":3,
		"pay_status_id":1,
		"ship_status_id":1,
		"product_id":["7"],
		"item_name":["橘皮乌龙"],
		"tier_id":["manual"],
		"unit_price":["88"],
		"qty":["1"],
		"unit":["件"],
		"spec":["454"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(payload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		OrderID int64 `json:"order_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var sourceID, orderTypeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(source_id,0), COALESCE(order_type_id,0) FROM %s.orders WHERE id=$1`, schema), resp.OrderID).Scan(&sourceID, &orderTypeID); err != nil {
		t.Fatalf("query saved order defaults: %v", err)
	}
	if sourceID != 1 || orderTypeID != 2 {
		t.Fatalf("saved header defaults source=%d order_type=%d, want 1/2", sourceID, orderTypeID)
	}
}

func TestOrderAPISaveRejectsNonPositiveManualPrice(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	e := newOrderAPITestEcho(pool, schema)

	for _, price := range []string{"", "abc", "0", "-1"} {
		t.Run(price, func(t *testing.T) {
			payload := fmt.Sprintf(`{
				"order_date":"2026-07-16",
				"customer_id":3,
				"pay_status_id":1,
				"ship_status_id":1,
				"product_id":["7"],
				"item_name":["橘皮乌龙"],
				"tier_id":["manual"],
				"unit_price":[%q],
				"qty":["1"],
				"unit":["件"],
				"spec":["454"]
			}`, price)
			req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(payload))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "手动单价必须大于0") {
				t.Fatalf("POST /api/order manual price %s status/body = %d/%s, want 400 positive-price error", price, rec.Code, rec.Body.String())
			}
		})
	}
	var count int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.orders`, schema)).Scan(&count); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if count != 0 {
		t.Fatalf("non-positive manual prices created %d orders, want 0", count)
	}
}

func TestSaveOrderCommandRejectsMissingInvalidAndNonPositiveManualPrice(t *testing.T) {
	for _, price := range []string{"", "abc", "0", "-1"} {
		t.Run(price, func(t *testing.T) {
			_, err := saveOrderCommandFromCreateRequest(CreateOrderRequest{
				OrderDate: "2026-07-16",
				TierID:    []string{"manual"},
				UnitPrice: []string{price},
			}, 0, "codex")
			if err == nil || !strings.Contains(err.Error(), "手动单价必须大于0") {
				t.Fatalf("manual price %q error = %v, want positive-price rejection", price, err)
			}
		})
	}
}

func TestOrderAPISaveRejectsCustomerMissingRequiredProfileDefaults(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.customers(id,name,customer_type,contact,phone,address,active,responsible_employee_id)
		VALUES (8,'缺资料客户','','缺资料','13800000008','杭州市缺资料路',true,5);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := `{
		"order_date":"2026-05-23",
		"customer_id":8,
		"pay_status_id":1,
		"ship_status_id":1,
		"product_id":["7"],
		"item_name":["橘皮乌龙"],
		"tier_id":["manual"],
		"unit_price":["88"],
		"qty":["1"],
		"unit":["件"],
		"spec":["454"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(payload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/order status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{"客户类型", "来源", "订单类型"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("POST /api/order missing required %s error: %s", want, rec.Body.String())
		}
	}
}

func TestOrderAPIFormReturnsLogisticsSettings(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"logistics_companies"`, `"顺丰"`, `"顺丰小件"`, `"顺丰大件"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("GET /api/order/form missing %s: %s", needle, body)
		}
	}
}

func TestOrderAPIPaidStatusRequiresPaymentVoucher(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.pay_statuses(id,name) VALUES (3,'已收款') ON CONFLICT DO NOTHING;
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := `{
		"order_date":"2026-05-23",
		"customer_id":3,
		"source_id":1,
		"order_type_id":1,
		"pay_status_id":3,
		"payment_method":"微信支付",
		"payment_goods_amount":"88.00",
		"payment_shipping_amount":"0.00",
		"ship_status_id":1,
		"product_id":["7"],
		"item_name":["橘皮乌龙"],
		"tier_id":["manual"],
		"unit_price":["88"],
		"qty":["1"],
		"unit":["件"],
		"spec":["454"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(payload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/order status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "payment_voucher_asset_id") {
		t.Fatalf("POST /api/order should require payment voucher, body=%s", rec.Body.String())
	}
}

func TestOrderAPIShippedStatusRequiresLogisticsProduct(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.ship_statuses(id,name) VALUES (2,'已发货') ON CONFLICT DO NOTHING;
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := `{
		"order_date":"2026-05-23",
		"customer_id":3,
		"source_id":1,
		"order_type_id":1,
		"pay_status_id":1,
		"ship_status_id":2,
		"product_id":["7"],
		"item_name":["橘皮乌龙"],
		"tier_id":["manual"],
		"unit_price":["88"],
		"qty":["1"],
		"unit":["件"],
		"spec":["454"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(payload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/order status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "logistics_company_id") {
		t.Fatalf("POST /api/order should require logistics company, body=%s", rec.Body.String())
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

func TestOrderAPIFormReturnsCustomerProductUsageForCommonProductSorting(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,default_price,active,retail_price_227g)
		VALUES
			(8,'客户常订拼配',58,true,58),
			(9,'其他客户常订',59,true,59);
		INSERT INTO %[1]s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES
			(101,'SO-COMMON-1','2026-05-01',3,1,2,1,1,100,false),
			(102,'SO-COMMON-2','2026-05-02',3,1,2,1,1,100,false),
			(103,'SO-COMMON-3','2026-05-03',3,1,2,1,1,100,false),
			(104,'SO-COMMON-VOID','2026-05-04',3,1,2,1,1,100,true),
			(105,'SO-OTHER-CUSTOMER','2026-05-05',4,1,2,1,1,100,false);
		INSERT INTO %[1]s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES
			(101,1,8,'客户常订拼配',1,'件','454',58,58),
			(102,1,8,'客户常订拼配',1,'件','454',58,58),
			(103,1,7,'橘皮乌龙',1,'件','454',50,50),
			(104,1,9,'作废订单商品',1,'件','454',59,59),
			(105,1,9,'其他客户常订',1,'件','454',59,59);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		CustomerProductUsages []salesapp.CustomerProductUsageOption `json:"customer_product_usages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode order form: %v", err)
	}
	var product8, product9 salesapp.CustomerProductUsageOption
	for _, row := range resp.CustomerProductUsages {
		if row.CustomerID == 3 && row.ProductID == 8 {
			product8 = row
		}
		if row.CustomerID == 3 && row.ProductID == 9 {
			product9 = row
		}
	}
	if product8.OrderCount != 2 || product8.ItemCount != 2 || product8.LastOrderDate != "2026-05-02" {
		t.Fatalf("customer 3 product 8 usage = %+v, want 2 orders / 2 items / 2026-05-02", product8)
	}
	if product9.ProductID != 0 {
		t.Fatalf("voided order product should not be counted for customer 3: %+v", product9)
	}
}

func TestOrderAPIFormUsesCustomerCommercialBeanListForProductOptions(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,default_price,active,retail_price_227g,customer_id,base_product_id,visibility,custom_type,product_kind)
		VALUES
			(8,'芬纳定制-红酒日晒-中深烘',0,true,0,3,0,'customer_only','custom_roast','roasted'),
			(9,'芬纳曲奇定制',0,true,0,3,0,'customer_only','custom_roast','roasted');
		INSERT INTO %[1]s.bean_list_publications(id, list_type, version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at)
		VALUES
			(9903,'commercial','F-1','published','customer','3','{}'::jsonb,
			'{"groups":[{"items":[{"productId":8,"name":"芬纳定制-红酒日晒-中深烘","commercial_wholesale_tiers":[{"label":"2磅-13磅","spec_g":454,"min_qty":2,"max_qty":13,"price_per_unit":65,"price_per_lb":65,"template_id":6,"template_tier_id":56,"display_unit":"lb"},{"label":"14-23磅","spec_g":454,"min_qty":14,"max_qty":23,"price_per_unit":59,"price_per_lb":59,"template_id":6,"template_tier_id":57,"display_unit":"lb"}]},{"productId":9,"name":"芬纳曲奇定制","commercial_wholesale_tiers":[{"label":"2磅-13磅","spec_g":454,"min_qty":2,"max_qty":13,"price_per_unit":52,"price_per_lb":52,"template_id":6,"template_tier_id":56,"display_unit":"lb"}]}]}]}'::jsonb,
			'芬纳客户豆单','codex','2026-05-22 09:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form?customer_id=3", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Products []struct {
			ID    int64  `json:"id"`
			Name  string `json:"name"`
			Tiers []struct {
				ID              int64   `json:"id"`
				SpecG           int64   `json:"spec_g"`
				MinQty          float64 `json:"min"`
				UnitPrice       float64 `json:"unit_price"`
				DisplayUnit     string  `json:"display_unit"`
				ProductKind     string  `json:"product_kind"`
				PriceSourceJSON string  `json:"price_source_json"`
			} `json:"tiers"`
		} `json:"products"`
		BeanListVersionOptions []salesapp.BeanListVersionOption `json:"bean_list_version_options"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode order form: %v", err)
	}
	names := make([]string, 0, len(resp.Products))
	var redWine *struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Tiers []struct {
			ID              int64   `json:"id"`
			SpecG           int64   `json:"spec_g"`
			MinQty          float64 `json:"min"`
			UnitPrice       float64 `json:"unit_price"`
			DisplayUnit     string  `json:"display_unit"`
			ProductKind     string  `json:"product_kind"`
			PriceSourceJSON string  `json:"price_source_json"`
		} `json:"tiers"`
	}
	for i := range resp.Products {
		names = append(names, resp.Products[i].Name)
		if resp.Products[i].ID == 8 {
			redWine = &resp.Products[i]
		}
	}
	if strings.Contains(strings.Join(names, ","), "橘皮乌龙") {
		t.Fatalf("customer commercial bean list should hide public products not in the customer bean list, got %v", names)
	}
	if redWine == nil {
		t.Fatalf("order form missing customer bean-list product 8: %v", names)
	}
	if len(redWine.Tiers) != 2 {
		t.Fatalf("customer commercial tiers = %+v, want 2 tiers", redWine.Tiers)
	}
	if redWine.Tiers[0].ID != 56 || redWine.Tiers[0].SpecG != 454 || redWine.Tiers[0].MinQty != 2 || redWine.Tiers[0].UnitPrice != 65 {
		t.Fatalf("first customer commercial tier = %+v", redWine.Tiers[0])
	}
	if redWine.Tiers[0].DisplayUnit != "lb" || redWine.Tiers[0].ProductKind != "roasted_bean" || !strings.Contains(redWine.Tiers[0].PriceSourceJSON, `"publication_id":9903`) {
		t.Fatalf("customer commercial tier source = %+v", redWine.Tiers[0])
	}
}

func TestOrderAPIFormReturnsAllPublishedCommercialBeanListTiersForVersionSwitching(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.bean_list_publications(id, list_type, version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at)
		VALUES
			(9902,'commercial','V3.0.13','published','official','','{}'::jsonb,
			'{"groups":[{"items":[{"productId":7,"name":"橘皮乌龙","commercial_wholesale_tiers":[{"label":"2磅-13磅","spec_g":454,"min_qty":2,"max_qty":13,"price_per_unit":61,"price_per_lb":61,"template_id":6,"template_tier_id":62,"display_unit":"lb"}]}]}]}'::jsonb,
			'公共旧版','codex','2026-05-23 09:00:00+08'),
			(9903,'commercial','V3.0.14','published','official','','{}'::jsonb,
			'{"groups":[{"items":[{"productId":7,"name":"橘皮乌龙","commercial_wholesale_tiers":[{"label":"2磅-13磅","spec_g":454,"min_qty":2,"max_qty":13,"price_per_unit":64,"price_per_lb":64,"template_id":6,"template_tier_id":63,"display_unit":"lb"}]}]}]}'::jsonb,
			'公共新版','codex','2026-05-24 09:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form?customer_id=3", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Products []struct {
			ID    int64 `json:"id"`
			Tiers []struct {
				UnitPrice       float64 `json:"unit_price"`
				PriceSourceJSON string  `json:"price_source_json"`
			} `json:"tiers"`
		} `json:"products"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode order form: %v", err)
	}
	seen := map[int64]float64{}
	for _, product := range resp.Products {
		if product.ID != 7 {
			continue
		}
		for _, tier := range product.Tiers {
			var source map[string]any
			if err := json.Unmarshal([]byte(tier.PriceSourceJSON), &source); err != nil {
				t.Fatalf("decode price source json %q: %v", tier.PriceSourceJSON, err)
			}
			id, _ := source["publication_id"].(float64)
			if id > 0 {
				seen[int64(id)] = tier.UnitPrice
			}
		}
	}
	if seen[9902] != 61 || seen[9903] != 64 {
		t.Fatalf("commercial product tiers by publication = %#v, want 9902=61 and 9903=64", seen)
	}
}

func TestOrderAPIFormIncludesRetailCountTiersBesideCommercialTiers(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.bean_list_publications(
			id, list_type, publication_purpose, version_no, status, owner_type, owner_key,
			config_json, content_json, changelog, actor, published_at
		) VALUES
			(9931,'commercial','factory_supply','COMM-V1','published','official','','{}'::jsonb,
			'{"price_rows":[{"product_id":7,"sku_id":7,"parent_product_id":7,"tier_label":"1袋+","min_qty":1,"final_unit_price":75,"price_unit":"袋","quantity_basis":"sales_spec_count","tier_quantity_unit":"227g袋装","effective_sales_spec":{"sku_id":7,"spec_key":"bag-227g","spec_name":"227g袋装","spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}}]}'::jsonb,
			'商用价格','codex','2026-07-20 09:00:00+08'),
			(9932,'retail','factory_supply','RETAIL-V1','published','official','','{}'::jsonb,
			'{"price_rows":[{"product_id":7,"sku_id":7,"parent_product_id":7,"tier_label":"2袋+","min_qty":2,"final_unit_price":68,"price_unit":"袋","quantity_basis":"sales_spec_count","tier_quantity_unit":"227g袋装","effective_sales_spec":{"sku_id":7,"spec_key":"bag-227g","spec_name":"227g袋装","spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}}]}'::jsonb,
			'零售价格','codex','2026-07-20 10:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Products               []salesapp.ProductOption         `json:"products"`
		BeanListVersionOptions []salesapp.BeanListVersionOption `json:"bean_list_version_options"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode order form: %v", err)
	}
	seen := map[string]salesapp.ProductTierOption{}
	for _, product := range resp.Products {
		if product.ID != 7 {
			continue
		}
		for _, tier := range product.Tiers {
			var source struct {
				ListType      string `json:"list_type"`
				PublicationID int64  `json:"publication_id"`
			}
			if err := json.Unmarshal([]byte(tier.PriceSourceJSON), &source); err == nil && source.PublicationID > 0 {
				seen[source.ListType] = tier
			}
		}
	}
	if seen["commercial"].UnitPrice != 75 {
		t.Fatalf("commercial tier missing or overwritten: %+v", seen)
	}
	retail := seen["retail"]
	if retail.UnitPrice != 68 || retail.SpecG != 227 || retail.QuantityBasis != "sales_spec_count" || retail.EffectiveSalesSpec["spec_key"] != "bag-227g" {
		t.Fatalf("retail count tier = %+v", retail)
	}
	retailVersionFound := false
	for _, option := range resp.BeanListVersionOptions {
		if option.ID == 9932 && option.ListType == "retail" {
			retailVersionFound = true
		}
	}
	if !retailVersionFound {
		t.Fatalf("retail version option missing: %+v", resp.BeanListVersionOptions)
	}
}

func TestOrderAPIFormReturnsLatestBeanListVersionDefaultForStaleWarning(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.bean_list_publications(id, list_type, version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at)
		VALUES
			(9902,'commercial','F-1','published','customer','3','{}'::jsonb,'{}'::jsonb,'旧版','codex','2026-05-21 09:00:00+08'),
			(9903,'commercial','F-2','published','customer','3','{}'::jsonb,'{}'::jsonb,'新版','codex','2026-05-22 09:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form?customer_id=3", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		BeanListVersionOptions []salesapp.BeanListVersionOption `json:"bean_list_version_options"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode order form: %v", err)
	}
	defaultByID := map[int64]bool{}
	for _, option := range resp.BeanListVersionOptions {
		if option.CustomerID == 3 && option.ListType == "commercial" {
			defaultByID[option.ID] = option.IsDefault
		}
	}
	if len(defaultByID) != 2 || defaultByID[9902] || !defaultByID[9903] {
		t.Fatalf("commercial bean list defaults = %#v, want old=false latest=true", defaultByID)
	}
}

func TestOrderAPIFormReturnsClassificationPriceListIdentityAndDefaultsPerGroup(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.bean_list_publications(
			id, list_type, publication_purpose, product_type_category_id, product_type_name,
			classification_template_id, classification_template_name,
			version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at
		) VALUES
			(9941,'commercial','factory_supply',0,'KMM商品供应售价',0,'','V3.0.18','published','official','','{}'::jsonb,'{}'::jsonb,'旧价格表','codex','2026-07-20 09:00:00+08'),
			(9942,'commercial','factory_supply',0,'咖啡豆',221,'熟豆','V3.0.19','published','official','','{}'::jsonb,'{}'::jsonb,'熟豆旧版','codex','2026-07-22 14:43:00+08'),
			(9943,'commercial','factory_supply',0,'咖啡豆',221,'熟豆','V3.0.21','published','official','','{}'::jsonb,'{}'::jsonb,'熟豆新版','codex','2026-07-22 15:00:00+08'),
			(9944,'commercial','factory_supply',0,'挂耳咖啡',2,'挂耳','V3.0.20','published','official','','{}'::jsonb,'{}'::jsonb,'挂耳新版','codex','2026-07-22 15:10:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		BeanListVersionOptions []struct {
			ID                         int64  `json:"id"`
			ClassificationTemplateID   int64  `json:"classification_template_id"`
			ClassificationTemplateName string `json:"classification_template_name"`
			IsDefault                  bool   `json:"is_default"`
		} `json:"bean_list_version_options"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode order form: %v", err)
	}
	byID := map[int64]struct {
		classificationID   int64
		classificationName string
		isDefault          bool
	}{}
	for _, option := range resp.BeanListVersionOptions {
		if option.ID < 9941 || option.ID > 9944 {
			continue
		}
		byID[option.ID] = struct {
			classificationID   int64
			classificationName string
			isDefault          bool
		}{option.ClassificationTemplateID, option.ClassificationTemplateName, option.IsDefault}
	}
	if got := byID[9942]; got.classificationID != 221 || got.classificationName != "熟豆" || got.isDefault {
		t.Fatalf("old roasted classification option=%+v", got)
	}
	if got := byID[9943]; got.classificationID != 221 || got.classificationName != "熟豆" || !got.isDefault {
		t.Fatalf("latest roasted classification option=%+v", got)
	}
	if got := byID[9944]; got.classificationID != 2 || got.classificationName != "挂耳" || !got.isDefault {
		t.Fatalf("latest drip classification option=%+v", got)
	}
	if got := byID[9941]; got.isDefault {
		t.Fatalf("legacy option must not be default while classified publications exist: %+v", got)
	}
}

func TestOrderAPIFormCustomerClassificationOnlyReplacesMatchingPublicGroup(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.bean_list_publications(
			id, list_type, publication_purpose, product_type_category_id, product_type_name,
			classification_template_id, classification_template_name,
			version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at
		) VALUES
			(9951,'commercial','factory_supply',0,'咖啡豆',221,'熟豆','C-1','published','customer','3','{}'::jsonb,'{}'::jsonb,'客户熟豆','codex','2026-07-22 16:00:00+08'),
			(9952,'commercial','factory_supply',0,'咖啡豆',221,'熟豆','P-1','published','official','','{}'::jsonb,'{}'::jsonb,'公共熟豆','codex','2026-07-22 15:00:00+08'),
			(9953,'commercial','factory_supply',0,'挂耳咖啡',2,'挂耳','P-2','published','official','','{}'::jsonb,'{}'::jsonb,'公共挂耳','codex','2026-07-22 15:10:00+08'),
			(9954,'commercial','factory_supply',0,'',0,'','C-LEGACY','published','customer','3','{}'::jsonb,'{}'::jsonb,'客户历史版','codex','2026-07-22 14:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form?customer_id=3", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		BeanListVersionOptions []salesapp.BeanListVersionOption `json:"bean_list_version_options"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode order form: %v", err)
	}
	got := map[int64]bool{}
	for _, option := range resp.BeanListVersionOptions {
		if option.CustomerID == 3 && option.ID >= 9951 && option.ID <= 9954 {
			got[option.ID] = option.IsCustomerOwned
		}
	}
	_, hasPublicSameClassification := got[9952]
	_, hasPublicOtherClassification := got[9953]
	if !got[9951] || hasPublicSameClassification || !hasPublicOtherClassification || got[9953] || !got[9954] {
		t.Fatalf("customer/public classification fallback=%#v, want customer 9951 and 9954 plus public 9953 only", got)
	}
}

func TestOrderAPIFormHidesWithdrawnPublicBeanListVersionsForFallbackCustomer(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.bean_list_publications(id, list_type, version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at)
		VALUES
			(9911,'commercial','P-1','withdrawn','official','','{}'::jsonb,'{}'::jsonb,'旧公共版','codex','2026-05-20 09:00:00+08'),
			(9912,'commercial','P-2','published','official','','{}'::jsonb,'{}'::jsonb,'新公共版','codex','2026-05-22 09:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form?customer_id=3", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		BeanListVersionOptions []salesapp.BeanListVersionOption `json:"bean_list_version_options"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode order form: %v", err)
	}
	defaultByID := map[int64]bool{}
	ownedByID := map[int64]bool{}
	for _, option := range resp.BeanListVersionOptions {
		if option.CustomerID == 3 && option.ListType == "commercial" {
			defaultByID[option.ID] = option.IsDefault
			ownedByID[option.ID] = option.IsCustomerOwned
		}
	}
	if len(defaultByID) != 1 || defaultByID[9911] || !defaultByID[9912] {
		t.Fatalf("public fallback versions = %#v, want only currently published version 9912", defaultByID)
	}
	if ownedByID[9911] || ownedByID[9912] {
		t.Fatalf("public fallback versions should not be customer owned: %#v", ownedByID)
	}
}

func TestOrderAPIFormReturnsGlobalPublicBeanListVersionsBeforeCustomerSelected(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.bean_list_publications(id, list_type, version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at)
		VALUES
			(9921,'commercial','P-1','published','official','','{}'::jsonb,'{}'::jsonb,'公共旧版','codex','2026-05-20 09:00:00+08'),
			(9922,'commercial','P-2','published','official','','{}'::jsonb,'{}'::jsonb,'公共新版','codex','2026-05-22 09:00:00+08'),
			(9923,'green','G-1','published','official','','{}'::jsonb,'{}'::jsonb,'公共生豆','codex','2026-05-22 10:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		BeanListVersionOptions []salesapp.BeanListVersionOption `json:"bean_list_version_options"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode order form: %v", err)
	}
	defaultByID := map[int64]bool{}
	for _, option := range resp.BeanListVersionOptions {
		if option.CustomerID == 0 {
			defaultByID[option.ID] = option.IsDefault
			if option.IsCustomerOwned {
				t.Fatalf("global public option should not be customer owned: %+v", option)
			}
		}
	}
	for _, id := range []int64{9921, 9922, 9923} {
		if _, ok := defaultByID[id]; !ok {
			t.Fatalf("global public bean-list options missing id %d; got %#v", id, defaultByID)
		}
	}
	if defaultByID[9921] || !defaultByID[9922] || !defaultByID[9923] {
		t.Fatalf("global public default flags = %#v, want commercial latest and green latest defaults", defaultByID)
	}
}

func TestOrderAPIRejectsWithdrawnPublicBeanListPublicationVersion(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.bean_list_publications(id, list_type, version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor)
		VALUES(9911,'commercial','P-1','withdrawn','official','','{}'::jsonb,'{"title":"公共豆单 P-1"}'::jsonb,'旧公共版','tester');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":                          "2026-05-23",
		"customer_id":                         3,
		"source_id":                           1,
		"order_type_id":                       1,
		"pay_status_id":                       2,
		"payment_method":                      "微信支付",
		"ship_status_id":                      1,
		"commercial_bean_list_publication_id": 9911,
		"product_id":                          []string{"7"},
		"tier_id":                             []string{""},
		"unit_price":                          []string{"99"},
		"item_name":                           []string{"橘皮乌龙"},
		"qty":                                 []string{"1"},
		"unit":                                []string{"件"},
		"spec":                                []string{"454"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid bean list publication") {
		t.Fatalf("POST /api/order status/body = %d/%s, want 400 invalid bean list publication", rec.Code, rec.Body.String())
	}
}

func TestOrderAPIFormHidesPublicGreenBeansWhenCustomerDisablesPublicSKU(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_sku_public_usage(customer_id, use_public_sku, use_public_categories)
		VALUES (3, false, false);
		INSERT INTO %[1]s.products(id,name,default_price,active,retail_price_227g,customer_id,base_product_id,visibility,custom_type,product_kind)
		VALUES
			(8,'芬纳定制-红酒日晒-中深烘',0,true,0,3,0,'customer_only','custom_roast','roasted'),
			(88,'岩师傅红酒日晒生豆',0,true,0,0,0,'public','','green_bean');
		INSERT INTO %[1]s.bean_list_publications(id, list_type, version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at)
		VALUES
			(9903,'commercial','F-1','published','customer','3','{}'::jsonb,
			'{"groups":[{"items":[{"productId":8,"name":"芬纳定制-红酒日晒-中深烘","commercial_wholesale_tiers":[{"label":"2磅-13磅","spec_g":454,"min_qty":2,"max_qty":13,"price_per_unit":65,"price_per_lb":65,"template_id":6,"template_tier_id":56,"display_unit":"lb"}]}]}]}'::jsonb,
			'芬纳客户豆单','codex','2026-05-22 09:00:00+08'),
			(9904,'green','G-1','published','official','','{}'::jsonb,
			'{"groups":[{"items":[{"productId":88,"name":"岩师傅红酒日晒生豆","green_bean_sale_tiers":[{"label":"1KG","spec_g":1000,"min_qty":1,"price_per_unit":62,"display_unit":"kg","template_id":5,"template_tier_id":51}]}]}]}'::jsonb,
			'公共生豆豆单','codex','2026-05-22 10:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form?customer_id=3", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if strings.Contains(body, "岩师傅红酒日晒生豆") {
		t.Fatalf("customer with use_public_sku=false should not see public green bean products: %s", body)
	}
	if !strings.Contains(body, "芬纳定制-红酒日晒-中深烘") {
		t.Fatalf("customer-owned bean-list product should remain visible: %s", body)
	}
	if !strings.Contains(body, `"customer_public_usages"`) || !strings.Contains(body, `"use_public_sku":false`) {
		t.Fatalf("order form should expose customer public SKU usage for client-side filtering: %s", body)
	}
}

func TestOrderAPIFormDoesNotReturnBoundRoastedTiersForGreenBeanProduct(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,default_price,active,product_kind,green_bean_type,green_bean_bom_product_id,customer_id,visibility,custom_type)
		VALUES (88,'兰卡拼配生豆',0,true,'green_bean','blend',7,3,'customer_only','public_sku_alias');
		INSERT INTO %[1]s.product_price_tiers(id, product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, min_qty_lb, max_qty_lb, price_per_lb, active)
		VALUES
			(8801,7,1000,24,49,81.91,52.86,107.93,37.18714,true),
			(8802,7,1000,50,99,78.01,110.13,218.06,35.41654,true);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form?customer_id=3", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Products []struct {
			ID          int64  `json:"id"`
			ProductKind string `json:"product_kind"`
			Tiers       []struct {
				ID        int64   `json:"id"`
				SpecG     int64   `json:"spec_g"`
				MinQty    float64 `json:"min"`
				UnitPrice float64 `json:"unit_price"`
			} `json:"tiers"`
		} `json:"products"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode order form: %v", err)
	}
	var green *struct {
		ID          int64  `json:"id"`
		ProductKind string `json:"product_kind"`
		Tiers       []struct {
			ID        int64   `json:"id"`
			SpecG     int64   `json:"spec_g"`
			MinQty    float64 `json:"min"`
			UnitPrice float64 `json:"unit_price"`
		} `json:"tiers"`
	}
	for i := range resp.Products {
		if resp.Products[i].ID == 88 {
			green = &resp.Products[i]
			break
		}
	}
	if green == nil {
		t.Fatalf("order form missing green bean product 88: %s", rec.Body.String())
	}
	if green.ProductKind != "green_bean" {
		t.Fatalf("product_kind=%q, want green_bean", green.ProductKind)
	}
	if len(green.Tiers) != 0 {
		t.Fatalf("green bean must not inherit bound roasted tiers, got %+v", green.Tiers)
	}
}

func TestOrderAPIFormReturnsPublishedGreenBeanListTiersForGreenBeanProduct(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,default_price,active,product_kind,green_bean_type,green_bean_bom_product_id,customer_id,visibility,custom_type)
		VALUES (88,'曲奇拼配2.0',0,true,'green_bean','blend',7,3,'customer_only','public_sku_alias');
		INSERT INTO %[1]s.product_price_tiers(id, product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, active, product_kind)
		VALUES (8803,88,454,2,13,63,true,'roasted_bean');
		INSERT INTO %[1]s.bean_list_publications(id, list_type, version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at)
		VALUES
			(9901,'green','G-old','published','customer','3','{}'::jsonb,
			'{"groups":[{"items":[{"productId":88,"name":"曲奇拼配2.0","green_bean_sale_tiers":[{"label":"旧档","spec_g":1000,"min_qty":1,"price_per_unit":58,"price_per_lb":26.33,"template_id":5,"template_tier_id":49,"display_unit":"kg"}]}]}]}'::jsonb,
			'旧生豆价','codex','2026-05-18 09:00:00+08'),
			(9902,'green','G-new','published','customer','3','{"customizers":{"88":{"greenPriceOverrides":{"51":62}}}}'::jsonb,
			'{"groups":[{"items":[{"productId":88,"name":"曲奇拼配2.0","green_bean_sale_tiers":[{"label":"1KG","spec_g":1000,"min_qty":1,"max_qty":59,"price_per_unit":51.75,"price_per_lb":23.49,"template_id":5,"template_tier_id":50,"display_unit":"kg"},{"label":"60kG","spec_g":1000,"min_qty":60,"price_per_unit":62,"price_per_lb":28.15,"template_id":5,"template_tier_id":51,"display_unit":"kg"}]}]}]}'::jsonb,
			'新生豆价','codex','2026-05-19 09:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/order/form?customer_id=3", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/order/form status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Products []struct {
			ID          int64  `json:"id"`
			ProductKind string `json:"product_kind"`
			Tiers       []struct {
				ID              int64    `json:"id"`
				SpecG           int64    `json:"spec_g"`
				MinQty          float64  `json:"min"`
				MaxQty          *float64 `json:"max"`
				UnitPrice       float64  `json:"unit_price"`
				DisplayUnit     string   `json:"display_unit"`
				ProductKind     string   `json:"product_kind"`
				PriceSourceJSON string   `json:"price_source_json"`
			} `json:"tiers"`
		} `json:"products"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode order form: %v", err)
	}
	var green *struct {
		ID          int64  `json:"id"`
		ProductKind string `json:"product_kind"`
		Tiers       []struct {
			ID              int64    `json:"id"`
			SpecG           int64    `json:"spec_g"`
			MinQty          float64  `json:"min"`
			MaxQty          *float64 `json:"max"`
			UnitPrice       float64  `json:"unit_price"`
			DisplayUnit     string   `json:"display_unit"`
			ProductKind     string   `json:"product_kind"`
			PriceSourceJSON string   `json:"price_source_json"`
		} `json:"tiers"`
	}
	for i := range resp.Products {
		if resp.Products[i].ID == 88 {
			green = &resp.Products[i]
			break
		}
	}
	if green == nil {
		t.Fatalf("order form missing green bean product 88: %s", rec.Body.String())
	}
	if len(green.Tiers) != 2 {
		t.Fatalf("green bean tiers = %+v, want 2 published tiers", green.Tiers)
	}
	if green.Tiers[0].ID != 50 || green.Tiers[0].SpecG != 1000 || green.Tiers[0].MinQty != 1 || green.Tiers[0].MaxQty == nil || *green.Tiers[0].MaxQty != 59 || green.Tiers[0].UnitPrice != 23.49 {
		t.Fatalf("first published green bean tier = %+v", green.Tiers[0])
	}
	if green.Tiers[0].DisplayUnit != "kg" || green.Tiers[1].DisplayUnit != "kg" {
		t.Fatalf("green bean tier display units = %q/%q, want kg/kg", green.Tiers[0].DisplayUnit, green.Tiers[1].DisplayUnit)
	}
	if green.Tiers[1].ID != 51 || green.Tiers[1].MinQty != 60 || green.Tiers[1].UnitPrice != 62 {
		t.Fatalf("second published green bean tier = %+v", green.Tiers[1])
	}
	if green.Tiers[0].ProductKind != "green_bean" || !strings.Contains(green.Tiers[0].PriceSourceJSON, `"publication_id":9902`) {
		t.Fatalf("green bean tier source = %+v", green.Tiers[0])
	}
	if !strings.Contains(green.Tiers[1].PriceSourceJSON, `"price_unit":"kg"`) {
		t.Fatalf("manual green bean tier source should preserve kg price unit: %+v", green.Tiers[1])
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

func TestOrderAPISavesBeanListKgDisplayUnitForSmallPackageInsidePublishedRange(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,default_price,active,retail_price_227g,customer_id,base_product_id,visibility,custom_type,product_kind)
		VALUES (808,'兰卡拼配',0,true,0,3,0,'customer_only','custom_roast','roasted');
		INSERT INTO %[1]s.bean_list_publications(id, list_type, version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at)
		VALUES
			(9909,'commercial','V3.0.9','published','customer','3','{}'::jsonb,
			'{"groups":[{"items":[{"productId":808,"name":"兰卡拼配","commercial_wholesale_tiers":[{"label":"25-49kg","spec_g":1000,"min_qty":25,"max_qty":49,"price_per_unit":82,"price_per_lb":37.23,"template_id":6,"template_tier_id":64,"display_unit":"kg","price_unit":"kg"}]}]}]}'::jsonb,
			'kg量单','codex','2026-05-23 09:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":                          "2026-05-23",
		"customer_id":                         3,
		"source_id":                           1,
		"order_type_id":                       1,
		"pay_status_id":                       2,
		"payment_method":                      "微信支付",
		"ship_status_id":                      1,
		"commercial_bean_list_publication_id": 9909,
		"product_id":                          []string{"808"},
		"tier_id":                             []string{"64"},
		"unit_price":                          []string{"82"},
		"item_name":                           []string{"兰卡拼配"},
		"qty":                                 []string{"313"},
		"unit":                                []string{"件"},
		"spec":                                []string{"80"},
		"product_kind":                        []string{"roasted_bean"},
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
	var version, priceSource string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(unit_price,0)::float8,
		       COALESCE(line_total,0)::float8,
		       COALESCE(bean_list_version_no,''),
		       COALESCE(price_source_json,'{}'::jsonb)::text
		FROM %s.order_items
		WHERE product_id=808
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&unitPrice, &lineTotal, &version, &priceSource); err != nil {
		t.Fatalf("query order item: %v", err)
	}
	if unitPrice != 82 || lineTotal != 2053.28 || version != "V3.0.9" {
		t.Fatalf("unit_price/line_total/version=%.2f/%.2f/%q, want 82.00/2053.28/V3.0.9", unitPrice, lineTotal, version)
	}
	if !strings.Contains(priceSource, `"price_unit":"kg"`) && !strings.Contains(priceSource, `"price_unit": "kg"`) {
		t.Fatalf("price_source_json should retain kg price unit: %s", priceSource)
	}
}

func TestOrderAPICreatesCommercialOrderFromPR440FlatPriceRows(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,default_price,active,retail_price_227g,customer_id,base_product_id,visibility,custom_type,product_kind)
		VALUES (809,'PR440 平铺价格商品',0,true,0,0,0,'public','','roasted_bean');
		INSERT INTO %[1]s.bean_list_publications(id, list_type, version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at)
		VALUES
			(9919,'commercial','PR440-FLAT-1','published','customer','3','{}'::jsonb,
			'{"groups":[{"items":[{"productId":809,"name":"PR440 平铺价格商品"}]}],"price_rows":[{"product_id":809,"product_name":"PR440 平铺价格商品","tier_label":"1kg+","min_qty":1,"final_unit_price":88,"original_final_unit_price":88,"price_unit":"kg","currency":"CNY","inventory_unit":"kg","inventory_conversion_json":{"kg":1},"source_price_record_id":0,"tier_template_id":1,"tier_template_source":"product","pricing_rule_id":1,"pricing_rule_source":"product","pricing_rule_version":"PR440/v1","cost_source_snapshot":{"material_id":1},"customer_reference_snapshot":{"customer_id":3,"customer_display_name":"客户显示名"},"manual_adjusted":false}]}'::jsonb,
			'PR440 平铺价格','codex','2026-06-07 09:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":                          "2026-06-07",
		"customer_id":                         3,
		"source_id":                           1,
		"order_type_id":                       1,
		"pay_status_id":                       2,
		"payment_method":                      "微信支付",
		"ship_status_id":                      1,
		"commercial_bean_list_publication_id": 9919,
		"product_id":                          []string{"809"},
		"tier_id":                             []string{"pr440-flat"},
		"unit_price":                          []string{"88"},
		"item_name":                           []string{"客户显示名"},
		"qty":                                 []string{"1"},
		"unit":                                []string{"kg"},
		"spec":                                []string{"1000"},
		"product_kind":                        []string{"roasted_bean"},
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
	var version, priceSource string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(unit_price,0)::float8,
		       COALESCE(line_total,0)::float8,
		       COALESCE(bean_list_version_no,''),
		       COALESCE(price_source_json,'{}'::jsonb)::text
		FROM %s.order_items
		WHERE product_id=809
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&unitPrice, &lineTotal, &version, &priceSource); err != nil {
		t.Fatalf("query order item: %v", err)
	}
	if unitPrice != 88 || lineTotal != 88 || version != "PR440-FLAT-1" {
		t.Fatalf("unit_price/line_total/version=%.2f/%.2f/%q, want 88.00/88.00/PR440-FLAT-1", unitPrice, lineTotal, version)
	}
	for _, want := range []string{`"price_unit":"kg"`, `"pricing_rule_version":"PR440/v1"`, `"customer_reference_snapshot"`} {
		if !strings.Contains(priceSource, want) {
			t.Fatalf("price_source_json missing %s: %s", want, priceSource)
		}
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

func TestOrderAPIIgnoresOrderResponsiblePayloadAndUsesCustomerProfile(t *testing.T) {
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
	if responsibleType != "employee" || responsibleID != 5 || responsibleName != "销售小王" {
		t.Fatalf("responsible party = %s/%d/%s, want employee/5/销售小王", responsibleType, responsibleID, responsibleName)
	}
}

func TestOrderAPIRejectsCustomerWithoutResponsibleEmployee(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.customers(id,name,contact,phone,address,active,default_source_id,default_order_type_id,responsible_employee_id)
		VALUES (40,'未分配客户','老板','13800000040','杭州市未分配路',true,1,2,0);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":       "2026-05-06",
		"customer_id":      40,
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

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "customer responsible employee required") {
		t.Fatalf("POST /api/order without customer responsible status=%d body=%s, want 400 customer responsible employee required", rec.Code, rec.Body.String())
	}
}

func TestFilterOrderProductsForCustomerKeepsPublicAndOwnProducts(t *testing.T) {
	products := []ProductOption{
		{ID: 1, Name: "公共拼配", CustomerID: 0, Visibility: "public"},
		{ID: 2, Name: "测试客户专属深烘", CustomerID: 3, Visibility: "customer_only"},
		{ID: 3, Name: "其他客户专属深烘", CustomerID: 4, Visibility: "customer_only"},
	}

	got := filterOrderProductsForCustomer(products, 3, nil)
	names := make([]string, 0, len(got))
	for _, product := range got {
		names = append(names, product.Name)
	}
	if strings.Join(names, ",") != "公共拼配,测试客户专属深烘" {
		t.Fatalf("filtered names = %q", strings.Join(names, ","))
	}
}

func TestFilterOrderProductsForCustomerLimitsCustomerOwnedBeanListScope(t *testing.T) {
	products := []ProductOption{
		{
			ID:          1,
			Name:        "公共拼配",
			CustomerID:  0,
			Visibility:  "public",
			ProductKind: "roasted",
			Tiers:       []ProductTierOption{{ID: 11, PriceSourceJSON: `{"source":"published_bean_list","list_type":"commercial","publication_id":12}`}},
		},
		{
			ID:          2,
			Name:        "芬纳定制-红酒日晒-中深烘",
			CustomerID:  3,
			Visibility:  "customer_only",
			ProductKind: "roasted",
			Tiers:       []ProductTierOption{{ID: 56, PriceSourceJSON: `{"source":"published_bean_list","list_type":"commercial","publication_id":9903}`}},
		},
		{
			ID:          3,
			Name:        "芬纳未发布定制",
			CustomerID:  3,
			Visibility:  "customer_only",
			ProductKind: "roasted",
			Tiers:       []ProductTierOption{},
		},
	}
	versionOptions := []salesapp.BeanListVersionOption{{
		CustomerID:      3,
		ListType:        "commercial",
		ID:              9903,
		IsCustomerOwned: true,
		IsDefault:       true,
	}}

	got := filterOrderProductsForCustomer(products, 3, versionOptions)
	names := make([]string, 0, len(got))
	for _, product := range got {
		names = append(names, product.Name)
	}
	if strings.Join(names, ",") != "芬纳定制-红酒日晒-中深烘" {
		t.Fatalf("filtered names = %q", strings.Join(names, ","))
	}
}

func TestFilterOrderProductsForCustomerKeepsCustomerOwnedAndVisiblePublicFallbackScopes(t *testing.T) {
	products := []ProductOption{
		{
			ID:          1,
			Name:        "客户熟豆",
			CustomerID:  3,
			Visibility:  "customer_only",
			ProductKind: "roasted",
			Tiers:       []ProductTierOption{{ID: 11, PriceSourceJSON: `{"source":"published_bean_list","list_type":"commercial","publication_id":9951}`}},
		},
		{
			ID:          2,
			Name:        "旧版多规格熟豆",
			CustomerID:  0,
			Visibility:  "public",
			ProductKind: "roasted",
			Tiers:       []ProductTierOption{{ID: 12, PriceSourceJSON: `{"source":"published_bean_list","list_type":"commercial","publication_id":9952}`}},
		},
		{
			ID:          3,
			Name:        "公共挂耳",
			CustomerID:  0,
			Visibility:  "public",
			ProductKind: "roasted",
			Tiers:       []ProductTierOption{{ID: 13, PriceSourceJSON: `{"source":"published_bean_list","list_type":"commercial","publication_id":9953}`}},
		},
		{
			ID:          4,
			Name:        "客户历史价格表商品",
			CustomerID:  3,
			Visibility:  "customer_only",
			ProductKind: "roasted",
			Tiers:       []ProductTierOption{{ID: 14, PriceSourceJSON: `{"source":"published_bean_list","list_type":"commercial","publication_id":9954}`}},
		},
		{
			ID:          5,
			Name:        "客户价格表引用的公共商品档案",
			CustomerID:  0,
			Visibility:  "public",
			ProductKind: "roasted",
			Tiers:       []ProductTierOption{{ID: 15, PriceSourceJSON: `{"source":"published_bean_list","list_type":"commercial","publication_id":9951}`}},
		},
	}
	versionOptions := []salesapp.BeanListVersionOption{
		{CustomerID: 3, ListType: "commercial", ClassificationTemplateID: 221, ID: 9951, IsCustomerOwned: true, IsDefault: true},
		{CustomerID: 3, ListType: "commercial", ClassificationTemplateID: 2, ID: 9953, IsCustomerOwned: false, IsDefault: true},
		{CustomerID: 3, ListType: "commercial", ID: 9954, IsCustomerOwned: true, IsDefault: false},
	}

	got := filterOrderProductsForCustomer(products, 3, versionOptions)
	names := make([]string, 0, len(got))
	for _, product := range got {
		names = append(names, product.Name)
	}
	if strings.Join(names, ",") != "客户熟豆,公共挂耳,客户历史价格表商品,客户价格表引用的公共商品档案" {
		t.Fatalf("filtered names = %q, want customer class A, public fallback class B, and customer legacy only", strings.Join(names, ","))
	}

	publicUsages := []salesapp.CustomerPublicUsageOption{{CustomerID: 3, UsePublicSKU: false}}
	got = filterOrderProductsForCustomer(products, 3, versionOptions, publicUsages)
	names = names[:0]
	for _, product := range got {
		names = append(names, product.Name)
	}
	if strings.Join(names, ",") != "客户熟豆,客户历史价格表商品,客户价格表引用的公共商品档案" {
		t.Fatalf("private-only filtered names = %q, want only customer-owned publication scopes", strings.Join(names, ","))
	}
}

func TestFilterOrderProductsForCustomerHonorsPublicSKUUsage(t *testing.T) {
	products := []ProductOption{
		{ID: 1, Name: "公共熟豆", CustomerID: 0, Visibility: "public", ProductKind: "roasted"},
		{ID: 2, Name: "岩师傅红酒日晒生豆", CustomerID: 0, Visibility: "public", ProductKind: "green_bean"},
		{ID: 3, Name: "芬纳定制-红酒日晒-中深烘", CustomerID: 74, Visibility: "customer_only", ProductKind: "roasted"},
	}
	publicUsages := []salesapp.CustomerPublicUsageOption{{
		CustomerID:   74,
		UsePublicSKU: false,
	}}

	got := filterOrderProductsForCustomer(products, 74, nil, publicUsages)
	names := make([]string, 0, len(got))
	for _, product := range got {
		names = append(names, product.Name)
	}
	if strings.Join(names, ",") != "芬纳定制-红酒日晒-中深烘" {
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
		`"quote_source_trace"`,
		`"price_list_version":"V3.0.8"`,
		`"pricing_rule_version":"PR-COST/v3"`,
		`"production_source_trace"`,
		`"bom_version_no":"BOM-A1/V002"`,
		`"process_route_name":"标准烘焙"`,
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
				PriceSourceJSON:       `{"source":"published_bean_list","publication_id":7,"version":"V3.0.8","tier_label":"24kg+","price_unit":"kg","final_unit_price":82,"pricing_rule_version":"PR-COST/v3","manual_adjusted":true,"cost_source_snapshot":{"bom_version_no":"BOM-A1/V002","process_route_name":"标准烘焙"}}`,
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

func TestOrderAPISaveCarriesDocumentAndOrderDates(t *testing.T) {
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
		"document_date":  "2026-05-23",
		"order_date":     "2026-05-20",
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

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if got := repo.cmd.DocumentDate.Format("2006-01-02"); got != "2026-05-23" {
		t.Fatalf("document date = %s, want 2026-05-23", got)
	}
	if got := repo.cmd.OrderDate.Format("2006-01-02"); got != "2026-05-20" {
		t.Fatalf("order date = %s, want 2026-05-20", got)
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

func TestOrderAPISaveCarriesUnitAmountItemDiscount(t *testing.T) {
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
		"order_date":     "2026-05-22",
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
		"discount_type":  []string{"unit_amount"},
		"discount_value": []string{"10"},
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
	if repo.cmd.Items[0].DiscountType != "unit_amount" || repo.cmd.Items[0].DiscountValue != 10 {
		t.Fatalf("captured item discount = %q/%v, want unit_amount/10", repo.cmd.Items[0].DiscountType, repo.cmd.Items[0].DiscountValue)
	}
}

func TestOrderAPISavesUnitAmountItemDiscountTotals(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-05-22",
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
		"discount_type":  []string{"unit_amount"},
		"discount_value": []string{"10"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var orderDiscount, grandTotal, lineDiscount, lineTotal float64
	var discountType string
	var discountValue float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(o.discount_amount,0), COALESCE(o.grand_total,0),
		       COALESCE(oi.discount_type,''), COALESCE(oi.discount_value,0),
		       COALESCE(oi.discount_amount,0), COALESCE(oi.line_total,0)
		FROM %s.orders o
		JOIN %s.order_items oi ON oi.order_id=o.id
		ORDER BY o.id DESC
		LIMIT 1
	`, schema, schema)).Scan(&orderDiscount, &grandTotal, &discountType, &discountValue, &lineDiscount, &lineTotal); err != nil {
		t.Fatalf("query saved order discount totals: %v", err)
	}
	if discountType != "unit_amount" || discountValue != 10 {
		t.Fatalf("saved item discount = %q/%v, want unit_amount/10", discountType, discountValue)
	}
	if orderDiscount != 20 || lineDiscount != 20 || lineTotal != 156 || grandTotal != 156 {
		t.Fatalf("discount totals = order %.2f line discount %.2f line %.2f grand %.2f; want 20/20/156/156", orderDiscount, lineDiscount, lineTotal, grandTotal)
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

func TestOrderAPISavesGreenBeanOrderRejectsMissingGreenBeanListPrice(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,default_price,active,product_kind,green_bean_type,green_bean_bom_product_id,customer_id,visibility,custom_type)
		VALUES (88,'兰卡拼配生豆',0,true,'green_bean','blend',7,3,'customer_only','public_sku_alias');
		INSERT INTO %[1]s.product_price_tiers(id, product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, min_qty_lb, max_qty_lb, price_per_lb, active)
		VALUES
			(8801,7,1000,24,49,81.91,52.86,107.93,37.18714,true),
			(8802,7,1000,50,99,78.01,110.13,218.06,35.41654,true);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":     "2026-05-19",
		"customer_id":    3,
		"source_id":      1,
		"order_type_id":  1,
		"pay_status_id":  1,
		"ship_status_id": 1,
		"product_id":     []string{"88"},
		"tier_id":        []string{"auto"},
		"unit_price":     []string{""},
		"item_name":      []string{"兰卡拼配生豆"},
		"product_kind":   []string{"green_bean"},
		"qty":            []string{"30"},
		"unit":           []string{"kg"},
		"spec":           []string{"1000"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/order status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "生豆豆单") && !strings.Contains(rec.Body.String(), "green bean") {
		t.Fatalf("missing green bean list price error, body=%s", rec.Body.String())
	}
}

func TestOrderAPISavesGreenBeanOrderUsingSelectedGreenBeanListPublication(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,default_price,active,product_kind,green_bean_type,green_bean_bom_product_id,customer_id,visibility,custom_type)
		VALUES (88,'兰卡拼配生豆',0,true,'green_bean','blend',7,3,'customer_only','public_sku_alias');
		INSERT INTO %[1]s.bean_list_publications(id, list_type, version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at)
		VALUES (9901,'green','G-2026-05-18','published','customer','3','{}'::jsonb,
			'{"groups":[{"items":[{"productId":88,"name":"兰卡拼配生豆","green_bean_sale_tiers":[{"label":"24-49kg","spec_g":1000,"min_qty":24,"max_qty":49,"price_per_unit":72,"display_unit":"kg","min_weight_g":24000,"max_weight_g":49000}]}]}]}'::jsonb,
			'生豆手工价','codex','2026-05-18 09:00:00+08'),
			(9902,'green','G-2026-05-19','published','customer','3','{}'::jsonb,
			'{"groups":[{"items":[{"productId":88,"name":"兰卡拼配生豆","green_bean_sale_tiers":[{"label":"24-49kg","spec_g":1000,"min_qty":24,"max_qty":49,"price_per_unit":99,"display_unit":"kg","min_weight_g":24000,"max_weight_g":49000}]}]}]}'::jsonb,
			'生豆较新版本','codex','2026-05-19 09:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":                     "2026-05-19",
		"customer_id":                    3,
		"source_id":                      1,
		"order_type_id":                  1,
		"pay_status_id":                  1,
		"ship_status_id":                 1,
		"bean_list_publication_id":       9902,
		"green_bean_list_publication_id": 9901,
		"product_id":                     []string{"88"},
		"tier_id":                        []string{"auto"},
		"unit_price":                     []string{""},
		"item_name":                      []string{"兰卡拼配生豆"},
		"product_kind":                   []string{"green_bean"},
		"qty":                            []string{"30"},
		"unit":                           []string{"kg"},
		"spec":                           []string{"1000"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var productKind, versionNo string
	var publicationID int64
	var unitPrice, lineTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(product_kind,''), COALESCE(bean_list_publication_id,0), COALESCE(bean_list_version_no,''), COALESCE(unit_price,0)::float8, COALESCE(line_total,0)::float8
		FROM %s.order_items
		WHERE product_id=88
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&productKind, &publicationID, &versionNo, &unitPrice, &lineTotal); err != nil {
		t.Fatalf("query green bean order item: %v", err)
	}
	if productKind != "green_bean" || publicationID != 9901 || versionNo != "G-2026-05-18" || unitPrice != 72 || lineTotal != 2160 {
		t.Fatalf("saved green item kind/pub/version/unit/total=%q/%d/%q/%.2f/%.2f, want green_bean/9901/G-2026-05-18/72.00/2160.00", productKind, publicationID, versionNo, unitPrice, lineTotal)
	}
}

func TestOrderAPISavesRetailLinesUsingEachCategoryPublication(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.products SET product_category_id=111 WHERE id=7;
		INSERT INTO %[1]s.products(id,name,default_price,active,product_kind,product_category_id)
		VALUES (8,'冻干咖啡',0,true,'instant_coffee',112);
		INSERT INTO %[1]s.bean_list_publications(
			id, list_type, publication_purpose, product_type_category_id, product_type_name,
			version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at
		) VALUES
			(9940,'commercial','factory_supply',101,'咖啡熟豆','COMM-HEADER','published','official','','{}'::jsonb,
			'{"price_rows":[]}'::jsonb,'订单表头商业发布','codex','2026-07-20 10:00:00+08'),
			(9941,'retail','factory_supply',101,'咖啡熟豆','RETAIL-ROASTED','published','official','','{}'::jsonb,
			'{"price_rows":[{"product_id":7,"sku_id":7,"spec_g":227,"min_qty":1,"final_unit_price":70,"price_unit":"袋","quantity_basis":"sales_spec_count"}]}'::jsonb,
			'熟豆零售价','codex','2026-07-20 11:00:00+08'),
			(9942,'retail','factory_supply',102,'速溶咖啡','RETAIL-INSTANT','published','official','','{}'::jsonb,
			'{"price_rows":[{"product_id":8,"sku_id":8,"spec_g":13,"min_qty":1,"final_unit_price":12,"price_unit":"条","quantity_basis":"sales_spec_count"}]}'::jsonb,
			'速溶零售价','codex','2026-07-20 12:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":                          "2026-07-20",
		"customer_id":                         3,
		"source_id":                           1,
		"order_type_id":                       2,
		"pay_status_id":                       1,
		"ship_status_id":                      1,
		"bean_list_publication_id":            9941,
		"commercial_bean_list_publication_id": 9940,
		"item_bean_list_publication_id":       []string{"9941", "9942"},
		"product_id":                          []string{"7", "8"},
		"tier_id":                             []string{"auto", "auto"},
		"unit_price":                          []string{"", ""},
		"item_name":                           []string{"橘皮乌龙", "冻干咖啡"},
		"product_kind":                        []string{"roasted_bean", "instant_coffee"},
		"qty":                                 []string{"2", "3"},
		"unit":                                []string{"袋", "条"},
		"spec":                                []string{"227", "13"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status=%d body=%s", rec.Code, rec.Body.String())
	}

	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT product_id, COALESCE(bean_list_publication_id,0), COALESCE(bean_list_version_no,''),
		       COALESCE(unit_price,0)::float8, COALESCE(line_total,0)::float8
		FROM %s.order_items
		WHERE product_id IN (7,8)
		ORDER BY product_id
	`, schema))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type savedLine struct {
		publicationID int64
		versionNo     string
		unitPrice     float64
		lineTotal     float64
	}
	got := map[int64]savedLine{}
	for rows.Next() {
		var productID int64
		var line savedLine
		if err := rows.Scan(&productID, &line.publicationID, &line.versionNo, &line.unitPrice, &line.lineTotal); err != nil {
			t.Fatal(err)
		}
		got[productID] = line
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if line := got[7]; line.publicationID != 9941 || line.versionNo != "RETAIL-ROASTED" || line.unitPrice != 70 || line.lineTotal != 140 {
		t.Fatalf("roasted retail line = %+v", line)
	}
	if line := got[8]; line.publicationID != 9942 || line.versionNo != "RETAIL-INSTANT" || line.unitPrice != 12 || line.lineTotal != 36 {
		t.Fatalf("instant retail line = %+v", line)
	}
}

func TestOrderAPISavesCommercialAndGreenLinesUsingItemPublicationsBeforeHeaders(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.customers SET default_order_type_id=1 WHERE id=3;
		INSERT INTO %[1]s.products(id,name,default_price,active,product_kind,green_bean_type,green_bean_bom_product_id)
		VALUES (88,'兰卡拼配生豆',0,true,'green_bean','blend',7);
		INSERT INTO %[1]s.bean_list_publications(
			id, list_type, publication_purpose, version_no, status, owner_type, owner_key,
			config_json, content_json, changelog, actor, published_at
		) VALUES
			(9950,'commercial','factory_supply','COMM-HEADER','published','official','','{}'::jsonb,
			'{"price_rows":[{"product_id":7,"sku_id":7,"spec_g":1000,"min_qty":1,"final_unit_price":199,"price_unit":"kg","quantity_basis":"sales_spec_count"}]}'::jsonb,
			'错误表头商业价','codex','2026-07-20 09:00:00+08'),
			(9951,'commercial','factory_supply','COMM-ITEM','published','official','','{}'::jsonb,
			'{"price_rows":[{"product_id":7,"sku_id":7,"spec_g":1000,"min_qty":1,"final_unit_price":71,"price_unit":"kg","quantity_basis":"sales_spec_count"}]}'::jsonb,
			'行级商业价','codex','2026-07-20 10:00:00+08'),
			(9960,'green','factory_supply','GREEN-HEADER','published','official','','{}'::jsonb,
			'{"price_rows":[{"product_id":88,"sku_id":88,"spec_g":1000,"min_qty":1,"final_unit_price":299,"price_unit":"kg","quantity_basis":"sales_spec_count"}]}'::jsonb,
			'错误表头生豆价','codex','2026-07-20 11:00:00+08'),
			(9961,'green','factory_supply','GREEN-ITEM','published','official','','{}'::jsonb,
			'{"price_rows":[{"product_id":88,"sku_id":88,"spec_g":1000,"min_qty":1,"final_unit_price":81,"price_unit":"kg","quantity_basis":"sales_spec_count"}]}'::jsonb,
			'行级生豆价','codex','2026-07-20 12:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":                          "2026-07-20",
		"customer_id":                         3,
		"source_id":                           1,
		"order_type_id":                       1,
		"pay_status_id":                       1,
		"ship_status_id":                      1,
		"commercial_bean_list_publication_id": 9950,
		"green_bean_list_publication_id":      9960,
		"item_bean_list_publication_id":       []string{"9951", "9961"},
		"product_id":                          []string{"7", "88"},
		"tier_id":                             []string{"auto", "auto"},
		"unit_price":                          []string{"", ""},
		"item_name":                           []string{"橘皮乌龙", "兰卡拼配生豆"},
		"product_kind":                        []string{"roasted_bean", "green_bean"},
		"qty":                                 []string{"2", "3"},
		"unit":                                []string{"kg", "kg"},
		"spec":                                []string{"1000", "1000"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status=%d body=%s", rec.Code, rec.Body.String())
	}

	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT product_id, COALESCE(bean_list_publication_id,0), COALESCE(bean_list_version_no,''),
		       COALESCE(unit_price,0)::float8, COALESCE(line_total,0)::float8
		FROM %s.order_items
		WHERE product_id IN (7,88)
		ORDER BY product_id
	`, schema))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type savedLine struct {
		publicationID int64
		versionNo     string
		unitPrice     float64
		lineTotal     float64
	}
	got := map[int64]savedLine{}
	for rows.Next() {
		var productID int64
		var line savedLine
		if err := rows.Scan(&productID, &line.publicationID, &line.versionNo, &line.unitPrice, &line.lineTotal); err != nil {
			t.Fatal(err)
		}
		got[productID] = line
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if line := got[7]; line.publicationID != 9951 || line.versionNo != "COMM-ITEM" || line.unitPrice != 71 || line.lineTotal != 142 {
		t.Fatalf("commercial item-selected line = %+v", line)
	}
	if line := got[88]; line.publicationID != 9961 || line.versionNo != "GREEN-ITEM" || line.unitPrice != 81 || line.lineTotal != 243 {
		t.Fatalf("green item-selected line = %+v", line)
	}
	var headerPublicationID int64
	var headerVersionNo string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(bean_list_publication_id,0), COALESCE(bean_list_version_no,'')
		FROM %s.orders
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&headerPublicationID, &headerVersionNo); err != nil {
		t.Fatal(err)
	}
	if headerPublicationID != 9951 || headerVersionNo != "COMM-ITEM" {
		t.Fatalf("commercial order header = %d/%q, want resolved line publication 9951/COMM-ITEM", headerPublicationID, headerVersionNo)
	}
}

func TestOrderAPIClearsHeaderForMultipleCommercialClassificationPublications(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.customers SET default_order_type_id=1 WHERE id=3;
		UPDATE %[1]s.products SET product_category_id=101 WHERE id=7;
		INSERT INTO %[1]s.products(id,name,default_price,active,product_kind,product_category_id)
		VALUES (8,'冻干咖啡',0,true,'instant_coffee',102);
		INSERT INTO %[1]s.bean_list_publications(
			id, list_type, publication_purpose, classification_template_id, classification_template_name,
			version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor, published_at
		) VALUES
			(9970,'commercial','factory_supply',0,'','COMM-HEADER','published','official','','{}'::jsonb,
			'{"price_rows":[{"product_id":7,"sku_id":7,"spec_g":1000,"min_qty":1,"final_unit_price":199,"price_unit":"kg","quantity_basis":"sales_spec_count"}]}'::jsonb,
			'错误兼容表头','codex','2026-07-20 09:00:00+08'),
			(9971,'commercial','factory_supply',101,'咖啡熟豆','COMM-ROASTED','published','official','','{}'::jsonb,
			'{"price_rows":[{"product_id":7,"sku_id":7,"spec_g":1000,"min_qty":1,"final_unit_price":71,"price_unit":"kg","quantity_basis":"sales_spec_count"}]}'::jsonb,
			'熟豆分类价','codex','2026-07-20 10:00:00+08'),
			(9972,'commercial','factory_supply',102,'速溶咖啡','COMM-INSTANT','published','official','','{}'::jsonb,
			'{"price_rows":[{"product_id":8,"sku_id":8,"spec_g":13,"min_qty":1,"final_unit_price":12,"price_unit":"条","quantity_basis":"sales_spec_count"}]}'::jsonb,
			'速溶分类价','codex','2026-07-20 11:00:00+08');
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{
		"order_date":                          "2026-07-20",
		"customer_id":                         3,
		"source_id":                           1,
		"order_type_id":                       1,
		"pay_status_id":                       1,
		"ship_status_id":                      1,
		"commercial_bean_list_publication_id": 9970,
		"item_bean_list_publication_id":       []string{"9971", "9972"},
		"product_id":                          []string{"7", "8"},
		"tier_id":                             []string{"auto", "auto"},
		"unit_price":                          []string{"", ""},
		"item_name":                           []string{"白月光瑰夏", "冻干咖啡"},
		"product_kind":                        []string{"roasted_bean", "instant_coffee"},
		"qty":                                 []string{"2", "3"},
		"unit":                                []string{"kg", "条"},
		"spec":                                []string{"1000", "13"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/order status=%d body=%s", rec.Code, rec.Body.String())
	}

	var headerPublicationID int64
	var headerVersionNo string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(bean_list_publication_id,0), COALESCE(bean_list_version_no,'')
		FROM %s.orders
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&headerPublicationID, &headerVersionNo); err != nil {
		t.Fatal(err)
	}
	if headerPublicationID != 0 || headerVersionNo != "" {
		t.Fatalf("multi-classification order header = %d/%q, want 0/empty", headerPublicationID, headerVersionNo)
	}

	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT product_id, COALESCE(bean_list_publication_id,0), COALESCE(bean_list_version_no,''),
		       COALESCE(unit_price,0)::float8, COALESCE(line_total,0)::float8
		FROM %s.order_items
		WHERE product_id IN (7,8)
		ORDER BY product_id
	`, schema))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type savedLine struct {
		publicationID int64
		versionNo     string
		unitPrice     float64
		lineTotal     float64
	}
	got := map[int64]savedLine{}
	for rows.Next() {
		var productID int64
		var line savedLine
		if err := rows.Scan(&productID, &line.publicationID, &line.versionNo, &line.unitPrice, &line.lineTotal); err != nil {
			t.Fatal(err)
		}
		got[productID] = line
	}
	if line := got[7]; line.publicationID != 9971 || line.versionNo != "COMM-ROASTED" || line.unitPrice != 71 || line.lineTotal != 142 {
		t.Fatalf("roasted classified line = %+v", line)
	}
	if line := got[8]; line.publicationID != 9972 || line.versionNo != "COMM-INSTANT" || line.unitPrice != 12 || line.lineTotal != 36 {
		t.Fatalf("instant classified line = %+v", line)
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
	seedOrderAPITestConcreteUnitConversion(t, ctx, pool, schema)
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
	seedOrderAPITestConcreteUnitConversion(t, ctx, pool, schema)
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
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	previousFilename := orderShippingFilename(salesapp.OrderShippingExportData{
		OrderDate:    "2026-04-27",
		CustomerName: "测试客户",
		OrderNo:      "SO-SHIP-SAVE-FAIL",
	})
	previousPath := filepath.Join(exportDir, previousFilename)
	previousContents := []byte("previous successful shipment export")
	if err := os.WriteFile(previousPath, previousContents, 0o644); err != nil {
		t.Fatal(err)
	}
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
	if !strings.Contains(rec.Body.String(), "快递录单 Excel 生成失败，请稍后重试") || strings.Contains(rec.Body.String(), "SQLSTATE") || strings.Contains(rec.Body.String(), "order_shipment_test_reject_generated") || strings.Contains(rec.Body.String(), "violates check constraint") {
		t.Fatalf("shipping Excel persistence error leaked database details: %s", rec.Body.String())
	}
	files, err := os.ReadDir(exportDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name() != previousFilename {
		t.Fatalf("failed shipment save changed existing shipping exports=%v", files)
	}
	gotContents, err := os.ReadFile(previousPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotContents, previousContents) {
		t.Fatalf("existing shipment export was overwritten: got %q", gotContents)
	}
}

func TestOrdersShippingExcelAPIAcceptsNoProductionShipReadyOrders(t *testing.T) {
	if !orderShippingReady(salesapp.OrderShippingExportData{ProcessStatus: "无需生产"}) {
		t.Fatal("无需生产 status should be treated as ready for shipping")
	}
	if !orderShippingReady(salesapp.OrderShippingExportData{ProcessStatus: "库存待发货"}) {
		t.Fatal("库存待发货 status should be treated as ready for shipping")
	}
	if orderShippingReady(salesapp.OrderShippingExportData{ProcessStatus: "生产完成", ShipStatus: "已发货"}) {
		t.Fatal("already shipped orders should not be treated as ready for another shipping export")
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

func TestOrderAPIShipReadyExcludesAlreadyShippedOrders(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.ship_statuses(name) VALUES ('已发货') ON CONFLICT(name) DO NOTHING;
		INSERT INTO %[1]s.order_process_statuses(name,sort,active) VALUES ('生产完成',20,true)
		ON CONFLICT(name) DO UPDATE SET sort=excluded.sort, active=excluded.active;
		INSERT INTO %[1]s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES
			(70, 'SO-SHIP-READY-NOT-SHIPPED', '2026-06-16', 3, 1, 2, (SELECT id FROM %[1]s.ship_statuses WHERE name='未发货' LIMIT 1), (SELECT id FROM %[1]s.order_process_statuses WHERE name='生产完成' LIMIT 1), 88, false),
			(71, 'SO-SHIP-READY-SHIPPED', '2026-06-16', 3, 1, 2, (SELECT id FROM %[1]s.ship_statuses WHERE name='已发货' LIMIT 1), (SELECT id FROM %[1]s.order_process_statuses WHERE name='生产完成' LIMIT 1), 88, false);
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/orders?ship_ready=1&limit=50", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders ship_ready status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "SO-SHIP-READY-NOT-SHIPPED") {
		t.Fatalf("ship_ready list should include ready unshipped order: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SO-SHIP-READY-SHIPPED") {
		t.Fatalf("ship_ready list should exclude already shipped order: %s", rec.Body.String())
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

func TestOrdersShippingTrackingAPIDeductsDefaultFinishedInventoryWithoutAllocation(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.ship_statuses(id,name) VALUES (2,'已发货') ON CONFLICT DO NOTHING;
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (33, 'SO-PRODUCED-NO-ALLOC-SHIP', '2026-06-06', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='生产完成' LIMIT 1), 100, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES (33,1,7,'橘皮乌龙',2,'袋','454g',50,100);
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES (7,454,'finished_goods',3,0);
	`, schema, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{"tracking_no": "SF-PRODUCED-001"})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/33/shipping-tracking", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/33/shipping-tracking status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var units, looseG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g
		FROM %s.finished_inventory
		WHERE product_id=7 AND spec_g=454 AND warehouse='finished_goods'
	`, schema)).Scan(&units, &looseG); err != nil {
		t.Fatalf("query finished inventory: %v", err)
	}
	if units != 1 || looseG != 0 {
		t.Fatalf("finished inventory after produced shipment = %d units + %dg, want 1 + 0g", units, looseG)
	}
	var deductionCount, ledgerCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.order_stock_deductions WHERE order_id=33 AND batch_code='SOURCE-WH:finished_goods' AND deducted_g=908`, schema)).Scan(&deductionCount); err != nil {
		t.Fatalf("query default warehouse deductions: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='sales_order_shipment' AND source_doc_id=33 AND warehouse='finished_goods' AND qty_change_g=-908`, schema)).Scan(&ledgerCount); err != nil {
		t.Fatalf("query default warehouse ledger: %v", err)
	}
	if deductionCount != 1 || ledgerCount != 1 {
		t.Fatalf("default warehouse deduction=%d ledger=%d, want 1/1", deductionCount, ledgerCount)
	}
}

func TestOrdersShippingTrackingAPIDeductsDefaultFinishedBatchWithoutAllocation(t *testing.T) {
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
		VALUES (35, 'SO-PRODUCED-BATCH-NO-ALLOC-SHIP', '2026-06-06', 3, 1, 2, 1, (SELECT id FROM %s.order_process_statuses WHERE name='生产完成' LIMIT 1), 100, false);
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)
		VALUES (35,1,7,'橘皮乌龙',2,'袋','454g',50,100);
	`, schema, schema, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	body, _ := json.Marshal(map[string]any{"tracking_no": "SF-PRODUCED-BATCH-001"})
	req := httptest.NewRequest(http.MethodPost, "/api/orders/35/shipping-tracking", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/orders/35/shipping-tracking status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var remainingG, remainingUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g,remaining_units FROM %s.stock_batches WHERE id=101`, schema)).Scan(&remainingG, &remainingUnits); err != nil {
		t.Fatalf("query finished stock batch: %v", err)
	}
	if remainingG != 0 || remainingUnits != 0 {
		t.Fatalf("oldest finished batch remaining = %dg/%d units, want 0/0", remainingG, remainingUnits)
	}
	var nextRemainingG, nextRemainingUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g,remaining_units FROM %s.stock_batches WHERE id=102`, schema)).Scan(&nextRemainingG, &nextRemainingUnits); err != nil {
		t.Fatalf("query next finished stock batch: %v", err)
	}
	if nextRemainingG != 908 || nextRemainingUnits != 2 {
		t.Fatalf("newer finished batch remaining = %dg/%d units, want 908/2", nextRemainingG, nextRemainingUnits)
	}
	var deductionCount, ledgerCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.order_stock_deductions WHERE order_id=35 AND batch_code='FP-OLD-454' AND deducted_g=908`, schema)).Scan(&deductionCount); err != nil {
		t.Fatalf("query batch deductions: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='sales_order_shipment' AND source_doc_id=35 AND source_batch_code='FP-OLD-454' AND qty_change_g=-908`, schema)).Scan(&ledgerCount); err != nil {
		t.Fatalf("query batch ledger: %v", err)
	}
	if deductionCount != 1 || ledgerCount != 1 {
		t.Fatalf("batch deduction=%d ledger=%d, want 1/1", deductionCount, ledgerCount)
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
	mustExecOrderAPITestSQL(t, ctx, pool, orderAPIProductionConfigDDL(schema))
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
		INSERT INTO %[1]s.company_departments(id,name,active) VALUES (1,'销售',true);
		INSERT INTO %[1]s.company_employees(id,name,phone,department_id,account_type,active) VALUES (5,'销售小王','13800000005',1,'internal_employee',true);
		INSERT INTO %[1]s.customers(id,name,customer_type,contact,phone,address,active,default_source_id,default_order_type_id,responsible_employee_id) VALUES (3,'测试客户','wholesale','测试收件人','13800000000','杭州市测试路',true,1,2,5);
		INSERT INTO %[1]s.sources(id,name) VALUES (1,'小程序');
		INSERT INTO %[1]s.order_types(id,name) VALUES (1,'批发订单'),(2,'零售订单');
		INSERT INTO %[1]s.pay_statuses(id,name) VALUES (1,'未付款'),(2,'已付款');
		INSERT INTO %[1]s.ship_statuses(id,name) VALUES (1,'未发货');
		INSERT INTO %[1]s.order_process_statuses(id,name,sort,active) VALUES (1,'待处理',10,true),(2,'库存待发货',33,true);
		SELECT setval(pg_get_serial_sequence('%[1]s.order_process_statuses','id'), (SELECT COALESCE(MAX(id),1) FROM %[1]s.order_process_statuses));
		INSERT INTO %[1]s.products(id,name,default_price,active,retail_price_227g,retail_price_250g)
		VALUES (7,'橘皮乌龙',50,true,50,56);
	`, schema))
}

func seedOrderAPITestConcreteUnitConversion(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.products
		SET unit_rule_override_json=jsonb_build_object(
			'default_sales_unit','件',
			'inventory_unit','件',
			'unit_conversion_json',jsonb_build_object('件',jsonb_build_object('件',1))::text
		)
		WHERE id=7;
	`, schema))
}

func seedOrderAPIResponsibleData(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.company_departments(id,name,active) VALUES (1,'销售',true) ON CONFLICT (id) DO NOTHING;
		INSERT INTO %[1]s.company_employees(id,name,phone,department_id,account_type,active)
		VALUES (5,'销售小王','13800000005',1,'internal_employee',true)
		ON CONFLICT (id) DO NOTHING;
		INSERT INTO %[1]s.customers(id,name,contact,phone,address,active,responsible_employee_id) VALUES (4,'渠道伙伴A','代理老张','13800000004','上海市渠道路',true,5);
	`, schema))
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
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES (7,0,0,454,'finished_goods',4,0);
	`, schema, schema, schema))
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
	CREATE TABLE %s.product_unit_templates (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		inventory_unit TEXT NOT NULL DEFAULT '',
		quote_unit TEXT NOT NULL DEFAULT '',
		order_unit TEXT NOT NULL DEFAULT '',
		unit_conversion_json JSONB NOT NULL DEFAULT '{}'::jsonb,
		sales_specs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
		integer_unit BOOLEAN NOT NULL DEFAULT false,
		active BOOLEAN NOT NULL DEFAULT true,
		deleted_at TIMESTAMPTZ
	);
	CREATE TABLE %s.product_config_templates (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		inventory_unit TEXT NOT NULL DEFAULT '',
		quote_unit TEXT NOT NULL DEFAULT '',
		order_unit TEXT NOT NULL DEFAULT '',
		unit_conversion_json JSONB NOT NULL DEFAULT '{}'::jsonb,
		integer_unit BOOLEAN NOT NULL DEFAULT false,
		unit_template_id BIGINT NOT NULL DEFAULT 0,
		special_attrs_schema_json JSONB NOT NULL DEFAULT '[]'::jsonb,
		active BOOLEAN NOT NULL DEFAULT true
	);
	CREATE TABLE %s.logistics_companies (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		sort INTEGER NOT NULL DEFAULT 0,
		active BOOLEAN NOT NULL DEFAULT true
	);
	CREATE TABLE %s.logistics_products (
		id BIGSERIAL PRIMARY KEY,
		company_id BIGINT NOT NULL DEFAULT 0,
		name TEXT NOT NULL DEFAULT '',
		sort INTEGER NOT NULL DEFAULT 0,
		active BOOLEAN NOT NULL DEFAULT true
	);
	CREATE TABLE %s.customers (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	customer_type TEXT NOT NULL DEFAULT '',
	company_name TEXT NOT NULL DEFAULT '',
	company_address TEXT NOT NULL DEFAULT '',
	company_phone TEXT NOT NULL DEFAULT '',
	contact TEXT NOT NULL DEFAULT '',
	phone TEXT NOT NULL DEFAULT '',
	address TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	default_source_id BIGINT,
	default_order_type_id BIGINT,
	responsible_employee_id BIGINT NOT NULL DEFAULT 0
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
	account_type TEXT NOT NULL DEFAULT 'internal_employee',
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
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
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
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
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
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL,
	warehouse TEXT NOT NULL DEFAULT 'finished_goods',
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, bom_spec_id, spec_g, warehouse)
);
CREATE TABLE %s.products (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	roast_level TEXT NOT NULL DEFAULT '',
	default_price NUMERIC NOT NULL DEFAULT 0,
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
	green_bean_type TEXT NOT NULL DEFAULT '',
	green_bean_bom_product_id BIGINT NOT NULL DEFAULT 0,
	product_kind TEXT NOT NULL DEFAULT 'roasted_bean',
	drip_bag_grams NUMERIC(12,3) NOT NULL DEFAULT 10,
	drip_box_bag_count INT NOT NULL DEFAULT 10,
		product_category_id BIGINT NOT NULL DEFAULT 0,
		unit_rule_override_json JSONB NOT NULL DEFAULT '{}'::jsonb,
		parent_product_id BIGINT NOT NULL DEFAULT 0,
		unit_template_id BIGINT NOT NULL DEFAULT 0,
		product_config_template_id BIGINT NOT NULL DEFAULT 0,
		auto_derived_sku BOOLEAN NOT NULL DEFAULT false,
		derived_unit_template_id BIGINT NOT NULL DEFAULT 0,
		derived_spec_key TEXT NOT NULL DEFAULT '',
		derived_spec_name TEXT NOT NULL DEFAULT '',
		derived_sales_unit TEXT NOT NULL DEFAULT '',
		derived_spec_status TEXT NOT NULL DEFAULT 'active',
		sku_name TEXT NOT NULL DEFAULT '',
		sku_code TEXT NOT NULL DEFAULT '',
		barcode TEXT NOT NULL DEFAULT '',
		spec_label TEXT NOT NULL DEFAULT '',
		net_content_qty NUMERIC NOT NULL DEFAULT 0,
		net_content_unit TEXT NOT NULL DEFAULT '',
		is_default_sku BOOLEAN NOT NULL DEFAULT false,
		default_sku_id BIGINT NOT NULL DEFAULT 0
	);
CREATE TABLE %s.product_categories (
	id BIGSERIAL PRIMARY KEY,
	parent_id BIGINT NOT NULL DEFAULT 0,
	name TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	inventory_unit TEXT NOT NULL DEFAULT '',
	quote_unit TEXT NOT NULL DEFAULT '',
	order_unit TEXT NOT NULL DEFAULT '',
	unit_conversion_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	integer_unit BOOLEAN NOT NULL DEFAULT false
);
CREATE TABLE %s.customer_product_aliases (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL,
	product_id BIGINT NOT NULL,
	display_name TEXT NOT NULL DEFAULT '',
	customer_item_code TEXT NOT NULL DEFAULT '',
	brand_name TEXT NOT NULL DEFAULT '',
	display_category_id BIGINT NOT NULL DEFAULT 0,
	sort_order INTEGER NOT NULL DEFAULT 0,
	include_in_price_list BOOLEAN NOT NULL DEFAULT true,
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %s.product_price_tiers (
	id BIGSERIAL PRIMARY KEY,
	product_id BIGINT,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
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
	publication_purpose TEXT NOT NULL DEFAULT 'factory_supply',
	product_type_category_id BIGINT NOT NULL DEFAULT 0,
	product_type_name TEXT NOT NULL DEFAULT '',
	classification_template_id BIGINT NOT NULL DEFAULT 0,
	classification_template_name TEXT NOT NULL DEFAULT '',
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
CREATE TABLE %s.customer_sku_public_usage (
	customer_id BIGINT PRIMARY KEY,
	use_public_sku BOOLEAN NOT NULL DEFAULT true,
	use_public_categories BOOLEAN NOT NULL DEFAULT false
);
CREATE TABLE %s.orders (
	id BIGSERIAL PRIMARY KEY,
	document_date DATE,
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
	logistics_company_id BIGINT NOT NULL DEFAULT 0,
	logistics_product_id BIGINT NOT NULL DEFAULT 0,
	payment_goods_amount NUMERIC NOT NULL DEFAULT 0,
	payment_shipping_amount NUMERIC NOT NULL DEFAULT 0,
	payment_voucher_asset_id BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.order_items (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT,
	line_no INTEGER,
	product_id BIGINT,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	customer_product_alias_id BIGINT NOT NULL DEFAULT 0,
	customer_product_display_name_snapshot TEXT NOT NULL DEFAULT '',
	customer_item_code_snapshot TEXT NOT NULL DEFAULT '',
	brand_name_snapshot TEXT NOT NULL DEFAULT '',
	product_code_snapshot TEXT NOT NULL DEFAULT '',
	product_name_snapshot TEXT NOT NULL DEFAULT '',
	price_tier_id BIGINT,
	price_overridden BOOLEAN NOT NULL DEFAULT false,
	bean_list_publication_id BIGINT,
	bean_list_version_no TEXT NOT NULL DEFAULT '',
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
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,
	need_g BIGINT NOT NULL DEFAULT 0,
	need_units BIGINT NOT NULL DEFAULT 0,
	batch_id BIGINT NOT NULL DEFAULT 0,
	batch_code TEXT NOT NULL DEFAULT '',
	allocated_g BIGINT NOT NULL DEFAULT 0,
	allocated_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.order_stock_deductions (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT NOT NULL REFERENCES %s.orders(id) ON DELETE CASCADE,
	product_id BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,
	batch_id BIGINT NOT NULL DEFAULT 0,
	batch_code TEXT NOT NULL DEFAULT '',
	deducted_g BIGINT NOT NULL DEFAULT 0,
	deducted_units BIGINT NOT NULL DEFAULT 0,
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(order_id, product_id, bom_spec_id, batch_code)
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
		`, schema, schema, schema, schema,
		schema, schema, schema, schema, schema, schema, schema, schema, schema, schema,
		schema, schema, schema, schema, schema, schema, schema, schema, schema, schema,
		schema, schema,
		schema, schema, schema, schema, schema, schema, schema, schema, schema, schema,
		schema, schema, schema, schema, schema, schema)
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

func orderAPIProductionConfigDDL(schema string) string {
	return fmt.Sprintf(`
CREATE TABLE %[1]s.product_production_configs (
	product_id BIGINT PRIMARY KEY,
	production_bom_id BIGINT NOT NULL DEFAULT 0,
	production_bom_version_id BIGINT NOT NULL DEFAULT 0,
	process_route_id BIGINT NOT NULL DEFAULT 0,
	industry_field_template_id BIGINT NOT NULL DEFAULT 0,
	expected_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0
);
CREATE TABLE %[1]s.product_production_config_fields (
	id BIGSERIAL PRIMARY KEY,
	product_id BIGINT NOT NULL,
	field_key TEXT NOT NULL DEFAULT '',
	label TEXT NOT NULL DEFAULT '',
	field_type TEXT NOT NULL DEFAULT 'text',
	unit TEXT NOT NULL DEFAULT '',
	value_text TEXT NOT NULL DEFAULT '',
	value_number NUMERIC(14,4),
	value_bool BOOLEAN,
	template_field_key TEXT NOT NULL DEFAULT '',
	required BOOLEAN NOT NULL DEFAULT false,
	options_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	show_in_price_list BOOLEAN NOT NULL DEFAULT true,
	sort_order INT NOT NULL DEFAULT 0
);
CREATE TABLE %[1]s.product_production_config_industry_templates (
	product_id BIGINT NOT NULL,
	template_id BIGINT NOT NULL,
	sort_order INT NOT NULL DEFAULT 0,
	PRIMARY KEY(product_id,template_id)
);
	`, schema)
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
