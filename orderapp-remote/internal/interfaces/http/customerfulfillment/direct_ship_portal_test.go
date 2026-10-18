package customerfulfillment

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	app "orderapp/internal/application/customerfulfillment"
	portal "orderapp/internal/application/customerportal"
	"strings"
	"testing"
)

type portalAddressFake struct {
	*fakeSharedProcessingService
	customerID int64
	saved      portal.SaveCustomerRecipientAddressCommand
	deleted    portal.DeleteCustomerRecipientAddressCommand
}

func (f *employeeDirectShipFake) LoadFulfillmentSettings(_ context.Context, id int64) (app.FulfillmentSettings, error) {
	return app.FulfillmentSettings{CustomerID: id, Revision: 2}, nil
}

func (f *portalAddressFake) ListRecipientAddressesForCustomer(_ context.Context, id int64, _ string) ([]portal.CustomerRecipientAddress, error) {
	f.customerID = id
	return []portal.CustomerRecipientAddress{}, nil
}
func (f *portalAddressFake) SaveRecipientAddressForCustomer(_ context.Context, c portal.SaveCustomerRecipientAddressCommand) (portal.CustomerRecipientAddress, error) {
	f.saved = c
	return portal.CustomerRecipientAddress{ID: c.ID, CustomerID: c.CustomerID}, nil
}
func (f *portalAddressFake) DeleteRecipientAddressForCustomer(_ context.Context, c portal.DeleteCustomerRecipientAddressCommand) error {
	f.deleted = c
	return nil
}

func TestPortalDirectShipAndAddressesAlwaysResolveAuthenticatedCustomer(t *testing.T) {
	svc := &employeeDirectShipFake{fakeCustomerFulfillmentService: &fakeCustomerFulfillmentService{customerOverviewResult: app.CustomerPortalOverview{CustomerID: 307, Capabilities: []string{"direct_ship"}}}}
	addresses := &portalAddressFake{fakeSharedProcessingService: &fakeSharedProcessingService{}}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(23))
			c.Set("operator_employee", "验收员工")
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, SharedProcessing: addresses})
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/api/customer-processing/portal/direct-ship/catalog?customer_id=999&fulfillment_mode=stock", "", 200},
		{"GET", "/api/customer-processing/portal/direct-ship/settings?customer_id=999", "", 200},
		{"POST", "/api/customer-processing/portal/direct-ship/preview", `{"usage_code":"product_order","customer_id":999,"employee_id":999,"mini_user_id":999,"fulfillment_mode":"stock"}`, 200},
		{"POST", "/api/customer-processing/portal/direct-ship/requests", `{"usage_code":"product_order","customer_id":999,"employee_id":999,"mini_user_id":999,"fulfillment_mode":"stock"}`, 201},
		{"GET", "/api/customer-processing/portal/recipient-addresses?customer_id=999", "", 200},
		{"POST", "/api/customer-processing/portal/recipient-addresses", `{"id":999,"customer_id":999,"recipient_name":"测试"}`, 200},
		{"PUT", "/api/customer-processing/portal/recipient-addresses/12", `{"id":999,"customer_id":999,"recipient_name":"测试"}`, 200},
		{"DELETE", "/api/customer-processing/portal/recipient-addresses/12?customer_id=999&revision=3", "", 200},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
	if svc.query.CustomerID != 307 || svc.command.UsageCode != "direct_ship" || svc.command.CustomerID != 307 || svc.command.EmployeeID != 23 || svc.command.MiniUserID != 0 || svc.command.Actor != "验收员工" {
		t.Fatalf("identity leak: %+v %+v", svc.query, svc.command)
	}
	if addresses.customerID != 307 || addresses.saved.CustomerID != 307 || addresses.saved.ID != 12 || addresses.deleted.CustomerID != 307 || addresses.deleted.ID != 12 || addresses.deleted.ExpectedRevision != 3 {
		t.Fatalf("address identity leak: %+v", addresses)
	}
}

func TestPortalDirectShipRequiresLoginBindingAndCapability(t *testing.T) {
	for _, tc := range []struct {
		name       string
		employeeID int64
		overview   app.CustomerPortalOverview
		want       int
	}{
		{"anonymous", 0, app.CustomerPortalOverview{CustomerID: 307, Capabilities: []string{"direct_ship"}}, 401},
		{"unbound", 23, app.CustomerPortalOverview{}, 403},
		{"disabled capability", 23, app.CustomerPortalOverview{CustomerID: 307}, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &employeeDirectShipFake{fakeCustomerFulfillmentService: &fakeCustomerFulfillmentService{customerOverviewResult: tc.overview}}
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("employee_id", tc.employeeID); return next(c) }
			})
			RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, SharedProcessing: &portalAddressFake{fakeSharedProcessingService: &fakeSharedProcessingService{}}})
			for _, path := range []string{"direct-ship/catalog", "direct-ship/settings", "recipient-addresses"} {
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, httptest.NewRequest("GET", "/api/customer-processing/portal/"+path+"?customer_id=307", nil))
				if rec.Code != tc.want {
					t.Fatalf("%s: status=%d body=%s", path, rec.Code, rec.Body.String())
				}
			}
		})
	}
}
