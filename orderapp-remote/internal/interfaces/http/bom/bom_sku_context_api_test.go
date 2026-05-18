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

func TestBomDetailExposesFinishedProductComponentFields(t *testing.T) {
	repo := &apiFakeRepo{
		detail: bomapp.Detail{
			ProductID: 1,
			Items: []bomapp.Item{{
				ID:                   9,
				ComponentType:        "finished_product",
				ComponentProductID:   7,
				ComponentProductName: "中烘熟豆",
				ComponentSpecG:       10,
				ConsumeUnit:          "g_per_bag",
				QtyPerUnit:           10,
			}},
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodGet, "/api/bom/detail/1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"component_type":"finished_product"`, `"component_product_id":7`, `"consume_unit":"g_per_bag"`, `"qty_per_unit":10`} {
		if !strings.Contains(body, want) {
			t.Fatalf("detail body missing %s: %s", want, body)
		}
	}
}

func TestBomSaveItemAcceptsFinishedProductComponentFields(t *testing.T) {
	repo := &apiFakeRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	body := `{"product_id":1,"component_type":"finished_product","component_product_id":7,"component_spec_g":10,"consume_unit":"g_per_bag","qty_per_unit":10}`
	req := httptest.NewRequest(http.MethodPost, "/api/bom/item/save", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.savedItem.ComponentType != "finished_product" || repo.savedItem.ComponentProductID != 7 || repo.savedItem.ConsumeUnit != "g_per_bag" || repo.savedItem.QtyPerUnit != 10 {
		t.Fatalf("saved item = %+v", repo.savedItem)
	}
}

var _ bomapp.Repository = (*apiFakeRepo)(nil)
