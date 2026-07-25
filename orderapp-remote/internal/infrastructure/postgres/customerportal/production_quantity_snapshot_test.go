package customerportal

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"orderapp/internal/infrastructure/postgres/orderbeans"
)

func TestPortalPublishedPriceSourceSnapshotFreezesProductionQuantity(t *testing.T) {
	source, err := portalPublishedPriceSourceSnapshot(
		orderbeans.ListTypeCommercial,
		orderbeans.Usage{PublicationID: 91, VersionNo: "V3.0.9"},
		551,
		orderbeans.PublishedPricing{
			UnitPrice:               68,
			PriceUnit:               "袋",
			InventoryUnit:           "kg",
			InventoryConversionJSON: `{"袋":{"kg":0.454}}`,
		},
		orderbeans.PublishedProductSpec{
			ConcretePublication:     true,
			ProductFound:            true,
			SKUID:                   551,
			ParentProductID:         550,
			SpecLabel:               "454g",
			SalesUnit:               "袋",
			InventoryUnit:           "kg",
			InventoryConversionJSON: `{"袋":{"kg":0.454}}`,
		},
	)
	if err != nil {
		t.Fatalf("portal snapshot: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(source), &decoded); err != nil {
		t.Fatalf("decode portal snapshot: %v", err)
	}
	snapshot, ok := decoded["production_quantity_snapshot"].(map[string]any)
	if !ok || snapshot["inventory_qty_per_sales_unit"] != 0.454 {
		t.Fatalf("portal production snapshot = %#v from %s", snapshot, source)
	}

	_, err = portalPublishedPriceSourceSnapshot(
		orderbeans.ListTypeCommercial,
		orderbeans.Usage{PublicationID: 91, VersionNo: "V3.0.9"},
		552,
		orderbeans.PublishedPricing{UnitPrice: 68},
		orderbeans.PublishedProductSpec{
			ConcretePublication: true,
			ProductFound:        true,
			SKUID:               552,
			ParentProductID:     550,
			SpecLabel:           "454g",
			SalesUnit:           "袋",
			InventoryUnit:       "kg",
		},
	)
	if err == nil || !strings.Contains(err.Error(), "缺少") {
		t.Fatalf("portal missing conversion err = %v, want blocking error", err)
	}
}

func TestPortalOrderWritePathsResolveLegacyProductionSpecBeforePersisting(t *testing.T) {
	raw, err := os.ReadFile("business_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	mallStart := strings.Index(source, "func (r Repository) CreateMallOrder")
	fulfillmentStart := strings.Index(source, "func (r Repository) CreateFulfillmentOrder")
	if mallStart < 0 || fulfillmentStart <= mallStart {
		t.Fatal("portal order write paths not found")
	}
	mall := source[mallStart:fulfillmentStart]
	if !strings.Contains(mall, "ResolveCurrentOrderProductionProductSpec") {
		t.Fatal("mall order must freeze the active concrete SKU catalog conversion")
	}
	if strings.Contains(mall, "ResolvePublishedProductSpecForPublication") {
		t.Fatal("mall price is not a price-list source and must not borrow a published spec")
	}

	fulfillment := source[fulfillmentStart:]
	if !strings.Contains(fulfillment, "ResolvePublishedProductSpecForPublication") ||
		!strings.Contains(fulfillment, "usage.PublicationID") {
		t.Fatal("published portal fulfillment order must keep its exact price-list spec snapshot")
	}
}
