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
