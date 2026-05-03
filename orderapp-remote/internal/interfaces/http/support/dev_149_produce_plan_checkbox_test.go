package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProducePlanInsufficientCheckboxRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-149",
		"DEV-149-01",
		"UT-149-01",
		"API-149-01",
		"REV-149-01",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestProducePlanInsufficientCheckboxSourceGuard(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue")))
	for _, want := range []string{
		"insufficientHeaderCheckbox",
		"insufficientSelection.indeterminate",
		"aria-label=\"全选库存不足商品\"",
		"toggleInsufficientRow",
		"buildInsufficientSelection",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProducePlanView.vue missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"库存不足全选/全取消",
		">全选库存不足<",
		">全取消<",
		"pickInsufficient",
		"clearSelected",
	} {
		if strings.Contains(src, unwanted) {
			t.Fatalf("ProducePlanView.vue still contains obsolete control %q", unwanted)
		}
	}
}
