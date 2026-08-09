package customerportal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

type processingCatalogPortalFake struct {
	fakeService
	allowed []int64
	seen    []int64
}

func (f *processingCatalogPortalFake) FilterProcessingCatalogProductIDs(_ context.Context, _ string, productIDs []int64) ([]int64, error) {
	f.seen = append([]int64(nil), productIDs...)
	return append([]int64(nil), f.allowed...), nil
}

func TestMiniProcessingCatalogReusesOrderProductFamiliesWithoutPrices(t *testing.T) {
	portal := &processingCatalogPortalFake{
		fakeService: fakeService{me: customerportalapp.CurrentContext{
			CurrentCustomerID: 8,
			Capabilities:      []customerportalapp.Capability{{Code: customerportalapp.CapabilityProcessing, Enabled: true}},
		}},
		allowed: []int64{551},
	}
	sales := &miniEmployeeSalesFake{orderFormResult: &salesapp.OrderFormData{
		Products: []salesapp.ProductOption{
			{
				ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎 227g",
				SKUName: "227g袋装", SKUCode: "WLG-227", SpecLabel: "227g", NetContentQty: 227, NetContentUnit: "g",
				ProductKind: "roasted_bean", Visibility: "public", DefaultPrice: 88,
				Tiers: []salesapp.ProductTierOption{{ID: 9, UnitPrice: 68}},
			},
			{ID: 552, SKUID: 552, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎 454g", SKUName: "454g袋装", SpecLabel: "454g", ProductKind: "roasted_bean", Visibility: "public"},
		},
		CustomerPublicUsages: []salesapp.CustomerPublicUsageOption{{CustomerID: 8, UsePublicSKU: true}},
	}}
	e := echo.New()
	registerMiniProcessingCatalogAPI(e, portal, sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/processing/catalog", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["current_customer_id"] != float64(8) {
		t.Fatalf("current_customer_id=%v", body["current_customer_id"])
	}
	families, _ := body["product_families"].([]any)
	if len(families) != 1 {
		t.Fatalf("families=%#v, want one BOM-configured family", families)
	}
	family := families[0].(map[string]any)
	specs := family["specs"].([]any)
	if len(specs) != 1 || specs[0].(map[string]any)["sku_id"] != float64(551) {
		t.Fatalf("specs=%#v, want only SKU 551", specs)
	}
	if _, exists := specs[0].(map[string]any)["tiers"]; exists {
		t.Fatalf("production catalog leaked price tiers: %#v", specs[0])
	}
	if len(portal.seen) != 2 {
		t.Fatalf("catalog filter saw %#v, want both visible concrete SKUs", portal.seen)
	}
}
