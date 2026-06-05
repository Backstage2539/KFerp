package support

import (
	"strings"
	"testing"
)

func TestDev426DripCustomerOrderAcceptsProductionBOMModel(t *testing.T) {
	for _, rel := range []string{
		"internal/infrastructure/postgres/customerfulfillment/repository.go",
		"internal/infrastructure/postgres/customerportal/business_repository.go",
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"production_boms",
			"production_bom_versions",
			"production_bom_version_items",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-426 production BOM compatibility marker %q", rel, want)
			}
		}
	}
}
