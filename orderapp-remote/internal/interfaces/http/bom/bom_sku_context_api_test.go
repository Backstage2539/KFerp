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
			{ProductID: 1, Product: "公共SKU", CustomerID: 0, Status: "active", ProductKind: "roasted_bean"},
			{ProductID: 2, Product: "客户SKU", CustomerID: 9, Status: "active", ProductKind: "roasted_bean"},
			{ProductID: 3, Product: "客户生豆SKU", CustomerID: 9, Status: "active", ProductKind: "green_bean"},
		},
		productRows: []bomapp.Option{
			{ID: 1, ProductCode: "SKU-000001", Name: "公共SKU", CustomerID: 0, ProductKind: "roasted_bean", InventoryUnit: "kg", InventoryUnitExplicit: false},
			{ID: 2, ProductCode: "SKU-000002", Name: "客户SKU", CustomerID: 9, ProductKind: "roasted_bean", InventoryUnit: "盒", InventoryUnitExplicit: true},
			{ID: 3, ProductCode: "SKU-000003", Name: "客户生豆SKU", CustomerID: 9, ProductKind: "green_bean"},
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	for _, tc := range []struct {
		path     string
		wantJSON string
	}{
		{path: "/api/bom/list", wantJSON: `"customer_id":9`},
		{path: "/api/bom/products", wantJSON: `"inventory_unit_explicit":true`},
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
		if got := rec.Body.String(); strings.Contains(got, "客户生豆SKU") || strings.Contains(got, `"product_id":3`) || strings.Contains(got, `"id":3`) {
			t.Fatalf("%s must not expose green bean SKU in BOM context: %s", tc.path, got)
		}
	}
}

func TestBomDetailExposesFinishedProductComponentFields(t *testing.T) {
	repo := &apiFakeRepo{
		detail: bomapp.Detail{
			ProductID: 1,
			YieldRate: 0.82,
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
	for _, want := range []string{`"component_type":"finished_product"`, `"component_product_id":7`, `"consume_unit":"g_per_bag"`, `"qty_per_unit":10`, `"expected_yield_rate":0.82`, `"expected_loss_rate":0.18`} {
		if !strings.Contains(body, want) {
			t.Fatalf("detail body missing %s: %s", want, body)
		}
	}
}

func TestBomDetailExposesSourceMetadataForInheritedBom(t *testing.T) {
	repo := &apiFakeRepo{
		detail: bomapp.Detail{
			ProductID:          188,
			ProductName:        "Karen 招牌拼配",
			YieldRate:          0.82,
			BomSourceType:      "inherit_current",
			EffectiveProductID: 21,
			SourceProductID:    21,
			SourceProductCode:  "SKU-21",
			SourceProductName:  "K001 精品意式拼配",
			SourceBomVersionID: 7,
			SourceBomVersionNo: "V003",
			DerivedFromLabel:   "继承：SKU-21 K001 精品意式拼配 / BOM V003",
			CanEditBOM:         false,
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodGet, "/api/bom/detail/188", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"bom_source_type":"inherit_current"`,
		`"effective_product_id":21`,
		`"source_product_code":"SKU-21"`,
		`"source_product_name":"K001 精品意式拼配"`,
		`"source_bom_version_id":7`,
		`"source_bom_version_no":"V003"`,
		`"derived_from_label":"继承：SKU-21 K001 精品意式拼配 / BOM V003"`,
		`"can_edit_bom":false`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("detail body missing %s: %s", want, body)
		}
	}
}

func TestBomDeriveOwnedAPIPassesActorAndProductID(t *testing.T) {
	repo := &apiFakeRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/bom/188/derive-owned", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.derivedOwned.ProductID != 188 {
		t.Fatalf("derive owned product id = %d, want 188", repo.derivedOwned.ProductID)
	}
}

func TestBomSaveAcceptsExpectedLossRate(t *testing.T) {
	repo := &apiFakeRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	body := `{"product_id":7,"expected_loss_rate":0.18}`
	req := httptest.NewRequest(http.MethodPost, "/api/bom/save", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.syncedYield.ProductID != 7 {
		t.Fatalf("product id = %d, want 7", repo.syncedYield.ProductID)
	}
	if repo.syncedYield.ExpectedLossRate == nil || *repo.syncedYield.ExpectedLossRate != 0.18 {
		t.Fatalf("expected loss rate = %v, want 0.18", repo.syncedYield.ExpectedLossRate)
	}
}

func TestBomSaveRejectsInvalidExpectedLossRate(t *testing.T) {
	repo := &apiFakeRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	body := `{"product_id":7,"expected_loss_rate":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/bom/save", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "expected_loss_rate") {
		t.Fatalf("body missing expected_loss_rate error: %s", rec.Body.String())
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

func TestBomSaveItemRejectsFinishedProductRatioPct(t *testing.T) {
	repo := &apiFakeRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	body := `{"product_id":1,"component_type":"finished_product","component_product_id":7,"consume_unit":"ratio_pct","ratio_pct":10}`
	req := httptest.NewRequest(http.MethodPost, "/api/bom/item/save", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "finished_product consume_unit must not be ratio_pct") {
		t.Fatalf("body missing finished product ratio error: %s", rec.Body.String())
	}
}

var _ bomapp.Repository = (*apiFakeRepo)(nil)
