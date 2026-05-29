package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionMultiSpecPartialFinishGuardEvidenceExists(t *testing.T) {
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "production_flow_api_test.go")))
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "production", "running_repository.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_PRODUCTION.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"cmd.Partial",
		"合并多规格生产暂不支持部分完工",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("production repository missing multi-spec partial finish marker %q", want)
		}
	}
	for _, want := range []string{
		"TestProduceFinishAPIRejectsPartialForMultiSpecRunWithoutWritingArtifacts",
		"合并多规格生产暂不支持部分完工",
		"produce_running_outputs",
		"finished_inventory",
		"material_consumption_logs",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("production API test missing multi-spec partial finish marker %q", want)
		}
	}
	for _, want := range []string{
		"合并多规格生产单",
		"不能使用部分完工",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("production manual missing multi-spec partial finish marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-234-PRODUCTION-MULTISPEC-PARTIAL-FINISH-GUARD",
		"DEV-234-PRODUCTION-MULTISPEC-PARTIAL-FINISH-GUARD",
		"TestProduceFinishAPIRejectsPartialForMultiSpecRunWithoutWritingArtifacts",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing multi-spec partial finish marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing multi-spec partial finish marker %q", want)
		}
	}
	for _, want := range []string{
		"PRODUCTION_MULTISPEC_PARTIAL_UI_CLICK_OK",
		"multi_spec_finish_click_with_partial_tamper",
		"ui_partial_checkbox_count=0",
		"db=running_outputs_2_finished_0_logs_0_inventory_0_stock_batches_0_consumption_0",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing multi-spec partial finish UI marker %q", want)
		}
	}
}
