package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionWIPReleaseRunningItemGuardEvidenceExists(t *testing.T) {
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "production_flow_api_test.go")))
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "production", "wip_reservation.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_PRODUCTION.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"whereSQL := strings.Join(where, \" AND \")",
		"EXISTS (SELECT 1 FROM %s.work_orders wo WHERE wo.id=res.work_order_id AND wo.work_order_no=$%d)",
		"UPDATE %s.work_order_material_reservations res",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("production WIP release repository missing running item guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestProduceWIPReservationReleaseAPIReleasesRunningReservationWithoutWorkOrderRow",
		"孤立WIP占用生豆",
		"released/400",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("production API test missing WIP release running item guard marker %q", want)
		}
	}
	for _, want := range []string{
		"即使历史占用缺少工单行",
		"响应的释放数量必须和数据库占用状态一致",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("production manual missing WIP release running item guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-238-PRODUCTION-WIP-RELEASE-RUNNING-ITEM-GUARD",
		"DEV-238-PRODUCTION-WIP-RELEASE-RUNNING-ITEM-GUARD",
		"TestProduceWIPReservationReleaseAPIReleasesRunningReservationWithoutWorkOrderRow",
		"PRODUCTION_WIP_RELEASE_RUNNING_ITEM_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing WIP release running item guard marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing WIP release running item guard marker %q", want)
		}
	}
}
