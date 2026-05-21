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
