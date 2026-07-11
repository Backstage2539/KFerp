package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev527RemoveIndustryCalculationPreviewContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-527-REMOVE-INDUSTRY-CALCULATION-PREVIEW",
			"DEV-527-REMOVE-PREVIEW-UI",
			"DEV-527-PRESERVE-TEMPLATE-EDITOR",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-527-REMOVE-INDUSTRY-CALCULATION-PREVIEW",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K77. 行业字段模板移除计算预览",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-527-REMOVE-INDUSTRY-CALCULATION-PREVIEW",
			"当前页面只维护模板名称、状态、说明和字段定义",
		},
		filepath.Join("docs", "acceptance", "2026-07-11-remove-industry-calculation-preview.md"): {
			"PR-527 行业字段模板移除计算预览验收记录",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-527 marker %q", rel, want)
			}
		}
	}

	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "IndustryFieldTemplatesView.vue")))
	for _, want := range []string{"行业字段模板", "字段定义", "新增字段", "保存模板"} {
		if !strings.Contains(view, want) {
			t.Fatalf("industry field template view missing preserved marker %q", want)
		}
	}
	for _, removed := range []string{"calculator-panel", "runCalculatorPreview", "/api/industry-calculators/preview", "计算预览", "业务预设"} {
		if strings.Contains(view, removed) {
			t.Fatalf("industry field template view still contains removed preview marker %q", removed)
		}
	}
}
