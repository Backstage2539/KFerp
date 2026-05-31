package sales

import (
	"os"
	"strings"
	"testing"
)

func TestEnsureOrderShippingTrackingTablesAddsLegacyTrackingColumnBeforeBackfill(t *testing.T) {
	body, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatalf("read schema.go: %v", err)
	}
	text := string(body)
	addColumn := strings.Index(text, "ADD COLUMN IF NOT EXISTS ship_tracking_no")
	backfill := strings.Index(text, "regexp_split_to_table(COALESCE(o.ship_tracking_no")
	if addColumn < 0 {
		t.Fatal("sales schema must add orders.ship_tracking_no for legacy or minimal order tables")
	}
	if backfill < 0 {
		t.Fatal("sales schema must backfill order_shipping_trackings from the legacy ship_tracking_no field")
	}
	if addColumn > backfill {
		t.Fatal("sales schema must add orders.ship_tracking_no before reading it during tracking backfill")
	}
}

func TestEnsureSchemaBackfillsERPOrdersForFulfillmentCustomers(t *testing.T) {
	body, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatalf("read schema.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"backfillERPOrdersForFulfillmentCustomers",
		"portal_service_code='product_order'",
		"COALESCE(o.portal_service_code,'')=''",
		"customer_erp_user_bindings",
		"public_sku_direct_ship",
		"processing_fulfillment",
		"channel_direct_ship",
		"p.enabled=true",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("sales schema missing fulfillment backfill marker %q", want)
		}
	}
}

func TestEnsureOrderItemUnitPricingColumnsAddsCustomerAliasSnapshots(t *testing.T) {
	body, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatalf("read schema.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"order_items ADD COLUMN IF NOT EXISTS customer_product_alias_id",
		"order_items ADD COLUMN IF NOT EXISTS customer_product_display_name_snapshot",
		"order_items ADD COLUMN IF NOT EXISTS customer_item_code_snapshot",
		"order_items ADD COLUMN IF NOT EXISTS brand_name_snapshot",
		"order_items ADD COLUMN IF NOT EXISTS product_code_snapshot",
		"order_items ADD COLUMN IF NOT EXISTS product_name_snapshot",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("sales schema missing order item alias snapshot column marker %q", want)
		}
	}
}
