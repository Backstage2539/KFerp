package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionWIPListAvailabilityGuardEvidenceExists(t *testing.T) {
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "production_flow_api_test.go")))
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "production", "wip_reservation.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_PRODUCTION.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"func (r Repository) ListWIPReservations",
		"AND l.qty_g > 0",
		"AND b.status='active'",
		"AND b.remaining_g > 0",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("production WIP list repository missing active remaining marker %q", want)
		}
	}
	for _, want := range []string{
		"TestProduceWIPReservationsAPIExcludesInactiveAndDepletedBatchesFromAvailability",
		"MB-WIP-LIST-INACTIVE",
		"MB-WIP-LIST-DEPLETED",
		"active-remaining-only 1000/600",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("production API test missing WIP list availability marker %q", want)
		}
	}
	for _, want := range []string{
		"active、仍有剩余且非待处理/拒收冻结批次",
		"已停用或已耗尽的 WIP 批次不会计入可用量",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("production manual missing WIP list availability marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-239-PRODUCTION-WIP-LIST-ACTIVE-REMAINING-AVAILABILITY",
		"DEV-239-PRODUCTION-WIP-LIST-ACTIVE-REMAINING-AVAILABILITY",
		"TestProduceWIPReservationsAPIExcludesInactiveAndDepletedBatchesFromAvailability",
		"PRODUCTION_WIP_LIST_ACTIVE_REMAINING_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing WIP list availability marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing WIP list availability marker %q", want)
		}
	}
}
