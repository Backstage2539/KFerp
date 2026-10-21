package customerportal

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
)

type miniEmployeeVoucherSalesFake struct {
	miniEmployeeSalesFake
	uploadCalls   int
	uploadCommand salesapp.SaveSalesOrderAssetCommand
}

func (f *miniEmployeeVoucherSalesFake) SaveSalesOrderAsset(_ context.Context, cmd salesapp.SaveSalesOrderAssetCommand) (salesapp.SalesOrderAsset, error) {
	f.uploadCalls++
	f.uploadCommand = cmd
	return salesapp.SalesOrderAsset{ID: 8, Kind: cmd.Kind, Filename: cmd.Filename}, nil
}

func TestMiniEmployeePaymentVoucherUploadUsesEmployeeScopeAndActor(t *testing.T) {
	for _, mode := range []string{"allowed", "anonymous", "customer", "read-only", "other-order", "void"} {
		t.Run(mode, func(t *testing.T) {
			sales := &miniEmployeeVoucherSalesFake{miniEmployeeSalesFake: miniEmployeeSalesFake{listResult: &salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, CustomerID: 8}}}}}
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
			if mode == "void" {
				sales.listResult.Rows[0].IsVoid = true
			}
			e := echo.New()
			registerMiniEmployeePaymentVoucherAPI(e, portal, sales, t.TempDir())
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			file, err := form.CreateFormFile("file", "receipt.png")
			if err != nil {
				t.Fatal(err)
			}
			_, _ = file.Write([]byte("receipt"))
			_ = form.Close()
			req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/orders/42/payment-voucher", &body)
			req.Header.Set(echo.HeaderContentType, form.FormDataContentType())
			if mode != "anonymous" {
				req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if mode != "allowed" {
				if rec.Code < 400 || sales.uploadCalls != 0 {
					t.Fatalf("status=%d uploads=%d body=%s", rec.Code, sales.uploadCalls, rec.Body.String())
				}
				return
			}
			var result struct {
				Asset salesapp.SalesOrderAsset `json:"asset"`
			}
			if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &result) != nil || result.Asset.ID != 8 || sales.uploadCalls != 1 || sales.uploadCommand.Actor != "mini-employee:7:销售甲" || sales.uploadCommand.Kind != "payment_voucher" {
				t.Fatalf("status=%d cmd=%+v body=%s", rec.Code, sales.uploadCommand, rec.Body.String())
			}
			if sales.listQuery.Scope != "mine" || sales.listQuery.EmployeeID != 7 {
				t.Fatalf("query=%+v", sales.listQuery)
			}
		})
	}
}
