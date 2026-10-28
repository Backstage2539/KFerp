package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
	portalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"
	portalhttp "orderapp/internal/interfaces/http/customerportal"
)

type priceCatalogPrincipal struct {
	portalhttp.Service
	customerID   int64
	productOrder bool
}

func (p *priceCatalogPrincipal) Me(context.Context, string) (portalapp.CurrentContext, error) {
	return portalapp.CurrentContext{CurrentCustomerID: p.customerID, MiniUserID: 711, Capabilities: []portalapp.Capability{{Code: portalapp.CapabilityDirectShip, Enabled: true}, {Code: portalapp.CapabilityProductOrder, Enabled: p.productOrder}}}, nil
}

type priceCatalogSales struct {
	form  salesapp.OrderFormData
	calls int
}

func (s *priceCatalogSales) OrderForm(context.Context, int64) (salesapp.OrderFormData, error) {
	s.calls++
	return s.form, nil
}
func (s *priceCatalogSales) SaveOrder(context.Context, salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error) {
	return salesapp.SaveOrderResult{}, fmt.Errorf("catalog must not write an order")
}

func TestBoundCatalogPriceTablesSurviveEmptyStockAndMiniAPIScope(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.customers(id,name) VALUES(501,'PR711客户A'),(502,'PR711客户B');INSERT INTO %[1]s.warehouses(code,name,kind,customer_id) VALUES('PR711-A','客户A','finished',501),('PR711-B','客户B','finished',502);INSERT INTO %[1]s.customer_fulfillment_settings(customer_id,modes_json) VALUES(501,'[{"mode":"stock","enabled":true,"warehouse_code":"PR711-A"},{"mode":"on_demand","enabled":true}]'),(502,'[{"mode":"stock","enabled":true,"warehouse_code":"PR711-B"}]')`, schema)); err != nil {
		t.Fatal(err)
	}
	publication := func(owner, key, name, version string) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publications(list_type,version_no,status,owner_type,owner_key,publication_purpose,publication_table_name,publication_table_key,config_json,content_json) VALUES('commercial',$1,'published',$2,$3,'factory_supply',$4,$4,'{}','{}') RETURNING id`, schema), version, owner, key, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	bound := publication("customer", "501", "PR711已绑定代发表", "V1")
	newer := publication("customer", "501", "PR711未绑定新版", "V2")
	public := publication("official", "", "PR711指定公共录单表", "V3")
	foreign := publication("customer", "502", "PR711其他客户表", "V4")
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_order_price_table_bindings(customer_id,usage_code,product_type_key,publication_id) VALUES(501,'direct_ship','legacy:commercial',$1),(501,'product_order','legacy:commercial',$2)`, schema), bound, public); err != nil {
		t.Fatal(err)
	}
	r := NewRepository(pool, schema)
	svc := app.NewService(r)
	sales := &priceCatalogSales{form: salesapp.OrderFormData{BeanListVersionOptions: []salesapp.BeanListVersionOption{{ID: bound, CustomerID: 501, IsCustomerOwned: true, ListType: "commercial"}, {ID: newer, CustomerID: 501, IsCustomerOwned: true, ListType: "commercial"}, {ID: public, ListType: "commercial"}}}}
	svc.UseSalesOrderService(sales)
	principal := &priceCatalogPrincipal{customerID: 501}
	e := echo.New()
	portalhttp.RegisterRoutes(e, portalhttp.Dependencies{CustomerPortal: principal, CustomerFulfillment: svc})
	get := func(path string, want int) []byte {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer isolated-PR711")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("GET %s status=%d want=%d body=%s", path, rec.Code, want, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	readCatalog := func(path string, wantID int64) {
		t.Helper()
		var catalog app.MiniDirectShipCatalog
		raw := get(path, 200)
		if err := json.Unmarshal(raw, &catalog); err != nil {
			t.Fatal(err)
		}
		if catalog.CurrentCustomerID != principal.customerID || len(catalog.ProductFamilies) != 0 {
			t.Fatalf("wrong empty-stock scope: %s", raw)
		}
		if wantID == 0 {
			if len(catalog.PriceTables) != 0 {
				t.Fatalf("unbound customer inherited price table: %s", raw)
			}
		} else if len(catalog.PriceTables) != 1 || catalog.PriceTables[0].ID != wantID {
			t.Fatalf("bound price metadata lost or replaced: %s", raw)
		}
	}
	readCatalog(fmt.Sprintf("/api/mini/direct-ship/catalog?fulfillment_mode=stock&customer_id=502&publication_id=%d", foreign), bound)
	readCatalog("/api/mini/direct-ship/catalog?fulfillment_mode=on_demand", bound)
	// Stock dropship may read its purchase price table, but may not place a product order without that capability.
	calls := sales.calls
	raw := get("/api/mini/product-orders/catalog?customer_id=501", 403)
	if !strings.Contains(string(raw), "capability not enabled") || sales.calls != calls {
		t.Fatal("product order capability was broadened")
	}
	var stockPreview app.MiniOrderPriceTablePreview
	if err := json.Unmarshal(get("/api/mini/order-price-tables/preview?usage_code=product_order", 200), &stockPreview); err != nil {
		t.Fatal(err)
	}
	if len(stockPreview.PriceTables) != 1 || stockPreview.PriceTables[0].ID != public {
		t.Fatalf("stock preview wrong table: %+v", stockPreview)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fulfillment_settings SET modes_json='[{"mode":"stock","enabled":false},{"mode":"on_demand","enabled":true}]' WHERE customer_id=501`, schema)); err != nil {
		t.Fatal(err)
	}
	get("/api/mini/order-price-tables/preview?usage_code=product_order", 400)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fulfillment_settings SET modes_json='[{"mode":"stock","enabled":true,"warehouse_code":"PR711-A"},{"mode":"on_demand","enabled":true}]' WHERE customer_id=501`, schema)); err != nil {
		t.Fatal(err)
	}
	principal.productOrder = true
	readCatalog("/api/mini/product-orders/catalog?customer_id=502", public)
	var preview app.MiniOrderPriceTablePreview
	if err := json.Unmarshal(get("/api/mini/order-price-tables/preview?usage_code=product_order", 200), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.PriceTables) != 1 || preview.PriceTables[0].ID != public {
		t.Fatalf("product ordering used wrong binding: %+v", preview)
	}
	principal.customerID = 502
	readCatalog("/api/mini/direct-ship/catalog?fulfillment_mode=stock&customer_id=501", 0)
	// An invalid historical binding must not authorize another customer's table
	// or cause a fallback to a newer unbound publication.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_order_price_table_bindings SET publication_id=$1 WHERE customer_id=501 AND usage_code='direct_ship'`, schema), foreign); err != nil {
		t.Fatal(err)
	}
	principal.customerID = 501
	readCatalog("/api/mini/direct-ship/catalog?fulfillment_mode=stock", 0)
	if _, err := r.CustomerOrderPriceTables(ctx, 501, "direct_ship"); err == nil || !strings.Contains(err.Error(), "已失效") {
		t.Fatalf("invalid binding should be distinct from missing: %v", err)
	}
	if _, err := r.CustomerOrderPriceTables(ctx, 502, "direct_ship"); err == nil || !strings.Contains(err.Error(), "尚未") {
		t.Fatalf("missing binding should report not configured: %v", err)
	}
}
