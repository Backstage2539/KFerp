package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev430BomUsageCurrentVersionRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-430-BOM-USAGE-CURRENT-VERSION",
		"DEV-430-BOM-USAGE-CURRENT-VERSION",
		"UT-430-BOM-USAGE-CURRENT-VERSION",
		"API-430-BOM-USAGE-CURRENT-VERSION",
		"REV-430-BOM-USAGE-CURRENT-VERSION",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-430 requirement seed missing %q", want)
		}
	}
}

func TestDev430BomUsageCurrentVersionSourceMarkers(t *testing.T) {
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go")))
	for _, want := range []string{
		"current_usage_versions AS (",
		"current_component_versions AS (",
		"JOIN current_usage_versions cv ON cv.bom_id=pb.id",
		"JOIN current_component_versions cv ON cv.bom_id=pb.id",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("repository missing current-version BOM usage marker %q", want)
		}
	}
}

func TestDev430BomUsageCurrentVersionDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-430-BOM-USAGE-CURRENT-VERSION",
			"商品档案和 BOM 详情的组件反查必须只读取每个生产 BOM 当前有效版本",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-430-BOM-USAGE-CURRENT-VERSION",
			"GoalE2E-0605-234447 咖啡熟豆",
			"BOM-001435",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-430",
			"组件反查只读取每个生产 BOM 的当前有效版本",
		},
		filepath.Join("docs", "acceptance", "2026-06-06-bom-usage-current-version.md"): {
			"PR-430",
			"current_usage_versions",
			"GoalE2E-0605-234447 咖啡熟豆",
		},
	}
	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-430 documentation marker %q", rel, want)
			}
		}
	}
}
