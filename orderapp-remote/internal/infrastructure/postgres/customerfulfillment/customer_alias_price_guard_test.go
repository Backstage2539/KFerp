package customerfulfillment

import (
	"os"
	"strings"
	"testing"
)

func TestRequireCustomerAliasDirectShipPriceRejectsZeroUnpublishedPrice(t *testing.T) {
	snap := directShipCustomerAliasSnapshot{CustomerProductAliasID: 91}
	err := requireCustomerAliasDirectShipPrice(snap, 0)
	if err == nil || !strings.Contains(err.Error(), "customer product price unpublished") {
		t.Fatalf("requireCustomerAliasDirectShipPrice err=%v, want unpublished price rejection", err)
	}

	if err := requireCustomerAliasDirectShipPrice(directShipCustomerAliasSnapshot{}, 0); err != nil {
		t.Fatalf("plain product without alias should allow legacy zero price fallback: %v", err)
	}
	if err := requireCustomerAliasDirectShipPrice(snap, 12.5); err != nil {
		t.Fatalf("positive resolved customer alias price should be allowed: %v", err)
	}
}

func TestCustomerDirectShipPricingUsesPublishedSnapshotsOnly(t *testing.T) {
	body, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"ResolvePublishedPricingForPublicationWithUnit",
		"ResolveUsageForPublication",
		"published_price_snapshot",
		"source_price_record_id",
		"inventory_conversion_json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer direct-ship pricing missing published snapshot marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"customerFulfillmentSubmittedUnitPriceTx",
		"customerFulfillmentDripUnitPriceTiersTx",
		"FROM %s.product_price_tiers",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("customer direct-ship pricing should not keep legacy price fallback %q", forbidden)
		}
	}
}
