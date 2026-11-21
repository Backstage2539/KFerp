package customerportal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
)

func TestEmployeeOrderFirstPaintSkipsCatalogAndDocuments(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{listResult: &salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42}}}}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders/42?include_documents=false", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 || sales.orderFormCalls != 0 || sales.documentListCalls != 0 {
		t.Fatalf("status=%d catalog=%d documents=%d", rec.Code, sales.orderFormCalls, sales.documentListCalls)
	}
}
