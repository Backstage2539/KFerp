package customerportal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type miniCustomerBillServiceFake struct {
	Service
	token  string
	billID int64
}

func (s *miniCustomerBillServiceFake) ListCustomerBills(_ context.Context, token string) ([]customerportalapp.CustomerBillSummary, error) {
	s.token = token
	return []customerportalapp.CustomerBillSummary{{ID: 41, SettlementNo: "CPB-19-41", Status: "confirmed", TotalAmount: "88.50", WorkOrderCount: 2}}, nil
}

func (s *miniCustomerBillServiceFake) GetCustomerBill(_ context.Context, token string, billID int64) (customerportalapp.CustomerBillDetail, error) {
	s.token = token
	s.billID = billID
	return customerportalapp.CustomerBillDetail{
		CustomerBillSummary: customerportalapp.CustomerBillSummary{ID: billID, SettlementNo: "CPB-19-41", TotalAmount: "88.50"},
		Lines:               []customerportalapp.CustomerBillLine{{WorkOrderID: 91, FeeName: "烘焙费", Basis: "actual_output_kg", Amount: "88.50"}},
	}, nil
}

func TestMiniCustomerBillsListAndDetailUseMiniToken(t *testing.T) {
	svc := &miniCustomerBillServiceFake{}
	e := echo.New()
	registerMiniAPI(e, svc, nil, nil, nil)

	for _, tc := range []struct {
		path string
		want string
	}{
		{"/api/mini/customer-bills", `"settlement_no":"CPB-19-41"`},
		{"/api/mini/customer-bills/41", `"fee_name":"烘焙费"`},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer token-19")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("GET %s status=%d body=%s", tc.path, rec.Code, rec.Body.String())
		}
	}
	if svc.token != "token-19" || svc.billID != 41 {
		t.Fatalf("mini bill service token=%q billID=%d", svc.token, svc.billID)
	}
}

func TestMiniCustomerBillsRequireTokenAndValidBillID(t *testing.T) {
	e := echo.New()
	registerMiniAPI(e, &miniCustomerBillServiceFake{}, nil, nil, nil)
	for _, path := range []string{"/api/mini/customer-bills", "/api/mini/customer-bills/0"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if strings.HasSuffix(path, "/0") {
			req.Header.Set(echo.HeaderAuthorization, "Bearer token-19")
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusBadRequest {
			t.Fatalf("GET %s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}
