package customerportal

import (
	"context"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	stockapp "orderapp/internal/application/stock"
	"testing"
)

type employeeOrderWarehouseFixture struct{ customerID int64 }

func (w *employeeOrderWarehouseFixture) ListWarehouses(_ context.Context, q stockapp.WarehouseListQuery) ([]stockapp.WarehouseRow, error) {
	w.customerID = q.CustomerID
	return []stockapp.WarehouseRow{{Code: "OK", Name: "客户仓", Active: true, CustomerID: 3, Kind: "finished"}, {Code: "FOREIGN", Active: true, CustomerID: 9, Kind: "finished"}, {Code: "RAW", Active: true, CustomerID: 3, Kind: "raw"}, {Code: "OFF", CustomerID: 3, Kind: "finished"}}, nil
}
func TestEmployeeRetainedOrderOtherFee(t *testing.T) {
	var req miniEmployeeOrderRequest
	if err := json.Unmarshal([]byte(`{"order_date":"2026-11-20","delivery_mode":"customer_stock","delivery_warehouse_code":"OK","other_fee":"18.50","other_fee_note":"包装"}`), &req); err != nil {
		t.Fatal(err)
	}
	cmd, err := miniEmployeeSaveOrderCommand(req, "worker", 0)
	if err != nil || cmd.DeliveryMode != "customer_stock" || cmd.DeliveryWarehouseCode != "OK" || cmd.OtherFee == nil || *cmd.OtherFee != 18.5 || cmd.OtherFeeNote == nil || *cmd.OtherFeeNote != "包装" {
		t.Fatalf("retained order fields lost: %+v, %v", cmd, err)
	}
}
func TestEmployeeOrderFormIncludesOnlyCustomerFinishedWarehouses(t *testing.T) {
	form := salesapp.OrderFormData{Customers: []salesapp.CustomerOption{{ID: 3}}}
	w := &employeeOrderWarehouseFixture{}
	e := echo.New()
	registerMiniEmployeeAPI(e, employeePortalService(), &miniEmployeeSalesFake{orderFormResult: &form}, Dependencies{EmployeeWarehouses: w})
	req := httptest.NewRequest("GET", "/api/mini/employee/order-form?customer_id=3", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var resp struct {
		Warehouses []struct{ Code, Name string } `json:"delivery_warehouses"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || w.customerID != 3 || len(resp.Warehouses) != 1 || resp.Warehouses[0].Code != "OK" {
		t.Fatalf("warehouses: %d %s", rec.Code, rec.Body.String())
	}
}
