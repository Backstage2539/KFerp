package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev273ProducePlanRoastOutputYieldAlignRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-273-PRODUCTION-PLAN-ROAST-OUTPUT-YIELD-ALIGN",
		"DEV-273-01",
		"DEV-273-02",
		"UT-273-01",
		"API-273-01",
		"REV-273-01",
		"实时显示预计成品",
		"损耗比与生产计划 BOM 出品率保持一致",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store seed missing %q", want)
		}
	}
}

func TestDev273ProducePlanRoastSuggestionUsesRealtimeOutputAndUnifiedYieldDisplay(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue")))
	for _, avoid := range []string{
		"预计成品(kg)",
		`v-model.number="row.batch_count"`,
		"roastExpectedFinishedKg(row)",
		"percent(row.yield_rate)",
	} {
		if strings.Contains(view, avoid) {
			t.Fatalf("ProducePlanView should not expose legacy roast suggestion marker %q", avoid)
		}
	}
	for _, want := range []string{
		"BOM摘要",
		"工艺路线摘要",
		"buildProductionPlanCreatePayload(filters, keys)",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("ProducePlanView missing generic manufacturing marker %q", want)
		}
	}
}
