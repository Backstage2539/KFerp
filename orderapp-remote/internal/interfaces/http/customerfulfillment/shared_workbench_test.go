package customerfulfillment

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	app "orderapp/internal/application/customerfulfillment"
	"testing"
)

func TestSharedWorkbenchUsesBoundCustomerAndChecksCapability(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{customerOverviewResult: app.CustomerPortalOverview{CustomerID: 149, CustomerName: "A", Capabilities: []string{"settlement"}}}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("employee_id", int64(23)); return next(c) }
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/api/customer-processing/portal/fulfillment/overview?customer_id=999", 200},
		{"/api/customer-processing/portal/fulfillment/billing/unbilled?customer_id=999", 200},
	} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != tc.want {
			t.Fatalf("%s: %d %s", tc.path, rec.Code, rec.Body.String())
		}
	}
	if svc.overviewQuery.CustomerID != 149 || svc.unbilledQuery.CustomerID != 149 {
		t.Fatalf("scope leaked: overview=%+v billing=%+v", svc.overviewQuery, svc.unbilledQuery)
	}
	svc.customerOverviewResult.Capabilities = nil
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("GET", "/api/customer-processing/portal/fulfillment/billing/unbilled", nil))
	if rec.Code != 403 {
		t.Fatalf("missing capability: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSharedWorkbenchTwoCustomersRemainIsolated(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("employee_id", int64(23)); return next(c) }
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})
	for _, id := range []int64{149, 150} {
		svc.customerOverviewResult = app.CustomerPortalOverview{CustomerID: id, Capabilities: []string{"settlement"}}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest("GET", "/api/customer-processing/portal/fulfillment/billing/unbilled?customer_id=999", nil))
		if rec.Code != 200 || svc.unbilledQuery.CustomerID != id {
			t.Fatalf("customer %d: %d scope=%d", id, rec.Code, svc.unbilledQuery.CustomerID)
		}
	}
}
