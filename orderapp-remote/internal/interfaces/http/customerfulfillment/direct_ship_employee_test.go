package customerfulfillment

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	app "orderapp/internal/application/customerfulfillment"
	"strings"
	"testing"
)

type employeeDirectShipFake struct {
	*fakeCustomerFulfillmentService
	command app.MiniDirectShipCommand
	query   app.MiniDirectShipCatalogQuery
}

func (f *employeeDirectShipFake) MiniDirectShipCatalog(_ context.Context, q app.MiniDirectShipCatalogQuery) (app.MiniDirectShipCatalog, error) {
	f.query = q
	return app.MiniDirectShipCatalog{CurrentCustomerID: q.CustomerID}, nil
}
func (f *employeeDirectShipFake) PreviewMiniDirectShip(_ context.Context, c app.MiniDirectShipCommand) (app.MiniDirectShipPreview, error) {
	f.command = c
	return app.MiniDirectShipPreview{CanSubmit: true}, nil
}
func (f *employeeDirectShipFake) SubmitMiniDirectShip(_ context.Context, c app.MiniDirectShipCommand) (app.MiniDirectShipRequest, error) {
	f.command = c
	return app.MiniDirectShipRequest{ID: 73, RequestNo: "DSR-73"}, nil
}
func TestEmployeeDirectShipUsesScopedSharedService(t *testing.T) {
	svc := &employeeDirectShipFake{fakeCustomerFulfillmentService: &fakeCustomerFulfillmentService{}}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(23))
			c.Set("operator_employee", "内部员工")
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/api/customer-fulfillment/74/direct-ship/catalog?fulfillment_mode=stock&customer_id=999", "", 200},
		{"POST", "/api/customer-fulfillment/74/direct-ship/preview", `{"customer_id":999,"fulfillment_mode":"stock","items":[{"product_id":58,"bom_spec_id":69,"qty":12}]}`, 200},
		{"POST", "/api/customer-fulfillment/74/direct-ship/requests", `{"customer_id":999,"fulfillment_mode":"stock","idempotency_key":"request-1"}`, 201},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%s status=%d body=%s", tc.path, rec.Code, rec.Body.String())
		}
	}
	if svc.query.CustomerID != 74 || svc.query.FulfillmentMode != "stock" || svc.command.CustomerID != 74 || svc.command.EmployeeID != 23 || svc.command.MiniUserID != 0 || svc.command.Actor != "内部员工" {
		t.Fatalf("query=%+v command=%+v", svc.query, svc.command)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/0/direct-ship/catalog", nil))
	if rec.Code != 400 {
		t.Fatalf("invalid customer status=%d", rec.Code)
	}
}
