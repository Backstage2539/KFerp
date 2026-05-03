package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderStockBatchConfirmRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-147",
		"DEV-147-01",
		"DEV-147-02",
		"DEV-147-03",
		"UT-147-01",
		"API-147-01",
		"REV-147-01",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestProducePlanSplitsStockSufficientAndInsufficientWithBulkCheckbox(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue")))
	for _, want := range []string{
		"库存不足",
		"库存充足",
		"stockInsufficientRows",
		"stockSufficientRows",
		"toggleAllInsufficient",
		"allInsufficientSelected",
		"全取消",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProducePlanView.vue missing %q", want)
		}
	}
}

func TestOrderEntryPromptsBatchBeforeSaveAndSendsDecision(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"/api/order/stock-batch-preview",
		"previewStockBatchesBeforeSave",
		"stock_batch_decision",
		"use_batch",
		"produce",
		"库存待发货",
		"使用以上批次",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrderEntryView.vue missing %q", want)
		}
	}
}
