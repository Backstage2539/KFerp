package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev489ProductionPlanPreviewLayoutContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-489-PRODUCTION-PLAN-PREVIEW-LAYOUT",
			"DEV-489-PREVIEW-TABLE-DRAG-COLLAPSE",
			"DEV-489-SPLIT-VISIBILITY-HINT",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"drag-scroll-wrap",
			"startTableScrollDrag",
			"收起待生产需求",
			"收起当前生产计划",
			"创建草稿生产计划后可填写工序产能拆分",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-489-PRODUCTION-PLAN-PREVIEW-LAYOUT",
			"计划预览表格支持横向拖拽滚动",
			"左右两栏支持收起和展开",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-489-PRODUCTION-PLAN-PREVIEW-LAYOUT",
			"拖拽计划预览表格可查看 BOM 摘要、计划投料和工艺路线摘要",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-489-PRODUCTION-PLAN-PREVIEW-LAYOUT",
			"收起待生产需求",
			"创建草稿生产计划后可填写工序产能拆分",
		},
		filepath.Join("docs", "acceptance", "2026-06-12-production-plan-preview-layout.md"): {
			"PR-489 Production Plan Preview Layout",
			"左右两栏支持收起和展开",
			"计划预览表格空白区域左右拖动",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-489 marker %q", rel, want)
			}
		}
	}
}
