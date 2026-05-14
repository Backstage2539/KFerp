package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionWIPAdjustQualityAvailabilityEvidenceExists(t *testing.T) {
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "production_flow_api_test.go")))
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "production", "wip_reservation.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_PRODUCTION.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')",
		"b.status='active'",
		"b.remaining_g > 0",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("production WIP reservation repository missing quality availability marker %q", want)
		}
	}
	for _, want := range []string{
		"TestProduceWIPReservationAdjustAPIExcludesHeldWIPFromReturnedAvailability",
		"MB-WIP-HOLD",
		"pass-quality-only 1000/100",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("production API test missing WIP adjust quality availability marker %q", want)
		}
	}
	for _, want := range []string{
		"WIP 占用抽屉的 WIP 总量和可用量只统计 active、仍有剩余且非待处理/拒收冻结批次",
		"调整后可用量没有包含冻结、拒收、已停用或已耗尽批次",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("production manual missing WIP adjust quality availability marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-237-PRODUCTION-WIP-ADJUST-QUALITY-AVAILABILITY",
		"DEV-237-PRODUCTION-WIP-ADJUST-QUALITY-AVAILABILITY",
		"TestProduceWIPReservationAdjustAPIExcludesHeldWIPFromReturnedAvailability",
		"PRODUCTION_WIP_ADJUST_QUALITY_AVAILABILITY_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing WIP adjust quality availability marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing WIP adjust quality availability marker %q", want)
		}
	}
}
