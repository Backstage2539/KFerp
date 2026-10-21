package customerportal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
)

type miniEmployeeStatusSalesFake struct {
	miniEmployeeSalesFake
	statusCommand salesapp.UpdateOrderStatusCommand
	statusCalls   int
	statusErr     error
}

func (f *miniEmployeeStatusSalesFake) UpdateOrderStatus(_ context.Context, cmd salesapp.UpdateOrderStatusCommand) error {
	f.statusCalls++
	f.statusCommand = cmd
	return f.statusErr
}

func miniEmployeeStatusFixture() *miniEmployeeStatusSalesFake {
	return &miniEmployeeStatusSalesFake{miniEmployeeSalesFake: miniEmployeeSalesFake{
		listResult:      &salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, OrderNo: "SO-42", CustomerID: 8, ShipStatus: "已发货"}}},
		orderFormResult: &salesapp.OrderFormData{EditData: &salesapp.OrderEditData{ID: 42, CustomerID: 8, EditRevision: "rev-status", PayStatusID: 1, ShipStatusID: 2}, PayStatuses: []salesapp.Option{{ID: 1, Name: "未付款"}, {ID: 2, Name: "已付款"}}, ShipStatuses: []salesapp.Option{{ID: 1, Name: "未发货"}, {ID: 2, Name: "已发货"}}},
		editability:     salesapp.OrderEditability{BlockReason: "订单已发货，不能再编辑"},
	}}
}

func TestMiniEmployeeOrderStatusUpdatesShippedOrderWithoutSavingItems(t *testing.T) {
	sales := miniEmployeeStatusFixture()
	e := echoForMiniStatus(sales)
	rec := miniStatusRequest(e, http.MethodPut, `{"edit_revision":"rev-status","pay_status_id":2,"ship_status_id":2,"payment_method":"微信支付","prepayment_amount":20,"payment_goods_amount":90,"payment_shipping_amount":10,"payment_voucher_asset_id":12}`)
	if rec.Code != http.StatusOK || sales.statusCalls != 1 || sales.saveCalls != 0 {
		t.Fatalf("status=%d statusCalls=%d saves=%d body=%s", rec.Code, sales.statusCalls, sales.saveCalls, rec.Body.String())
	}
	cmd := sales.statusCommand
	if cmd.OrderID != 42 || cmd.Actor != "mini-employee:7:销售甲" || cmd.ExpectedEditRevision != "rev-status" || cmd.PayStatusID != 2 || cmd.ShipStatusID != 2 || cmd.PaymentMethod != "微信支付" || cmd.PrepaymentAmount == nil || *cmd.PrepaymentAmount != 20 {
		t.Fatalf("command=%+v", cmd)
	}
	if cmd.PaymentGoodsAmount == nil || *cmd.PaymentGoodsAmount != 90 || cmd.PaymentShippingAmount == nil || *cmd.PaymentShippingAmount != 10 || cmd.PaymentVoucherAssetID == nil || *cmd.PaymentVoucherAssetID != 12 {
		t.Fatalf("receipt command=%+v", cmd)
	}
}

func TestMiniEmployeeOrderStatusRejectsUnrelatedFieldsAndStaleRevision(t *testing.T) {
	for _, body := range []string{
		`{"edit_revision":"rev-status","pay_status_id":2,"ship_status_id":2,"payment_method":"现金","items":[]}`,
		`{"edit_revision":"old","pay_status_id":2,"ship_status_id":2,"payment_method":"现金"}`,
		`{"pay_status_id":2,"ship_status_id":2,"payment_method":"现金"}`,
	} {
		sales := miniEmployeeStatusFixture()
		rec := miniStatusRequest(echoForMiniStatus(sales), http.MethodPut, body)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if sales.statusCalls != 0 || sales.saveCalls != 0 {
			t.Fatal("invalid request wrote order")
		}
	}
}

func TestMiniEmployeeOrderStatusDetailExposesSeparateCapabilityAndOptions(t *testing.T) {
	sales := miniEmployeeStatusFixture()
	rec := miniStatusRequest(echoForMiniStatus(sales), http.MethodGet, "")
	var result struct {
		CanEdit             bool `json:"can_edit"`
		CanUpdateStatus     bool `json:"can_update_status"`
		CanUpdateShipStatus bool `json:"can_update_ship_status"`
		StatusOptions       struct {
			PayStatuses  []salesapp.Option `json:"pay_statuses"`
			ShipStatuses []salesapp.Option `json:"ship_statuses"`
		} `json:"status_options"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &result) != nil || result.CanEdit || !result.CanUpdateStatus || !result.CanUpdateShipStatus || len(result.StatusOptions.PayStatuses) != 2 || len(result.StatusOptions.ShipStatuses) != 2 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniEmployeeOrderStatusDetailPreservesLegacyUnsetStatus(t *testing.T) {
	sales := miniEmployeeStatusFixture()
	sales.orderFormResult.EditData.ShipStatusID = 0
	rec := miniStatusRequest(echoForMiniStatus(sales), http.MethodGet, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":0,"name":"已发货"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniEmployeeOrderStatusCannotBypassVoidOrFulfillmentShipping(t *testing.T) {
	for _, mode := range []string{"void", "managed", "pending"} {
		sales := miniEmployeeStatusFixture()
		if mode == "void" {
			sales.listResult.Rows[0].IsVoid = true
		}
		if mode == "managed" || mode == "pending" {
			sales.listResult.Rows[0].ConfirmationRequired = true
			sales.listResult.Rows[0].ConfirmationStatus = "accepted"
		}
		if mode == "pending" {
			sales.listResult.Rows[0].ConfirmationStatus = "pending"
		}
		rec := miniStatusRequest(echoForMiniStatus(sales), http.MethodPut, `{"edit_revision":"rev-status","pay_status_id":2,"ship_status_id":1,"payment_method":"现金"}`)
		if rec.Code != http.StatusConflict || sales.statusCalls != 0 {
			t.Fatalf("mode=%s status=%d body=%s", mode, rec.Code, rec.Body.String())
		}
	}
	// An accepted fulfillment order may record payment with shipping unchanged.
	sales := miniEmployeeStatusFixture()
	sales.listResult.Rows[0].ConfirmationRequired = true
	sales.listResult.Rows[0].ConfirmationStatus = "accepted"
	rec := miniStatusRequest(echoForMiniStatus(sales), http.MethodPut, `{"edit_revision":"rev-status","pay_status_id":2,"ship_status_id":2,"payment_method":"现金"}`)
	if rec.Code != http.StatusOK || sales.statusCalls != 1 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniEmployeeOrderStatusRequiresEmployeePermissionAndOwnOrder(t *testing.T) {
	for _, mode := range []string{"anonymous", "customer", "read-only", "other-order"} {
		sales := miniEmployeeStatusFixture()
		portal := employeePortalService()
		if mode == "customer" {
			portal.me.AccountType = "customer"
		}
		if mode == "read-only" {
			portal.me.Permissions = []string{"orders.read"}
		}
		if mode == "other-order" {
			sales.listResult.Rows = nil
		}
		e := echo.New()
		registerMiniEmployeeAPI(e, portal, sales)
		req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/orders/42/status", strings.NewReader(`{"edit_revision":"rev-status","pay_status_id":2,"ship_status_id":2,"payment_method":"现金"}`))
		req.Header.Set("Content-Type", "application/json")
		if mode != "anonymous" {
			req.Header.Set("Authorization", "Bearer token")
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code < 400 || sales.statusCalls != 0 {
			t.Fatalf("mode=%s status=%d body=%s", mode, rec.Code, rec.Body.String())
		}
		if mode == "other-order" && (sales.listQuery.Scope != "mine" || sales.listQuery.EmployeeID != 7) {
			t.Fatalf("query=%+v", sales.listQuery)
		}
	}
}

func echoForMiniStatus(sales *miniEmployeeStatusSalesFake) *echo.Echo {
	e := echo.New()
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	return e
}

func miniStatusRequest(e *echo.Echo, method, body string) *httptest.ResponseRecorder {
	path := "/api/mini/employee/orders/42/status"
	if method == http.MethodGet {
		path = "/api/mini/employee/orders/42"
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}
