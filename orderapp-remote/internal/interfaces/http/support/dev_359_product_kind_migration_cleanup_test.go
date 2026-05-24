package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev359ProductKindMigrationCleanupRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-359-PRODUCT-KIND-MIGRATION-CLEANUP",
		"DEV-359-PRODUCT-KIND-MIGRATION-CLEANUP",
		"UT-359-PRODUCT-KIND-MIGRATION-CLEANUP",
		"API-359-PRODUCT-KIND-MIGRATION-CLEANUP",
		"REV-359-PRODUCT-KIND-MIGRATION-CLEANUP",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product kind migration cleanup seed missing %q", want)
		}
	}
}

func TestDev359ProductKindMigrationCleanupDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-359-PRODUCT-KIND-MIGRATION-CLEANUP",
			"product_kind",
			"历史订单",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-359-PRODUCT-KIND-MIGRATION-CLEANUP",
			"速溶咖啡",
			"历史豆单",
		},
		filepath.Join("docs", "acceptance", "2026-05-24-product-kind-migration-cleanup.md"): {
			"PR-359-PRODUCT-KIND-MIGRATION-CLEANUP",
			"完整场景",
			"速溶咖啡",
			"历史兼容",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-359-PRODUCT-KIND-MIGRATION-CLEANUP",
			"产品价格表",
			"速溶咖啡",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"PR-359-PRODUCT-KIND-MIGRATION-CLEANUP",
			"产品价格表",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-359-PRODUCT-KIND-MIGRATION-CLEANUP",
			"工序模板",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-359-PRODUCT-KIND-MIGRATION-CLEANUP",
			"产品类型",
			"产品子类型",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product kind migration cleanup docs marker %q", rel, want)
			}
		}
	}
}
