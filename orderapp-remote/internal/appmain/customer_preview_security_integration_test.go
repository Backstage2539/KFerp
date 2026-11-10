package appmain

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	fulfillment "orderapp/internal/application/customerfulfillment"
	fulfillmentpg "orderapp/internal/infrastructure/postgres/customerfulfillment"
	portalhttp "orderapp/internal/interfaces/http/customerportal"
)

// This fixture uses the same router registration and real PostgreSQL services
// as the server, with a unique schema in ORDERAPP_TEST_DATABASE_URL.
func TestCustomerPreviewRealRoutesDenyWritesAndCrossCustomerAccess(t *testing.T) {
	f := newSampleFixture(t)
	f.e = echo.New()
	portalhttp.RegisterRoutes(f.e, portalhttp.Dependencies{
		CustomerPortal:      f.portal,
		CustomerFulfillment: fulfillment.NewService(fulfillmentpg.NewRepository(f.pool, f.schema)),
	})
	customerA := f.id(`INSERT INTO %s.customers(name,active) VALUES('预览客户甲',true) RETURNING id`)
	customerB := f.id(`INSERT INTO %s.customers(name,active) VALUES('隔离客户乙',true) RETURNING id`)
	f.exec(`INSERT INTO %s.customer_portal_profiles(customer_id,enabled) VALUES($1,true),($2,true)`, customerA, customerB)
	f.exec(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled,config_json) VALUES($1,'product_order',true,'{}'),($1,'settlement',true,'{}'),($2,'product_order',true,'{}'),($2,'settlement',true,'{}')`, customerA, customerB)
	orderA := f.id(`INSERT INTO %s.orders(order_no,customer_id,is_void,order_date) VALUES('PREVIEW-A',$1,false,CURRENT_DATE) RETURNING id`, customerA)
	orderB := f.id(`INSERT INTO %s.orders(order_no,customer_id,is_void,order_date) VALUES('SECRET-B',$1,false,CURRENT_DATE) RETURNING id`, customerB)
	billA := f.id(`INSERT INTO %s.customer_settlement_batches(customer_id,settlement_no,status,total_amount,confirmed_at) VALUES($1,'BILL-A','confirmed',10,now()) RETURNING id`, customerA)
	billB := f.id(`INSERT INTO %s.customer_settlement_batches(customer_id,settlement_no,status,total_amount,confirmed_at) VALUES($1,'BILL-B','confirmed',30,now()) RETURNING id`, customerB)

	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		f.e.ServeHTTP(rec, req)
		return rec
	}
	start := call("POST", "/api/mini/employee/customer-preview", f.adminToken, fmt.Sprintf(`{"customer_id":%d}`, customerA))
	if start.Code != 200 {
		t.Fatalf("start preview: %d %s", start.Code, start.Body.String())
	}
	var issued struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(start.Body.Bytes(), &issued); err != nil || issued.Token == "" {
		t.Fatalf("preview token: %+v %v", issued, err)
	}
	if strings.Contains(start.Body.String(), f.adminToken) || issued.Token == f.adminToken {
		t.Fatal("preview response exposed the employee token")
	}
	beforeOrders := f.id(`SELECT count(*) FROM %s.orders`)
	beforeAddresses := f.id(`SELECT count(*) FROM %s.customer_recipient_addresses`)
	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/mini/product-orders"},
		{"POST", "/api/mini/processing-requests"},
		{"POST", fmt.Sprintf("/api/mini/customer-account/settlements/%d/confirm", billA)},
		{"POST", "/api/mini/recipient-addresses"},
		{"PUT", "/api/mini/recipient-addresses/1"},
		{"DELETE", "/api/mini/recipient-addresses/1"},
		{"PUT", fmt.Sprintf("/api/mini/employee/orders/%d", orderA)},
		{"GET", "/api/mini/employee/orders"},
	} {
		rec := call(tc.method, tc.path, issued.Token, `{}`)
		if rec.Code != 403 {
			t.Errorf("%s %s: %d %s, expected 403", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
	if f.id(`SELECT count(*) FROM %s.orders`) != beforeOrders || f.id(`SELECT count(*) FROM %s.customer_recipient_addresses`) != beforeAddresses {
		t.Fatal("a forbidden preview request modified business records")
	}
	orders := call("GET", fmt.Sprintf("/api/mini/services/orders?customer_id=%d", customerB), issued.Token, "")
	if orders.Code != 200 || !strings.Contains(orders.Body.String(), "PREVIEW-A") || strings.Contains(orders.Body.String(), "SECRET-B") {
		t.Fatalf("order customer scope: %d %s", orders.Code, orders.Body.String())
	}
	foreignOrder := call("GET", fmt.Sprintf("/api/mini/services/orders?order_id=%d&customer_id=%d", orderB, customerB), issued.Token, "")
	if foreignOrder.Code != 200 || strings.Contains(foreignOrder.Body.String(), "SECRET-B") {
		t.Fatalf("foreign order leaked: %d %s", foreignOrder.Code, foreignOrder.Body.String())
	}
	ownBill := call("GET", fmt.Sprintf("/api/mini/customer-issued-bills/%d?customer_id=%d", billA, customerB), issued.Token, "")
	if ownBill.Code != 200 || !strings.Contains(ownBill.Body.String(), "BILL-A") {
		t.Fatalf("own bill unavailable: %d %s", ownBill.Code, ownBill.Body.String())
	}
	foreignBill := call("GET", fmt.Sprintf("/api/mini/customer-issued-bills/%d?customer_id=%d", billB, customerB), issued.Token, "")
	if foreignBill.Code != 404 || strings.Contains(foreignBill.Body.String(), "BILL-B") {
		t.Fatalf("foreign bill leaked: %d %s", foreignBill.Code, foreignBill.Body.String())
	}
	listBills := call("GET", fmt.Sprintf("/api/mini/customer-issued-bills?customer_id=%d", customerB), issued.Token, "")
	if listBills.Code != 200 || !strings.Contains(listBills.Body.String(), "BILL-A") || strings.Contains(listBills.Body.String(), "BILL-B") {
		t.Fatalf("bill customer scope: %d %s", listBills.Code, listBills.Body.String())
	}
	admin := call("GET", "/api/mini/me", f.adminToken, "")
	if admin.Code != 200 || !strings.Contains(admin.Body.String(), `"account_type":"employee"`) {
		t.Fatalf("original employee session changed: %d %s", admin.Code, admin.Body.String())
	}
	f.exec(`DELETE FROM %s.employee_roles WHERE employee_id=$1 AND role_code='admin'`, f.admin.EmployeeID)
	revoked := call("GET", "/api/mini/me", issued.Token, "")
	if revoked.Code != 401 {
		t.Fatalf("revoked admin preview: %d %s", revoked.Code, revoked.Body.String())
	}
	f.exec(`INSERT INTO %s.employee_roles(employee_id,role_code) VALUES($1,'admin')`, f.admin.EmployeeID)
	if rec := call("GET", "/api/mini/me", issued.Token, ""); rec.Code != 401 {
		t.Fatalf("revoked preview revived after role restoration: %d %s", rec.Code, rec.Body.String())
	}
}
