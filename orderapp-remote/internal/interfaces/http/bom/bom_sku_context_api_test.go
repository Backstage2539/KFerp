package bom

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bomapp "orderapp/internal/application/bom"

	"github.com/labstack/echo/v4"
)

func TestBomListAndProductsExposeCustomerID(t *testing.T) {
	repo := &apiFakeRepo{
		listRows: []bomapp.ListItem{
			{ProductID: 1, Product: "公共SKU", CustomerID: 0, Status: "active"},
			{ProductID: 2, Product: "客户SKU", CustomerID: 9, Status: "active"},
		},
		productRows: []bomapp.Option{
			{ID: 1, Name: "公共SKU", CustomerID: 0},
			{ID: 2, Name: "客户SKU", CustomerID: 9},
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	for _, tc := range []struct {
		path     string
		wantJSON string
	}{
		{path: "/api/bom/list", wantJSON: `"customer_id":9`},
		{path: "/api/bom/products", wantJSON: `"customer_id":9`},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", tc.path, rec.Code, rec.Body.String())
		}
		if !json.Valid(rec.Body.Bytes()) {
			t.Fatalf("%s returned invalid json: %s", tc.path, rec.Body.String())
		}
		if got := rec.Body.String(); !strings.Contains(got, tc.wantJSON) {
			t.Fatalf("%s body missing %s: %s", tc.path, tc.wantJSON, got)
		}
	}
}

var _ bomapp.Repository = (*apiFakeRepo)(nil)
