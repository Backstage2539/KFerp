package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev543PriceListPieceTierLabelContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-543-PRICE-LIST-PIECE-TIER-LABEL",
		"DEV-543-PIECE-TIER-LABEL",
		"DEV-543-SERVER-NAME-NORMALIZATION",
		"DEV-543-DOCS-DEPLOY",
		"REV-543-PRICE-LIST-PIECE-TIER-LABEL",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing PR-543 seed %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "costing-price-list-workflow.js"): {
			"priceListSalesSpecCountTierLabel", "件",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"normalizeConcreteProductSpecPublicationSnapshots", "ParentProductName", "product_name_snapshot", "display_name_snapshot",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-543-PRICE-LIST-PIECE-TIER-LABEL", "2-13件", "历史发布",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-543-PRICE-LIST-PIECE-TIER-LABEL", "白月光瑰夏", "2-13件",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"2-13件", "规格单独", "不重新发布",
		},
		filepath.Join("docs", "acceptance", "2026-07-21-price-list-piece-tier-label.md"): {
			"PR-543", "RED", "GREEN", "生产环境未部署",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-543 marker %q", rel, want)
			}
		}
	}
}
