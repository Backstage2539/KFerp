package support

import (
	"strings"
	"testing"
)

func TestDev305GreenBeanOrderLbUnitPriceSeed(t *testing.T) {
	store := string(readOrderAppFileForTest(t, "internal/interfaces/http/support/req_store.go"))
	for _, want := range []string{
		"PR-305-GREEN-BEAN-ORDER-LB-UNIT-PRICE",
		"DEV-305-GREEN-BEAN-ORDER-LB-UNIT-PRICE",
		"UT-305-GREEN-BEAN-ORDER-LB-UNIT-PRICE",
		"API-305-GREEN-BEAN-ORDER-LB-UNIT-PRICE",
		"REV-305-GREEN-BEAN-ORDER-LB-UNIT-PRICE",
		"docs/acceptance/2026-05-21-green-bean-order-lb-unit-price.md",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-305 seed marker %q", want)
		}
	}
}

func TestDev305GreenBeanManualLbPriceManuals(t *testing.T) {
	markers := map[string][]string{
		"docs/OP_MANUAL_COSTING.md":                                    []string{"档位价格输入框是元/磅", "60kg+"},
		"docs/OP_MANUAL_GREEN_BEAN_SALES.md":                           []string{"60kg+` 手工价 62", "price_per_lb"},
		"docs/OP_MANUAL_ORDER_SALES.md":                                []string{"60kg+ 62/磅"},
		"docs/acceptance/2026-05-21-green-bean-order-lb-unit-price.md": []string{"60kg+ 62/磅", "price_per_lb"},
	}
	for rel, wants := range markers {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing green bean manual lb price marker %q", rel, want)
			}
		}
	}
}
