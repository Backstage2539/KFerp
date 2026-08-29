package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev618BomRouteCapacityErrorDetailContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "standard_cost_capacity_issue.go"): {
			"FindStandardCostCapacityIssue",
			"工艺路线",
			"标准成本产能档",
			"工位",
			"不适用工序",
			"已停用",
			"重新选择有效的标准成本产能档",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"): {
			"postgresinfra.FindStandardCostCapacityIssue",
			"postgresinfra.StandardCostCapacityIssue",
		},
		filepath.Join("internal", "infrastructure", "postgres", "manufacturing", "repository.go"): {
			"postgresinfra.FindStandardCostCapacityIssue",
		},
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-618-BOM-ROUTE-CAPACITY-ERROR-DETAIL",
			"DEV-618-ROUTE-CAPACITY-ISSUE-DIAGNOSTIC",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-618-BOM-ROUTE-CAPACITY-ERROR-DETAIL",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-618-BOM-ROUTE-CAPACITY-ERROR-DETAIL",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"工艺路线 -> 第几道工序 -> 产能档 -> 工位",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-618 marker %q", rel, want)
			}
		}
	}

	for _, rel := range []string{
		filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"),
		filepath.Join("internal", "infrastructure", "postgres", "manufacturing", "repository.go"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		if strings.Contains(src, "标准成本产能档必须来自启用工位且适用当前工序") {
			t.Fatalf("%s still contains the old ambiguous capacity error", rel)
		}
	}
}
