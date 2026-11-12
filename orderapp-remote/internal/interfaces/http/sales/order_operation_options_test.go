package sales

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"
	"testing"
)

type operationOptionsRepo struct {
	salesapp.Repository
	calls int
}

func TestOperationOptionsRealMiddlewareRejectsCustomerThroughERP(t *testing.T) {
	for _, tt := range []struct {
		name, account, permission, path string
		status, calls                   int
	}{
		{"customer ERP bypass", support.AccountTypeChannelCustomer, "orders.write", "/api/order-operation-options?product_id=8", 403, 0},
		{"employee permitted", support.AccountTypeInternalEmployee, "orders.write", "/api/order-operation-options?product_id=8", 200, 1},
		{"employee without write", support.AccountTypeInternalEmployee, "orders.read", "/api/order-operation-options?product_id=8", 403, 0},
		{"customer portal visible", support.AccountTypeChannelCustomer, "customer_processing.read", "/api/customer-processing/portal/order/operation-options?service=direct_ship&product_id=7", 200, 1},
		{"customer portal hidden", support.AccountTypeChannelCustomer, "customer_processing.read", "/api/customer-processing/portal/order/operation-options?service=direct_ship&product_id=8", 403, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &operationOptionsRepo{}
			authz := &orderAPIAuthzService{actor: authzapp.Actor{AccountType: tt.account, Permissions: []string{tt.permission}}}
			h := orderAPIHandler{sales: salesapp.NewService(repo), customerScope: workspaceTestScope{}, authz: authz}
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("employee_id", int64(1)); return next(c) }
			})
			e.Use(support.AuthorizationMiddleware(authz))
			e.GET("/api/order-operation-options", h.operationOptions)
			e.GET("/api/customer-processing/portal/order/operation-options", h.customerOperationOptions)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != tt.status || repo.calls != tt.calls {
				t.Fatalf("status=%d calls=%d want %d/%d: %s", rec.Code, repo.calls, tt.status, tt.calls, rec.Body.String())
			}
		})
	}
}

func TestERPOperationOptionsHonorCustomerScopedEntry(t *testing.T) {
	repo := &operationOptionsRepo{}
	h := orderAPIHandler{sales: salesapp.NewService(repo), customerScope: workspaceTestScope{}}
	e := echo.New()
	e.GET("/api/order-operation-options", h.operationOptions)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("customer_fulfillment_order_scope_limited", true)
			return next(c)
		}
	})
	for _, tt := range []struct {
		query  string
		status int
	}{{"product_id=7", 200}, {"product_id=8", 403}, {"product_id=7&customer_id=4", 403}, {"product_id=7&bom_spec_id=oops", 400}} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/order-operation-options?"+tt.query, nil))
		if rec.Code != tt.status {
			t.Fatalf("%s got %d want %d: %s", tt.query, rec.Code, tt.status, rec.Body.String())
		}
	}
	if repo.calls != 1 {
		t.Fatalf("scope bypassed: %d calls", repo.calls)
	}
}

type operationOptionsMallScope struct{ workspaceTestScope }

func (operationOptionsMallScope) CustomerWorkspace(_ context.Context, employee, customer int64, page string, _ int64) (map[string]any, error) {
	if employee != 1 || customer != 0 || page != "mall" {
		return nil, fmt.Errorf("商城未开通")
	}
	return map[string]any{"customer_id": int64(3)}, nil
}

type operationOptionsMall struct {
	CustomerMallService
	calls int
}

func (m *operationOptionsMall) GetERPMallOperationOptions(_ context.Context, customer, id int64) ([]salesapp.OrderOperationOption, error) {
	m.calls++
	if customer != 3 || id != 42 {
		return nil, fmt.Errorf("商城商品不可用")
	}
	return []salesapp.OrderOperationOption{}, nil
}
func TestMallOperationOptionsEnforceCustomerContextAndProductIdentity(t *testing.T) {
	mall := &operationOptionsMall{}
	e := echo.New()
	registerCustomerMallOperationOptionsRoute(e, Dependencies{CustomerScope: operationOptionsMallScope{}, CustomerMall: mall})
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("employee_id", int64(1)); return next(c) }
	})
	for _, tt := range []struct {
		query  string
		status int
	}{{"mall_product_id=42", 200}, {"mall_product_id=42&customer_id=4", 403}, {"mall_product_id=43", 400}, {"mall_product_id=oops", 400}} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/customer-processing/portal/mall/operation-options?"+tt.query, nil))
		if rec.Code != tt.status {
			t.Fatalf("%s got %d want %d: %s", tt.query, rec.Code, tt.status, rec.Body.String())
		}
	}
	if mall.calls != 2 {
		t.Fatalf("scope or parameter validation bypassed: %d", mall.calls)
	}
}

func (r *operationOptionsRepo) OrderForm(context.Context, int64) (salesapp.OrderFormData, error) {
	return salesapp.OrderFormData{Products: []salesapp.ProductOption{{ID: 7, Name: "本客户商品", CustomerID: 3, Visibility: "customer_only"}, {ID: 8, Name: "其他客户商品", CustomerID: 4, Visibility: "customer_only"}}}, nil
}
func (r *operationOptionsRepo) OrderOperationOptions(context.Context, int64, int64, int64) ([]salesapp.OrderOperationOption, error) {
	r.calls++
	return []salesapp.OrderOperationOption{{OutputType: "product", OutputID: 7, OutputName: "本客户商品", RouteID: 31, StepID: 32, OperationID: 33, OperationName: "包装"}}, nil
}

func TestCustomerOperationOptionsEnforceCapabilityCustomerAndProductVisibility(t *testing.T) {
	repo := &operationOptionsRepo{}
	h := orderAPIHandler{sales: salesapp.NewService(repo), customerScope: workspaceTestScope{}}
	e := echo.New()
	e.GET("/api/customer-processing/portal/order/operation-options", h.customerOperationOptions)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("employee_id", int64(1)); return next(c) }
	})
	for _, tt := range []struct {
		query  string
		status int
	}{{"service=direct_ship&product_id=7", 200}, {"service=direct_ship&product_id=8", 403}, {"service=direct_ship&product_id=7&customer_id=4", 403}, {"service=product_order&product_id=7", 403}, {"service=processing&product_id=7", 400}, {"service=direct_ship&product_id=oops", 400}} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/customer-processing/portal/order/operation-options?"+tt.query, nil))
		if rec.Code != tt.status {
			t.Fatalf("%s got %d want %d: %s", tt.query, rec.Code, tt.status, rec.Body.String())
		}
	}
	if repo.calls != 1 {
		t.Fatalf("hidden products must not reach graph query: %d calls", repo.calls)
	}
}
