package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionPartialOrderStatusGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "production", "running_repository.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "production_flow_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"orderHasRemainingProductionGapTx",
		"completeOrderIfAllRunningDone",
		"production_logs",
		"order_stock_decisions",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("production repository missing partial-order status marker %q", want)
		}
	}
	for _, want := range []string{
		"TestProduceFinishAPIKeepsOrderInProductionWhenOtherItemsRemainUnproduced",
		"SO-PARTIAL-ORDER",
		"want 生产中 until all order items are produced",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("production API test missing partial-order status marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-208-PRODUCTION-PARTIAL-ORDER-STATUS-GUARD",
		"多品项订单只完成部分生产品项",
		"TestProduceFinishAPIKeepsOrderInProductionWhenOtherItemsRemainUnproduced",
		"PRODUCTION_PARTIAL_ORDER_STATUS_UI_CLICK_OK",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing partial-order status marker %q", want)
		}
	}
}

func TestProductionPartialOrderStatusGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-208-PRODUCTION-PARTIAL-ORDER-STATUS-GUARD",
		"DEV-208-PRODUCTION-PARTIAL-ORDER-STATUS-GUARD",
		"UT-208-PRODUCTION-PARTIAL-ORDER-STATUS-GUARD",
		"API-208-PRODUCTION-PARTIAL-ORDER-STATUS-GUARD",
		"REV-208-PRODUCTION-PARTIAL-ORDER-STATUS-GUARD",
		"PRODUCTION_PARTIAL_ORDER_STATUS_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestProductionPartialOrderStatusGuardManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"多品项订单",
			"剩余生产缺口",
			"生产完成",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing partial-order status marker %q", path, want)
			}
		}
	}
}
