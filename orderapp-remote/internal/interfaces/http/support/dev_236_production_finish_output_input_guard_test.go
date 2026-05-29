package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionFinishOutputInputRatioGuardEvidenceExists(t *testing.T) {
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "production_flow_api_test.go")))
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "production", "running_repository.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_PRODUCTION.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"validateFinishedOutputWithinConsumedInput",
		"finished output cannot exceed consumed input",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("production repository missing output/input guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestProduceFinishAPIRejectsOutputGreaterThanConsumedInputWithoutWritingArtifacts",
		"finished output cannot exceed consumed input",
		"production_logs",
		"finished_inventory",
		"material_consumption_logs",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("production API test missing output/input guard marker %q", want)
		}
	}
	for _, want := range []string{
		"成品总克重不能大于本次消耗投料",
		"不会写完成日志、成品库存、FP 批次或扣料日志",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("production manual missing output/input guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-236-PRODUCTION-FINISH-OUTPUT-INPUT-RATIO-GUARD",
		"DEV-236-PRODUCTION-FINISH-OUTPUT-INPUT-RATIO-GUARD",
		"TestProduceFinishAPIRejectsOutputGreaterThanConsumedInputWithoutWritingArtifacts",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing output/input guard marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing output/input guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PRODUCTION_FINISH_OUTPUT_INPUT_UI_CLICK_OK",
		"finish_3x227g_with_600g_input",
		"http://127.0.0.1:18134/vue-shell?view=produceRunning",
		"running|0|0|0|0",
		"finished output cannot exceed consumed input",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing output/input UI marker %q", want)
		}
	}
}
