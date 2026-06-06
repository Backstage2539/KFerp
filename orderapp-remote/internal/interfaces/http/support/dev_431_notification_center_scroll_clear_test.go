package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev431NotificationCenterScrollClearSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-431-NOTIFICATION-CENTER-SCROLL-CLEAR",
		"DEV-431-NOTIFICATION-WINDOW-CONTROLS",
		"DEV-431-NOTIFICATION-CLEAR-READ-SYNC",
		"UT-431-NOTIFICATION-CENTER-SCROLL-CLEAR",
		"API-431-NOTIFICATION-CENTER-SCROLL-CLEAR",
		"REV-431-NOTIFICATION-CENTER-SCROLL-CLEAR",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-431 requirement seed missing %q", want)
		}
	}
}

func TestDev431NotificationCenterScrollClearWiring(t *testing.T) {
	app := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	lib := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "global-notifications.js")))
	api := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "api", "message-center.js")))

	for _, want := range []string{
		"const notificationFetchLimit = 100",
		"fetchERPNotifications(notificationFetchLimit)",
		`class="notification-window-toolbar"`,
		`aria-label="上一条通知"`,
		`aria-label="下一条通知"`,
		`@click="clearAllNotifications"`,
		"notificationBackendIDs(allNotifications.value)",
		"Promise.allSettled(ids.map((id) => markNotificationRead(id)))",
		"visibleNotifications = computed(() => notificationWindow(allNotifications.value, notificationWindowStart.value, notificationWindowSize))",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("App.vue missing notification center marker %q", want)
		}
	}
	for _, want := range []string{
		"export function notificationWindow",
		"export function clampNotificationWindowStart",
		"export function notificationBackendIDs",
	} {
		if !strings.Contains(lib, want) {
			t.Fatalf("global-notifications.js missing marker %q", want)
		}
	}
	if !strings.Contains(api, "export function fetchERPNotifications(limit = 100)") {
		t.Fatalf("message-center API should default to fetching the full notification window")
	}
}

func TestDev431NotificationCenterScrollClearDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-431-NOTIFICATION-CENTER-SCROLL-CLEAR",
			"上下箭头",
			"一键清空",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-431-NOTIFICATION-CENTER-SCROLL-CLEAR",
			"清空通知",
			"不再回弹",
		},
		filepath.Join("docs", "OP_MANUAL_NOTIFICATIONS.md"): {
			"PR-431-NOTIFICATION-CENTER-SCROLL-CLEAR",
			"清空",
			"上下箭头",
		},
		filepath.Join("docs", "acceptance", "2026-06-06-notification-center-scroll-clear.md"): {
			"PR-431",
			"notificationBackendIDs",
			"清空通知",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-431 documentation marker %q", rel, want)
			}
		}
	}
}
