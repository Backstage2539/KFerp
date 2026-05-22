package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev322CustomerCategoryCleanupAndBomVersionRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-322-CUSTOMER-SKU-CATEGORY-CLEANUP-BOM-VERSION-DEACTIVATE",
		"DEV-322-CUSTOMER-SKU-CATEGORY-CLEANUP-BOM-VERSION-DEACTIVATE",
		"UT-322-CUSTOMER-SKU-CATEGORY-CLEANUP-BOM-VERSION-DEACTIVATE",
		"API-322-CUSTOMER-SKU-CATEGORY-CLEANUP-BOM-VERSION-DEACTIVATE",
		"REV-322-CUSTOMER-SKU-CATEGORY-CLEANUP-BOM-VERSION-DEACTIVATE",
		"active BOM version",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 322 requirement seed missing %q", want)
		}
	}
}

func TestDev322CustomerCategoryCleanupAndBomVersionWiring(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go")))
	for _, want := range []string{
		"func cleanupLegacyPublicCopiesTx",
		"FROM %[1]s.product_categories child",
		"child.active=true",
		"child.parent_id=c.id",
		"func (r Repository) DeactivateProducts",
		"UPDATE %s.bom_versions SET status='disabled'",
		"WHERE product_id = ANY($1) AND status='active'",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 322 repository wiring missing %q", want)
		}
	}
}

func TestDev322DocsAndAcceptanceCoverCategoryCleanupAndBomVersion(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_COSTING.md"),
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"),
		filepath.Join("docs", "acceptance", "2026-05-22-customer-sku-category-cleanup-bom-version-deactivate.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-322",
			"活跃二级分类",
			"active BOM",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 322 documentation marker %q", rel, want)
			}
		}
	}
}
