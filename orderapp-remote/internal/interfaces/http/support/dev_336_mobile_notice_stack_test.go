package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev336MobileNoticeStackSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-336-MOBILE-NOTICE-STACK",
		"DEV-336-MOBILE-NOTICE-STACK",
		"UT-336-MOBILE-NOTICE-STACK",
		"API-336-MOBILE-NOTICE-STACK",
		"REV-336-MOBILE-NOTICE-STACK",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-336 requirement seed missing %q", want)
		}
	}
}

func TestDev336MobileNoticeStackWiring(t *testing.T) {
	app := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	for _, want := range []string{
		`class="global-notification-stack"`,
		`visibleNotifications = computed(() => notifications.value.slice(0, 3))`,
		`notificationStack = ref(null)`,
		`getBoundingClientRect().bottom`,
		`notificationStackStyle = computed`,
		`--kferp-notice-stack-space`,
		`margin-top: -10px`,
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("App.vue missing mobile notice stack marker %q", want)
		}
	}

	orderEntry := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		`--notice-stack-offset: var(--kferp-notice-stack-space, 0px)`,
		`top: calc(max(12px, env(safe-area-inset-top)) + var(--notice-stack-offset))`,
	} {
		if !strings.Contains(orderEntry, want) {
			t.Fatalf("OrderEntryView.vue missing mobile notice offset marker %q", want)
		}
	}
}

func TestDev336MobileNoticeStackDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_NOTIFICATIONS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-23-mobile-notice-stack.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-336-MOBILE-NOTICE-STACK",
			"手机",
			"通知",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-336 documentation marker %q", rel, want)
			}
		}
	}
}
