package customerportal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	customerportalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

type miniEmployeeSalesFake struct {
	listQuery salesapp.OrderListQuery
	save      salesapp.SaveOrderCommand
}

func (f *miniEmployeeSalesFake) ListOrders(_ context.Context, query salesapp.OrderListQuery) (salesapp.OrderListResult, error) {
	f.listQuery = query
	return salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 1, OrderNo: "SO-1"}}}, nil
}
func (f *miniEmployeeSalesFake) OrderForm(context.Context, int64) (salesapp.OrderFormData, error) {
	return salesapp.OrderFormData{Today: "2026-07-30", Customers: []salesapp.CustomerOption{{ID: 8, Name: "客户A"}}}, nil
}
func (f *miniEmployeeSalesFake) SaveOrder(_ context.Context, cmd salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error) {
	f.save = cmd
	return salesapp.SaveOrderResult{OrderID: 9, OrderNo: "SO-9"}, nil
}

func employeePortalService() fakeService {
	return fakeService{me: customerportalapp.CurrentContext{
		AccountType: "employee", EmployeeID: 7, EmployeeName: "销售甲",
		Roles: []string{"sales"}, Permissions: []string{"orders.read", "orders.write"},
	}}
}

func TestMiniEmployeeOrderListScopesSalesToOwnOrders(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders?q=SO", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"order_no":"SO-1"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if sales.listQuery.Scope != "mine" || sales.listQuery.EmployeeID != 7 || sales.listQuery.Q != "SO" {
		t.Fatalf("query=%+v", sales.listQuery)
	}
}

func TestMiniEmployeeOrderCreateWritesResponsibleEmployee(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	body := `{"order_date":"2026-07-30","customer_id":8,"items":[{"product_id":3,"name":"咖啡豆","qty":2,"spec_g":454,"unit_price":68}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"order_no":"SO-9"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if sales.save.ResponsibleID != 7 || sales.save.ResponsibleType != "employee" || !sales.save.OrderDate.Equal(time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("save=%+v", sales.save)
	}
}

func TestMiniEmployeeAPIRejectsCustomerAccount(t *testing.T) {
	e := echo.New()
	registerMiniEmployeeAPI(e, fakeService{me: customerportalapp.CurrentContext{AccountType: "customer"}}, &miniEmployeeSalesFake{})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
