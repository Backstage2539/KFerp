package sales

import (
	"strings"
	"testing"
)

func TestValidateCustomerAliasPublishedPriceRejectsZeroUnpublishedPrice(t *testing.T) {
	err := validateCustomerAliasPublishedPrice(91, false, 0)
	if err == nil || !strings.Contains(err.Error(), "customer product price unpublished") {
		t.Fatalf("validateCustomerAliasPublishedPrice err=%v, want unpublished price rejection", err)
	}

	if err := validateCustomerAliasPublishedPrice(0, false, 0); err != nil {
		t.Fatalf("plain product without alias should allow legacy zero price fallback: %v", err)
	}
	if err := validateCustomerAliasPublishedPrice(91, true, 0); err != nil {
		t.Fatalf("manual price override should be allowed for ERP order entry: %v", err)
	}
	if err := validateCustomerAliasPublishedPrice(91, false, 12.5); err != nil {
		t.Fatalf("positive resolved customer alias price should be allowed: %v", err)
	}
}
