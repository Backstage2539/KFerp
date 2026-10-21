package customerportal

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

type miniEmployeeLegacyFreightSales struct {
	*miniEmployeeSalesFake
}

func (f *miniEmployeeLegacyFreightSales) SaveOrder(ctx context.Context, cmd salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error) {
	if cmd.FreightProtocol == "" && !cmd.LegacyEmployeeFreight {
		return salesapp.SaveOrderResult{}, errors.New("请更新客户端并确认运费待实际账单出账")
	}
	return f.miniEmployeeSalesFake.SaveOrder(ctx, cmd)
}

func miniEmployeeLegacyFreightRequest(method, path, fields string) *http.Request {
	body := `{"order_date":"2026-10-21","customer_id":8,` + fields + `"items":[{"product_id":551,"parent_product_id":550,"qty":1,"spec_g":227,"bean_list_publication_id":901,"bean_list_version_no":"V9.1"}]}`
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	return req
}

func TestMiniEmployeeLegacyFreightCreatePreservesCurrentOrderCharge(t *testing.T) {
	for _, tt := range []struct {
		name   string
		fields string
		amount float64
	}{
		{name: "manual freight", fields: `"shipping_amount":20,`, amount: 20},
		{name: "explicit zero", fields: `"shipping_amount":0,`},
		{name: "omitted freight"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			sales := &miniEmployeeLegacyFreightSales{miniEmployeeSalesFake: &miniEmployeeSalesFake{}}
			registerMiniEmployeeAPI(e, employeePortalService(), sales)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, miniEmployeeLegacyFreightRequest(http.MethodPost, "/api/mini/employee/orders", tt.fields))
			if rec.Code != http.StatusCreated || sales.saveCalls != 1 {
				t.Fatalf("legacy employee order rejected: status=%d save_calls=%d body=%s", rec.Code, sales.saveCalls, rec.Body.String())
			}
			cmd := sales.save
			if !cmd.LegacyEmployeeFreight || cmd.DraftEmployeeID != 7 || cmd.ShippingAmount != tt.amount || cmd.FreightProtocol != "" || cmd.FreightQuoteToken != "" || cmd.FreightOverride != nil || cmd.CustomerSubmission {
				t.Fatalf("legacy fixed charge or employee identity changed: %+v", cmd)
			}
		})
	}
}

func TestMiniEmployeeLegacyFreightCreateDoesNotDowngradeQuoteRequests(t *testing.T) {
	for _, fields := range []string{
		`"freight_protocol":"order_freight_v2",`,
		`"freight_protocol":"deferred_freight_v1",`,
		`"freight_protocol":"unknown",`,
		`"freight_protocol":" ",`,
		`"freight_quote_token":"quote",`,
		`"expected_freight_token":"expected",`,
		`"freight_override":{},`,
		`"shipping_province":"云南省",`,
		`"shipping_city":"普洱市",`,
		`"shipping_district":"思茅区",`,
		`"LegacyEmployeeFreight":true,"DraftEmployeeID":99,"freight_quote_token":"quote",`,
	} {
		t.Run(fields, func(t *testing.T) {
			e := echo.New()
			sales := &miniEmployeeSalesFake{}
			registerMiniEmployeeAPI(e, employeePortalService(), sales)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, miniEmployeeLegacyFreightRequest(http.MethodPost, "/api/mini/employee/orders", fields))
			if rec.Code != http.StatusCreated || sales.saveCalls != 1 || sales.save.LegacyEmployeeFreight || sales.save.DraftEmployeeID != 7 {
				t.Fatalf("modern or incomplete quote request downgraded: status=%d command=%+v body=%s", rec.Code, sales.save, rec.Body.String())
			}
		})
	}
}

func TestMiniEmployeeLegacyFreightCreateRequiresAuthentication(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := miniEmployeeLegacyFreightRequest(http.MethodPost, "/api/mini/employee/orders", `"LegacyEmployeeFreight":true,"DraftEmployeeID":7,`)
	req.Header.Del(echo.HeaderAuthorization)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || sales.saveCalls != 0 {
		t.Fatalf("unauthenticated legacy request accepted: status=%d calls=%d", rec.Code, sales.saveCalls)
	}
}

func TestMiniEmployeeLegacyFreightUpdateDoesNotEnableCreateCompatibility(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{listResult: &salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 42, CustomerID: 8}}}}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, miniEmployeeLegacyFreightRequest(http.MethodPut, "/api/mini/employee/orders/42", `"shipping_amount":20,"LegacyEmployeeFreight":true,"DraftEmployeeID":7,`))
	if rec.Code != http.StatusOK || sales.saveCalls != 1 || sales.save.LegacyEmployeeFreight || sales.save.DraftEmployeeID != 0 || sales.save.EditID != 42 {
		t.Fatalf("legacy create compatibility leaked to edit: status=%d command=%+v body=%s", rec.Code, sales.save, rec.Body.String())
	}
}
