package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev675BeanListPerformanceRequirementsAndEvidence(t *testing.T) {
	for rel, markers := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-662-BEAN-LIST-PERFORMANCE", "DEV-674-COSTING-QUERY", "DEV-675-COSTING-PERFORMANCE-DELIVERY", `code: "DEV-675-COSTING-PERFORMANCE-DELIVERY", title: "真实 PostgreSQL 性能验收与开发正式环境交付", status: "done"`,
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"商品价格表接口性能优化", "P95 不超过 1 秒", "不增加浏览器结果缓存",
		},
		filepath.Join("docs", "acceptance", "2026-09-14-bean-list-performance.md"): {
			"SET LOCAL jit = off", "bom_unit_cost AS MATERIALIZED", "客户 450",
		},
	} {
		content := string(readOrderAppFileForTest(t, rel))
		for _, marker := range markers {
			if !strings.Contains(content, marker) {
				t.Fatalf("%s missing PR-662 marker %q", rel, marker)
			}
		}
	}
}
