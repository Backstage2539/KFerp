package sales

import (
	"os"
	"strings"
	"testing"
)

func TestOrderFormProductQueryKeepsRoastLevelAndProductKindScanShape(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, "SELECT id, name, COALESCE(roast_level,'')") {
		t.Fatalf("order form product query must select roast_level as the third product column before default_price")
	}
	if strings.Contains(text, "SELECT id, name, COALESCE(NULLIF(product_kind,''),'roasted')") {
		t.Fatalf("order form product query must not scan product_kind into ProductOption.RoastLevel")
	}
}

func TestOrderFormProductQueryExposesBoundRoastedTiersForGreenBeanProducts(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"green_bean_bom_product_id",
		"green_bean_bound_roasted_tier",
		"source_product_id",
		"NOT EXISTS",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("order form product tiers must expose bound roasted tiers for green bean products; missing %q", want)
		}
	}
}

func TestOrderSaveUsesBoundRoastedTierFallbackForGreenBeanOrders(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"greenBeanOrderPriceProductIDTx",
		"resolveAutoWeightTierPriceTx",
		"green_bean_bound_roasted_tier",
		"source_product_id",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("green bean order save must fall back to bound roasted tiers; missing %q", want)
		}
	}
}
