package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev411CustomerProductConfigTemplatePricingSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-411-CUSTOMER-PRODUCT-CONFIG-TEMPLATE-PRICING",
		"DEV-411-CUSTOMER-PRODUCT-RENAME",
		"DEV-411-ALIAS-CONFIG-TEMPLATE",
		"DEV-411-PRICE-LIST-CONFIG-TEMPLATE-SOURCE",
		"客户商品",
		"商品配置模板",
		"计价方式",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing customer product config template pricing marker %q", want)
		}
	}
}
