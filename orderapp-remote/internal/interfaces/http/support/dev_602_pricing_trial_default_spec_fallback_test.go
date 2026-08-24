package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev602PricingTrialDefaultSpecFallbackContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "costing", "production_bom_cost.go"): {
			"versionDefaultSpecKeys",
			"variant.is_default",
			"specification cost under the product key",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-602 marker %q", rel, want)
			}
		}
	}

	costingSrc := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "costing", "production_bom_cost.go")))
	for _, want := range []string{
		"BOM组件单价为 0：请维护物料采购价；半成品物料需绑定默认已发布的制造 BOM",
		"BOM组件成本单位无法换算：消耗单位",
	} {
		if !strings.Contains(costingSrc, want) {
			t.Fatalf("costing repository missing specific trial error reason %q", want)
		}
	}
}
