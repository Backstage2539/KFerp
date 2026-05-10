package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev172ProducePlanNullProductRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-172",
		"DEV-172-01",
		"UT-172-01",
		"API-172-01",
		"REV-172-01",
		"product_id 为空",
		"TestProducePlanSkipsOrderItemsWithoutProductID",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 172 seed missing %q", want)
		}
	}
}

func TestDev172ProducePlanFiltersUnlinkedProducts(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "production", "unprod_summary.go")))
	for _, want := range []string{
		"COALESCE(oi.product_id,0) > 0",
		"COALESCE(d.product_id,0) > 0",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("unproduced summary query missing product guard %q", want)
		}
	}
}

func TestDev172ProducePlanRegressionAndManual(t *testing.T) {
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "produce_plan_api_test.go")))
	for _, want := range []string{
		"TestProducePlanSkipsOrderItemsWithoutProductID",
		"product_id,unit_price,line_total",
		"NULL,50,150",
		"SO-UNLINKED-ITEM",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("produce plan API regression test missing %q", want)
		}
	}

	for _, rel := range []string{
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"未绑定", "商品 SKU"} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing produce plan null product documentation marker %q", rel, want)
			}
		}
	}
}
