package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionCancelWIPReleaseEvidenceExists(t *testing.T) {
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "production_flow_api_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_PRODUCTION.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"TestProduceCancelAPIReleasesWIPReservationAndCancelsWorkOrder",
		"work_order_material_reservations",
		"returned_g",
		"released",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("production cancel API test missing WIP release marker %q", want)
		}
	}
	for _, want := range []string{
		"取消生产时，系统会同步取消生产工单和工序卡",
		"未消耗 WIP 占用为已释放",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("production manual missing WIP release marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-232-PRODUCTION-CANCEL-WIP-RELEASE-EVIDENCE",
		"DEV-232-PRODUCTION-CANCEL-WIP-RELEASE-EVIDENCE",
		"TestProduceCancelAPIReleasesWIPReservationAndCancelsWorkOrder",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing WIP release marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing WIP release marker %q", want)
		}
	}
	for _, want := range []string{
		"PRODUCTION_CANCEL_WIP_RELEASE_UI_CLICK_OK",
		"cancel_running_item_77",
		"http://127.0.0.1:18136/vue-shell?view=produceRunning",
		"生产已取消",
		"cancelled|cancelled|cancelled|released:400",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing WIP release UI marker %q", want)
		}
	}
}
