package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev343OrderNoticePaymentHintSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-343-ORDER-NOTICE-DEDUP-PAYMENT-HINT",
		"DEV-343-ORDER-NOTICE-DEDUP-PAYMENT-HINT",
		"UT-343-ORDER-NOTICE-DEDUP-PAYMENT-HINT",
		"API-343-ORDER-NOTICE-DEDUP-PAYMENT-HINT",
		"REV-343-ORDER-NOTICE-DEDUP-PAYMENT-HINT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-341 requirement seed missing %q", want)
		}
	}
}

func TestDev343OrderNoticePaymentHintWiring(t *testing.T) {
	appSrc := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	orderSrc := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	messageSrc := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "messagecenter", "service.go")))
	messageRepoSrc := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "messagecenter", "repository.go")))
	for _, want := range []string{
		"dedupeNotifications",
		"allNotifications = computed(() => dedupeNotifications([...localNotifications.value, ...filterDismissedNotifications(notifications.value, dismissedNotificationIDs.value)]))",
		"visibleNotifications = computed(() => notificationWindow(allNotifications.value, notificationWindowStart.value, notificationWindowSize))",
		"dedupeNotificationRows",
		"SELECT DISTINCT ON (e.id)",
	} {
		if !strings.Contains(appSrc+messageSrc+messageRepoSrc, want) {
			t.Fatalf("notification dedupe wiring missing %q", want)
		}
	}
	for _, want := range []string{
		"paymentGoodsAmountSuggestion",
		"showPaymentGoodsAmountSuggestion",
		"applyPaymentGoodsAmountSuggestion",
		"amount-suggestion-popover",
		"货款 {{ paymentGoodsAmountSuggestion }}",
	} {
		if !strings.Contains(orderSrc, want) {
			t.Fatalf("OrderEntryView.vue missing payment hint marker %q", want)
		}
	}
}

func TestDev343OrderNoticePaymentHintDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-343-ORDER-NOTICE-DEDUP-PAYMENT-HINT",
			"货款提示",
			"重复通知",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-343-ORDER-NOTICE-DEDUP-PAYMENT-HINT",
			"货款提示",
			"重复通知",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"PR-343-ORDER-NOTICE-DEDUP-PAYMENT-HINT",
			"货款提示",
		},
		filepath.Join("docs", "OP_MANUAL_NOTIFICATIONS.md"): {
			"PR-343-ORDER-NOTICE-DEDUP-PAYMENT-HINT",
			"重复通知",
		},
		filepath.Join("docs", "acceptance", "2026-05-23-order-notice-payment-hint.md"): {
			"PR-343-ORDER-NOTICE-DEDUP-PAYMENT-HINT",
			"货款提示",
			"重复通知",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-341 documentation marker %q", rel, want)
			}
		}
	}
}
