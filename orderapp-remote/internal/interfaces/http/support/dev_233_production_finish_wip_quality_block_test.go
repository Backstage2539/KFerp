package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionFinishWIPQualityBlockEvidenceExists(t *testing.T) {
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "production_flow_api_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_PRODUCTION.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"TestProduceFinishAPIRejectsHeldWIPBatchWithoutWritingFinishArtifacts",
		"WIP stock blocked by quality status",
		"production_logs",
		"finished_inventory",
		"material_consumption_logs",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("production finish API test missing WIP quality block marker %q", want)
		}
	}
	for _, want := range []string{
		"完成生产遇到 WIP 批次质检待处理或不通过",
		"冻结 WIP 批次导致完成生产失败",
		"quality block",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("production manual missing WIP quality block marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-233-PRODUCTION-FINISH-WIP-QUALITY-BLOCK-EVIDENCE",
		"DEV-233-PRODUCTION-FINISH-WIP-QUALITY-BLOCK-EVIDENCE",
		"TestProduceFinishAPIRejectsHeldWIPBatchWithoutWritingFinishArtifacts",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing WIP quality block marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing WIP quality block marker %q", want)
		}
	}
	for _, want := range []string{
		"PRODUCTION_FINISH_WIP_QUALITY_UI_CLICK_OK",
		"finish_held_wip_batch",
		"http://127.0.0.1:18135/vue-shell?view=produceRunning",
		"WIP stock blocked by quality status for material 10",
		"running|0|0|0|0",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing WIP quality UI marker %q", want)
		}
	}
}
