package customerfulfillment

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"orderapp/internal/infrastructure/postgres/orderbeans"
)

func TestCustomerFulfillmentPublishedPriceSourceSnapshotFreezesProductionQuantity(t *testing.T) {
	source, err := customerFulfillmentPublishedPriceSourceSnapshot(
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
		t.Fatalf("customer fulfillment snapshot: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(source), &decoded); err != nil {
		t.Fatalf("decode customer fulfillment snapshot: %v", err)
	}
	snapshot, ok := decoded["production_quantity_snapshot"].(map[string]any)
	if !ok || snapshot["inventory_qty_per_sales_unit"] != 0.454 {
		t.Fatalf("customer fulfillment production snapshot = %#v from %s", snapshot, source)
	}

	_, err = customerFulfillmentPublishedPriceSourceSnapshot(
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
		t.Fatalf("customer fulfillment missing conversion err = %v, want blocking error", err)
	}
}

func TestCustomerFulfillmentOrderWritePathsResolveLegacyProductionSpecBeforePersisting(t *testing.T) {
	raw, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	quotedStart := strings.Index(source, "func (r *Repository) quoteSubmittedDirectShipItemTx")
	quotedEnd := strings.Index(source[quotedStart:], "func (r *Repository) quoteSubmittedDirectShipItemForERPRebuildTx")
	importStart := strings.Index(source, "func (r *Repository) applyDirectShipItemTx")
	importEnd := strings.Index(source[importStart:], "func upsertCustomerFulfillmentOrderTrackingsTx")
	if quotedStart < 0 || quotedEnd <= 0 || importStart < 0 || importEnd <= 0 {
		t.Fatal("customer fulfillment order write paths not found")
	}
	quoted := source[quotedStart : quotedStart+quotedEnd]
	if !strings.Contains(quoted, "ResolvePublishedProductSpecForPublication") ||
		!strings.Contains(quoted, "beanListUsage.PublicationID") {
		t.Fatal("submitted direct-ship order must keep its exact published pricing spec")
	}

	imported := source[importStart : importStart+importEnd]
	if !strings.Contains(imported, "ResolveCurrentOrderProductionProductSpec") {
		t.Fatal("direct-ship import must freeze the active concrete SKU catalog conversion")
	}
	if strings.Contains(imported, "ResolvePublishedProductSpecForPublication") ||
		strings.Contains(imported, "ResolveUsage(") {
		t.Fatal("zero-price direct-ship import must not search or borrow a published price list")
	}
}
