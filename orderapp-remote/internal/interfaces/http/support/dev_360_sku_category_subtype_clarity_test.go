package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev360SkuCategorySubtypeClarityRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-360-SKU-CATEGORY-SUBTYPE-CLARITY",
		"DEV-360-SKU-CATEGORY-SUBTYPE-CLARITY",
		"UT-360-SKU-CATEGORY-SUBTYPE-CLARITY",
		"API-360-SKU-CATEGORY-SUBTYPE-CLARITY",
		"REV-360-SKU-CATEGORY-SUBTYPE-CLARITY",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SKU category subtype clarity seed missing %q", want)
		}
	}
}

func TestDev360SkuCategorySubtypeClarityDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-360-SKU-CATEGORY-SUBTYPE-CLARITY",
			"新增 SKU",
			"产品类别",
		},
			filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
				"PR-360-SKU-CATEGORY-SUBTYPE-CLARITY",
				"单位规则维护在“商品配置模板”里",
				"不会超出",
			},
			filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
				"PR-360-SKU-CATEGORY-SUBTYPE-CLARITY",
				"单位规则维护在“商品配置模板”里",
				"已发布价格表和历史订单不会被回改",
			},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing SKU category subtype clarity docs marker %q", rel, want)
			}
		}
	}
}
