package customerfulfillment

import (
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
