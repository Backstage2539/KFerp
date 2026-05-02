package support

import (
	"strings"
	"testing"
)

func TestFinanceRequirementsManualAndSeedsAreRecorded(t *testing.T) {
	for _, tc := range []struct {
		path  string
		wants []string
	}{
		{
			path: "internal/interfaces/http/support/req_store.go",
			wants: []string{
				"PR-FIN-001",
				"DEV-FIN-001",
				"UT-FIN-001",
				"API-FIN-001",
				"REV-FIN-001",
				"PR-FIN-003",
				"PR-FIN-004",
			},
		},
		{
			path: "docs/REQUIREMENTS.md",
			wants: []string{
				"财务月结",
				"咖啡贸易商",
				"咖啡壳豆加工厂",
				"关联员工",
				"模糊搜索",
			},
		},
		{
			path: "docs/ACCEPTANCE_TESTS.md",
			wants: []string{
				"财务月结",
				"强锁账",
				"PDF 和 Excel",
				"点击员工",
				"付款方式",
			},
		},
		{
			path: "docs/finance-monthly-closing-user-manual.md",
			wants: []string{
				"财务首页",
				"月度结账",
				"结账后调整",
				"关联员工",
				"候选",
			},
		},
	} {
		src := readSupportTestFile(t, tc.path)
		for _, want := range tc.wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", tc.path, want)
			}
		}
	}
}
