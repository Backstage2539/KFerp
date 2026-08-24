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

type processingBOMSpecCatalogPortalFake struct {
	fakeService
	targets []customerportalapp.ProcessingCatalogTarget
	seen    []int64
}

func (f *processingBOMSpecCatalogPortalFake) ListProcessingCatalogTargets(_ context.Context, _ string, productIDs []int64) ([]customerportalapp.ProcessingCatalogTarget, error) {
	f.seen = append([]int64(nil), productIDs...)
	return append([]customerportalapp.ProcessingCatalogTarget(nil), f.targets...), nil
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

func TestMiniProcessingCatalogUsesParentBOMSpecIdentityAfterCutover(t *testing.T) {
	portal := &processingBOMSpecCatalogPortalFake{
		fakeService: fakeService{me: customerportalapp.CurrentContext{
			CurrentCustomerID: 8,
			Capabilities:      []customerportalapp.Capability{{Code: customerportalapp.CapabilityProcessing, Enabled: true}},
		}},
		targets: []customerportalapp.ProcessingCatalogTarget{
			{ProductID: 10, BomSpecID: 101, BomVariantID: 1001, SpecName: "227g袋", InventoryUnit: "袋", IsDefault: true, SortOrder: 1},
			{ProductID: 551},
		},
	}
	sales := &miniEmployeeSalesFake{orderFormResult: &salesapp.OrderFormData{
		Products: []salesapp.ProductOption{
			{ID: 10, SKUID: 10, ParentProductID: 10, ParentProductName: "商品A", Name: "商品A", ProductKind: "roasted_bean", Visibility: "public"},
			{ID: 11, SKUID: 11, ParentProductID: 10, ParentProductName: "商品A", Name: "商品A旧227g", SKUName: "旧227g", SpecLabel: "227g", ProductKind: "roasted_bean", Visibility: "public", DefaultPrice: 99},
			{ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "旧模式商品", Name: "旧模式商品 454g", SKUName: "454g", SpecLabel: "454g", ProductKind: "roasted_bean", Visibility: "public", DefaultPrice: 88},
		},
		ProductBOMSpecOptions: []salesapp.ProductBOMSpecOption{{
			ParentProductID: 10, LegacyChildProductID: 11, BomID: 100, BomVersionID: 1000, BomVersionNo: "v1",
			BomSpecID: 101, BomVariantID: 1001, SpecCode: "BOM-SPEC-000101", SpecKey: "bag-227", SpecName: "227g袋", InventoryUnit: "袋",
			Published: true, IsDefault: true, SortOrder: 1, MigrationState: "cutover", WriteProductID: 10, WriteBomSpecID: 101, WriteBomVariantID: 1001,
			Tiers: []salesapp.ProductTierOption{{ID: 9, UnitPrice: 68}},
		}},
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
	var body struct {
		CurrentCustomerID int64 `json:"current_customer_id"`
		Families          []struct {
			ParentProductID int64 `json:"parent_product_id"`
			Specs           []struct {
				ProductID    int64 `json:"product_id"`
				SKUID        int64 `json:"sku_id"`
				BomSpecID    int64 `json:"bom_spec_id"`
				BomVariantID int64 `json:"bom_variant_id"`
				SpecG        int64 `json:"spec_g"`
				Tiers        []any `json:"tiers"`
			} `json:"specs"`
		} `json:"product_families"`
		Options []salesapp.ProductBOMSpecOption `json:"product_bom_spec_options"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.CurrentCustomerID != 8 || len(body.Families) != 2 || len(body.Options) != 1 {
		t.Fatalf("catalog=%+v body=%s", body, rec.Body.String())
	}
	var canonicalFound, legacyFound bool
	for _, family := range body.Families {
		for _, spec := range family.Specs {
			if spec.BomSpecID == 101 {
				canonicalFound = spec.ProductID == 10 && spec.SKUID == 0 && spec.BomVariantID == 1001 && spec.SpecG == 0 && len(spec.Tiers) == 0
			}
			if spec.SKUID == 551 {
				legacyFound = spec.BomSpecID == 0 && len(spec.Tiers) == 0
			}
			if spec.SKUID == 11 || spec.ProductID == 11 {
				t.Fatalf("cutover legacy child leaked: %+v", spec)
			}
		}
	}
	if !canonicalFound || !legacyFound {
		t.Fatalf("canonical=%v legacy=%v body=%s", canonicalFound, legacyFound, rec.Body.String())
	}
	if body.Options[0].ParentProductID != 10 || body.Options[0].BomSpecID != 101 || body.Options[0].BomVariantID != 1001 || len(body.Options[0].Tiers) != 0 {
		t.Fatalf("options=%+v", body.Options)
	}
	if len(portal.seen) != 4 {
		t.Fatalf("catalog targets saw %#v, want parent and concrete visible identities", portal.seen)
	}
}
