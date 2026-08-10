package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev596SemiFinishedPackagingBomContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "domain", "bom", "bom_kind.go"): {
			"BomKindProduct",
			"BomKindSpecPackaging",
			"IsValidBomKind",
			"IsPackagingBomKind",
		},
		filepath.Join("internal", "application", "bom", "service.go"): {
			"BomKind",
			"OutputIsSemiFinished",
			"validatePackagingBomDraftItem",
			"packaging BOM items must use fixed quantity",
			"packaging BOM items must be materials",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "schema.go"): {
			"bom_kind",
			"output_is_semi_finished",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"): {
			"validatePackagingBomVersionItems",
			"pb.bom_kind",
			"pb.output_is_semi_finished",
		},
		filepath.Join("internal", "interfaces", "http", "bom", "bom_api.go"): {
			"BomKind",
			"OutputIsSemiFinished",
		},
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-596-SEMI-FINISHED-PACKAGING-BOM",
			"DEV-596-BOM-KIND-EXTENSION",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-596 marker %q", rel, want)
			}
		}
	}
}
