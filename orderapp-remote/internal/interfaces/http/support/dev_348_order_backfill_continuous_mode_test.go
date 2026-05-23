package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev348OrderBackfillContinuousModeSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-348-ORDER-BACKFILL-CONTINUOUS-MODE",
		"DEV-348-ORDER-BACKFILL-CONTINUOUS-MODE",
		"UT-348-ORDER-BACKFILL-CONTINUOUS-MODE",
		"API-348-ORDER-BACKFILL-CONTINUOUS-MODE",
		"REV-348-ORDER-BACKFILL-CONTINUOUS-MODE",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("order backfill continuous mode seed missing %q", want)
		}
	}
}

func TestDev348OrderBackfillContinuousModeWiring(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"backfillMode",
		"canUseBackfillMode",
		"保存并继续补录",
		"保存并查看订单",
		"save({ continueBackfill: true })",
		"resetForBackfillContinuation",
		"rows.value = [newRow()]",
		"paymentVoucher.value = null",
		"paymentVoucherFile.value = null",
		"saveOrderEntryDraft()",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrderEntryView.vue missing backfill continuous mode marker %q", want)
		}
	}
}

func TestDev348OrderBackfillContinuousModeDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-348-ORDER-BACKFILL-CONTINUOUS-MODE",
			"保存并继续补录",
			"订单补录",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-348-ORDER-BACKFILL-CONTINUOUS-MODE",
			"保存并继续补录",
			"保存并查看订单",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"PR-348-ORDER-BACKFILL-CONTINUOUS-MODE",
			"保存并继续补录",
			"保存并查看订单",
		},
		filepath.Join("docs", "acceptance", "2026-05-23-order-backfill-continuous-mode.md"): {
			"PR-348-ORDER-BACKFILL-CONTINUOUS-MODE",
			"node --test src/lib/order-entry.test.js",
			"go test ./internal/interfaces/http/support -run TestDev348 -count=1",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing order backfill continuous mode marker %q", rel, want)
			}
		}
	}
}
