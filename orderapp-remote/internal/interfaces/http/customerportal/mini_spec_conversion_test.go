package customerportal

import (
	"context"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	customerportalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"
	"strings"
	"testing"
)

type miniConversionFake struct {
	miniEmployeeSalesFake
	quote      salesapp.SpecConversionQuoteRequest
	quoteCalls int
	quoteError error
}

func (f *miniConversionFake) OrderSpecConversions(_ context.Context, c, p, b int64) (salesapp.SpecConversionCatalog, error) {
	return salesapp.SpecConversionCatalog{Specs: []salesapp.SpecConversionSpec{{ProductID: p, SpecID: 12, Name: "500g"}}}, nil
}
func (f *miniConversionFake) QuoteSpecConversion(_ context.Context, q salesapp.SpecConversionQuoteRequest) (salesapp.SpecConversionQuote, error) {
	f.quote = q
	f.quoteCalls++
	return salesapp.SpecConversionQuote{SpecConversionQuoteRequest: q, UnitPrice: "52.86"}, f.quoteError
}
func TestMiniConversionQuotesRequireEmployeeAndForwardRevision(t *testing.T) {
	for _, tc := range []struct {
		portal fakeService
		status int
	}{{employeePortalService(), 200}, {fakeService{me: customerportalapp.CurrentContext{AccountType: "customer", CurrentCustomerID: 8}}, 403}, {fakeService{me: customerportalapp.CurrentContext{AccountType: "employee", EmployeeID: 7, Permissions: []string{"orders.read"}}}, 403}} {
		e := echo.New()
		svc := &miniConversionFake{}
		registerMiniEmployeeAPI(e, tc.portal, svc)
		req := httptest.NewRequest("POST", "/api/mini/employee/spec-conversions/quote", strings.NewReader(`{"customer_id":8,"product_id":7,"publication_id":901,"group_id":2,"revision":4,"source_bom_spec_id":11,"source_bom_variant_id":21,"target_bom_spec_id":12,"target_bom_variant_id":22,"quantity":2,"price_row_key":"source"}`))
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
		if tc.status == 200 {
			if svc.quote.Revision != 4 || svc.quote.SourceSpecID != 11 || svc.quote.TargetSpecID != 12 {
				t.Fatal(svc.quote)
			}
		} else if svc.quoteCalls != 0 {
			t.Fatal("unauthorized quote")
		}
	}
}
func TestMiniConversionOrderCreateAndEditKeepSourceSeparateFromTarget(t *testing.T) {
	body := `{"order_date":"2026-10-16","customer_id":8,"selected_price_table_ids":[901],"items":[{"product_id":7,"item_id":33,"bom_spec_id":12,"bom_variant_id":22,"qty":2,"unit":"袋","bean_list_publication_id":901,"price_selection_mode":"converted_tier","selected_price_row_key":"source","unit_price":0.01,"spec_conversion":{"group_id":2,"revision":4,"source_bom_spec_id":11,"source_bom_variant_id":21}}]}`
	e := echo.New()
	svc := &miniConversionFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), svc)
	req := httptest.NewRequest("POST", "/api/mini/employee/orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	item := svc.save.Items[0]
	if item.BomSpecID != 12 || item.SpecConversion.SourceSpecID != 11 || item.ManualPrice != nil {
		t.Fatal(item)
	}
	var request miniEmployeeOrderRequest
	_ = json.Unmarshal([]byte(body), &request)
	cmd, err := miniEmployeeSaveOrderCommand(request, "employee", 42)
	if err != nil || cmd.Items[0].ItemID != 33 {
		t.Fatal(cmd, err)
	}
	frozen := `{"price_selection_mode":"converted_tier","spec_conversion":{"group_id":2,"revision":4,"source_bom_spec_id":11,"source_bom_variant_id":21}}`
	form := salesapp.OrderFormData{Customers: []salesapp.CustomerOption{{ID: 8}}, EditData: &salesapp.OrderEditData{ID: 42, CustomerID: 8, Items: []salesapp.OrderEditItem{{ItemID: 33, ProductID: 7, BomSpecID: 12, BomVariantID: 22, BeanListPublicationID: 901, PriceSourceJSON: frozen}}}}
	if message, err := miniEmployeePrepareCurrentCatalogFromForm(form, &cmd); err != nil || message != "" {
		t.Fatalf("frozen edit: %s %v", message, err)
	}
	cmd.Items[0].SpecConversion.Reprice = true
	if message, _ := miniEmployeePrepareCurrentCatalogFromForm(form, &cmd); message == "" {
		t.Fatal("reprice accepted withdrawn source")
	}
}
